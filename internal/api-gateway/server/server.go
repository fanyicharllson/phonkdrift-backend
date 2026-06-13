package server

import (
	"log"
	"net"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/delivery/http"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GatewayServer struct {
	authpb.UnimplementedAuthServiceServer
	trackpb.UnimplementedTrackServiceServer
	Cfg        *config.Config
	AuthClient authpb.AuthServiceClient
	trackProxy *TrackProxy
}

func Run(cfg *config.Config) error {
	// Connect to internal Auth microservice
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth Service: %v", err)
	}
	

	// Connect to internal Track microservice
	trackProxy, err := NewTrackProxy(cfg.TrackServiceAddr) 
	if err != nil {
		log.Fatalf("Failed to connect to internal Track Microservice: %v", err)
	}

	server := &GatewayServer{
		Cfg:        cfg,
		AuthClient: authpb.NewAuthServiceClient(authConn),
		trackProxy: trackProxy,
	}

	// 1. Run HTTP REST in background for Web/Postman
	go http.StartHTTPServer(server.Cfg, server.AuthClient)

	// 2. Run Mobile gRPC listener (Blocks and keeps main alive)
	server.StartMobileGRPCListener()
	return nil
}

func (s *GatewayServer) StartMobileGRPCListener() {
	lis, err := net.Listen("tcp", s.Cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen for mobile gRPC on port %s: %v", s.Cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer()

	// Register both backend services on the multiplexed gateway port! 
	authpb.RegisterAuthServiceServer(grpcServer, s)
	trackpb.RegisterTrackServiceServer(grpcServer, s)

	log.Printf("gRPC Mobile Proxy listening safely on port %s 📱🚀", s.Cfg.GRPCPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve mobile gRPC traffic: %v", err)
	}
}