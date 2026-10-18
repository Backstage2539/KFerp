package customerfulfillment

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	portal "orderapp/internal/application/customerportal"
	"orderapp/internal/interfaces/http/support"
)

type portalRecipientAddresses interface {
	ListRecipientAddressesForCustomer(context.Context, int64, string) ([]portal.CustomerRecipientAddress, error)
	SaveRecipientAddressForCustomer(context.Context, portal.SaveCustomerRecipientAddressCommand) (portal.CustomerRecipientAddress, error)
	DeleteRecipientAddressForCustomer(context.Context, portal.DeleteCustomerRecipientAddressCommand) error
}

// Portal routes never accept a customer selector. Resolve the active ERP binding
// on every request; the application service revalidates mode, stock and quotes.
func registerPortalDirectShipAPI(e *echo.Echo, deps Dependencies) {
	resolve := func(c echo.Context) (int64, error) {
		employeeID := support.CurrentEmployeeID(c)
		if employeeID <= 0 {
			return 0, echo.NewHTTPError(http.StatusUnauthorized, "employee required")
		}
		overview, err := deps.CustomerFulfillment.CustomerPortalOverview(c.Request().Context(), employeeID)
		if err != nil {
			return 0, echo.NewHTTPError(http.StatusForbidden, "客户履约账号不可用")
		}
		if overview.CustomerID <= 0 {
			return 0, echo.NewHTTPError(http.StatusForbidden, "customer required")
		}
		for _, code := range overview.Capabilities {
			if code == "direct_ship" {
				return overview.CustomerID, nil
			}
		}
		return 0, echo.NewHTTPError(http.StatusForbidden, "未开通一件代发")
	}
	e.GET("/api/customer-processing/portal/direct-ship/settings", func(c echo.Context) error {
		id, err := resolve(c)
		if err != nil {
			return err
		}
		svc, ok := deps.CustomerFulfillment.(interface {
			LoadFulfillmentSettings(context.Context, int64) (app.FulfillmentSettings, error)
		})
		if !ok {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "代发服务暂不可用")
		}
		settings, err := svc.LoadFulfillmentSettings(c.Request().Context(), id)
		if err != nil {
			return customerPortalError(c, err)
		}
		return c.JSON(http.StatusOK, settings)
	})
	e.GET("/api/customer-processing/portal/direct-ship/catalog", func(c echo.Context) error {
		id, err := resolve(c)
		if err != nil {
			return err
		}
		svc, ok := deps.CustomerFulfillment.(employeeDirectShipService)
		if !ok {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "代发服务暂不可用")
		}
		result, err := svc.MiniDirectShipCatalog(c.Request().Context(), app.MiniDirectShipCatalogQuery{CustomerID: id, Q: c.QueryParam("q"), Category: c.QueryParam("category"), FulfillmentMode: strings.TrimSpace(c.QueryParam("fulfillment_mode"))})
		if err != nil {
			return customerPortalError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	for _, submit := range []bool{false, true} {
		submit := submit
		path := "/api/customer-processing/portal/direct-ship/preview"
		if submit {
			path = "/api/customer-processing/portal/direct-ship/requests"
		}
		e.POST(path, func(c echo.Context) error {
			id, err := resolve(c)
			if err != nil {
				return err
			}
			svc, ok := deps.CustomerFulfillment.(employeeDirectShipService)
			if !ok {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "代发服务暂不可用")
			}
			var cmd app.MiniDirectShipCommand
			if err := c.Bind(&cmd); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
			}
			cmd.CustomerID = id
			cmd.UsageCode = "direct_ship"
			cmd.EmployeeID = support.CurrentEmployeeID(c)
			cmd.MiniUserID = 0
			cmd.Actor = currentCustomerFulfillmentActor(c)
			if submit {
				result, err := svc.SubmitMiniDirectShip(c.Request().Context(), cmd)
				if err != nil {
					return customerPortalError(c, err)
				}
				return c.JSON(http.StatusCreated, result)
			}
			result, err := svc.PreviewMiniDirectShip(c.Request().Context(), cmd)
			if err != nil {
				return customerPortalError(c, err)
			}
			return c.JSON(http.StatusOK, result)
		})
	}
	addresses, ok := deps.SharedProcessing.(portalRecipientAddresses)
	if !ok {
		return
	}
	const base = "/api/customer-processing/portal/recipient-addresses"
	e.GET(base, func(c echo.Context) error {
		id, err := resolve(c)
		if err != nil {
			return err
		}
		rows, err := addresses.ListRecipientAddressesForCustomer(c.Request().Context(), id, c.QueryParam("q"))
		if err != nil {
			return customerPortalError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	save := func(c echo.Context) error {
		id, err := resolve(c)
		if err != nil {
			return err
		}
		addressID := int64(0)
		if c.Param("address_id") != "" {
			addressID, err = parseID(c.Param("address_id"), "address")
			if err != nil {
				return customerFulfillmentError(c, 400, err)
			}
		}
		var cmd portal.SaveCustomerRecipientAddressCommand
		if err := c.Bind(&cmd); err != nil {
			return echo.NewHTTPError(400, "invalid request")
		}
		cmd.CustomerID = id
		cmd.ID = addressID
		cmd.Actor = support.ActorOf(c)
		row, err := addresses.SaveRecipientAddressForCustomer(c.Request().Context(), cmd)
		if err != nil {
			return customerPortalError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	e.POST(base, save)
	e.PUT(base+"/:address_id", save)
	e.DELETE(base+"/:address_id", func(c echo.Context) error {
		id, err := resolve(c)
		if err != nil {
			return err
		}
		addressID, err := parseID(c.Param("address_id"), "address")
		if err != nil {
			return customerFulfillmentError(c, 400, err)
		}
		revision, err := strconv.ParseInt(c.QueryParam("revision"), 10, 64)
		if err != nil || revision <= 0 {
			return echo.NewHTTPError(400, "revision required")
		}
		err = addresses.DeleteRecipientAddressForCustomer(c.Request().Context(), portal.DeleteCustomerRecipientAddressCommand{ID: addressID, CustomerID: id, ExpectedRevision: revision, Actor: support.ActorOf(c)})
		if err != nil {
			return customerPortalError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
}
