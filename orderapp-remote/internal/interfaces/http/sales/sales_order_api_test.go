package sales

import (
	"context"
	"encoding/json"
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

	body := strings.NewReader(`{"note":"请密封保存\n第二行","payment_text":"微信\n对公转账","seal_x_mm":42,"seal_y_mm":21,"seal_width_mm":38}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"note":"请密封保存\n第二行"`,
		`"payment_text":"微信\n对公转账"`,
		`"seal_x_mm":42`,
		`"seal_y_mm":21`,
		`"seal_width_mm":38`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("POST response missing %s: %s", want, rec.Body.String())
		}
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
	var created salesapp.SalesOrderDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode generated document: %v", err)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/orders/1/sales-order-latest.pdf", nil)
	downloadRec := httptest.NewRecorder()
	e.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("download status=%d content-type=%q body=%s", downloadRec.Code, downloadRec.Header().Get(echo.HeaderContentType), downloadRec.Body.String())
	}
	historyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/orders/1/sales-orders/%d.pdf", created.ID), nil)
	historyRec := httptest.NewRecorder()
	e.ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK || historyRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("history download status=%d content-type=%q body=%s", historyRec.Code, historyRec.Header().Get(echo.HeaderContentType), historyRec.Body.String())
	}
}

func TestSalesOrderPreviewAPIDoesNotCreateDocumentVersion(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderAPITestOrder(t, ctx, pool, schema)
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	previewReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-order-preview", nil)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	for _, want := range []string{
		`"next_version_no":1`,
		`"order_no":"SO-20260430-0008"`,
		`"customer_name":"测试客户"`,
		`"items":[`,
	} {
		if !strings.Contains(previewRec.Body.String(), want) {
			t.Fatalf("preview response missing %s: %s", want, previewRec.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-orders", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"rows":[]`) {
		t.Fatalf("list after preview status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	generateReq := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-orders", nil)
	generateRec := httptest.NewRecorder()
	e.ServeHTTP(generateRec, generateReq)
	if generateRec.Code != http.StatusOK || !strings.Contains(generateRec.Body.String(), `"version_no":1`) {
		t.Fatalf("generate after preview status=%d body=%s", generateRec.Code, generateRec.Body.String())
	}
}

func TestParseSalesOrderDocumentIDAcceptsPDFPathFallback(t *testing.T) {
	got, err := parseSalesOrderDocumentID("", "/orders/254/sales-orders/1.pdf")
	if err != nil {
		t.Fatalf("parse pdf fallback: %v", err)
	}
	if got != 1 {
		t.Fatalf("parse pdf fallback = %d, want 1", got)
	}
	got, err = parseSalesOrderDocumentID("2.pdf", "/orders/254/sales-orders/2.pdf")
	if err != nil {
		t.Fatalf("parse param with suffix: %v", err)
	}
	if got != 2 {
		t.Fatalf("parse param with suffix = %d, want 2", got)
	}
	if _, err := parseSalesOrderDocumentID("", "/orders/254/sales-orders/0.pdf"); err == nil {
		t.Fatal("parse invalid document id error = nil")
	}
}

func TestSalesOrderDocumentAPIUsesCustomerCompanyFallback(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderAPITestOrder(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.customers
		SET company_name='', company_address='上海市徐汇区', company_phone='021-12345678'
		WHERE id=3`, schema))
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"company_name":"测试客户"`,
		`"customer_company_name":"测试客户"`,
		`"customer_company_address":"上海市徐汇区"`,
		`"customer_company_phone":"021-12345678"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("POST response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestSalesOrderPreviewAPIUsesGlobalCompanyProfile(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderAPITestOrder(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name) VALUES(1, '棵凡咖啡')`, schema))
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-order-preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"company_name":"棵凡咖啡"`) {
		t.Fatalf("preview response should use global company profile name: %s", rec.Body.String())
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
