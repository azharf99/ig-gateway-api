package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	AppURL         string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBSSLMode      string
	RedisHost      string
	RedisPort      string
	RedisPassword  string
	JWTSecret      string
	JWTExpiryHours int
	IGClientID     string
	IGClientSecret string
	IGRedirectURI  string
}

var AppConfig *Config

func LoadConfig() {
	// Try loading .env file, but ignore error if it doesn't exist (e.g. in production docker)
	_ = godotenv.Load()

	expiryHours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "72"))
	if err != nil {
		expiryHours = 72
	}

	AppConfig = &Config{
		Port:           getEnv("PORT", "8080"),
		AppURL:         getEnv("APP_URL", "http://localhost:8080"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "ig_gateway"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		JWTSecret:      getEnv("JWT_SECRET", "super-secret-key-change-in-prod"),
		JWTExpiryHours: expiryHours,
		IGClientID:     getEnv("INSTAGRAM_CLIENT_ID", ""),
		IGClientSecret: getEnv("INSTAGRAM_CLIENT_SECRET", ""),
		IGRedirectURI:  getEnv("INSTAGRAM_REDIRECT_URI", "http://localhost:8080/api/v1/auth/instagram/callback"),
	}

	log.Println("Configuration loaded successfully")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
