package config

import "os"

type Config struct {
	HTTPPort        string
	GRPCPort        string
	AuthServiceAddr string
	TrackServiceAddr string 
	ChatServiceAddr  string 
}

func LoadConfig() *Config {
	return &Config{
		HTTPPort:        getEnv("HTTP_GATEWAY_PORT", ":8080"),
		GRPCPort:        getEnv("GRPC_GATEWAY_PORT", ":50050"),
		AuthServiceAddr: getEnv("AUTH_SERVICE_ADDR", "localhost:50051"),
		TrackServiceAddr: getEnv("TRACK_SERVICE_ADDR", "localhost:50052"),
		ChatServiceAddr:  getEnv("CHAT_SERVICE_ADDR", "localhost:50053"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}