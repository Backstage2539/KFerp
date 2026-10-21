package customerportal

import (
	"context"
	"testing"
)

type fulfillmentDependentCapabilitiesRepository struct {
	*fakeRepository
	modes []string
}

func (r *fulfillmentDependentCapabilitiesRepository) FulfillmentModes(context.Context, int64) ([]string, error) {
	return r.modes, nil
}

func TestMeDerivesRequiredProductOrderingFromEitherDropshipMode(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		modes                []string
		configured, required bool
	}{
		{"stock enables product ordering", []string{"stock"}, false, true},
		{"on demand enables product ordering", []string{"on_demand"}, false, true},
		{"both modes require product ordering", []string{"stock", "on_demand"}, false, true},
		{"closing all modes restores disabled configuration", nil, false, false},
		{"ordinary product ordering remains enabled", nil, true, false},
		{"unknown mode does not grant product ordering", []string{"unknown"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]any{"required_by_fulfillment": true, "keep": "configured"}
			base := &fakeRepository{context: CurrentContext{CurrentCustomerID: 74, Capabilities: []Capability{{Code: CapabilityProductOrder, Enabled: tc.configured, Config: config}}}}
			r := &fulfillmentDependentCapabilitiesRepository{fakeRepository: base, modes: tc.modes}
			current, err := NewService(r, nil).Me(context.Background(), "isolated-token")
			if err != nil {
				t.Fatal(err)
			}
			if current.HasCapability(CapabilityProductOrder) != (tc.configured || tc.required) {
				t.Fatalf("product ordering=%v want configured=%v required=%v", current.HasCapability(CapabilityProductOrder), tc.configured, tc.required)
			}
			for _, c := range current.Capabilities {
				if c.Code == CapabilityProductOrder {
					if (c.Config["required_by_fulfillment"] == true) != tc.required || c.Config["keep"] != "configured" {
						t.Fatalf("bad dependency metadata: %+v", c)
					}
				}
			}
			if base.context.Capabilities[0].Enabled != tc.configured || config["required_by_fulfillment"] != true {
				t.Fatal("effective dependency mutated configured/shared capabilities")
			}
		})
	}
}

func TestPortalAdminDetailDerivesRequiredProductOrderingAfterLiveTemplate(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		modes                []string
		configured, required bool
	}{
		{"stock", []string{"stock"}, false, true},
		{"on_demand", []string{"on_demand"}, false, true},
		{"both", []string{"stock", "on_demand"}, false, true},
		{"all_disabled", nil, false, false},
		{"ordinary_enabled", nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			template, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
			for i := range template.Capabilities {
				if template.Capabilities[i].Code == CapabilityProductOrder {
					template.Capabilities[i].Enabled = tc.configured
				}
			}
			repo := &fulfillmentDependentCapabilitiesRepository{
				fakeRepository: &fakeRepository{
					templates:    []CapabilityTemplate{template},
					portalDetail: PortalAdminDetail{Customer: PortalAdminCustomer{CapabilityTemplateKey: template.Key}},
				},
				modes: tc.modes,
			}
			detail, err := NewService(repo, nil).PortalAdminDetail(context.Background(), 74)
			if err != nil {
				t.Fatal(err)
			}
			for _, capability := range detail.Capabilities {
				if capability.Code != CapabilityProductOrder {
					continue
				}
				if capability.Enabled != (tc.configured || tc.required) || (capability.Config["required_by_fulfillment"] == true) != tc.required {
					t.Fatalf("admin service lost fulfillment dependency after template read: %+v", capability)
				}
				if capability.Config["public_sku_aliases"] != true {
					t.Fatalf("admin service lost template configuration: %+v", capability)
				}
			}
		})
	}
}
