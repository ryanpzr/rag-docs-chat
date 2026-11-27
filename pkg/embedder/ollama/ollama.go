// Package ollama implements embedder.Embedder with a local Ollama server.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ryanpzr/rag-docs-chat/pkg/embedder"
)

// nomic-embed-text was trained with task prefixes; using them noticeably
// improves retrieval. Other models get no prefix.
var taskPrefixes = map[string]struct{ document, query string }{
	"nomic-embed-text": {document: "search_document: ", query: "search_query: "},
}

type Config struct {
	BaseURL    string // e.g. http://localhost:11434
	Model      string // e.g. nomic-embed-text
	Dimensions int
	BatchSize  int // texts per request; <= 0 means all at once
	HTTPClient *http.Client
}

type Client struct {
	cfg            Config
	documentPrefix string
	queryPrefix    string
}

var _ embedder.Embedder = (*Client)(nil)

func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	p := taskPrefixes[strings.SplitN(cfg.Model, ":", 2)[0]]
	return &Client{cfg: cfg, documentPrefix: p.document, queryPrefix: p.query}
}

func (c *Client) Dimensions() int { return c.cfg.Dimensions }

func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	batch := c.cfg.BatchSize
	if batch <= 0 {
		batch = len(texts)
	}
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += batch {
		part := texts[i:min(i+batch, len(texts))]
		inputs := make([]string, len(part))
		for j, t := range part {
			inputs[j] = c.documentPrefix + t
		}
		vecs, err := c.embed(ctx, inputs)
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.embed(ctx, []string{c.queryPrefix + text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// embed calls POST /api/embed and checks that one vector came back per input.
func (c *Client) embed(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.cfg.Model, Input: inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ollama embed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama embed: decode response: %w", err)
	}
	if len(out.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("ollama embed: got %d embeddings for %d inputs", len(out.Embeddings), len(inputs))
	}
	return out.Embeddings, nil
}
