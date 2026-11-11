package sales

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"
)

func TestInitialReceiptDateMapping(t *testing.T) {
	var req orderSaveAPIRequest
	if err := json.Unmarshal([]byte(`{"customer_id":3,"order_date":"2026-11-10","receipt_date":"2026-11-09","prepayment_amount":"20"}`), &req); err != nil {
		t.Fatal(err)
	}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), 0, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.ReceiptDate != "2026-11-09" || cmd.PrepaymentAmount == nil || *cmd.PrepaymentAmount != 20 {
		t.Fatalf("mapping=%+v", cmd)
	}
}

func TestInitialReceiptCreateRequestHasIdempotency(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)
	body := `{"order_date":"2026-05-09","customer_id":3,"source_id":1,"order_type_id":1,"pay_status_id":2,"payment_method":"微信支付","prepayment_amount":"20","receipt_date":"2026-11-09","request_id":"initial-order-request-123","ship_status_id":1,"product_id":["7"],"tier_id":["manual"],"unit_price":["88"],"item_name":["花魁"],"qty":["1"],"unit":["件"],"spec":["100"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || repo.cmd.CustomerRequestID != "initial-order-request-123" || repo.cmd.CustomerRequestHash == "" || repo.cmd.ReceiptDate != "2026-11-09" {
		t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, repo.cmd, rec.Body.String())
	}
}
