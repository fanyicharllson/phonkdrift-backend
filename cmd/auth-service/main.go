package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"

	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting PhonkDrift Auth Microservice...")

	// 1. Load Configurations
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Critical: Could not load configurations: %v", err)
	}

	// 2. Connect to Supabase Cloud Postgres
	log.Println("Establishing connection to Supabase Cloud Storage...")
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Critical: Database driver initialization failed: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Critical: Database handshake failed: %v", err)
	}
	log.Println("Database connection verified successfully. ✓")

	// 3. Connect to CloudAMQP (RabbitMQ) Broker
	log.Println("Connecting to CloudAMQP Event Broker...")
	amqpConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Critical: Failed to connect to RabbitMQ broker: %v", err)
	}
	defer amqpConn.Close()
	log.Println("RabbitMQ Event Broker connection established successfully. ✓")

	// Open a unique, transient channel over the TCP connection connection
	ch, err := amqpConn.Channel()
	if err != nil {
		log.Fatalf("Critical: Failed to open an AMQP network channel: %v", err)
	}
	defer ch.Close()

	// Declare a durable Queue to store registration events safely
	queueName := "auth.user_registered"
	_, err = ch.QueueDeclare(
		queueName, // queue name
		true,      // durable (survives broker crashes)
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Critical: Failed to declare AMQP queue structure: %v", err)
	}

	// 4. Fire a Test Event to verify publishing works flawlessly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testBody := `{"username":"test_phonk_head", "email":"verify@phonkdrift.com", "code":"123456"}`
	err = ch.PublishWithContext(ctx,
		"",        // exchange (default nameless exchange routes directly by queue name match)
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        []byte(testBody),
		},
	)
	if err != nil {
		log.Printf("Warning: Could not publish test verification event packet: %v", err)
	} else {
		log.Printf("Verification Pipeline Verification: Test event successfully pushed to '%s' pipeline! 🚀", queueName)
	}

	// 5. Fire up the TCP Network Listener
	address := ":" + cfg.GRPCPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Critical: Failed to bind TCP listener on port %s: %v", cfg.GRPCPort, err)
	}
	log.Printf("Auth gRPC Service bound successfully on %s ✓", address)

	grpcServer := grpc.NewServer()

	log.Printf("Auth Microservice engine is fully idling on port %s. Listening...", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Critical: Failed to launch gRPC engine runtime: %v", err)
	}
}
