package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"os"
	"path/filepath"
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

func TestSalesOrderSettingsRegistersSealToolRoutes(t *testing.T) {
	e := echo.New()
	registerSalesOrderSettingsRoutes(e, salesapp.NewService(nil), t.TempDir())
	routes := map[string]bool{}
	for _, route := range e.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"POST /api/settings/sales-order/seal-position",
		"POST /api/settings/sales-order/seal/remove-background",
	} {
		if !routes[want] {
			t.Fatalf("missing route %s", want)
		}
	}
}

func TestRemoveSealImageBackgroundTurnsLightNeutralPixelsTransparent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seal.png")
	writeSealWithWhiteBackground(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := removeSealImageBackground(data)
	if err != nil {
		t.Fatalf("remove background: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	_, _, _, whiteAlpha := img.At(0, 0).RGBA()
	redR, _, _, redAlpha := img.At(1, 1).RGBA()
	if whiteAlpha != 0 {
		t.Fatalf("white background alpha = %d, want 0", whiteAlpha)
	}
	if redAlpha == 0 || redR == 0 {
		t.Fatalf("red foreground should stay visible, rgba=(%d,%d)", redR, redAlpha)
	}
}

func TestSalesOrderSealPositionAPIOnlyUpdatesCoordinates(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	settingsBody := strings.NewReader(`{"note":"第一行\n第二行","payment_text":"微信付款","seal_x_mm":32,"seal_y_mm":22,"seal_width_mm":42}`)
	settingsReq := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order", settingsBody)
	settingsReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	settingsRec := httptest.NewRecorder()
	e.ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("settings status=%d body=%s", settingsRec.Code, settingsRec.Body.String())
	}

	body := strings.NewReader(`{"seal_x_mm":58,"seal_y_mm":19,"seal_width_mm":46}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order/seal-position", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seal position status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"note":"第一行\n第二行"`,
		`"payment_text":"微信付款"`,
		`"seal_x_mm":58`,
		`"seal_y_mm":19`,
		`"seal_width_mm":46`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("seal position response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestSalesOrderImageGenerationUsesSavedSealPosition(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderAPITestOrder(t, ctx, pool, schema)
	assetDir := t.TempDir()
	objectKey := "sales_order_assets/seal/current-seal.png"
	writeSealWithWhiteBackground(t, filepath.Join(assetDir, objectKey))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sales_order_assets(id, kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES(1101, 'seal', 'current-seal.png', 'image/png', 12, 'seed', '%s', '测试员')`, schema, objectKey))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sales_order_settings(id, note, payment_text, seal_asset_id, seal_x_mm, seal_y_mm, seal_width_mm)
		VALUES(1, '说明', '微信', 1101, 32, 22, 42)`, schema))
	e := newSalesOrderAPITestEcho(pool, schema, assetDir)

	body := strings.NewReader(`{"seal_x_mm":61,"seal_y_mm":18,"seal_width_mm":48}`)
	positionReq := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order/seal-position", body)
	positionReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	positionRec := httptest.NewRecorder()
	e.ServeHTTP(positionRec, positionReq)
	if positionRec.Code != http.StatusOK {
		t.Fatalf("seal position status=%d body=%s", positionRec.Code, positionRec.Body.String())
	}

	imageReq := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-order-images", nil)
	imageRec := httptest.NewRecorder()
	e.ServeHTTP(imageRec, imageReq)
	if imageRec.Code != http.StatusOK {
		t.Fatalf("image status=%d body=%s", imageRec.Code, imageRec.Body.String())
	}
	var doc salesapp.SalesOrderImageDocument
	if err := json.Unmarshal(imageRec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode image response: %v", err)
	}
	if doc.Snapshot.Seal == nil {
		t.Fatalf("generated image snapshot missing seal: %+v", doc.Snapshot)
	}
	if doc.Snapshot.Seal.XMM != 61 || doc.Snapshot.Seal.YMM != 18 || doc.Snapshot.Seal.WidthMM != 48 {
		t.Fatalf("generated image snapshot seal = %+v, want x=61 y=18 width=48", doc.Snapshot.Seal)
	}
}

func TestSalesOrderSealBackgroundRemovalCreatesTransparentPNG(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	assetDir := t.TempDir()
	objectKey := "sales_order_assets/seal/original-seal.png"
	assetPath := filepath.Join(assetDir, objectKey)
	writeSealWithWhiteBackground(t, assetPath)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sales_order_assets(id, kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES(1001, 'seal', 'original-seal.png', 'image/png', 12, 'seed', '%s', '测试员')`, schema, objectKey))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sales_order_settings(id, note, payment_text, seal_asset_id, seal_x_mm, seal_y_mm, seal_width_mm)
		VALUES(1, '说明', '微信', 1001, 32, 22, 42)`, schema))
	e := newSalesOrderAPITestEcho(pool, schema, assetDir)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order/seal/remove-background", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove background status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"content_type":"image/png"`) || !strings.Contains(rec.Body.String(), "-transparent.png") {
		t.Fatalf("remove background response should return transparent PNG asset: %s", rec.Body.String())
	}

	var payload struct {
		Asset salesapp.SalesOrderAsset `json:"asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode remove background response: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(assetDir, payload.Asset.ObjectKey))
	if err != nil {
		t.Fatalf("read transparent seal: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode transparent seal: %v", err)
	}
	_, _, _, whiteAlpha := img.At(0, 0).RGBA()
	redR, _, _, redAlpha := img.At(1, 1).RGBA()
	if whiteAlpha != 0 {
		t.Fatalf("white background alpha = %d, want 0", whiteAlpha)
	}
	if redAlpha == 0 || redR == 0 {
		t.Fatalf("seal foreground should remain opaque red, rgba=(%d,%d)", redR, redAlpha)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings/sales-order", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), payload.Asset.ObjectKey) {
		t.Fatalf("settings should point to transparent seal status=%d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestSalesOrderSettingsServesSalesOrderAssets(t *testing.T) {
	assetDir := t.TempDir()
	assetPath := filepath.Join(assetDir, "sales_order_assets", "payment_code", "qr.pic")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, 0o644); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	registerSalesOrderSettingsRoutes(e, salesapp.NewService(nil), assetDir)

	req := httptest.NewRequest(http.MethodGet, "/assets/sales_order_assets/payment_code/qr.pic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get(echo.HeaderContentType), "image/jpeg") {
		t.Fatalf("asset route status=%d content-type=%q body=%q", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.String())
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

	imageReq := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-order-images", nil)
	imageRec := httptest.NewRecorder()
	e.ServeHTTP(imageRec, imageReq)
	if imageRec.Code != http.StatusOK {
		t.Fatalf("image POST status=%d body=%s", imageRec.Code, imageRec.Body.String())
	}
	if !strings.Contains(imageRec.Body.String(), `"version_no":1`) || !strings.Contains(imageRec.Body.String(), `"download_url":"`) {
		t.Fatalf("image POST response = %s", imageRec.Body.String())
	}
	var createdImage salesapp.SalesOrderImageDocument
	if err := json.Unmarshal(imageRec.Body.Bytes(), &createdImage); err != nil {
		t.Fatalf("decode generated image document: %v", err)
	}

	imageListReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-order-images", nil)
	imageListRec := httptest.NewRecorder()
	e.ServeHTTP(imageListRec, imageListReq)
	if imageListRec.Code != http.StatusOK || !strings.Contains(imageListRec.Body.String(), `"rows":[`) {
		t.Fatalf("image list status=%d body=%s", imageListRec.Code, imageListRec.Body.String())
	}
	imageLatestReq := httptest.NewRequest(http.MethodGet, "/orders/1/sales-order-image-latest.png", nil)
	imageLatestRec := httptest.NewRecorder()
	e.ServeHTTP(imageLatestRec, imageLatestReq)
	if imageLatestRec.Code != http.StatusOK || imageLatestRec.Header().Get(echo.HeaderContentType) != "image/png" {
		t.Fatalf("latest image download status=%d content-type=%q body=%s", imageLatestRec.Code, imageLatestRec.Header().Get(echo.HeaderContentType), imageLatestRec.Body.String())
	}
	imageHistoryReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/orders/1/sales-order-images/%d.png", createdImage.ID), nil)
	imageHistoryRec := httptest.NewRecorder()
	e.ServeHTTP(imageHistoryRec, imageHistoryReq)
	if imageHistoryRec.Code != http.StatusOK || imageHistoryRec.Header().Get(echo.HeaderContentType) != "image/png" {
		t.Fatalf("history image download status=%d content-type=%q body=%s", imageHistoryRec.Code, imageHistoryRec.Header().Get(echo.HeaderContentType), imageHistoryRec.Body.String())
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
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address, taxpayer_id, bank_account_name, bank_name, bank_account_no) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县', '91530827MACGJ29D6J', '孟连口加农业科技有限公司', '中国农业银行孟连支行', '6222000000000000')`, schema))
	e := newSalesOrderAPITestEcho(pool, schema, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-order-preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"company_name":"棵凡咖啡"`,
		`"company_address":"云南省普洱市孟连县"`,
		`"taxpayer_id":"91530827MACGJ29D6J"`,
		`"bank_account_name":"孟连口加农业科技有限公司"`,
		`"bank_name":"中国农业银行孟连支行"`,
		`"bank_account_no":"6222000000000000"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("preview response should use global company account profile, missing %s: %s", want, rec.Body.String())
		}
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
	registerSalesOrderSettingsRoutes(e, svc, assetDir)
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

func writeSealWithWhiteBackground(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	img.Set(1, 1, color.RGBA{R: 190, G: 18, B: 30, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
