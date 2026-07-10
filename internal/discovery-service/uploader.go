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
	s3Client    *s3.Client
	bucket      string
	cdnBaseURL  string
	cookiesPath string
}

func NewUploader(accessKey, secret, endpoint, bucket, cdnURL, cookiesPath string) (*Uploader, error) {
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
		s3Client:    s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }),
		bucket:      bucket,
		cdnBaseURL:  cdnURL,
		cookiesPath: cookiesPath,
	}, nil
}

// DownloadAndUpload downloads audio via yt-dlp and uploads to DO Spaces
// Returns the permanent CDN URL
func (u *Uploader) DownloadAndUpload(ctx context.Context, youtubeID string) (string, error) {
	log.Printf("⬇️  Downloading audio for YouTube ID: %s", youtubeID)

	// yt-dlp: download best audio as mp3 to stdout
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", youtubeID)
	args := []string{
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--js-runtimes", "node",
		"--no-playlist",
	}
	if u.cookiesPath != "" {
		args = append(args, "--cookies", u.cookiesPath)
	}
	args = append(args, "-o", "-", videoURL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

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
	return u.PutObject(ctx, objectKey, audioData.Bytes(), "audio/mpeg")
}

// PutObject uploads arbitrary bytes to DO Spaces under the given key and
// returns the permanent CDN URL. Shared by DownloadAndUpload (audio) and
// any caller uploading other media (e.g. profile avatars).
func (u *Uploader) PutObject(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	log.Printf("⬆️  Uploading to DO Spaces: %s", key)

	_, err := u.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         "public-read",
	})
	if err != nil {
		return "", fmt.Errorf("S3 upload failed: %w", err)
	}

	cdnURL := fmt.Sprintf("%s/%s", strings.TrimRight(u.cdnBaseURL, "/"), key)
	log.Printf("🌐 Live at CDN: %s", cdnURL)

	return cdnURL, nil
}
