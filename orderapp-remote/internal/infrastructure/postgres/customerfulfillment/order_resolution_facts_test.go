package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
	"sync"
	"testing"

	domain "orderapp/internal/domain/orderresolution"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func TestOrderResolutionZeroNetOrderDoesNotInventLegacyCash(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total,discount_amount,pay_status_id)VALUES(78280,'ZERO-NET',501,90,0,90,(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1));INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78280,78280,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78280)
	if err != nil {
		t.Fatal(err)
	}
	if c.TotalCents != 0 || c.PaidCents != 0 {
		t.Fatalf("discounted zero price became fictitious cash: %+v", c)
	}
	result, err := repo.Submit(ctx, 78280, domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "zero-net-close"}, "sales")
	if err != nil || len(result.Steps) != 0 {
		t.Fatalf("zero cash created refund obligation: %+v %v", result, err)
	}
}

func TestOrderResolutionLegacyAccountKeepsDeliveredReceivable(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total)VALUES(78281,'LEGACY-RECEIVABLE',501,90,0);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78281,78281,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.order_stock_deductions(order_id,product_id,spec_g,deducted_units,deducted_g,batch_code)VALUES(78281,911,1000,1,1000,'LEGACY-DELIVERED')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78281)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Submit(ctx, 78281, domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "legacy-close"}, "sales"); err != nil {
		t.Fatal(err)
	}
	account, err := NewRepository(pool, schema).CustomerAccount(ctx, app.AccountQuery{CustomerID: 501, OrderID: 78281})
	if err != nil || len(account.Rows) != 1 || account.Rows[0].TotalCents != 3000 || account.Rows[0].DueCents != 3000 {
		t.Fatalf("delivered receivable cleared: %+v %v", account.Rows, err)
	}
}

func TestOrderResolutionKeepsReceivedPurchaseHistoryAfterHoldAndClose(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code,total_amount,grand_total)VALUES(78260,'RETAINED-HISTORY',501,'customer_stock','DEV-E2E-A1',600,600);
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78260,78260,1,911,'coffee','1000g',20,'袋',30,600);
UPDATE %[1]s.customer_stock_entitlements SET received_qty=8 WHERE source_order_id=78260;`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	for _, action := range []string{"hold", "resume", "close"} {
		c, err := repo.Context(ctx, 78260)
		if err != nil {
			t.Fatal(err)
		}
		cmd := domain.Command{Action: action, Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: action + "-history"}
		if action == "close" {
			for _, d := range c.Dependencies {
				cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "keep_sale"})
			}
		}
		if _, err = repo.Submit(ctx, 78260, cmd, "sales"); err != nil {
			t.Fatal(err)
		}
		totals, err := customerstock.ListPurchaseTotals(ctx, pool, schema, 501, "DEV-E2E-A1")
		want := int64(20)
		if action == "close" {
			want = 8
		}
		if err != nil || len(totals) != 1 || totals[0].PurchasedQty != want || totals[0].ReceivedQty != 8 {
			t.Errorf("%s lost purchase/receipt history: %+v %v", action, totals, err)
		}
	}
}

func TestOrderResolutionClosedWorkspaceAndFrozenLineIdentity(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.customer_capability_templates(template_key,label,erp_permissions,erp_view_keys,capabilities_json,active)VALUES('resolution_home','订单处理测试','["customer_processing.read","customer_processing.submit"]','["customerProcessingPortal"]','[{"code":"direct_ship","enabled":true},{"code":"settlement","enabled":true}]',true);
INSERT INTO %[1]s.customer_portal_profiles(customer_id,enabled,capability_template_key)VALUES(501,true,'resolution_home')ON CONFLICT(customer_id)DO UPDATE SET enabled=true,capability_template_key='resolution_home';
INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total)VALUES(78250,'HOME-CLOSED',501,90,90),(78251,'HOME-OPEN',501,30,30);
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78250,78250,1,911,'coffee','1000g',3,'袋',30,90),(78251,78251,1,911,'coffee','1000g',1,'袋',30,30);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78250)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Submit(ctx, 78250, domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "home-close"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("workspace", func(t *testing.T) {
		home, err := NewRepository(pool, schema).CustomerWorkspace(ctx, 501, "home", 0)
		if err != nil {
			t.Fatal(err)
		}
		if home["orders"] != 2 || home["pending_shipment"] != 1 || home["missing_recipient"] != 1 || home["pending_settlement_amount"] != float64(30) {
			t.Fatalf("closed order counted as actionable: %+v", home)
		}
	})
	for _, change := range []string{"unit='kg'", "sales_unit='kg'", "unit_price=20", "logistics_unit_weight_g=500", "price_source_json='{\"changed\":true}'"} {
		t.Run(change, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.order_items SET %s WHERE id=78250", schema, change))
			if err == nil || !strings.Contains(err.Error(), "订单处理") {
				t.Fatalf("historical identity or frozen price edited: %s: %v", change, err)
			}
		})
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET order_date='2026-11-06' WHERE id=78250", schema)); err != nil {
		t.Fatalf("ordinary date edit blocked: %v", err)
	}
}

func TestOrderResolutionIssuedBillPreservesSnapshotAndAdjustsDue(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total,prepayment_amount,payment_goods_amount)VALUES(78204,'ISSUED-CLOSE',501,'2026-10-01',90,90,40,40);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78204,78204,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true`, schema))
	if err != nil {
		t.Fatal(err)
	}
	billing := NewRepository(pool, schema)
	previewCmd := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: 78204}}}
	preview, err := billing.PreviewBilling(ctx, previewCmd)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: previewCmd, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "issue-before-close"})
	if err != nil {
		t.Fatal(err)
	}
	var before string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(l)ORDER BY id)::text FROM %s.customer_settlement_lines l WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78204)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "selected", Lines: []domain.Selection{{ItemID: 78204, Quantity: 1}}, Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "close-issued"}
	result, err := repo.Submit(ctx, 78204, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 || result.Steps[0].Kind != "billing" {
		t.Fatalf("missing billing approval: %+v", result)
	}
	result, err = repo.CompleteStep(ctx, 78204, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "adjust", Note: "核对原账单并冲减剩余商品", Reference: "credit-evidence", IdempotencyKey: "confirm-bill"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := billing.GetIssuedBill(ctx, 501, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TotalAmountCents != 5000 || updated.DueAmountCents != 2000 || updated.ReceivableCents != 6000 {
		t.Fatalf("bill balances wrong: %+v", updated)
	}
	var after string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(l)ORDER BY id)::text FROM %s.customer_settlement_lines l WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&after); err != nil || before != after {
		t.Fatalf("issued snapshot rewritten: %v", err)
	}
	expected := int64(2000)
	revision := updated.PaymentRevision
	paid, err := billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", Actor: "finance", IdempotencyKey: "pay-current-due", ExpectedDueAmountCents: &expected, ExpectedPaymentRevision: &revision, PaymentMethod: "测试银行"})
	if err != nil {
		t.Fatal(err)
	}
	if paid.DueAmountCents != 0 {
		t.Fatalf("paid=%+v", paid)
	}
	var cash int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint FROM %s.orders WHERE id=78204`, schema)).Scan(&cash); err != nil || cash != 6000 {
		t.Fatalf("cash=%d err=%v", cash, err)
	}
	c, err = repo.Context(ctx, 78204)
	if err != nil || c.PaidCents != 6000 {
		t.Fatalf("adjusted paid status invented cash: %+v %v", c, err)
	}
	cmd.Version = c.Version
	cmd.IdempotencyKey = "close-one-more"
	result, err = repo.Submit(ctx, 78204, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var billingStep, fundsStep int64
	for _, s := range result.Steps {
		if s.Kind == "billing" {
			billingStep = s.ID
		}
		if s.Kind == "funds" {
			fundsStep = s.ID
		}
	}
	_, err = repo.CompleteStep(ctx, 78204, result.ID, billingStep, resolutionpg.StepCommand{Choice: "adjust", Reference: "second-credit", Note: "再关闭一袋", IdempotencyKey: "second-bill"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteStep(ctx, 78204, result.ID, fundsStep, resolutionpg.StepCommand{Choice: "refund", AmountCents: 3000, Account: "测试银行", Reference: "refund-30", PostingDate: "2026-11-07", Note: "登记实际退款", IdempotencyKey: "refund-after-bill"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	c, err = repo.Context(ctx, 78204)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Action = "reopen"
	cmd.Scope = "all"
	cmd.Lines = nil
	cmd.Version = c.Version
	cmd.IdempotencyKey = "reopen-after-refund"
	result, err = repo.Submit(ctx, 78204, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteStep(ctx, 78204, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "adjust", Reference: "reopen-debit", Note: "重新开启重新形成应收", IdempotencyKey: "reopen-bill"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	updated, err = billing.GetIssuedBill(ctx, 501, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DueAmountCents != 6000 {
		t.Fatalf("refunded cash still counted on reopening: %+v", updated)
	}
	expected = 6000
	revision = updated.PaymentRevision
	paid, err = billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", Actor: "finance", IdempotencyKey: "pay-reopened", ExpectedDueAmountCents: &expected, ExpectedPaymentRevision: &revision, PaymentMethod: "测试银行"})
	if err != nil {
		t.Fatal(err)
	}
	if paid.DueAmountCents != 0 {
		t.Fatalf("reopened bill not paid: %+v", paid)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint FROM %s.orders WHERE id=78204`, schema)).Scan(&cash); err != nil || cash != 12000 {
		t.Fatalf("cash history lost: cash=%d err=%v", cash, err)
	}
	expected, revision = paid.DueAmountCents, paid.PaymentRevision
	reverse := app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "reverse", Actor: "finance", IdempotencyKey: "reverse-latest-addition", ExpectedDueAmountCents: &expected, ExpectedPaymentRevision: &revision}
	reversed, err := billing.ChangeBillingPayment(ctx, reverse)
	if err != nil {
		t.Fatal(err)
	}
	if reversed.DueAmountCents != 6000 {
		t.Fatalf("reversed original payments instead of latest addition: %+v", reversed)
	}
	if _, err = billing.ChangeBillingPayment(ctx, reverse); err != nil {
		t.Fatalf("repeat reverse: %v", err)
	}
	var active int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)FROM %s.customer_settlement_payments WHERE settlement_batch_id=$1 AND reversed_at IS NULL`, schema), bill.BatchID).Scan(&active); err != nil || active != 1 {
		t.Fatalf("earlier receipt lost: %d %v", active, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint FROM %s.orders WHERE id=78204`, schema)).Scan(&cash); err != nil || cash != 6000 {
		t.Fatalf("reverse cash=%d %v", cash, err)
	}
	expected, revision = reversed.DueAmountCents, reversed.PaymentRevision
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, key := range []string{"repay-A", "repay-B"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, e := billing.ChangeBillingPayment(ctx, app.BillingPaymentCommand{CustomerID: 501, BatchID: bill.BatchID, Action: "pay", Actor: "finance", IdempotencyKey: key, ExpectedDueAmountCents: &expected, ExpectedPaymentRevision: &revision, PaymentMethod: "测试银行"})
			outcomes <- e
		}(key)
	}
	wg.Wait()
	close(outcomes)
	var succeeded int
	for e := range outcomes {
		if e == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent stale payments succeeded %d times", succeeded)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint FROM %s.orders WHERE id=78204`, schema)).Scan(&cash); err != nil || cash != 12000 {
		t.Fatalf("concurrent cash=%d %v", cash, err)
	}
}

func TestOrderResolutionPartialShipmentUsesExactLineQuantity(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78201,'PARTIAL-SHIP',501,150);
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)
VALUES(78201,78201,1,911,'coffee A','1000g',3,'袋',30,90),(78202,78201,2,912,'coffee B','500g',2,'袋',30,60);
INSERT INTO %[1]s.order_stock_deductions(order_id,product_id,spec_g,deducted_units,deducted_g,batch_code)VALUES(78201,911,1000,1,1000,'PARTIAL-BATCH');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78201)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Lines) != 2 || c.Lines[0].Delivered != 1 || c.Lines[1].Delivered != 0 {
		t.Fatalf("shipment quantities conflated: %+v", c.Lines)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "close-partial"}
	result, err := repo.Submit(ctx, 78201, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.CreditCents != 12000 {
		t.Fatalf("delivered receivable lost: %+v", result.Plan)
	}
	var credit, deductions int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_credit_cents,(SELECT sum(deducted_units)FROM %[1]s.order_stock_deductions WHERE order_id=78201) FROM %[1]s.orders WHERE id=78201`, schema)).Scan(&credit, &deductions); err != nil || credit != 12000 || deductions != 1 {
		t.Fatalf("credit=%d deductions=%d err=%v", credit, deductions, err)
	}
}

func TestOrderResolutionNewInvoiceRequiresNewConfirmation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,shipping_amount,grand_total)VALUES(78203,'NEW-INVOICE',501,90,10,100);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78203,78203,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.order_invoices(order_id,status)VALUES(78203,'requested')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78203)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "recorded", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "invoice-start", Dispositions: []domain.Disposition{{Kind: "fee", ID: -1, Choice: "keep"}}}
	result, err := repo.Submit(ctx, 78203, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var external, fee int64
	for _, s := range result.Steps {
		if s.Kind == "external" {
			external = s.ID
		}
		if s.Kind == "fee" {
			fee = s.ID
		}
	}
	_, err = repo.CompleteStep(ctx, 78203, result.ID, external, resolutionpg.StepCommand{Choice: "recorded", Note: "已登记原凭据", Reference: "original", IdempotencyKey: "confirm-original"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_invoices SET status='uploaded',updated_by='new-invoice' WHERE order_id=78203`, schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteStep(ctx, 78203, result.ID, fee, resolutionpg.StepCommand{Choice: "keep", Note: "保留收费", IdempotencyKey: "confirm-fee"}, "finance")
	if err == nil {
		t.Fatal("new invoice reused old external confirmation")
	}
	c, err = repo.Context(ctx, 78203)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = c.Version
	cmd.IdempotencyKey = "refresh-invoice"
	result, err = repo.Refresh(ctx, 78203, result.ID, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var pendingExternal int
	for _, s := range result.Steps {
		if s.Kind == "external" && s.Status == "pending" {
			pendingExternal++
		}
	}
	if pendingExternal != 1 {
		t.Fatalf("refresh did not require new invoice confirmation: %+v", result.Steps)
	}
}

func TestOrderResolutionIndependentFeeAdjustmentKeepsOriginal(t *testing.T) {
	for _, issued := range []bool{false, true} {
		t.Run(fmt.Sprint("issued=", issued), func(t *testing.T) {
			pool, schema := newMiniDirectShipTestDB(t)
			seedMiniDirectShipStock(t, pool, schema)
			ctx := context.Background()
			_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,grand_total)VALUES(78205,'FEE-CLOSE',501,'2026-10-01',90,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78205,78205,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'settlement',true)ON CONFLICT(customer_id,capability_code)DO UPDATE SET enabled=true;INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78205,501,'order',78205,'direct_ship_service',10,'2026-10-01')`, schema))
			if err != nil {
				t.Fatal(err)
			}
			billing := NewRepository(pool, schema)
			var batch int64
			if issued {
				pc := app.BillingPreviewCommand{CustomerID: 501, Sources: []app.BillingSource{{ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: 78205}}}
				p, e := billing.PreviewBilling(ctx, pc)
				if e != nil {
					t.Fatal(e)
				}
				b, e := billing.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: p.PreviewVersion, IdempotencyKey: "fee-bill"})
				if e != nil {
					t.Fatal(e)
				}
				batch = b.BatchID
			}
			repo := resolutionpg.NewRepository(pool, schema)
			c, err := repo.Context(ctx, 78205)
			if err != nil {
				t.Fatal(err)
			}
			cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "reduce-fee", Dispositions: []domain.Disposition{{Kind: "fee", ID: 78205, Choice: "partial_waiver", AmountCents: 500}}}
			result, err := repo.Submit(ctx, 78205, cmd, "sales")
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range result.Steps {
				_, err = repo.CompleteStep(ctx, 78205, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对费用减免五元", Reference: "fee-adjustment", IdempotencyKey: fmt.Sprint("confirm-", step.ID)}, "finance")
				if err != nil {
					t.Fatal(err)
				}
			}
			if issued {
				b, e := billing.GetIssuedBill(ctx, 501, batch)
				if e != nil || b.TotalAmountCents != 1000 || b.DueAmountCents != 500 {
					t.Fatalf("bill=%+v err=%v", b, e)
				}
			} else {
				page, e := billing.ListUnbilledCharges(ctx, app.BillingQuery{CustomerID: 501})
				if e != nil {
					t.Fatal(e)
				}
				var due int64
				for _, line := range page.Rows {
					if line.SourceType == "customer_fee_item" && line.SourceID == 78205 {
						due = line.DueCents
					}
				}
				if due != 500 {
					t.Fatalf("fee due=%d", due)
				}
			}
			var original, credit int64
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(amount*100)::bigint,resolution_credit_cents FROM %s.customer_fee_items WHERE id=78205`, schema)).Scan(&original, &credit); err != nil || original != 1000 || credit != 500 {
				t.Fatalf("original=%d credit=%d err=%v", original, credit, err)
			}
		})
	}
}

func TestOrderResolutionContinueCompanyRequiresProductionConfirmation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78206,'CONTINUE-COMPANY',501,90),(78207,'KEEP-OTHER',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78206,78206,1,911,'coffee','1000g',3,'袋',30,90),(78207,78207,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78206,'CONTINUE-SHARED','running');INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,order_nos)VALUES(78206,78206,911,'coffee',1000,6000,'CONTINUE-COMPANY,KEEP-OTHER');INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,product_id,product_name,spec_g,planned_g,order_nos,status)VALUES(78206,'CONTINUE-WORK',78206,78206,911,'coffee',1000,6000,'CONTINUE-COMPANY,KEEP-OTHER','running');INSERT INTO %[1]s.job_cards(id,work_order_id,status)VALUES(78206,78206,'pending')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78206)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "continue-shared", Dispositions: []domain.Disposition{{Kind: "production", ID: 78206, Choice: "continue_company"}}}
	result, err := repo.Submit(ctx, 78206, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='running',started_at=now() WHERE id=78206`, schema)); err == nil {
		t.Fatal("started before production disposition confirmation")
	}
	result, err = repo.CompleteStep(ctx, 78206, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "continue_company", Note: "保留其他订单份额，本单份额转公司备货", IdempotencyKey: "approve-company"}, "production")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("result=%+v", result)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='running',started_at=now() WHERE id=78206`, schema)); err != nil {
		t.Fatal(err)
	}
	var nos, other string
	var closed int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT order_nos,(SELECT resolution_status FROM %[1]s.orders WHERE id=78207),(SELECT resolution_closed_qty::int FROM %[1]s.order_items WHERE id=78207)FROM %[1]s.work_orders WHERE id=78206`, schema)).Scan(&nos, &other, &closed); err != nil || nos != "CONTINUE-COMPANY,KEEP-OTHER" || other != "open" || closed != 0 {
		t.Fatalf("shared production changed: %s %s %d %v", nos, other, closed, err)
	}
}

func TestOrderResolutionDirectShipCancellationPreservesOtherPackages(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipCanonicalStock(t, pool, schema)
	ctx := context.Background()
	fulfillment := NewRepository(pool, schema)
	created, err := fulfillment.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, EmployeeID: 701, MiniUserID: 801, IdempotencyKey: "resolution-packages", RecipientName: "隔离样本", RecipientPhone: "13800138000", Province: "云南省", City: "普洱市", District: "思茅区", DetailAddress: "测试路8号", Items: []app.MiniDirectShipItemCommand{{ProductID: 911, BomSpecID: 9111, BomVariantID: 9112, Qty: 3}}, Actor: "employee:703"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Packages) != 2 {
		t.Fatalf("packages=%+v", created.Packages)
	}
	if _, err = fulfillment.CancelMiniDirectShipRequest(ctx, 501, created.ID, "employee:703"); err == nil {
		t.Fatal("legacy cancellation bypassed explicit reason and fee choices")
	}
	first, second := created.Packages[0].OrderID, created.Packages[1].OrderID
	var originalStock, otherBefore string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT(SELECT jsonb_agg(to_jsonb(b)ORDER BY id)::text FROM %[1]s.stock_batches b),(SELECT to_jsonb(o)::text FROM %[1]s.orders o WHERE id=$1)`, schema), second).Scan(&originalStock, &otherBefore); err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	cmd := domain.Command{Action: "cancel", Reason: "entry_error", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "cancel-first-package"}
	for _, d := range c.Dependencies {
		switch d.Kind {
		case "direct_ship":
			found = true
			cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "cancel"})
		case "fee":
			cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "waive"})
		}
	}
	if !found {
		t.Fatal("own direct shipment request missing from dependencies")
	}
	result, err := repo.Submit(ctx, first, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Steps {
		result, err = repo.CompleteStep(ctx, first, result.ID, s.ID, resolutionpg.StepCommand{Choice: s.Choice, Note: "取消本包裹，其他包裹保留", Reference: "test-evidence", IdempotencyKey: fmt.Sprint("package-step-", s.ID)}, "admin")
		if err != nil {
			t.Fatal(err)
		}
	}
	if result.Status != "completed" {
		t.Fatalf("result=%+v", result)
	}
	var afterStock, otherAfter string
	var status string
	var active int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT(SELECT jsonb_agg(to_jsonb(b)ORDER BY id)::text FROM %[1]s.stock_batches b),(SELECT to_jsonb(o)::text FROM %[1]s.orders o WHERE id=$1),(SELECT status FROM %[1]s.customer_direct_ship_request_orders WHERE order_id=$2),(SELECT count(*) FROM %[1]s.customer_direct_ship_request_allocations WHERE order_id=$1 AND status='reserved')`, schema), second, first).Scan(&afterStock, &otherAfter, &status, &active); err != nil || afterStock != originalStock || otherAfter != otherBefore || status != "cancelled" || active != 1 {
		t.Fatalf("unrelated facts changed: status=%s active=%d err=%v", status, active, err)
	}
	var released, remaining int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),COALESCE(sum(allocated_units),0) FROM %s.order_stock_batch_allocations WHERE order_id=$1`, schema), first).Scan(&released, &remaining); err != nil || released != 1 || remaining != 0 {
		t.Fatalf("reservation projection not retained/released: rows=%d units=%d err=%v", released, remaining, err)
	}
}

func TestOrderResolutionOrdinaryReservationRequiresExplicitReleaseAndCapsLaterWrites(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78220,'RESERVED-CLOSE',501,90);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78220,78220,1,911,'coffee','1000g',3,'袋',30,90);
 INSERT INTO %[1]s.order_stock_batch_allocations(id,order_id,product_id,spec_g,batch_id,batch_code,allocated_units,allocated_g)VALUES(78220,78220,911,1000,1001,'DEV-E2E-FP-A1',3,3000)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78220)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "selected", Lines: []domain.Selection{{ItemID: 78220, Quantity: 1}}, Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "stock-release"}
	if _, err = repo.Submit(ctx, 78220, cmd, "sales"); err == nil {
		t.Fatal("closed reserved goods without an explicit disposition")
	}
	cmd.Dispositions = []domain.Disposition{{Kind: "stock_reservation", ID: 78220, Choice: "release", Quantity: 1}}
	result, err := repo.Submit(ctx, 78220, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 || result.Steps[0].Permission != "stock.resolve" {
		t.Fatalf("warehouse confirmation absent: %+v", result)
	}
	result, err = repo.CompleteStep(ctx, 78220, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "release", Note: "释放一袋未发货预留", IdempotencyKey: "release-stock"}, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	var units, grams, snapshots int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT allocated_units,allocated_g,(SELECT count(*) FROM %[1]s.order_resolution_reservation_releases WHERE order_id=78220)FROM %[1]s.order_stock_batch_allocations WHERE id=78220`, schema)).Scan(&units, &grams, &snapshots); err != nil || units != 2 || grams != 2000 || snapshots != 1 {
		t.Fatalf("release=%d/%d snapshots=%d err=%v", units, grams, snapshots, err)
	}
	for _, sql := range []string{
		`UPDATE %[1]s.order_stock_batch_allocations SET allocated_units=3,allocated_g=3000 WHERE id=78220`,
		`INSERT INTO %[1]s.order_stock_deductions(order_id,product_id,spec_g,batch_id,batch_code,deducted_units,deducted_g)VALUES(78220,911,1000,1001,'DEV-E2E-FP-A1',3,3000)`,
	} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatalf("legacy writer exceeded remaining quantity: %s", sql)
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_stock_deductions(order_id,product_id,spec_g,batch_id,batch_code,deducted_units,deducted_g)VALUES(78220,911,1000,1001,'DEV-E2E-FP-A1',2,2000)`, schema)); err != nil {
		t.Fatalf("valid remaining shipment: %v", err)
	}
}

func TestOrderResolutionCompletedCustomerProductionCannotBecomeCompanyStock(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78240,'CUSTOMER-OWNED-PRODUCTION',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78240,78240,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78240,'CUSTOMER-PRODUCTION','completed');INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,customer_id,spec_g,planned_g,order_nos)VALUES(78240,78240,911,'customer coffee',501,1000,3000,'CUSTOMER-OWNED-PRODUCTION');UPDATE %[1]s.production_plan_items SET target_warehouse='DEV-E2E-A1' WHERE id=78240`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78240)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Submit(ctx, 78240, domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "wrong-ownership", Dispositions: []domain.Disposition{{Kind: "production", ID: 78240, Choice: "keep_company_stock"}}}, "sales")
	if err == nil {
		t.Fatal("customer-owned completed production was labeled and disposed as company stock")
	}
}

func TestOrderResolutionProductionOwnershipUsesInventoryFacts(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"plan_warehouse", `UPDATE %[1]s.production_plan_items SET target_warehouse='DEV-E2E-A1' WHERE id=78340`},
		{"work_warehouse", `UPDATE %[1]s.work_orders SET target_warehouse='DEV-E2E-A1' WHERE id=78340`},
		{"processing", `UPDATE %[1]s.production_plan_items SET processing_request_item_id=1 WHERE id=78340`},
		{"output_material", `INSERT INTO %[1]s.materials(id,code,name,owner_customer_id)VALUES(78340,'OWNED-OUTPUT','customer material',501);UPDATE %[1]s.work_orders SET output_type='material',output_material_id=78340 WHERE id=78340`},
		{"receipt", `INSERT INTO %[1]s.stock_entries(id,entry_no,entry_type,status,work_order_id)VALUES(78340,'OWNED-RECEIPT','finished_receipt','submitted',78340);INSERT INTO %[1]s.stock_entry_items(stock_entry_id,product_id,owner_customer_id,qty_units,to_warehouse)VALUES(78340,911,501,3,'DEV-E2E-A1')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newMiniDirectShipTestDB(t)
			seedMiniDirectShipStock(t, pool, schema)
			ctx := context.Background()
			_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78340,'OWNERSHIP-FACTS','completed');INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g)VALUES(78340,78340,911,'coffee',1000,3000);INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,product_id,product_name,spec_g,planned_g,status)VALUES(78340,'OWNERSHIP-WORK',78340,78340,911,'coffee',1000,3000,'completed');`+tc.change, schema))
			if err != nil {
				t.Fatal(err)
			}
			var issue string
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s.order_resolution_company_production_issue(78340)`, schema)).Scan(&issue); err != nil {
				t.Fatal(err)
			}
			if issue == "" {
				t.Fatal("customer inventory fact accepted as company stock")
			}
		})
	}
}
