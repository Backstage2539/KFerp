package customerfulfillment

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	"orderapp/internal/interfaces/http/support"
)

// portalWorkbenchScope always resolves the customer from the signed-in identity.
func (a api) portalWorkbenchScope(c echo.Context, capability string) (app.CustomerPortalOverview, error) {
	id := support.CurrentEmployeeID(c)
	if id <= 0 {
		return app.CustomerPortalOverview{}, echo.NewHTTPError(http.StatusUnauthorized, "请先登录")
	}
	o, err := a.svc.CustomerPortalOverview(c.Request().Context(), id)
	if err != nil || o.CustomerID <= 0 {
		return app.CustomerPortalOverview{}, echo.NewHTTPError(http.StatusForbidden, "客户账号不可用")
	}
	if capability != "" && !portalHasCapability(o, capability) {
		return app.CustomerPortalOverview{}, echo.NewHTTPError(http.StatusForbidden, "客户未开通此功能")
	}
	return o, nil
}
func portalHasCapability(o app.CustomerPortalOverview, capability string) bool {
	for _, v := range o.Capabilities {
		if v == capability {
			return true
		}
	}
	return false
}
func (a api) portalWorkbenchOverview(c echo.Context) error {
	bound, err := a.portalWorkbenchScope(c, "")
	if err != nil {
		return err
	}
	o, err := a.svc.Overview(c.Request().Context(), app.OverviewQuery{CustomerID: bound.CustomerID})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	result := app.Overview{CustomerID: bound.CustomerID, CustomerName: bound.CustomerName, Capabilities: bound.Capabilities}
	if portalHasCapability(bound, "inventory_custody") || portalHasCapability(bound, "direct_ship") {
		result.Assets = o.Assets
	}
	if portalHasCapability(bound, "processing") {
		result.ProcessingOrders = o.ProcessingOrders
	}
	if portalHasCapability(bound, "settlement") {
		result.Fees = o.Fees
		result.ChargeTotals = o.ChargeTotals
		result.Operations.PendingPayment = o.Operations.PendingPayment
		result.Operations.PendingOrderCents = o.Operations.PendingOrderCents
		result.Operations.UnsettledFeeCents = o.Operations.UnsettledFeeCents
		result.Operations.UnsettledFeeCount = o.Operations.UnsettledFeeCount
	}
	if portalHasCapability(bound, "direct_ship") || portalHasCapability(bound, "product_order") || portalHasCapability(bound, "processing") || portalHasCapability(bound, "mall") {
		result.Operations.PendingShipment = o.Operations.PendingShipment
		result.Operations.StockWaiting = o.Operations.StockWaiting
		result.Operations.MissingRecipient = o.Operations.MissingRecipient
		result.Operations.PendingConfirmation = o.Operations.PendingConfirmation
	}
	return c.JSON(http.StatusOK, result)
}
func (a api) portalWorkbenchBilling(handler echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		bound, err := a.portalWorkbenchScope(c, "settlement")
		if err != nil {
			return err
		}
		bill := c.Param("bill_id")
		if bill != "" {
			c.SetParamNames("customer_id", "bill_id")
			c.SetParamValues(strconv.FormatInt(bound.CustomerID, 10), bill)
		} else {
			c.SetParamNames("customer_id")
			c.SetParamValues(strconv.FormatInt(bound.CustomerID, 10))
		}
		if handler == nil {
			return fmt.Errorf("missing billing handler")
		}
		return handler(c)
	}
}
