package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProduceStartHandlerPersistsInputG(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-TEST-START','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'橘皮乌龙',2,'袋','227g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
	`, schema, schema, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	form := url.Values{
		"selected":      {"1-227"},
		"input_g_1_227": {"600"},
	}
	rec := serveProductionFlowForm(t, app, http.MethodPost, "/produce/start", form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /produce/start status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/produce/running?ok=1" {
		t.Fatalf("POST /produce/start Location = %q, want %q", got, "/produce/running?ok=1")
	}

	var inputG, plannedUnits, plannedLooseG int64
	var bomYieldRate float64
	var status, startedBy string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT input_g, bom_yield_rate, planned_units, planned_loose_g, status, started_by
		FROM %s.produce_running_items
		WHERE product_id=1 AND spec_g=227
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&inputG, &bomYieldRate, &plannedUnits, &plannedLooseG, &status, &startedBy); err != nil {
		t.Fatalf("query running item: %v", err)
	}
	if inputG != 600 {
		t.Fatalf("running item input_g = %d, want 600", inputG)
	}
	if math.Abs(bomYieldRate-0.82) > 0.0001 {
		t.Fatalf("running item bom_yield_rate = %.4f, want 0.8200", bomYieldRate)
	}
	if plannedUnits != 2 || plannedLooseG != 0 {
		t.Fatalf("planned inventory = %d units + %dg, want 2 units + 0g", plannedUnits, plannedLooseG)
	}
	if status != "running" {
		t.Fatalf("running item status = %q, want running", status)
	}
	if startedBy != "测试员" {
		t.Fatalf("running item started_by = %q, want 测试员", startedBy)
	}

	var processStatusName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(s.name,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses s ON s.id=o.process_status_id
		WHERE o.id=1
	`, schema, schema)).Scan(&processStatusName); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if processStatusName != "生产中" {
		t.Fatalf("order process_status = %q, want 生产中", processStatusName)
	}
}

func TestProduceFinishHandlerWritesProductionLog(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-TEST-FINISH','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1));
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-001','卡蒂姆水洗','bean','g',1000,0,54,0),
			(11,'BAG-227','227g豆袋','pack','个',0,10,1,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.packaging_spec_material_map(spec_g,material_id) VALUES (227,11);
		INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g) VALUES (1,227,1,0);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-TEST-001',1,'橘皮乌龙',227,454,'SO-TEST-FINISH','running',
			'测试员',now(),600,0.8200,2,0
		);
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	form := url.Values{
		"id":               {"1"},
		"finished_units":   {"2"},
		"finished_loose_g": {"10"},
	}
	rec := serveProductionFlowForm(t, app, http.MethodPost, "/produce/running/finish", form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /produce/running/finish status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/produce/running?ok=1" {
		t.Fatalf("POST /produce/running/finish Location = %q, want %q", got, "/produce/running?ok=1")
	}

	var finishedUnits, finishedLooseG, finishedTotalG, inputG int64
	var actualYield, bomYield float64
	var finishedBy, materialSummary string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT finished_units, finished_loose_g, finished_total_g, input_g,
		       actual_yield_rate, bom_yield_rate, finished_by, material_summary::text
		FROM %s.production_logs
		WHERE running_item_id=1
	`, schema)).Scan(&finishedUnits, &finishedLooseG, &finishedTotalG, &inputG, &actualYield, &bomYield, &finishedBy, &materialSummary); err != nil {
		t.Fatalf("query production log: %v", err)
	}
	if finishedUnits != 2 || finishedLooseG != 10 || finishedTotalG != 464 {
		t.Fatalf("production log output = %d units + %dg => %dg, want 2 units + 10g => 464g", finishedUnits, finishedLooseG, finishedTotalG)
	}
	if inputG != 600 {
		t.Fatalf("production log input_g = %d, want 600", inputG)
	}
	if math.Abs(actualYield-0.7733) > 0.0001 {
		t.Fatalf("production log actual_yield_rate = %.4f, want 0.7733", actualYield)
	}
	if math.Abs(bomYield-0.82) > 0.0001 {
		t.Fatalf("production log bom_yield_rate = %.4f, want 0.8200", bomYield)
	}
	if finishedBy != "测试员" {
		t.Fatalf("production log finished_by = %q, want 测试员", finishedBy)
	}
	var items []materialConsumptionSummaryItem
	if err := json.Unmarshal([]byte(materialSummary), &items); err != nil {
		t.Fatalf("unmarshal material_summary: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("material summary items = %d, want 2", len(items))
	}
	byName := map[string]materialConsumptionSummaryItem{}
	for _, item := range items {
		byName[item.MaterialName] = item
	}
	rawItem, ok := byName["卡蒂姆水洗"]
	if !ok || rawItem.DeductG != 600 {
		t.Fatalf("raw material summary = %+v, want deduct_g=600", rawItem)
	}
	bagItem, ok := byName["227g豆袋"]
	if !ok || bagItem.DeductUnits != 2 {
		t.Fatalf("bag material summary = %+v, want deduct_units=2", bagItem)
	}

	var runningStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.produce_running_items WHERE id=1`, schema)).Scan(&runningStatus); err != nil {
		t.Fatalf("query running status: %v", err)
	}
	if runningStatus != "done" {
		t.Fatalf("running item status = %q, want done", runningStatus)
	}

	var invUnits, invLooseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=1 AND spec_g=227`, schema)).Scan(&invUnits, &invLooseG); err != nil {
		t.Fatalf("query finished inventory: %v", err)
	}
	if invUnits != 3 || invLooseG != 10 {
		t.Fatalf("finished inventory = %d units + %dg, want 3 units + 10g", invUnits, invLooseG)
	}

	var rawOnhandG, bagOnhandUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=10`, schema)).Scan(&rawOnhandG); err != nil {
		t.Fatalf("query raw material: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.materials WHERE id=11`, schema)).Scan(&bagOnhandUnits); err != nil {
		t.Fatalf("query bag material: %v", err)
	}
	if rawOnhandG != 400 {
		t.Fatalf("raw material onhand_g = %d, want 400", rawOnhandG)
	}
	if bagOnhandUnits != 8 {
		t.Fatalf("bag material onhand_units = %d, want 8", bagOnhandUnits)
	}

	var processStatusName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(s.name,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses s ON s.id=o.process_status_id
		WHERE o.id=1
	`, schema, schema)).Scan(&processStatusName); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if processStatusName != "生产完成" {
		t.Fatalf("order process_status = %q, want 生产完成", processStatusName)
	}
}

func newProductionFlowTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for production flow API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_prod_flow_%d", time.Now().UnixNano())
	mustExecProductionFlowTestSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, productionFlowTestBaseDDL(schema))
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatalf("ensureAppSchema: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}

func newProductionFlowTestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	registerProductionFlowPages(e, pool, schema)
	return e
}

func serveProductionFlowForm(t *testing.T, e *echo.Echo, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func mustExecProductionFlowTestSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}

func productionFlowTestBaseDDL(schema string) string {
	return fmt.Sprintf(`
		CREATE TABLE %s.products (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			default_price NUMERIC NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %s.product_price_tiers (
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT REFERENCES %s.products(id) ON DELETE CASCADE,
			min_qty_lb NUMERIC,
			max_qty_lb NUMERIC,
			price_per_lb NUMERIC(12,2),
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %s.order_process_statuses (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %s.orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT,
			order_date DATE,
			is_void BOOLEAN NOT NULL DEFAULT false,
			process_status_id INTEGER REFERENCES %s.order_process_statuses(id)
		);
		CREATE TABLE %s.order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT REFERENCES %s.orders(id) ON DELETE CASCADE,
			line_no INTEGER,
			item_name TEXT,
			qty NUMERIC,
			unit TEXT,
			spec TEXT,
			product_id BIGINT REFERENCES %s.products(id),
			unit_price NUMERIC,
			line_total NUMERIC,
			price_overridden BOOLEAN NOT NULL DEFAULT false
		);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema)
}
