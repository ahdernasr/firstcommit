// Package config centralises all environment / flag configuration for the API.
// It should be imported only by `cmd/server` (and test code). Business‑logic
// layers receive an already‑built Config instance via dependency‑injection.
package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every runtime option the server needs.
// Keep it flat and simple—prefer primitive types over embedding structs.
type Config struct {
	// Network
	Port string

	// Data stores
	MongoURI          string
	FederatedMongoURI string
	DBName            string

	// External services
	GitHubToken string

	// Server tuning
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// ProjectID and Location
	ProjectID string
	Location  string
}

// Load parses the environment (and an optional .env file) into Config.
// It panics on missing critical variables so mis‑configurations fail fast.
func Load() Config {
	// godotenv.Load() is a no‑op if .env doesn't exist—safe in production.
	_ = godotenv.Load()

	return Config{
		Port:              must("PORT"),
		MongoURI:          must("MONGODB_URI"),
		FederatedMongoURI: must("FEDERATED_MONGODB_URI"),
		DBName:            getEnv("MONGODB_DB", "ai_action"),
		GitHubToken:       must("GITHUB_TOKEN"),
		ProjectID:         must("GCP_PROJECT_ID"),
		Location:          must("GCP_LOCATION"),
		ReadTimeout:       getDuration("READ_TIMEOUT_SEC", 5),
		WriteTimeout:      getDuration("WRITE_TIMEOUT_SEC", 10),
	}
}

// must fetches a required env var or terminates the program.
func must(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("env var %s is required", key)
	}
	return val
}

// getEnv returns env[key] if set, otherwise defaultVal.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getDuration reads an integer (seconds) from env, falling back to defaultSec.
func getDuration(key string, defaultSec int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			return time.Duration(sec) * time.Second
		}
		log.Printf("invalid %s=%q; using default %ds", key, v, defaultSec)
	}
	return time.Duration(defaultSec) * time.Second
}
