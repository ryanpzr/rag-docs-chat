package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanpzr/rag-docs-chat/pkg/embedder/ollama"
)

type request struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// newServer returns a fake Ollama that embeds each input as [len(input)].
func newServer(t *testing.T, got *[]request) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		var req request
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		*got = append(*got, req)
		out := make([][]float32, len(req.Input))
		for i, in := range req.Input {
			out[i] = []float32{float32(len(in))}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEmbedDocuments_BatchesAndAddsNomicPrefix(t *testing.T) {
	var reqs []request
	srv := newServer(t, &reqs)
	c := ollama.New(ollama.Config{BaseURL: srv.URL + "/", Model: "nomic-embed-text", Dimensions: 1, BatchSize: 2})

	vecs, err := c.EmbedDocuments(context.Background(), []string{"a", "bb", "ccc"})

	require.NoError(t, err)
	require.Len(t, reqs, 2)
	assert.Equal(t, []string{"search_document: a", "search_document: bb"}, reqs[0].Input)
	assert.Equal(t, []string{"search_document: ccc"}, reqs[1].Input)
	assert.Equal(t, "nomic-embed-text", reqs[0].Model)
	prefix := float32(len("search_document: "))
	assert.Equal(t, [][]float32{{prefix + 1}, {prefix + 2}, {prefix + 3}}, vecs)
}

func TestEmbedQuery_UsesQueryPrefix(t *testing.T) {
	var reqs []request
	srv := newServer(t, &reqs)
	c := ollama.New(ollama.Config{BaseURL: srv.URL, Model: "nomic-embed-text:latest", Dimensions: 1})

	_, err := c.EmbedQuery(context.Background(), "what is rag?")

	require.NoError(t, err)
	assert.Equal(t, []string{"search_query: what is rag?"}, reqs[0].Input)
}

func TestEmbed_OtherModelsGetNoPrefix(t *testing.T) {
	var reqs []request
	srv := newServer(t, &reqs)
	c := ollama.New(ollama.Config{BaseURL: srv.URL, Model: "mxbai-embed-large", Dimensions: 1})

	_, err := c.EmbedDocuments(context.Background(), []string{"text"})

	require.NoError(t, err)
	assert.Equal(t, []string{"text"}, reqs[0].Input)
}

func TestEmbed_ServerErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `model "nomic-embed-text" not found`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	c := ollama.New(ollama.Config{BaseURL: srv.URL, Model: "nomic-embed-text", Dimensions: 1})

	_, err := c.EmbedDocuments(context.Background(), []string{"text"})

	assert.ErrorContains(t, err, "status 404")
	assert.ErrorContains(t, err, "not found")
}
