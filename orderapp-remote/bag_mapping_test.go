package main

import "testing"

func TestValidBagSpecMappingInputs(t *testing.T) {
	if err := validBagSpecMappingInputs(454, 10); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if err := validBagSpecMappingInputs(0, 10); err == nil {
		t.Fatalf("expected spec_g error")
	}
	if err := validBagSpecMappingInputs(454, 0); err == nil {
		t.Fatalf("expected material_id error")
	}
}

func TestMappingNameBySpec(t *testing.T) {
	m := []BagSpecMapping{
		{SpecG: 454, MaterialName: " 银袋454g "},
		{SpecG: 250, MaterialName: ""},
		{SpecG: 0, MaterialName: "非法"},
	}
	out := mappingNameBySpec(m)
	if len(out) != 1 {
		t.Fatalf("len=%d want=1", len(out))
	}
	if out[454] != "银袋454g" {
		t.Fatalf("name=%q want=银袋454g", out[454])
	}
}
