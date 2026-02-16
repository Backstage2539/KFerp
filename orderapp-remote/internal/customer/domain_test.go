package customer

import "testing"

func TestCustomer_Normalize_DefaultsRawName(t *testing.T) {
	c := Customer{Name: "  ACME  "}
	c.Normalize()
	if c.Name != "ACME" {
		t.Fatalf("expected normalized name")
	}
	if c.RawName != "ACME" {
		t.Fatalf("expected raw name default")
	}
}

func TestCustomer_Validate_NameRequired(t *testing.T) {
	c := Customer{Name: "  "}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}
