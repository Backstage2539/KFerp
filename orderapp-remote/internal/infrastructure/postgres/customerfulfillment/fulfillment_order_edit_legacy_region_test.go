package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

// Reproduce a previously accepted county-level-city address stored in the
// request while the original billing quote contains the canonical hierarchy.
func TestFulfillmentEditHistoricalRegionDateOnlyHTTP(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, err := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	old := v.Fees
	old.Freight.Destination = domain.Destination{Province: "云南省", City: "大理白族自治州", District: "大理市"}
	old.Freight.Token = domain.Token(*old.Freight)
	old.FeeQuoteToken = app.FulfillmentFeeQuoteToken(old)
	feeJSON, _ := json.Marshal(old)
	freightJSON, _ := json.Marshal(old.Freight)
	var templateJSON []byte
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.shipping_fee_template_versions WHERE template_id=$1 AND revision=$2`, f.schema), old.Freight.TemplateID, old.Freight.TemplateRevision).Scan(&templateJSON); err != nil {
		t.Fatal(err)
	}
	var tm domain.Template
	if err = json.Unmarshal(templateJSON, &tm); err != nil {
		t.Fatal(err)
	}
	tm.Rules = nil
	tm.CarrierID, tm.ProductID = 1, 1
	tm.Groups = []domain.Group{{Name: "测试云南", RegionCodes: []string{"530000"}, Tiers: []domain.Tier{{UpperWeightG: 1000, FeeCents: 800}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200}, {Name: "测试北京", RegionCodes: []string{"110000"}, Tiers: []domain.Tier{{UpperWeightG: 1000, FeeCents: 1200}}, AdditionalWeightG: 1000, AdditionalFeeCents: 300}}
	templateJSON, _ = json.Marshal(tm)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE %s.customer_direct_ship_requests SET province='云南省',city='大理市',district='大理古城' WHERE id=$1`, []any{v.RequestID}},
		{`UPDATE %s.orders SET receiver_address='云南省大理市大理古城测试路1号' WHERE id=$1`, []any{v.OrderID}},
		{`UPDATE %s.order_freight_snapshots SET snapshot_json=$2 WHERE order_id=$1`, []any{v.OrderID, freightJSON}},
		{`UPDATE %s.fulfillment_order_fee_snapshots SET quote_json=$2 WHERE order_id=$1`, []any{v.OrderID, feeJSON}},
		{`UPDATE %s.shipping_fee_template_versions SET snapshot_json=$1 WHERE template_id=$2 AND revision=$3`, []any{templateJSON, tm.ID, tm.Revision}},
	} {
		if _, err = f.pool.Exec(ctx, fmt.Sprintf(q.sql, f.schema), q.args...); err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("actor", "employee:703")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(postgressales.NewRepository(f.pool, f.schema)), Authz: fulfillmentEditTestAuthz{}, FulfillmentEditor: f.svc})
	path := fmt.Sprintf("/api/orders/%d/fulfillment", v.OrderID)
	send := func(method, url string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, url, strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w
	}
	w := send("GET", path, nil)
	if w.Code != 200 {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	c := editCommand(v)
	c.OrderDate = "2026-09-15"
	w = send("POST", path+"/preview", c)
	if w.Code != 200 {
		t.Fatalf("date-only preview: %d %s", w.Code, w.Body.String())
	}
	var p app.FulfillmentOrderEditPreview
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Fees.Freight.Token != old.Freight.Token || p.Fees.FeeQuoteToken != old.FeeQuoteToken {
		t.Fatal("unchanged order repriced")
	}
	// A quantity change must reprice with the trusted canonical original area.
	changedQty := c
	changedQty.Items = append([]app.FulfillmentOrderEditItemCommand(nil), c.Items...)
	changedQty.Items[0].Qty++
	w = send("POST", path+"/preview", changedQty)
	if w.Code != 200 {
		t.Fatalf("quantity preview: %d %s", w.Code, w.Body.String())
	}
	var changed app.FulfillmentOrderEditPreview
	if err = json.Unmarshal(w.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Fees.Freight.Token == old.Freight.Token || changed.Fees.Freight.Destination.District != "大理市" {
		t.Fatal("quantity change did not reprice canonical district")
	}
	// Different locality is not silently treated as the old address.
	changedAddress := c
	changedAddress.ReceiverAddress = "云南省大理市大理下关测试路2号"
	changedAddress.District = "大理下关"
	w = send("POST", path+"/preview", changedAddress)
	if w.Code == 200 {
		t.Fatal("changed ambiguous address bypassed region validation")
	}
	for _, bad := range []struct{ address, province, city, district string }{
		{c.ReceiverAddress, c.Province, c.City, "大理下关"},
		{c.ReceiverAddress + "附加文字", c.Province, c.City, c.District},
		{c.ReceiverAddress, "北京市", c.City, c.District},
		{c.ReceiverAddress, c.Province, "昆明市", "五华区"},
	} {
		attempt := c
		attempt.ReceiverAddress, attempt.Province, attempt.City, attempt.District = bad.address, bad.province, bad.city, bad.district
		if result := send("POST", path+"/preview", attempt); result.Code == 200 {
			t.Fatalf("changed address or region bypassed validation: %+v", bad)
		}
	}
	changedAddress.ReceiverAddress = "北京市朝阳区测试路2号"
	changedAddress.Province, changedAddress.City, changedAddress.District = "北京市", "北京市", "朝阳区"
	w = send("POST", path+"/preview", changedAddress)
	if w.Code != 200 {
		t.Fatalf("valid new destination: %d %s", w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Fees.Freight.Token == old.Freight.Token || changed.Fees.Freight.Destination.District != "朝阳区" {
		t.Fatal("new destination not repriced")
	}
	c.PreviewToken = p.PreviewToken
	w = send("PATCH", path, c)
	if w.Code != 200 {
		t.Fatalf("date-only save: %d %s", w.Code, w.Body.String())
	}
	var date, province, city, district string
	var savedFreight, savedFee []byte
	var auditCount int
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_char(o.order_date,'YYYY-MM-DD'),d.province,d.city,d.district,fs.snapshot_json,qs.quote_json FROM %[1]s.orders o JOIN %[1]s.customer_direct_ship_requests d ON d.id=$2 JOIN %[1]s.order_freight_snapshots fs ON fs.order_id=o.id JOIN %[1]s.fulfillment_order_fee_snapshots qs ON qs.order_id=o.id WHERE o.id=$1`, f.schema), v.OrderID, v.RequestID).Scan(&date, &province, &city, &district, &savedFreight, &savedFee); err != nil {
		t.Fatal(err)
	}
	if date != c.OrderDate || province != v.Province || city != v.City || district != v.District {
		t.Fatal("date or original address changed unexpectedly")
	}
	var sf domain.Snapshot
	var sq app.FulfillmentFeeQuote
	if err = json.Unmarshal(savedFreight, &sf); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(savedFee, &sq); err != nil {
		t.Fatal(err)
	}
	if sf.Token != old.Freight.Token || sq.FeeQuoteToken != old.FeeQuoteToken {
		t.Fatal("historical fee snapshot changed")
	}
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='fulfillment_edit'`, f.schema), v.OrderID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("missing order audit: %d %v", auditCount, err)
	}
	// Even a well-formed but different frozen destination must never authorize
	// treating the historical request region as the quoted destination.
	sq.Freight.Destination = domain.Destination{Province: "云南省", City: "昆明市", District: "五华区"}
	sq.Freight.Token = domain.Token(*sq.Freight)
	sq.FeeQuoteToken = app.FulfillmentFeeQuoteToken(sq)
	newFee, _ := json.Marshal(sq)
	newFreight, _ := json.Marshal(sq.Freight)
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_order_fee_snapshots SET quote_json=$2 WHERE order_id=$1`, f.schema), v.OrderID, newFee); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_freight_snapshots SET snapshot_json=$2 WHERE order_id=$1`, f.schema), v.OrderID, newFreight); err != nil {
		t.Fatal(err)
	}
	w = send("GET", path, nil)
	if w.Code != 200 {
		t.Fatalf("mismatched snapshot GET: %d %s", w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if w = send("POST", path+"/preview", editCommand(v)); w.Code == 200 {
		t.Fatal("different frozen destination authorized legacy reinterpretation")
	}
}
