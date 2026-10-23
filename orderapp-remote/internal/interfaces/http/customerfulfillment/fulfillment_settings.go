package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
)

type fulfillmentSettingsService interface {
	LoadFulfillmentSettings(context.Context, int64) (app.FulfillmentSettings, error)
	SaveFulfillmentSettings(context.Context, app.SaveFulfillmentSettingsCommand) (app.FulfillmentSettings, error)
	ListShippingFeeTemplates(context.Context) ([]app.ShippingFeeTemplate, error)
	SaveShippingFeeTemplate(context.Context, app.SaveShippingFeeTemplateCommand) (app.ShippingFeeTemplate, error)
	QuoteFulfillmentFees(context.Context, app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error)
	AdjustFulfillmentOrderFees(context.Context, app.AdjustFulfillmentOrderFeesCommand) (app.FulfillmentFeeQuote, error)
	GetFulfillmentOrderFees(context.Context, int64, int64) (app.FulfillmentFeeQuote, error)
}

func registerFulfillmentSettingsRoutes(e *echo.Echo, base Service) {
	registerFulfillmentDefaultsRoutes(e, base)
	s, ok := base.(fulfillmentSettingsService)
	if !ok {
		return
	}
	e.GET("/api/customer-fulfillment/:customer_id/settings", func(c echo.Context) error {
		id, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		out, err := s.LoadFulfillmentSettings(c.Request().Context(), id)
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		templates, err := s.ListShippingFeeTemplates(c.Request().Context())
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, map[string]any{"customer_id": out.CustomerID, "revision": out.Revision, "modes": out.Modes, "shipping_templates": templates, "defaults": out.Defaults, "defaults_revision": out.DefaultsRevision})
	})
	e.PUT("/api/customer-fulfillment/:customer_id/settings", func(c echo.Context) error {
		id, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		var req struct {
			app.FulfillmentSettings
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if err = c.Bind(&req); err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		req.CustomerID = id
		out, err := s.SaveFulfillmentSettings(c.Request().Context(), app.SaveFulfillmentSettingsCommand{Settings: req.FulfillmentSettings, ExpectedRevision: req.ExpectedRevision, Actor: currentCustomerFulfillmentActor(c)})
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
	e.POST("/api/customer-fulfillment/:customer_id/fees/preview", func(c echo.Context) error {
		id, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		var req app.FulfillmentFeeQuoteCommand
		if err = c.Bind(&req); err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		req.CustomerID = id
		req.EmployeeAdjustment = true
		out, err := s.QuoteFulfillmentFees(c.Request().Context(), req)
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
	e.PUT("/api/customer-fulfillment/:customer_id/orders/:order_id/fees", func(c echo.Context) error {
		id, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		orderID, err := parseID(c.Param("order_id"), "order")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		var req app.AdjustFulfillmentOrderFeesCommand
		if err = c.Bind(&req); err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		req.CustomerID = id
		req.OrderID = orderID
		req.Actor = currentCustomerFulfillmentActor(c)
		out, err := s.AdjustFulfillmentOrderFees(c.Request().Context(), req)
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
	e.GET("/api/customer-fulfillment/:customer_id/orders/:order_id/fees", func(c echo.Context) error {
		id, err := parseID(c.Param("customer_id"), "customer")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		orderID, err := parseID(c.Param("order_id"), "order")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		out, err := s.GetFulfillmentOrderFees(c.Request().Context(), id, orderID)
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
	e.GET("/api/settings/logistics/shipping-templates", func(c echo.Context) error {
		out, err := s.ListShippingFeeTemplates(c.Request().Context())
		if err != nil {
			return customerFulfillmentError(c, http.StatusInternalServerError, err)
		}
		return c.JSON(200, map[string]any{"templates": out})
	})
	save := func(c echo.Context) error {
		var req struct {
			app.ShippingFeeTemplate
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if err := c.Bind(&req); err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		if c.Param("id") != "" {
			id, err := parseID(c.Param("id"), "template")
			if err != nil {
				return customerFulfillmentError(c, 400, err)
			}
			req.ID = id
		} else {
			req.ID = 0
		}
		out, err := s.SaveShippingFeeTemplate(c.Request().Context(), app.SaveShippingFeeTemplateCommand{Template: req.ShippingFeeTemplate, ExpectedRevision: req.ExpectedRevision, Actor: currentCustomerFulfillmentActor(c)})
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	}
	e.POST("/api/settings/logistics/shipping-templates", save)
	e.PUT("/api/settings/logistics/shipping-templates/:id", save)
}
