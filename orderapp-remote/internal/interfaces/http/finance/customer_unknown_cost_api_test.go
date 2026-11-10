package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	appfinance "orderapp/internal/application/finance"
	postgresfinance "orderapp/internal/infrastructure/postgres/finance"

	"github.com/labstack/echo/v4"
)

func TestCustomerReportWithUnknownProductionCost(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.production_batch_costs ADD COLUMN cost_unavailable boolean NOT NULL DEFAULT false;
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户一');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,total_amount,grand_total) VALUES(1,'TEST','2026-11-02',1,100,100),(2,'KNOWN','2026-12-02',1,100,100);
 INSERT INTO %[1]s.production_batch_costs(product_name,total_cost,created_at,cost_unavailable) VALUES('unknown',12,'2026-11-03',true),('known',20,'2026-12-03',false);
 INSERT INTO %[1]s.finance_monthly_reports(month,status,snapshot_json) VALUES('2026-11','closed','{"month":"2026-11","status":"closed","main_business_cost":777,"gross_profit":123}');`, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})
	get := func(path string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var result map[string]any
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		}
		return rec, result
	}
	_, customer := get("/api/finance/reports/2026-11?customer_id=1")
	if customer["cost_unavailable"] != true || customer["gross_profit"] != nil || customer["operating_net_profit"] != nil {
		t.Fatalf("customer report exposed definite profit: %+v", customer)
	}
	for _, suffix := range []string{"pdf", "excel"} {
		rec, _ := get("/api/finance/reports/2026-11/" + suffix + "?customer_id=1")
		if rec.Code == http.StatusOK {
			t.Fatalf("unknown cost export %s succeeded", suffix)
		}
	}
	_, drilldown := get("/api/finance/reports/2026-11/drilldown?customer_id=1")
	found := false
	for _, raw := range drilldown["sections"].([]any) {
		s := raw.(map[string]any)
		if s["section"] == "main_cost" {
			found = true
			if s["cost_unavailable"] != true || s["total"] != nil {
				t.Fatalf("customer drilldown exposed cost: %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("missing unknown-cost section")
	}
	_, known := get("/api/finance/reports/2026-12?customer_id=1")
	if known["cost_unavailable"] != false || known["gross_profit"] == nil {
		t.Fatalf("known customer cost was blocked: %+v", known)
	}
	_, history := get("/api/finance/reports/2026-11")
	if history["gross_profit"] != float64(123) || history["main_business_cost"] != float64(777) {
		t.Fatalf("closed global snapshot changed: %+v", history)
	}
}
