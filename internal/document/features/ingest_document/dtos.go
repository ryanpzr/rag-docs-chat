package ingestdocument

import "time"

// IngestInput is an uploaded file as received by the handler.
type IngestInput struct {
	Filename string
	Content  []byte
}

type DocumentResponse struct {
	ID         int64     `json:"id"`
	Filename   string    `json:"filename"`
	SizeBytes  int       `json:"size_bytes"`
	ChunkCount int       `json:"chunk_count"`
	CreatedAt  time.Time `json:"created_at"`
}
