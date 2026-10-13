package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port              string
	DatabaseURL       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	Environment       string
	IdempotencyTTL    time.Duration
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
}

func loadDotEnv(filepath string) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
	}
}

func Load() *Config {
	loadDotEnv(".env")

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://ledger_user:ledger_secret@localhost:5432/ledger_db?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := getEnv("REDIS_PASSWORD", "")
	redisDBStr := getEnv("REDIS_DB", "0")
	redisDB, _ := strconv.Atoi(redisDBStr)
	env := getEnv("ENVIRONMENT", "development")

	ttlHoursStr := getEnv("IDEMPOTENCY_TTL_HOURS", "24")
	ttlHours, err := strconv.Atoi(ttlHoursStr)
	if err != nil || ttlHours <= 0 {
		ttlHours = 24
	}

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		RedisAddr:         redisAddr,
		RedisPassword:     redisPass,
		RedisDB:           redisDB,
		Environment:       env,
		IdempotencyTTL:    time.Duration(ttlHours) * time.Hour,
		DBMaxOpenConns:    25,
		DBMaxIdleConns:    10,
		DBConnMaxLifetime: 15 * time.Minute,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
