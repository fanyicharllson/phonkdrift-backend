package server

import (
	"database/sql"
	"log"
	"net"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/delivery/http"
	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	discovery "github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	chatpb "github.com/fanyicharllson/phonkdrift-backend/pb/chat"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GatewayServer struct {
	authpb.UnimplementedAuthServiceServer
	trackpb.UnimplementedTrackServiceServer
	chatpb.UnimplementedChatServiceServer
	Cfg         *config.Config
	AuthClient  authpb.AuthServiceClient
	TrackClient trackpb.TrackServiceClient
	ChatClient  chatpb.ChatServiceClient
	trackProxy  *TrackProxy
	chatProxy   *ChatProxy
}

func Run(cfg *config.Config) error {
	// Connect to internal Auth microservice
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth Service: %v", err)
	}

	// Connect to internal Track microservice
	trackConn, err := grpc.NewClient(cfg.TrackServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to establish raw gRPC handle to Track Service: %v", err)
	}

	trackProxy, err := NewTrackProxy(cfg.TrackServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to internal Track Microservice proxy context: %v", err)
	}

	// Connect to internal Chat microservice
	chatConn, err := grpc.NewClient(cfg.ChatServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Chat Service: %v", err)
	}

	chatProxy, err := NewChatProxy(cfg.ChatServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to internal Chat Microservice proxy context: %v", err)
	}

	server := &GatewayServer{
		Cfg:         cfg,
		AuthClient:  authpb.NewAuthServiceClient(authConn),
		TrackClient: trackpb.NewTrackServiceClient(trackConn), // Instantiate track service connection client layer
		ChatClient:  chatpb.NewChatServiceClient(chatConn),
		trackProxy:  trackProxy,
		chatProxy:   chatProxy,
	}

	// Connect to S3/DO Spaces for manual track seeding
	var uploader *discovery.Uploader
	if cfg.DOSpacesKey != "" {
		uploader, err = discovery.NewUploader(
			cfg.DOSpacesKey,
			cfg.DOSpacesSecret,
			cfg.DOSpacesEndpoint,
			cfg.DOSpacesBucket,
			cfg.DOSpacesCDNURL,
			cfg.YtDlpCookiesPath,
		)
		if err != nil {
			log.Printf("Warning: Failed to initialize S3 uploader in Gateway: %v", err)
		}
	}

	// Optionally connect to Track DB for manual discovery trigger
	var scheduler *discovery.Scheduler
	if cfg.TrackDbSource != "" && uploader != nil {
		dbConn, err := sql.Open("postgres", cfg.TrackDbSource)
		if err == nil {
			if err := dbConn.Ping(); err == nil {
				worker := discovery.NewWorker(cfg.YouTubeAPIKey)
				trackRepo := trackdb.New(dbConn)
				scheduler = discovery.NewScheduler(worker, uploader, trackRepo, 12*time.Hour)
			} else {
				log.Printf("Warning: Failed to ping track DB for gateway scheduler: %v", err)
			}
		} else {
			log.Printf("Warning: Failed to connect to track DB for gateway: %v", err)
		}
	}

	// 1. Run HTTP REST in background for Web/Postman - passing BOTH client targets seamlessly!
	go http.StartHTTPServer(server.Cfg, server.AuthClient, server.TrackClient, uploader, scheduler)

	// 2. Run Mobile gRPC listener (Blocks and keeps main alive)
	server.StartMobileGRPCListener()
	return nil
}

func (s *GatewayServer) StartMobileGRPCListener() {
	address := ":" + s.Cfg.ApiGatewayGrpcPort
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen for mobile gRPC on port %s: %v", s.Cfg.ApiGatewayGrpcPort, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(s.authUnaryInterceptor()),
		grpc.StreamInterceptor(s.authStreamInterceptor()),
	)

	// Register all backend services on the multiplexed gateway port!
	authpb.RegisterAuthServiceServer(grpcServer, s)
	trackpb.RegisterTrackServiceServer(grpcServer, s)
	chatpb.RegisterChatServiceServer(grpcServer, s)

	log.Printf("gRPC Mobile Proxy listening safely on port %s 📱🚀", s.Cfg.ApiGatewayGrpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve mobile gRPC traffic: %v", err)
	}
}
