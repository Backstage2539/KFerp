package customerportal

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	customerapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

func miniEmployeeOrderCards(c echo.Context, employee customerapp.CurrentContext, sales EmployeeSales) error {
	query := salesapp.EmployeeOrderCardsQuery{Q: strings.TrimSpace(c.QueryParam("q")), Limit: 4}
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 50 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "每页订单数须为 1 至 50"})
		}
		query.Limit = n
	}
	if raw := c.QueryParam("cursor"); raw != "" {
		date, idText, ok := strings.Cut(raw, "~")
		id, err := strconv.ParseInt(idText, 10, 64)
		if !ok || err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单分页位置无效"})
		}
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单分页日期无效"})
			}
		}
		query.After = &salesapp.EmployeeOrderCursor{Date: date, ID: id}
	}
	if !containsMiniRole(employee.Roles, "admin") {
		query.Scope = "mine"
		query.EmployeeID = employee.EmployeeID
	}
	reader, ok := sales.(salesapp.EmployeeOrderCardReader)
	if !ok {
		return miniInternalError(c)
	}
	page, err := reader.ListEmployeeOrderCards(c.Request().Context(), query)
	if err != nil {
		return miniInternalError(c)
	}
	next := ""
	if page.Next != nil {
		next = page.Next.Date + "~" + strconv.FormatInt(page.Next.ID, 10)
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": page.Rows, "has_next": page.Next != nil, "next_cursor": next})
}
