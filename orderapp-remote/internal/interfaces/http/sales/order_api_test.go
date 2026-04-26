package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	support "orderapp/internal/interfaces/http/support"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestOrderEntryRedirectsToVueShell(t *testing.T) {
	e := echo.New()
	registerOrderRoutes(e, nil, "public")

	req := httptest.NewRequest(http.MethodGet, "/order?edit_id=9", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("GET /order status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "/vue-shell?view=order&edit_id=9" {
		t.Fatalf("GET /order Location = %q, want Vue order shell with edit_id", got)
	}
}

func TestOrderAPIFormReturnsRetailSpecs(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"products"`, `"retail_specs":[227,250]`, `"retail_price_227g":50`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIListUsesSalesReadModel(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (9, 'SO-API-LIST', '2026-04-26', 3, 2, 1, 1, 1, 123.45, false);
		INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value)
		VALUES (9, '测试员', 'create', '', 'SO-API-LIST');
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"rows"`, `"order_no":"SO-API-LIST"`, `"summary"`, `"order_types"`, `"process_statuses"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPISavesRetailCustomSpecPrice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-25",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  2,
		"pay_status_id":  1,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"unit_price":     []string{""},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"300"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var spec string
	var lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(spec,''), COALESCE(line_total,0)
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&spec, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if spec != "300g" {
		t.Fatalf("saved spec = %q, want 300g", spec)
	}
	if lineTotal != 134 {
		t.Fatalf("line_total = %.2f, want 134.00", lineTotal)
	}
}

func newOrderAPITestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for order API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_order_api_%d", time.Now().UnixNano())
	mustExecOrderAPITestSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	mustExecOrderAPITestSQL(t, ctx, pool, orderAPITestDDL(schema))
	if err := support.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatalf("ensureAuditTables: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}

func newOrderAPITestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, pool, schema)
	return e
}

func seedOrderAPITestData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id,name,active) VALUES (3,'测试客户',true);
		INSERT INTO %s.sources(id,name) VALUES (1,'小程序');
		INSERT INTO %s.order_types(id,name) VALUES (1,'批发订单'),(2,'零售订单');
		INSERT INTO %s.pay_statuses(id,name) VALUES (1,'未付款');
		INSERT INTO %s.ship_statuses(id,name) VALUES (1,'未发货');
		INSERT INTO %s.order_process_statuses(id,name,sort,active) VALUES (1,'待处理',10,true);
		INSERT INTO %s.products(id,name,default_price,active,retail_price_227g,retail_price_250g)
		VALUES (7,'橘皮乌龙',50,true,50,56);
	`, schema, schema, schema, schema, schema, schema, schema))
}

func mustExecOrderAPITestSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}

func orderAPITestDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.sources (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.order_types (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.pay_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.ship_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.order_process_statuses (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	sort INTEGER NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.products (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	roast_level TEXT NOT NULL DEFAULT '',
	default_price NUMERIC NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	retail_price_100g NUMERIC NOT NULL DEFAULT 0,
	retail_price_200g NUMERIC NOT NULL DEFAULT 0,
	retail_price_227g NUMERIC NOT NULL DEFAULT 0,
	retail_price_250g NUMERIC NOT NULL DEFAULT 0
);
CREATE TABLE %s.product_price_tiers (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT,
	spec_g INTEGER NOT NULL DEFAULT 454,
	min_qty_units NUMERIC,
	max_qty_units NUMERIC,
	price_per_unit NUMERIC,
	min_qty_lb NUMERIC,
	max_qty_lb NUMERIC,
	price_per_lb NUMERIC,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.orders (
	id BIGSERIAL PRIMARY KEY,
	order_date DATE,
	customer_id BIGINT,
	source_id BIGINT,
	order_type_id BIGINT,
	pay_status_id BIGINT,
	ship_status_id BIGINT,
	ship_method TEXT,
	ship_tracking_no TEXT,
	notes TEXT,
	total_amount NUMERIC NOT NULL DEFAULT 0,
	shipping_amount NUMERIC NOT NULL DEFAULT 0,
	discount_amount NUMERIC NOT NULL DEFAULT 0,
	round_to_int BOOLEAN NOT NULL DEFAULT false,
	rounding_amount NUMERIC NOT NULL DEFAULT 0,
	grand_total NUMERIC NOT NULL DEFAULT 0,
	express_fee TEXT,
	outsource_material_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_roast_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_packaging_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_manual_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_tax_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_other_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_total_fee NUMERIC NOT NULL DEFAULT 0,
	order_no TEXT,
	is_void BOOLEAN NOT NULL DEFAULT false,
	voided_at TIMESTAMPTZ,
	void_reason TEXT,
	process_status_id INTEGER,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT,
	line_no INTEGER,
	product_id BIGINT,
	price_tier_id BIGINT,
	price_overridden BOOLEAN NOT NULL DEFAULT false,
	item_name TEXT,
	qty NUMERIC,
	unit TEXT,
	spec TEXT,
	unit_price NUMERIC NOT NULL DEFAULT 0,
	line_total NUMERIC NOT NULL DEFAULT 0
);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
}
