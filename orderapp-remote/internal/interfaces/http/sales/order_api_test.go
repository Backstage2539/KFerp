package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

func TestOrderEntryRedirectsToVueShell(t *testing.T) {
	e := echo.New()
	registerOrderRoutes(e, nil)

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

func TestOrderAPIFormReturnsCustomerDefaultsForOrderEntry(t *testing.T) {
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
	for _, needle := range []string{`"default_source_id":1`, `"default_order_type_id":2`, `"py"`, `"pyi"`} {
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

func TestOrderAPIDefaultsNewOrderToPaidAndUnshipped(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-27",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  0,
		"ship_status_id": 0,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"1"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payStatusID, shipStatusID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(pay_status_id,0), COALESCE(ship_status_id,0)
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&payStatusID, &shipStatusID); err != nil {
		t.Fatalf("query order statuses: %v", err)
	}
	if payStatusID != 2 || shipStatusID != 1 {
		t.Fatalf("saved statuses pay=%d ship=%d, want pay=2 ship=1", payStatusID, shipStatusID)
	}
}

func TestOrderAPISaveDoesNotGenerateShippingExcel(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-27",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OrderNo          string `json:"order_no"`
		ShippingExcelURL string `json:"shipping_excel_url"`
		Error            string `json:"shipping_excel_error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || resp.ShippingExcelURL != "" {
		t.Fatalf("order save should not process shipping excel, response = %+v body=%s", resp, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err == nil && len(files) > 0 {
		t.Fatalf("order save generated shipping exports = %d, want 0", len(files))
	}
}

func TestOrdersShippingExcelAPIGeneratesFromSelectedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
		INSERT INTO %s.order_process_statuses(id,name,sort,active) VALUES (2,'生产完成',20,true);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (20, 'SO-SHIP-LIST', '2026-04-27', 3, 1, 2, 1, 2, 176, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (20, 1, 7, '橘皮乌龙', 2, '件', '454g', 88, 176);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{20}})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ShippingExcelURL string `json:"shipping_excel_url"`
		Error            string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || resp.ShippingExcelURL == "" {
		t.Fatalf("shipping excel response = %+v body=%s", resp, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("A2") != "测试收件人" || cell("B2") != "13800000000" || cell("D2") != "寄件人" || cell("H2") != "1" || cell("I2") != "茶叶" || cell("J2") != "0.1" {
		t.Fatalf("shipping excel cells A2=%q B2=%q D2=%q H2=%q I2=%q J2=%q", cell("A2"), cell("B2"), cell("D2"), cell("H2"), cell("I2"), cell("J2"))
	}
	if !strings.Contains(cell("N2"), "SO-SHIP-LIST") || !strings.Contains(cell("N2"), "橘皮乌龙 454g x2件") {
		t.Fatalf("shipping excel remark N2=%q", cell("N2"))
	}
	if strings.Contains(cell("N2"), "单价") || strings.Contains(cell("N2"), "小计") || strings.Contains(cell("N2"), "88") || strings.Contains(cell("N2"), "176") {
		t.Fatalf("shipping excel remark should not include price or subtotal N2=%q", cell("N2"))
	}
}

func TestOrdersShippingExcelAPIUsesSelectedSenderProfile(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES
			(2, '默认仓库', '默认寄件人', '13900000000', '默认地址', '默认公司', '茶叶', '标快', true, true),
			(3, '门店', '门店寄件人', '13900000003', '门店地址', '门店公司', '茶叶', '特快', false, true);
		INSERT INTO %s.order_process_statuses(id,name,sort,active) VALUES (2,'生产完成',20,true);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (22, 'SO-SENDER-SELECTED', '2026-04-27', 3, 1, 2, 1, 2, 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (22, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{22}, "sender_id": int64(3)})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("D2") != "门店寄件人" || cell("E2") != "13900000003" || cell("F2") != "门店地址" || cell("O2") != "门店公司" || cell("P2") != "特快" {
		t.Fatalf("selected sender cells D2=%q E2=%q F2=%q O2=%q P2=%q", cell("D2"), cell("E2"), cell("F2"), cell("O2"), cell("P2"))
	}
}

func TestOrdersShippingExcelAPIUsesPerOrderSenderOverrides(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES
			(2, '默认仓库', '默认寄件人', '13900000000', '默认地址', '默认公司', '茶叶', '标快', true, true),
			(3, '门店', '门店寄件人', '13900000003', '门店地址', '门店公司', '茶叶', '特快', false, true);
		INSERT INTO %s.order_process_statuses(id,name,sort,active) VALUES (2,'生产完成',20,true);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(23, 'SO-SENDER-DEFAULT', '2026-04-27', 3, 1, 2, 1, 2, 88, false),
			(24, 'SO-SENDER-OVERRIDE', '2026-04-27', 3, 1, 2, 1, 2, 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES
			(23, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88),
			(24, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{
		"order_ids": []int64{23, 24},
		"sender_id": int64(2),
		"order_senders": []map[string]any{
			{"order_id": int64(24), "sender_id": int64(3)},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("D2") != "默认寄件人" || cell("D3") != "门店寄件人" || cell("P2") != "标快" || cell("P3") != "特快" {
		t.Fatalf("sender rows D2=%q D3=%q P2=%q P3=%q", cell("D2"), cell("D3"), cell("P2"), cell("P3"))
	}
}

func TestOrdersShippingExcelAPIRejectsUnfinishedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (21, 'SO-NOT-FINISHED', '2026-04-27', 3, 1, 2, 1, 1, 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (21, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{21}})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "尚未生产完成") {
		t.Fatalf("unfinished order error body=%s", rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err == nil && len(files) > 0 {
		t.Fatalf("unfinished order generated shipping exports = %d, want 0", len(files))
	}
}

func TestSenderSettingsAPIListsProfilesWithDefault(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES (2, '仓库', '仓库寄件人', '13900000002', '仓库地址', '仓库公司', '茶叶', '标快', true, true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/sender", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/sender status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"profiles"`, `"sender_label":"仓库"`, `"is_default":true`, `"profile"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("sender settings response missing %s: %s", needle, body)
		}
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
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc)
	registerOrderShippingExcelRoutes(e, svc)
	registerSenderSettingsPage(e, svc)
	return e
}

func seedOrderAPITestData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id,name,contact,phone,address,active,default_source_id,default_order_type_id) VALUES (3,'测试客户','测试收件人','13800000000','杭州市测试路',true,1,2);
		INSERT INTO %s.sources(id,name) VALUES (1,'小程序');
		INSERT INTO %s.order_types(id,name) VALUES (1,'批发订单'),(2,'零售订单');
		INSERT INTO %s.pay_statuses(id,name) VALUES (1,'未付款'),(2,'已付款');
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
	contact TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	default_source_id BIGINT,
	default_order_type_id BIGINT
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
CREATE TABLE %s.sender_settings (
	id SMALLINT PRIMARY KEY DEFAULT 1,
	sender_label TEXT NOT NULL DEFAULT '',
	sender_name TEXT NOT NULL DEFAULT '',
	sender_phone TEXT NOT NULL DEFAULT '',
	sender_addr TEXT NOT NULL DEFAULT '',
	sender_company TEXT NOT NULL DEFAULT '',
	sender_goods TEXT NOT NULL DEFAULT '茶叶',
	sf_biz_type TEXT NOT NULL DEFAULT '',
	is_default BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.sender_settings(id, sender_label, is_default, active) VALUES(1, '默认寄件人', true, true);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
}

func writeOrderShippingTemplateForTest(t *testing.T, path string) {
	t.Helper()
	wb := excelize.NewFile()
	sheet := wb.GetSheetName(0)
	headers := []string{"收件人", "收件人手机/电话", "收件地址", "寄件人", "寄件人手机/电话", "寄件地址", "收件公司", "包裹件数", "托寄物", "重量", "长", "宽", "高", "备注(选填)", "寄件公司", "业务类型", "包装服务费"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := wb.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
}
