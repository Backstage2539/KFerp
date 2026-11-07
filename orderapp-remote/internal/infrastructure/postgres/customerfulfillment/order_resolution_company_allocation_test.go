package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	stockapp "orderapp/internal/application/stock"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"testing"
	"time"
)

func companyAllocationFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total) VALUES(78400,'ALLOC-SOURCE',501,90,90),(78401,'ALLOC-TARGET',502,90,90),(78402,'ALLOC-OTHER',501,60,60);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit,unit_price,line_total) VALUES(78400,78400,1,910,'咖啡',7031,7032,3,'袋',30,90),(78401,78401,1,910,'咖啡',7031,7032,3,'袋',30,90),(78402,78402,1,910,'咖啡',7031,7032,2,'袋',30,60);
 INSERT INTO %[1]s.production_plans(id,plan_no,status) VALUES(78400,'ALLOC-SHARED-PLAN','completed');
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,bom_spec_id,bom_variant_id,spec_g,planned_inventory_qty,order_nos) VALUES(78400,78400,910,'咖啡',7031,7032,0,5,'ALLOC-SOURCE,ALLOC-OTHER');
 INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,bom_spec_id,bom_variant_id,spec_g,need_g,status,started_at,finished_at) VALUES(78400,'ALLOC-RUN',910,'咖啡',7031,7032,0,0,'done',now(),now());
 INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,running_item_id,product_id,product_name,bom_spec_id,bom_variant_id,spec_g,planned_inventory_qty,order_nos,status) VALUES(78400,'ALLOC-WORK',78400,78400,78400,910,'咖啡',7031,7032,0,5,'ALLOC-SOURCE,ALLOC-OTHER','completed');
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,source_doc_type,source_doc_id,qty_units,remaining_units,quality_status) VALUES(78400,'ALLOC-COMPANY-BATCH','finished_product',910,'咖啡',7031,7032,0,0,'production_run',78400,5,5,'passed');
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units) VALUES(910,7031,7032,0,'finished_goods',0,5);
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES('finished_product',910,'咖啡',7031,7032,0,0,'finished_goods','production_run',78400,'ALLOC-COMPANY-BATCH',5,5);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	return pool, schema
}
func companyAllocationCommand(version string) domain.Command {
	return domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-08", Version: version, IdempotencyKey: "company-allocation", Dispositions: []domain.Disposition{{Kind: "production", ID: 78400, Choice: "allocate", StockSelections: []domain.StockSelection{{BatchID: 78400, TargetItemID: 78401, Quantity: 3}}}}}
}
func TestOrderResolutionCompanyAllocationPreservesSharedProductionAndStock(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	var dep domain.Dependency
	for _, d := range current.Dependencies {
		if d.Kind == "production" && d.ID == 78400 {
			dep = d
		}
	}
	if len(dep.CompanyStock) != 1 || dep.CompanyStock[0].BatchID != 78400 || dep.CompanyStock[0].AvailableQuantity != 3 {
		t.Fatalf("company allocation must expose only free stock after protecting the shared other demand: %+v", dep)
	}
	cmd := companyAllocationCommand(current.Version)
	snapshot := func() string {
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('stock',(SELECT to_jsonb(b) FROM %[1]s.stock_batches b WHERE id=78400),'inventory',(SELECT to_jsonb(i) FROM %[1]s.finished_inventory i WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods'),'production',(SELECT to_jsonb(p) FROM %[1]s.production_plans p WHERE id=78400),'work',(SELECT to_jsonb(w) FROM %[1]s.work_orders w WHERE id=78400),'other_order',(SELECT to_jsonb(o) FROM %[1]s.orders o WHERE id=78402),'other_item',(SELECT to_jsonb(i) FROM %[1]s.order_items i WHERE id=78402))::text`, schema)).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := snapshot()
	result, err := r.Submit(ctx, 78400, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 || result.Steps[0].Permission != "stock.resolve" {
		t.Fatalf("missing warehouse assignment confirmation: %+v", result)
	}
	confirm := resolutionpg.StepCommand{Choice: "allocate", Note: "原批次三袋分配给指定新需求，其他订单两袋保留", IdempotencyKey: "company-allocation-confirm"}
	for n := 0; n < 2; n++ {
		result, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, confirm, "warehouse")
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Applied || result.Status != "completed" || snapshot() != before {
		t.Fatal("assignment changed physical stock or shared production history")
	}
	var qty int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(allocated_units),0) FROM %s.order_stock_batch_allocations WHERE order_id=78401 AND batch_id=78400`, schema)).Scan(&qty); err != nil {
		t.Fatal(err)
	}
	if qty != 3 {
		t.Fatalf("target reservation=%d want 3", qty)
	}
	var process string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(s.name,'') FROM %[1]s.orders o LEFT JOIN %[1]s.order_process_statuses s ON s.id=o.process_status_id WHERE o.id=78401`, schema)).Scan(&process); err != nil {
		t.Fatal(err)
	}
	if process != "库存待发货" {
		t.Fatalf("fully allocated target still has production demand: %q", process)
	}
	_, err = stockapp.NewService(stockpg.NewRepository(pool, schema)).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Operator: "warehouse", Items: []stockapp.StockDocumentItemCommand{{ProductID: 910, ItemType: "finished_product", BomSpecID: 7031, BomVariantID: 7032, QtyUnits: 3, BatchCode: "ALLOC-COMPANY-BATCH", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}}})
	if err == nil {
		t.Fatal("transfer took the target order reservation")
	}
	options, err := stockpg.NewRepository(pool, schema).ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "finished_goods", ItemType: "finished_product", ItemID: 910, BomSpecID: 7031})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range options.Rows {
		if row.BatchID == 78400 && row.AvailableUnits != 2 {
			t.Fatalf("allocated stock still transferable: %+v", row)
		}
	}
}

func TestOrderResolutionCompanyAllocationRevalidatesAndRollsBack(t *testing.T) {
	cases := []struct{ name, change string }{
		{"physical shortage", `UPDATE %[1]s.stock_batches SET remaining_units=4 WHERE id=78400;UPDATE %[1]s.finished_inventory SET onhand_units=4 WHERE product_id=910 AND warehouse='finished_goods';UPDATE %[1]s.stock_ledger_entries SET qty_after_units=4 WHERE source_batch_code='ALLOC-COMPANY-BATCH'`},
		{"target customer changed", `UPDATE %[1]s.orders SET customer_id=501 WHERE id=78401`},
		{"shared demand unit differs", `UPDATE %[1]s.order_items SET unit='箱' WHERE id=78402`},
		{"target paused", `UPDATE %[1]s.orders SET fulfillment_paused=true WHERE id=78401`},
		{"target customer stock", `UPDATE %[1]s.orders SET customer_id=501,delivery_mode='customer_stock',delivery_warehouse_code='DEV-E2E-A1' WHERE id=78401`},
		{"target warehouse", `UPDATE %[1]s.orders SET source_warehouse='finished_shop' WHERE id=78401`},
		{"target converted unit", `UPDATE %[1]s.order_items SET unit='箱' WHERE id=78401`},
		{"other reserve", `INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,batch_id,batch_code,warehouse,allocated_units)VALUES(78401,78401,910,7031,7032,78400,'ALLOC-COMPANY-BATCH','finished_goods',1)`},
		{"quality held", `UPDATE %[1]s.stock_batches SET quality_status='hold' WHERE id=78400`},
		{"audit rollback", `CREATE FUNCTION %[1]s.fail_company_allocate_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='resolution_stock_allocate' THEN RAISE EXCEPTION 'forced stock allocation audit failure'; END IF; RETURN NEW; END $$;CREATE TRIGGER fail_company_allocate_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_company_allocate_audit()`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := companyAllocationFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			current, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78400, companyAllocationCommand(current.Version), "sales")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.change, schema)); err != nil {
				t.Fatal(err)
			}
			_, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "allocate", Note: "核对指定成品", IdempotencyKey: "company-guard"}, "warehouse")
			if err == nil {
				t.Fatal("unsafe allocation accepted")
			}
			var closed, credit int64
			var applied bool
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT i.resolution_closed_qty::bigint,o.resolution_credit_cents,r.applied FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id JOIN %[1]s.order_resolution_requests r ON r.order_id=o.id WHERE o.id=78400`, schema)).Scan(&closed, &credit, &applied); err != nil {
				t.Fatal(err)
			}
			if closed != 0 || credit != 0 || applied {
				t.Fatal("failed allocation left partially closed order")
			}
			if tc.name == "audit rollback" {
				assertMiniDirectShipCount(t, pool, schema, "order_stock_batch_allocations", "order_id=78401", 0)
			}
		})
	}
}

func TestOrderResolutionCompanyAllocationRequiresExplicitReservationRelease(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_stock_batch_allocations(id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,batch_id,batch_code,warehouse,allocated_units)VALUES(78400,78400,78400,910,7031,7032,78400,'ALLOC-COMPANY-BATCH','finished_goods',3)`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := companyAllocationCommand(current.Version)
	cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: "stock_reservation", ID: 78400, Choice: "release", Quantity: 3})
	result, err := r.Submit(ctx, 78400, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		result, err = r.CompleteStep(ctx, 78400, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "明确释放原预留并分配指定成品", IdempotencyKey: fmt.Sprintf("stock-release-%d", step.ID)}, "warehouse")
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Applied {
		t.Fatal("allocation not applied")
	}
	var oldQty, newQty int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT sum(allocated_units) FROM %[1]s.order_stock_batch_allocations WHERE order_id=78400),(SELECT sum(allocated_units) FROM %[1]s.order_stock_batch_allocations WHERE order_id=78401)`, schema)).Scan(&oldQty, &newQty); err != nil {
		t.Fatal(err)
	}
	if oldQty != 0 || newQty != 3 {
		t.Fatalf("source=%d target=%d", oldQty, newQty)
	}
}

func TestOrderResolutionCompanyAllocationAPIAndConcurrentConfirmation(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "company-stock-api")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(nil))
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(path string, body any, want int) []byte {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s: status %d expected %d: %s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := companyAllocationCommand(current.Version)
	missing := companyAllocationCommand(current.Version)
	missing.Dispositions[0].StockSelections = nil
	call("/api/orders/78400/resolution/preview", missing, 400)
	call("/api/orders/78400/resolution/preview", cmd, 200)
	var result resolutionpg.Result
	if err = json.Unmarshal(call("/api/orders/78400/resolution/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	var replay resolutionpg.Result
	if err = json.Unmarshal(call("/api/orders/78400/resolution/submit", cmd, 200), &replay); err != nil {
		t.Fatal(err)
	}
	if replay.ID != result.ID {
		t.Fatal("duplicate processing record")
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func(n int) {
			<-start
			_, err := r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "allocate", Note: "核对原批次和指定客户订单", IdempotencyKey: fmt.Sprintf("parallel-stock-%d", n)}, "warehouse")
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
	assertMiniDirectShipCount(t, pool, schema, "order_stock_batch_allocations", "order_id=78401", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_stock_allocate'", 1)
}

func TestOrderResolutionCompanyAllocationPartialTargetDoesNotMarkWholeOrderReady(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET qty=5,line_total=150 WHERE id=78401;UPDATE %[1]s.orders SET total_amount=150,grand_total=150 WHERE id=78401`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, companyAllocationCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	result, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "allocate", Note: "只分配三袋，目标订单另两袋仍未齐备", IdempotencyKey: "partial-target-stock"}, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	var ready bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(s.name,'')='库存待发货' FROM %[1]s.orders o LEFT JOIN %[1]s.order_process_statuses s ON s.id=o.process_status_id WHERE o.id=78401`, schema)).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("partially allocated target falsely marked ready")
	}
}

func TestOrderResolutionCompanyAllocationChangedTargetRequiresNewWarehouseConfirmation(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := companyAllocationCommand(current.Version)
	cmd.ExternalStatus = "unknown"
	result, err := r.Submit(ctx, 78400, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var priorID int64
	for _, step := range result.Steps {
		if step.Kind == "production" {
			priorID = step.ID
		}
	}
	result, err = r.CompleteStep(ctx, 78400, result.ID, priorID, resolutionpg.StepCommand{Choice: "allocate", Note: "仓库已核对，等待财务", IdempotencyKey: "stock-before-target-change"}, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatal("applied before required financial check")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET qty=5,line_total=150 WHERE id=78401;UPDATE %[1]s.orders SET total_amount=150,grand_total=150 WHERE id=78401`, schema)); err != nil {
		t.Fatal(err)
	}
	current, err = r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = current.Version
	cmd.IdempotencyKey = "company-target-refresh"
	result, err = r.Refresh(ctx, 78400, result.ID, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range result.Steps {
		if step.Kind == "production" && step.ID != priorID && step.Status == "pending" {
			found = true
		}
	}
	if !found {
		t.Fatal("old warehouse approval reused after the target order changed")
	}
}

func TestOrderResolutionCompanyAllocationRejectsTargetWithForeignOwnership(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET qty=5,line_total=150 WHERE id=78401;UPDATE %[1]s.orders SET total_amount=150,grand_total=150 WHERE id=78401;
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)VALUES(78403,'FOREIGN-OWNER','finished_product',910,'他人寄存',7031,7032,501,1,1,'passed');
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,owner_customer_id,warehouse,source_batch_code,qty_change_units,qty_after_units)VALUES('finished_product',910,'他人寄存',7031,7032,501,'DEV-E2E-A1','FOREIGN-OWNER',1,1);
 UPDATE %[1]s.finished_inventory SET onhand_units=1 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,batch_id,batch_code,warehouse,allocated_units)VALUES(78401,78401,910,7031,7032,78403,'FOREIGN-OWNER','DEV-E2E-A1',1)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Submit(ctx, 78400, companyAllocationCommand(current.Version), "sales"); err == nil {
		t.Fatal("target with inconsistent customer ownership accepted")
	}
}

func TestOrderResolutionCompanyAllocationPreservesFrozenHistoricalVariant(t *testing.T) {
	pool, schema := companyAllocationFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_type='product' AND output_id=910`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, companyAllocationCommand(current.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	result, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "allocate", Note: "原订单和真实批次均冻结旧版本", IdempotencyKey: "historical-company-stock"}, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	var variant int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.order_stock_batch_allocations WHERE order_id=78401`, schema)).Scan(&variant); err != nil {
		t.Fatal(err)
	}
	if !result.Applied || variant != 7032 {
		t.Fatalf("frozen source silently replaced: %+v variant=%d", result, variant)
	}
}

func TestOrderResolutionHistoricalCompanyAllocationRejectsWrongIdentity(t *testing.T) {
	for _, tc := range []struct{ name, before, values string }{
		{"wrong_line", "", "78400,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',1,0,'finished_goods'"},
		{"wrong_batch_code", "", "78401,78401,910,7031,7032,0,78400,'OTHER',1,0,'finished_goods'"},
		{"wrong_warehouse", "", "78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',1,0,'material'"},
		{"over_demand", "", "78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',4,0,'finished_goods'"},
		{"grams", "", "78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',1,1,'finished_goods'"},
		{"zero", "", "78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',0,0,'finished_goods'"},
		{"retained_target", "UPDATE %s.orders SET delivery_mode='customer_stock',delivery_warehouse_code='DEV-E2E-B1' WHERE id=78401", ""},
		{"dropship_target", "UPDATE %s.orders SET fulfillment_mode='stock' WHERE id=78401", ""},
		{"paused_target", "UPDATE %s.orders SET fulfillment_paused=true WHERE id=78401", ""},
		{"closed_target", "UPDATE %s.orders SET resolution_status='closed' WHERE id=78401", ""},
		{"foreign_owner", "UPDATE %s.stock_batches SET owner_customer_id=501 WHERE id=78400", ""},
		{"already_reserved", "INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_units,warehouse)VALUES(78402,78402,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',5,'finished_goods')", ""},
		{"already_fulfilled_target", "INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_units,warehouse)VALUES(78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',3,'finished_goods')", ""},
		{"wrong_unit", "UPDATE %s.order_items SET unit='箱' WHERE id=78401", ""},
		{"quality_hold", "UPDATE %s.stock_batches SET quality_status='hold' WHERE id=78400", ""},
		{"insufficient_batch", "UPDATE %s.stock_batches SET remaining_units=0 WHERE id=78400", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := companyAllocationFixture(t)
			ctx := context.Background()
			if tc.before != "" {
				if _, err := pool.Exec(ctx, fmt.Sprintf(tc.before, schema)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_id=910`, schema)); err != nil {
				t.Fatal(err)
			}
			if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
				t.Fatal(err)
			}
			values := tc.values
			if values == "" {
				values = "78401,78401,910,7031,7032,0,78400,'ALLOC-COMPANY-BATCH',1,0,'finished_goods'"
			}
			_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_units,allocated_g,warehouse)VALUES(%s)`, schema, values))
			if err == nil {
				t.Fatal("invalid frozen company allocation accepted")
			}
		})
	}
}
