package main

import (
	"database/sql"
	"log"
	"net"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	"google.golang.org/grpc"

	// Standard PostgreSQL driver wrapper
	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting PhonkDrift Auth Microservice...")

	// 1. Load Configurations using the config layer (looking at current directory for .env)
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Critical: Could not load configurations: %v", err)
	}

	// 2. Connect to Supabase Cloud Postgres database
	log.Println("Establishing connection to Supabase Cloud Storage...")
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Critical: Database driver initialization failed: %v", err)
	}
	defer db.Close()

	// Perform an explicit connection handshake test
	err = db.Ping()
	if err != nil {
		log.Fatalf("Critical: Database handshake failed! Double-check your Supabase credentials: %v", err)
	}
	log.Println("Database connection verified successfully. Database is online. ✓")

	// 3. Fire up the TCP Network Listener
	address := ":" + cfg.GRPCPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Critical: Failed to bind TCP listener on port %s: %v", cfg.GRPCPort, err)
	}
	log.Printf("Auth gRPC Service bound to network transport successfully on %s ✓", address)

	// 4. Instantiate and spark the gRPC Base Server
	grpcServer := grpc.NewServer()

	// Note: Later on, we will register our business logic handler routes here:
	// authpb.RegisterAuthServiceServer(grpcServer, ourServiceHandler)

	log.Printf("Auth Microservice engine engine is fully idling on port %s. Listening for binary traffic...", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Critical: Failed to launch gRPC engine runtime: %v", err)
	}
}
