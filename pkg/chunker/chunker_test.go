package chunker_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanpzr/rag-docs-chat/pkg/chunker"
)

func TestNew_RejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name          string
		size, overlap int
	}{
		{"zero size", 0, 0},
		{"negative size", -1, 0},
		{"negative overlap", 10, -1},
		{"overlap equal to size", 10, 10},
		{"overlap larger than size", 10, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := chunker.New(tc.size, tc.overlap)
			assert.Error(t, err)
		})
	}
}

func TestSplit_EmptyAndWhitespace(t *testing.T) {
	c, err := chunker.New(100, 10)
	require.NoError(t, err)

	assert.Empty(t, c.Split(""))
	assert.Empty(t, c.Split("   \n\n\t  "))
}

func TestSplit_ShortTextIsOneTrimmedChunk(t *testing.T) {
	c, err := chunker.New(100, 10)
	require.NoError(t, err)

	got := c.Split("  hello world  \n")
	assert.Equal(t, []chunker.Chunk{{Index: 0, Content: "hello world"}}, got)
}

func TestSplit_ChunksRespectSizeAndAreIndexed(t *testing.T) {
	c, err := chunker.New(50, 10)
	require.NoError(t, err)

	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	chunks := c.Split(text)

	require.Greater(t, len(chunks), 1)
	for i, ch := range chunks {
		assert.Equal(t, i, ch.Index)
		assert.LessOrEqual(t, utf8.RuneCountInString(ch.Content), 50)
		assert.NotEmpty(t, ch.Content)
	}
}

func TestSplit_ConsecutiveChunksOverlap(t *testing.T) {
	c, err := chunker.New(40, 15)
	require.NoError(t, err)

	words := make([]string, 60)
	for i := range words {
		words[i] = "w" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	chunks := c.Split(strings.Join(words, " "))

	require.Greater(t, len(chunks), 2)
	for i := 1; i < len(chunks); i++ {
		prevWords := strings.Fields(chunks[i-1].Content)
		first := strings.Fields(chunks[i].Content)[0]
		assert.Contains(t, prevWords, first,
			"chunk %d should start with a word from the end of chunk %d", i, i-1)
	}
}

func TestSplit_NoOverlapCoversTextExactlyOnce(t *testing.T) {
	c, err := chunker.New(30, 0)
	require.NoError(t, err)

	text := strings.Repeat("alpha beta gamma delta ", 10)
	chunks := c.Split(text)

	var words []string
	for _, ch := range chunks {
		words = append(words, strings.Fields(ch.Content)...)
	}
	assert.Equal(t, strings.Fields(text), words)
}

func TestSplit_PrefersParagraphBreaks(t *testing.T) {
	c, err := chunker.New(60, 0)
	require.NoError(t, err)

	para1 := "First paragraph talks about one topic here."
	para2 := "Second paragraph is about something else."
	chunks := c.Split(para1 + "\n\n" + para2)

	require.Len(t, chunks, 2)
	assert.Equal(t, para1, chunks[0].Content)
	assert.Equal(t, para2, chunks[1].Content)
}

func TestSplit_DoesNotCutWords(t *testing.T) {
	c, err := chunker.New(25, 5)
	require.NoError(t, err)

	text := "retrieval augmented generation combines search with language models"
	vocabulary := strings.Fields(text)
	for _, ch := range c.Split(text) {
		for _, w := range strings.Fields(ch.Content) {
			assert.Contains(t, vocabulary, w)
		}
	}
}

func TestSplit_HardCutsTextWithoutSpaces(t *testing.T) {
	c, err := chunker.New(10, 2)
	require.NoError(t, err)

	chunks := c.Split(strings.Repeat("x", 35))

	require.NotEmpty(t, chunks)
	for _, ch := range chunks {
		assert.LessOrEqual(t, len(ch.Content), 10)
	}
	// Every hard cut advances by size - overlap.
	assert.Len(t, chunks, 5) // starts at 0, 8, 16, 24, 32
}

func TestSplit_KeepsMultiByteCharactersIntact(t *testing.T) {
	c, err := chunker.New(7, 2)
	require.NoError(t, err)

	text := strings.Repeat("ação 日本語 ", 10)
	for _, ch := range c.Split(text) {
		assert.True(t, utf8.ValidString(ch.Content), "invalid UTF-8 in %q", ch.Content)
		assert.LessOrEqual(t, utf8.RuneCountInString(ch.Content), 7)
	}
}
