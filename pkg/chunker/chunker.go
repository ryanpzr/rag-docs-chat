// Package chunker splits text into overlapping chunks for embedding.
package chunker

import (
	"fmt"
	"strings"
	"unicode"
)

// Chunk is a piece of the source text. Index is its position in the
// sequence of chunks, starting at 0.
type Chunk struct {
	Index   int
	Content string
}

// Chunker splits text into chunks of at most Size characters (runes), where
// consecutive chunks share about Overlap characters. Overlap keeps a sentence
// that straddles a boundary retrievable from either side.
type Chunker struct {
	size    int
	overlap int
}

// New returns a Chunker. size must be positive and overlap must be in
// [0, size).
func New(size, overlap int) (*Chunker, error) {
	if size <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", size)
	}
	if overlap < 0 || overlap >= size {
		return nil, fmt.Errorf("chunk overlap must be in [0, %d), got %d", size, overlap)
	}
	return &Chunker{size: size, overlap: overlap}, nil
}

// breakSeparators are the places where a chunk may end, from most to least
// preferred: paragraph, line, sentence, word.
var breakSeparators = []string{"\n\n", "\n", ". ", "? ", "! ", " "}

// Split returns the chunks of text. Whitespace-only chunks are dropped and
// each chunk is trimmed. Sizes are counted in runes, so multi-byte UTF-8
// characters are never cut in half.
func (c *Chunker) Split(text string) []Chunk {
	runes := []rune(text)
	n := len(runes)
	var chunks []Chunk

	start := 0
	for start < n {
		end := min(start+c.size, n)
		if end < n {
			end = c.findBreak(runes, start, end)
		}

		if content := strings.TrimSpace(string(runes[start:end])); content != "" {
			chunks = append(chunks, Chunk{Index: len(chunks), Content: content})
		}
		if end == n {
			break
		}

		next := end - c.overlap
		// Always move forward, even when a natural break left a chunk
		// shorter than the overlap.
		if next <= start {
			next = end
		}
		// Do not start a chunk in the middle of a word.
		start = alignToWordStart(runes, next, end)
	}
	return chunks
}

// findBreak looks for the latest preferred separator in the second half of
// runes[start:end] and returns the index right after it. It falls back to
// end (a hard cut) when there is none.
func (c *Chunker) findBreak(runes []rune, start, end int) int {
	window := string(runes[start:end])
	minPos := len(string(runes[start : start+(end-start)/2]))
	for _, sep := range breakSeparators {
		if i := strings.LastIndex(window, sep); i >= minPos {
			// Convert the byte offset back to a rune offset.
			return start + len([]rune(window[:i+len(sep)]))
		}
	}
	return end
}

// alignToWordStart moves pos forward to the start of the next word if it
// falls inside one, without passing limit.
func alignToWordStart(runes []rune, pos, limit int) int {
	if pos == 0 || unicode.IsSpace(runes[pos-1]) {
		return pos
	}
	for i := pos; i < limit; i++ {
		if unicode.IsSpace(runes[i]) {
			return i + 1
		}
	}
	return pos
}
