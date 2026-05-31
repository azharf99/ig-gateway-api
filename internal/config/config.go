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
	EncryptionKey  string
	AllowedOrigins string
	GinMode        string
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
		AppURL:         getEnv("APP_URL", "http://localhost:8090"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", ""),
		DBName:         getEnv("DB_NAME", "ig_gateway"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTExpiryHours: expiryHours,
		IGClientID:     getEnv("INSTAGRAM_CLIENT_ID", ""),
		IGClientSecret: getEnv("INSTAGRAM_CLIENT_SECRET", ""),
		IGRedirectURI:  getEnv("INSTAGRAM_REDIRECT_URI", "http://localhost:8090/auth/instagram/callback"),
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "https://ig.azharfa.cloud,http://localhost:5173"),
		GinMode:        getEnv("GIN_MODE", "release"),
	}

	// Hard strict validation
	if AppConfig.JWTSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if AppConfig.IGClientID == "" || AppConfig.IGClientSecret == "" {
		log.Fatal("INSTAGRAM_CLIENT_ID and INSTAGRAM_CLIENT_SECRET environment variables are required")
	}
	if AppConfig.EncryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
	}
	if len(AppConfig.EncryptionKey) < 32 {
		log.Fatal("ENCRYPTION_KEY must be at least 32 characters long")
	}

	log.Println("Configuration loaded successfully")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

