package ingestdocument

import (
	"context"

	"github.com/ryanpzr/rag-docs-chat/internal/document/domain"
	"github.com/ryanpzr/rag-docs-chat/pkg/chunker"
)

type Usecase interface {
	Ingest(ctx context.Context, in IngestInput) (DocumentResponse, error)
}

// Repository is the part of shared.Repository this feature needs.
type Repository interface {
	CreateWithChunks(ctx context.Context, doc domain.Document, chunks []domain.Chunk) (domain.Document, error)
}

// Embedder is the part of embedder.Embedder this feature needs.
type Embedder interface {
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// Chunker splits a document's text into chunks.
type Chunker interface {
	Split(text string) []chunker.Chunk
}
