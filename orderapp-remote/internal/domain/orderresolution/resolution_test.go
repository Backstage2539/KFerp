package orderresolution

import (
	"strings"
	"testing"
)

func fixture() (Context, Command) {
	c := Context{OrderID: 150, OrderNo: "SO-20261031-0003", Status: "open", TotalCents: 216000, Lines: []Line{{ID: 1, Quantity: 24, AmountCents: 72000}, {ID: 2, Quantity: 24, AmountCents: 144000}}}
	cmd := Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-06"}
	return c, cmd
}

func TestUnavailableCapabilitiesCannotBeEnabledByPolicyOrSubmitted(t *testing.T) {
	c, cmd := fixture()
	c.Unavailable = map[string]string{"production:stop": "停止在制流程尚未开放"}
	c.Dependencies = []Dependency{{Kind: "production", ID: 19, Status: "running"}}
	cmd.Dispositions = []Disposition{{Kind: "production", ID: 19, Choice: "stop"}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("unavailable production choice accepted")
	}
	for _, o := range OptionsFor(c)["production"] {
		if o.Value == "stop" && (!o.Disabled || o.Reason == "") {
			t.Fatalf("unavailable option not explained: %+v", o)
		}
	}
}

func TestExplicitChoicesRequired(t *testing.T) {
	for _, field := range []string{"action", "reason", "scope", "timing", "external", "period", "date"} {
		t.Run(field, func(t *testing.T) {
			c, cmd := fixture()
			switch field {
			case "action":
				cmd.Action = ""
			case "reason":
				cmd.Reason = ""
			case "scope":
				cmd.Scope = ""
			case "timing":
				cmd.Timing = ""
			case "external":
				cmd.ExternalStatus = ""
			case "period":
				cmd.PostingMode = ""
			case "date":
				cmd.PostingDate = ""
			}
			if _, err := Preview(c, cmd); err == nil {
				t.Fatal("missing choice silently defaulted")
			}
		})
	}
}
func TestCloseSharedCompletedProductionKeepsTrace(t *testing.T) {
	c, cmd := fixture()
	c.Dependencies = []Dependency{{Kind: "production", ID: 19, Label: "PP-19", Status: "completed", Shared: true}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("production disposition must be explicitly selected")
	}
	cmd.Dispositions = []Disposition{{Kind: "production", ID: 19, Choice: "keep_company_stock"}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.CreditCents != 216000 || len(p.Lines) != 2 || p.Status != "closed" {
		t.Fatalf("preview=%+v", p)
	}
	if c.Lines[0].Quantity != 24 || c.Dependencies[0].Status != "completed" {
		t.Fatal("preview mutated history")
	}
	cmd.Action = "cancel"
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("completed production cannot disappear through cancel")
	}
}
func TestPartialCloseAndRounding(t *testing.T) {
	c, cmd := fixture()
	c.Lines = []Line{{ID: 1, Quantity: 3, Delivered: 1, AmountCents: 100}}
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 1}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.CreditCents != 33 || p.Status != "partially_closed" {
		t.Fatalf("preview=%+v", p)
	}
	c.Lines[0].Closed = 1
	c.Lines[0].ClosedCents = 33
	p, err = Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.CreditCents != 34 {
		t.Fatalf("last cancellation cent=%d", p.CreditCents)
	}
	cmd.Lines[0].Quantity = 3
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("closed delivered quantity")
	}
}
func TestPaidCloseDoesNotInventRefund(t *testing.T) {
	c, cmd := fixture()
	c.PaidCents = 216000
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.RefundCents != 0 || !hasTask(p, "funds") {
		t.Fatalf("must leave money as pending disposition: %+v", p)
	}
}
func TestReopenDoesNotUndoFinancialHistory(t *testing.T) {
	c, cmd := fixture()
	c.Lines[0].Closed = 12
	c.Lines[0].ClosedCents = 36000
	c.CreditUsedCents = 1000
	cmd.Action = "reopen"
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 6}}
	p, err := Preview(c, cmd)
	if err != nil || p.CreditCents != -18000 || p.Status != "partially_closed" || p.RefundCents != 0 || hasTask(p, "funds") {
		t.Fatalf("reopen must restore only selected obligation and preserve settled money: %+v %v", p, err)
	}
}

func TestPartPaidPartialCloseHasNoRefundWhenPaymentStillOwed(t *testing.T) {
	c, cmd := fixture()
	c.PaidCents = 10000
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 1}}
	p, err := Preview(c, cmd)
	if err != nil || hasTask(p, "funds") {
		t.Fatalf("no refundable balance but funds task created: %+v %v", p, err)
	}
}

func TestCustomerOwnershipReturnCannotBeHiddenInRemainingClosure(t *testing.T) {
	c, cmd := fixture()
	c.Lines[0].Delivered = 24
	c.Dependencies = []Dependency{{Kind: "customer_stock", ID: 5, Quantity: 24}}
	cmd.Dispositions = []Disposition{{Kind: "customer_stock", ID: 5, Choice: "ownership_return"}}
	if _, err := Preview(c, cmd); err == nil || !strings.Contains(err.Error(), "退货") {
		t.Fatalf("remaining closure implicitly returned delivered property: %v", err)
	}
}

func TestWholeOrderCreditCannotExceedFrozenReceivable(t *testing.T) {
	c, cmd := fixture()
	c.TotalCents = 100000
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("credit exceeded frozen net receivable")
	}
}
func TestRetainedReceiptIsFulfilledNotFuturePromise(t *testing.T) {
	c, cmd := fixture()
	c.Lines[0].Delivered = 24
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lines) != 1 || p.CreditCents != 144000 {
		t.Fatalf("received customer property cancelled: %+v", p)
	}
}
func TestUnknownOrDuplicateSelectionsRejected(t *testing.T) {
	for _, s := range [][]Selection{{{ItemID: 99, Quantity: 1}}, {{ItemID: 1, Quantity: 1}, {ItemID: 1, Quantity: 1}}, {{ItemID: 1, Quantity: -1}}} {
		c, cmd := fixture()
		cmd.Scope = "selected"
		cmd.Lines = s
		if _, err := Preview(c, cmd); err == nil {
			t.Fatalf("accepted %v", s)
		}
	}
}
func hasTask(p Plan, k string) bool {
	for _, v := range p.Tasks {
		if v.Kind == k {
			return true
		}
	}
	return false
}

func TestCancelCannotErasePartialDelivery(t *testing.T) {
	c, cmd := fixture()
	c.Lines[0].Delivered = 1
	cmd.Action = "cancel"
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("cancel erased partial delivery")
	}
}
func TestUnfinishedProductionCannotBeKeptAsAlreadyFinishedStock(t *testing.T) {
	c, cmd := fixture()
	c.Dependencies = []Dependency{{Kind: "production", ID: 1, Status: "submitted", Label: "PP-1"}}
	cmd.Dispositions = []Disposition{{Kind: "production", ID: 1, Choice: "keep_company_stock"}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("unfinished production silently continued")
	}
}

func TestPolicyCanRestrictAndRequireApprovalWithoutRelaxingFacts(t *testing.T) {
	c, cmd := fixture()
	c.Policy = Policy{Disabled: map[string][]string{"action": {"close"}}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("disabled action bypassed")
	}
	c.Policy = Policy{Approvals: map[string]string{"close": "finance.resolve"}}
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTask(p, "approval") {
		t.Fatal("configured approval missing")
	}
	cmd.Scope = "selected"
	cmd.Lines = []Selection{{ItemID: 1, Quantity: 25}}
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("policy bypassed quantity cap")
	}
}

func TestFundsPreviewConsolidatesSplitsAndCapsTotal(t *testing.T) {
	c, cmd := fixture()
	c.PaidCents = c.TotalCents
	cmd.Dispositions = append(cmd.Dispositions, Disposition{Kind: "funds", ID: 1, Choice: "refund", AmountCents: 100}, Disposition{Kind: "funds", ID: 2, Choice: "customer_credit", AmountCents: 200})
	p, err := Preview(c, cmd)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, task := range p.Tasks {
		if task.Kind == "funds" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("split funds created %d independently completable tasks", count)
	}
	cmd.Dispositions[0].AmountCents = c.TotalCents + 1
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("allocated more than refundable balance")
	}
}

func TestHeaderFreightRequiresExplicitDispositionAndPreservesResidualReceivable(t *testing.T) {
	c, cmd := fixture()
	c.TotalCents += 500
	c.Dependencies = []Dependency{{Kind: "fee", ID: -1, Label: "运费", AmountCents: 500}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("freight disposition chosen implicitly")
	}
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: -1, Choice: "keep"}}
	p, err := Preview(c, cmd)
	if err != nil || p.CreditCents != 216000 {
		t.Fatalf("keep freight: %+v %v", p, err)
	}
	cmd.Action = "cancel"
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("cancel left unpaid freight hidden on cancelled order")
	}
	cmd.Dispositions[0].Choice = "waive"
	p, err = Preview(c, cmd)
	if err != nil || p.CreditCents != 216500 {
		t.Fatalf("waive freight: %+v %v", p, err)
	}
	cmd.Action = "close"
	cmd.Dispositions[0].Choice = "partial_waiver"
	cmd.Dispositions[0].AmountCents = 201
	p, err = Preview(c, cmd)
	if err != nil || p.CreditCents != 216201 {
		t.Fatalf("partial waiver: %+v %v", p, err)
	}
	cmd.Dispositions[0].AmountCents = 501
	if _, err = Preview(c, cmd); err == nil {
		t.Fatal("fee reduction exceeded original fee")
	}
}

func TestNegotiatedFeeRequiresExplicitAmountEvenForZero(t *testing.T) {
	c, cmd := fixture()
	c.TotalCents += 100
	c.Dependencies = []Dependency{{Kind: "fee", ID: -1, AmountCents: 100}}
	cmd.Dispositions = []Disposition{{Kind: "fee", ID: -1, Choice: "negotiated"}}
	if _, err := Preview(c, cmd); err == nil {
		t.Fatal("omitted negotiation amount waived fee")
	}
}
