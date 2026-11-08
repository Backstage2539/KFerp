package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestCompanyAllocationRequiresExactBatchTargetAndQuantity(t *testing.T) {
	for _, tc := range []struct {
		name, selections string
		valid            bool
	}{
		{"missing selection", `[]`, false},
		{"wrong batch", `[{"batch_id":302,"target_item_id":401,"quantity":2}]`, false},
		{"wrong target", `[{"batch_id":301,"target_item_id":402,"quantity":2}]`, false},
		{"missing quantity", `[{"batch_id":301,"target_item_id":401}]`, false},
		{"exceeds free stock", `[{"batch_id":301,"target_item_id":401,"quantity":4}]`, false},
		{"exceeds target remainder", `[{"batch_id":301,"target_item_id":401,"quantity":3}]`, false},
		{"duplicate", `[{"batch_id":301,"target_item_id":401,"quantity":1},{"batch_id":301,"target_item_id":401,"quantity":1}]`, false},
		{"explicit allocation", `[{"batch_id":301,"target_item_id":401,"quantity":2}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, cmd := fixture()
			if err := json.Unmarshal([]byte(`[{"kind":"production","id":19,"label":"已完成共享生产","status":"completed","shared":true,"company_stock":[{"batch_id":301,"label":"批次 FP-301 · 公司成品仓","available_quantity":3,"close_limit":3,"source_item_id":1,"targets":[{"order_id":40,"order_item_id":401,"label":"SO-40 · 原冻结规格","remaining_quantity":2}]}]}]`), &c.Dependencies); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`[{"kind":"production","id":19,"choice":"allocate","target_id":40,"stock_selections":`+tc.selections+`}]`), &cmd.Dispositions); err != nil {
				t.Fatal(err)
			}
			plan, err := Preview(c, cmd)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				if len(plan.Tasks) != 1 || plan.Tasks[0].Permission != "stock.resolve" {
					t.Fatalf("stock assignment requires warehouse confirmation: %+v", plan.Tasks)
				}
			} else if err == nil {
				t.Fatal("implicit or invalid company inventory assignment accepted")
			}
		})
	}
}

func TestCompanyAllocationCannotMultiplyClosingQuantityAcrossPlans(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, AmountCents: 9000}}
	c.TotalCents = 9000
	for _, id := range []int64{19, 20} {
		c.Dependencies = append(c.Dependencies, Dependency{Kind: "production", ID: id, Status: "completed", CompanyStock: []CompanyStockSource{{BatchID: id, SourceItemID: 1, AvailableQuantity: 3, CloseLimit: 3, Targets: []CompanyStockTarget{{OrderID: 40, OrderItemID: 401, RemainingQuantity: 6}}}}})
		cmd.Dispositions = append(cmd.Dispositions, Disposition{Kind: "production", ID: id, Choice: "allocate", StockSelections: []StockSelection{{BatchID: id, TargetItemID: 401, Quantity: 2}}})
	}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("multiple production plans allocated more than the closing demand")
	}
}

func TestCompanyAllocationProtectsStillOpenUnreservedQuantity(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, AmountCents: 9000}}
	c.TotalCents = 9000
	c.Dependencies = []Dependency{{Kind: "production", ID: 19, Status: "completed", CompanyStock: []CompanyStockSource{{BatchID: 301, SourceItemID: 1, AvailableQuantity: 1, CloseLimit: 3, UnreservedQuantity: 3, Targets: []CompanyStockTarget{{OrderID: 40, OrderItemID: 401, RemainingQuantity: 3}}}}}}
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 1}}
	cmd.Dispositions = []Disposition{{Kind: "production", ID: 19, Choice: "allocate", StockSelections: []StockSelection{{BatchID: 301, TargetItemID: 401, Quantity: 1}}}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("partial close allocated stock needed for continuing fulfillment")
	}
}
