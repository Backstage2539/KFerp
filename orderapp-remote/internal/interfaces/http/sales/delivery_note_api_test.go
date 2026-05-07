package sales

import (
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

func TestDeliveryNoteDocumentAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteAPITestOrder(t, ctx, pool, schema, true)
	e := newDeliveryNoteAPITestEcho(pool, schema, t.TempDir())

	saveReq := httptest.NewRequest(http.MethodPost, "/api/orders/1/delivery-note", strings.NewReader(`{"posting_date":"2026-05-02","source_warehouse":"finished_goods","delivery_method":"顺丰冷运","tracking_no":"SF123456789","note":"随货附出库单"}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}

	previewReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/delivery-note-preview", nil)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	for _, want := range []string{
		`"next_version_no":1`,
		`"delivery_note_no":"DN-SO-20260502-0001"`,
		`"source_warehouse":"finished_goods"`,
		`"delivery_method":"顺丰冷运"`,
		`"tracking_no":"SF123456789"`,
		`"items":[`,
	} {
		if !strings.Contains(previewRec.Body.String(), want) {
			t.Fatalf("preview response missing %s: %s", want, previewRec.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/delivery-notes", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"rows":[]`) {
		t.Fatalf("list after preview status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	generateReq := httptest.NewRequest(http.MethodPost, "/api/orders/1/delivery-notes", nil)
	generateRec := httptest.NewRecorder()
	e.ServeHTTP(generateRec, generateReq)
	if generateRec.Code != http.StatusOK || !strings.Contains(generateRec.Body.String(), `"version_no":1`) {
		t.Fatalf("generate status=%d body=%s", generateRec.Code, generateRec.Body.String())
	}
	var created salesapp.DeliveryNoteDocument
	if err := json.Unmarshal(generateRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode generated document: %v", err)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/orders/1/delivery-note-latest.pdf", nil)
	downloadRec := httptest.NewRecorder()
	e.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("download status=%d content-type=%q body=%s", downloadRec.Code, downloadRec.Header().Get(echo.HeaderContentType), downloadRec.Body.String())
	}
	historyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/orders/1/delivery-notes/%d.pdf", created.ID), nil)
	historyRec := httptest.NewRecorder()
	e.ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK || historyRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("history download status=%d content-type=%q body=%s", historyRec.Code, historyRec.Header().Get(echo.HeaderContentType), historyRec.Body.String())
	}
}

func TestDeliveryNotePreviewIncludesConfiguredSeal(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteAPITestOrder(t, ctx, pool, schema, true)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sales_order_assets(id, kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES(201, 'seal', 'seal.png', 'image/png', 128, 'abc', 'sales-order/seal/seal.png', '测试员');
		INSERT INTO %s.sales_order_settings(id, company_name, seal_asset_id, seal_x_mm, seal_y_mm, seal_width_mm, updated_by)
		VALUES(1, '棵凡咖啡', 201, 48, 9, 30, '测试员')
		ON CONFLICT(id) DO UPDATE SET seal_asset_id=excluded.seal_asset_id, seal_x_mm=excluded.seal_x_mm, seal_y_mm=excluded.seal_y_mm, seal_width_mm=excluded.seal_width_mm;
	`, schema, schema))

	e := newDeliveryNoteAPITestEcho(pool, schema, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/orders/1/delivery-note-preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var preview struct {
		Snapshot salesdomain.DeliveryNoteSnapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Snapshot.Seal == nil {
		t.Fatalf("delivery note preview missing configured seal: %s", rec.Body.String())
	}
	if preview.Snapshot.Seal.ID != 201 || preview.Snapshot.Seal.XMM != 48 || preview.Snapshot.Seal.YMM != 9 || preview.Snapshot.Seal.WidthMM != 30 {
		t.Fatalf("delivery note seal = %+v", preview.Snapshot.Seal)
	}
	if preview.Snapshot.Seal.URL != "/assets/sales-order/seal/seal.png" {
		t.Fatalf("delivery note seal URL = %q", preview.Snapshot.Seal.URL)
	}
}

func TestDeliveryNoteRequiresShippedOrder(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteAPITestOrder(t, ctx, pool, schema, false)
	e := newDeliveryNoteAPITestEcho(pool, schema, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/orders/1/delivery-note-preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "order must be shipped") {
		t.Fatalf("unshipped preview status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestParseDeliveryNoteDocumentIDAcceptsPDFPathFallback(t *testing.T) {
	got, err := parseDeliveryNoteDocumentID("", "/orders/254/delivery-notes/1.pdf")
	if err != nil {
		t.Fatalf("parse pdf fallback: %v", err)
	}
	if got != 1 {
		t.Fatalf("parse pdf fallback = %d, want 1", got)
	}
	got, err = parseDeliveryNoteDocumentID("2.pdf", "/orders/254/delivery-notes/2.pdf")
	if err != nil {
		t.Fatalf("parse param with suffix: %v", err)
	}
	if got != 2 {
		t.Fatalf("parse param with suffix = %d, want 2", got)
	}
	if _, err := parseDeliveryNoteDocumentID("", "/orders/254/delivery-notes/0.pdf"); err == nil {
		t.Fatal("parse invalid document id error = nil")
	}
}

func newDeliveryNoteAPITestEcho(pool *pgxpool.Pool, schema string, assetDir string) *echo.Echo {
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
	registerDeliveryNoteDocumentRoutes(e, svc)
	return e
}

func seedDeliveryNoteAPITestOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, shipped bool) {
	t.Helper()
	statusName := "未发货"
	trackingNo := ""
	if shipped {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.ship_statuses(name)
			SELECT '已发货'
			WHERE NOT EXISTS (SELECT 1 FROM %s.ship_statuses WHERE name='已发货')`, schema, schema))
		statusName = "已发货"
		trackingNo = "SF123456789"
	}
	shipStatusID := 1
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.ship_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), statusName).Scan(&shipStatusID); err != nil {
		t.Fatalf("query ship status %q: %v", statusName, err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县')`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.orders(id, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, ship_method, ship_tracking_no, total_amount, shipping_amount, discount_amount, grand_total, order_no)
		VALUES(1, '2026-05-02', 3, 1, 2, 2, %d, '顺丰', '%s', 134, 0, 0, 134, 'SO-20260502-0001')`, schema, shipStatusID, trackingNo))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total)
		VALUES(1, 1, 7, '橘皮乌龙', 2, '件', '300g', 67, 134)`, schema))
}
