package main

import (
	"strings"
	"testing"
)

func TestGenerateLocalToken(t *testing.T) {
	token := generateLocalToken()

	if len(token) == 0 {
		t.Error("expected non-empty token")
	}

	// Test that tokens are different on subsequent calls.
	token2 := generateLocalToken()
	if token == token2 {
		t.Error("expected different tokens on subsequent calls")
	}

	// Test that token doesn't contain problematic characters.
	if strings.Contains(token, " ") {
		t.Error("token should not contain spaces")
	}

	// Test expected length (base64 encoding of 16 bytes, truncated to 22 chars).
	if len(token) != 22 {
		t.Errorf("expected token length 22, got %d", len(token))
	}

	// Test fallback behavior by temporarily disabling randomness.
	// We can't easily test this without changing the function, so we'll skip it.
}
