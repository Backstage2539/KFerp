package customerportal

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

const maxRecipientParseRequestBytes = 64 << 10

type recipientParseRequest struct {
	Text string `json:"text"`
}

func registerRecipientParseAPI(e *echo.Echo, portal Service, authz supporthttp.AuthzService) {
	e.POST("/api/customer-recipient/parse", func(c echo.Context) error {
		if err := requireRecipientParseAccess(c, portal, authz); err != nil {
			return err
		}

		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxRecipientParseRequestBytes)
		decoder := json.NewDecoder(c.Request().Body)
		var req recipientParseRequest
		if err := decoder.Decode(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}

		result, err := customerapp.ParseRecipientText(req.Text)
		switch {
		case errors.Is(err, customerapp.ErrRecipientTextRequired):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请粘贴收货信息"})
		case errors.Is(err, customerapp.ErrRecipientTextTooLong):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "收货信息过长，请缩短后重试"})
		case err != nil:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "收货信息无法解析"})
		default:
			return c.JSON(http.StatusOK, result)
		}
	})
}

func requireRecipientParseAccess(c echo.Context, portal Service, authz supporthttp.AuthzService) error {
	actor, ok, err := supporthttp.CurrentActor(c, authz)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied"})
	}
	if ok {
		if !actor.Can("customers.read") {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied", "permission": "customers.read"})
		}
		return nil
	}

	token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "请先登录"})
	}
	if portal == nil {
		return miniInternalError(c)
	}
	current, err := portal.Me(c.Request().Context(), token)
	if err != nil {
		return miniSessionError(c, err)
	}
	if current.AccountType == "employee" {
		if (!containsMiniRole(current.Roles, "sales") && !containsMiniRole(current.Roles, "admin")) || !containsMiniRole(current.Permissions, "customers.read") {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "当前员工无此权限"})
		}
		return nil
	}
	if current.AccountType == "customer" && current.CurrentCustomerID > 0 && current.HasAnyCapability([]string{
		customerportalapp.CapabilityDirectShip,
		customerportalapp.CapabilityProductOrder,
	}) {
		return nil
	}
	return c.JSON(http.StatusForbidden, map[string]string{"error": "当前客户无发货权限"})
}
