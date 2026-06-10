package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fanyicharllson/phonkdrift-backend/internal/notification-service/workers"
	"github.com/rabbitmq/amqp091-go"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("Starting Centralized PhonkDrift Notification Microservice Engine...")

	// Load local .env directly since this is a pure background event consumer worker engine
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No explicit .env file discovered, using system env variables")
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	resendKey := os.Getenv("RESEND_API_KEY")

	if rabbitURL == "" || resendKey == "" {
		log.Fatal("Critical: Missing environment strings. Verify RABBITMQ_URL and RESEND_API_KEY match context.")
	}

	// Connect to RabbitMQ Broker
	conn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("Critical: Failed to bind broker session connection: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Critical: Failed to generate functional session channel: %v", err)
	}
	defer ch.Close()

	// Launch background processing thread
	notificationWorker := worker.NewNotificationWorker(ch, resendKey)
	notificationWorker.Start()

	// Keep main thread alive safely until manual system interruption 
	log.Println("Notification service runtime loop idling successfully. Use Ctrl+C to terminate runtime engine.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Notification Service engine cleanly... Adios!")
}