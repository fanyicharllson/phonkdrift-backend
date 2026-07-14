package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/broadcaster"
	grpcDelivery "github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/delivery/grpc"
	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/repository"
	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/usecase"
	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	chatpb "github.com/fanyicharllson/phonkdrift-backend/pb/chat"

	_ "github.com/lib/pq"
	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Starting PhonkDrift Chat Microservice... 💬")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical: Could not load configurations: %v", err)
	}

	if cfg.ChatDbSource == "" {
		log.Fatal("CRITICAL: CHAT_DB_SOURCE environment variable is not defined")
	}

	db, err := sql.Open("postgres", cfg.ChatDbSource)
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

	// Connect to Auth Service — used to denormalize sender username/avatar
	// onto each message, and to send push notifications on replies.
	if cfg.AuthServiceAddr == "" {
		log.Fatal("CRITICAL: AUTH_SERVICE_ADDR environment variable is not defined")
	}
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Critical: Failed to connect to Auth Service: %v", err)
	}
	authClient := authpb.NewAuthServiceClient(authConn)

	bc := broadcaster.New(ch)
	if err := bc.Start(context.Background()); err != nil {
		log.Fatalf("Critical: Failed to start chat broadcaster: %v", err)
	}

	chatRepo := repository.NewChatRepository(db)
	chatUsecase := usecase.NewChatUsecase(chatRepo, bc, authClient)
	chatHandler := grpcDelivery.NewChatGRPCHandler(chatUsecase, bc)

	address := ":" + cfg.ChatGrpcPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Critical: Failed to bind TCP listener on port %s: %v", cfg.ChatGrpcPort, err)
	}

	grpcServer := grpc.NewServer()
	chatpb.RegisterChatServiceServer(grpcServer, chatHandler)
	reflection.Register(grpcServer)

	log.Printf("Chat Microservice engine is fully idling on port %s. Listening...", cfg.ChatGrpcPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Critical: Failed to launch gRPC engine runtime: %v", err)
	}
}
