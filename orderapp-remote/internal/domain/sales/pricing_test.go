package sales

import (
	"math"
	"testing"
)

func TestApplyRoundToIntDisabled(t *testing.T) {
	grand, rounding := ApplyRoundToInt(12.34, false)
	if grand != 12.34 || rounding != 0 {
		t.Fatalf("expected unchanged total and zero rounding, got %.2f/%.2f", grand, rounding)
	}
}

func TestApplyRoundToIntEnabled(t *testing.T) {
	grand, rounding := ApplyRoundToInt(12.34, true)
	if grand != 12 {
		t.Fatalf("expected grand=12, got %.2f", grand)
	}
	if math.Abs(rounding-(-0.34)) > 0.000001 {
		t.Fatalf("expected rounding=-0.34, got %.2f", rounding)
	}
}
