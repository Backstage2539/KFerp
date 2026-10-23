package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
)

type fulfillmentDefaultsService interface {
	LoadFulfillmentDefaults(context.Context) (app.FulfillmentDefaults, error)
	SaveFulfillmentDefaults(context.Context, app.SaveFulfillmentDefaultsCommand) (app.FulfillmentDefaults, error)
}

func registerFulfillmentDefaultsRoutes(e *echo.Echo, base Service) {
	s, ok := base.(fulfillmentDefaultsService)
	if !ok {
		return
	}
	e.GET("/api/settings/logistics/fulfillment-defaults", func(c echo.Context) error {
		out, err := s.LoadFulfillmentDefaults(c.Request().Context())
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
	e.PUT("/api/settings/logistics/fulfillment-defaults", func(c echo.Context) error {
		var req struct {
			app.FulfillmentDefaults
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if err := c.Bind(&req); err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		out, err := s.SaveFulfillmentDefaults(c.Request().Context(), app.SaveFulfillmentDefaultsCommand{Defaults: req.FulfillmentDefaults, ExpectedRevision: req.ExpectedRevision, Actor: currentCustomerFulfillmentActor(c)})
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		return c.JSON(200, out)
	})
}
