package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
	"time"

	domain "orderapp/internal/domain/orderresolution"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func returnStepID(t *testing.T, result resolutionpg.Result, kind string) int64 {
	t.Helper()
	for _, step := range result.Steps {
		if step.Kind == kind && step.Status != "superseded" {
			return step.ID
		}
	}
	t.Fatalf("missing %s in %+v", kind, result.Steps)
	return 0
}
func TestOrderResolutionReturnCreditAndCashRemainSeparate(t *testing.T) {
	for _, paid := range []bool{false, true} {
		t.Run(fmt.Sprint(paid), func(t *testing.T) {
			pool, schema := orderReturnFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			if paid {
				if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=90,payment_goods_amount=90 WHERE id=78500`, schema)); err != nil {
					t.Fatal(err)
				}
			}
			current, err := r.Context(ctx, 78500)
			if err != nil {
				t.Fatal(err)
			}
			cmd := returnCommand(current.Version)
			cmd.ReturnSettlement = "credit_returned_goods"
			result, err := r.Submit(ctx, 78500, cmd, "sales")
			if err != nil {
				t.Fatal(err)
			}
			financeID, stockID := returnStepID(t, result, "return_finance"), returnStepID(t, result, "return_stock")
			fund := resolutionpg.StepCommand{Choice: "refund", AmountCents: 3000, Account: "明确选择的付款账户", Reference: "RETURN-CASH", PostingDate: "2026-11-08", Note: "人工核对实际付款凭据", IdempotencyKey: "refund-part"}
			if paid {
				if _, err = r.CompleteStep(ctx, 78500, result.ID, returnStepID(t, result, "funds"), fund, "finance"); err == nil {
					t.Fatal("cash executed before warehouse and finance confirmations")
				}
			}
			result, err = r.CompleteStep(ctx, 78500, result.ID, financeID, resolutionpg.StepCommand{Choice: "credit_returned_goods", Note: "按所选实物数量冲减商品款", IdempotencyKey: "return-credit"}, "finance")
			if err != nil {
				t.Fatal(err)
			}
			current, err = r.Context(ctx, 78500)
			if err != nil || current.CreditCents != 0 || current.Lines[0].Returned != 0 {
				t.Fatalf("confirmation prematurely posted credit or stock: %+v %v", current, err)
			}
			result, err = r.CompleteStep(ctx, 78500, result.ID, stockID, resolutionpg.StepCommand{Choice: "receive", Note: "实际验收", Reference: "GOODS-RETURN", PostingDate: "2026-11-08", IdempotencyKey: "stock"}, "warehouse")
			if err != nil {
				t.Fatal(err)
			}
			current, err = r.Context(ctx, 78500)
			if err != nil || !result.Applied || current.CreditCents != 9000 || current.CreditUsedCents != 0 || current.Lines[0].Returned != 3 {
				t.Fatalf("credit and receipt mismatch: %+v %+v %v", current, result, err)
			}
			if paid {
				fundsID := returnStepID(t, result, "funds")
				result, err = r.CompleteStep(ctx, 78500, result.ID, fundsID, fund, "finance")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = r.CompleteStep(ctx, 78500, result.ID, fundsID, fund, "finance"); err != nil {
					t.Fatal(err)
				}
				excessive := fund
				excessive.AmountCents = 6001
				excessive.IdempotencyKey = "excess"
				excessive.Reference = "EXCESS"
				if _, err = r.CompleteStep(ctx, 78500, result.ID, fundsID, excessive, "finance"); err == nil {
					t.Fatal("over-refund allowed")
				}
				result, err = r.CompleteStep(ctx, 78500, result.ID, fundsID, resolutionpg.StepCommand{Choice: "customer_credit", AmountCents: 6000, PostingDate: "2026-11-08", Note: "客户选择余款留存余额", IdempotencyKey: "retain-rest"}, "finance")
				if err != nil {
					t.Fatal(err)
				}
				var refunds, balances, received int64
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT sum(amount_cents) FROM %[1]s.order_resolution_financial_entries WHERE kind='refund'),(SELECT sum(amount_cents) FROM %[1]s.order_resolution_financial_entries WHERE kind='customer_credit'),round(payment_goods_amount*100)::bigint FROM %[1]s.orders WHERE id=78500`, schema)).Scan(&refunds, &balances, &received); err != nil || refunds != 3000 || balances != 6000 || received != 9000 {
					t.Fatalf("refund=%d balance=%d received=%d err=%v", refunds, balances, received, err)
				}
			}
			if result.Status != "completed" {
				t.Fatalf("not completed: %+v", result)
			}
		})
	}
}

func TestOrderResolutionReturnAndCloseRoundCumulatively(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders SET total_amount=.02,grand_total=.02 WHERE id=78500;UPDATE %[1]s.order_items SET qty=4,line_total=.02,unit_price=.005 WHERE id=78500`, schema)); err != nil {
		t.Fatal(err)
	}
	for n, qty := range []int64{1, 0, 2} {
		current, err := r.Context(ctx, 78500)
		if err != nil {
			t.Fatal(err)
		}
		cmd := returnCommand(current.Version)
		cmd.IdempotencyKey = fmt.Sprintf("round-%d", n)
		if qty == 0 {
			cmd.Action = "close"
			cmd.ReturnSettlement = ""
			cmd.ReturnSelections = nil
		} else {
			cmd.Scope = "selected"
			cmd.Lines = []domain.Selection{{ItemID: 78500, Quantity: float64(qty)}}
			cmd.ReturnSettlement = "credit_returned_goods"
			cmd.ReturnSelections[0].Quantity = qty
		}
		result, err := r.Submit(ctx, 78500, cmd, "sales")
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range result.Steps {
			c := resolutionpg.StepCommand{Choice: step.Choice, Note: "核对本次实物与金额", Reference: fmt.Sprint(n), PostingDate: "2026-11-08", IdempotencyKey: fmt.Sprint(step.ID)}
			result, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, c, "authorized-role")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	current, err := r.Context(ctx, 78500)
	if err != nil || current.CreditCents != 2 || current.Lines[0].Returned != 3 || current.Lines[0].Closed != 1 || current.Lines[0].ReturnCreditQuantity != 3 {
		t.Fatalf("rounded total or physical facts incorrect: %+v %v", current, err)
	}
}

func TestOrderResolutionReturnConcurrentWarehouseConfirmation(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func(n int) {
			<-start
			_, err := r.CompleteStep(ctx, 78500, result.ID, stockID, resolutionpg.StepCommand{Choice: "receive", Note: "并发核对同一次退货", Reference: "SAME-RECEIPT", PostingDate: "2026-11-08", IdempotencyKey: fmt.Sprintf("parallel-%d", n)}, "warehouse")
			results <- err
		}(n)
	}
	close(start)
	success := 0
	for n := 0; n < 2; n++ {
		if err := <-results; err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent confirmations succeeded %d times", success)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "status='posted'", 1)
	assertMiniDirectShipCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='order_resolution_return'", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_return_stock'", 1)
}

func TestOrderResolutionReturnRejectsClosedReceiptPeriodAndCanRetry(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_monthly_reports(month,status)VALUES('2026-10','closed')`, schema)); err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "核对收货日期", Reference: "DATE-RECEIPT", PostingDate: "2026-10-31", IdempotencyKey: "old-period"}
	if _, err = r.CompleteStep(ctx, 78500, result.ID, stockID, confirm, "warehouse"); err == nil {
		t.Fatal("closed month receipt allowed")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "true", 0)
	confirm.PostingDate = "2026-11-08"
	confirm.IdempotencyKey = "new-date"
	if _, err = r.CompleteStep(ctx, 78500, result.ID, stockID, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "posting_date='2026-11-08'", 1)
	if err = resolutionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("schema re-init destroyed posted return: %v", err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "status='posted'", 1)
}

func TestOrderResolutionReturnCannotReceiveBeforeRequiredApproval(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	if _, err := r.SavePolicy(ctx, 1, domain.Policy{Approvals: map[string]string{"return": "finance.resolve"}}, "admin"); err != nil {
		t.Fatal(err)
	}
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "实际退回核对", Reference: "APPROVAL-RETURN", PostingDate: "2026-11-08", IdempotencyKey: "approval-first"}
	if _, err = r.CompleteStep(ctx, 78500, result.ID, stockID, confirm, "warehouse"); err == nil {
		t.Fatal("irreversible stock receipt bypassed required approval")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "true", 0)
	if _, err = r.CompleteStep(ctx, 78500, result.ID, returnStepID(t, result, "approval"), resolutionpg.StepCommand{Choice: "approve", Note: "同意本次原批次退回方案", IdempotencyKey: "approve"}, "finance"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78500, result.ID, stockID, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
}
