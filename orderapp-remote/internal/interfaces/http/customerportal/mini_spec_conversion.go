package customerportal

import (
	"context"
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	"strconv"
)

type employeeSpecConversions interface {
	OrderSpecConversions(context.Context, int64, int64, int64) (salesapp.SpecConversionCatalog, error)
	QuoteSpecConversion(context.Context, salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, error)
}

func registerMiniSpecConversions(e *echo.Echo, portal Service, sales EmployeeSales) {
	e.GET("/api/mini/employee/spec-conversions", func(c echo.Context) error {
		if _, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write"); err != nil {
			return miniEmployeeAuthError(c, err)
		}
		svc, ok := sales.(employeeSpecConversions)
		if !ok {
			return miniInternalError(c)
		}
		customerID, _ := strconv.ParseInt(c.QueryParam("customer_id"), 10, 64)
		productID, _ := strconv.ParseInt(c.QueryParam("product_id"), 10, 64)
		publicationID, _ := strconv.ParseInt(c.QueryParam("publication_id"), 10, 64)
		data, err := svc.OrderSpecConversions(c.Request().Context(), customerID, productID, publicationID)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, data)
	})
	e.POST("/api/mini/employee/spec-conversions/quote", func(c echo.Context) error {
		if _, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write"); err != nil {
			return miniEmployeeAuthError(c, err)
		}
		svc, ok := sales.(employeeSpecConversions)
		if !ok {
			return miniInternalError(c)
		}
		var cmd salesapp.SpecConversionQuoteRequest
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(400, map[string]string{"error": "规格换算参数无效"})
		}
		data, err := svc.QuoteSpecConversion(c.Request().Context(), cmd)
		if err != nil {
			if confirmation, ok := salesapp.PricingRuleConfirmationDetails(err); ok {
				return c.JSON(409, map[string]string{"error": err.Error(), "code": "pricing_rule_confirmation_required", "current_rule_revision": confirmation.Revision, "pricing_rule_name": confirmation.RuleName})
			}
			status := 400
			if _, ok := salesapp.OrderEditConflictMessage(err); ok {
				status = 409
			}
			return c.JSON(status, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, data)
	})
}
