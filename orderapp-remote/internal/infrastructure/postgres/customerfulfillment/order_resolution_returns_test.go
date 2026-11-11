package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
)

func orderReturnFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total)VALUES(78500,'RETURN-ORIGINAL',501,90,90);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit,unit_price,line_total)VALUES(78500,78500,1,910,'咖啡',7031,7032,3,'袋',30,90);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,source_doc_type,source_doc_id,qty_units,remaining_units,quality_status,unit_cost)VALUES(78500,'RETURN-SOURCE-BATCH','finished_product',910,'咖啡',7031,7032,0,0,'production_run',78500,5,2,'passed',12.5);
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units)VALUES(910,7031,7032,0,'finished_goods',0,2);
 INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_units,warehouse)VALUES(78500,78500,910,7031,7032,0,78500,'RETURN-SOURCE-BATCH',3,'finished_goods');
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units)VALUES('finished_product',910,'咖啡',7031,7032,0,0,'finished_goods','production_run',78500,'RETURN-SOURCE-BATCH',0,5,5),('finished_product',910,'咖啡',7031,7032,0,0,'finished_goods','sales_order_shipment',78500,'RETURN-SOURCE-BATCH',5,-3,2);
 INSERT INTO %[1]s.order_stock_deductions(id,order_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,deducted_units,source_doc_type,source_doc_id)VALUES(78500,78500,910,7031,7032,0,78500,'RETURN-SOURCE-BATCH',3,'sales_order_shipment_inventory_sync_v2',78500);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	return pool, schema
}
func returnCommand(version string) domain.Command {
	return domain.Command{Action: "return", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-08", ReturnSettlement: "keep_receivable", ReturnSelections: []domain.ReturnSelection{{SourceKind: "shipment", SourceID: 78500, BatchID: 78500, Quantity: 3, Ownership: "company", Warehouse: "finished_goods", Quality: "quarantine"}}, Version: version, IdempotencyKey: "return-original"}
}
func TestOrderResolutionReturnSourcesRequireRealShipment(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.ReturnSources) != 1 {
		t.Fatalf("missing exact posted shipment sources: %+v", current.ReturnSources)
	}
	source := current.ReturnSources[0]
	if source.Kind != "shipment" || source.ID != 78500 || source.BatchID != 78500 || source.ItemID != 78500 || source.AvailableQuantity != 3 || source.Version == "" {
		t.Fatalf("incorrect source identity: %+v", source)
	}
	found := false
	for _, destination := range source.Destinations {
		if destination.Warehouse == "finished_goods" && destination.Ownership == "company" && destination.OwnerCustomerID == 0 {
			found = true
		}
		if destination.OwnerCustomerID > 0 {
			t.Fatal("company return silently offered another owner")
		}
	}
	if !found {
		t.Fatal("missing explicit company destination")
	}
	for _, tc := range []struct{ name, sql string }{
		{"missing shipment", `DELETE FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment'`},
		{"wrong batch code", `UPDATE %s.order_stock_deductions SET batch_code='OTHER' WHERE id=78500`},
		{"wrong owner", `UPDATE %s.stock_batches SET owner_customer_id=502 WHERE id=78500`},
		{"ambiguous line", `INSERT INTO %s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit,unit_price,line_total)VALUES(78501,78500,2,910,'同品另一行',7031,7032,1,'袋',30,30)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other, otherSchema := orderReturnFixture(t)
			if _, err := other.Exec(ctx, fmt.Sprintf(tc.sql, otherSchema)); err != nil {
				t.Fatal(err)
			}
			got, err := resolutionpg.NewRepository(other, otherSchema).Context(ctx, 78500)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.ReturnSources) != 0 {
				t.Fatalf("invalid shipment became return stock: %+v", got.ReturnSources)
			}
		})
	}
}

func TestOrderResolutionReturnPostsStockIndependentlyFromReceivable(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	var warehouseStep, financeStep int64
	for _, step := range result.Steps {
		if step.Kind == "return_stock" {
			warehouseStep = step.ID
		}
		if step.Kind == "return_finance" {
			financeStep = step.ID
		}
	}
	if warehouseStep == 0 || financeStep == 0 {
		t.Fatalf("independent confirmations missing: %+v", result.Steps)
	}
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "核对原出库与实际退回，整批待检隔离", Reference: "退货验收-TEST", PostingDate: "2026-11-08", IdempotencyKey: "return-stock-once"}
	result, err = r.CompleteStep(ctx, 78500, result.ID, warehouseStep, confirm, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatal("physical return silently confirmed receivable treatment")
	}
	var sourceRemaining, inventory, rows, units, credit, logs int64
	var quality string
	assertStock := func() {
		t.Helper()
		err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT
 (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78500),
 (SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods'),
 (SELECT count(*) FROM %[1]s.order_resolution_stock_returns WHERE order_id=78500 AND status='posted'),
 (SELECT COALESCE(sum(remaining_units),0) FROM %[1]s.stock_batches WHERE source_doc_type='order_resolution_return'),
 (SELECT COALESCE(max(quality_status),'') FROM %[1]s.stock_batches WHERE source_doc_type='order_resolution_return'),
 (SELECT resolution_credit_cents FROM %[1]s.orders WHERE id=78500),
 (SELECT count(*) FROM %[1]s.audit_logs WHERE action='resolution_return_stock')`, schema)).Scan(&sourceRemaining, &inventory, &rows, &units, &quality, &credit, &logs)
		if err != nil {
			t.Fatal(err)
		}
		if sourceRemaining != 2 || inventory != 5 || rows != 1 || units != 3 || quality != "hold" || credit != 0 || logs != 1 {
			t.Fatalf("return changed original shipment, duplicated stock or chose money: source=%d inventory=%d returns=%d units=%d quality=%s credit=%d logs=%d", sourceRemaining, inventory, rows, units, quality, credit, logs)
		}
	}
	assertStock()
	if _, err = r.CompleteStep(ctx, 78500, result.ID, warehouseStep, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
	assertStock()
	current, err = r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	if current.Lines[0].Returned != 3 || len(current.ReturnSources) != 0 {
		t.Fatalf("posted physical return is not independently visible: %+v", current)
	}
	result, err = r.CompleteStep(ctx, 78500, result.ID, financeStep, resolutionpg.StepCommand{Choice: "keep_receivable", Note: "商品款按协商保留，本次不冲减或退款", IdempotencyKey: "return-finance"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Status != "completed" {
		t.Fatalf("return did not finish: %+v", result)
	}
	assertStock()
	current, err = r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	if current.Lines[0].Returned != 3 || current.CreditCents != 0 {
		t.Fatalf("physical quantity counted twice on final confirmation: %+v", current)
	}
}

func TestOrderResolutionReturnPreservesHistoricalBatch(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_id=910`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
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
	for _, step := range result.Steps {
		if step.Kind == "return_stock" {
			result, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "原出库实物", Note: "保持原批次冻结身份与成本", PostingDate: "2026-11-08", IdempotencyKey: "frozen-return"}, "warehouse")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var variant int64
	var cost float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id,unit_cost FROM %s.stock_batches WHERE source_doc_type='order_resolution_return'`, schema)).Scan(&variant, &cost); err != nil {
		t.Fatal(err)
	}
	if variant != 7032 || cost != 12.5 {
		t.Fatalf("return replaced original BOM or cost: %d %f", variant, cost)
	}
	for _, sql := range []string{
		`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,bom_spec_id,bom_variant_id,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_units,remaining_units)VALUES('FORGED','finished_product',910,7031,7032,0,'order_resolution_return',999,'RETURN-SOURCE-BATCH',1,1)`,
		`INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units)SELECT 'finished_product',910,7031,7032,0,'finished_goods','order_resolution_return',id,target_batch_code,5,3,8 FROM %[1]s.order_resolution_stock_returns`,
	} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatal("unbacked or reused historical return accepted")
		}
	}
}

func TestOrderResolutionPostedReturnRecordsAreImmutable(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "return_stock" {
			result, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "R-immutable", Note: "实际退回待检", PostingDate: "2026-11-08", IdempotencyKey: "immutable-return"}, "warehouse")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sql := range []string{`UPDATE %s.order_resolution_stock_returns SET quantity=quantity+1`, `DELETE FROM %s.order_resolution_stock_returns`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatal("posted return history rewritten")
		}
	}
}

func TestOrderResolutionReturnRejectsStaleOrUnapprovedStockAtomically(t *testing.T) {
	for _, tc := range []struct{ name, mutation string }{
		{"foreign destination inventory", `UPDATE %s.finished_inventory SET owner_customer_id=502 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods'`},
		{"changed source cost", `UPDATE %s.stock_batches SET unit_cost=13 WHERE id=78500`},
		{"missing original shipment", `DELETE FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment'`},
		{"disabled destination", `UPDATE %s.warehouses SET active=false WHERE code='finished_goods'`},
		{"audit failure", `CREATE FUNCTION %[1]s.reject_return_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='resolution_return_stock' THEN RAISE EXCEPTION 'forced return audit failure';END IF;RETURN NEW;END $$;CREATE TRIGGER reject_return_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_return_audit()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := orderReturnFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			current, err := r.Context(ctx, 78500)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78500, returnCommand(current.Version), "sales")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.mutation, schema)); err != nil {
				t.Fatal(err)
			}
			snapshot := func() string {
				t.Helper()
				var raw string
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM %[1]s.stock_batches b),'inventory',(SELECT jsonb_agg(to_jsonb(i) ORDER BY product_id,bom_spec_id,warehouse) FROM %[1]s.finished_inventory i),'ledger',(SELECT jsonb_agg(to_jsonb(l) ORDER BY id) FROM %[1]s.stock_ledger_entries l),'returns',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM %[1]s.order_resolution_stock_returns r),'credit',(SELECT resolution_credit_cents FROM %[1]s.orders WHERE id=78500))::text`, schema)).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				return raw
			}
			before := snapshot()
			for _, step := range result.Steps {
				if step.Kind == "return_stock" {
					_, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "R-negative", Note: "退回核对", PostingDate: "2026-11-08", IdempotencyKey: "invalid-return"}, "warehouse")
					if err == nil {
						t.Fatal("invalid return accepted")
					}
				}
			}
			if after := snapshot(); after != before {
				t.Fatal("failed return retained partial stock or receivable changes")
			}
		})
	}
}

func TestOrderResolutionDropshipReturnKeepsCustomerOwnershipAndPackage(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders SET fulfillment_mode='stock',source_warehouse='DEV-E2E-A1' WHERE id=78500;
 UPDATE %[1]s.stock_batches SET owner_customer_id=501 WHERE id=78500;
 UPDATE %[1]s.finished_inventory SET onhand_units=0 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods';
 UPDATE %[1]s.finished_inventory SET onhand_units=2 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 UPDATE %[1]s.stock_ledger_entries SET owner_customer_id=501,warehouse='DEV-E2E-A1' WHERE source_batch_code='RETURN-SOURCE-BATCH';
 UPDATE %[1]s.order_stock_batch_allocations SET warehouse='DEV-E2E-A1' WHERE order_id=78500;
 INSERT INTO %[1]s.customer_direct_ship_requests(id,request_no,customer_id,fulfillment_mode)VALUES(78500,'RETURN-DSR',501,'stock');
 INSERT INTO %[1]s.customer_direct_ship_request_orders(request_id,order_id,order_no,warehouse_code)VALUES(78500,78500,'RETURN-ORIGINAL','DEV-E2E-A1');`, schema)); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('request',(SELECT to_jsonb(r) FROM %[1]s.customer_direct_ship_requests r WHERE id=78500),'package',(SELECT to_jsonb(p) FROM %[1]s.customer_direct_ship_request_orders p WHERE order_id=78500),'deduction',(SELECT to_jsonb(d) FROM %[1]s.order_stock_deductions d WHERE id=78500))::text`, schema)).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := snapshot()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.ReturnSources) != 1 {
		t.Fatalf("missing customer-owned shipment: %+v", current.ReturnSources)
	}
	cmd := returnCommand(current.Version)
	cmd.ReturnSelections[0].Ownership = "customer"
	cmd.ReturnSelections[0].Warehouse = "DEV-E2E-A1"
	cmd.Dispositions = []domain.Disposition{{Kind: "direct_ship", ID: 78500, Choice: "keep_fulfilled"}}
	result, err := r.Submit(ctx, 78500, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		choice := step.Choice
		result, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: choice, Note: "保持客户货权并保留原包裹记录", Reference: "R-customer", PostingDate: "2026-11-08", IdempotencyKey: fmt.Sprintf("customer-return-%d", step.ID)}, "test")
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Applied || snapshot() != before {
		t.Fatal("customer return changed original package or shipment")
	}
	var owner, units int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT owner_customer_id,remaining_units FROM %s.stock_batches WHERE source_doc_type='order_resolution_return'`, schema)).Scan(&owner, &units); err != nil {
		t.Fatal(err)
	}
	if owner != 501 || units != 3 {
		t.Fatalf("return silently transferred ownership: %d %d", owner, units)
	}
}

func TestOrderResolutionReturnPreservesAllocatedHistoricalBatchForNewerOrder(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_id=910`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET bom_variant_id=7034 WHERE id=78500`, schema)); err != nil {
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
	for _, step := range result.Steps {
		if step.Kind == "return_stock" {
			result, err = r.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "原出库实物", Note: "保持原批次冻结身份与成本", PostingDate: "2026-11-08", IdempotencyKey: "frozen-return"}, "warehouse")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var variant int64
	var cost float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id,unit_cost FROM %s.stock_batches WHERE source_doc_type='order_resolution_return'`, schema)).Scan(&variant, &cost); err != nil {
		t.Fatal(err)
	}
	if variant != 7032 || cost != 12.5 {
		t.Fatalf("return replaced original BOM or cost: %d %f", variant, cost)
	}
	for _, sql := range []string{
		`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,bom_spec_id,bom_variant_id,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_units,remaining_units)VALUES('FORGED','finished_product',910,7031,7032,0,'order_resolution_return',999,'RETURN-SOURCE-BATCH',1,1)`,
		`INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units)SELECT 'finished_product',910,7031,7032,0,'finished_goods','order_resolution_return',id,target_batch_code,5,3,8 FROM %[1]s.order_resolution_stock_returns`,
	} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatal("unbacked or reused historical return accepted")
		}
	}
}

func TestOrderResolutionReturnCopiesExactStoredCost(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET unit_cost=12.34567890123456789 WHERE id=78500`, schema)); err != nil {
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
	if _, err = r.CompleteStep(ctx, 78500, result.ID, returnStepID(t, result, "return_stock"), resolutionpg.StepCommand{Choice: "receive", Note: "保持原成本", Reference: "COST-RETURN", PostingDate: "2026-11-08", IdempotencyKey: "exact-cost"}, "warehouse"); err != nil {
		t.Fatal(err)
	}
	var equal bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT returned.unit_cost=original.unit_cost FROM %[1]s.stock_batches returned CROSS JOIN %[1]s.stock_batches original WHERE original.id=78500 AND returned.source_doc_type='order_resolution_return'`, schema)).Scan(&equal); err != nil || !equal {
		t.Fatalf("original stored cost changed: %v %v", equal, err)
	}
}

func TestOrderReturnPreservesUnknownBatchCost(t *testing.T) {
	pool, schema := orderReturnFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET cost_unavailable=true WHERE id=78500`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	current, err := repo.Context(ctx, 78500)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Submit(ctx, 78500, returnCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Kind == "return_stock" {
			_, err = repo.CompleteStep(ctx, 78500, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "原出库", Note: "已确认实物", PostingDate: "2026-11-08", IdempotencyKey: "unknown-return"}, "warehouse")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var unavailable bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable FROM %s.stock_batches WHERE source_doc_type='order_resolution_return'`, schema)).Scan(&unavailable); err != nil || !unavailable {
		t.Fatalf("returned cost unavailable=%v err=%v", unavailable, err)
	}
}
