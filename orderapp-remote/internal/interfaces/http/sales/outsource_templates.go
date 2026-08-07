package sales

import (
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type OutsourceTemplateTier struct {
	ID         int64
	TemplateID int64
	MinQty     int64
	MaxQty     *int64
	Multiplier float64
}

func registerOutsourceSettingsRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	e.GET("/api/outsource/templates", func(c echo.Context) error {
		rows, err := salesSvc.ListOutsourceTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.POST("/api/outsource/templates", func(c echo.Context) error {
		var req salesapp.SaveOutsourceTemplateCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		if err := salesSvc.SaveOutsourceTemplate(c.Request().Context(), req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/finance/customer-processing-billing/options", func(c echo.Context) error {
		options, err := salesSvc.GetProcessingBillingOptions(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "options": options})
	})

	e.GET("/api/finance/customer-processing-billing/candidates", func(c echo.Context) error {
		customerID, err := strconv.ParseInt(strings.TrimSpace(c.QueryParam("customer_id")), 10, 64)
		if err != nil || customerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "customer_id required"})
		}
		rows, err := salesSvc.ListProcessingBillingCandidates(c.Request().Context(), customerID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.GET("/api/finance/customer-processing-billing/runs", func(c echo.Context) error {
		customerID, err := strconv.ParseInt(strings.TrimSpace(c.QueryParam("customer_id")), 10, 64)
		if err != nil || customerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "customer_id required"})
		}
		limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
		rows, err := salesSvc.ListProcessingBillingRuns(c.Request().Context(), salesapp.ProcessingBillingRunsQuery{CustomerID: customerID, Limit: limit})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.POST("/api/finance/customer-processing-billing/preview", func(c echo.Context) error {
		var req salesapp.PreviewProcessingBillingCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		preview, err := salesSvc.PreviewProcessingBilling(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "preview": preview})
	})

	e.POST("/api/finance/customer-processing-billing/confirm", func(c echo.Context) error {
		var req salesapp.ConfirmProcessingBillingCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		result, err := salesSvc.ConfirmProcessingBilling(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "result": result})
	})

	e.POST("/api/finance/customer-processing-billing/runs/:id/pay", func(c echo.Context) error {
		billingRunID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || billingRunID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "billing_run_id required"})
		}
		var req salesapp.PayProcessingBillingCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		req.BillingRunID = billingRunID
		req.Actor = support.ActorOf(c)
		result, err := salesSvc.PayProcessingBilling(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "result": result})
	})

	e.POST("/api/finance/customer-processing-billing/runs/:id/reverse", func(c echo.Context) error {
		billingRunID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || billingRunID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "billing_run_id required"})
		}
		var req salesapp.ReverseProcessingBillingCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		req.BillingRunID = billingRunID
		req.Actor = support.ActorOf(c)
		result, err := salesSvc.ReverseProcessingBilling(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "result": result})
	})

	e.POST("/api/finance/customer-processing-billing/runs/:id/adjustments", func(c echo.Context) error {
		billingRunID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || billingRunID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "billing_run_id required"})
		}
		var req salesapp.AdjustProcessingBillingCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		req.BillingRunID = billingRunID
		req.Actor = support.ActorOf(c)
		result, err := salesSvc.AdjustProcessingBilling(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "result": result})
	})

	e.GET("/settings/outsource", func(c echo.Context) error {
		target := "/vue-shell?view=outsourceSettings"
		if raw := strings.TrimSpace(c.QueryString()); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})

}
