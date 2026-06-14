package main

import (
	"database/sql"
	"log"
	"net"

	grpcDelivery "github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/delivery/grpc"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/repository"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/usecase"
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
		log.Printf("⚠️ Primary Message Broker unreachable: %v. Initiating failover fallback...", err)

		if cfg.RabbitMQFallbackURL == "" {
			log.Fatalf("Critical: Primary broker failed and no RabbitMQFallbackURL environment string was defined.")
		}

		log.Printf("Attempting connection to Internal Fallback RabbitMQ Broker... 🏎️")
		amqpConn, err = amqp091.Dial(cfg.RabbitMQFallbackURL)
		if err != nil {
			log.Fatalf("Critical: Total system blackout. Both primary and fallback message brokers are unreachable: %v", err)
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
