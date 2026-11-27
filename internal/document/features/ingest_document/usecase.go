package ingestdocument

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ryanpzr/rag-docs-chat/internal/document/domain"
	"github.com/ryanpzr/rag-docs-chat/pkg/apperrors"
)

// allowedExtensions are the file types that can be ingested. PDF needs text
// extraction and comes later.
var allowedExtensions = map[string]bool{".txt": true, ".md": true}

// utf8BOM is the byte order mark some editors (e.g. Notepad) put at the
// start of UTF-8 files.
const utf8BOM = "\xef\xbb\xbf"

type usecase struct {
	repo           Repository
	embedder       Embedder
	chunker        Chunker
	maxUploadBytes int64
}

func NewUsecase(repo Repository, emb Embedder, ch Chunker, maxUploadBytes int64) Usecase {
	return &usecase{repo: repo, embedder: emb, chunker: ch, maxUploadBytes: maxUploadBytes}
}

func (u *usecase) Ingest(ctx context.Context, in IngestInput) (DocumentResponse, error) {
	filename := filepath.Base(strings.TrimSpace(in.Filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return DocumentResponse{}, apperrors.NewBadRequest("filename is required")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExtensions[ext] {
		return DocumentResponse{}, apperrors.NewUnsupportedMediaType("only .txt and .md files are supported")
	}
	if int64(len(in.Content)) > u.maxUploadBytes {
		return DocumentResponse{}, apperrors.NewPayloadTooLarge(
			fmt.Sprintf("file is larger than the %d bytes limit", u.maxUploadBytes))
	}
	if !utf8.Valid(in.Content) {
		return DocumentResponse{}, apperrors.NewBadRequest("file must be UTF-8 text")
	}

	text := strings.TrimPrefix(string(in.Content), utf8BOM)
	if strings.TrimSpace(text) == "" {
		return DocumentResponse{}, apperrors.NewBadRequest("file is empty")
	}

	pieces := u.chunker.Split(text)
	if len(pieces) == 0 {
		return DocumentResponse{}, apperrors.NewBadRequest("file has no text to index")
	}

	texts := make([]string, len(pieces))
	for i, p := range pieces {
		texts[i] = p.Content
	}
	vectors, err := u.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return DocumentResponse{}, apperrors.NewBadGateway("failed to generate embeddings", err)
	}
	if len(vectors) != len(pieces) {
		return DocumentResponse{}, apperrors.NewInternal("failed to generate embeddings",
			fmt.Errorf("got %d embeddings for %d chunks", len(vectors), len(pieces)))
	}

	chunks := make([]domain.Chunk, len(pieces))
	for i, p := range pieces {
		if got, want := len(vectors[i]), u.embedder.Dimensions(); got != want {
			return DocumentResponse{}, apperrors.NewInternal("failed to generate embeddings",
				fmt.Errorf("chunk %d: embedding has %d dimensions, expected %d", i, got, want))
		}
		chunks[i] = domain.Chunk{Index: p.Index, Content: p.Content, Embedding: vectors[i]}
	}

	doc, err := u.repo.CreateWithChunks(ctx, domain.Document{
		Filename:  filename,
		Content:   text,
		SizeBytes: len(in.Content),
	}, chunks)
	if err != nil {
		return DocumentResponse{}, apperrors.NewInternal("failed to store document", err)
	}

	return DocumentResponse{
		ID:         doc.ID,
		Filename:   doc.Filename,
		SizeBytes:  doc.SizeBytes,
		ChunkCount: len(chunks),
		CreatedAt:  doc.CreatedAt,
	}, nil
}
