package bot

import (
	"strings"
	"testing"
)

func TestSplitMessageKeepsShortTextIntact(t *testing.T) {
	t.Parallel()

	chunks := splitMessage("short message", 4000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "short message" {
		t.Fatalf("unexpected chunk content: %q", chunks[0])
	}
}

func TestSplitMessageSplitsLongTextAndPreservesContent(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", 4050) + "\n" + strings.Repeat("b", 4050)
	chunks := splitMessage(text, 4000)
	if len(chunks) < 3 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	var rebuilt strings.Builder
	for i, chunk := range chunks {
		if len([]rune(chunk)) > 4000 {
			t.Fatalf("chunk %d exceeds max length: %d", i, len([]rune(chunk)))
		}
		rebuilt.WriteString(chunk)
	}

	if rebuilt.String() != text {
		t.Fatal("rebuilt text does not match original")
	}
}

func TestSplitMessageHandlesUnicodeSafely(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("🙂", 4500)
	chunks := splitMessage(text, 4000)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len([]rune(chunks[0])) != 4000 {
		t.Fatalf("expected first chunk len 4000, got %d", len([]rune(chunks[0])))
	}
	if len([]rune(chunks[1])) != 500 {
		t.Fatalf("expected second chunk len 500, got %d", len([]rune(chunks[1])))
	}
}
