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

	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Critical: Could not load configurations: %v", err)
	}

	// Connect to Supabase Cloud Postgres
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Critical: Database driver initialization failed: %v", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("Critical: Database handshake failed: %v", err)
	}
	log.Println("Database connection verified successfully. ✓")

	// Connect to CloudAMQP Broker
	amqpConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Critical: Failed to connect to RabbitMQ broker: %v", err)
	}
	defer amqpConn.Close()
	log.Println("RabbitMQ Event Broker connection established successfully. ✓")

	ch, err := amqpConn.Channel()
	if err != nil {
		log.Fatalf("Critical: Failed to open an AMQP network channel: %v", err)
	}
	defer ch.Close()

	// Ensure our communication queue exists
	_, err = ch.QueueDeclare("auth.user_registered", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Failed to declare AMQP queue structure: %v", err)
	}

	// Instantiate Hexagonal Core Dependencies
	authRepo := repository.NewAuthRepository(db)
	eventPub := repository.NewEventPublisher(ch)
	authUseCase := usecase.NewAuthUseCase(authRepo, eventPub)

	// Fire up the TCP Network Listener
	address := ":" + cfg.GRPCPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Critical: Failed to bind TCP listener on port %s: %v", cfg.GRPCPort, err)
	}
	log.Printf("Auth gRPC Service bound successfully on %s ✓", address)

	grpcServer := grpc.NewServer()

	// REGISTER YOUR NEW AUTH HANDLER LAYER HERE 🚀
	authHandler := grpcDelivery.NewAuthGRPCHandler(authUseCase) // adjust 'grpcDelivery' alias as needed based on your imports
	authpb.RegisterAuthServiceServer(grpcServer, authHandler)
	
	log.Printf("Auth Microservice engine is fully idling on port %s. Listening...", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Critical: Failed to launch gRPC engine runtime: %v", err)
	}
}