package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
)

type employeeDirectShipCancelService interface {
	CancelEmployeeDirectShipOrder(context.Context, app.CancelEmployeeDirectShipOrderCommand) (app.MiniDirectShipRequest, error)
}

func registerEmployeeDirectShipCancelAPI(e *echo.Echo, base Service) {
	e.POST("/api/customer-fulfillment/:customer_id/direct-ship/orders/:order_id/cancel", func(c echo.Context) error {
		customerID, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		orderID, err := parseID(c.Param("order_id"), "order")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		s, ok := base.(employeeDirectShipCancelService)
		if !ok {
			return c.JSON(503, map[string]string{"error": "代发取消服务暂不可用"})
		}
		out, err := s.CancelEmployeeDirectShipOrder(c.Request().Context(), app.CancelEmployeeDirectShipOrderCommand{CustomerID: customerID, OrderID: orderID, Actor: currentCustomerFulfillmentActor(c)})
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
}
