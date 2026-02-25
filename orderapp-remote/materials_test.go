package main

import "testing"

func TestNormalizeMaterialInput_DefaultsAndTrim(t *testing.T) {
	code, name, kind, unit, err := normalizeMaterialInput("  M001 ", "  豆袋 ", "", "", 1, 2, 3, 4)
	if err != nil { t.Fatalf("err=%v", err) }
	if code != "M001" || name != "豆袋" || kind != "other" || unit != "g" {
		t.Fatalf("unexpected values: %q %q %q %q", code, name, kind, unit)
	}
}

func TestNormalizeMaterialInput_RejectNegativeQty(t *testing.T) {
	_, _, _, _, err := normalizeMaterialInput("M001", "豆袋", "pack", "unit", -1, 0, 0, 0)
	if err == nil { t.Fatalf("expected error") }
}
