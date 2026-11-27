// Package app wires repositories, usecases and handlers into an HTTP router.
package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"

	"github.com/ryanpzr/rag-docs-chat/internal/config"
	ingestdocument "github.com/ryanpzr/rag-docs-chat/internal/document/features/ingest_document"
	listdocuments "github.com/ryanpzr/rag-docs-chat/internal/document/features/list_documents"
	"github.com/ryanpzr/rag-docs-chat/internal/document/shared"
	"github.com/ryanpzr/rag-docs-chat/pkg/apperrors"
	"github.com/ryanpzr/rag-docs-chat/pkg/chunker"
	"github.com/ryanpzr/rag-docs-chat/pkg/embedder"
	"github.com/ryanpzr/rag-docs-chat/pkg/embedder/ollama"
)

// OpenPool connects to Postgres and registers the pgvector types on every
// connection. The vector extension must already exist (run the migrations).
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}

// NewEmbedder builds the embedder selected by EMBEDDINGS_PROVIDER.
func NewEmbedder(cfg config.Config) (embedder.Embedder, error) {
	switch cfg.EmbeddingsProvider {
	case "ollama":
		return ollama.New(ollama.Config{
			BaseURL:    cfg.OllamaURL,
			Model:      cfg.EmbeddingsModel,
			Dimensions: cfg.EmbeddingsDimensions,
			BatchSize:  cfg.EmbeddingsBatchSize,
		}), nil
	default:
		return nil, fmt.Errorf("unknown EMBEDDINGS_PROVIDER %q (supported: ollama)", cfg.EmbeddingsProvider)
	}
}

// NewRouter checks that the database schema matches the embedder and
// returns the HTTP routes.
func NewRouter(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, emb embedder.Embedder) (http.Handler, error) {
	ch, err := chunker.New(cfg.ChunkSize, cfg.ChunkOverlap)
	if err != nil {
		return nil, err
	}

	repo := shared.NewRepository(pool)
	dims, err := repo.EmbeddingDimensions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w (did the migrations run?)", err)
	}
	if dims != emb.Dimensions() {
		return nil, fmt.Errorf("chunks.embedding is vector(%d) but EMBEDDINGS_DIMENSIONS is %d", dims, emb.Dimensions())
	}

	ingest := ingestdocument.NewHandler(
		ingestdocument.NewUsecase(repo, emb, ch, cfg.MaxUploadBytes), cfg.MaxUploadBytes)
	list := listdocuments.NewHandler(listdocuments.NewUsecase(repo))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		apperrors.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /documents", ingest.Ingest)
	mux.HandleFunc("GET /documents", list.List)
	return mux, nil
}
