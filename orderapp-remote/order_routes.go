package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	if err := h.sales.UpdateHeader(ctx, id, updateHeaderCommandFromRequest(req)); err != nil {
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
	data := PageData{Today: time.Now().Format("2006-01-02")}
	if c.QueryParam("ok") == "1" {
		data.Ok = true
		data.OrderNo = c.QueryParam("order_no")
	}
	if err := loadOptions(c.Request().Context(), h.pool, h.schema, &data); err != nil {
		data.Error = err.Error()
	}
	if v := strings.TrimSpace(c.QueryParam("edit_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			ed, ferr := fetchOrderEdit(c.Request().Context(), h.pool, h.schema, id)
			if ferr != nil {
				data.Error = ferr.Error()
			} else if ed == nil {
				data.Error = "order not found"
			} else {
				data.EditMode = true
				data.EditID = id
				if ed.OrderDate != "" {
					data.Today = ed.OrderDate
				}
				type editItem struct {
					ProductID   int64  `json:"product_id"`
					ProductName string `json:"product_name"`
					TierID      string `json:"tier_id"`
					UnitPrice   string `json:"unit_price"`
					Qty         string `json:"qty"`
					Unit        string `json:"unit"`
					Spec        string `json:"spec"`
				}
				items := make([]editItem, 0, len(ed.Items))
				for _, it := range ed.Items {
					spec := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(it.Spec)), "g")
					items = append(items, editItem{
						ProductID:   it.ProductID,
						ProductName: it.Product,
						TierID:      "auto",
						UnitPrice:   it.UnitPrice,
						Qty:         it.Qty,
						Unit:        it.Unit,
						Spec:        spec,
					})
				}
				payload := map[string]any{
					"order_date":              ed.OrderDate,
					"customer_id":             strconv.FormatInt(ed.CustomerID, 10),
					"source_id":               strconv.FormatInt(ed.SourceID, 10),
					"order_type_id":           strconv.FormatInt(ed.OrderTypeID, 10),
					"pay_status_id":           strconv.FormatInt(ed.PayStatusID, 10),
					"ship_status_id":          strconv.FormatInt(ed.ShipStatusID, 10),
					"ship_method":             ed.ShipMethod,
					"ship_tracking_no":        ed.ShipTrackingNo,
					"notes":                   ed.Notes,
					"shipping_amount":         ed.ShippingAmount,
					"discount_amount":         ed.DiscountAmount,
					"round_to_int":            ed.RoundToInt,
					"express_fee":             ed.ExpressFee,
					"outsource_material_fee":  ed.OutsourceMaterialFee,
					"outsource_roast_fee":     ed.OutsourceRoastFee,
					"outsource_packaging_fee": ed.OutsourcePackagingFee,
					"outsource_manual_fee":    ed.OutsourceManualFee,
					"outsource_tax_fee":       ed.OutsourceTaxFee,
					"outsource_other_fee":     ed.OutsourceOtherFee,
					"items":                   items,
				}
				if b, err := json.Marshal(payload); err == nil {
					data.EditDataJSON = template.JS(string(b))
				}
			}
		}
	}
	data.ProductsJSON = buildProductsJSON(data.Products)
	return c.Render(http.StatusOK, "order.html", data)

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

	res, err := h.sales.SaveOrder(c.Request().Context(), saveOrderCommandFromCreateRequest(req, editID))
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if res.Edited {
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", res.OrderID))
	}
	return c.Redirect(http.StatusSeeOther, "/order?ok=1&order_no="+url.QueryEscape(res.OrderNo))
}
