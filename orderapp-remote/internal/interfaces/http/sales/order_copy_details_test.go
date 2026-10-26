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
	repo := &orderCopyRepo{snapshot: salesapp.OrderCopySnapshot{CustomerName: "棵凡咖啡", PaymentMethod: "银行转账", PayStatus: "未付款", Order: salesapp.OrderEditData{ID: 1, CustomerID: 1, OrderNo: "SO-1", Items: []salesapp.OrderEditItem{{Product: "豆", Spec: "454g", Qty: "2", Unit: "袋"}}}}}
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
			text, ok := body["text"].(string)
			if len(body) != 2 || body["order_id"] != float64(1) || !ok || !strings.Contains(text, "付款状态："+status) || !strings.Contains(text, "客户名称：棵凡咖啡") || !strings.Contains(text, "付款方式：银行转账") {
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
		 INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,document_date,pay_status_id,payment_method,receiver_name,receiver_phone,receiver_address) VALUES(9001,'SO-COPY',3,'2026-10-18','2026-10-17',1,'银行转账','','','');
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
	for _, want := range []string{"客户名称：测试客户", "付款方式：银行转账", "客户名称｜454g｜1.25 袋", "2. 挂耳｜10袋/盒｜3 盒", "收件地址：当前地址", "收件人：当前联系人", "电话：9999"} {
		if !strings.Contains(body.Text, want) {
			t.Fatalf("missing %s: %s", want, body.Text)
		}
	}
	for _, bad := range []string{"旧名", "2026-10-17", "规格：", "数量："} {
		if strings.Contains(body.Text, bad) {
			t.Fatal(body.Text)
		}
	}
}

func TestOrderCopyDetailsPostgresRecipientSources(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.customers SET contact='客户联系人',phone='13800000000',address='客户默认地址' WHERE id=3`, schema))
	e := newOrderAPITestEcho(pool, schema)
	for index, tc := range []struct {
		name, service, recipient, phone, address string
		wantName, wantPhone, wantAddress         string
	}{
		{"legacy", "", "", "", "", "客户联系人", "13800000000", "客户默认地址"},
		{"saved", "", "指定收件人", "13900000000", "指定地址", "指定收件人", "13900000000", "指定地址"},
		{"partial legacy", "", "指定收件人", "", "", "指定收件人", "13800000000", "客户默认地址"},
		{"direct ship missing", "direct_ship", "", "", "", "未填写", "未填写", "未填写"},
		{"product order missing", "product_order", "", "", "", "未填写", "未填写", "未填写"},
		{"processing ship missing", "processing_ship", "", "", "", "未填写", "未填写", "未填写"},
		{"direct ship saved", "direct_ship", "指定收件人", "13900000000", "指定地址", "指定收件人", "13900000000", "指定地址"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := int64(9100 + index)
			_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(id,order_no,customer_id,order_date,pay_status_id,portal_service_code,receiver_name,receiver_phone,receiver_address) VALUES($1,$2,3,'2026-10-19',1,$3,$4,$5,$6)`, schema), id, fmt.Sprintf("SO-COPY-%d", id), tc.service, tc.recipient, tc.phone, tc.address)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", fmt.Sprintf("/api/orders/%d/copy-details", id), nil))
			if rec.Code != 200 {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"收件人：" + tc.wantName, "电话：" + tc.wantPhone, "收件地址：" + tc.wantAddress} {
				if !strings.Contains(body.Text, want) {
					t.Fatalf("missing %s: %s", want, body.Text)
				}
			}
		})
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
