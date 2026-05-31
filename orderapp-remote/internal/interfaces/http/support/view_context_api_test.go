package support

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	authzapp "orderapp/internal/application/authz"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestViewContextOptionsAPIListsCustomersAndOrders(t *testing.T) {
	ctx := context.Background()
	pool, schema := newViewContextAPITestDB(t)
	seedViewContextAPITestData(t, ctx, pool, schema)

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(5))
			c.Set("actor", "内部运营")
			return next(c)
		}
	})
	RegisterRoutes(e, pool, schema, Dependencies{Authz: &fakeAuthzService{actor: authzapp.Actor{
		Name:        "内部运营",
		AccountType: AccountTypeInternalEmployee,
		Permissions: []string{"customers.read", "orders.read"},
	}}})

	customerPayload := requestViewContextOptions(t, e, "/api/view-context/options?type=customer&q=Kar")
	if len(customerPayload.Options) != 1 {
		t.Fatalf("customer options len=%d body=%s", len(customerPayload.Options), customerPayload.Raw)
	}
	if got := customerPayload.Options[0]; got.Type != "customer" || got.CustomerID != 11 || got.Label != "Karen" {
		t.Fatalf("customer option=%+v, want Karen customer #11", got)
	}

	orderPayload := requestViewContextOptions(t, e, "/api/view-context/options?type=order&q=SO-KAREN")
	if len(orderPayload.Options) != 1 {
		t.Fatalf("order options len=%d body=%s", len(orderPayload.Options), orderPayload.Raw)
	}
	if got := orderPayload.Options[0]; got.Type != "order" || got.OrderID != 101 || got.OrderNo != "SO-KAREN-001" || got.CustomerID != 11 || got.CustomerName != "Karen" {
		t.Fatalf("order option=%+v, want SO-KAREN-001 / Karen", got)
	}
	if strings.Contains(orderPayload.Raw, "SO-VOID-001") {
		t.Fatalf("voided orders must not be selectable as view context: %s", orderPayload.Raw)
	}
}

func TestViewContextOptionsAPIRejectsExternalCustomerCrossScope(t *testing.T) {
	ctx := context.Background()
	pool, schema := newViewContextAPITestDB(t)
	seedViewContextAPITestData(t, ctx, pool, schema)

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(77))
			c.Set("actor", "Karen账号")
			return next(c)
		}
	})
	RegisterRoutes(e, pool, schema, Dependencies{Authz: &fakeAuthzService{actor: authzapp.Actor{
		Name:        "Karen账号",
		AccountType: AccountTypeChannelCustomer,
		Permissions: []string{"customer_processing.read"},
	}}})

	ownPayload := requestViewContextOptions(t, e, "/api/view-context/options?type=customer&q=Karen")
	if len(ownPayload.Options) != 1 || ownPayload.Options[0].CustomerID != 11 {
		t.Fatalf("external customer own options=%s, want only bound customer 11", ownPayload.Raw)
	}

	for _, path := range []string{
		"/api/view-context/options?type=customer&customer_id=12",
		"/api/view-context/options?type=order&order_id=102",
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status=%d body=%s, want 403", path, rec.Code, rec.Body.String())
		}
	}
}

func TestViewContextPresetCRUDAuditsWrites(t *testing.T) {
	ctx := context.Background()
	pool, schema := newViewContextAPITestDB(t)
	seedViewContextAPITestData(t, ctx, pool, schema)

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(5))
			c.Set("actor", "内部运营")
			return next(c)
		}
	})
	RegisterRoutes(e, pool, schema, Dependencies{Authz: &fakeAuthzService{actor: authzapp.Actor{
		Name:        "内部运营",
		AccountType: AccountTypeInternalEmployee,
		Permissions: []string{"customers.read", "orders.read"},
	}}})

	createBody := `{"name":"Karen 履约视图","context_type":"customer","context_json":{"customer_id":11},"menu_keys_json":["customerFulfillment","orders"],"sort_order":3}`
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/view-context/presets", strings.NewReader(createBody))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Preset struct {
			ID int64 `json:"id"`
		} `json:"preset"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Preset.ID <= 0 {
		t.Fatalf("create response missing preset id: %s", createRec.Body.String())
	}

	updateBody := `{"name":"Karen 订单视图","context_type":"order","context_json":{"order_id":101,"customer_id":11},"menu_keys_json":["orders"],"sort_order":1}`
	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/view-context/presets/%d", created.Preset.ID), strings.NewReader(updateBody))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRec.Code, updateRec.Body.String())
	}

	disableRec := httptest.NewRecorder()
	e.ServeHTTP(disableRec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/view-context/presets/%d/disable", created.Preset.ID), nil))
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}

	var auditCount int
	err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.audit_logs
		WHERE entity_type='view_context_preset'
		  AND entity_id=$1
		  AND action IN ('create_view_context_preset','update_view_context_preset','disable_view_context_preset')
	`, schema), created.Preset.ID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit logs: %v", err)
	}
	if auditCount != 3 {
		t.Fatalf("auditCount=%d, want create/update/disable audit logs", auditCount)
	}
}

type viewContextOptionsResponse struct {
	Raw     string
	Options []struct {
		Type         string `json:"type"`
		ID           int64  `json:"id"`
		Label        string `json:"label"`
		CustomerID   int64  `json:"customer_id"`
		CustomerName string `json:"customer_name"`
		OrderID      int64  `json:"order_id"`
		OrderNo      string `json:"order_no"`
	} `json:"options"`
}

func requestViewContextOptions(t *testing.T, e *echo.Echo, path string) viewContextOptionsResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	var payload viewContextOptionsResponse
	payload.Raw = rec.Body.String()
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v body=%s", err, rec.Body.String())
	}
	return payload
}

func newViewContextAPITestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for view context API tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("view_context_api_%d", time.Now().UnixNano())
	mustExecViewContextAPISQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if err := EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureAuditTables: %v", err)
	}
	if err := ensureViewContextPresetTables(ctx, pool, schema); err != nil {
		t.Fatalf("ensureViewContextPresetTables: %v", err)
	}
	return pool, schema
}

func seedViewContextAPITestData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecViewContextAPISQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	company_name TEXT NOT NULL DEFAULT '',
	contact TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.orders (
	id BIGINT PRIMARY KEY,
	order_no TEXT NOT NULL DEFAULT '',
	order_date DATE,
	customer_id BIGINT NOT NULL DEFAULT 0,
	is_void BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.company_employees (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	account_type TEXT NOT NULL DEFAULT 'internal_employee',
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.customer_erp_user_bindings (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL,
	employee_id BIGINT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active'
);
INSERT INTO %s.customers(id,name,company_name,contact,phone,active) VALUES
	(11,'Karen','Karen Coffee','Karen','13800000011',true),
	(12,'Other','Other Coffee','Other','13800000012',true),
	(13,'Disabled','Disabled Coffee','','',false);
INSERT INTO %s.orders(id,order_no,order_date,customer_id,is_void) VALUES
	(101,'SO-KAREN-001','2026-05-31',11,false),
	(102,'SO-OTHER-001','2026-05-31',12,false),
	(103,'SO-VOID-001','2026-05-31',11,true);
INSERT INTO %s.company_employees(id,name,account_type,active) VALUES
	(5,'内部运营','internal_employee',true),
	(77,'Karen账号','channel_customer',true);
INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,status) VALUES
	(11,77,'active');
`, schema, schema, schema, schema, schema, schema, schema, schema))
}

func mustExecViewContextAPISQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec SQL failed: %v\n%s", err, sql)
	}
}
