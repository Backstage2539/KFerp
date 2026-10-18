package customerportal

import (
	"github.com/labstack/echo/v4"
	"net/http"
	salesapp "orderapp/internal/application/sales"
)

func registerMiniSpecConversions(e *echo.Echo, portal Service, sales EmployeeSales) {
	retired := func(c echo.Context) error {
		if _, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write"); err != nil {
			return miniEmployeeAuthError(c, err)
		}
		return c.JSON(http.StatusGone, map[string]string{"error": salesapp.PriceTableOrderScopeMessage})
	}
	e.GET("/api/mini/employee/spec-conversions", retired)
	e.POST("/api/mini/employee/spec-conversions/quote", retired)
}
