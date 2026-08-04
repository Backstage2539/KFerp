package customerportal

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	customerapp "orderapp/internal/application/customer"
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

	if _, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "customers.read"); err != nil {
		return miniEmployeeAuthError(c, err)
	}
	return nil
}
