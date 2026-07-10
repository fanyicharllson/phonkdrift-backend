package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Initializing PhonkDrift Background Discovery Service... 🎵")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical: Could not load configuration values: %v", err)
	}

	if cfg.TrackDbSource == "" {
		log.Fatal("CRITICAL: TRACK_DB_SOURCE environment variable is not defined")
	}

	// 1. Database Connection
	conn, err := sql.Open("postgres", cfg.TrackDbSource)
	if err != nil {
		log.Fatalf("Failed to establish database handle: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("Database connection validation failed: %v", err)
	}
	log.Println("Successfully connected to Track Database! ⚡")

	// 2. Redis Connection (check connectivity)
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: Redis cache verification check failed: %v", err)
	} else {
		log.Printf("Successfully verified Redis Cache connection at %s! 🚀\n", cfg.RedisAddr)
	}
	defer rdb.Close()

	// 3. Initialize Discovery components
	worker := discovery.NewWorker(cfg.YouTubeAPIKey)
	
	uploader, err := discovery.NewUploader(
		cfg.DOSpacesKey,
		cfg.DOSpacesSecret,
		cfg.DOSpacesEndpoint,
		cfg.DOSpacesBucket,
		cfg.DOSpacesCDNURL,
		cfg.YtDlpCookiesPath,
	)
	if err != nil {
		log.Fatalf("Failed to initialize S3 uploader: %v", err)
	}

	repo := trackdb.New(conn)

	// Cycle every 12 hours
	scheduler := discovery.NewScheduler(worker, uploader, repo, 12*time.Hour)

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping discovery service gracefully...")
		cancel()
	}()

	// Start Scheduler (blocking)
	scheduler.Start(ctx)
}
