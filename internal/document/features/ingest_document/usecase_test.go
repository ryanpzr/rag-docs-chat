package ingestdocument_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanpzr/rag-docs-chat/internal/document/domain"
	ingestdocument "github.com/ryanpzr/rag-docs-chat/internal/document/features/ingest_document"
	"github.com/ryanpzr/rag-docs-chat/pkg/apperrors"
	"github.com/ryanpzr/rag-docs-chat/pkg/chunker"
)

const dims = 3

// fakeEmbedder returns a deterministic vector per text and records calls.
type fakeEmbedder struct {
	dims    int
	err     error
	calls   [][]string
	results func(texts []string) [][]float32
}

func (f *fakeEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	f.calls = append(f.calls, texts)
	if f.err != nil {
		return nil, f.err
	}
	if f.results != nil {
		return f.results(texts), nil
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, f.dims)
		v[0] = float32(len(t))
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbedder) Dimensions() int { return f.dims }

// fakeRepo stores what it receives in memory.
type fakeRepo struct {
	err    error
	doc    *domain.Document
	chunks []domain.Chunk
}

func (f *fakeRepo) CreateWithChunks(_ context.Context, doc domain.Document, chunks []domain.Chunk) (domain.Document, error) {
	if f.err != nil {
		return domain.Document{}, f.err
	}
	doc.ID = 42
	doc.CreatedAt = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	f.doc, f.chunks = &doc, chunks
	return doc, nil
}

type deps struct {
	repo *fakeRepo
	emb  *fakeEmbedder
	uc   ingestdocument.Usecase
}

func newDeps(t *testing.T) deps {
	t.Helper()
	ch, err := chunker.New(20, 5)
	require.NoError(t, err)
	d := deps{repo: &fakeRepo{}, emb: &fakeEmbedder{dims: dims}}
	d.uc = ingestdocument.NewUsecase(d.repo, d.emb, ch, 1024)
	return d
}

func assertAppError(t *testing.T, err error, code int) {
	t.Helper()
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

func TestIngest_StoresDocumentWithEmbeddedChunks(t *testing.T) {
	d := newDeps(t)
	content := "Go is a language. pgvector stores vectors. RAG retrieves chunks."

	resp, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "notes.md",
		Content:  []byte(content),
	})

	require.NoError(t, err)
	require.NotNil(t, d.repo.doc)
	assert.Equal(t, int64(42), resp.ID)
	assert.Equal(t, "notes.md", resp.Filename)
	assert.Equal(t, len(content), resp.SizeBytes)
	assert.Equal(t, len(d.repo.chunks), resp.ChunkCount)
	assert.Greater(t, resp.ChunkCount, 1)

	assert.Equal(t, content, d.repo.doc.Content)
	require.Len(t, d.emb.calls, 1, "all chunks are embedded in one call")
	for i, c := range d.repo.chunks {
		assert.Equal(t, i, c.Index)
		assert.Equal(t, d.emb.calls[0][i], c.Content)
		assert.Len(t, c.Embedding, dims)
		assert.Equal(t, float32(len(c.Content)), c.Embedding[0], "chunk keeps its own embedding")
	}
}

func TestIngest_AcceptsTxtAndUppercaseExtension(t *testing.T) {
	for _, name := range []string{"a.txt", "README.MD", "dir/sub/file.Txt"} {
		t.Run(name, func(t *testing.T) {
			d := newDeps(t)
			resp, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
				Filename: name, Content: []byte("some text"),
			})
			require.NoError(t, err)
			assert.NotContains(t, resp.Filename, "/", "path components are stripped")
		})
	}
}

func TestIngest_StripsUTF8BOM(t *testing.T) {
	d := newDeps(t)
	_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "bom.txt", Content: []byte("\xef\xbb\xbfhello"),
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", d.repo.doc.Content)
}

func TestIngest_ValidationErrors(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		content  []byte
		code     int
	}{
		{"missing filename", "", []byte("text"), http.StatusBadRequest},
		{"unsupported extension", "report.pdf", []byte("text"), http.StatusUnsupportedMediaType},
		{"no extension", "README", []byte("text"), http.StatusUnsupportedMediaType},
		{"too large", "big.txt", []byte(strings.Repeat("a", 1025)), http.StatusRequestEntityTooLarge},
		{"invalid UTF-8", "bin.txt", []byte{0xff, 0xfe, 0x00, 0x01}, http.StatusBadRequest},
		{"empty", "empty.txt", []byte(""), http.StatusBadRequest},
		{"whitespace only", "blank.md", []byte(" \n\t\n "), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDeps(t)
			_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
				Filename: tc.filename, Content: tc.content,
			})
			assertAppError(t, err, tc.code)
			assert.Empty(t, d.emb.calls, "nothing is embedded for invalid input")
			assert.Nil(t, d.repo.doc, "nothing is stored for invalid input")
		})
	}
}

func TestIngest_EmbedderFailureIsBadGateway(t *testing.T) {
	d := newDeps(t)
	d.emb.err = errors.New("connection refused")

	_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "a.txt", Content: []byte("hello world"),
	})

	assertAppError(t, err, http.StatusBadGateway)
	assert.ErrorContains(t, err, "connection refused", "cause is kept for logging")
	assert.Nil(t, d.repo.doc)
}

func TestIngest_WrongEmbeddingCountIsInternal(t *testing.T) {
	d := newDeps(t)
	d.emb.results = func([]string) [][]float32 { return nil }

	_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "a.txt", Content: []byte("hello world"),
	})

	assertAppError(t, err, http.StatusInternalServerError)
	assert.Nil(t, d.repo.doc)
}

func TestIngest_WrongEmbeddingDimensionIsInternal(t *testing.T) {
	d := newDeps(t)
	d.emb.results = func(texts []string) [][]float32 {
		out := make([][]float32, len(texts))
		for i := range out {
			out[i] = make([]float32, dims+1)
		}
		return out
	}

	_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "a.txt", Content: []byte("hello world"),
	})

	assertAppError(t, err, http.StatusInternalServerError)
	assert.Nil(t, d.repo.doc)
}

func TestIngest_RepositoryFailureIsInternal(t *testing.T) {
	d := newDeps(t)
	d.repo.err = errors.New(`relation "chunks" does not exist`)

	_, err := d.uc.Ingest(context.Background(), ingestdocument.IngestInput{
		Filename: "a.txt", Content: []byte("hello world"),
	})

	assertAppError(t, err, http.StatusInternalServerError)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.NotContains(t, appErr.Message, "relation", "database details stay out of the client message")
}
