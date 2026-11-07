package orderresolution

import "testing"

func TestFeeOnlyCreditPreservesClosedSharedFulfillment(t *testing.T) {
	c, cmd := fixture()
	c.Status = "closed"
	for i := range c.Lines {
		c.Lines[i].Closed = c.Lines[i].Quantity
		c.Lines[i].ClosedCents = c.Lines[i].AmountCents
	}
	c.CreditCents = c.TotalCents
	c.Dependencies = []Dependency{{Kind: "production", ID: 19, Label: "shared", Status: "completed", Shared: true}, {Kind: "fee", ID: 101, AmountCents: 500, PaidCents: 1000, SettledOutCents: 500}}
	cmd.Action, cmd.Scope, cmd.Timing = "credit", "fees_only", "settle_first"
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: 101, Choice: "waive"}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "closed" || len(p.Lines) != 0 || p.CreditCents != 0 || len(p.Fees) != 1 || p.Fees[0].CreditCents != 500 {
		t.Fatalf("fee correction changed fulfillment: %+v", p)
	}
	if !planHasTaskSource(p, "funds", 101) || planHasTask(p, "production") {
		t.Fatalf("incorrect obligations: %+v", p)
	}
	cmd.Dispositions = append(cmd.Dispositions, Disposition{Kind: "production", ID: 19, Choice: "keep_company_stock"})
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("fee-only scope accepted a production disposition")
	}
	cmd.Dispositions = cmd.Dispositions[:1]
	cmd.Action = "close"
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("closure accepted fee-only scope")
	}
}

func TestLegacySettledFeeDoesNotInventReceipt(t *testing.T) {
	c, cmd := fixture()
	c.Dependencies = []Dependency{{Kind: "fee", ID: 101, Status: "settled", CashIssue: "历史结算收款待核对", AmountCents: 1000}}
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: 101, Choice: "waive"}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("settled-only fee cannot establish available cash for a credit")
	}
	cmd.Dispositions[0].Choice = "keep"
	if _, err := Preview(c, cmd); err != nil {
		t.Fatalf("keeping unknown receipt must remain possible: %v", err)
	}
}

func TestFundsPreviewKeepsFeeAndMerchandiseBudgetsSeparate(t *testing.T) {
	c, cmd := fixture()
	c.PaidCents = c.TotalCents
	c.Dependencies = []Dependency{{Kind: "fee", ID: 101, AmountCents: 1000, PaidCents: 1000}}
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: 101, Choice: "waive"}, {Kind: "funds", ID: 1, Choice: "refund", AmountCents: c.TotalCents}, {Kind: "funds", ID: 2, SourceFeeID: 101, Choice: "customer_credit", AmountCents: 1000}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !planHasTaskSource(p, "funds", 0) || !planHasTaskSource(p, "funds", 101) {
		t.Fatalf("missing separate funding sources: %+v", p)
	}
	cmd.Dispositions[2].AmountCents = 1001
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("fee borrowed merchandise cash")
	}
	cmd.Dispositions[2].AmountCents = 1000
	cmd.Dispositions[2].SourceFeeID = 102
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("unknown fee source accepted")
	}
}

func TestExistingNegativeFeeAdjustmentCanBeKeptWithoutWaivingAgain(t *testing.T) {
	c, cmd := fixture()
	c.Dependencies = []Dependency{{Kind: "fee", ID: 102, AmountCents: -500, Label: "原运费冲减"}}
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: 102, Choice: "keep"}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Fees) != 1 || p.Fees[0].CreditCents != 0 || planHasTaskSource(p, "funds", 102) {
		t.Fatalf("negative historical fee changed: %+v", p)
	}
	cmd.Dispositions[0].Choice = "waive"
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("negative adjustment waived again")
	}
}
