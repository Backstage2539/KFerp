package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestCompanyScrapRequiresExplicitBatchCostAndTwoRoles(t *testing.T) {
	for _, tc := range []struct {
		name, selections, treatment, reference string
		valid                                  bool
	}{
		{"missing batch", `[]`, "retain_cost", "验收报损", false},
		{"missing cost choice", `[{"batch_id":301,"quantity":2}]`, "", "验收报损", false},
		{"missing loss reason", `[{"batch_id":301,"quantity":2}]`, "retain_cost", "", false},
		{"other batch", `[{"batch_id":302,"quantity":2}]`, "retain_cost", "验收报损", false},
		{"other target", `[{"batch_id":301,"target_item_id":40,"quantity":2}]`, "retain_cost", "验收报损", false},
		{"excess shared quantity", `[{"batch_id":301,"quantity":4}]`, "retain_cost", "验收报损", false},
		{"preserve original cost", `[{"batch_id":301,"quantity":2}]`, "retain_cost", "验收报损", true},
		{"reclassify original cost", `[{"batch_id":301,"quantity":2}]`, "reclassify_loss", "验收报损", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, cmd := fixture()
			if err := json.Unmarshal([]byte(`[{"kind":"production","id":19,"status":"completed","shared":true,"company_stock":[{"batch_id":301,"label":"原批次","source_item_id":1,"available_quantity":3,"close_limit":3,"unit_cost_micros":10000000,"cost_record_id":7,"cost_month":"2026-10","cost_date":"2026-10-01"}]}]`), &c.Dependencies); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(map[string]any{"kind": "production", "id": 19, "choice": "scrap", "stock_selections": json.RawMessage(tc.selections), "cost_treatment": tc.treatment, "reference": tc.reference})
			var d Disposition
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			cmd.Dispositions = []Disposition{d}
			plan, err := Preview(c, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatal("implicit or invalid loss disposition accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			roles := map[string]bool{}
			for _, task := range plan.Tasks {
				roles[task.Permission] = true
			}
			if !roles["stock.resolve"] || !roles["finance.resolve"] {
				t.Fatalf("scrap needs warehouse and finance: %+v", plan.Tasks)
			}
		})
	}
}
