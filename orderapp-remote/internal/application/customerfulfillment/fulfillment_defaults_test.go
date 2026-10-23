package customerfulfillment

import "testing"

func TestFulfillmentDefaultsOnlyReplaceExplicitlyInheritedFields(t *testing.T) {
	d := FulfillmentModeSettings{Mode: "stock", FeeConfigured: true, FixedFeeCents: 300, PerPackageFeeCents: 80, ShippingFree: true, ShippingTemplateID: 7, ShippingPolicy: &FulfillmentShippingPolicy{TemplateID: 7, SurchargeCents: 50, FreeShipping: true}}
	legacy := FulfillmentModeSettings{Mode: "stock", Enabled: false, WarehouseCode: "OWN", FeeConfigured: false, FixedFeeCents: 0, ShippingPolicy: &FulfillmentShippingPolicy{Revision: 3}}
	got := ResolveFulfillmentModeDefaults(legacy, d)
	if got.Enabled || got.FeeConfigured || got.FixedFeeCents != 0 || got.ShippingFree || got.WarehouseCode != "OWN" {
		t.Fatalf("legacy changed: %+v", got)
	}
	custom := legacy
	custom.Enabled, custom.FeeConfigured = true, true
	custom.InheritedFields = []string{"per_package_fee_cents", "shipping_template_id", "shipping_surcharge_cents"}
	got = ResolveFulfillmentModeDefaults(custom, d)
	if got.FixedFeeCents != 0 || got.ShippingFree || got.PerPackageFeeCents != 80 || got.ShippingPolicy.TemplateID != 7 || got.ShippingPolicy.SurchargeCents != 50 || got.ShippingPolicy.Revision != 3 {
		t.Fatalf("explicit zero/false lost: %+v", got)
	}
	if custom.ShippingPolicy.TemplateID != 0 {
		t.Fatal("resolver mutated stored policy")
	}
	custom.InheritedFields = FulfillmentDefaultFields()
	got = ResolveFulfillmentModeDefaults(custom, d)
	if got.FixedFeeCents != 300 || !got.ShippingFree || got.WarehouseCode != "OWN" {
		t.Fatalf("restore default: %+v", got)
	}
	d.FixedFeeCents = 900
	if ResolveFulfillmentModeDefaults(custom, d).FixedFeeCents != 900 {
		t.Fatal("inherited setting did not follow shared default")
	}
}

func TestFulfillmentDefaultsRejectIdentityAndIncompleteFeeInheritance(t *testing.T) {
	s := FulfillmentSettings{CustomerID: 1, Modes: []FulfillmentModeSettings{{Mode: "on_demand", InheritedFields: []string{"enabled"}}}}
	if ValidateFulfillmentSettings(s) == nil {
		t.Fatal("capability may not be inherited")
	}
	s.Modes[0].InheritedFields = []string{"fixed_fee_cents", "fixed_fee_cents"}
	if ValidateFulfillmentSettings(s) == nil {
		t.Fatal("duplicate field accepted")
	}
	got := ResolveFulfillmentModeDefaults(FulfillmentModeSettings{FeeConfigured: true, InheritedFields: []string{"fixed_fee_cents"}}, FulfillmentModeSettings{})
	if got.FeeConfigured {
		t.Fatal("unconfigured default silently became free")
	}
}
