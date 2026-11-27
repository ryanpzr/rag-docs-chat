package domain

import "time"

// Document is an ingested file.
type Document struct {
	ID        int64
	Filename  string
	Content   string
	SizeBytes int
	CreatedAt time.Time
}

// Chunk is a piece of a document together with its embedding.
type Chunk struct {
	Index     int
	Content   string
	Embedding []float32
}

// DocumentSummary is a document as shown in listings, without its content.
type DocumentSummary struct {
	ID         int64
	Filename   string
	SizeBytes  int
	ChunkCount int
	CreatedAt  time.Time
}
