package sales

import (
	"github.com/labstack/echo/v4"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
)

func (h orderAPIHandler) requireConversionEmployee(c echo.Context, permission string) error {
	actor, ok, err := support.CurrentActor(c, h.authz)
	if err != nil || !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "请先登录员工账号"})
	}
	if actor.AccountType == support.AccountTypeChannelCustomer || !actor.Can(permission) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "没有规格换算操作权限"})
	}
	return nil
}
func conversionResponse(c echo.Context, data any, err error) error {
	if err != nil {
		if confirmation, ok := salesapp.PricingRuleConfirmationDetails(err); ok {
			return c.JSON(409, map[string]string{"error": err.Error(), "code": "pricing_rule_confirmation_required", "current_rule_revision": confirmation.Revision, "pricing_rule_name": confirmation.RuleName})
		}
		status := http.StatusBadRequest
		if _, ok := salesapp.OrderEditConflictMessage(err); ok {
			status = http.StatusConflict
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, data)
}
func (h orderAPIHandler) conversionSettings(c echo.Context) error {
	if err := h.requireConversionEmployee(c, "products.read"); err != nil || c.Response().Committed {
		return err
	}
	data, err := h.sales.SpecConversionSettings(c.Request().Context())
	return conversionResponse(c, data, err)
}
func (h orderAPIHandler) saveConversionGroup(c echo.Context) error {
	if err := h.requireConversionEmployee(c, "products.write"); err != nil || c.Response().Committed {
		return err
	}
	var cmd salesapp.SaveSpecConversionGroupCommand
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(400, map[string]string{"error": "换算组参数无效"})
	}
	cmd.Actor = support.ActorOf(c)
	data, err := h.sales.SaveSpecConversionGroup(c.Request().Context(), cmd)
	return conversionResponse(c, data, err)
}
func (h orderAPIHandler) orderConversions(c echo.Context) error {
	if err := h.requireConversionEmployee(c, "orders.write"); err != nil || c.Response().Committed {
		return err
	}
	customerID, _ := strconv.ParseInt(c.QueryParam("customer_id"), 10, 64)
	productID, _ := strconv.ParseInt(c.QueryParam("product_id"), 10, 64)
	publicationID, _ := strconv.ParseInt(c.QueryParam("publication_id"), 10, 64)
	data, err := h.sales.OrderSpecConversions(c.Request().Context(), customerID, productID, publicationID)
	return conversionResponse(c, data, err)
}
func (h orderAPIHandler) conversionQuote(c echo.Context) error {
	if err := h.requireConversionEmployee(c, "orders.write"); err != nil || c.Response().Committed {
		return err
	}
	var cmd salesapp.SpecConversionQuoteRequest
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(400, map[string]string{"error": "规格换算参数无效"})
	}
	data, err := h.sales.QuoteSpecConversion(c.Request().Context(), cmd)
	return conversionResponse(c, data, err)
}
