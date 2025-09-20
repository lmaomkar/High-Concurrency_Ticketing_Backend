package config

import (
	"os"
	"strconv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Env                    string
	HTTPPort               int
	PostgresURL            string
	RedisAddr              string
	KafkaBrokers           string
	JWTSigningSecret       string
	SMTPHost               string
	SMTPPort               int
	SMTPUser               string
	SMTPPass               string
	SMTPFrom               string
	AdminEmail             string
	AdminSuperUserPassword string
	MaxWorkerRoutineCount  int
	MaxDBConnections       int
	PaymentURL             string
}

func Load() Config {
	port := getenvInt("HTTP_PORT", 8080)
	smtpPort := getenvInt("SMTP_PORT", 587)
	maxWorkerRoutineCount := getenvInt("MAX_WORKERS", 10)
	maxDBConnections := getenvInt("MAX_DB_CONNECTIONS", 20)
	return Config{
		Env:                    getenv("APP_ENV", "development"),
		HTTPPort:               port,
		PostgresURL:            getenv("POSTGRES_URL", "postgres://evently:evently@localhost:5432/evently?sslmode=disable"),
		RedisAddr:              getenv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:           getenv("KAFKA_BROKERS", "localhost:9092"),
		JWTSigningSecret:       getenv("JWT_SECRET", "dev-secret"),
		SMTPHost:               getenv("SMTP_HOST", "localhost"),
		SMTPPort:               smtpPort,
		SMTPUser:               getenv("SMTP_USER", ""),
		SMTPPass:               getenv("SMTP_PASS", ""),
		SMTPFrom:               getenv("SMTP_FROM", "noreply@evently.local"),
		AdminEmail:             getenv("ADMIN_EMAIL", "admin@evently.com"),
		AdminSuperUserPassword: getenv("ADMIN_PASSWORD", "admin"),
		MaxWorkerRoutineCount:  maxWorkerRoutineCount,
		MaxDBConnections:       maxDBConnections,
		PaymentURL:             getenv("PAYMENT_URL", "http://localhost:8080"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
