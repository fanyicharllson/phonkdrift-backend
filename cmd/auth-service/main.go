package main

import (
	"database/sql"
	"log"
	"net"
	"time"

	grpcDelivery "github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/delivery/grpc"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/repository"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/usecase"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/workers"
	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"

	"github.com/fanyicharllson/phonkdrift-backend/pb/auth"

	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting PhonkDrift Auth Microservice...")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical: Could not load configurations: %v", err)
	}

	// Connect to Supabase Cloud Postgres
	db, err := sql.Open("postgres", cfg.AuthDbSource)
	if err != nil {
		log.Fatalf("Critical: Database driver initialization failed: %v", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("Critical: Database handshake failed: %v", err)
	}
	log.Println("Database connection verified successfully. ✓")

	// Connect to RabbitMQ Broker with automated fallback protection
	var amqpConn *amqp091.Connection

	log.Printf("Attempting connection to Primary RabbitMQ Broker... 🌐")
	amqpConn, err = amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
	log.Printf("⚠️ Primary Message Broker unreachable. Initiating failover fallback...")
	
	if cfg.RabbitMQFallbackURL == "" {
		log.Fatalf("Critical: Primary broker failed and no RabbitMQFallbackURL was defined.")
	}

	// 🔄 Max Retry Loop to accommodate local container boot delays
	maxRetries := 30
	for i := 1; i <= maxRetries; i++ {
		log.Printf("Attempting connection to Internal Fallback Broker (Attempt %d/%d)... 🏎️", i, maxRetries)
		amqpConn, err = amqp091.Dial(cfg.RabbitMQFallbackURL)
		if err == nil {
			break // Connection succeeded! Break out of the loop
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

	// 🚀 CENTRAL TOPIC EXCHANGE TOPOLOGY (CLEAN & DECOUPLED)
	err = ch.ExchangeDeclare(
		"auth.events", // exchange name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		log.Fatalf("Critical: Failed to declare core exchange topology mapping: %v", err)
	}

	// Instantiate Hexagonal Core Dependencies
	authRepo := repository.NewAuthRepository(db)
	eventPub := repository.NewEventPublisher(ch)
	authUseCase := usecase.NewAuthUseCase(authRepo, eventPub)

	// Avatar upload worker: consumes profile.avatar_updated events (published after
	// the gateway has already uploaded the image to DO Spaces) and persists the URL + FCM push
	avatarWorker := workers.NewAvatarWorker(ch, authRepo)
	avatarWorker.Start()

	// Fire up the TCP Network Listener
	address := ":" + cfg.AuthGrpcPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Critical: Failed to bind TCP listener on port %s: %v", cfg.AuthGrpcPort, err)
	}
	log.Printf("Auth gRPC Service bound successfully on %s ✓", address)

	grpcServer := grpc.NewServer()

	authHandler := grpcDelivery.NewAuthGRPCHandler(authUseCase)
	authpb.RegisterAuthServiceServer(grpcServer, authHandler)

	log.Printf("Auth Microservice engine is fully idling on port %s. Listening...", cfg.AuthGrpcPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Critical: Failed to launch gRPC engine runtime: %v", err)
	}
}
