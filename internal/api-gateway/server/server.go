package server

import (
	"log"
	"net"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/config"
	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/delivery/http"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GatewayServer struct {
	authpb.UnimplementedAuthServiceServer
	Cfg        *config.Config
	AuthClient authpb.AuthServiceClient
}

func Run(cfg *config.Config) error {
	// Connect to internal Auth microservice
	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth Service: %v", err)
	}
	defer authConn.Close()

	server := &GatewayServer{
		Cfg:        cfg,
		AuthClient: authpb.NewAuthServiceClient(authConn),
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

	// Register the auth service on the gateway port! 
	// When mobile hits this port, the gateway wraps the call and sends it down to the client stub.
	authpb.RegisterAuthServiceServer(grpcServer, s)

	log.Printf("gRPC Mobile Proxy listening safely on port %s 📱", s.Cfg.GRPCPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve mobile gRPC traffic: %v", err)
	}
}

