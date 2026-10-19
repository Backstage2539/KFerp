package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

type employeeDirectShipService interface {
	MiniDirectShipCatalog(context.Context, app.MiniDirectShipCatalogQuery) (app.MiniDirectShipCatalog, error)
	PreviewMiniDirectShip(context.Context, app.MiniDirectShipCommand) (app.MiniDirectShipPreview, error)
	SubmitMiniDirectShip(context.Context, app.MiniDirectShipCommand) (app.MiniDirectShipRequest, error)
}

// AuthorizationMiddleware restricts these adapters to internal stock.write.
// Customer, employee and actor always come from the path/authenticated session.
func registerEmployeeDirectShipAPI(e *echo.Echo, service Service) {
	getService := func(c echo.Context) (employeeDirectShipService, int64, error) {
		id, err := strconv.ParseInt(c.Param("customer_id"), 10, 64)
		if err != nil || id <= 0 {
			return nil, 0, echo.NewHTTPError(http.StatusBadRequest, "customer required")
		}
		svc, ok := service.(employeeDirectShipService)
		if !ok {
			return nil, 0, echo.NewHTTPError(http.StatusServiceUnavailable, "代发服务暂不可用")
		}
		return svc, id, nil
	}
	e.GET("/api/customer-fulfillment/:customer_id/direct-ship/catalog", func(c echo.Context) error {
		svc, id, err := getService(c)
		if err != nil {
			return err
		}
		result, err := svc.MiniDirectShipCatalog(c.Request().Context(), app.MiniDirectShipCatalogQuery{CustomerID: id, Q: c.QueryParam("q"), Category: c.QueryParam("category"), FulfillmentMode: strings.TrimSpace(c.QueryParam("fulfillment_mode"))})
		if err != nil {
			return customerFulfillmentError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	for _, submit := range []bool{false, true} {
		submit := submit
		path := "/api/customer-fulfillment/:customer_id/direct-ship/preview"
		if submit {
			path = "/api/customer-fulfillment/:customer_id/direct-ship/requests"
		}
		e.POST(path, func(c echo.Context) error {
			svc, id, err := getService(c)
			if err != nil {
				return err
			}
			var cmd app.MiniDirectShipCommand
			if err := c.Bind(&cmd); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
			}
			cmd.CustomerID = id
			cmd.EmployeeID = support.CurrentEmployeeID(c)
			cmd.MiniUserID = 0
			cmd.Actor = currentCustomerFulfillmentActor(c)
			if submit {
				result, err := svc.SubmitMiniDirectShip(c.Request().Context(), cmd)
				if err != nil {
					return customerFulfillmentError(c, http.StatusBadRequest, err)
				}
				return c.JSON(http.StatusCreated, result)
			}
			result, err := svc.PreviewMiniDirectShip(c.Request().Context(), cmd)
			if err != nil {
				return customerFulfillmentError(c, http.StatusBadRequest, err)
			}
			return c.JSON(http.StatusOK, result)
		})
	}
}
