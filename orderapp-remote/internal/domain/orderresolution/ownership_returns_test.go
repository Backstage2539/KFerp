package orderresolution

import "testing"

func ownershipReturnCommandFixture(t *testing.T) (Context, Command) {
	t.Helper()
	c, cmd := fixture()
	c.DeliveryMode = "customer_stock"
	c.Lines = []Line{{ID: 1, Quantity: 5, Delivered: 3, AmountCents: 15000}}
	c.TotalCents = 15000
	c.Dependencies = []Dependency{{Kind: "customer_stock", ID: 11, Quantity: 3, Status: "active", Label: "原入库权益"}}
	c.ReturnSources = []ReturnSource{{Kind: "ownership", ID: 101, ItemID: 1, EntitlementID: 11, BatchID: 201, Label: "原入库货权批次", Unit: "袋", AvailableQuantity: 3, BatchAvailableQuantity: 3, Destinations: []ReturnDestination{{Ownership: "company", Warehouse: "finished_goods"}}}}
	cmd.Action = "return"
	cmd.ReturnSettlement = "credit_returned_goods"
	cmd.Dispositions = []Disposition{{Kind: "customer_stock", ID: 11, Choice: "ownership_return"}}
	cmd.ReturnSelections = []ReturnSelection{{SourceKind: "ownership", SourceID: 101, BatchID: 201, Quantity: 3, Ownership: "company", Warehouse: "finished_goods", Quality: "quarantine"}}
	return c, cmd
}
func TestOwnershipReturnRequiresExplicitMatchingEntitlementDisposition(t *testing.T) {
	for _, tc := range []struct {
		choice string
		valid  bool
	}{{"ownership_return", true}, {"keep_sale", false}, {"physical_return", false}, {"defer", false}} {
		t.Run(tc.choice, func(t *testing.T) {
			c, cmd := ownershipReturnCommandFixture(t)
			cmd.Dispositions[0].Choice = tc.choice
			plan, err := Preview(c, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatal("ownership transfer ignored original sale disposition")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, task := range plan.Tasks {
				if task.Kind == "customer_stock" {
					t.Fatal("same physical transfer produced a duplicate warehouse task")
				}
			}
			if plan.CreditCents != 9000 {
				t.Fatalf("wrong original sale credit: %+v", plan)
			}
		})
	}
}
func TestOwnershipReturnRejectsAnotherReceiptAndSharedBatchOveruse(t *testing.T) {
	c, cmd := ownershipReturnCommandFixture(t)
	c.ReturnSources[0].EntitlementID = 12
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("another sale's receipt was accepted")
	}
	c, cmd = ownershipReturnCommandFixture(t)
	c.DeliveryMode = "ship"
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("normal shipment silently became an ownership transfer")
	}
	c, cmd = ownershipReturnCommandFixture(t)
	c.Lines[0].Delivered = 4
	c.ReturnSources[0].AvailableQuantity = 2
	s := c.ReturnSources[0]
	s.ID = 102
	c.ReturnSources = append(c.ReturnSources, s)
	cmd.ReturnSelections[0].Quantity = 2
	choice := cmd.ReturnSelections[0]
	choice.SourceID = 102
	cmd.ReturnSelections = append(cmd.ReturnSelections, choice)
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("two receipts reused the same free batch quantity")
	}
}
