package config

import "os"

type Config struct {
	GRPCPort string
	DBSource string
}

func LoadConfig() *Config {
	return &Config{
		GRPCPort: getEnv("TRACK_SERVICE_PORT", ":50052"),
		DBSource: getEnv("TRACK_DB_SOURCE", ""), // Loaded from your track service .env
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}