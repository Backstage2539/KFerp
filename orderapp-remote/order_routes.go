package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerOrderRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := orderHandler{
		pool:   pool,
		schema: schema,
		sales:  salesapp.NewService(postgresSalesRepository{pool: pool, schema: schema}),
	}

	// Orders list
	e.GET("/orders", h.index)

	e.POST("/orders/:id/inline", h.inlineUpdate)

	e.GET("/orders/:id/audit", h.audit)

	// Merged detail+edit: clicking order number goes to unified edit page.
	e.GET("/orders/:id", h.detailRedirect)

	// Unified edit: reuse /order page logic.
	e.GET("/orders/:id/edit", h.editRedirect)
	e.POST("/orders/:id/edit", h.editPost)

	// Void / unvoid
	e.POST("/orders/:id/void", h.void)
	e.POST("/orders/:id/unvoid", h.unvoid)

	// Create order
	e.GET("/order", h.entry)

	e.POST("/order", h.create)

}

type orderHandler struct {
	pool   *pgxpool.Pool
	schema string
	sales  *salesapp.Service
}

func (h orderHandler) index(c echo.Context) error {
	if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
		return vueShellRedirect(c, "orders")
	}

	data := OrdersPageData{
		Q:      strings.TrimSpace(c.QueryParam("q")),
		From:   strings.TrimSpace(c.QueryParam("from")),
		To:     strings.TrimSpace(c.QueryParam("to")),
		Preset: strings.TrimSpace(c.QueryParam("preset")),
		Void:   strings.TrimSpace(c.QueryParam("void")),
		Limit:  10,
		Offset: 0,
	}
	if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.CustomerID = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("pay_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.PayStatusFilter = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("ship_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.ShipStatusFilter = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("process_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.ProcStatusFilter = n
		}
	}
	data.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
	if data.Preset == "unprod" {
		data.UnproducedOnly = true
	}
	if data.Void == "" {
		data.Void = "normal"
	}
	if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			data.Limit = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			data.Offset = n
		}
	}
	// page is 1-indexed; overrides offset when provided
	if v := strings.TrimSpace(c.QueryParam("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			data.Offset = (n - 1) * data.Limit
		}
	}

	if data.Limit > 0 {
		data.Page = (data.Offset / data.Limit) + 1
	} else {
		data.Page = 1
	}

	rows, hasNext, errOrders := fetchOrders(c.Request().Context(), h.pool, h.schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly, data.Limit, data.Offset)
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", h.schema)); err == nil {
		data.OrderTypeOpts = opts
	} else {
		data.OrderTypeOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", h.schema)); err == nil {
		data.PayOpts = opts
	} else {
		data.PayOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", h.schema)); err == nil {
		data.ShipOpts = opts
	} else {
		data.ShipOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_process_statuses WHERE active=true ORDER BY sort,id", h.schema)); err == nil {
		data.ProcessOpts = opts
	} else {
		data.ProcessOpts = nil
	}
	// summary (best effort)
	if s, err := fetchOrdersSummary(c.Request().Context(), h.pool, h.schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly); err == nil {
		data.Summary = s
	}

	if errOrders != nil {
		data.Error = errOrders.Error()
		return c.Render(http.StatusOK, "orders.html", data)
	}
	data.Rows = rows
	data.HasPrev = data.Offset > 0
	data.HasNext = hasNext
	if data.Limit > 0 {
		data.Page = (data.Offset / data.Limit) + 1
	} else {
		data.Page = 1
	}
	return c.Render(http.StatusOK, "orders.html", data)

}

func (h orderHandler) inlineUpdate(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req InlineUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	ctx := c.Request().Context()
	if err := h.sales.InlineUpdate(ctx, id, actorOf(c), inlineUpdateCommandFromRequest(req)); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)

}

func (h orderHandler) audit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	rows, err := fetchAuditLogs(c.Request().Context(), h.pool, h.schema, id, 50)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, rows)

}

func (h orderHandler) detailRedirect(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d/edit", id))

}

func (h orderHandler) editRedirect(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/order?edit_id=%d", id))

}

func (h orderHandler) editPost(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req UpdateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	ctx := c.Request().Context()
	if err := h.sales.UpdateHeader(ctx, id, updateHeaderCommandFromRequest(req, actorOf(c))); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) void(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	reason := strings.TrimSpace(c.FormValue("reason"))
	if err := h.sales.Void(c.Request().Context(), id, actorOf(c), reason); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) unvoid(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	if err := h.sales.Unvoid(c.Request().Context(), id, actorOf(c)); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) entry(c echo.Context) error {
	target := "/vue-shell?view=order"
	if raw := c.QueryString(); raw != "" {
		target += "&" + raw
	}
	return c.Redirect(http.StatusFound, target)

}

func (h orderHandler) create(c echo.Context) error {
	if err := requireEmployeeBound(c); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}

	editID := int64(0)
	if v := strings.TrimSpace(c.FormValue("edit_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			editID = n
		}
	}

	cmd, err := saveOrderCommandFromCreateRequest(req, editID, actorOf(c))
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	res, err := h.sales.SaveOrder(c.Request().Context(), cmd)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if res.Edited {
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", res.OrderID))
	}
	return c.Redirect(http.StatusSeeOther, "/order?ok=1&order_no="+url.QueryEscape(res.OrderNo))
}
