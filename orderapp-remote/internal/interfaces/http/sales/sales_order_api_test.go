package sales

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestSalesOrderSettingsAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	body := strings.NewReader(`{"company_name":"浅焙作坊咖啡","note":"请密封保存","payment_text":"微信或对公转账"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"company_name":"浅焙作坊咖啡"`) {
		t.Fatalf("POST response = %s", rec.Body.String())
	}
}

func TestSalesOrderDocumentAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderAPITestOrder(t, ctx, pool, schema)
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	settingsBody := strings.NewReader(`{"company_name":"浅焙作坊咖啡","note":"请密封保存","payment_text":"微信或对公转账"}`)
	settingsReq := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order", settingsBody)
	settingsReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	settingsRec := httptest.NewRecorder()
	e.ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("settings status=%d body=%s", settingsRec.Code, settingsRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"version_no":1`) {
		t.Fatalf("POST response = %s", rec.Body.String())
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/orders/1/sales-order-latest.pdf", nil)
	downloadRec := httptest.NewRecorder()
	e.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("download status=%d content-type=%q body=%s", downloadRec.Code, downloadRec.Header().Get(echo.HeaderContentType), downloadRec.Body.String())
	}
}

func newSalesOrderAPITestEcho(pool *pgxpool.Pool, schema string, assetDir string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema, postgressales.WithSalesOrderAssetDir(assetDir)))
	registerSalesOrderSettingsRoutes(e, svc)
	registerSalesOrderDocumentRoutes(e, svc)
	return e
}

func seedSalesOrderAPITestOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.orders(id, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, total_amount, shipping_amount, discount_amount, grand_total, order_no)
		VALUES(1, '2026-04-30', 3, 1, 2, 1, 1, 134, 0, 0, 134, 'SO-20260430-0008')`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total)
		VALUES(1, 1, 7, '橘皮乌龙', 2, '件', '300g', 67, 134)`, schema))
}
