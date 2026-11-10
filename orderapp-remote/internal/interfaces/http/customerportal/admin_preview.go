package customerportal

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	customer "orderapp/internal/application/customer"
	portal "orderapp/internal/application/customerportal"
)

type previewMe interface {
	Me(context.Context, string) (portal.CurrentContext, error)
}
type previewOperations interface {
	StartAdminPreview(context.Context, string, int64) (portal.AdminPreviewSession, error)
	EndAdminPreview(context.Context, string) error
}

// Fail closed globally for every bearer preview token, including unknown routes
// and non-mini APIs. Only reviewed, customer-scoped reads are permitted.
func adminPreviewReadOnlyMiddleware(svc previewMe) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
			if !portal.IsAdminPreviewToken(token) {
				return next(c)
			}
			if svc == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "只读预览已失效"})
			}
			current, err := svc.Me(c.Request().Context(), token)
			if err != nil || current.AccountType != "preview" || current.CurrentCustomerID <= 0 {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "只读预览已失效"})
			}
			if !previewRouteAllowed(c) {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "只读预览不支持此操作"})
			}
			return next(c)
		}
	}
}

func previewRouteAllowed(c echo.Context) bool {
	method, path := c.Request().Method, c.Path()
	if method == http.MethodPost {
		return path == "/api/mini/customer-preview/exit"
	}
	if method != http.MethodGet {
		return false
	}
	switch path {
	case "/api/mini/me", "/api/mini/order-price-tables/preview", "/api/mini/customer-issued-bills", "/api/mini/customer-inventory", "/api/mini/customer-inventory/assets", "/api/mini/processing-requests":
		return true
	case "/api/mini/services/:key":
		return c.Param("key") == "orders" || c.Param("key") == "inventory" || c.Param("key") == "settlement" || c.Param("key") == "shipping"
	case "/api/mini/customer-issued-bills/:id":
		v := c.Param("id")
		for _, ext := range []string{".pdf", ".xlsx"} {
			v = strings.TrimSuffix(v, ext)
		}
		n, err := strconv.ParseInt(v, 10, 64)
		return err == nil && n > 0
	case "/api/mini/customer-inventory/:product_id/batches":
		n, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		return err == nil && n > 0
	case "/api/mini/processing-requests/:id":
		n, err := strconv.ParseInt(c.Param("id"), 10, 64)
		return err == nil && n > 0
	case "/api/mini/customer-inventory/assets/:inventory_type/:item_id/ledger":
		n, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
		return err == nil && n > 0
	}
	return false
}

func registerAdminPreviewAPI(e *echo.Echo, svc Service, customers CustomerMaintenance) {
	e.Use(adminPreviewReadOnlyMiddleware(svc))
	e.GET("/api/mini/employee/customer-preview/customers", func(c echo.Context) error {
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		current, err := svc.Me(c.Request().Context(), token)
		if err != nil || current.AccountType != "employee" || !containsMiniRole(current.Roles, "admin") {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "仅管理员可查看"})
		}
		if customers == nil {
			return miniInternalError(c)
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}
		page, _ := strconv.Atoi(c.QueryParam("page"))
		if page <= 0 {
			page = 1
		}
		active := true
		rows, err := customers.ListManaged(c.Request().Context(), miniCustomerPrincipal(current), customer.ListQuery{Query: strings.TrimSpace(c.QueryParam("q")), Limit: limit, Offset: (page - 1) * limit, Active: &active})
		if err != nil {
			return miniCustomerError(c, err)
		}
		type item struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		result := make([]item, 0, len(rows.Rows))
		for _, row := range rows.Rows {
			result = append(result, item{ID: row.ID, Name: row.Name})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": result, "has_next": rows.HasNext})
	})
	e.POST("/api/mini/employee/customer-preview", func(c echo.Context) error {
		ops, ok := svc.(previewOperations)
		if !ok {
			return miniInternalError(c)
		}
		var req struct {
			CustomerID int64 `json:"customer_id"`
		}
		if err := c.Bind(&req); err != nil || req.CustomerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择客户"})
		}
		issued, err := ops.StartAdminPreview(c.Request().Context(), miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization)), req.CustomerID)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "无法进入客户只读预览"})
		}
		return c.JSON(http.StatusOK, issued)
	})
	e.POST("/api/mini/customer-preview/exit", func(c echo.Context) error {
		ops, ok := svc.(previewOperations)
		if !ok {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if !portal.IsAdminPreviewToken(token) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "预览凭证无效"})
		}
		if err := ops.EndAdminPreview(c.Request().Context(), token); err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "退出预览失败"})
		}
		return c.NoContent(http.StatusNoContent)
	})
}
