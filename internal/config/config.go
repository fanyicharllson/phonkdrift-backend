package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config holds all environmental variables for the microservices
type Config struct {
	GRPCPort    string `mapstructure:"GRPC_PORT"`
	DatabaseURL string `mapstructure:"DATABASE_URL"`
	JWTSecret   string `mapstructure:"JWT_SECRET"`
	RabbitMQURL string `mapstructure:"RABBITMQ_URL"`
	ResendAPIKey string `mapstructure:"RESEND_API_KEY"`
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig(path string) (config Config, err error) {
	// 1. Force Viper to look for exactly a ".env" file in the path provided
	viper.SetConfigFile(path + "/.env")

	// 2. Read environment variables from the OS if present (for Docker Compose/Production)
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		log.Printf("Warning: Hey Charllson! Could not read config file explicitly: %v. Relying on system env.", err)
		err = nil 
	}

	err = viper.Unmarshal(&config)
	return
}