package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
	"testing"
)

type employeeCancelAPIFake struct {
	Service
	cmd app.CancelEmployeeDirectShipOrderCommand
}

func (f *employeeCancelAPIFake) CancelEmployeeDirectShipOrder(_ context.Context, c app.CancelEmployeeDirectShipOrderCommand) (app.MiniDirectShipRequest, error) {
	f.cmd = c
	return app.MiniDirectShipRequest{ID: 71, Status: "cancelled"}, nil
}
func TestEmployeeCancelDirectShipOrderAPIUsesPathScopeAndSessionActor(t *testing.T) {
	f := &employeeCancelAPIFake{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("operator_employee", "内部员工"); return next(c) }
	})
	registerEmployeeDirectShipCancelAPI(e, f)
	req := httptest.NewRequest("POST", "/api/customer-fulfillment/147/direct-ship/orders/98/cancel", strings.NewReader(`{"customer_id":999,"order_id":999,"actor":"外部伪造"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || f.cmd.CustomerID != 147 || f.cmd.OrderID != 98 || f.cmd.Actor != "内部员工" {
		t.Fatalf("status=%d command=%+v body=%s", rec.Code, f.cmd, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("POST", "/api/customer-fulfillment/0/direct-ship/orders/98/cancel", nil))
	if rec.Code != 400 {
		t.Fatalf("invalid customer status=%d", rec.Code)
	}
}
