package sales

import (
	"fmt"
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

func registerOrderRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := orderHandler{
		sales: salesSvc,
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

	// Void is irreversible. Operators can copy a voided order into a new order instead.
	e.POST("/orders/:id/void", h.void)

	// Create order
	e.GET("/order", h.entry)
}

type orderHandler struct {
	sales *salesapp.Service
}

func (h orderHandler) index(c echo.Context) error {
	return support.VueShellRedirect(c, "orders")
}

func (h orderHandler) inlineUpdate(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req support.InlineUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	ctx := c.Request().Context()
	if err := h.sales.InlineUpdate(ctx, id, support.ActorOf(c), inlineUpdateCommandFromRequest(req)); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)

}

func (h orderHandler) audit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	rows, err := h.sales.ListOrderAuditLogs(c.Request().Context(), id, 50)
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
	return c.Redirect(http.StatusSeeOther, support.PrefixRelativeLocation(c, fmt.Sprintf("/orders/%d/edit", id)))

}

func (h orderHandler) editRedirect(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return c.Redirect(http.StatusSeeOther, support.PrefixRelativeLocation(c, fmt.Sprintf("/order?edit_id=%d", id)))

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
	if err := h.sales.UpdateHeader(ctx, id, updateHeaderCommandFromRequest(req, support.ActorOf(c))); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, support.PrefixRelativeLocation(c, fmt.Sprintf("/orders/%d", id)))

}

func (h orderHandler) void(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	reason := strings.TrimSpace(c.FormValue("reason"))
	if err := h.sales.Void(c.Request().Context(), id, support.ActorOf(c), reason); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, support.PrefixRelativeLocation(c, fmt.Sprintf("/orders/%d", id)))

}

func (h orderHandler) entry(c echo.Context) error {
	target := "/vue-shell?view=order"
	if raw := c.QueryString(); raw != "" {
		target += "&" + raw
	}
	return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))

}
