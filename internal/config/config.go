// Package config reads the application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	MaxUploadBytes int64

	ChunkSize    int
	ChunkOverlap int

	EmbeddingsProvider   string
	EmbeddingsModel      string
	EmbeddingsDimensions int
	EmbeddingsBatchSize  int
	OllamaURL            string
}

// Load reads the configuration from the environment, applying defaults that
// match docker-compose.yml.
func Load() (Config, error) {
	var errs []error
	intVar := func(key string, def int) int {
		v, err := getInt(key, def)
		errs = append(errs, err)
		return v
	}

	cfg := Config{
		HTTPAddr:       getString("HTTP_ADDR", ":8080"),
		DatabaseURL:    getString("DATABASE_URL", "postgres://rag:rag@localhost:5433/rag?sslmode=disable"),
		MaxUploadBytes: int64(intVar("MAX_UPLOAD_BYTES", 5<<20)),

		ChunkSize:    intVar("CHUNK_SIZE", 1000),
		ChunkOverlap: intVar("CHUNK_OVERLAP", 200),

		EmbeddingsProvider:   getString("EMBEDDINGS_PROVIDER", "ollama"),
		EmbeddingsModel:      getString("EMBEDDINGS_MODEL", "nomic-embed-text"),
		EmbeddingsDimensions: intVar("EMBEDDINGS_DIMENSIONS", 768),
		EmbeddingsBatchSize:  intVar("EMBEDDINGS_BATCH_SIZE", 32),
		OllamaURL:            getString("OLLAMA_URL", "http://localhost:11434"),
	}
	if cfg.MaxUploadBytes <= 0 {
		errs = append(errs, errors.New("MAX_UPLOAD_BYTES must be positive"))
	}
	if cfg.EmbeddingsDimensions <= 0 {
		errs = append(errs, errors.New("EMBEDDINGS_DIMENSIONS must be positive"))
	}
	return cfg, errors.Join(errs...)
}

func getString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
}
