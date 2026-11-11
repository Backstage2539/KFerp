package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"sync"
	"testing"
)

func TestOrderResolutionPaidIndependentFeeKeepsCashAndSplitsSettlement(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total)VALUES(78401,'PAID-FEE-SOURCE',501,'2026-10-01',90,90),(78402,'PAID-FEE-TARGET',501,'2026-10-01',30,30);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78401,78401,1,911,'coffee','1000g',3,'袋',30,90),(78402,78402,1,911,'coffee','1000g',1,'袋',30,30);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true;INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78401,501,'order',78401,'direct_ship_service',10,'2026-10-01')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	billing := NewRepository(pool, schema)
	pc := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: 78401}}}
	preview, err := billing.PreviewBilling(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "paid-fee-bill"})
	if err != nil {
		t.Fatal(err)
	}
	bill, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "isolated transfer", IdempotencyKey: "paid-fee-cash", Actor: "finance"})
	if err != nil {
		t.Fatal(err)
	}
	var receiptBefore string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM %s.customer_settlement_payments p WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&receiptBefore); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "paid-fee-close", Dispositions: []domain.Disposition{{Kind: "fee", ID: 78401, Choice: "partial_waiver", AmountCents: 500}}}
	result, err := r.Submit(ctx, 78401, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var funds int64
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			if step.SourceID != 78401 {
				t.Fatalf("fee cash lost source: %+v", step)
			}
			funds = step.ID
			continue
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对五元费用减免", Reference: "fee-credit-proof", IdempotencyKey: fmt.Sprint("fee-confirm-", step.ID)}, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	if funds == 0 {
		t.Fatal("paid fee credit has no explicit funds disposition")
	}
	excessive := resolutionpg.StepCommand{Choice: "refund", AmountCents: 501, Account: "test bank", Reference: "too-much", PostingDate: "2026-11-07", Note: "test", IdempotencyKey: "fee-excess"}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, excessive, "finance"); err == nil {
		t.Fatal("fee refund exceeded confirmed available cash")
	}
	for _, payment := range []resolutionpg.StepCommand{
		{Choice: "refund", AmountCents: 200, Account: "test bank", Reference: "fee-refund", PostingDate: "2026-11-07", Note: "实际退款核对", IdempotencyKey: "fee-refund"},
		{Choice: "customer_credit", AmountCents: 200, PostingDate: "2026-11-07", Note: "客户要求保留", IdempotencyKey: "fee-balance"},
		{Choice: "offset", AmountCents: 100, TargetID: 78402, PostingDate: "2026-11-07", Note: "客户指定抵扣", IdempotencyKey: "fee-offset"},
	} {
		result, err = r.CompleteStep(ctx, 78401, result.ID, funds, payment, "finance")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, payment, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	if result.Status != "completed" {
		t.Fatalf("fee clearing remains pending: %+v", result)
	}
	var original, credit, orderOut, targetOffset int64
	var receiptAfter string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(f.amount*100)::bigint,f.resolution_credit_cents,(SELECT resolution_settled_out_cents FROM %[1]s.orders WHERE id=78401),(SELECT resolution_offset_cents FROM %[1]s.orders WHERE id=78402),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM %[1]s.customer_settlement_payments p WHERE settlement_batch_id=f.settlement_batch_id) FROM %[1]s.customer_fee_items f WHERE id=78401`, schema)).Scan(&original, &credit, &orderOut, &targetOffset, &receiptAfter); err != nil {
		t.Fatal(err)
	}
	if original != 1000 || credit != 500 || orderOut != 0 || targetOffset != 100 || receiptAfter != receiptBefore {
		t.Fatalf("cash/history/source mismatch original=%d credit=%d orderOut=%d target=%d historySame=%v", original, credit, orderOut, targetOffset, receiptAfter == receiptBefore)
	}
	balances, err := r.Balances(ctx, 78401)
	if err != nil || len(balances) != 1 || balances[0].AvailableCents != 200 {
		t.Fatalf("fee customer balance: %+v %v", balances, err)
	}

	current, err := r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	if current.PaidCents != 0 || current.CreditUsedCents != 0 {
		t.Fatal("independent fee cash contaminated merchandise balance")
	}
	for _, dep := range current.Dependencies {
		if dep.Kind == "fee" && dep.ID == 78401 && (dep.PaidCents != 1000 || dep.SettledOutCents != 500 || dep.AmountCents != 500) {
			t.Fatalf("fee receipt trace changed: %+v", dep)
		}
	}
	bill, err = billing.GetIssuedBill(ctx, 501, bill.BatchID)
	if err != nil || bill.TotalAmountCents != 1000 || bill.DueAmountCents != 0 {
		t.Fatalf("bill=%+v err=%v", bill, err)
	}

	if len(bill.Lines) != 1 || bill.Lines[0].ReceivedCents != 500 {
		t.Fatalf("bill retained fee settlement must be 500: %+v", bill.Lines)
	}

	account, err := billing.CustomerAccount(ctx, app.AccountQuery{CustomerID: 501, OrderID: 78401})
	if err != nil {
		t.Fatal(err)
	}
	if len(account.Fees) != 1 {
		t.Fatalf("fee account history missing: %+v", account.Fees)
	}
	raw, _ := json.Marshal(account.Fees[0])
	var feeAmounts map[string]any
	_ = json.Unmarshal(raw, &feeAmounts)
	if feeAmounts["original_amount_cents"] != float64(1000) || feeAmounts["adjustment_cents"] != float64(500) || feeAmounts["cash_received_cents"] != float64(1000) || feeAmounts["settled_out_cents"] != float64(500) {
		t.Fatalf("account fee cash and credit are not separate: %s", raw)
	}
	if bill.PaymentReversible {
		t.Fatal("bill UI still offers reversal of fee cash already disposed")
	}
	_, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "reverse", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, IdempotencyKey: "reverse-consumed-fee-cash", Actor: "finance"})
	if err == nil {
		t.Fatal("fee cash already refunded could be reversed again")
	}
	// A later fee negotiation is independent of closed merchandise fulfillment.
	current, err = r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Action, cmd.Scope, cmd.Timing = "credit", "fees_only", "settle_first"
	cmd.Version, cmd.IdempotencyKey = current.Version, "paid-fee-second-correction"
	cmd.Dispositions = []domain.Disposition{{Kind: "fee", ID: 78401, Choice: "waive"}}
	result, err = r.Submit(ctx, 78401, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			funds = step.ID
			continue
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对剩余费用全免", Reference: "second-credit", IdempotencyKey: fmt.Sprint("second-", step.ID)}, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	refund := resolutionpg.StepCommand{Choice: "refund", AmountCents: 300, Account: "test bank", Reference: "second-fee-refund-300", PostingDate: "2026-11-07", Note: "客户取消，核对退款", IdempotencyKey: "second-fee-refund-300"}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_fee_audit() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN IF NEW.action='resolution_step' THEN RAISE EXCEPTION 'isolated fee audit failure';END IF;RETURN NEW;END $f$;CREATE TRIGGER reject_fee_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_fee_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, refund, "finance"); err == nil {
		t.Fatal("fee refund ignored failing audit")
	}
	var outgoing int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_settled_out_cents FROM %s.customer_fee_items WHERE id=78401`, schema)).Scan(&outgoing); err != nil || outgoing != 500 {
		t.Fatalf("failed refund changed cash: %d %v", outgoing, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_fee_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, refund, "finance"); err != nil {
		t.Fatalf("same-key failed refund could not retry: %v", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			attempt := refund
			attempt.AmountCents = 200
			attempt.Reference = fmt.Sprint("last-fee-refund-", n)
			attempt.IdempotencyKey = attempt.Reference
			_, e := r.CompleteStep(ctx, 78401, result.ID, funds, attempt, "finance")
			results <- e
		}(n)
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded := 0
	for e := range results {
		if e == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("remaining fee cash paid %d times", succeeded)
	}
	var status string
	var closed float64
	var creditAfter int64
	var paused bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.resolution_status,o.fulfillment_paused,i.resolution_closed_qty,f.resolution_credit_cents,f.resolution_settled_out_cents FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id JOIN %[1]s.customer_fee_items f ON f.source_id=o.id WHERE o.id=78401`, schema)).Scan(&status, &paused, &closed, &creditAfter, &outgoing); err != nil {
		t.Fatal(err)
	}
	if status != "closed" || paused || closed != 3 || creditAfter != 1000 || outgoing != 1000 {
		t.Fatalf("fee-only final state: %s paused=%v closed=%v credit=%d outgoing=%d", status, paused, closed, creditAfter, outgoing)
	}

}

func TestOrderResolutionMerchandiseAndFeeCashUseSameBankVoucherWithoutMixingSources(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total,prepayment_amount,payment_goods_amount)VALUES(78421,'TWO-CASH-SOURCES',501,'2026-10-01',90,90,90,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78421,78421,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true;INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78421,501,'order',78421,'direct_ship_service',10,'2026-10-01')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	billing := NewRepository(pool, schema)
	pc := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: 78421}}}
	preview, err := billing.PreviewBilling(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "multi-fee-bill"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "test bank", IdempotencyKey: "multi-fee-cash", Actor: "finance"}); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "isolated-finance")
			return next(c)
		}
	})
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(path string, body any) (resolutionpg.Result, error) {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var out resolutionpg.Result
		if rec.Code != 200 {
			return out, fmt.Errorf("HTTP %d: %s", rec.Code, rec.Body.String())
		}
		err := json.Unmarshal(rec.Body.Bytes(), &out)
		return out, err
	}
	c, err := r.Context(ctx, 78421)
	if err != nil {
		t.Fatal(err)
	}
	result, err := call("/api/orders/78421/resolution/submit", domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "multi-source-close", Dispositions: []domain.Disposition{{Kind: "fee", ID: 78421, Choice: "waive"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			continue
		}
		if _, err = call(fmt.Sprintf("/api/orders/78421/resolution/%d/steps/%d", result.ID, step.ID), resolutionpg.StepCommand{Choice: step.Choice, Note: "核对费用全免", Reference: "multi-credit", IdempotencyKey: fmt.Sprint("multi-credit-", step.ID)}); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, step := range result.Steps {
		if step.Kind != "funds" {
			continue
		}
		count++
		amount := int64(9000)
		if step.SourceID == 78421 {
			amount = 1000
		}
		_, err = call(fmt.Sprintf("/api/orders/78421/resolution/%d/steps/%d", result.ID, step.ID), resolutionpg.StepCommand{Choice: "refund", AmountCents: amount, Account: "test bank", Reference: "combined-bank-transfer", PostingDate: "2026-11-07", Note: "一笔退款分别核对商品与费用来源", IdempotencyKey: fmt.Sprint("multi-refund-", step.ID)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 2 {
		t.Fatalf("cash sources coalesced: %+v", result.Steps)
	}
	var goods, fee int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.resolution_settled_out_cents,f.resolution_settled_out_cents FROM %[1]s.orders o JOIN %[1]s.customer_fee_items f ON f.source_id=o.id WHERE o.id=78421`, schema)).Scan(&goods, &fee); err != nil || goods != 9000 || fee != 1000 {
		t.Fatalf("goods=%d fee=%d err=%v", goods, fee, err)
	}
}

func TestOrderResolutionPartiallyThenFullyPaidFeeKeepsCashAndSplitsSettlement(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total)VALUES(78401,'PAID-FEE-SOURCE',501,'2026-10-01',90,90),(78402,'PAID-FEE-TARGET',501,'2026-10-01',30,30);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78401,78401,1,911,'coffee','1000g',3,'袋',30,90),(78402,78402,1,911,'coffee','1000g',1,'袋',30,30);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true;INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78401,501,'order',78401,'direct_ship_service',10,'2026-10-01')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	billing := NewRepository(pool, schema)
	pc := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: 78401}}}
	preview, err := billing.PreviewBilling(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "paid-fee-bill"})
	if err != nil {
		t.Fatal(err)
	}
	partialAmount := int64(400)
	bill, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "isolated transfer", IdempotencyKey: "paid-fee-partial", Actor: "finance", ActualCents: &partialAmount, Allocations: []app.BillingPaymentLineAllocation{{LineID: bill.Lines[0].ID, AmountCents: partialAmount}}})
	if err != nil {
		t.Fatal(err)
	}
	bill, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "isolated transfer", IdempotencyKey: "paid-fee-cash", Actor: "finance"})
	if err != nil {
		t.Fatal(err)
	}
	var receiptBefore string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM %s.customer_settlement_payments p WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&receiptBefore); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "paid-fee-close", Dispositions: []domain.Disposition{{Kind: "fee", ID: 78401, Choice: "partial_waiver", AmountCents: 500}}}
	result, err := r.Submit(ctx, 78401, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var funds int64
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			if step.SourceID != 78401 {
				t.Fatalf("fee cash lost source: %+v", step)
			}
			funds = step.ID
			continue
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对五元费用减免", Reference: "fee-credit-proof", IdempotencyKey: fmt.Sprint("fee-confirm-", step.ID)}, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	if funds == 0 {
		t.Fatal("paid fee credit has no explicit funds disposition")
	}
	excessive := resolutionpg.StepCommand{Choice: "refund", AmountCents: 501, Account: "test bank", Reference: "too-much", PostingDate: "2026-11-07", Note: "test", IdempotencyKey: "fee-excess"}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, excessive, "finance"); err == nil {
		t.Fatal("fee refund exceeded confirmed available cash")
	}
	for _, payment := range []resolutionpg.StepCommand{
		{Choice: "refund", AmountCents: 200, Account: "test bank", Reference: "fee-refund", PostingDate: "2026-11-07", Note: "实际退款核对", IdempotencyKey: "fee-refund"},
		{Choice: "customer_credit", AmountCents: 200, PostingDate: "2026-11-07", Note: "客户要求保留", IdempotencyKey: "fee-balance"},
		{Choice: "offset", AmountCents: 100, TargetID: 78402, PostingDate: "2026-11-07", Note: "客户指定抵扣", IdempotencyKey: "fee-offset"},
	} {
		result, err = r.CompleteStep(ctx, 78401, result.ID, funds, payment, "finance")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, payment, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	if result.Status != "completed" {
		t.Fatalf("fee clearing remains pending: %+v", result)
	}
	var original, credit, orderOut, targetOffset int64
	var receiptAfter string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(f.amount*100)::bigint,f.resolution_credit_cents,(SELECT resolution_settled_out_cents FROM %[1]s.orders WHERE id=78401),(SELECT resolution_offset_cents FROM %[1]s.orders WHERE id=78402),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM %[1]s.customer_settlement_payments p WHERE settlement_batch_id=f.settlement_batch_id) FROM %[1]s.customer_fee_items f WHERE id=78401`, schema)).Scan(&original, &credit, &orderOut, &targetOffset, &receiptAfter); err != nil {
		t.Fatal(err)
	}
	if original != 1000 || credit != 500 || orderOut != 0 || targetOffset != 100 || receiptAfter != receiptBefore {
		t.Fatalf("cash/history/source mismatch original=%d credit=%d orderOut=%d target=%d historySame=%v", original, credit, orderOut, targetOffset, receiptAfter == receiptBefore)
	}
	balances, err := r.Balances(ctx, 78401)
	if err != nil || len(balances) != 1 || balances[0].AvailableCents != 200 {
		t.Fatalf("fee customer balance: %+v %v", balances, err)
	}

	current, err := r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	if current.PaidCents != 0 || current.CreditUsedCents != 0 {
		t.Fatal("independent fee cash contaminated merchandise balance")
	}
	for _, dep := range current.Dependencies {
		if dep.Kind == "fee" && dep.ID == 78401 && (dep.PaidCents != 1000 || dep.SettledOutCents != 500 || dep.AmountCents != 500) {
			t.Fatalf("fee receipt trace changed: %+v", dep)
		}
	}
	bill, err = billing.GetIssuedBill(ctx, 501, bill.BatchID)
	if err != nil || bill.TotalAmountCents != 1000 || bill.DueAmountCents != 0 {
		t.Fatalf("bill=%+v err=%v", bill, err)
	}

	if len(bill.Lines) != 1 || bill.Lines[0].ReceivedCents != 500 {
		t.Fatalf("bill retained fee settlement must be 500: %+v", bill.Lines)
	}

	account, err := billing.CustomerAccount(ctx, app.AccountQuery{CustomerID: 501, OrderID: 78401})
	if err != nil {
		t.Fatal(err)
	}
	if len(account.Fees) != 1 {
		t.Fatalf("fee account history missing: %+v", account.Fees)
	}
	raw, _ := json.Marshal(account.Fees[0])
	var feeAmounts map[string]any
	_ = json.Unmarshal(raw, &feeAmounts)
	if feeAmounts["original_amount_cents"] != float64(1000) || feeAmounts["adjustment_cents"] != float64(500) || feeAmounts["cash_received_cents"] != float64(1000) || feeAmounts["settled_out_cents"] != float64(500) {
		t.Fatalf("account fee cash and credit are not separate: %s", raw)
	}
	if bill.PaymentReversible {
		t.Fatal("bill UI still offers reversal of fee cash already disposed")
	}
	_, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "reverse", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, IdempotencyKey: "reverse-consumed-fee-cash", Actor: "finance"})
	if err == nil {
		t.Fatal("fee cash already refunded could be reversed again")
	}
	// A later fee negotiation is independent of closed merchandise fulfillment.
	current, err = r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Action, cmd.Scope, cmd.Timing = "credit", "fees_only", "settle_first"
	cmd.Version, cmd.IdempotencyKey = current.Version, "paid-fee-second-correction"
	cmd.Dispositions = []domain.Disposition{{Kind: "fee", ID: 78401, Choice: "waive"}}
	result, err = r.Submit(ctx, 78401, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			funds = step.ID
			continue
		}
		if _, err = r.CompleteStep(ctx, 78401, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对剩余费用全免", Reference: "second-credit", IdempotencyKey: fmt.Sprint("second-", step.ID)}, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	refund := resolutionpg.StepCommand{Choice: "refund", AmountCents: 300, Account: "test bank", Reference: "second-fee-refund-300", PostingDate: "2026-11-07", Note: "客户取消，核对退款", IdempotencyKey: "second-fee-refund-300"}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_fee_audit() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN IF NEW.action='resolution_step' THEN RAISE EXCEPTION 'isolated fee audit failure';END IF;RETURN NEW;END $f$;CREATE TRIGGER reject_fee_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_fee_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, refund, "finance"); err == nil {
		t.Fatal("fee refund ignored failing audit")
	}
	var outgoing int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_settled_out_cents FROM %s.customer_fee_items WHERE id=78401`, schema)).Scan(&outgoing); err != nil || outgoing != 500 {
		t.Fatalf("failed refund changed cash: %d %v", outgoing, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_fee_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78401, result.ID, funds, refund, "finance"); err != nil {
		t.Fatalf("same-key failed refund could not retry: %v", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			attempt := refund
			attempt.AmountCents = 200
			attempt.Reference = fmt.Sprint("last-fee-refund-", n)
			attempt.IdempotencyKey = attempt.Reference
			_, e := r.CompleteStep(ctx, 78401, result.ID, funds, attempt, "finance")
			results <- e
		}(n)
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded := 0
	for e := range results {
		if e == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("remaining fee cash paid %d times", succeeded)
	}
	var status string
	var closed float64
	var creditAfter int64
	var paused bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.resolution_status,o.fulfillment_paused,i.resolution_closed_qty,f.resolution_credit_cents,f.resolution_settled_out_cents FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id JOIN %[1]s.customer_fee_items f ON f.source_id=o.id WHERE o.id=78401`, schema)).Scan(&status, &paused, &closed, &creditAfter, &outgoing); err != nil {
		t.Fatal(err)
	}
	if status != "closed" || paused || closed != 3 || creditAfter != 1000 || outgoing != 1000 {
		t.Fatalf("fee-only final state: %s paused=%v closed=%v credit=%d outgoing=%d", status, paused, closed, creditAfter, outgoing)
	}

}

func TestOrderResolutionPartialThenFullMixedOrderAndFeeRefund(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total,prepayment_amount,payment_goods_amount)VALUES(78421,'TWO-CASH-SOURCES',501,'2026-10-01',90,90,0,0);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78421,78421,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true;INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78421,501,'order',78421,'direct_ship_service',10,'2026-10-01')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	billing := NewRepository(pool, schema)
	pc := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: 78421}, {ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: 78421}}}
	preview, err := billing.PreviewBilling(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "multi-fee-bill"})
	if err != nil {
		t.Fatal(err)
	}
	var orderLine int64
	for _, line := range bill.Lines {
		if line.SourceType == "order" && line.SourceID == 78421 {
			orderLine = line.ID
		}
	}
	if orderLine == 0 {
		t.Fatal("order line missing")
	}
	amount := int64(4000)
	bill, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "test bank", IdempotencyKey: "mixed-partial", Actor: "finance", ActualCents: &amount, Allocations: []app.BillingPaymentLineAllocation{{LineID: orderLine, AmountCents: amount}}})
	if err != nil {
		t.Fatal(err)
	}
	bill, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", ExpectedDueAmountCents: &bill.DueAmountCents, ExpectedPaymentRevision: &bill.PaymentRevision, PaymentMethod: "test bank", IdempotencyKey: "multi-fee-cash", Actor: "finance"})
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "isolated-finance")
			return next(c)
		}
	})
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(path string, body any) (resolutionpg.Result, error) {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var out resolutionpg.Result
		if rec.Code != 200 {
			return out, fmt.Errorf("HTTP %d: %s", rec.Code, rec.Body.String())
		}
		err := json.Unmarshal(rec.Body.Bytes(), &out)
		return out, err
	}
	c, err := r.Context(ctx, 78421)
	if err != nil {
		t.Fatal(err)
	}
	result, err := call("/api/orders/78421/resolution/submit", domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "multi-source-close", Dispositions: []domain.Disposition{{Kind: "fee", ID: 78421, Choice: "waive"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "funds" {
			continue
		}
		if _, err = call(fmt.Sprintf("/api/orders/78421/resolution/%d/steps/%d", result.ID, step.ID), resolutionpg.StepCommand{Choice: step.Choice, Note: "核对费用全免", Reference: "multi-credit", IdempotencyKey: fmt.Sprint("multi-credit-", step.ID)}); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, step := range result.Steps {
		if step.Kind != "funds" {
			continue
		}
		count++
		amount := int64(9000)
		if step.SourceID == 78421 {
			amount = 1000
		}
		_, err = call(fmt.Sprintf("/api/orders/78421/resolution/%d/steps/%d", result.ID, step.ID), resolutionpg.StepCommand{Choice: "refund", AmountCents: amount, Account: "test bank", Reference: "combined-bank-transfer", PostingDate: "2026-11-07", Note: "一笔退款分别核对商品与费用来源", IdempotencyKey: fmt.Sprint("multi-refund-", step.ID)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 2 {
		t.Fatalf("cash sources coalesced: %+v", result.Steps)
	}
	var goods, fee int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.resolution_settled_out_cents,f.resolution_settled_out_cents FROM %[1]s.orders o JOIN %[1]s.customer_fee_items f ON f.source_id=o.id WHERE o.id=78421`, schema)).Scan(&goods, &fee); err != nil || goods != 9000 || fee != 1000 {
		t.Fatalf("goods=%d fee=%d err=%v", goods, fee, err)
	}
}
