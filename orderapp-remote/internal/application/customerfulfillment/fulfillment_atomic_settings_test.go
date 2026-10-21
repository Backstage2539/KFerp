package customerfulfillment

import (
	"encoding/json"
	"testing"
)

func TestAtomicFulfillmentSettingsValidateNestedPolicyAndDisabledDraft(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mode      string
		wantError bool
	}{
		{"nested policy without legacy projection", `{"mode":"on_demand","enabled":true,"fee_configured":true,"shipping_policy":{"template_id":8,"surcharge_cents":100,"free_shipping":false,"expected_revision":0}}`, false},
		{"free shipping without template", `{"mode":"on_demand","enabled":true,"fee_configured":true,"shipping_policy":{"template_id":0,"surcharge_cents":0,"free_shipping":true,"expected_revision":0}}`, false},
		{"nested policy needs explicit revision", `{"mode":"on_demand","enabled":true,"fee_configured":true,"shipping_template_id":8,"shipping_policy":{"template_id":8,"surcharge_cents":0}}`, true},
		{"negative nested surcharge", `{"mode":"on_demand","enabled":true,"fee_configured":true,"shipping_template_id":8,"shipping_policy":{"template_id":8,"surcharge_cents":-1,"expected_revision":0}}`, true},
		{"disabled stale draft is ignored", `{"mode":"stock","enabled":false,"fixed_fee_cents":-1,"shipping_policy":{"template_id":0,"surcharge_cents":-1}}`, false},
		{"enabled stock still requires warehouse", `{"mode":"stock","enabled":true,"fee_configured":true,"shipping_policy":{"template_id":8,"surcharge_cents":0,"expected_revision":0}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var settings FulfillmentSettings
			if err := json.Unmarshal([]byte(`{"customer_id":1,"modes":[`+tc.mode+`]}`), &settings); err != nil {
				t.Fatal(err)
			}
			err := ValidateFulfillmentSettings(settings)
			if (err != nil) != tc.wantError {
				t.Fatalf("validation error=%v wantError=%v", err, tc.wantError)
			}
		})
	}
}
