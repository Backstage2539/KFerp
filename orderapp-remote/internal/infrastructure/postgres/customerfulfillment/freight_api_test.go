package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
	freighthttp "orderapp/internal/interfaces/http/freight"
	"strings"
	"testing"
)

func TestFreightThreeSceneAPIAndPolicyIsolation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	if _, e := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.logistics_companies(id,name,active)VALUES(705,'PR705独立物流',true);INSERT INTO %s.logistics_products(id,company_id,name,active)VALUES(705,705,'PR705物流产品',true)`, schema, schema)); e != nil {
		t.Fatal(e)
	}
	r := NewRepository(pool, schema)
	svc := app.NewService(r)
	e := echo.New()
	freighthttp.RegisterRoutes(e, pool, schema, svc)
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: svc})
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s status%d want%d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	template := domain.Template{Name: "PR705独立10元", Active: true, CarrierID: 705, ProductID: 705, Groups: []domain.Group{{Name: "云南", RegionCodes: []string{"530000"}, Tiers: []domain.Tier{{UpperWeightG: 2000, FeeCents: 1000}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}
	var saved domain.Template
	json.Unmarshal(call(http.MethodPost, "/api/settings/logistics/shipping-templates", template, 200), &saved)
	for _, scene := range []string{"sale", "stock", "on_demand"} {
		call(http.MethodPut, "/api/customer-fulfillment/501/shipping-policies/"+scene, map[string]any{"template_id": saved.ID, "surcharge_cents": 1000, "expected_revision": 0}, 200)
	}
	// Other customer has no configuration and cannot inherit a policy.
	call(http.MethodPost, "/api/customer-fulfillment/502/freight/preview", map[string]any{"province": "云南省", "items": []map[string]any{{"product_id": 911, "qty": 1}}}, 400)
	call(http.MethodPut, "/api/customer-fulfillment/501/settings", map[string]any{"expected_revision": 0, "modes": []map[string]any{{"mode": "stock", "enabled": true, "warehouse_code": "DEV-E2E-A1", "fee_configured": true, "fixed_fee_cents": 300, "shipping_template_id": saved.ID}, {"mode": "on_demand", "enabled": true, "fee_configured": true, "fixed_fee_cents": 300, "shipping_template_id": saved.ID}}}, 200)
	var sale domain.Snapshot
	json.Unmarshal(call(http.MethodPost, "/api/customer-fulfillment/501/freight/preview", map[string]any{"fulfillment_mode": "stock", "province": "云南", "items": []map[string]any{{"product_id": 911, "qty": 1}}}, 200), &sale)
	if sale.Policy.Scene != "sale" || sale.EstimatedTotalCents != 2000 || sale.CurrentReceivableCents != 0 {
		t.Fatal(sale)
	}
	for _, scene := range []string{"stock", "on_demand"} {
		var q app.FulfillmentFeeQuote
		json.Unmarshal(call(http.MethodPost, "/api/customer-fulfillment/501/fees/preview", map[string]any{"fulfillment_mode": scene, "province": "云南", "items": []map[string]any{{"product_id": 911, "qty": 1}}}, 200), &q)
		if q.ServiceFeeCents != 300 || q.ShippingFeeCents != 0 || q.Freight == nil || q.Freight.EstimatedTotalCents != 2000 {
			t.Fatal(q)
		}
	}
	// No client may bypass order protocol confirmation or forge surcharge/free shipping.
	_, err := svc.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{CustomerID: 501, FulfillmentMode: "stock", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南", DetailAddress: "独立验收路", Items: []app.MiniDirectShipItemCommand{{ProductID: 911, SpecG: 1000, Qty: 1}}, IdempotencyKey: "freight-old-client"})
	if err == nil || !strings.Contains(err.Error(), "更新客户端") {
		t.Fatal("old client", err)
	}
	zero := int64(0)
	_, err = r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "stock", Province: "云南", Items: []app.FulfillmentFeeItem{{ProductID: 911, Qty: 1}}, FreightOverride: &domain.Override{SurchargeCents: &zero, Reason: "customer forged"}})
	if err == nil {
		t.Fatal("customer override accepted")
	}
	policies, err := freightpg.NewRepository(pool, schema).Policies(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range policies {
		if p.TemplateID != saved.ID || p.SurchargeCents != 1000 {
			t.Fatal(p)
		}
	}
	var regions struct {
		Version string          `json:"version"`
		Regions []domain.Region `json:"regions"`
	}
	json.Unmarshal(call(http.MethodGet, "/api/settings/logistics/regions?q=yns", nil, 200), &regions)
	if regions.Version != domain.RegionVersion || len(regions.Regions) == 0 {
		t.Fatal(regions)
	}
	var orderID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,total_amount,grand_total,ship_tracking_no,logistics_company_id,logistics_product_id)VALUES('PR705-API-ORDINARY',501,60,60,'000SF705API',705,705)RETURNING id`, schema)).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	fr := freightpg.NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = fr.SaveSnapshotTx(ctx, tx, orderID, sale, "test"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	book := excelize.NewFile()
	defer book.Close()
	book.SetSheetRow("Sheet1", "A1", &[]any{"运单号", "最终实际运费"})
	book.SetSheetRow("Sheet1", "A2", &[]any{"000SF705API", "12.00"})
	xlsx, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	upload := func(mapped bool) []byte {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		file, _ := form.CreateFormFile("file", "actual.xlsx")
		file.Write(xlsx.Bytes())
		form.WriteField("carrier_id", "705")
		if mapped {
			form.WriteField("tracking_column", "1")
			form.WriteField("amount_column", "2")
		}
		form.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/settings/logistics/freight-imports/preview", &body)
		req.Header.Set(echo.HeaderContentType, form.FormDataContentType())
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var columns struct {
		Headers []string `json:"headers"`
	}
	json.Unmarshal(upload(false), &columns)
	if len(columns.Headers) != 2 {
		t.Fatal(columns)
	}
	var batch freightpg.ImportBatch
	json.Unmarshal(upload(true), &batch)
	if len(batch.Rows) != 1 || batch.Rows[0].DeltaCents != 2200 || batch.Rows[0].TrackingNo != "000SF705API" {
		t.Fatal(batch)
	}
	call(http.MethodPost, fmt.Sprintf("/api/settings/logistics/freight-imports/%d/confirm", batch.ID), map[string]any{}, 200)
	call(http.MethodPost, fmt.Sprintf("/api/settings/logistics/freight-imports/%d/confirm", batch.ID), map[string]any{}, 200)
	account, err := r.CustomerAccount(ctx, app.AccountQuery{CustomerID: 501})
	if err != nil || account.Summary.FeeShippingCents != 2200 {
		t.Fatalf("HTTP actual billing %+v %v", account.Summary, err)
	}

}
