package appmain

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type orderAPIHandler struct {
	pool   *pgxpool.Pool
	schema string
	sales  *salesapp.Service
}

type apiOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type orderFormAPIResponse struct {
	Today        string      `json:"today"`
	Customers    []apiOption `json:"customers"`
	Sources      []apiOption `json:"sources"`
	ShipStatuses []apiOption `json:"ship_statuses"`
	PayStatuses  []apiOption `json:"pay_statuses"`
	OrderTypes   []apiOption `json:"order_types"`
	Products     []jsProduct `json:"products"`
	EditMode     bool        `json:"edit_mode"`
	EditID       int64       `json:"edit_id"`
	EditData     any         `json:"edit_data,omitempty"`
}

type orderSaveAPIRequest struct {
	EditID                int64  `json:"edit_id"`
	OrderDate             string `json:"order_date"`
	CustomerID            int64  `json:"customer_id"`
	SourceID              int64  `json:"source_id"`
	OrderTypeID           int64  `json:"order_type_id"`
	PayStatusID           int64  `json:"pay_status_id"`
	ShipStatusID          int64  `json:"ship_status_id"`
	ShipMethod            string `json:"ship_method"`
	ShipTrackingNo        string `json:"ship_tracking_no"`
	Notes                 string `json:"notes"`
	ShippingAmount        string `json:"shipping_amount"`
	DiscountAmount        string `json:"discount_amount"`
	RoundToInt            string `json:"round_to_int"`
	ExpressFee            string `json:"express_fee"`
	OutsourceMaterialFee  string `json:"outsource_material_fee"`
	OutsourceRoastFee     string `json:"outsource_roast_fee"`
	OutsourcePackagingFee string `json:"outsource_packaging_fee"`
	OutsourceManualFee    string `json:"outsource_manual_fee"`
	OutsourceTaxFee       string `json:"outsource_tax_fee"`
	OutsourceOtherFee     string `json:"outsource_other_fee"`

	ProductID []string `json:"product_id"`
	TierID    []string `json:"tier_id"`
	UnitPrice []string `json:"unit_price"`
	ItemName  []string `json:"item_name"`
	Qty       []string `json:"qty"`
	Unit      []string `json:"unit"`
	Spec      []string `json:"spec"`
}

func registerOrderAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := orderAPIHandler{
		pool:   pool,
		schema: schema,
		sales:  salesapp.NewService(postgresSalesRepository{pool: pool, schema: schema}),
	}
	e.GET("/api/orders", h.list)
	e.GET("/api/order/form", h.form)
	e.POST("/api/order", h.save)
}

func (h orderAPIHandler) list(c echo.Context) error {
	query := ordersQueryFromContext(c)
	rows, hasNext, err := fetchOrders(c.Request().Context(), h.pool, h.schema, query.Q, query.From, query.To, query.Void, query.CustomerID, query.PayStatusID, query.ShipStatusID, query.ProcessStatusID, query.UnproducedOnly, query.CompletedOnly, query.Limit, query.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	summary, _ := fetchOrdersSummary(c.Request().Context(), h.pool, h.schema, query.Q, query.From, query.To, query.Void, query.CustomerID, query.PayStatusID, query.ShipStatusID, query.ProcessStatusID, query.UnproducedOnly, query.CompletedOnly)
	orderTypes, _ := fetchOptions(c.Request().Context(), h.pool, "SELECT id, name FROM "+h.schema+".order_types ORDER BY id")
	payStatuses, _ := fetchOptions(c.Request().Context(), h.pool, "SELECT id, name FROM "+h.schema+".pay_statuses ORDER BY id")
	shipStatuses, _ := fetchOptions(c.Request().Context(), h.pool, "SELECT id, name FROM "+h.schema+".ship_statuses ORDER BY id")
	processStatuses, _ := fetchOptions(c.Request().Context(), h.pool, "SELECT id, name FROM "+h.schema+".order_process_statuses WHERE active=true ORDER BY sort,id")

	return c.JSON(http.StatusOK, map[string]any{
		"rows":             rows,
		"summary":          summary,
		"order_types":      apiOptions(orderTypes),
		"pay_statuses":     apiOptions(payStatuses),
		"ship_statuses":    apiOptions(shipStatuses),
		"process_statuses": apiOptions(processStatuses),
		"page":             query.Page,
		"limit":            query.Limit,
		"offset":           query.Offset,
		"has_prev":         query.Offset > 0,
		"has_next":         hasNext,
	})
}

func (h orderAPIHandler) form(c echo.Context) error {
	data := PageData{Today: time.Now().Format("2006-01-02")}
	if err := loadOptions(c.Request().Context(), h.pool, h.schema, &data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	resp := orderFormAPIResponse{
		Today:        data.Today,
		Customers:    apiOptions(data.Customers),
		Sources:      apiOptions(data.Sources),
		ShipStatuses: apiOptions(data.ShipStatuses),
		PayStatuses:  apiOptions(data.PayStatuses),
		OrderTypes:   apiOptions(data.OrderTypes),
		Products:     apiProducts(data.Products),
	}

	if v := strings.TrimSpace(c.QueryParam("edit_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid edit_id"})
		}
		ed, err := fetchOrderEdit(c.Request().Context(), h.pool, h.schema, id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if ed == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
		}
		resp.EditMode = true
		resp.EditID = id
		resp.Today = ed.OrderDate
		resp.EditData = editDataForAPI(ed)
	}

	return c.JSON(http.StatusOK, resp)
}

type ordersAPIQuery struct {
	Q               string
	From            string
	To              string
	Void            string
	CustomerID      int64
	PayStatusID     int64
	ShipStatusID    int64
	ProcessStatusID int64
	UnproducedOnly  bool
	CompletedOnly   bool
	Limit           int
	Offset          int
	Page            int
}

func ordersQueryFromContext(c echo.Context) ordersAPIQuery {
	q := ordersAPIQuery{
		Q:     strings.TrimSpace(c.QueryParam("q")),
		From:  strings.TrimSpace(c.QueryParam("from")),
		To:    strings.TrimSpace(c.QueryParam("to")),
		Void:  strings.TrimSpace(c.QueryParam("void")),
		Limit: intParam(c, "limit", 10),
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	q.Offset = intParam(c, "offset", 0)
	if q.Offset < 0 {
		q.Offset = 0
	}
	if page := intParam(c, "page", 0); page > 0 {
		q.Offset = (page - 1) * q.Limit
	}
	if q.Limit > 0 {
		q.Page = (q.Offset / q.Limit) + 1
	} else {
		q.Page = 1
	}
	q.CustomerID = int64(intParam(c, "customer_id", 0))
	q.PayStatusID = int64(intParam(c, "pay_status_id", 0))
	q.ShipStatusID = int64(intParam(c, "ship_status_id", 0))
	q.ProcessStatusID = int64(intParam(c, "process_status_id", 0))
	q.UnproducedOnly = strings.TrimSpace(c.QueryParam("preset")) == "unprod"
	q.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
	if q.Void == "" {
		q.Void = "normal"
	}
	return q
}

func (h orderAPIHandler) save(c echo.Context) error {
	if err := requireEmployeeBound(c); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req orderSaveAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), req.EditID, actorOf(c))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	res, err := h.sales.SaveOrder(c.Request().Context(), cmd)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	redirectURL := "/order?ok=1&order_no=" + res.OrderNo
	if res.Edited {
		redirectURL = "/orders/" + strconv.FormatInt(res.OrderID, 10)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"order_id":     res.OrderID,
		"order_no":     res.OrderNo,
		"edited":       res.Edited,
		"redirect_url": redirectURL,
	})
}

func (r orderSaveAPIRequest) toCreateRequest() CreateOrderRequest {
	return CreateOrderRequest{
		OrderDate:             r.OrderDate,
		CustomerID:            r.CustomerID,
		SourceID:              r.SourceID,
		OrderTypeID:           r.OrderTypeID,
		PayStatusID:           r.PayStatusID,
		ShipStatusID:          r.ShipStatusID,
		ShipMethod:            r.ShipMethod,
		ShipTrackingNo:        r.ShipTrackingNo,
		Notes:                 r.Notes,
		ShippingAmount:        r.ShippingAmount,
		DiscountAmount:        r.DiscountAmount,
		RoundToInt:            r.RoundToInt,
		ExpressFee:            r.ExpressFee,
		OutsourceMaterialFee:  r.OutsourceMaterialFee,
		OutsourceRoastFee:     r.OutsourceRoastFee,
		OutsourcePackagingFee: r.OutsourcePackagingFee,
		OutsourceManualFee:    r.OutsourceManualFee,
		OutsourceTaxFee:       r.OutsourceTaxFee,
		OutsourceOtherFee:     r.OutsourceOtherFee,
		ProductID:             r.ProductID,
		TierID:                r.TierID,
		UnitPrice:             r.UnitPrice,
		ItemName:              r.ItemName,
		Qty:                   r.Qty,
		Unit:                  r.Unit,
		Spec:                  r.Spec,
	}
}

func apiOptions(in []Option) []apiOption {
	out := make([]apiOption, 0, len(in))
	for _, item := range in {
		out = append(out, apiOption{ID: item.ID, Name: item.Name})
	}
	return out
}

func apiProducts(ps []ProductOption) []jsProduct {
	out := make([]jsProduct, 0, len(ps))
	for _, p := range ps {
		jp := jsProduct{
			ID:              p.ID,
			Name:            p.Name,
			Py:              pinyinFull(p.Name),
			Pyi:             pinyinInitials(p.Name),
			RetailPrice100G: p.RetailPrice100G,
			RetailPrice200G: p.RetailPrice200G,
			RetailPrice227G: p.RetailPrice227G,
			RetailPrice250G: p.RetailPrice250G,
			RetailSpecs:     p.RetailSpecs,
		}
		for _, t := range p.Tiers {
			jp.Tiers = append(jp.Tiers, jsTier{ID: t.ID, SpecG: t.SpecG, Min: t.MinQty, Max: t.MaxQty, UnitPrice: t.UnitPrice})
		}
		out = append(out, jp)
	}
	return out
}

func editDataForAPI(ed *OrderEditData) map[string]any {
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
		tierID := "auto"
		if it.PriceTierID > 0 {
			tierID = strconv.FormatInt(it.PriceTierID, 10)
		}
		items = append(items, editItem{
			ProductID:   it.ProductID,
			ProductName: it.Product,
			TierID:      tierID,
			UnitPrice:   it.UnitPrice,
			Qty:         it.Qty,
			Unit:        it.Unit,
			Spec:        spec,
		})
	}
	return map[string]any{
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
}
