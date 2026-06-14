package main

import (
	"database/sql"
	"log"
	"net"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	delivery "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/delivery/grpc"
	db "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/usecase"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	_ "github.com/lib/pq" // Pure Go PostgreSQL driver registration
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Initializing PhonkDrift Track & Streaming Microservice... 🎵")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Critical: Could not load configuration values: %v", err)
	}

	if cfg.TrackDbSource == "" {
		log.Fatal("CRITICAL: TRACK_DB_SOURCE environment variable is not defined")
	}

	// 2. Open Connection Pool to Supabase Track Database
	conn, err := sql.Open("postgres", cfg.TrackDbSource)
	if err != nil {
		log.Fatalf("Failed to establish database handle: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("Database connection validation failed: %v", err)
	}
	log.Println("Successfully connected to Supabase Track Database Cluster! ⚡")

	// 2.1 Initialize Redis Client using centralized configurations
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	log.Printf("Successfully connected to Redis Cache at %s! 🚀\n", cfg.RedisAddr)

	repositoryObj := db.New(conn)
	trackUsecase := usecase.NewTrackUsecase(repositoryObj, rdb)
	grpcHandler := delivery.NewTrackGRPCHandler(trackUsecase)

	address := ":" + cfg.TrackGrpcPort
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to bind TCP network port %s: %v", cfg.TrackGrpcPort, err)
	}

	grpcServer := grpc.NewServer()
	trackpb.RegisterTrackServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	log.Printf("Track Service Core Engine listening safely on port %s 🏎️🔥", cfg.TrackGrpcPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Track Service runtime crash encountered: %v", err)
	}
}
