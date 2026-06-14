package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all environment variables for the monorepo services
type Config struct {
	// Infrastructure Gateways
	AuthServiceAddr string `mapstructure:"AUTH_SERVICE_ADDR"`
	TrackServiceAddr string `mapstructure:"TRACK_SERVICE_ADDR"`
	ChatServiceAddr  string `mapstructure:"CHAT_SERVICE_ADDR"` // Comming soon placeholder

	// Ports
	ApiGatewayHttpPort string `mapstructure:"API_GATEWAY_HTTP_PORT"`
    ApiGatewayGrpcPort string `mapstructure:"API_GATEWAY_GRPC_PORT"`
	AuthGrpcPort   string `mapstructure:"AUTH_GRPC_PORT"`
	TrackGrpcPort  string `mapstructure:"TRACK_GRPC_PORT"`
	ChatGrpcPort   string `mapstructure:"CHAT_GRPC_PORT"` // Comming soon placeholder

	// Database Connection Matrix
	AuthDbSource  string `mapstructure:"AUTH_DB_SOURCE"`
	TrackDbSource string `mapstructure:"TRACK_DB_SOURCE"`
	ChatDbSource  string `mapstructure:"CHAT_DB_SOURCE"` // Comming soon placeholder
	RedisAddr     string `mapstructure:"REDIS_ADDR"`

	// Third-Party Cloud Integrations
	RabbitMQURL  string `mapstructure:"RABBITMQ_URL"`
	ResendAPIKey string `mapstructure:"RESEND_API_KEY"`
	JWTSecret    string `mapstructure:"JWT_SECRET"`

	RabbitMQFallbackURL string `mapstructure:"RABBITMQ_FALLBACK_URL"` // Internal K8s / Local Container Address
}

// LoadConfig reads configuration values from a root .env file OR system environment variables
func LoadConfig() (config Config, err error) {
	// Look for a .env file at the project execution directory context
	viper.AddConfigPath(".")
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	// Allow Viper to read directly from Docker Compose / Production system environment variables
	viper.AutomaticEnv()
	// Replace dots with underscores in env keys automatically
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err = viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Local .env file not found (%v). Relying purely on system environment variables.", err)
		err = nil // Do not panic; fall back to active system env arrays
	}

	err = viper.Unmarshal(&config)
	return
}