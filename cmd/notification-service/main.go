package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/notification-service/workers"
	"github.com/rabbitmq/amqp091-go"
)

func main() {
	log.Println("Starting Centralized PhonkDrift Notification Microservice Engine...")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical error parsing microservice variables configuration layer: %v", err)
	}

	if cfg.RabbitMQURL == "" || cfg.ResendAPIKey == "" {
		log.Fatal("Critical: Missing environment strings. Verify RABBITMQ_URL and RESEND_API_KEY match configuration.")
	}

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
		log.Fatalf("Critical: Failed to generate functional session channel: %v", err)
	}
	defer ch.Close()

	notificationWorker := worker.NewNotificationWorker(ch, cfg.ResendAPIKey)
	notificationWorker.Start()

	log.Println("Notification service runtime loop idling successfully. Use Ctrl+C to terminate runtime engine.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Notification Service cleanly... Adios!")
}
