package sales

import (
	"context"
	"net/http"
	"net/http/httptest"
	fulfillmentapp "orderapp/internal/application/customerfulfillment"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	salesapp "orderapp/internal/application/sales"
)

func TestStaffFulfillmentEditRoutesRequireInternalOrderWrite(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch} {
		for _, tc := range []struct {
			name                  string
			authenticated, scoped bool
			account               string
			permissions           []string
			want                  int
		}{
			{name: "anonymous", want: http.StatusUnauthorized},
			{name: "no_permission", authenticated: true, account: "internal_employee", want: http.StatusForbidden},
			{name: "channel_account", authenticated: true, account: "channel_customer", permissions: []string{"orders.write"}, want: http.StatusForbidden},
			{name: "customer_scope", authenticated: true, scoped: true, account: "internal_employee", permissions: []string{"orders.write"}, want: http.StatusForbidden},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				e := echo.New()
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						if tc.authenticated {
							c.Set("employee_id", int64(5))
							c.Set("actor", "内部测试员工")
						}
						c.Set("customer_fulfillment_order_scope_limited", tc.scoped)
						return next(c)
					}
				})
				authz := &orderAPIAuthzService{actor: authzapp.Actor{AccountType: tc.account, Permissions: tc.permissions}}
				registerOrderAPI(e, salesapp.NewService(nil), nil, authz)
				path := "/api/orders/117/fulfillment"
				if method == http.MethodPost {
					path += "/preview"
				}
				r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
				r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				w := httptest.NewRecorder()
				e.ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("%s: status=%d want=%d body=%s", path, w.Code, tc.want, w.Body.String())
				}
			})
		}
	}
}

type fulfillmentEditAPIFake struct {
	command                fulfillmentapp.FulfillmentOrderEditCommand
	reads, previews, saves int
	err                    error
}

func (f *fulfillmentEditAPIFake) FulfillmentOrderEdit(_ context.Context, id int64) (fulfillmentapp.FulfillmentOrderEditContext, error) {
	f.reads++
	return fulfillmentapp.FulfillmentOrderEditContext{OrderID: id, OrderNo: "SO-TEST", CustomerID: 488, FulfillmentMode: "stock", EditRevision: "revision", CanEdit: true}, f.err
}
func (f *fulfillmentEditAPIFake) PreviewFulfillmentOrderEdit(_ context.Context, c fulfillmentapp.FulfillmentOrderEditCommand) (fulfillmentapp.FulfillmentOrderEditPreview, error) {
	f.previews++
	f.command = c
	return fulfillmentapp.FulfillmentOrderEditPreview{OrderID: c.OrderID, PreviewToken: "preview"}, f.err
}
func (f *fulfillmentEditAPIFake) UpdateFulfillmentOrder(_ context.Context, c fulfillmentapp.FulfillmentOrderEditCommand) (fulfillmentapp.FulfillmentOrderEditResult, error) {
	f.saves++
	f.command = c
	return fulfillmentapp.FulfillmentOrderEditResult{OK: true, FulfillmentOrderEditPreview: fulfillmentapp.FulfillmentOrderEditPreview{OrderID: c.OrderID, EditRevision: "saved"}}, f.err
}
func fulfillmentEditAPIForTest(editor FulfillmentEditor) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(5))
			c.Set("actor", "订单维护员工")
			return next(c)
		}
	})
	authz := &orderAPIAuthzService{actor: authzapp.Actor{AccountType: "internal_employee", Permissions: []string{"orders.write"}}}
	registerOrderAPI(e, salesapp.NewService(nil), nil, authz, editor)
	return e
}
func sendFulfillmentEditTest(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}
func TestStaffFulfillmentEditRoutesStrictCommandsAndDispatch(t *testing.T) {
	f := &fulfillmentEditAPIFake{}
	e := fulfillmentEditAPIForTest(f)
	w := sendFulfillmentEditTest(e, "GET", "/api/orders/117/fulfillment", "")
	if w.Code != 200 || f.reads != 1 || f.saves != 0 || !strings.Contains(w.Body.String(), `"customer_id":488`) {
		t.Fatalf("read context=%d %s", w.Code, w.Body.String())
	}
	payload := `{"edit_revision":"revision","receiver_name":"收件人","receiver_phone":"13800000001","receiver_address":"云南省昆明市测试地址","receiver_company":"测试公司","province":"云南省","city":"昆明市","district":"五华区","order_date":"2026-10-22","notes":"编辑备注","items":[{"item_id":310,"product_id":7,"bom_spec_id":19,"bom_variant_id":1185,"qty":4}]}`
	w = sendFulfillmentEditTest(e, "POST", "/api/orders/117/fulfillment/preview", payload)
	if w.Code != 200 || f.previews != 1 || f.command.OrderID != 117 || f.command.CustomerID != 0 || f.command.Actor != "订单维护员工" || f.command.RecipientOnly || len(f.command.Items) != 1 || f.command.Items[0].ItemID != 310 {
		t.Fatalf("preview dispatch=%d %+v %s", w.Code, f.command, w.Body.String())
	}
	payload = strings.TrimSuffix(payload, "}") + `,"preview_token":"preview"}`
	w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/fulfillment", payload)
	if w.Code != 200 || f.saves != 1 || f.command.PreviewToken != "preview" || f.command.ReceiverCompany == nil || *f.command.ReceiverCompany != "测试公司" {
		t.Fatalf("save dispatch=%d %+v %s", w.Code, f.command, w.Body.String())
	}
	for _, bad := range []string{
		`{}`, `{"edit_revision":"revision"}`, `{"edit_revision":"revision","preview_token":"preview","customer_id":999}`,
		`{"edit_revision":"revision","preview_token":"preview","order_id":999}`, `{"edit_revision":"revision","preview_token":"preview","actor":"admin"}`,
		`{"edit_revision":"revision","preview_token":"preview","recipient_only":true}`, `{"edit_revision":"revision","preview_token":"preview","fulfillment_mode":"on_demand"}`,
		`{"edit_revision":"revision","preview_token":"preview","payment_goods_amount":"0"}`, `{"edit_revision":"revision","preview_token":"preview","items":[{"item_id":310,"qty":4,"customer_id":999}]}`,
		`{"edit_revision":"revision","preview_token":"preview"} {}`, `{"edit_revision":"revision","preview_token":"preview","notes":"` + strings.Repeat("x", 257<<10) + `"}`,
	} {
		w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/fulfillment", bad)
		if w.Code != 400 || f.saves != 1 {
			t.Fatalf("invalid body called writer: code=%d saves=%d", w.Code, f.saves)
		}
	}
	w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/fulfillment", `{"edit_revision":"revision","preview_token":"preview","service_fee_cents":0,"adjustment_reason":"减免"}`)
	if w.Code != 403 || f.saves != 1 {
		t.Fatalf("orders.write cannot bypass customer fee adjustment permission: %d %s", w.Code, w.Body.String())
	}
	f.err = salesapp.NewOrderEditConflictError("订单已变更，请重新核对")
	w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/fulfillment", payload)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "订单已变更") {
		t.Fatalf("conflict=%d %s", w.Code, w.Body.String())
	}
}
func TestStaffRecipientUsesFullFulfillmentTransaction(t *testing.T) {
	f := &fulfillmentEditAPIFake{}
	e := fulfillmentEditAPIForTest(f)
	w := sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/recipient", `{"customer_id":488,"edit_revision":"revision","receiver_name":"收件人","receiver_phone":"13800000001","receiver_address":"新地址"}`)
	if w.Code != 200 || f.saves != 1 || !f.command.RecipientOnly || f.command.CustomerID != 488 || f.command.OrderID != 117 || f.command.ExpectedEditRevision != "revision" || f.command.ReceiverCompany != nil || len(f.command.Items) != 0 {
		t.Fatalf("compatibility did not delegate=%d %+v %s", w.Code, f.command, w.Body.String())
	}
	w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/recipient", `{"edit_revision":"revision","items":[]}`)
	if w.Code != 400 || f.saves != 1 {
		t.Fatalf("recipient accepted item replacement=%d %s", w.Code, w.Body.String())
	}
	e = fulfillmentEditAPIForTest(nil)
	w = sendFulfillmentEditTest(e, "PATCH", "/api/orders/117/recipient", `{"edit_revision":"revision","receiver_address":"新地址"}`)
	if w.Code != 503 {
		t.Fatalf("missing fulfillment service must not fall back to address-only writes: %d %s", w.Code, w.Body.String())
	}
}
