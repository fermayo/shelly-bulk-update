package main

import (
	"fmt"
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{name: "empty string", input: "", n: 5, want: ""},
		{name: "shorter than limit", input: "short", n: 10, want: "short"},
		{name: "exactly at limit", input: "exact", n: 5, want: "exact"},
		{name: "longer than limit", input: "a long device name here", n: 10, want: "a long de…"},
		{name: "limit of one", input: "abc", n: 1, want: "…"},
		{name: "counts runes not bytes", input: "héllo wörld", n: 6, want: "héllo…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.input, tt.n); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestSpinner(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration
		want    string
	}{
		{name: "start", elapsed: 0, want: "⠋"},
		{name: "first step", elapsed: 120 * time.Millisecond, want: "⠙"},
		{name: "second step", elapsed: 240 * time.Millisecond, want: "⠹"},
		{name: "full cycle wraps to start", elapsed: 1200 * time.Millisecond, want: "⠋"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spinner(tt.elapsed); got != tt.want {
				t.Errorf("spinner(%v) = %q, want %q", tt.elapsed, got, tt.want)
			}
		})
	}
}

func TestFormatDeviceStatus(t *testing.T) {
	tests := []struct {
		status   deviceStatus
		message  string
		wantIcon string
		wantText string
	}{
		{statusQueued, "", "●", "queued"},
		{statusChecking, "", "◌", "checking for updates..."},
		{statusUpToDate, "1.2.3", "✓", "up to date (1.2.3)"},
		{statusUpToDate, "", "✓", "up to date"},
		{statusUpdating, "1.2.3 → 1.4.0", "↻", "updating 1.2.3 → 1.4.0..."},
		{statusUpdated, "1.4.0", "✓", "updated! (1.4.0)"},
		{statusFailed, "HTTP 401", "✗", "failed: HTTP 401"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.status), func(t *testing.T) {
			dev := &deviceState{status: tt.status, message: tt.message}
			icon, _, text := formatDeviceStatus(dev)
			if icon != tt.wantIcon || text != tt.wantText {
				t.Errorf("formatDeviceStatus(%v, %q) = (%q, %q), want (%q, %q)",
					tt.status, tt.message, icon, text, tt.wantIcon, tt.wantText)
			}
		})
	}
}
