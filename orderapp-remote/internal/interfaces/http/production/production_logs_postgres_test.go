package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	authzpg "orderapp/internal/infrastructure/postgres/authz"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	"os"
	"testing"
	"time"
)

func seedPR659Logs(t *testing.T) (*echo.Echo, string, func()) {
	t.Helper()
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, `SET TIME ZONE 'UTC'`)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.products(id,name) VALUES(659,'隔离榛巧拼配');
 INSERT INTO %[1]s.production_logs(running_item_id,batch_id,product_id,product_name,spec_g,order_nos,planned_need_g,input_g,finished_units,finished_loose_g,finished_total_g,actual_yield_rate,started_by,started_at,finished_by,finished_at,inventory_units_before,inventory_units_after,material_summary)
 SELECT 659,'PB-LOG-'||n,659,'隔离榛巧拼配',454,'SO-LOG-'||n,908,1000,2,10,918,0.918,'验收员工A','2026-09-12 23:30+08'::timestamptz,'验收员工B','2026-09-13 00:15+08'::timestamptz,5,7,'[{"material_name":"验收生豆","deduct_g":1000,"unit":"kg","batch_code":"MB-LOG-659"},{"material_name":"包装袋","deduct_units":2,"unit":"个","batch_code":"MB-BAG-659"}]'::jsonb FROM generate_series(1,205) n;
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units) VALUES('FP-LOG-659','finished_product',659,'隔离榛巧拼配',454,'production_run',659,918,2,918,2);
 INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,product_name,status) VALUES(659,'WO-LOG-659',659,'PB-LOG',659,'隔离榛巧拼配','completed');
 `, schema))
	e := echo.New()
	registerProductionLogPages(e, productionapp.NewService(productionpg.NewRepository(pool, schema)))
	return e, schema, func() {
		ready := os.Getenv("ORDERAPP_LOG_ACCEPTANCE_READY")
		if ready == "" {
			return
		}
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.customer_processing_production_demands RENAME TO fixture_minimal_demands`, schema))
		if err := authzpg.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte("orderapp-mobile-auth:Fixture659!"))
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.company_departments(id,name,active) VALUES(659,'隔离日志验收',true); INSERT INTO %[1]s.company_employees(id,name,phone,department_id,active,account_type) VALUES(6591,'隔离日志验收员','19900006591',659,true,'internal_employee'); INSERT INTO %[1]s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password) VALUES(6591,'%[2]s',false,false); INSERT INTO %[1]s.employee_roles(employee_id,role_code) VALUES(6591,'admin');`, schema, hex.EncodeToString(hash[:])))
		if err := os.WriteFile(ready, []byte(schema), 0600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Second)
			if _, err := os.Stat(ready); os.IsNotExist(err) {
				return
			}
		}
	}
}

func TestPR659LogsPostgresPaginationSearchAndContext(t *testing.T) {
	e, _, _ := seedPR659Logs(t)
	request := func(query string) map[string]any {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/produce/logs?"+query, nil))
		if rec.Code != 200 {
			t.Fatalf("%s: %s", query, rec.Body.String())
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := request("limit=20&page=1")
	if request("from=2026-09-13&to=2026-09-13")["total"] != float64(205) {
		t.Fatal("Beijing midnight records missing from selected day")
	}
	last := request("limit=20&page=11")
	if first["total"] != float64(205) || len(first["rows"].([]any)) != 20 || len(last["rows"].([]any)) != 5 {
		t.Fatalf("wrong paging: first=%v last=%v", first, last)
	}
	if first["rows"].([]any)[0].(map[string]any)["batch_id"] != "PB-LOG-205" || last["rows"].([]any)[4].(map[string]any)["batch_id"] != "PB-LOG-1" {
		t.Fatal("unstable order or inaccessible older records")
	}
	for _, q := range []string{"batch_id=PB-LOG-2", "q=SO-LOG-205", "work_order_id=659&running_item_id=659&batch_id=PB-LOG-2"} {
		if request(q)["total"] != float64(1) {
			t.Fatalf("scope not honored: %s", q)
		}
	}
	if request("q=FP-LOG-659")["total"] != float64(205) {
		t.Fatal("finished batch search unavailable")
	}
	for _, q := range []string{"q=%25", "operator=unknown", "work_order_id=9999", "from=2026-09-14", "to=2026-09-12"} {
		if request(q)["total"] != float64(0) {
			t.Fatalf("unexpected records: %s", q)
		}
	}
	if request("page=999999999&limit=20")["page"] != float64(11) {
		t.Fatal("out-of-range page not clamped")
	}
	row := request("batch_id=PB-LOG-2")["rows"].([]any)[0].(map[string]any)
	if row["input_g"] != float64(1000) || row["finished_total_g"] != float64(918) || row["finished_batch_code"] != "FP-LOG-659" {
		t.Fatalf("snapshot changed: %v", row)
	}
}

func TestPR659LogsBrowserFixture(t *testing.T) {
	if os.Getenv("ORDERAPP_LOG_ACCEPTANCE_READY") == "" {
		t.Skip("opt-in isolated browser fixture")
	}
	_, _, wait := seedPR659Logs(t)
	wait()
}
