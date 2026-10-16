package specconversion

import "testing"

func TestConvertExplicitRatio(t *testing.T) {
	for _, tc := range []struct{ price, source, target, want string }{
		{"48", "0.454", "0.5", "52.86"},
		{"48", "0.454", "0.25", "26.43"},
		{"45.40", "0.454", "0.5", "50.00"},
		{"120", "12", "1", "10.00"}, // counts, without weight semantics
		{"1.005", "1", "1", "1.01"},
	} {
		got, err := Convert(tc.price, tc.source, tc.target)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: got %q, %v", tc, got, err)
		}
	}
	for _, bad := range []string{"0", "-1", "NaN", "Inf", "", "1/2"} {
		if _, err := Convert("48", bad, "0.5"); err == nil {
			t.Fatalf("accepted invalid ratio %q", bad)
		}
	}
}

func TestMatchUsesExplicitIdentityAndRejectsAmbiguity(t *testing.T) {
	g := Group{Name: "包装转KG", BaseUnit: "kg", Active: true, Members: []Member{
		{TemplateID: 7, TemplateKey: "spec-a", InventoryUnit: "袋", BaseQuantity: "0.454"},
		{SpecID: 22, InventoryUnit: "袋", BaseQuantity: "0.5"},
	}}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []Identity{{SpecID: 10, TemplateID: 7, TemplateKey: "spec-a", InventoryUnit: "袋"}, {SpecID: 22, InventoryUnit: "袋"}} {
		if _, err := g.Match(spec); err != nil {
			t.Fatal(err)
		}
	}
	for _, spec := range []Identity{{SpecID: 99, InventoryUnit: "袋"}, {SpecID: 10, TemplateID: 8, TemplateKey: "spec-a", InventoryUnit: "袋"}, {SpecID: 22, InventoryUnit: "盒"}} {
		if _, err := g.Match(spec); err == nil {
			t.Fatalf("matched unrelated spec %+v", spec)
		}
	}
	g.Members = append(g.Members, Member{SpecID: 10, InventoryUnit: "袋", BaseQuantity: "0.5"})
	if _, err := g.Match(Identity{SpecID: 10, TemplateID: 7, TemplateKey: "spec-a", InventoryUnit: "袋"}); err == nil {
		t.Fatal("accepted conflicting associations")
	}
	g.Members = append(g.Members, g.Members[0])
	if err := g.Validate(); err == nil {
		t.Fatal("accepted duplicate association")
	}
}
