package finance

import (
	"fmt"
	"net/http"
	appfinance "orderapp/internal/application/finance"
	financeexcel "orderapp/internal/infrastructure/excel"
	financepdf "orderapp/internal/infrastructure/pdf"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func registerFinanceAPI(e *echo.Echo, svc Service) {
	e.GET("/api/finance/settings", func(c echo.Context) error {
		settings, err := svc.Settings(c.Request().Context(), actorFromRequest(c))
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, err)
		}
		return c.JSON(http.StatusOK, settings)
	})

	e.POST("/api/finance/settings", func(c echo.Context) error {
		var req appfinance.SettingsSnapshot
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		settings, err := svc.SaveSettings(c.Request().Context(), req, actorFromRequest(c))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, settings)
	})

	e.POST("/api/finance/settings/closing-mode", func(c echo.Context) error {
		var req struct {
			Mode string `json:"mode"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		settings, err := svc.SwitchClosingMode(c.Request().Context(), appfinance.SwitchClosingModeCommand{Mode: req.Mode, Actor: actorFromRequest(c)})
		if err != nil {
			return jsonError(c, http.StatusForbidden, err)
		}
		return c.JSON(http.StatusOK, settings)
	})

	e.GET("/api/finance/dashboard", func(c echo.Context) error {
		resp, err := svc.Dashboard(c.Request().Context(), monthParam(c))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/finance/employees", func(c echo.Context) error {
		rows, err := svc.ListExpenseEmployees(c.Request().Context())
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, err)
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/finance/expenses", func(c echo.Context) error {
		filter, err := expenseFilterFromRequest(c)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		rows, err := svc.ListExpenses(c.Request().Context(), filter)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/finance/expenses", func(c echo.Context) error {
		var req appfinance.CreateExpenseCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = actorFromRequest(c)
		row, err := svc.CreateExpense(c.Request().Context(), req)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/finance/tax-ledger", func(c echo.Context) error {
		rows, err := svc.ListTaxLedger(c.Request().Context(), monthParam(c))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/finance/tax-ledger", func(c echo.Context) error {
		var req appfinance.CreateTaxLedgerCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = actorFromRequest(c)
		row, err := svc.CreateTaxLedgerEntry(c.Request().Context(), req)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/finance/adjustments", func(c echo.Context) error {
		var req appfinance.CreateAdjustmentCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = actorFromRequest(c)
		row, err := svc.CreateAdjustment(c.Request().Context(), req)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/finance/reports/:month/pdf", func(c echo.Context) error {
		report, err := svc.DraftReport(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		b, err := financepdf.RenderFinanceMonthlyReport(report)
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, err)
		}
		filename := fmt.Sprintf("KFerp-finance-report-%s.pdf", report.Month)
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return c.Blob(http.StatusOK, "application/pdf", b)
	})

	e.GET("/api/finance/reports/:month/excel", func(c echo.Context) error {
		report, err := svc.DraftReport(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		b, err := financeexcel.RenderFinanceMonthlyReport(report)
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, err)
		}
		filename := fmt.Sprintf("KFerp-finance-report-%s.xlsx", report.Month)
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
	})

	e.GET("/api/finance/reports/:month/closing-review", func(c echo.Context) error {
		review, err := svc.ClosingReview(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, review)
	})

	e.GET("/api/finance/reports/:month/drilldown", func(c echo.Context) error {
		drilldown, err := svc.ReportDrilldown(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, drilldown)
	})

	e.GET("/api/finance/reports/:month/accountant-handoff.xlsx", func(c echo.Context) error {
		handoff, err := svc.AccountantHandoff(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		b, err := financeexcel.RenderFinanceAccountantHandoff(handoff)
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, err)
		}
		filename := fmt.Sprintf("KFerp-accountant-handoff-%s.xlsx", handoff.Month)
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
	})

	e.GET("/api/finance/reports/:month", func(c echo.Context) error {
		report, err := svc.DraftReport(c.Request().Context(), c.Param("month"))
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, report)
	})

	e.POST("/api/finance/reports/:month/close", func(c echo.Context) error {
		report, err := svc.CloseMonth(c.Request().Context(), appfinance.CloseMonthCommand{Month: c.Param("month"), Actor: actorFromRequest(c)})
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, report)
	})
}

func actorFromRequest(c echo.Context) string {
	if actor := strings.TrimSpace(c.Request().Header.Get("X-Actor")); actor != "" {
		return actor
	}
	return support.ActorOf(c)
}

func monthParam(c echo.Context) string {
	if month := strings.TrimSpace(c.QueryParam("month")); month != "" {
		return month
	}
	return time.Now().Format("2006-01")
}

func expenseFilterFromRequest(c echo.Context) (appfinance.ExpenseFilter, error) {
	filter := appfinance.ExpenseFilter{Month: monthParam(c)}
	if value := strings.TrimSpace(c.QueryParam("employee_id")); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 0 {
			return appfinance.ExpenseFilter{}, fmt.Errorf("invalid employee_id")
		}
		filter.EmployeeID = id
	}
	if value := strings.TrimSpace(c.QueryParam("customer_id")); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 0 {
			return appfinance.ExpenseFilter{}, fmt.Errorf("invalid customer_id")
		}
		filter.CustomerID = id
	}
	return filter, nil
}

func jsonError(c echo.Context, status int, err error) error {
	return c.JSON(status, map[string]string{"error": err.Error()})
}
