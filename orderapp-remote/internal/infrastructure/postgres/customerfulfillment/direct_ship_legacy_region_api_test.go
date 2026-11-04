package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	freightdomain "orderapp/internal/domain/freight"
	postgresportal "orderapp/internal/infrastructure/postgres/customerportal"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
)

func TestEmployeeDirectShipLegacyCityAddressPreviewSubmitReplay(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	if err := postgresportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_recipient_addresses(id,customer_id,recipient_name,phone,province,city,district,detail_address) VALUES(99,501,'测试','13800000000','云南省','大理市','大理古城','测试路1号')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES(71411,'REGION-SOURCE',501,'customer_stock','DEV-E2E-A1'); INSERT INTO %[1]s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,unit_price,line_total) VALUES(71411,1,910,7031,7032,'客户已购','454g袋',12,'袋',30,360); INSERT INTO %[1]s.logistics_companies(id,name,active) VALUES(705,'区域测试',true); INSERT INTO %[1]s.logistics_products(id,company_id,name,active) VALUES(705,705,'区域测试',true)`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	templates, err := repo.ListShippingFeeTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tm := templates[0]
	tm.CarrierID, tm.ProductID = 705, 705
	tm.Rules = nil
	tm.Groups = []freightdomain.Group{{Name: "云南", RegionCodes: []string{"530000"}, Tiers: []freightdomain.Tier{{UpperWeightG: 3000, FeeCents: 800}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}, {Name: "大理市", RegionCodes: []string{"532901"}, Tiers: []freightdomain.Tier{{UpperWeightG: 3000, FeeCents: 2300}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}
	if _, err = repo.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Template: tm, ExpectedRevision: tm.Revision, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("operator_employee", "销售员")
			return next(c)
		}
	})
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: svc})
	cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", FulfillmentMode: "stock", IdempotencyKey: "REGION-TEST-1", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", District: "大理古城", DetailAddress: "测试路1号", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	call := func(path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status=%d: %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var preview app.MiniDirectShipPreview
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/preview", cmd, 200), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Fees == nil || preview.Fees.Freight == nil || preview.Fees.Freight.Estimate.RegionCode != "532901" || preview.Fees.Freight.EstimatedTotalCents != 2300 {
		t.Fatalf("preview=%+v", preview.Fees)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	var created, replay app.MiniDirectShipRequest
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/requests", cmd, 201), &created); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/requests", cmd, 201), &replay); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.ID != replay.ID {
		t.Fatalf("retry created=%+v replay=%+v", created, replay)
	}
	var address string
	var raw []byte
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.receiver_address,s.quote_json FROM %[1]s.orders o JOIN %[1]s.fulfillment_order_fee_snapshots s ON s.order_id=o.id WHERE o.id=$1`, schema), created.OrderID).Scan(&address, &raw); err != nil {
		t.Fatal(err)
	}
	var q app.FulfillmentFeeQuote
	if err = json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if address != "云南省大理市大理古城测试路1号" || q.FeeQuoteToken != preview.FeeQuoteToken || q.City != "大理白族自治州" || q.District != "大理市" {
		t.Fatalf("address=%q quote=%+v", address, q)
	}
	var city, district, detail string
	var revision int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT city,district,detail_address,revision FROM %s.customer_recipient_addresses WHERE id=99`, schema)).Scan(&city, &district, &detail, &revision); err != nil {
		t.Fatal(err)
	}
	if city != "大理市" || district != "大理古城" || detail != "测试路1号" || revision != 1 {
		t.Fatalf("address book changed: %s %s %s rev %d", city, district, detail, revision)
	}
}

func TestLegacyDestinationKeepsSuccessfulLegacyAndFreeQuotes(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	r := NewRepository(pool, schema)
	cmd := app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "stock", Province: "云南省", City: "大理市", District: "大理古城", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	legacy, err := r.QuoteFulfillmentFees(ctx, cmd)
	if err != nil || legacy.Freight == nil || legacy.City != cmd.City || legacy.District != cmd.District || legacy.Freight.Destination.City != cmd.City || legacy.Freight.EstimatedTotalCents != 1200 {
		t.Fatalf("legacy quote=%+v error=%v", legacy, err)
	}
	fr := freightpg.NewRepository(pool, schema)
	policy, err := fr.LoadPolicy(ctx, pool, 501, "stock", false)
	if err != nil {
		t.Fatal(err)
	}
	policy.FreeShipping = true
	if _, err = fr.SavePolicy(ctx, policy, policy.Revision, "test"); err != nil {
		t.Fatal(err)
	}
	free, err := r.QuoteFulfillmentFees(ctx, cmd)
	if err != nil || free.Freight == nil || free.Freight.Status != "free" || free.Freight.Destination.City != cmd.City || free.Freight.Destination.District != cmd.District {
		t.Fatalf("free quote=%+v error=%v", free, err)
	}
}

func TestEmployeeOnDemandLegacyCityAddressPreviewSubmitReplay(t *testing.T) {
	f := newOnDemandFulfillmentEditFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.logistics_companies(id,name,active) VALUES(705,'区域测试',true); INSERT INTO %[1]s.logistics_products(id,company_id,name,active) VALUES(705,705,'区域测试',true)`, f.schema)); err != nil {
		t.Fatal(err)
	}
	templates, err := f.repo.ListShippingFeeTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tm := templates[0]
	tm.CarrierID, tm.ProductID = 705, 705
	tm.Rules = nil
	tm.Groups = []freightdomain.Group{{Name: "云南", RegionCodes: []string{"530000"}, Tiers: []freightdomain.Tier{{UpperWeightG: 3000, FeeCents: 800}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}, {Name: "大理市", RegionCodes: []string{"532901"}, Tiers: []freightdomain.Tier{{UpperWeightG: 3000, FeeCents: 2300}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}
	if _, err = f.repo.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Template: tm, ExpectedRevision: tm.Revision, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("operator_employee", "销售员")
			return next(c)
		}
	})
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: f.svc})
	cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", FulfillmentMode: "on_demand", IdempotencyKey: "REGION-ON-DEMAND", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", District: "大理古城", DetailAddress: "测试路1号", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	call := func(path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status=%d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var p app.MiniDirectShipPreview
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/preview", cmd, 200), &p); err != nil {
		t.Fatal(err)
	}
	if p.Fees == nil || p.Fees.Freight == nil || p.Fees.Freight.Estimate.RegionCode != "532901" || p.Fees.Freight.EstimatedTotalCents != 2300 {
		t.Fatalf("preview=%+v", p.Fees)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = p.PriceQuoteToken, p.FeeQuoteToken
	var created, replay app.MiniDirectShipRequest
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/requests", cmd, 201), &created); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/requests", cmd, 201), &replay); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.ID != replay.ID {
		t.Fatalf("created=%+v replay=%+v", created, replay)
	}
	var address string
	var raw []byte
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.receiver_address,s.quote_json FROM %[1]s.orders o JOIN %[1]s.fulfillment_order_fee_snapshots s ON s.order_id=o.id WHERE o.id=$1`, f.schema), created.OrderID).Scan(&address, &raw); err != nil {
		t.Fatal(err)
	}
	var q app.FulfillmentFeeQuote
	if err = json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if address != "云南省大理市大理古城测试路1号" || q.FeeQuoteToken != p.FeeQuoteToken || q.City != "大理白族自治州" || q.District != "大理市" {
		t.Fatalf("address=%q quote=%+v", address, q)
	}
}
