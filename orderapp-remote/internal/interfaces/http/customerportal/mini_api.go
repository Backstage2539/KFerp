package customerportal

import (
	"errors"
	"net/http"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniLoginRequest struct {
	Code     string `json:"code"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
}

type switchCustomerRequest struct {
	CustomerID int64 `json:"customer_id"`
}

func registerMiniAPI(e *echo.Echo, svc Service) {
	e.POST("/api/mini/login", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		var req miniLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.Login(c.Request().Context(), customerportalapp.LoginCommand{Code: req.Code, Phone: req.Phone, Nickname: req.Nickname})
		if err != nil {
			return miniLoginError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/me", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		result, err := svc.Me(c.Request().Context(), token)
		if err != nil {
			return miniSessionError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/current-customer", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req switchCustomerRequest
		if err := c.Bind(&req); err != nil || req.CustomerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		result, err := svc.SwitchCurrentCustomer(c.Request().Context(), token, req.CustomerID)
		if err != nil {
			return miniSwitchCustomerError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
}

func miniLoginError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniLoginDisabled) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "mini login disabled"})
	}
	if errors.Is(err, customerportalapp.ErrMiniUserDisabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "mini user disabled"})
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return miniInternalError(c)
}

func miniSessionError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	return miniInternalError(c)
}

func miniSwitchCustomerError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	if errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	return miniInternalError(c)
}

func miniInternalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func isMiniValidationError(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "code required", "openid required", "customer required", "mini token required":
		return true
	default:
		return false
	}
}

func miniTokenFromHeader(authz string) string {
	authz = strings.TrimSpace(authz)
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}
