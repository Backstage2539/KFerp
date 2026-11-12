package sales

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
)

func operationOptionID(c echo.Context, key string, required bool) (int64, error) {
	raw := c.QueryParam(key)
	if raw == "" && !required {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 || (required && id == 0) {
		return 0, fmt.Errorf("无效的 %s", key)
	}
	return id, nil
}

func (h orderAPIHandler) operationOptions(c echo.Context) error {
	// Customer accounts must enter through the bound portal scope, even if a
	// legacy role accidentally grants the internal order-entry permission.
	if c.Get("portal_customer_id") == nil {
		actor, _, err := support.CurrentActor(c, h.authz)
		if err != nil || actor.AccountType == support.AccountTypeChannelCustomer {
			return c.JSON(403, map[string]string{"error": "请从客户录单入口选择工序"})
		}
	}
	product, err := operationOptionID(c, "product_id", true)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	spec, err := operationOptionID(c, "bom_spec_id", false)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	variant, err := operationOptionID(c, "bom_variant_id", false)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	customer, err := operationOptionID(c, "customer_id", false)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	if portal, ok := c.Get("portal_customer_id").(int64); ok {
		if customer > 0 && customer != portal {
			return c.JSON(403, map[string]string{"error": "客户不匹配"})
		}
		customer = portal
	} else if support.CustomerFulfillmentOrderScopeLimited(c) {
		if h.customerScope == nil {
			return c.JSON(403, map[string]string{"error": "客户服务不可用"})
		}
		bound, e := h.customerScope.BoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if e != nil || bound <= 0 || customer > 0 && customer != bound {
			return c.JSON(403, map[string]string{"error": "客户不匹配"})
		}
		customer = bound
	}
	if customer > 0 {
		data, e := h.sales.OrderForm(c.Request().Context(), 0)
		if e != nil {
			return c.JSON(400, map[string]string{"error": e.Error()})
		}
		visible := salesapp.FilterOrderProductsForCustomer(data.Products, customer, data.BeanListVersionOptions, data.CustomerPublicUsages)
		found := false
		for _, p := range visible {
			if p.ID == product {
				found = true
				break
			}
		}
		if !found {
			return c.JSON(403, map[string]string{"error": "当前客户不可使用该商品"})
		}
	}
	options, err := h.sales.OrderOperationOptions(c.Request().Context(), product, spec, variant)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	if options == nil {
		options = []salesapp.OrderOperationOption{}
	}
	return c.JSON(200, map[string]any{"options": options})
}

func (h orderAPIHandler) customerOperationOptions(c echo.Context) error {
	mode := c.QueryParam("service")
	if mode == "" {
		mode = "product_order"
	}
	if mode != "direct_ship" && mode != "product_order" {
		return c.JSON(400, map[string]string{"error": "无效录单能力"})
	}
	scope, err := h.portalContext(c, mode, 0)
	if err != nil {
		return c.JSON(403, map[string]string{"error": err.Error()})
	}
	customer, ok := scope["customer_id"].(int64)
	if !ok || customer <= 0 {
		return c.JSON(403, map[string]string{"error": "客户绑定不可用"})
	}
	c.Set("portal_customer_id", customer)
	return h.operationOptions(c)
}

type customerMallOperationOptionsService interface {
	GetERPMallOperationOptions(context.Context, int64, int64) ([]salesapp.OrderOperationOption, error)
}

func registerCustomerMallOperationOptionsRoute(e *echo.Echo, deps Dependencies) {
	h := orderAPIHandler{customerScope: deps.CustomerScope}
	e.GET("/api/customer-processing/portal/mall/operation-options", func(c echo.Context) error {
		scope, err := h.portalContext(c, "mall", 0)
		if err != nil {
			return c.JSON(403, map[string]string{"error": err.Error()})
		}
		customer, _ := scope["customer_id"].(int64)
		if customer <= 0 {
			return c.JSON(403, map[string]string{"error": "客户绑定不可用"})
		}
		requested, err := operationOptionID(c, "customer_id", false)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if requested > 0 && requested != customer {
			return c.JSON(403, map[string]string{"error": "客户不匹配"})
		}
		id, err := operationOptionID(c, "mall_product_id", true)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		svc, ok := deps.CustomerMall.(customerMallOperationOptionsService)
		if !ok {
			return c.JSON(503, map[string]string{"error": "商城工序候选暂不可用"})
		}
		options, err := svc.GetERPMallOperationOptions(c.Request().Context(), customer, id)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if options == nil {
			options = []salesapp.OrderOperationOption{}
		}
		return c.JSON(200, map[string]any{"options": options})
	})
}
