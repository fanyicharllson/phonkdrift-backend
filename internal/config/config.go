package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	// Infrastructure Gateways
	AuthServiceAddr  string `mapstructure:"AUTH_SERVICE_ADDR"`
	TrackServiceAddr string `mapstructure:"TRACK_SERVICE_ADDR"`
	ChatServiceAddr  string `mapstructure:"CHAT_SERVICE_ADDR"`

	// Ports
	ApiGatewayHttpPort string `mapstructure:"API_GATEWAY_HTTP_PORT"`
	ApiGatewayGrpcPort string `mapstructure:"API_GATEWAY_GRPC_PORT"`
	AuthGrpcPort       string `mapstructure:"AUTH_GRPC_PORT"`
	TrackGrpcPort      string `mapstructure:"TRACK_GRPC_PORT"`
	ChatGrpcPort       string `mapstructure:"CHAT_GRPC_PORT"`

	// Database Connection Matrix
	AuthDbSource  string `mapstructure:"AUTH_DB_SOURCE"`
	TrackDbSource string `mapstructure:"TRACK_DB_SOURCE"`
	ChatDbSource  string `mapstructure:"CHAT_DB_SOURCE"`
	RedisAddr     string `mapstructure:"REDIS_ADDR"`

	// Third-Party Cloud Integrations
	RabbitMQURL         string `mapstructure:"RABBITMQ_URL"`
	RabbitMQFallbackURL string `mapstructure:"RABBITMQ_FALLBACK_URL"`
	ResendAPIKey        string `mapstructure:"RESEND_API_KEY"`
	JWTSecret           string `mapstructure:"JWT_SECRET"`

	// DigitalOcean Spaces
	DOSpacesKey      string `mapstructure:"DO_SPACES_KEY"`
	DOSpacesSecret   string `mapstructure:"DO_SPACES_SECRET"`
	DOSpacesBucket   string `mapstructure:"DO_SPACES_BUCKET"`
	DOSpacesEndpoint string `mapstructure:"DO_SPACES_ENDPOINT"`
	DOSpacesCDNURL   string `mapstructure:"DO_SPACES_CDN_URL"`

	// YouTube Discovery
	YouTubeAPIKey string `mapstructure:"YOUTUBE_API_KEY"`

	// FCM Push Notifications
	FCMServiceAccountJSON string `mapstructure:"FCM_SERVICE_ACCOUNT_JSON"`

	// Admin
	AdminJWTSecret  string `mapstructure:"ADMIN_JWT_SECRET"`
	AdminAllowedIPs string `mapstructure:"ADMIN_ALLOWED_IPS"`
}

func LoadConfig() (config Config, err error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// explicitly bind every key so Unmarshal picks up
	// system env vars injected by Kubernetes secrets/configmaps
	envKeys := []string{
		"AUTH_SERVICE_ADDR", "TRACK_SERVICE_ADDR", "CHAT_SERVICE_ADDR",
		"API_GATEWAY_HTTP_PORT", "API_GATEWAY_GRPC_PORT",
		"AUTH_GRPC_PORT", "TRACK_GRPC_PORT", "CHAT_GRPC_PORT",
		"AUTH_DB_SOURCE", "TRACK_DB_SOURCE", "CHAT_DB_SOURCE",
		"REDIS_ADDR",
		"RABBITMQ_URL", "RABBITMQ_FALLBACK_URL",
		"RESEND_API_KEY", "JWT_SECRET",
		"DO_SPACES_KEY", "DO_SPACES_SECRET", "DO_SPACES_BUCKET",
		"DO_SPACES_ENDPOINT", "DO_SPACES_CDN_URL",
		"YOUTUBE_API_KEY", "FCM_SERVICE_ACCOUNT_JSON", "ADMIN_JWT_SECRET",
		"ADMIN_ALLOWED_IPS",
	}
	for _, key := range envKeys {
		if err := viper.BindEnv(key); err != nil {
			log.Printf("Warning: Failed to bind env key %s: %v", key, err)
		}
	}

	if err = viper.ReadInConfig(); err != nil {
		log.Printf("Warning: No .env file found (%v). Using system env vars.", err)
		err = nil // not fatal in K8s — secrets come from environment
	}

	err = viper.Unmarshal(&config)
	return
}
