// Package embedder defines how the application turns text into vectors.
// Providers (Ollama today; Voyage or OpenAI later) live in subpackages and
// implement Embedder, so they can be swapped through configuration.
package embedder

import "context"

// Embedder turns text into embedding vectors.
//
// Documents and queries are embedded through separate methods because
// several models (nomic-embed-text, Voyage) embed them differently for
// better retrieval.
type Embedder interface {
	// EmbedDocuments returns one vector per input text, in the same order.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery returns the vector of a search query.
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	// Dimensions is the length of every vector this embedder returns.
	Dimensions() int
}
