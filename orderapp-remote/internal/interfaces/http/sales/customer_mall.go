package sales

import (
	"context"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerportal"
	support "orderapp/internal/interfaces/http/support"
)

type CustomerMallService interface {
	GetERPMallPage(context.Context, int64) (app.MallPage, error)
	CreateERPMallOrder(context.Context, app.CreateMallOrderCommand) (app.FulfillmentOrder, error)
}

func registerCustomerMallRoutes(e *echo.Echo, deps Dependencies) {
	h := orderAPIHandler{customerScope: deps.CustomerScope}
	e.GET("/api/customer-processing/portal/mall", func(c echo.Context) error {
		scope, err := h.portalContext(c, "mall", 0)
		if err != nil {
			return c.JSON(403, map[string]string{"error": err.Error()})
		}
		if deps.CustomerMall == nil {
			return c.JSON(503, map[string]string{"error": "商城暂不可用"})
		}
		id, _ := scope["customer_id"].(int64)
		data, err := deps.CustomerMall.GetERPMallPage(c.Request().Context(), id)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, data)
	})
	e.POST("/api/customer-processing/portal/mall/orders", func(c echo.Context) error {
		scope, err := h.portalContext(c, "mall", 0)
		if err != nil {
			return c.JSON(403, map[string]string{"error": err.Error()})
		}
		var req struct {
			Name    string `json:"recipient_name"`
			Phone   string `json:"recipient_phone"`
			Address string `json:"recipient_address"`
			Note    string `json:"note"`
			Items   []struct {
				ID        int64  `json:"mall_product_id"`
				Qty       int64  `json:"qty"`
				SalesUnit string `json:"sales_unit"`
			} `json:"items"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "无效订单"})
		}
		id, _ := scope["customer_id"].(int64)
		cmd := app.CreateMallOrderCommand{Actor: support.ActorOf(c), CustomerID: id, RecipientName: req.Name, RecipientPhone: req.Phone, RecipientAddress: req.Address, Note: req.Note}
		for _, item := range req.Items {
			cmd.Items = append(cmd.Items, app.MallOrderItemCommand{MallProductID: item.ID, Qty: item.Qty, SalesUnit: item.SalesUnit})
		}
		if deps.CustomerMall == nil {
			return c.JSON(503, map[string]string{"error": "商城暂不可用"})
		}
		result, err := deps.CustomerMall.CreateERPMallOrder(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, result)
	})
}
