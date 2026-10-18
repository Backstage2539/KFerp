package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
	"strings"
	"testing"
)

type orderCopyRepo struct {
	salesapp.Repository
	calls    int
	snapshot salesapp.OrderCopySnapshot
}

func (r *orderCopyRepo) LoadOrderCopySnapshot(context.Context, int64) (*salesapp.OrderCopySnapshot, error) {
	r.calls++
	s := r.snapshot
	return &s, nil
}
func TestOrderCopyDetailsAPIRefreshAndCustomerIsolation(t *testing.T) {
	repo := &orderCopyRepo{snapshot: salesapp.OrderCopySnapshot{PayStatus: "未付款", Order: salesapp.OrderEditData{ID: 1, CustomerID: 1, OrderNo: "SO-1", Items: []salesapp.OrderEditItem{{Product: "豆", Spec: "454g", Qty: "2", Unit: "袋"}}}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil, accountScopeStub{customer: 1})
	for _, path := range []string{"/api/orders/1/copy-details", accountPrefix + "/orders/1/copy-details"} {
		for _, status := range []string{"未付款", "已付款"} {
			repo.snapshot.PayStatus = status
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			if rec.Code != 200 {
				t.Fatalf("%s %d %s", path, rec.Code, rec.Body.String())
			}
			var body map[string]any
			json.Unmarshal(rec.Body.Bytes(), &body)
			if len(body) != 2 || body["order_id"] != float64(1) || !strings.Contains(body["text"].(string), "付款状态："+status) {
				t.Fatal(body)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("copy must not cache")
			}
		}
	}
	before := repo.calls
	for _, path := range []string{accountPrefix + "/orders/999/copy-details", accountPrefix + "/orders/1/copy-details?customer_id=2"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 403 && rec.Code != 404 {
			t.Fatalf("%s %d", path, rec.Code)
		}
	}
	if repo.calls != before {
		t.Fatal("unauthorized snapshot read")
	}
}
func TestOrderCopyDetailsPostgresSavedAddressAndSnapshots(t *testing.T) {
	pool, s := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, s)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,document_date,pay_status_id,receiver_name,receiver_phone,receiver_address) VALUES(9001,'SO-COPY',3,'2026-10-18','2026-10-17',1,'','','');
 INSERT INTO %[1]s.order_items(order_id,line_no,item_name,product_name_snapshot,customer_product_display_name_snapshot,spec,qty,unit,price_source_json) VALUES
 (9001,1,'旧名','快照名称','客户名称','454',1.25,'袋','{}'),(9001,2,'挂耳','','','10袋/盒',3,'盒','{}');
 UPDATE %[1]s.customers SET contact='当前联系人',address='当前地址',phone='9999' WHERE id=3;
 `, s))
	e := newOrderAPITestEcho(pool, s)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/orders/9001/copy-details", nil))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Text string `json:"text"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	for _, want := range []string{"客户名称｜规格：454g｜数量：1.25 袋", "2. 挂耳｜规格：10袋/盒｜数量：3 盒", "收件地址：未填写", "收件人：未填写"} {
		if !strings.Contains(body.Text, want) {
			t.Fatalf("missing %s: %s", want, body.Text)
		}
	}
	for _, bad := range []string{"当前地址", "当前联系人", "9999", "2026-10-17"} {
		if strings.Contains(body.Text, bad) {
			t.Fatal(body.Text)
		}
	}
}

func (r *orderCopyRepo) ListOrders(_ context.Context, q salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	if q.OrderID == 88 && (q.CustomerID == 0 || q.CustomerID == 152) && q.FulfillmentEmployeeID == 7 {
		return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 88, CustomerID: 152}}}, nil
	}
	return salesapp.OrderListResult{}, nil
}
func TestOrderCopyDetailsWorkbenchScope(t *testing.T) {
	for _, id := range []int64{88, 89} {
		repo := &orderCopyRepo{snapshot: salesapp.OrderCopySnapshot{Order: salesapp.OrderEditData{ID: id, CustomerID: 152, OrderNo: "SO-PRIVATE"}}}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
		})
		e.Use(support.AuthorizationMiddleware(&orderAPIAuthzService{actor: authzapp.Actor{Permissions: []string{"customer_processing.read"}}}))
		registerOrderAPI(e, salesapp.NewService(repo), nil)
		r := httptest.NewRecorder()
		e.ServeHTTP(r, httptest.NewRequest("GET", fmt.Sprintf("/api/orders/%d/copy-details", id), nil))
		if id == 88 && r.Code != 200 {
			t.Fatal(r.Code, r.Body)
		}
		if id == 89 && (r.Code != 403 || repo.calls != 0 || strings.Contains(r.Body.String(), "SO-PRIVATE")) {
			t.Fatalf("out of scope response %d %s", r.Code, r.Body)
		}
	}
}
