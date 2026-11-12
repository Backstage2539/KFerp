package finance

import (
	"context"
	"fmt"
	appfinance "orderapp/internal/application/finance"
	"testing"
)

func TestMatchSalesCostUsesPostedBatchesAndReportsMissing(t *testing.T) {
	item := salesCostItem{quantity: 10}
	parts := []salesCostPart{
		{quantity: 4, unitCost: 12, source: "actual", batch: "A"},
		{quantity: 3, unitCost: 13, source: "actual", batch: "B"},
	}
	got := matchSalesCost(item, parts)
	if got.knownCost != 87 || got.knownQty != 7 || got.pendingQty != 3 {
		t.Fatalf("actual matched cost = %+v; want 87 for 7 and pending 3", got)
	}
}

func TestMatchSalesCostDoesNotDoubleCountOrGuessCost(t *testing.T) {
	item := salesCostItem{quantity: 10}
	parts := []salesCostPart{
		{quantity: 5, unitCost: 10, source: "actual", batch: "production:7"},
		{quantity: 8, unitCost: 12, source: "production", batch: "production:7"},
	}
	got := matchSalesCost(item, parts)
	if got.knownQty != 8 || got.knownCost != 86 || got.pendingQty != 2 {
		t.Fatalf("overlapping batch = %+v; want 86 for 8 and pending 2", got)
	}
	unknown := matchSalesCost(item, []salesCostPart{{quantity: 10, unitCost: 0, unknown: true, source: "actual"}})
	if unknown.pendingQty != 10 || unknown.knownQty != 0 {
		t.Fatalf("unknown cost treated as zero: %+v", unknown)
	}
	zero := matchSalesCost(item, []salesCostPart{{quantity: 10, unitCost: 0, source: "actual"}})
	if zero.knownQty != 10 || zero.pendingQty != 0 || zero.knownCost != 0 {
		t.Fatalf("valid zero cost is not known: %+v", zero)
	}
}

func TestConvertBatchCostUsesFrozenUnits(t *testing.T) {
	cases := []struct {
		unit                                                  string
		weight, units, grams, batchG, cost, qtyWant, costWant float64
		valid                                                 bool
	}{
		{"kg", 0, 0, 2000, 5000, 100, 2, 100, true},
		{"g", 0, 0, 500, 5000, 100, 500, .1, true},
		{"袋", 227, 2, 454, 1000, 100, 2, 22.7, true},
		{"袋", 0, 2, 454, 1000, 100, 0, 0, false},
		{"件", 0, 2, 0, 0, 3, 2, 3, true},
	}
	for _, tc := range cases {
		q, c, ok := convertBatchCost(tc.unit, tc.weight, tc.units, tc.grams, tc.batchG, tc.cost)
		if ok != tc.valid || q != tc.qtyWant || c != tc.costWant {
			t.Fatalf("%+v got qty %v cost %v valid %v", tc, q, c, ok)
		}
	}
}

func TestCustomerOwnedBatchWithUnconvertibleQuantityStaysUnknown(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,qty numeric,item_name text,unit text);
 CREATE TABLE %[1]s.stock_batches(id bigint PRIMARY KEY,batch_code text,qty_g bigint,unit_cost numeric,cost_unavailable boolean,owner_customer_id bigint,source_doc_type text,source_doc_id bigint,item_type text,item_id bigint,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.order_stock_deductions(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,batch_id bigint,batch_code text,deducted_units bigint,deducted_g bigint);
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,grand_total) VALUES(11,'SO-11','2026-11-01',1,100);
 INSERT INTO %[1]s.order_items VALUES(101,11,1,2,3,2,'咖啡','kg');
 INSERT INTO %[1]s.stock_batches VALUES(1,'A',0,60,false,1,'legacy',1,'finished_product',1,2,3);
 INSERT INTO %[1]s.order_stock_deductions VALUES(1,11,1,2,3,0,1,'A',2,0);`, schema))
	totals, _, err := NewRepository(pool, schema).MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || !totals.CostUnavailable {
		t.Fatalf("customer-owned quantity without kg conversion must remain unknown: %+v, err=%v", totals, err)
	}
}

func TestRevenueOrderWithoutItemsHasUnknownCost(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,qty numeric,item_name text,unit text);
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,grand_total) VALUES(11,'NO-ITEMS','2026-11-01',1,100);`, schema))
	rows, err := NewRepository(pool, schema).orderSalesCostDetails(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || len(rows) != 1 || !rows[0].CostUnavailable || rows[0].Date != "2026-11-01" {
		t.Fatalf("revenue order with no items must stay pending: %+v err=%v", rows, err)
	}
}

func TestRevenueOrderWithAbsentItemsTableHasUnknownCost(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,grand_total) VALUES(11,'NO-TABLE','2026-11-01',1,100),(12,'ZERO','2026-11-01',1,0);`, schema))
	rows, err := NewRepository(pool, schema).orderSalesCostDetails(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || len(rows) != 1 || !rows[0].CostUnavailable || rows[0].SourceID != 11 {
		t.Fatalf("only revenue order with no item table must stay pending: %+v err=%v", rows, err)
	}
}

func TestOrderSalesCostsFollowOrdersAndExcludeUnsoldProduction(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 ALTER TABLE %[1]s.production_batch_costs ADD COLUMN running_item_id bigint, ADD COLUMN finished_g bigint NOT NULL DEFAULT 0, ADD COLUMN unit_cost_per_kg numeric NOT NULL DEFAULT 0, ADD COLUMN cost_unavailable boolean NOT NULL DEFAULT false;
 CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec text,qty numeric,item_name text,unit text,resolution_closed_qty numeric DEFAULT 0,material_unit_weight_g numeric,weight_snapshot_json jsonb);
 CREATE TABLE %[1]s.stock_batches(id bigint PRIMARY KEY,batch_code text,qty_g bigint,unit_cost numeric,cost_unavailable boolean,owner_customer_id bigint,source_doc_type text,source_doc_id bigint,item_type text,item_id bigint,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.order_stock_deductions(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,batch_id bigint,batch_code text,deducted_units bigint,deducted_g bigint);
 CREATE TABLE %[1]s.produce_running_items(id bigint PRIMARY KEY,batch_id text,product_id bigint,spec_g bigint,output_type text,status text,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.produce_batch_allocations(id bigint PRIMARY KEY,order_item_id bigint,batch_id text,product_id bigint,spec_g bigint,allocated_units bigint);
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,grand_total) VALUES(11,'SO-11','2026-11-01',1,1000);
 INSERT INTO %[1]s.order_items VALUES(101,11,1,2,3,'1000g',10,'咖啡','袋',0,1000,'{"historical_unknown":false}');
 INSERT INTO %[1]s.produce_running_items VALUES(1,'P1',1,1000,'product','done',2,3);
 INSERT INTO %[1]s.production_batch_costs(running_item_id,product_name,total_cost,finished_g,unit_cost_per_kg,created_at) VALUES(1,'咖啡',1000,10000,100,'2026-12-01'),(99,'备货及半成品',10000,100000,100,'2026-11-01');
 INSERT INTO %[1]s.stock_batches VALUES(1,'A',4000,60,false,0,'production_run',1,'finished_product',1,2,3),(2,'B',2000,60,false,0,'production_run',1,'finished_product',1,2,3);
 INSERT INTO %[1]s.order_stock_deductions VALUES(1,11,1,2,3,1000,1,'A',4,4000),(2,11,1,2,3,1000,2,'B',2,2000);
 INSERT INTO %[1]s.produce_batch_allocations VALUES(1,101,'P1',1,1000,10);
 `, schema))
	repo := NewRepository(pool, schema)
	totals, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if totals.MainBusinessCost != 760 || totals.CostUnavailable {
		t.Fatalf("totals %+v, want known 760", totals)
	}
	details, err := repo.FinanceSourceDetails(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, d := range details {
		if d.SourceType == "order_sales_cost" {
			if d.Date != "2026-11-01" {
				t.Fatalf("wrong attribution: %+v", d)
			}
			sum += float64(d.Amount)
		}
	}
	if sum != 760 {
		t.Fatalf("details cost %.2f want 760", sum)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_batch_costs SET finished_g=5000 WHERE running_item_id=1`, schema))
	partial, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || !partial.CostUnavailable || partial.MainBusinessCost != 360 {
		t.Fatalf("insufficient completion must stay pending, got %+v err %v", partial, err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_batch_costs SET finished_g=10000,unit_cost_per_kg=0 WHERE running_item_id=1;
 UPDATE %[1]s.stock_batches SET unit_cost=0;`, schema))
	zero, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || zero.CostUnavailable || zero.MainBusinessCost != 0 {
		t.Fatalf("explicit zero costs must remain known: %+v err %v", zero, err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_batch_costs SET finished_g=10000 WHERE running_item_id=1;
 UPDATE %[1]s.production_batch_costs SET unit_cost_per_kg=100 WHERE running_item_id=1;
 UPDATE %[1]s.stock_batches SET owner_customer_id=1,unit_cost=60;
 CREATE TABLE %[1]s.order_resolution_stock_returns(id bigint,order_id bigint,order_item_id bigint,posting_date date,quantity bigint,unit_cost numeric,status text,source_kind text,source_owner_customer_id bigint);
 INSERT INTO %[1]s.order_resolution_stock_returns VALUES(1,11,101,'2026-12-05',1,60,'posted','ownership',1),(2,11,101,'2026-12-06',2,60,'posted','shipment',1),(3,11,101,'2026-12-07',1,60,'posted','shipment',0);
 UPDATE %[1]s.order_items SET resolution_closed_qty=4 WHERE id=101;`, schema))
	owned, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || owned.CostUnavailable || owned.MainBusinessCost != 400 {
		t.Fatalf("customer owned stock must not be charged again: %+v err %v", owned, err)
	}
	returned, err := repo.orderSalesCostDetails(ctx, appfinance.ReportFilter{Month: "2026-12", CustomerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(returned) != 3 || returned[0].Amount != -60 || returned[0].Date != "2026-12-05" || returned[1].Amount != 0 || returned[1].Date != "2026-12-06" || returned[2].Amount != -60 || returned[2].Date != "2026-12-07" {
		t.Fatalf("return must be posted on its actual date: %+v", returned)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.order_items SET spec='1kg',material_unit_weight_g=NULL,weight_snapshot_json='{}' WHERE id=101`, schema))
	missing, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil || !missing.CostUnavailable {
		t.Fatalf("historical missing frozen weight must stay unknown: %+v err %v", missing, err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.order_items VALUES(102,11,1,2,3,'227g*2',1,'多规格','袋',0,454,'{"historical_unknown":false}')`, schema))
	ambiguous, err := repo.orderSalesCostDetails(ctx, appfinance.ReportFilter{Month: "2026-11", CustomerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range ambiguous {
		if row.Date == "2026-11-01" && !row.CostUnavailable {
			t.Fatalf("ambiguous duplicate identity must be pending: %+v", row)
		}
	}
}
