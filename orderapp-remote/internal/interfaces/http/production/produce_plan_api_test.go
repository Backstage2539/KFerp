package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"

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

func TestProducePlanSummaryAPIReturnsExactYieldRateForRoastPlans(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曲奇拼配',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-PLAN-002','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',1,'袋','1000g',1,50,50);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8150);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		RoastPlans []struct {
			YieldRate float64 `json:"yield_rate"`
		} `json:"roast_plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.RoastPlans) == 0 {
		t.Fatalf("roast_plans empty: %s", rec.Body.String())
	}
	if payload.RoastPlans[0].YieldRate != 0.815 {
		t.Fatalf("roast plan yield_rate = %.4f, want 0.8150", payload.RoastPlans[0].YieldRate)
	}
}

func TestProducePlanTreatsDeclinedStockBatchDecisionAsProductionGap(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曲奇拼配',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-DECLINE-STOCK','2026-05-03',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',2,'袋','454g',1,50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES (1,454,'finished_goods',5,0);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (1,'produce','测试员');
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-DECLINE-STOCK"`, `"need_g":908`, `"inv_g":2270`, `"gap_g":908`, `"plan_rows"`, `"roast_plans"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("declined stock plan response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "库存充足") {
		t.Fatalf("declined stock plan should not return stock sufficient tip: %s", body)
	}
}

func TestProducePlanExcludesShippedOrdersWithBlankProcessStatus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曜石2.0',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.ship_statuses(name) VALUES ('未发货'),('已发货');
		ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_status_id BIGINT;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id,ship_status_id) VALUES
			(1,'SO-SHIPPED-BLANK','2026-03-28',false,NULL,(SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1)),
			(2,'SO-DECLINE-STOCK','2026-05-03',false,NULL,(SELECT id FROM %s.ship_statuses WHERE name='未发货' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'曜石2.0',80,'袋','454g',1,50,4000),
			(2,1,'曜石2.0',2,'袋','454g',1,50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES (1,454,'finished_goods',170,0);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (2,'produce','测试员');
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-DECLINE-STOCK"`, `"need_units":2`, `"need_g":908`, `"gap_g":908`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-SHIPPED-BLANK") || strings.Contains(body, `"need_units":82`) {
		t.Fatalf("shipped blank-process order should not inflate production need: %s", body)
	}
}

func TestProducePlanSkipsOrderItemsWithoutProductID(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曲奇拼配',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-UNLINKED-ITEM','2026-05-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(2,'SO-LINKED-ITEM','2026-05-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'历史未绑定商品',3,'袋','100g/袋',NULL,50,150),
			(2,1,'曲奇拼配',2,'袋','454g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-LINKED-ITEM"`, `"product_id":1`, `"need_units":2`, `"gap_g":908`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-UNLINKED-ITEM") || strings.Contains(body, "历史未绑定商品") {
		t.Fatalf("unlinked historical order item should not enter production plan: %s", body)
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
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	registerUnprodSummaryPages(e)
	registerUnprodSummaryAPI(e, productionSvc)
	registerProductionFlowPages(e, productionSvc)
	return e
}
