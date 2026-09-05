package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	AppName string
	AppEnv  string
	AppPort string
	AppURL  string

	DBDriver   string // mysql | postgres
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	JWTSecret       string
	AccessTTLMin    int
	RefreshTTLHours int

	StripeSecretKey     string
	StripeWebhookSecret string
	SSLCommerzStoreID   string
	SSLCommerzStorePass string
	SSLCommerzSandbox   bool

	CORSOrigins []string

	UploadDir   string
	MaxUploadMB int64
}

// Load reads .env (if present) and builds the Config.
func Load() *Config {
	_ = godotenv.Load() // .env is optional; real env vars win

	cfg := &Config{
		AppName: getEnv("APP_NAME", "GocoreCMS"),
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnv("APP_PORT", "8080"),
		AppURL:  getEnv("APP_URL", "http://localhost:8080"),

		DBDriver:   strings.ToLower(getEnv("DB_DRIVER", "mysql")),
		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", "Gocore_cms"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:       getEnv("JWT_SECRET", "insecure-dev-secret"),
		AccessTTLMin:    getEnvInt("JWT_ACCESS_TTL_MIN", 60),
		RefreshTTLHours: getEnvInt("JWT_REFRESH_TTL_HOURS", 168),

		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		SSLCommerzStoreID:   getEnv("SSLCOMMERZ_STORE_ID", ""),
		SSLCommerzStorePass: getEnv("SSLCOMMERZ_STORE_PASSWORD", ""),
		SSLCommerzSandbox:   getEnv("SSLCOMMERZ_SANDBOX", "true") == "true",

		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),

		UploadDir:   getEnv("UPLOAD_DIR", "./storage/uploads"),
		MaxUploadMB: int64(getEnvInt("MAX_UPLOAD_MB", 20)),
	}
	return cfg
}

// DSN builds the database connection string for the selected driver.
func (c *Config) DSN() string {
	if c.DBDriver == "postgres" {
		return fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
			c.DBHost, c.DBUser, c.DBPassword, c.DBName, c.DBPort, c.DBSSLMode,
		)
	}
	// mysql (default)
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
