package shared

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/ryanpzr/rag-docs-chat/internal/document/domain"
)

// Repository only talks to the database. It wraps errors with context and
// applies no business rules; validation and mapping errors to responses are
// the usecases' job.
type Repository interface {
	// CreateWithChunks stores the document and its chunks in one
	// transaction and returns the document with its ID and CreatedAt set.
	CreateWithChunks(ctx context.Context, doc domain.Document, chunks []domain.Chunk) (domain.Document, error)
	// ListSummaries returns every document with its chunk count, newest first.
	ListSummaries(ctx context.Context) ([]domain.DocumentSummary, error)
	// EmbeddingDimensions returns the dimension of the chunks.embedding column.
	EmbeddingDimensions(ctx context.Context) (int, error)
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) CreateWithChunks(ctx context.Context, doc domain.Document, chunks []domain.Chunk) (domain.Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Document{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	err = tx.QueryRow(ctx, insertDocument, doc.Filename, doc.Content, doc.SizeBytes).
		Scan(&doc.ID, &doc.CreatedAt)
	if err != nil {
		return domain.Document{}, fmt.Errorf("insert document: %w", err)
	}

	rows := make([][]any, len(chunks))
	for i, c := range chunks {
		rows[i] = []any{doc.ID, c.Index, c.Content, pgvector.NewVector(c.Embedding)}
	}
	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"chunks"},
		[]string{"document_id", "chunk_index", "content", "embedding"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return domain.Document{}, fmt.Errorf("insert chunks: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Document{}, fmt.Errorf("commit: %w", err)
	}
	return doc, nil
}

func (r *repository) ListSummaries(ctx context.Context) ([]domain.DocumentSummary, error) {
	rows, err := r.pool.Query(ctx, listSummaries)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	docs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DocumentSummary, error) {
		var d domain.DocumentSummary
		err := row.Scan(&d.ID, &d.Filename, &d.SizeBytes, &d.ChunkCount, &d.CreatedAt)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan documents: %w", err)
	}
	return docs, nil
}

func (r *repository) EmbeddingDimensions(ctx context.Context) (int, error) {
	var dims int
	if err := r.pool.QueryRow(ctx, embeddingDimensions).Scan(&dims); err != nil {
		return 0, fmt.Errorf("read embedding dimensions: %w", err)
	}
	return dims, nil
}
