package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// splitMessage tests
// ---------------------------------------------------------------------------

func TestSplitMessage_ShortMessage(t *testing.T) {
	msg := "Hello, world!"
	chunks := splitMessage(msg, 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != msg {
		t.Errorf("expected %q, got %q", msg, chunks[0])
	}
}

func TestSplitMessage_EmptyString(t *testing.T) {
	chunks := splitMessage("", 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty string, got %d", len(chunks))
	}
	if chunks[0] != "" {
		t.Errorf("expected empty string, got %q", chunks[0])
	}
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	msg := strings.Repeat("A", 4096)
	chunks := splitMessage(msg, 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for exact-limit message, got %d", len(chunks))
	}
	if chunks[0] != msg {
		t.Errorf("message content mismatch")
	}
}

func TestSplitMessage_OneByteBeyondLimit(t *testing.T) {
	msg := strings.Repeat("A", 4097)
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	// Reassembled must equal original
	reassembled := strings.Join(chunks, "")
	if reassembled != msg {
		t.Errorf("reassembled message length %d != original %d", len(reassembled), len(msg))
	}
}

func TestSplitMessage_LongMessage_SplitsOnNewline(t *testing.T) {
	// Build a message with newlines so split prefers newline boundary
	line := strings.Repeat("X", 100) + "\n"
	msg := strings.Repeat(line, 50) // 50 * 101 = 5050 chars
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 4096+200 { // some slack for tag closure
			t.Errorf("chunk %d exceeds limit: %d chars", i, len(c))
		}
	}
}

func TestSplitMessage_UnclosedBoldTag(t *testing.T) {
	// First part has unclosed <b>, split should close+reopen
	part1 := "<b>" + strings.Repeat("A", 4090)
	part2 := strings.Repeat("B", 100) + "</b>"
	msg := part1 + part2
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	// First chunk should end with </b>
	if !strings.HasSuffix(chunks[0], "</b>") {
		t.Errorf("first chunk should close <b> tag, got suffix: %q", chunks[0][len(chunks[0])-20:])
	}
	// Second chunk should start with <b>
	if !strings.HasPrefix(chunks[1], "<b>") {
		t.Errorf("second chunk should reopen <b> tag, got prefix: %q", chunks[1][:20])
	}
}

func TestSplitMessage_UnclosedNestedTags(t *testing.T) {
	// <b><i>text... split ... text</i></b>
	part1 := "<b><i>" + strings.Repeat("A", 4090)
	part2 := strings.Repeat("B", 100) + "</i></b>"
	msg := part1 + part2
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	// First chunk ends with closing tags in reverse: </i></b>
	if !strings.HasSuffix(chunks[0], "</i></b>") {
		t.Errorf("first chunk should close nested tags in reverse, tail: %q", chunks[0][len(chunks[0])-30:])
	}
	// Second chunk reopens in order: <b><i>
	if !strings.HasPrefix(chunks[1], "<b><i>") {
		t.Errorf("second chunk should reopen tags in order, head: %q", chunks[1][:30])
	}
}

func TestSplitMessage_UnclosedPreTag(t *testing.T) {
	part1 := "<pre>" + strings.Repeat("A", 4090)
	part2 := strings.Repeat("B", 100) + "</pre>"
	msg := part1 + part2
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	if !strings.HasSuffix(chunks[0], "</pre>") {
		t.Errorf("first chunk should close <pre>, tail: %q", chunks[0][len(chunks[0])-20:])
	}
	if !strings.HasPrefix(chunks[1], "<pre>") {
		t.Errorf("second chunk should reopen <pre>, head: %q", chunks[1][:20])
	}
}

func TestSplitMessage_UnclosedCodeTag(t *testing.T) {
	part1 := "<code>" + strings.Repeat("X", 4090)
	part2 := strings.Repeat("Y", 100) + "</code>"
	msg := part1 + part2
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	if !strings.HasSuffix(chunks[0], "</code>") {
		t.Errorf("first chunk should close <code>")
	}
	if !strings.HasPrefix(chunks[1], "<code>") {
		t.Errorf("second chunk should reopen <code>")
	}
}

func TestSplitMessage_MultibyteUTF8(t *testing.T) {
	// Use multi-byte chars (emoji = 4 bytes each)
	emoji := "\U0001F600" // 😀
	msg := strings.Repeat(emoji, 2000) // 8000 bytes, 2000 runes
	chunks := splitMessage(msg, 4096)
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(chunks))
	}
	// Each chunk must be valid UTF-8
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
	}
}

func TestSplitMessage_NoTagsPreserveContent(t *testing.T) {
	// Ensure all content is preserved when no tags are involved
	msg := strings.Repeat("Hello World\n", 500) // ~6000 chars
	chunks := splitMessage(msg, 4096)
	reassembled := strings.Join(chunks, "")
	if reassembled != msg {
		t.Errorf("content lost: original %d chars, reassembled %d chars", len(msg), len(reassembled))
	}
}

func TestSplitMessage_SingleCharChunks(t *testing.T) {
	// Degenerate case: very small maxLen
	msg := "ABCDE"
	chunks := splitMessage(msg, 2)
	reassembled := strings.Join(chunks, "")
	if reassembled != msg {
		t.Errorf("expected %q, got %q", msg, reassembled)
	}
}

// ---------------------------------------------------------------------------
// detectUnclosedTags tests
// ---------------------------------------------------------------------------

func TestDetectUnclosedTags_Empty(t *testing.T) {
	tags := detectUnclosedTags("")
	if len(tags) != 0 {
		t.Errorf("expected no tags for empty string, got %v", tags)
	}
}

func TestDetectUnclosedTags_Balanced(t *testing.T) {
	text := "<b>bold</b> and <i>italic</i> and <code>code</code>"
	tags := detectUnclosedTags(text)
	if len(tags) != 0 {
		t.Errorf("expected no unclosed tags, got %v", tags)
	}
}

func TestDetectUnclosedTags_BalancedNested(t *testing.T) {
	text := "<b><i>nested</i></b>"
	tags := detectUnclosedTags(text)
	if len(tags) != 0 {
		t.Errorf("expected no unclosed tags, got %v", tags)
	}
}

func TestDetectUnclosedTags_SingleUnclosed(t *testing.T) {
	text := "<b>bold text without close"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("expected [b], got %v", tags)
	}
}

func TestDetectUnclosedTags_MultipleUnclosed(t *testing.T) {
	text := "<b><i>bold italic without close"
	tags := detectUnclosedTags(text)
	if len(tags) != 2 {
		t.Fatalf("expected 2 unclosed tags, got %v", tags)
	}
	if tags[0] != "b" || tags[1] != "i" {
		t.Errorf("expected [b, i], got %v", tags)
	}
}

func TestDetectUnclosedTags_MismatchedClose(t *testing.T) {
	// </b> without opening — should not crash, just ignore
	text := "</b>some text"
	tags := detectUnclosedTags(text)
	if len(tags) != 0 {
		t.Errorf("expected no unclosed tags for mismatched close, got %v", tags)
	}
}

func TestDetectUnclosedTags_MismatchedNested(t *testing.T) {
	// Open <b>, then close </i> — should not pop <b>
	text := "<b>text</i>"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("expected [b] (mismatched close should not pop), got %v", tags)
	}
}

func TestDetectUnclosedTags_UntrackedTags(t *testing.T) {
	// Tags not in tracked set should be ignored
	text := "<div>text</div><b>bold"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("expected [b], got %v", tags)
	}
}

func TestDetectUnclosedTags_PartialTag(t *testing.T) {
	// Tag opened with < but no closing > — should not crash
	text := "some text <b"
	tags := detectUnclosedTags(text)
	if len(tags) != 0 {
		t.Errorf("expected no tags for partial tag, got %v", tags)
	}
}

func TestDetectUnclosedTags_PreTag(t *testing.T) {
	text := "<pre>some preformatted text"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "pre" {
		t.Errorf("expected [pre], got %v", tags)
	}
}

func TestDetectUnclosedTags_TagWithAttributes(t *testing.T) {
	// Opening tag with attributes — name should be extracted correctly
	text := "<code class=\"python\">print('hello')"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "code" {
		t.Errorf("expected [code], got %v", tags)
	}
}

func TestDetectUnclosedTags_DeeplyNested(t *testing.T) {
	text := "<b><i><code>deeply nested"
	tags := detectUnclosedTags(text)
	if len(tags) != 3 {
		t.Fatalf("expected 3 unclosed tags, got %v", tags)
	}
	expected := []string{"b", "i", "code"}
	for i, want := range expected {
		if tags[i] != want {
			t.Errorf("tag[%d]: expected %q, got %q", i, want, tags[i])
		}
	}
}

func TestDetectUnclosedTags_PartiallyClosedNesting(t *testing.T) {
	// Open b, i, code — close code, i — b remains
	text := "<b><i><code>text</code></i>more"
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("expected [b], got %v", tags)
	}
}

func TestDetectUnclosedTags_NeverClosed(t *testing.T) {
	// Single opening tag with lots of text
	text := "<b>" + strings.Repeat("word ", 1000)
	tags := detectUnclosedTags(text)
	if len(tags) != 1 || tags[0] != "b" {
		t.Errorf("expected [b], got %v", tags)
	}
}
