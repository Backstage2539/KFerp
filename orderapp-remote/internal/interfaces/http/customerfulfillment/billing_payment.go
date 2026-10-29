package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
)

type billingPaymentService interface {
	ChangeBillingPayment(context.Context, app.BillingPaymentCommand) (app.BillingBill, error)
}

func (a api) changeBillingPayment(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	billID, err := parseID(c.Param("bill_id"), "bill")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var cmd app.BillingPaymentCommand
	if err = c.Bind(&cmd); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	svc, ok := a.svc.(billingPaymentService)
	if !ok {
		return customerFulfillmentError(c, http.StatusServiceUnavailable, fmt.Errorf("账单付款功能不可用"))
	}
	cmd.CustomerID, cmd.BatchID, cmd.Actor = customerID, billID, currentCustomerFulfillmentActor(c)
	bill, err := svc.ChangeBillingPayment(c.Request().Context(), cmd)
	if err != nil {
		return customerFulfillmentError(c, http.StatusConflict, err)
	}
	return c.JSON(http.StatusOK, bill)
}
