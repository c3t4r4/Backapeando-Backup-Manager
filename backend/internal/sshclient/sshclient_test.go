package sshclient

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\\''quote'"},
		{"with;semi", "'with;semi'"},
		{"with`backtick", "'with`backtick'"},
		{"with$(sub)", "'with$(sub)'"},
		{"_.-valid", "_.-valid"},
		{"foo123", "foo123"},
		{"foo_bar-baz.txt", "foo_bar-baz.txt"},
		{"", "''"},
	}
	for _, tt := range tests {
		got := ShellQuote(tt.input)
		if got != tt.expected {
			t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestShellQuotePreservesSimpleStrings(t *testing.T) {
	// Strings that should pass through unchanged
	simple := []string{
		"hostname",
		"user_name",
		"file.txt",
		"id-123",
	}
	for _, s := range simple {
		got := ShellQuote(s)
		if got != s {
			t.Errorf("ShellQuote(%q) should pass through unchanged, got %q", s, got)
		}
	}
}

func TestShellQuoteHandlesSpecialChars(t *testing.T) {
	// Strings with special characters should be quoted
	special := []string{
		"hello world",
		"path/to/file",
		"var=$USER",
		"echo|grep",
		"test&test",
		"dir/*",
	}
	for _, s := range special {
		got := ShellQuote(s)
		if !isAlphaNumericDash(s) && got == s {
			t.Errorf("ShellQuote(%q) should be quoted for safety", s)
		}
	}
}
