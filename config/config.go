package config

import (
	"log"
	"os"
)

type Config struct {
	ServerPort   string
	ServerHost   string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	ImageBaseURL string
}

var AppConfig Config

func LoadConfig() {
	AppConfig = Config{
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		ServerHost:   getEnv("SERVER_HOST", "localhost"),
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "3306"),
		DBUser:       getEnv("DB_USER", "root"),
		DBPassword:   getEnv("DB_PASSWORD", ""),
		DBName:       getEnv("DB_NAME", "backer"),
		ImageBaseURL: getEnv("IMAGE_BASE_URL", "http://localhost:8080"),
	}
	log.Println("Config loaded successfully")
	log.Printf("Image Base URL: %s\n", AppConfig.ImageBaseURL)
}

// getEnv reads environment variable, returns default if not found
func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
