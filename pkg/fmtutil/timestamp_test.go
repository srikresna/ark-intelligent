package fmtutil

import (
	"testing"
	"time"
)

func TestUpdatedAt(t *testing.T) {
	// 2026-01-15 10:30:00 UTC = 17:30 WIB
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	got := UpdatedAt(ts)
	expected := "<i>Updated: 15 Jan 17:30 WIB</i>"
	if got != expected {
		t.Errorf("UpdatedAt() = %q, want %q", got, expected)
	}
}

func TestUpdatedAtShort(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	got := UpdatedAtShort(ts)
	expected := "17:30 WIB"
	if got != expected {
		t.Errorf("UpdatedAtShort() = %q, want %q", got, expected)
	}
}

func TestFormatDateTimeWIB(t *testing.T) {
	ts := time.Date(2026, 3, 31, 15, 0, 0, 0, time.UTC)
	got := FormatDateTimeWIB(ts)
	expected := "31 Mar 22:00 WIB"
	if got != expected {
		t.Errorf("FormatDateTimeWIB() = %q, want %q", got, expected)
	}
}
