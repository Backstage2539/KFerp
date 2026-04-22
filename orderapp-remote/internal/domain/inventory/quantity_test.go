package inventory

import "testing"

func TestNormalize(t *testing.T) {
	spec := int64(454)
	q, err := Normalize(spec, Quantity{Units: 5, LooseG: 10})
	if err != nil {
		t.Fatal(err)
	}
	if q.Units != 5 || q.LooseG != 10 {
		t.Fatalf("expected 5+10g, got %d+%dg", q.Units, q.LooseG)
	}
	q, err = Normalize(spec, Quantity{Units: 5, LooseG: 470})
	if err != nil {
		t.Fatal(err)
	}
	if q.Units != 6 || q.LooseG != 16 {
		t.Fatalf("expected 6+16g, got %d+%dg", q.Units, q.LooseG)
	}
}

func TestDeductEnough(t *testing.T) {
	spec := int64(454)
	remain, deducted, gap, err := Deduct(spec, Quantity{Units: 0, LooseG: 500}, 454)
	if err != nil {
		t.Fatal(err)
	}
	if gap != 0 {
		t.Fatalf("expected gap=0, got %d", gap)
	}
	if deducted != 454 {
		t.Fatalf("expected deducted=454, got %d", deducted)
	}
	if remain.Units != 0 || remain.LooseG != 46 {
		t.Fatalf("expected remain 0+46g, got %d+%dg", remain.Units, remain.LooseG)
	}
}

func TestDeductInsufficient(t *testing.T) {
	spec := int64(454)
	remain, deducted, gap, err := Deduct(spec, Quantity{Units: 1, LooseG: 0}, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if remain.Units != 1 || remain.LooseG != 0 {
		t.Fatalf("expected remain unchanged 1+0g, got %d+%dg", remain.Units, remain.LooseG)
	}
	if deducted != 0 {
		t.Fatalf("expected deducted=0, got %d", deducted)
	}
	if gap != 546 {
		t.Fatalf("expected gap=546, got %d", gap)
	}
}

func TestDeductZero(t *testing.T) {
	spec := int64(454)
	remain, deducted, gap, err := Deduct(spec, Quantity{Units: 2, LooseG: 10}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if deducted != 0 || gap != 0 {
		t.Fatalf("expected deducted/gap 0, got %d/%d", deducted, gap)
	}
	if remain.Units != 2 || remain.LooseG != 10 {
		t.Fatalf("expected remain unchanged, got %d+%dg", remain.Units, remain.LooseG)
	}
}
