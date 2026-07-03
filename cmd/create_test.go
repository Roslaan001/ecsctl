package cmd

import (
	"testing"
)

func TestParseTags_KeyValue(t *testing.T) {
	result := parseTags([]string{"env=production", "team=platform"})

	if result["env"] != "production" {
		t.Errorf("env: got %q, want %q", result["env"], "production")
	}
	if result["team"] != "platform" {
		t.Errorf("team: got %q, want %q", result["team"], "platform")
	}
}

func TestParseTags_Empty(t *testing.T) {
	result := parseTags([]string{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseTags_ValueContainsEquals(t *testing.T) {
	// Value itself contains an '=' — only the first = should be the delimiter
	result := parseTags([]string{"url=https://example.com?foo=bar"})
	if result["url"] != "https://example.com?foo=bar" {
		t.Errorf("url: got %q, want %q", result["url"], "https://example.com?foo=bar")
	}
}

func TestParseTags_NoEqualsSign(t *testing.T) {
	// Malformed entry with no '=' — should be silently skipped
	result := parseTags([]string{"malformed"})
	if len(result) != 0 {
		t.Errorf("expected empty map for malformed input, got %v", result)
	}
}

func TestParseTags_EmptyValue(t *testing.T) {
	result := parseTags([]string{"key="})
	if result["key"] != "" {
		t.Errorf("key: got %q, want empty string", result["key"])
	}
}

func TestParseTags_MultipleEntries(t *testing.T) {
	input := []string{"a=1", "b=2", "c=3"}
	result := parseTags(input)
	if len(result) != 3 {
		t.Errorf("expected 3 tags, got %d", len(result))
	}
}
