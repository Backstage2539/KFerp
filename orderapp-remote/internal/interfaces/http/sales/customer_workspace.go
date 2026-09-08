package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

type customerWorkspaceResolver interface {
	CustomerWorkspace(context.Context, int64, int64, string, int64) (map[string]any, error)
}

func (h orderAPIHandler) portalContext(c echo.Context, page string, id int64) (map[string]any, error) {
	resolver, ok := h.customerScope.(customerWorkspaceResolver)
	if !ok {
		return nil, fmt.Errorf("客户服务不可用")
	}
	employeeID := support.CurrentEmployeeID(c)
	customerID := int64(0)
	if allowed, _ := c.Get("portal_internal_allowed").(bool); allowed {
		customerID, _ = strconv.ParseInt(c.QueryParam("customer_id"), 10, 64)
		if customerID > 0 {
			employeeID = 0
		}
	}
	if employeeID <= 0 && customerID <= 0 {
		return nil, fmt.Errorf("需要登录客户账户")
	}
	return resolver.CustomerWorkspace(c.Request().Context(), employeeID, customerID, page, id)
}

func (h orderAPIHandler) customerWorkspace(c echo.Context) error {
	page := strings.TrimSpace(c.QueryParam("page"))
	if page == "" {
		page = "home"
	}
	id, _ := strconv.ParseInt(c.QueryParam("publication_id"), 10, 64)
	data, err := h.portalContext(c, page, id)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, data)
}
func (h orderAPIHandler) customerForm(c echo.Context) error {
	mode := c.QueryParam("service")
	if mode == "" {
		mode = "direct_ship"
	}
	if mode != "direct_ship" && mode != "product_order" {
		return c.JSON(400, map[string]string{"error": "无效录单能力"})
	}
	data, err := h.portalContext(c, mode, 0)
	if err != nil {
		return c.JSON(403, map[string]string{"error": err.Error()})
	}
	if c.QueryParam("edit_id") != "" {
		return c.JSON(403, map[string]string{"error": "客户只能补全收件信息"})
	}
	c.Set("portal_customer_id", data["customer_id"])
	return h.form(c)
}
func (h orderAPIHandler) customerRecipient(c echo.Context) error {
	data, err := h.portalContext(c, "context", 0)
	if err != nil {
		return c.JSON(403, map[string]string{"error": err.Error()})
	}
	codes, _ := data["capabilities"].([]string)
	allowed := false
	for _, code := range codes {
		if code == "direct_ship" || code == "product_order" {
			allowed = true
		}
	}
	if !allowed {
		return c.JSON(403, map[string]string{"error": "录单能力未开通"})
	}
	var cmd salesapp.CustomerRecipientCommand
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(400, map[string]string{"error": "无效收件信息"})
	}
	cmd.CustomerID, _ = data["customer_id"].(int64)
	cmd.OrderID, _ = strconv.ParseInt(c.Param("id"), 10, 64)
	cmd.Actor = support.ActorOf(c)
	if err := h.sales.UpdateCustomerRecipient(c.Request().Context(), cmd); err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, map[string]any{"ok": true, "order_id": cmd.OrderID})
}
func (h orderAPIHandler) prepareCustomerCommand(c echo.Context, req orderSaveAPIRequest, cmd *salesapp.SaveOrderCommand) error {
	for _, price := range req.UnitPrice {
		if strings.TrimSpace(price) != "" {
			return fmt.Errorf("客户订单价格必须来自所选价格表")
		}
	}
	mode := strings.TrimSpace(req.PortalServiceCode)
	if mode == "" {
		mode = "direct_ship"
	}
	if mode != "direct_ship" && mode != "product_order" {
		return fmt.Errorf("无效录单能力")
	}
	data, err := h.portalContext(c, mode, 0)
	if err != nil {
		return err
	}
	id, _ := data["customer_id"].(int64)
	if cmd.CustomerID != id {
		return fmt.Errorf("customer scope mismatch")
	}
	cmd.PortalServiceCode = mode
	cmd.CustomerSubmission = true
	cmd.BackfillMode = req.BackfillMode
	cmd.RequireCurrentDefaultPublications = true
	cmd.CustomerRequestID = strings.TrimSpace(req.RequestID)
	if len(cmd.CustomerRequestID) < 16 || len(cmd.CustomerRequestID) > 100 {
		return fmt.Errorf("录单请求标识无效，请刷新后重试")
	}
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	cmd.CustomerRequestHash = hex.EncodeToString(hash[:])
	cmd.StockBatchDecision = ""
	cmd.SourceWarehouse = ""
	cmd.ResponsibleID = 0
	cmd.ResponsibleType = ""
	cmd.ResponsibleName = ""
	cmd.OrdersScope = "fulfillment"
	return salesapp.ValidateCustomerOrder(*cmd, req.BackfillMode)
}

func scopeCustomerForm(resp *orderFormAPIResponse, id int64) {
	customers := resp.Customers[:0]
	for _, row := range resp.Customers {
		if row.ID == id {
			customers = append(customers, row)
		}
	}
	resp.Customers = customers
	resp.Employees = []employeeAPIOption{}
	versions := resp.BeanListVersionOptions[:0]
	allowed := map[int64]bool{}
	for _, row := range resp.BeanListVersionOptions {
		if row.CustomerID == id || row.CustomerID == 0 && !row.IsCustomerOwned {
			versions = append(versions, row)
			allowed[row.ID] = true
		}
	}
	resp.BeanListVersionOptions = versions
	usages := resp.CustomerPublicUsages[:0]
	for _, row := range resp.CustomerPublicUsages {
		if row.CustomerID == id {
			usages = append(usages, row)
		}
	}
	resp.CustomerPublicUsages = usages
	productUsages := resp.CustomerProductUsages[:0]
	for _, row := range resp.CustomerProductUsages {
		if row.CustomerID == id {
			productUsages = append(productUsages, row)
		}
	}
	resp.CustomerProductUsages = productUsages
	products := map[int64]bool{}
	for _, row := range resp.Products {
		for _, key := range []string{"id", "parent_product_id"} {
			if value, ok := row[key].(int64); ok {
				products[value] = true
			}
		}
	}
	specs := resp.ProductBOMSpecOptions[:0]
	for _, row := range resp.ProductBOMSpecOptions {
		if !products[row.ParentProductID] && !products[row.LegacyChildProductID] && !products[row.WriteProductID] {
			continue
		}
		tiers := []salesapp.ProductTierOption{}
		for _, tier := range row.Tiers {
			if allowed[tier.PublicationID] {
				tiers = append(tiers, tier)
			}
		}
		row.Tiers = tiers
		specs = append(specs, row)
	}
	resp.ProductBOMSpecOptions = specs
}

func (h orderAPIHandler) requireCustomerOrdersCapability(c echo.Context) error {
	if !support.CustomerFulfillmentOrderScopeLimited(c) {
		return nil
	}
	if _, ok := h.customerScope.(customerWorkspaceResolver); !ok {
		return nil
	}
	data, err := h.portalContext(c, "context", 0)
	if err != nil {
		return echo.NewHTTPError(403, err.Error())
	}
	codes, _ := data["capabilities"].([]string)
	for _, code := range codes {
		if code == "direct_ship" || code == "product_order" || code == "processing" || code == "mall" {
			return nil
		}
	}
	return echo.NewHTTPError(403, "订单能力未开通")
}
func customerDetailSpecOptions(c echo.Context, data salesapp.OrderFormData) []salesapp.ProductBOMSpecOption {
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		return []salesapp.ProductBOMSpecOption{}
	}
	return data.ProductBOMSpecOptions
}
