package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestCombinedDocumentRoutesRegisterPreviewGenerateAndDownload(t *testing.T) {
	e := echo.New()
	registerCombinedDocumentRoutes(e, salesapp.NewService(nil))
	routes := map[string]bool{}
	for _, route := range e.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/orders/combined/sales-order-preview",
		"GET /api/orders/combined/sales-order-preview.pdf",
		"POST /api/orders/combined/sales-orders",
		"GET /orders/combined/sales-orders/:doc_id.pdf",
		"GET /api/orders/combined/delivery-note-preview",
		"GET /api/orders/combined/delivery-note-preview.pdf",
		"POST /api/orders/combined/delivery-notes",
		"GET /orders/combined/delivery-notes/:doc_id.pdf",
	} {
		if !routes[want] {
			t.Fatalf("missing route %s", want)
		}
	}
}

func TestCombinedSalesOrderDocumentAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedCombinedDocumentOrders(t, ctx, pool, schema, true)
	e := newCombinedDocumentAPITestEcho(pool, schema, t.TempDir())

	previewReq := httptest.NewRequest(http.MethodGet, "/api/orders/combined/sales-order-preview?order_ids=1,2", nil)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("sales preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	var preview salesapp.CombinedSalesOrderPreview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.NextVersionNo != 1 || len(preview.Snapshot.Groups) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	for _, want := range []string{
		`"order_nos":["SO-20260520-0001","SO-20260521-0002"]`,
		`"customer_name":"测试客户"`,
		`"document_date":"2026-05-24"`,
		`"order_date":"2026-05-20"`,
		`"grand_total":"324.00"`,
	} {
		if !strings.Contains(previewRec.Body.String(), want) {
			t.Fatalf("sales preview missing %s: %s", want, previewRec.Body.String())
		}
	}

	previewPDFReq := httptest.NewRequest(http.MethodGet, "/api/orders/combined/sales-order-preview.pdf?order_ids=1,2", nil)
	previewPDFRec := httptest.NewRecorder()
	e.ServeHTTP(previewPDFRec, previewPDFReq)
	if previewPDFRec.Code != http.StatusOK || previewPDFRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("sales preview pdf status=%d content-type=%q body=%s", previewPDFRec.Code, previewPDFRec.Header().Get(echo.HeaderContentType), previewPDFRec.Body.String())
	}
	if !bytes.HasPrefix(previewPDFRec.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("sales preview pdf prefix=%q", previewPDFRec.Body.Bytes()[:min(len(previewPDFRec.Body.Bytes()), 8)])
	}

	generateReq := httptest.NewRequest(http.MethodPost, "/api/orders/combined/sales-orders", strings.NewReader(`{"order_ids":[1,2]}`))
	generateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	generateRec := httptest.NewRecorder()
	e.ServeHTTP(generateRec, generateReq)
	if generateRec.Code != http.StatusOK {
		t.Fatalf("sales generate status=%d body=%s", generateRec.Code, generateRec.Body.String())
	}
	var created salesapp.CombinedSalesOrderDocument
	if err := json.Unmarshal(generateRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode generated sales doc: %v", err)
	}
	if created.VersionNo != 1 || created.DownloadURL == "" || len(created.Snapshot.Groups) != 2 {
		t.Fatalf("generated sales doc = %+v", created)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, created.DownloadURL, nil)
	downloadRec := httptest.NewRecorder()
	e.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("sales download status=%d content-type=%q body=%s", downloadRec.Code, downloadRec.Header().Get(echo.HeaderContentType), downloadRec.Body.String())
	}
	assertCombinedAuditLog(t, ctx, pool, schema, "combined_sales_order_document", "SO-20260520-0001, SO-20260521-0002")
}

func TestCombinedDocumentsRejectCrossCustomerOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedCombinedDocumentOrders(t, ctx, pool, schema, true)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.customers(id, name) VALUES(8, '其他客户') ON CONFLICT (id) DO NOTHING`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET customer_id=8 WHERE id=2`, schema))
	e := newCombinedDocumentAPITestEcho(pool, schema, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/orders/combined/sales-order-preview?order_ids=1,2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "same customer") {
		t.Fatalf("cross-customer sales preview status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCombinedDeliveryNoteDocumentAPIRequiresShippedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedCombinedDocumentOrders(t, ctx, pool, schema, true)
	e := newCombinedDocumentAPITestEcho(pool, schema, t.TempDir())

	previewReq := httptest.NewRequest(http.MethodGet, "/api/orders/combined/delivery-note-preview?order_ids=1,2", nil)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("delivery preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	var preview salesapp.CombinedDeliveryNotePreview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode delivery preview: %v", err)
	}
	if preview.NextVersionNo != 1 || len(preview.Snapshot.Groups) != 2 {
		t.Fatalf("delivery preview = %+v", preview)
	}
	for _, want := range []string{
		`"delivery_note_no":"CDN-SO-20260520-0001-SO-20260521-0002"`,
		`"receiver_name":"张三"`,
		`"tracking_no":"SF002"`,
		`"order_date":"2026-05-21"`,
	} {
		if !strings.Contains(previewRec.Body.String(), want) {
			t.Fatalf("delivery preview missing %s: %s", want, previewRec.Body.String())
		}
	}

	generateReq := httptest.NewRequest(http.MethodPost, "/api/orders/combined/delivery-notes", strings.NewReader(`{"order_ids":[1,2]}`))
	generateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	generateRec := httptest.NewRecorder()
	e.ServeHTTP(generateRec, generateReq)
	if generateRec.Code != http.StatusOK {
		t.Fatalf("delivery generate status=%d body=%s", generateRec.Code, generateRec.Body.String())
	}
	var created salesapp.CombinedDeliveryNoteDocument
	if err := json.Unmarshal(generateRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode generated delivery doc: %v", err)
	}
	if created.VersionNo != 1 || created.DownloadURL == "" || len(created.Snapshot.Groups) != 2 {
		t.Fatalf("generated delivery doc = %+v", created)
	}
	downloadReq := httptest.NewRequest(http.MethodGet, created.DownloadURL, nil)
	downloadRec := httptest.NewRecorder()
	e.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("delivery download status=%d content-type=%q body=%s", downloadRec.Code, downloadRec.Header().Get(echo.HeaderContentType), downloadRec.Body.String())
	}
	assertCombinedAuditLog(t, ctx, pool, schema, "combined_delivery_note_document", "SO-20260520-0001, SO-20260521-0002")

	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=1 WHERE id=2`, schema))
	unshippedReq := httptest.NewRequest(http.MethodGet, "/api/orders/combined/delivery-note-preview?order_ids=1,2", nil)
	unshippedRec := httptest.NewRecorder()
	e.ServeHTTP(unshippedRec, unshippedReq)
	if unshippedRec.Code != http.StatusBadRequest || !strings.Contains(unshippedRec.Body.String(), "shipped") {
		t.Fatalf("unshipped delivery preview status=%d body=%s", unshippedRec.Code, unshippedRec.Body.String())
	}
}

func newCombinedDocumentAPITestEcho(pool *pgxpool.Pool, schema string, assetDir string) *echo.Echo {
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
	registerCombinedDocumentRoutes(e, svc)
	return e
}

func seedCombinedDocumentOrders(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, shipped bool) {
	t.Helper()
	shipStatusID := 1
	if shipped {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.ship_statuses(name)
			SELECT '已发货'
			WHERE NOT EXISTS (SELECT 1 FROM %s.ship_statuses WHERE name='已发货')`, schema, schema))
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.ship_statuses WHERE name='已发货' ORDER BY id LIMIT 1`, schema)).Scan(&shipStatusID); err != nil {
			t.Fatalf("query shipped status: %v", err)
		}
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address, taxpayer_id, bank_account_name, bank_name, bank_account_no)
		VALUES(1, '棵凡咖啡', '云南省普洱市孟连县', '91530827MACGJ29D6J', '孟连口加农业科技有限公司', '中国农业银行孟连支行', '6222000000000000')`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.orders(id, document_date, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, ship_method, ship_tracking_no, receiver_name, receiver_phone, receiver_address, total_amount, shipping_amount, discount_amount, grand_total, order_no, sales_order_note)
		VALUES
			(1, '2026-05-24', '2026-05-20', 3, 1, 2, 2, %d, '顺丰', 'SF001', '张三', '13800000001', '上海市一号路', 134, 0, 0, 134, 'SO-20260520-0001', '第一单备注'),
			(2, '2026-05-24', '2026-05-21', 3, 1, 2, 2, %d, '顺丰冷运', 'SF002', '李四', '13800000002', '上海市二号路', 190, 10, 10, 190, 'SO-20260521-0002', '第二单备注')`, schema, shipStatusID, shipStatusID))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, item_note, qty, unit, spec, unit_price, discount_amount, line_total)
		VALUES
			(1, 1, 7, '橘皮乌龙', '磨粉', 2, '件', '300g', 67, 0, 134),
			(2, 1, 7, '白月光-瑰夏', '贴标', 1, '件', '227g', 190, 0, 190)`, schema))
}

func assertCombinedAuditLog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, entityType, orderNos string) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs WHERE entity_type=$1 AND meta::text LIKE $2`, schema), entityType, "%"+orderNos+"%").Scan(&count); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if count == 0 {
		t.Fatalf("missing audit log entity_type=%s orderNos=%s", entityType, orderNos)
	}
}

var _ = salesdomain.NextCombinedDocumentVersion
