package customerportal

import (
	"net/http"
	"strconv"

	customerportalapp "orderapp/internal/application/customerportal"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func registerRecipientAddressAPI(e *echo.Echo, portal Service, authz supporthttp.AuthzService) {
	e.GET("/api/mini/recipient-addresses", func(c echo.Context) error {
		svc, token, err := miniRecipientAddressService(c, portal)
		if err != nil {
			return err
		}
		rows, err := svc.ListRecipientAddresses(c.Request().Context(), token)
		if err != nil {
			return miniSessionError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/mini/recipient-addresses", func(c echo.Context) error {
		return saveMiniRecipientAddress(c, portal, 0)
	})
	e.PUT("/api/mini/recipient-addresses/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "address required"})
		}
		return saveMiniRecipientAddress(c, portal, id)
	})
	e.DELETE("/api/mini/recipient-addresses/:id", func(c echo.Context) error {
		svc, token, err := miniRecipientAddressService(c, portal)
		if err != nil {
			return err
		}
		id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
		revision, revisionErr := strconv.ParseInt(c.QueryParam("revision"), 10, 64)
		if parseErr != nil || id <= 0 || revisionErr != nil || revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "address and revision required"})
		}
		if err := svc.DeleteRecipientAddress(c.Request().Context(), token, id, revision); err != nil {
			return recipientAddressError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/customer-portal/admin/customers/:id/recipient-addresses", func(c echo.Context) error {
		svc, customerID, err := adminRecipientAddressService(c, portal, authz, "customers.read")
		if err != nil {
			return err
		}
		rows, err := svc.ListRecipientAddressesForCustomer(c.Request().Context(), customerID, c.QueryParam("q"))
		if err != nil {
			return recipientAddressError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/customer-portal/admin/customers/:id/recipient-addresses", func(c echo.Context) error {
		return saveAdminRecipientAddress(c, portal, authz, 0)
	})
	e.PUT("/api/customer-portal/admin/customers/:id/recipient-addresses/:address_id", func(c echo.Context) error {
		addressID, err := strconv.ParseInt(c.Param("address_id"), 10, 64)
		if err != nil || addressID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "address required"})
		}
		return saveAdminRecipientAddress(c, portal, authz, addressID)
	})
	e.DELETE("/api/customer-portal/admin/customers/:id/recipient-addresses/:address_id", func(c echo.Context) error {
		svc, customerID, err := adminRecipientAddressService(c, portal, authz, "customers.write")
		if err != nil {
			return err
		}
		addressID, parseErr := strconv.ParseInt(c.Param("address_id"), 10, 64)
		revision, revisionErr := strconv.ParseInt(c.QueryParam("revision"), 10, 64)
		if parseErr != nil || addressID <= 0 || revisionErr != nil || revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "address and revision required"})
		}
		if err := svc.DeleteRecipientAddressForCustomer(c.Request().Context(), customerportalapp.DeleteCustomerRecipientAddressCommand{ID: addressID, CustomerID: customerID, ExpectedRevision: revision, Actor: supporthttp.ActorOf(c)}); err != nil {
			return recipientAddressError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func saveMiniRecipientAddress(c echo.Context, portal Service, id int64) error {
	svc, token, err := miniRecipientAddressService(c, portal)
	if err != nil {
		return err
	}
	var cmd customerportalapp.SaveCustomerRecipientAddressCommand
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	cmd.ID = id
	row, err := svc.SaveRecipientAddress(c.Request().Context(), token, cmd)
	if err != nil {
		return recipientAddressError(c, err)
	}
	return c.JSON(http.StatusOK, row)
}

func saveAdminRecipientAddress(c echo.Context, portal Service, authz supporthttp.AuthzService, addressID int64) error {
	svc, customerID, err := adminRecipientAddressService(c, portal, authz, "customers.write")
	if err != nil {
		return err
	}
	var cmd customerportalapp.SaveCustomerRecipientAddressCommand
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	cmd.ID = addressID
	cmd.CustomerID = customerID
	cmd.Actor = supporthttp.ActorOf(c)
	row, err := svc.SaveRecipientAddressForCustomer(c.Request().Context(), cmd)
	if err != nil {
		return recipientAddressError(c, err)
	}
	return c.JSON(http.StatusOK, row)
}

func miniRecipientAddressService(c echo.Context, portal Service) (RecipientAddressService, string, error) {
	svc, ok := portal.(RecipientAddressService)
	if !ok {
		return nil, "", miniInternalError(c)
	}
	token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		return nil, "", c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
	}
	return svc, token, nil
}

func adminRecipientAddressService(c echo.Context, portal Service, authz supporthttp.AuthzService, permission string) (RecipientAddressService, int64, error) {
	svc, ok := portal.(RecipientAddressService)
	if !ok {
		return nil, 0, c.JSON(http.StatusNotImplemented, map[string]string{"error": "recipient address service unavailable"})
	}
	actor, ok, err := supporthttp.CurrentActor(c, authz)
	if err != nil || !ok || !actor.Can(permission) {
		return nil, 0, c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied", "permission": permission})
	}
	customerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || customerID <= 0 {
		return nil, 0, c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
	}
	return svc, customerID, nil
}

func recipientAddressError(c echo.Context, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if message == "收件地址已变化，请刷新后重试" {
		return c.JSON(http.StatusConflict, map[string]string{"error": message})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
}
