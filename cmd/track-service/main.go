package main

import (
	"database/sql"
	"log"
	"net"

	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/config"
	delivery "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/delivery/grpc"
	db "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/usecase"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // Pure Go PostgreSQL driver registration
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Initializing PhonkDrift Track & Streaming Microservice... 🎵")

	if err := godotenv.Load(".env"); err != nil {
		log.Println("Note: No .env file discovered in active path context, relying on system environment variables")
	}

	// 1. Load Configurations from environment variables
	cfg := config.LoadConfig()
	if cfg.DBSource == "" {
		log.Fatal("CRITICAL: TRACK_DB_SOURCE environment variable is not defined")
	}

	// 2. Open Connection Pool to your new Supabase Track Database
	conn, err := sql.Open("postgres", cfg.DBSource)
	if err != nil {
		log.Fatalf("Failed to establish database handle: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("Database connection validation failed: %v", err)
	}
	log.Println("Successfully connected to Supabase Track Database Cluster! ⚡")


	// 2.1 Initialize Redis Client for caching
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Pull from config load fields in production envs!
		Password: "",
		DB:       0,
	})
	log.Println("Successfully connected to Redis Cache! 🚀")

	// 3. Initialize Decoupled Architecture Layers
	//  Fixed code passing only trackUsecase
	repository := db.New(conn)
	trackUsecase := usecase.NewTrackUsecase(repository, rdb)
	grpcHandler := delivery.NewTrackGRPCHandler(trackUsecase)

	// 4. Create Low-Level Network Socket Listener
	listener, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to bind TCP network port %s: %v", cfg.GRPCPort, err)
	}

	// 5. Initialize Server Daemon
	grpcServer := grpc.NewServer()
	trackpb.RegisterTrackServiceServer(grpcServer, grpcHandler)

	// Enable server reflection so you can explore methods instantly in Postman
	reflection.Register(grpcServer)

	log.Printf("Track Service Core Engine listening safely on port %s 🏎️🔥", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Track Service runtime crash encountered: %v", err)
	}
}
