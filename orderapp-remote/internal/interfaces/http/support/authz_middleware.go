package support

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func AuthorizationMiddleware(authz AuthzService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
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
	if path == "/api/company/profile" {
		return "settings.write"
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
	if strings.HasPrefix(path, "/api/order/form") {
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
