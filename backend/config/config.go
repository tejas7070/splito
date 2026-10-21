package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port       string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBName     string
	DBTLS      bool
	DBDSN      string
	RedisAddr  string
	RedisPass  string
	JWTSecret  string
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
	GoogleClientID string
	AppEnv         string
	AllowedOrigins string
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func LoadConfig() *Config {
	loadEnvFile(".env")
	loadEnvFile("backend/.env")

	return &Config{
		Port:       getEnv("PORT", "8080"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", "splito_db"),
		DBTLS:      getEnv("DB_TLS", "false") == "true",
		DBDSN:      getEnv("DATABASE_URL", getEnv("DB_DSN", "")),
		RedisAddr:  getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPass:  getEnv("REDIS_PASSWORD", ""),
		JWTSecret:  getEnv("JWT_SECRET", "super-secret-splito-jwt-key"),
		SMTPHost:   getEnv("SMTP_HOST", ""),
		SMTPPort:   getEnv("SMTP_PORT", "587"),
		SMTPUser:   getEnv("SMTP_USER", ""),
		SMTPPass:       getEnv("SMTP_PASS", ""),
		SMTPFrom:       getEnv("SMTP_FROM", ""),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		AppEnv:         getEnv("APP_ENV", "development"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}
