package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestReturnRequiresExplicitGoodsAccountingChoice(t *testing.T) {
	for _, tc := range []struct {
		choice string
		valid  bool
		credit int64
	}{{"", false, 0}, {"keep_receivable", true, 0}, {"credit_returned_goods", true, 9000}, {"guessed", false, 0}} {
		t.Run(tc.choice, func(t *testing.T) {
			c, cmd := fixture()
			c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 3, AmountCents: 9000}}
			c.TotalCents = 9000
			c.PaidCents = 9000
			cmd.Action = "return"
			raw, _ := json.Marshal(map[string]any{"return_settlement": tc.choice})
			if err := json.Unmarshal(raw, &cmd); err != nil {
				t.Fatal(err)
			}
			if cmd.Action == "return" {
				setReturnStockFixture(t, &c, &cmd)
			}
			plan, err := Preview(c, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatal("return automatically chose the accounting treatment")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plan.CreditCents != tc.credit {
				t.Fatalf("credit=%d want %d", plan.CreditCents, tc.credit)
			}
			found := false
			for _, task := range plan.Tasks {
				if task.Kind == "return_finance" && task.Choice == tc.choice && task.Permission == "finance.resolve" {
					found = true
				}
				if tc.choice == "keep_receivable" && task.Kind == "funds" {
					t.Fatal("physical return silently made cash available")
				}
			}
			if !found {
				t.Fatal("missing explicit financial confirmation for returned goods")
			}
		})
	}
}

func TestReturnAndClosureShareOriginalGoodsRounding(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 1, Returned: 1, AmountCents: 2, CreditedCents: 1}}
	c.TotalCents = 2
	c.CreditCents = 1
	if err := json.Unmarshal([]byte(`{"return_credit_quantity":1,"return_credit_cents":1}`), &c.Lines[0]); err != nil {
		t.Fatal(err)
	}
	cmd.Action = "close"
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 1}}
	if cmd.Action == "return" {
		setReturnStockFixture(t, &c, &cmd)
	}
	plan, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CreditCents != 0 {
		t.Fatalf("return + partial closure duplicated a rounded cent: %+v", plan)
	}
	c.Lines[0].Closed = 1
	c.Lines[0].ClosedCents = 0
	c.Lines[0].Delivered = 2
	cmd.Action = "return"
	cmd.ReturnSettlement = "credit_returned_goods"
	if cmd.Action == "return" {
		setReturnStockFixture(t, &c, &cmd)
	}
	plan, err = Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CreditCents != 1 {
		t.Fatalf("final returned unit did not receive the exact remaining cent: %+v", plan)
	}
}

func TestReturnedGoodsKeepReceivableDoesNotConsumeCreditQuantity(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 3, Returned: 1, AmountCents: 2}}
	c.TotalCents = 2
	cmd.Action = "return"
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 1}}
	cmd.ReturnSettlement = "credit_returned_goods"
	if cmd.Action == "return" {
		setReturnStockFixture(t, &c, &cmd)
	}
	plan, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CreditCents != 1 {
		t.Fatalf("previous uncredited physical return changed monetary allocation: %+v", plan)
	}
}

func setReturnStockFixture(t *testing.T, c *Context, cmd *Command) {
	t.Helper()
	raw := []byte(`{"return_sources":[{"kind":"shipment","id":101,"item_id":1,"batch_id":201,"label":"原出库批次 R-1","unit":"袋","available_quantity":3,"batch_available_quantity":3,"version":"source-v1","destinations":[{"ownership":"company","warehouse":"finished_goods","label":"公司成品仓","owner_customer_id":0}]}]}`)
	if err := json.Unmarshal(raw, c); err != nil {
		t.Fatal(err)
	}
	qty := c.Lines[0].Delivered - c.Lines[0].Returned
	if cmd.Scope == "selected" {
		qty = cmd.Lines[0].Quantity
	}
	raw, _ = json.Marshal(map[string]any{"return_selections": []any{map[string]any{"source_kind": "shipment", "source_id": 101, "batch_id": 201, "quantity": qty, "ownership": "company", "warehouse": "finished_goods", "quality": "quarantine"}}})
	if err := json.Unmarshal(raw, cmd); err != nil {
		t.Fatal(err)
	}
}

func TestReturnRequiresExactSourceOwnershipWarehouseAndQuantity(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		valid         bool
	}{
		{"valid", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":3,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, true},
		{"missing", `[]`, false},
		{"unknown_source", `[{"source_kind":"shipment","source_id":999,"batch_id":201,"quantity":3,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"wrong_batch", `[{"source_kind":"shipment","source_id":101,"batch_id":999,"quantity":3,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"missing_owner", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":3,"warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"wrong_owner", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":3,"ownership":"customer","warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"wrong_warehouse", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":3,"ownership":"company","warehouse":"other","quality":"quarantine"}]`, false},
		{"missing_quality", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":3,"ownership":"company","warehouse":"finished_goods"}]`, false},
		{"short", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":2,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"excess", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":4,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, false},
		{"duplicate", `[{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":1,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"},{"source_kind":"shipment","source_id":101,"batch_id":201,"quantity":2,"ownership":"company","warehouse":"finished_goods","quality":"quarantine"}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, cmd := fixture()
			c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 3, AmountCents: 9000}}
			c.TotalCents = 9000
			cmd.Action = "return"
			cmd.ReturnSettlement = "keep_receivable"
			setReturnStockFixture(t, &c, &cmd)
			cmd.ReturnSelections = nil
			if err := json.Unmarshal([]byte(`{"return_selections":`+tc.payload+`}`), &cmd); err != nil {
				t.Fatal(err)
			}
			plan, err := Preview(c, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatalf("invalid return accepted: %+v", plan)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, task := range plan.Tasks {
				if task.Kind == "return_stock" && task.Choice == "receive" && task.Permission == "stock.resolve" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing explicit warehouse return task")
			}
		})
	}
}

func TestReturnKeepsFulfilledPackageHistoryExplicitly(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 3, AmountCents: 9000}}
	c.TotalCents = 9000
	c.FulfillmentMode = "stock"
	c.Dependencies = []Dependency{{Kind: "direct_ship", ID: 9, Relation: "package", Status: "reserved", Label: "原代发包裹"}}
	cmd.Action = "return"
	cmd.ReturnSettlement = "keep_receivable"
	cmd.Dispositions = []Disposition{{Kind: "direct_ship", ID: 9, Choice: "keep_fulfilled"}}
	setReturnStockFixture(t, &c, &cmd)
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range p.Tasks {
		if task.Kind == "direct_ship" {
			t.Fatal("return tried to cancel or rewrite original package")
		}
	}
	cmd.Dispositions[0].Choice = "cancel"
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("return accepted cancellation of the shipped package")
	}
}

func TestReturnPolicyAllowsOnlyConfiguredDisposition(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 3, AmountCents: 9000}}
	c.TotalCents = 9000
	cmd.Action = "return"
	cmd.ReturnSettlement = "keep_receivable"
	setReturnStockFixture(t, &c, &cmd)
	c.Policy.Disabled = map[string][]string{"return_finance": {"credit_returned_goods"}, "return_quality": {"restock"}}
	if _, err := Preview(c, cmd); err != nil {
		t.Fatalf("allowed explicit choices rejected: %v", err)
	}
	cmd.ReturnSettlement = "credit_returned_goods"
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("disabled return credit accepted")
	}
	cmd.ReturnSettlement = "keep_receivable"
	cmd.ReturnSelections[0].Quality = "restock"
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("disabled quality disposition accepted")
	}
}
