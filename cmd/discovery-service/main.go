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
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"github.com/rabbitmq/amqp091-go"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// Connect to RabbitMQ Broker with automated fallback protection (same pattern as auth-service)
	var amqpConn *amqp091.Connection
	log.Printf("Attempting connection to Primary RabbitMQ Broker... 🌐")
	amqpConn, err = amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Printf("⚠️ Primary Message Broker unreachable. Initiating failover fallback...")

		if cfg.RabbitMQFallbackURL == "" {
			log.Fatalf("Critical: Primary broker failed and no RabbitMQFallbackURL was defined.")
		}

		maxRetries := 30
		for i := 1; i <= maxRetries; i++ {
			log.Printf("Attempting connection to Internal Fallback Broker (Attempt %d/%d)... 🏎️", i, maxRetries)
			amqpConn, err = amqp091.Dial(cfg.RabbitMQFallbackURL)
			if err == nil {
				break
			}
			log.Printf("⚠️ Fallback broker not ready yet (connection refused). Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
		}

		if err != nil {
			log.Fatalf("Critical: Total system blackout. Fallback broker remained unreachable after retries: %v", err)
		}
	}
	defer amqpConn.Close()
	log.Println("RabbitMQ Event Broker connection safely established! ✓")

	ch, err := amqpConn.Channel()
	if err != nil {
		log.Fatalf("Critical: Failed to open an AMQP network channel: %v", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare("track.events", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("Critical: Failed to declare track.events exchange: %v", err)
	}

	// On-demand background download worker: fires when track-service persists a
	// track from a user favorite/playlist-add, independent of the 12h scheduled cycle
	trackDownloadWorker := discovery.NewTrackDownloadWorker(ch, uploader, repo)
	trackDownloadWorker.Start()

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

	// Trending notifier: auto-broadcasts a push whenever a track becomes
	// approved-but-unannounced, on top of the existing manual admin trigger
	if cfg.AuthServiceAddr != "" {
		authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: Failed to connect to Auth Service for trending notifier: %v", err)
		} else {
			authClient := authpb.NewAuthServiceClient(authConn)
			notifier := discovery.NewTrendingNotifier(repo, authClient, 15*time.Minute)
			go notifier.Start(ctx)
		}
	} else {
		log.Println("Warning: AUTH_SERVICE_ADDR not set — automatic trending notifications disabled")
	}

	// Start Scheduler (blocking)
	scheduler.Start(ctx)
}
