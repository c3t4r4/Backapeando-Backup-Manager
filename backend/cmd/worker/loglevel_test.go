package main

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		raw   string
		level slog.Level
		ok    bool
	}{
		{"", slog.LevelInfo, true},
		{"info", slog.LevelInfo, true},
		{" DEBUG ", slog.LevelDebug, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"verbose", slog.LevelInfo, false},
	}
	for _, tc := range cases {
		level, ok := parseLogLevel(tc.raw)
		if level != tc.level || ok != tc.ok {
			t.Errorf("parseLogLevel(%q) = %s, %v; want %s, %v", tc.raw, level, ok, tc.level, tc.ok)
		}
	}
}
