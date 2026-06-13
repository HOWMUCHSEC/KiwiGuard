package application

import (
	"strings"
	"testing"
)

func TestRedactTextMergesAndSortsSpans(t *testing.T) {
	redacted, count, err := RedactText("alice@example.com token", []TextSpan{
		{Start: 18, End: 23},
		{Start: 0, End: 5},
		{Start: 3, End: 17},
	})
	if err != nil {
		t.Fatalf("RedactText returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("redaction count = %d, want 2", count)
	}
	if redacted != "[REDACTED] [REDACTED]" {
		t.Fatalf("redacted text = %q", redacted)
	}
}

func TestRedactTextIgnoresInvalidRanges(t *testing.T) {
	redacted, count, err := RedactText("safe text", []TextSpan{
		{Start: -1, End: 2},
		{Start: 4, End: 4},
		{Start: 20, End: 21},
	})
	if err != nil {
		t.Fatalf("RedactText returned error: %v", err)
	}
	if count != 0 || redacted != "safe text" {
		t.Fatalf("redaction = (%q, %d), want original text and zero count", redacted, count)
	}
}

func TestRedactTextRejectsInvalidUTF8Boundary(t *testing.T) {
	_, _, err := RedactText("pii: 你", []TextSpan{{Start: len("pii: ") + 1, End: len("pii: 你")}})
	if err == nil {
		t.Fatal("RedactText returned nil error for invalid UTF-8 boundary")
	}
	if !strings.Contains(err.Error(), "invalid utf-8 boundary") {
		t.Fatalf("error = %v, want invalid utf-8 boundary", err)
	}
}
