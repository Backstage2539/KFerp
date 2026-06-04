package support

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const customerFulfillmentOrderScopeLimitedContextKey = "customer_fulfillment_order_scope_limited"
const customerFinanceScopeLimitedContextKey = "customer_finance_scope_limited"

func AuthorizationMiddleware(authz AuthzService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isFulfillmentOrderListRequest(c.Request().Method, c.Request().URL.Path, c.QueryParam("scope")) {
				if err := authorizeFulfillmentOrderList(c, authz); err != nil {
					return err
				}
				return next(c)
			}
			if isFulfillmentOrderDetailRequest(c.Request().Method, c.Request().URL.Path) {
				if err := authorizeFulfillmentOrderList(c, authz); err != nil {
					return err
				}
				return next(c)
			}
			if isCustomerFinanceReadRequest(c.Request().Method, c.Request().URL.Path) {
				if err := authorizeCustomerFinanceRead(c, authz); err != nil {
					return err
				}
				return next(c)
			}
			permission := requiredPermissionForRequest(c.Request().Method, c.Request().URL.Path)
			if permission == "" {
				return next(c)
			}
			if err := requireCurrentPermission(c, authz, permission); err != nil {
				return err
			}
			return next(c)
		}
	}
}

func CustomerFulfillmentOrderScopeLimited(c echo.Context) bool {
	v, _ := c.Get(customerFulfillmentOrderScopeLimitedContextKey).(bool)
	return v
}

func CustomerFinanceScopeLimited(c echo.Context) bool {
	v, _ := c.Get(customerFinanceScopeLimitedContextKey).(bool)
	return v
}

func isFulfillmentOrderListRequest(method, path, scope string) bool {
	path = strings.TrimSpace(path)
	return method == http.MethodGet &&
		(path == "/api/orders" || path == "/app/api/orders") &&
		strings.TrimSpace(scope) == "fulfillment"
}

func isFulfillmentOrderDetailRequest(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/app")
	if !strings.HasPrefix(path, "/api/orders/") || !strings.HasSuffix(path, "/detail") {
		return false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/orders/"), "/detail")
	if idPart == "" || strings.Contains(idPart, "/") {
		return false
	}
	id, err := strconv.ParseInt(idPart, 10, 64)
	return err == nil && id > 0
}

func isCustomerFinanceReadRequest(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/app")
	if path == "/api/finance/expenses" {
		return true
	}
	if !strings.HasPrefix(path, "/api/finance/reports/") {
		return false
	}
	if strings.HasSuffix(path, "/accountant-handoff.xlsx") {
		return false
	}
	if strings.Contains(path, "/close") {
		return false
	}
	return true
}

func authorizeFulfillmentOrderList(c echo.Context, authz AuthzService) error {
	actor, ok, err := CurrentActor(c, authz)
	if err != nil {
		return permissionJSONError(c, http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !ok {
		return permissionJSONError(c, http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	if actor.Can("orders.read") {
		return nil
	}
	if actor.Can("customer_processing.read") {
		c.Set(customerFulfillmentOrderScopeLimitedContextKey, true)
		return nil
	}
	return permissionJSONError(c, http.StatusForbidden, map[string]string{"error": "permission denied", "permission": "orders.read"})
}

func authorizeCustomerFinanceRead(c echo.Context, authz AuthzService) error {
	actor, ok, err := CurrentActor(c, authz)
	if err != nil {
		return permissionJSONError(c, http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !ok {
		return permissionJSONError(c, http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	if actor.Can("finance.read") {
		return nil
	}
	if actor.Can("customer_processing.read") {
		c.Set(customerFinanceScopeLimitedContextKey, true)
		return nil
	}
	return permissionJSONError(c, http.StatusForbidden, map[string]string{"error": "permission denied", "permission": "finance.read"})
}

func requiredPermissionForRequest(method, path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}
	if strings.HasPrefix(path, "/api/auth/") {
		return ""
	}
	if strings.HasPrefix(path, "/api/req/") || strings.HasPrefix(path, "/api/migrate/") {
		return "requirements.manage"
	}
	if strings.HasPrefix(path, "/api/audit") {
		return "audit.read"
	}
	if strings.HasPrefix(path, "/api/ui-settings") {
		if method == http.MethodGet {
			return ""
		}
		return "settings.write"
	}
	if path == "/api/company/profile" {
		return "settings.write"
	}
	if path == "/api/settings/sales-order/seals" && method == http.MethodGet {
		return "orders.write"
	}
	if strings.HasPrefix(path, "/api/company/") {
		return "company.manage"
	}
	if strings.HasPrefix(path, "/api/settings/") || strings.HasPrefix(path, "/api/outsource/") {
		return "settings.write"
	}
	if strings.HasPrefix(path, "/api/product-settings") {
		if method == http.MethodGet {
			return "products.read"
		}
		return "products.write"
	}
	if strings.HasPrefix(path, "/api/products/inventory") {
		if method == http.MethodGet {
			return "stock.read"
		}
		return "stock.write"
	}
	if strings.HasPrefix(path, "/api/products") {
		if method == http.MethodGet {
			return "products.read"
		}
		return "products.write"
	}
	if strings.HasPrefix(path, "/api/customers") {
		if method == http.MethodGet {
			return "customers.read"
		}
		return "customers.write"
	}
	if strings.HasPrefix(path, "/api/customer-portal/admin/") {
		if method == http.MethodGet {
			return "customers.read"
		}
		return "customers.write"
	}
	if strings.HasPrefix(path, "/api/customer-processing/portal") {
		if method == http.MethodGet {
			return "customer_processing.read"
		}
		return "customer_processing.submit"
	}
	if strings.HasPrefix(path, "/api/customer-fulfillment") {
		if method == http.MethodGet {
			return "stock.read"
		}
		return "stock.write"
	}
	if strings.HasPrefix(path, "/api/message-center/rules") {
		return "settings.write"
	}
	if strings.HasPrefix(path, "/api/message-center") {
		return "orders.read"
	}
	if strings.HasPrefix(path, "/api/order/form") {
		return "orders.write"
	}
	if path == "/api/share-resources" {
		return "orders.write"
	}
	if strings.HasPrefix(path, "/api/contracts") {
		if method == http.MethodGet {
			return "orders.read"
		}
		return "orders.write"
	}
	if strings.HasPrefix(path, "/api/order") {
		if method == http.MethodGet {
			return "orders.read"
		}
		return "orders.write"
	}
	if strings.HasPrefix(path, "/api/orders") {
		if method == http.MethodGet {
			return "orders.read"
		}
		return "orders.write"
	}
	if strings.HasPrefix(path, "/api/stock/") {
		if method == http.MethodGet {
			return "stock.read"
		}
		return "stock.write"
	}
	if strings.HasPrefix(path, "/api/purchase/") {
		if method == http.MethodGet {
			return "purchase.read"
		}
		return "purchase.write"
	}
	if strings.HasPrefix(path, "/api/materials") {
		if method == http.MethodGet {
			return "materials.read"
		}
		return "materials.write"
	}
	if strings.HasPrefix(path, "/api/bom") {
		if method == http.MethodGet {
			return "bom.read"
		}
		return "bom.write"
	}
	if isBeanListPublicationPDFRequest(path) {
		return "costing.read"
	}
	if strings.HasPrefix(path, "/api/costing/bean-list/publications") {
		if method == http.MethodGet {
			return "costing.read"
		}
		return "auth.manage"
	}
	if strings.HasPrefix(path, "/api/costing/bean-list/drafts") {
		return "costing.read"
	}
	if strings.HasPrefix(path, "/api/costing/") {
		if method == http.MethodGet {
			return "costing.read"
		}
		return "costing.write"
	}
	if strings.HasPrefix(path, "/api/finance/employees") {
		return "finance.write"
	}
	if strings.HasPrefix(path, "/api/finance/settings/closing-mode") {
		return "finance.close_mode.manage"
	}
	if strings.HasPrefix(path, "/api/finance/reports/") && strings.Contains(path, "/close") {
		return "finance.close"
	}
	if strings.HasPrefix(path, "/api/finance/adjustments") {
		return "finance.close"
	}
	if strings.HasPrefix(path, "/api/finance/") {
		if method == http.MethodGet {
			return "finance.read"
		}
		return "finance.write"
	}
	if strings.HasPrefix(path, "/api/produce/allocations") {
		return "stock.read"
	}
	if strings.HasPrefix(path, "/api/produce/machines") {
		if method == http.MethodGet {
			return "production.read"
		}
		return "settings.write"
	}
	if strings.HasPrefix(path, "/api/produce/running/finish") ||
		strings.HasPrefix(path, "/api/produce/running/cancel") ||
		strings.HasPrefix(path, "/api/produce/start") {
		return "production.run"
	}
	if strings.HasPrefix(path, "/api/produce/") {
		if method == http.MethodGet {
			return "production.read"
		}
		return "production.run"
	}
	return ""
}

func isBeanListPublicationPDFRequest(path string) bool {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/app")
	if !strings.HasPrefix(path, "/api/costing/bean-list/publications/") || !strings.HasSuffix(path, "/pdf") {
		return false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/costing/bean-list/publications/"), "/pdf")
	if idPart == "" || strings.Contains(idPart, "/") {
		return false
	}
	id, err := strconv.ParseInt(idPart, 10, 64)
	return err == nil && id > 0
}
