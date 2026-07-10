package main

import (
	"database/sql"
	"log"
	"net"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository"
	delivery "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/delivery/grpc"
	db "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/usecase"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	_ "github.com/lib/pq" // Pure Go PostgreSQL driver registration
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Initializing PhonkDrift Track & Streaming Microservice... 🎵")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical: Could not load configuration values: %v", err)
	}

	if cfg.TrackDbSource == "" {
		log.Fatal("CRITICAL: TRACK_DB_SOURCE environment variable is not defined")
	}

	// 2. Open Connection Pool to Supabase Track Database
	conn, err := sql.Open("postgres", cfg.TrackDbSource)
	if err != nil {
		log.Fatalf("Failed to establish database handle: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("Database connection validation failed: %v", err)
	}
	log.Println("Successfully connected to Supabase Track Database Cluster! ⚡")

	// 2.1 Initialize Redis Client using centralized configurations
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	log.Printf("Successfully connected to Redis Cache at %s! 🚀\n", cfg.RedisAddr)

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

	trackEventPublisher := repository.NewTrackEventPublisher(ch)

	repositoryObj := db.New(conn)
	trackUsecase := usecase.NewTrackUsecase(repositoryObj, rdb, cfg.YouTubeAPIKey, cfg.YtDlpCookiesPath, trackEventPublisher)
	grpcHandler := delivery.NewTrackGRPCHandler(trackUsecase)

	address := ":" + cfg.TrackGrpcPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to bind TCP network port %s: %v", cfg.TrackGrpcPort, err)
	}

	grpcServer := grpc.NewServer()
	trackpb.RegisterTrackServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	log.Printf("Track Service Core Engine listening safely on port %s 🏎️🔥", cfg.TrackGrpcPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Track Service runtime crash encountered: %v", err)
	}
}
