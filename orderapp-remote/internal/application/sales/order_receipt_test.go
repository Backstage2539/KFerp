package sales

import (
	"testing"
	"time"
)

func TestReceiptDateUsesShanghaiDay(t *testing.T) {
	now := time.Date(2026, 11, 10, 16, 30, 0, 0, time.UTC) // Shanghai is already November 11.
	if err := validateReceiptDate("2026-11-11", now); err != nil {
		t.Fatalf("today rejected: %v", err)
	}
	if err := validateReceiptDate("2026-11-10", now); err != nil {
		t.Fatalf("historical rejected: %v", err)
	}
	if err := validateReceiptDate("2026-11-12", now); err == nil {
		t.Fatal("future date accepted")
	}
}
