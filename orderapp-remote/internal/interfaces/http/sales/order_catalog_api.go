package sales

import (
	"github.com/labstack/echo/v4"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

func (h orderAPIHandler) catalog(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	customerID, err := strconv.ParseInt(c.QueryParam("customer_id"), 10, 64)
	if err != nil || customerID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "客户编号不正确"})
	}
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		if h.customerScope == nil {
			return echo.NewHTTPError(http.StatusForbidden, "客户范围不可用")
		}
		bound, scopeErr := h.customerScope.BoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if scopeErr != nil || bound != customerID {
			return echo.NewHTTPError(http.StatusForbidden, "客户范围不匹配")
		}
	}
	if portalID, ok := c.Get("portal_customer_id").(int64); ok && portalID != customerID {
		return echo.NewHTTPError(http.StatusForbidden, "客户范围不匹配")
	}
	form, err := h.sales.OrderForm(c.Request().Context(), 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "商品目录读取失败")
	}
	found := false
	for _, customer := range form.Customers {
		if customer.ID == customerID {
			found = true
			break
		}
	}
	if !found {
		return echo.NewHTTPError(http.StatusNotFound, "客户不存在")
	}
	ids := []int64{}
	if raw := strings.TrimSpace(c.QueryParam("selected_price_table_ids")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, parseErr := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if parseErr != nil || id <= 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "价格表编号不正确")
			}
			ids = append(ids, id)
		}
	}
	resolved, err := salesapp.ResolveOrderCatalog(form, customerID, ids, true)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"customer_id": customerID, "catalog_updated": resolved.Updated, "products": resolved.Catalog.Products, "product_families": resolved.Catalog.Families, "product_bom_spec_options": resolved.Catalog.BOMSpecOptions, "price_table_options": resolved.PriceTableOptions, "selected_price_table_ids": resolved.SelectedPriceTableIDs})
}
