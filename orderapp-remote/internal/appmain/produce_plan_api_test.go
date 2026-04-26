package appmain

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曲奇拼配',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-PLAN-001','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',1,'袋','1000g',1,50,50);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES
			(10,'RAW-A','豆子A','bean','g',0,0,10,0),
			(11,'RAW-B','豆子B','bean','g',0,0,10,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES
			(1,10,0.7500),
			(1,11,0.2500);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"roast_plans"`, `"materials"`, `"plan_rows"`, `"final_input_g":2000`, `"qty":1500`, `"qty":500`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
}

func newProducePlanTestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	registerUnprodSummaryPages(e, pool, schema)
	registerUnprodSummaryAPI(e, pool, schema)
	registerProductionFlowPages(e, pool, schema)
	return e
}
