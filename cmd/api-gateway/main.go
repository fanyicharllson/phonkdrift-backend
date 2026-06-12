package main

import (
	"log"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/server" // Rename your internal main to server
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Booting up PhonkDrift API Gateway Cluster Entrypoint... 🚀")
	if err := server.Run(cfg); err != nil {
		log.Fatalf("API Gateway critical system crash: %v", err)
	}
}			