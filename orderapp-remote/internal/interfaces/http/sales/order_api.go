package sales

import (
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type orderAPIHandler struct {
	sales *salesapp.Service
}

type apiOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type customerAPIOption struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Contact            string `json:"contact,omitempty"`
	Phone              string `json:"phone,omitempty"`
	Py                 string `json:"py"`
	Pyi                string `json:"pyi"`
	DefaultSourceID    int64  `json:"default_source_id,omitempty"`
	DefaultOrderTypeID int64  `json:"default_order_type_id,omitempty"`
}

type employeeAPIOption struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone,omitempty"`
	DepartmentID int64  `json:"department_id,omitempty"`
	Department   string `json:"department,omitempty"`
	Py           string `json:"py"`
	Pyi          string `json:"pyi"`
}

type orderFormAPIResponse struct {
	Today        string              `json:"today"`
	Customers    []customerAPIOption `json:"customers"`
	Employees    []employeeAPIOption `json:"employees"`
	Sources      []apiOption         `json:"sources"`
	ShipStatuses []apiOption         `json:"ship_statuses"`
	PayStatuses  []apiOption         `json:"pay_statuses"`
	OrderTypes   []apiOption         `json:"order_types"`
	Products     []jsProduct         `json:"products"`
	EditMode     bool                `json:"edit_mode"`
	EditID       int64               `json:"edit_id"`
	EditData     any                 `json:"edit_data,omitempty"`
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
	ResponsibleType       string `json:"responsible_type"`
	ResponsibleID         int64  `json:"responsible_id"`
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
	StockBatchDecision    string `json:"stock_batch_decision"`

	ProductID []string `json:"product_id"`
	TierID    []string `json:"tier_id"`
	UnitPrice []string `json:"unit_price"`
	ItemName  []string `json:"item_name"`
	Qty       []string `json:"qty"`
	Unit      []string `json:"unit"`
	Spec      []string `json:"spec"`
}

func registerOrderAPI(e *echo.Echo, salesSvc *salesapp.Service) {
	h := orderAPIHandler{
		sales: salesSvc,
	}
	e.GET("/api/orders", h.list)
	e.GET("/api/order/form", h.form)
	e.POST("/api/order/stock-batch-preview", h.stockBatchPreview)
	e.POST("/api/order", h.save)
}

func (h orderAPIHandler) list(c echo.Context) error {
	query := ordersQueryFromContext(c)
	result, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{
		Q:               query.Q,
		From:            query.From,
		To:              query.To,
		Void:            query.Void,
		CustomerID:      query.CustomerID,
		PayStatusID:     query.PayStatusID,
		ShipStatusID:    query.ShipStatusID,
		ProcessStatusID: query.ProcessStatusID,
		UnproducedOnly:  query.UnproducedOnly,
		CompletedOnly:   query.CompletedOnly,
		ShipReadyOnly:   query.ShipReadyOnly,
		Limit:           query.Limit,
		Offset:          query.Offset,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"rows":             result.Rows,
		"summary":          result.Summary,
		"order_types":      result.OrderTypes,
		"pay_statuses":     result.PayStatuses,
		"ship_statuses":    result.ShipStatuses,
		"process_statuses": result.ProcessStatuses,
		"page":             query.Page,
		"limit":            query.Limit,
		"offset":           query.Offset,
		"has_prev":         query.Offset > 0,
		"has_next":         result.HasNext,
	})
}

func (h orderAPIHandler) form(c echo.Context) error {
	editID := int64(0)
	if v := strings.TrimSpace(c.QueryParam("edit_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid edit_id"})
		}
		editID = id
	}

	data, err := h.sales.OrderForm(c.Request().Context(), editID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	customerID := int64(0)
	filterByCustomer := false
	if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid customer_id"})
		}
		customerID = id
		filterByCustomer = true
	}
	if filterByCustomer {
		data.Products = filterOrderProductsForCustomer(data.Products, customerID)
	}

	resp := orderFormAPIResponse{
		Today:        data.Today,
		Customers:    apiCustomerOptions(data.Customers),
		Employees:    apiEmployeeOptions(data.Employees),
		Sources:      apiOptions(data.Sources),
		ShipStatuses: apiOptions(data.ShipStatuses),
		PayStatuses:  apiOptions(data.PayStatuses),
		OrderTypes:   apiOptions(data.OrderTypes),
		Products:     apiProducts(data.Products),
	}

	if editID > 0 {
		if data.EditData == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
		}
		resp.EditMode = true
		resp.EditID = editID
		resp.Today = data.EditData.OrderDate
		resp.EditData = editDataForAPI(data.EditData)
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
	ShipReadyOnly   bool
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
		Limit: support.IntParam(c, "limit", 10),
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	q.Offset = support.IntParam(c, "offset", 0)
	if q.Offset < 0 {
		q.Offset = 0
	}
	if page := support.IntParam(c, "page", 0); page > 0 {
		q.Offset = (page - 1) * q.Limit
	}
	if q.Limit > 0 {
		q.Page = (q.Offset / q.Limit) + 1
	} else {
		q.Page = 1
	}
	q.CustomerID = int64(support.IntParam(c, "customer_id", 0))
	q.PayStatusID = int64(support.IntParam(c, "pay_status_id", 0))
	q.ShipStatusID = int64(support.IntParam(c, "ship_status_id", 0))
	q.ProcessStatusID = int64(support.IntParam(c, "process_status_id", 0))
	q.UnproducedOnly = strings.TrimSpace(c.QueryParam("preset")) == "unprod"
	q.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
	q.ShipReadyOnly = strings.TrimSpace(c.QueryParam("ship_ready")) == "1"
	if q.Void == "" {
		q.Void = "normal"
	}
	return q
}

func (h orderAPIHandler) save(c echo.Context) error {
	if err := support.RequireEmployeeBound(c); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req orderSaveAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), req.EditID, support.ActorOf(c))
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
	redirectURL = support.PrefixRelativeLocation(c, redirectURL)
	return c.JSON(http.StatusOK, map[string]any{
		"order_id":         res.OrderID,
		"order_no":         res.OrderNo,
		"edited":           res.Edited,
		"redirect_url":     redirectURL,
		"stock_batch_used": res.StockBatchUsed,
	})
}

func (h orderAPIHandler) stockBatchPreview(c echo.Context) error {
	var req orderSaveAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), req.EditID, support.ActorOf(c))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	preview, err := h.sales.PreviewOrderStockBatches(c.Request().Context(), salesapp.OrderStockBatchPreviewCommand{EditID: req.EditID, Items: cmd.Items})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
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
		ResponsibleType:       r.ResponsibleType,
		ResponsibleID:         r.ResponsibleID,
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
		StockBatchDecision:    r.StockBatchDecision,
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

func apiCustomerOptions(in []CustomerOption) []customerAPIOption {
	out := make([]customerAPIOption, 0, len(in))
	for _, item := range in {
		out = append(out, customerAPIOption{
			ID:                 item.ID,
			Name:               item.Name,
			Contact:            item.Contact,
			Phone:              item.Phone,
			Py:                 support.PinyinFull(item.Name),
			Pyi:                support.PinyinInitials(item.Name),
			DefaultSourceID:    item.DefaultSourceID,
			DefaultOrderTypeID: item.DefaultOrderTypeID,
		})
	}
	return out
}

func apiEmployeeOptions(in []EmployeeOption) []employeeAPIOption {
	out := make([]employeeAPIOption, 0, len(in))
	for _, item := range in {
		out = append(out, employeeAPIOption{
			ID:           item.ID,
			Name:         item.Name,
			Phone:        item.Phone,
			DepartmentID: item.DepartmentID,
			Department:   item.Department,
			Py:           support.PinyinFull(item.Name),
			Pyi:          support.PinyinInitials(item.Name),
		})
	}
	return out
}

func apiProducts(ps []ProductOption) []jsProduct {
	out := make([]jsProduct, 0, len(ps))
	for _, p := range ps {
		jp := jsProduct{
			ID:              p.ID,
			Name:            p.Name,
			Py:              support.PinyinFull(p.Name),
			Pyi:             support.PinyinInitials(p.Name),
			RetailPrice100G: p.RetailPrice100G,
			RetailPrice200G: p.RetailPrice200G,
			RetailPrice227G: p.RetailPrice227G,
			RetailPrice250G: p.RetailPrice250G,
			CustomerID:      p.CustomerID,
			BaseProductID:   p.BaseProductID,
			Visibility:      productVisibilityForAPI(p.Visibility, p.CustomerID),
			CustomType:      p.CustomType,
			RetailSpecs:     p.RetailSpecs,
		}
		for _, t := range p.Tiers {
			jp.Tiers = append(jp.Tiers, jsTier{ID: t.ID, SpecG: t.SpecG, Min: t.MinQty, Max: t.MaxQty, UnitPrice: t.UnitPrice})
		}
		out = append(out, jp)
	}
	return out
}

func filterOrderProductsForCustomer(products []ProductOption, customerID int64) []ProductOption {
	out := make([]ProductOption, 0, len(products))
	for _, product := range products {
		visibility := productVisibilityForAPI(product.Visibility, product.CustomerID)
		if visibility == "public" || product.CustomerID == 0 {
			product.Visibility = "public"
			out = append(out, product)
			continue
		}
		if customerID > 0 && product.CustomerID == customerID {
			product.Visibility = "customer_only"
			out = append(out, product)
		}
	}
	return out
}

func productVisibilityForAPI(visibility string, customerID int64) string {
	visibility = strings.TrimSpace(visibility)
	if visibility != "" {
		return visibility
	}
	if customerID > 0 {
		return "customer_only"
	}
	return "public"
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
		"responsible_type":        ed.ResponsibleType,
		"responsible_id":          ed.ResponsibleID,
		"responsible_name":        ed.ResponsibleName,
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
