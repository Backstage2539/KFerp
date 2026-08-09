package sales

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestProcessingBillingPreviewConfirmSnapshotsAndIdempotency(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s.customers(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.audit_logs(id BIGSERIAL PRIMARY KEY,ts TIMESTAMPTZ DEFAULT now(),actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.work_orders(id BIGINT PRIMARY KEY,work_order_no TEXT,product_id BIGINT,product_name TEXT,spec_g BIGINT,status TEXT,running_item_id BIGINT,completed_at TIMESTAMPTZ)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_processing_production_demands(id BIGSERIAL PRIMARY KEY,customer_id BIGINT,request_no TEXT,linked_work_order_id BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.produce_running_items(id BIGINT PRIMARY KEY,input_g BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.produce_running_outputs(id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,product_id BIGINT,spec_g BIGINT,finished_units BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_batch_costs(id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,finished_g BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_logs(id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,completion_no BIGINT NOT NULL DEFAULT 1,input_g BIGINT NOT NULL DEFAULT 0,finished_total_g BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.job_cards(id BIGSERIAL PRIMARY KEY,work_order_id BIGINT,actual_input_qty NUMERIC NOT NULL DEFAULT 0,actual_minutes INT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.materials(id BIGINT PRIMARY KEY,purchase_price NUMERIC)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.material_batches(id BIGINT PRIMARY KEY,unit_cost NUMERIC)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_batches(id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,unit_cost NUMERIC NOT NULL DEFAULT 0)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.material_consumption_logs(id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,material_id BIGINT,material_batch_id BIGINT,material_batch_code TEXT NOT NULL DEFAULT '',deduct_g BIGINT,deduct_units BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_processing_material_reservations(id BIGSERIAL PRIMARY KEY,work_order_id BIGINT,material_id BIGINT,component_type TEXT NOT NULL DEFAULT 'material',component_product_id BIGINT NOT NULL DEFAULT 0,component_spec_g BIGINT NOT NULL DEFAULT 0,material_batch_id BIGINT,finished_stock_batch_id BIGINT NOT NULL DEFAULT 0,reserved_g BIGINT,required_g BIGINT,reserved_units BIGINT,required_units BIGINT,consumed_g BIGINT DEFAULT 0,consumed_units BIGINT DEFAULT 0,returned_g BIGINT DEFAULT 0,returned_units BIGINT DEFAULT 0,source_owner_type TEXT,source_customer_id BIGINT,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_fee_items(id BIGSERIAL PRIMARY KEY,customer_id BIGINT,source_type TEXT,source_id BIGINT,fee_type TEXT,amount NUMERIC,currency TEXT,occurred_at TIMESTAMPTZ,settlement_batch_id BIGINT,status TEXT,note TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_settlement_batches(id BIGSERIAL PRIMARY KEY,customer_id BIGINT,settlement_no TEXT,period_from DATE,period_to DATE,status TEXT,total_amount NUMERIC,confirmed_at TIMESTAMPTZ,paid_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT now(),created_by TEXT)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare processing billing schema: %v\n%s", err, stmt)
		}
	}
	for pass := 1; pass <= 2; pass++ {
		if err := ensureOutsourceTemplateTables(ctx, pool, schema); err != nil {
			t.Fatalf("ensureOutsourceTemplateTables pass %d: %v", pass, err)
		}
		if err := ensureProcessingBillingTables(ctx, pool, schema); err != nil {
			t.Fatalf("ensureProcessingBillingTables pass %d: %v", pass, err)
		}
	}
	for _, stmt := range []string{
		fmt.Sprintf(`INSERT INTO %s.customers VALUES(19,'9.9 COFFEE LAB'),(20,'其他客户')`, schema),
		fmt.Sprintf(`INSERT INTO %s.work_orders VALUES(91,'WO-91',501,'客户拼配',400,'completed',71,'2026-08-06 10:00+08')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_processing_production_demands(customer_id,request_no,linked_work_order_id) VALUES(19,'PJ-19',91)`, schema),
		fmt.Sprintf(`INSERT INTO %s.produce_running_items VALUES(71,10000)`, schema),
		fmt.Sprintf(`INSERT INTO %s.produce_running_outputs(running_item_id,product_id,spec_g,finished_units) VALUES(71,501,400,20)`, schema),
		fmt.Sprintf(`INSERT INTO %s.production_batch_costs(running_item_id,finished_g) VALUES(71,8000)`, schema),
		fmt.Sprintf(`INSERT INTO %s.production_logs(running_item_id,completion_no,input_g,finished_total_g) VALUES(71,1,8000,5000),(71,1,8000,3000)`, schema),
		fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,actual_input_qty,actual_minutes) VALUES(91,0,25),(91,0,35)`, schema),
		fmt.Sprintf(`INSERT INTO %s.materials VALUES(301,10)`, schema),
		fmt.Sprintf(`INSERT INTO %s.material_batches VALUES(1,10)`, schema),
		fmt.Sprintf(`INSERT INTO %s.stock_batches(id,batch_code,item_type,item_id,unit_cost) VALUES(11,'FP-COMP-11','finished_product',601,50)`, schema),
		fmt.Sprintf(`INSERT INTO %s.material_consumption_logs(running_item_id,material_id,material_batch_id,material_batch_code,deduct_g,deduct_units) VALUES(71,301,1,'MAT-1',800,0),(71,601,0,'FP-COMP-11',1000,0)`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_processing_material_reservations(work_order_id,material_id,material_batch_id,reserved_g,required_g,reserved_units,required_units,consumed_g,consumed_units,returned_g,returned_units,source_owner_type,source_customer_id,status) VALUES(91,301,1,600,600,0,0,600,0,0,0,'customer',19,'consumed'),(91,301,1,400,400,0,0,200,0,200,0,'factory',0,'consumed')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_processing_material_reservations(work_order_id,material_id,component_type,component_product_id,component_spec_g,finished_stock_batch_id,reserved_g,required_g,reserved_units,required_units,consumed_g,consumed_units,returned_g,returned_units,source_owner_type,source_customer_id,status) VALUES(91,601,'finished_product',601,250,11,1000,1000,0,0,1000,0,0,0,'factory',0,'consumed')`, schema),
		fmt.Sprintf(`INSERT INTO %s.work_orders VALUES(93,'WO-93',503,'分次完工',400,'completed',73,'2026-08-06 12:00+08'),(94,'WO-94',504,'历史工序卡',400,'completed',74,'2026-08-06 13:00+08'),(95,'WO-95',505,'历史计划投入',400,'completed',75,'2026-08-06 14:00+08'),(96,'WO-96',506,'缺少实耗',400,'completed',76,'2026-08-06 15:00+08')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_processing_production_demands(customer_id,request_no,linked_work_order_id) VALUES(19,'PJ-93',93),(19,'PJ-94',94),(19,'PJ-95',95),(19,'PJ-96',96)`, schema),
		fmt.Sprintf(`INSERT INTO %s.produce_running_items VALUES(73,12000),(74,9000),(75,5000),(76,400)`, schema),
		fmt.Sprintf(`INSERT INTO %s.production_logs(running_item_id,completion_no,input_g,finished_total_g) VALUES(73,1,4000,3000),(73,1,4000,500),(73,2,6000,5000),(73,2,6000,500)`, schema),
		fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,actual_input_qty,actual_minutes) VALUES(94,7500,20)`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_processing_material_reservations(work_order_id,material_id,material_batch_id,reserved_g,required_g,reserved_units,required_units,consumed_g,consumed_units,returned_g,returned_units,source_owner_type,source_customer_id,status) VALUES(96,301,1,400,400,0,0,0,0,0,0,'factory',0,'consumed')`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed processing billing: %v\n%s", err, stmt)
		}
	}
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for workOrderID, wantInputKG := range map[int64]float64{91: 8, 93: 10, 94: 7.5, 95: 5} {
		metrics, metricsErr := repo.processingBillingMetricsTx(ctx, tx, 19, workOrderID)
		if metricsErr != nil || metrics.ActualInputKG != wantInputKG {
			_ = tx.Rollback(ctx)
			t.Fatalf("work order %d actual input=%v want=%v err=%v", workOrderID, metrics.ActualInputKG, wantInputKG, metricsErr)
		}
	}
	if cost, costErr := repo.factoryMaterialActualCostTx(ctx, tx, 96, 76); costErr != nil || cost != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("missing actual consumption must not guess reservation amount: cost=%v err=%v", cost, costErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveOutsourceTemplate(ctx, salesapp.SaveOutsourceTemplateCommand{
		Name: "实际代加工费", Actor: "财务甲", Rules: []salesapp.OutsourceTemplateRuleInput{
			{FeeType: "roasting", Name: "产出加工费", Basis: salesapp.BillingBasisActualOutputKG, UnitPrice: 2, SortOrder: 10},
			{FeeType: "material", Name: "工厂物料费", Basis: salesapp.BillingBasisFactoryMaterialActualCost, UnitPrice: 1, SortOrder: 20},
		},
	}); err != nil {
		t.Fatalf("SaveOutsourceTemplate: %v", err)
	}
	templates, err := repo.ListOutsourceTemplates(ctx)
	if err != nil || len(templates) != 1 || templates[0].CurrentVersionID <= 0 {
		t.Fatalf("ListOutsourceTemplates() templates=%+v err=%v", templates, err)
	}
	preview, err := repo.PreviewProcessingBilling(ctx, salesapp.PreviewProcessingBillingCommand{
		CustomerID: 19, TemplateID: templates[0].ID, WorkOrderIDs: []int64{91},
	})
	if err != nil {
		t.Fatalf("PreviewProcessingBilling: %v", err)
	}
	if len(preview.WorkOrders) != 1 || len(preview.Lines) != 2 {
		t.Fatalf("preview=%+v", preview)
	}
	metrics := preview.WorkOrders[0]
	if metrics.ActualInputKG != 8 || metrics.ActualOutputKG != 8 || metrics.ActualMinutes != 60 || metrics.ActualUnits != 20 || math.Abs(metrics.FactoryMaterialActualCost-52) > 0.001 {
		t.Fatalf("metrics=%+v, duplicate completion input/customer-owned material must be excluded and factory finished component included", metrics)
	}
	if preview.TotalAmount != 68 {
		t.Fatalf("preview total=%v lines=%+v", preview.TotalAmount, preview.Lines)
	}
	if _, err := repo.PreviewProcessingBilling(ctx, salesapp.PreviewProcessingBillingCommand{
		CustomerID: 19, TemplateID: templates[0].ID, WorkOrderIDs: []int64{96},
	}); err == nil || !strings.Contains(err.Error(), "缺少实际工厂物料耗用数据") {
		t.Fatalf("missing factory actual consumption preview error=%v", err)
	}

	confirmResults := make(chan salesapp.ProcessingBillingConfirmation, 2)
	confirmErrors := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, confirmErr := repo.ConfirmProcessingBilling(ctx, salesapp.ConfirmProcessingBillingCommand{
				CustomerID: 19, TemplateVersionID: preview.TemplateVersionID, WorkOrderIDs: []int64{91}, Actor: "财务甲",
			})
			confirmResults <- result
			confirmErrors <- confirmErr
		}()
	}
	wg.Wait()
	close(confirmResults)
	close(confirmErrors)
	for confirmErr := range confirmErrors {
		if confirmErr != nil {
			t.Fatalf("concurrent ConfirmProcessingBilling() err=%v", confirmErr)
		}
	}
	var confirmed salesapp.ProcessingBillingConfirmation
	for result := range confirmResults {
		if confirmed.BillingRunID == 0 {
			confirmed = result
			continue
		}
		if result.BillingRunID != confirmed.BillingRunID || result.SettlementBatchID != confirmed.SettlementBatchID {
			t.Fatalf("concurrent confirmations differ: first=%+v second=%+v", confirmed, result)
		}
		if result.Reused == confirmed.Reused {
			t.Fatalf("exactly one concurrent confirmation should be reused: first=%+v second=%+v", confirmed, result)
		}
	}
	if confirmed.BillingRunID <= 0 || confirmed.SettlementBatchID <= 0 || confirmed.TotalAmount != 68 {
		t.Fatalf("ConfirmProcessingBilling()=%+v", confirmed)
	}
	reused, err := repo.ConfirmProcessingBilling(ctx, salesapp.ConfirmProcessingBillingCommand{
		CustomerID: 19, TemplateVersionID: preview.TemplateVersionID, WorkOrderIDs: []int64{91}, Actor: "财务甲",
	})
	if err != nil || !reused.Reused || reused.BillingRunID != confirmed.BillingRunID {
		t.Fatalf("idempotent confirm=%+v err=%v", reused, err)
	}
	for table, want := range map[string]int{"processing_billing_runs": 1, "processing_billing_work_orders": 1, "processing_billing_line_snapshots": 2, "customer_fee_items": 2, "customer_settlement_batches": 1} {
		var count int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.%s`, schema, table)).Scan(&count); err != nil || count != want {
			t.Fatalf("%s count=%d want=%d err=%v", table, count, want, err)
		}
	}
	var feeTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(amount),0)::float8 FROM %s.customer_fee_items`, schema)).Scan(&feeTotal); err != nil || feeTotal != 68 {
		t.Fatalf("fee total=%v err=%v", feeTotal, err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.audit_logs WHERE entity_type='customer_processing_bill' AND action='confirm'`, schema)).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("bill audit count=%d err=%v", auditCount, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_orders VALUES(92,'WO-92',502,'客户拼配二',400,'completed',72,'2026-08-06 11:00+08')`, schema)); err != nil {
		t.Fatalf("extend work order fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_billing_work_orders(
			billing_run_id,work_order_id,work_order_no,product_name,completed_at
		) VALUES($1,92,'WO-92','客户拼配二','2026-08-06 11:00+08')
	`, schema), confirmed.BillingRunID); err != nil {
		t.Fatalf("extend bill fixture: %v", err)
	}
	if _, err := repo.ConfirmProcessingBilling(ctx, salesapp.ConfirmProcessingBillingCommand{
		CustomerID: 19, TemplateVersionID: preview.TemplateVersionID, WorkOrderIDs: []int64{91}, Actor: "财务甲",
	}); err == nil {
		t.Fatal("a subset of an existing multi-work-order bill must not be reused as if it were an exact idempotent request")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.processing_billing_work_orders WHERE work_order_id=92`, schema)); err != nil {
		t.Fatalf("clean extended bill fixture: %v", err)
	}

	svc := salesapp.NewService(repo)
	paid, err := svc.PayProcessingBilling(ctx, salesapp.PayProcessingBillingCommand{BillingRunID: confirmed.BillingRunID, Actor: "财务甲", Note: "微信收款"})
	if err != nil || paid.Status != salesapp.ProcessingBillingStatusPaid {
		t.Fatalf("PayProcessingBilling()=%+v err=%v", paid, err)
	}
	paidAgain, err := svc.PayProcessingBilling(ctx, salesapp.PayProcessingBillingCommand{BillingRunID: confirmed.BillingRunID, Actor: "财务甲"})
	if err != nil || !paidAgain.Reused || paidAgain.BillingRunID != confirmed.BillingRunID {
		t.Fatalf("idempotent pay=%+v err=%v", paidAgain, err)
	}
	paidConfirm, err := svc.ConfirmProcessingBilling(ctx, salesapp.ConfirmProcessingBillingCommand{
		CustomerID: 19, TemplateVersionID: preview.TemplateVersionID, WorkOrderIDs: []int64{91}, Actor: "财务甲",
	})
	if err != nil || !paidConfirm.Reused || paidConfirm.BillingRunID != confirmed.BillingRunID {
		t.Fatalf("paid bill must reuse immutable confirmation=%+v err=%v", paidConfirm, err)
	}

	adjustmentCommand := salesapp.AdjustProcessingBillingCommand{
		BillingRunID: confirmed.BillingRunID, Actor: "财务甲", Reason: "补收人工费",
		Lines: []salesapp.ProcessingBillingAdjustmentLineInput{{WorkOrderID: 91, FeeType: "labor", FeeName: "补收人工", Amount: 5}},
	}
	adjusted, err := svc.AdjustProcessingBilling(ctx, adjustmentCommand)
	if err != nil || adjusted.SourceBillingRunID != confirmed.BillingRunID || adjusted.TotalAmount != 5 {
		t.Fatalf("AdjustProcessingBilling()=%+v err=%v", adjusted, err)
	}
	adjustedAgain, err := svc.AdjustProcessingBilling(ctx, adjustmentCommand)
	if err != nil || !adjustedAgain.Reused || adjustedAgain.BillingRunID != adjusted.BillingRunID {
		t.Fatalf("idempotent adjustment=%+v err=%v", adjustedAgain, err)
	}

	var originalLineCount int
	var originalLineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::int,COALESCE(SUM(amount),0)::float8
		FROM %s.processing_billing_line_snapshots WHERE billing_run_id=$1
	`, schema), confirmed.BillingRunID).Scan(&originalLineCount, &originalLineTotal); err != nil || originalLineCount != 2 || originalLineTotal != 68 {
		t.Fatalf("original snapshot count=%d total=%v err=%v", originalLineCount, originalLineTotal, err)
	}

	reversed, err := svc.ReverseProcessingBilling(ctx, salesapp.ReverseProcessingBillingCommand{BillingRunID: confirmed.BillingRunID, Actor: "财务甲", Reason: "重复计费"})
	if err != nil || reversed.SourceBillingRunID != confirmed.BillingRunID || reversed.TotalAmount != -68 {
		t.Fatalf("ReverseProcessingBilling()=%+v err=%v", reversed, err)
	}
	reversedAgain, err := svc.ReverseProcessingBilling(ctx, salesapp.ReverseProcessingBillingCommand{BillingRunID: confirmed.BillingRunID, Actor: "财务甲", Reason: "重复计费"})
	if err != nil || !reversedAgain.Reused || reversedAgain.BillingRunID != reversed.BillingRunID {
		t.Fatalf("idempotent reversal=%+v err=%v", reversedAgain, err)
	}
	if _, err := svc.PayProcessingBilling(ctx, salesapp.PayProcessingBillingCommand{BillingRunID: confirmed.BillingRunID, Actor: "财务甲"}); err == nil {
		t.Fatal("reversed bill accepted payment")
	}
	if _, err := svc.ConfirmProcessingBilling(ctx, salesapp.ConfirmProcessingBillingCommand{
		CustomerID: 19, TemplateVersionID: preview.TemplateVersionID, WorkOrderIDs: []int64{91}, Actor: "财务甲",
	}); err == nil {
		t.Fatal("reversed bill was recalculated")
	}

	runs, err := svc.ListProcessingBillingRuns(ctx, salesapp.ProcessingBillingRunsQuery{CustomerID: 19})
	if err != nil || len(runs) != 3 {
		t.Fatalf("ListProcessingBillingRuns()=%+v err=%v", runs, err)
	}
	for table, want := range map[string]int{"processing_billing_runs": 3, "processing_billing_work_orders": 3, "processing_billing_line_snapshots": 5, "customer_fee_items": 5, "customer_settlement_batches": 3} {
		var count int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.%s`, schema, table)).Scan(&count); err != nil || count != want {
			t.Fatalf("lifecycle %s count=%d want=%d err=%v", table, count, want, err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(amount),0)::float8 FROM %s.customer_fee_items`, schema)).Scan(&feeTotal); err != nil || feeTotal != 5 {
		t.Fatalf("lifecycle fee total=%v err=%v", feeTotal, err)
	}
	for action, want := range map[string]int{"confirm": 1, "pay": 1, "adjust": 1, "reverse": 1} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.audit_logs WHERE entity_type='customer_processing_bill' AND action=$1`, schema), action).Scan(&auditCount); err != nil || auditCount != want {
			t.Fatalf("audit action=%s count=%d want=%d err=%v", action, auditCount, want, err)
		}
	}
	var originalStatus string
	var paidAtPresent bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,paid_at IS NOT NULL FROM %s.processing_billing_runs WHERE id=$1`, schema), confirmed.BillingRunID).Scan(&originalStatus, &paidAtPresent); err != nil || originalStatus != salesapp.ProcessingBillingStatusReversed || !paidAtPresent {
		t.Fatalf("original lifecycle status=%s paid_at=%v err=%v", originalStatus, paidAtPresent, err)
	}
}
