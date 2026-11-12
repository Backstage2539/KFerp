package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	appfinance "orderapp/internal/application/finance"
	postgresfinance "orderapp/internal/infrastructure/postgres/finance"
)

// Exercise the actual HTTP handlers with 200 orders and overlapping production
// and shipped batches. Time is logged for deployment audits, not gated on CPU.
func TestDailyFinanceHTTPAt200Orders(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`
 ALTER TABLE %[1]s.order_items ADD COLUMN bom_spec_id bigint NOT NULL DEFAULT 0, ADD COLUMN bom_variant_id bigint NOT NULL DEFAULT 0, ADD COLUMN material_unit_weight_g numeric, ADD COLUMN weight_snapshot_json jsonb;
 CREATE TABLE %[1]s.stock_batches(id bigint PRIMARY KEY,batch_code text,qty_g bigint,unit_cost numeric,cost_unavailable boolean,owner_customer_id bigint,source_doc_type text,source_doc_id bigint,item_type text,item_id bigint,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.order_stock_deductions(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,batch_id bigint,batch_code text,deducted_units bigint,deducted_g bigint);
 CREATE TABLE %[1]s.produce_running_items(id bigint PRIMARY KEY,batch_id text,product_id bigint,spec_g bigint,output_type text,status text,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.produce_batch_allocations(id bigint PRIMARY KEY,order_item_id bigint,batch_id text,product_id bigint,spec_g bigint,allocated_units bigint);
 ALTER TABLE %[1]s.production_batch_costs ADD COLUMN running_item_id bigint, ADD COLUMN finished_g bigint NOT NULL DEFAULT 0, ADD COLUMN unit_cost_per_kg numeric NOT NULL DEFAULT 0, ADD COLUMN cost_unavailable boolean NOT NULL DEFAULT false;
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,grand_total) SELECT n,'SO-'||n,DATE '2026-11-01'+(n%%12),1,100 FROM generate_series(1,200) n;
 INSERT INTO %[1]s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id,qty,item_name,unit,material_unit_weight_g,weight_snapshot_json) SELECT n,n,1,2,3,1,'咖啡','件',1000,'{"historical_unknown":false}' FROM generate_series(1,200) n;
 INSERT INTO %[1]s.produce_running_items SELECT n,'P-'||n,1,1000,'product','done',2,3 FROM generate_series(1,200) n;
 INSERT INTO %[1]s.production_batch_costs(running_item_id,product_name,total_cost,finished_g,unit_cost_per_kg,created_at) SELECT n,'咖啡',40,1000,40,'2026-11-01' FROM generate_series(1,200) n;
 INSERT INTO %[1]s.produce_batch_allocations SELECT n,n,'P-'||n,1,1000,1 FROM generate_series(1,200) n;
 INSERT INTO %[1]s.stock_batches SELECT n,'B-'||n,1000,40,false,0,'production_run',n,'finished_product',1,2,3 FROM generate_series(1,200) n;
 INSERT INTO %[1]s.order_stock_deductions SELECT n,n,1,2,3,1000,n,'B-'||n,1,1000 FROM generate_series(1,200) n;`, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})
	for _, path := range []string{"/api/finance/dashboard?month=2026-11", "/api/finance/daily-ledger?month=2026-11&page=1&page_size=20"} {
		started := time.Now()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		elapsed := time.Since(started)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if report, ok := response["report"].(map[string]any); ok && (report["main_business_cost"] != float64(8000) || report["cost_unavailable"] != false) {
			t.Fatalf("dashboard: %+v", report)
		}
		if total, ok := response["total"]; ok && total != float64(400) {
			t.Fatalf("ledger total=%v", total)
		}
		t.Logf("orders=200 path=%s elapsed=%s", path, elapsed)
	}
}
