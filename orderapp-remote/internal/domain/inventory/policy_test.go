package inventory

import "testing"

func TestDecideDeductionInsufficientForbidden(t *testing.T) {
	d := DecideDeduction(900, 1000, 200)
	if d.Allowed {
		t.Fatalf("expected forbidden when insufficient")
	}
	if d.Reason != "insufficient" {
		t.Fatalf("expected reason=insufficient, got %q", d.Reason)
	}
}

func TestDecideDeductionBelowWarningAllowedWithPrompt(t *testing.T) {
	d := DecideDeduction(1200, 500, 800)
	if !d.Allowed {
		t.Fatalf("expected allowed")
	}
	if !d.WarningLow {
		t.Fatalf("expected warning when after below warning line")
	}
	if d.DeductAfter != 700 {
		t.Fatalf("expected after=700, got %d", d.DeductAfter)
	}
}

func TestDecideDeductionNormalAllowedNoWarning(t *testing.T) {
	d := DecideDeduction(1500, 500, 900)
	if !d.Allowed {
		t.Fatalf("expected allowed")
	}
	if d.WarningLow {
		t.Fatalf("expected no warning")
	}
	if d.DeductAfter != 1000 {
		t.Fatalf("expected after=1000, got %d", d.DeductAfter)
	}
}
