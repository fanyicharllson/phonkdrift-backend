package discovery

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Uploader struct {
	s3Client   *s3.Client
	bucket     string
	cdnBaseURL string
}

func NewUploader(accessKey, secret, endpoint, bucket, cdnURL string) (*Uploader, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("fra1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secret, ""),
		),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure S3 client: %w", err)
	}

	return &Uploader{
		s3Client:   s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }),
		bucket:     bucket,
		cdnBaseURL: cdnURL,
	}, nil
}

// DownloadAndUpload downloads audio via yt-dlp and uploads to DO Spaces
// Returns the permanent CDN URL
func (u *Uploader) DownloadAndUpload(ctx context.Context, youtubeID string) (string, error) {
	log.Printf("⬇️  Downloading audio for YouTube ID: %s", youtubeID)

	// yt-dlp: download best audio as mp3 to stdout
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", youtubeID)
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", "-", // output to stdout
		videoURL,
	)

	var audioData bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &audioData
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %s (%w)", stderr.String(), err)
	}

	if audioData.Len() == 0 {
		return "", fmt.Errorf("yt-dlp returned empty audio data")
	}

	log.Printf("✅ Downloaded %d MB for %s",
		audioData.Len()/1024/1024, youtubeID)

	// Upload to DO Spaces
	objectKey := fmt.Sprintf("audio/%s.mp3", youtubeID)
	log.Printf("⬆️  Uploading to DO Spaces: %s", objectKey)

	_, err := u.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(audioData.Bytes()),
		ContentType: aws.String("audio/mpeg"),
		ACL:         "public-read",
	})
	if err != nil {
		return "", fmt.Errorf("S3 upload failed: %w", err)
	}

	cdnURL := fmt.Sprintf("%s/%s", strings.TrimRight(u.cdnBaseURL, "/"), objectKey)
	log.Printf("🌐 Track live at CDN: %s", cdnURL)

	return cdnURL, nil
}