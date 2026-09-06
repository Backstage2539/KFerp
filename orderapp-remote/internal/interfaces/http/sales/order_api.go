package sales

import (
	"encoding/json"
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type orderAPIHandler struct {
	sales         *salesapp.Service
	messages      MessagePublisher
	assetDir      string
	customerScope CustomerScopeResolver
}

type apiOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type customerAPIOption struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	CustomerType            string `json:"customer_type,omitempty"`
	Contact                 string `json:"contact,omitempty"`
	Phone                   string `json:"phone,omitempty"`
	Address                 string `json:"address,omitempty"`
	CompanyName             string `json:"company_name,omitempty"`
	CompanyAddress          string `json:"company_address,omitempty"`
	CompanyPhone            string `json:"company_phone,omitempty"`
	Py                      string `json:"py"`
	Pyi                     string `json:"pyi"`
	DefaultSourceID         int64  `json:"default_source_id,omitempty"`
	DefaultOrderTypeID      int64  `json:"default_order_type_id,omitempty"`
	ResponsibleEmployeeID   int64  `json:"responsible_employee_id,omitempty"`
	ResponsibleEmployeeName string `json:"responsible_employee_name,omitempty"`
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
	Today                  string                                `json:"today"`
	Customers              []customerAPIOption                   `json:"customers"`
	Employees              []employeeAPIOption                   `json:"employees"`
	Sources                []apiOption                           `json:"sources"`
	ShipStatuses           []apiOption                           `json:"ship_statuses"`
	PayStatuses            []apiOption                           `json:"pay_statuses"`
	OrderTypes             []apiOption                           `json:"order_types"`
	Products               []map[string]any                      `json:"products"`
	ProductFamilies        []map[string]any                      `json:"product_families"`
	ProductBOMSpecOptions  []salesapp.ProductBOMSpecOption       `json:"product_bom_spec_options"`
	Logistics              []salesapp.LogisticsCompany           `json:"logistics_companies"`
	BeanListVersionOptions []salesapp.BeanListVersionOption      `json:"bean_list_version_options"`
	CustomerPublicUsages   []salesapp.CustomerPublicUsageOption  `json:"customer_public_usages"`
	CustomerProductUsages  []salesapp.CustomerProductUsageOption `json:"customer_product_usages"`
	EditMode               bool                                  `json:"edit_mode"`
	EditID                 int64                                 `json:"edit_id"`
	EditData               any                                   `json:"edit_data,omitempty"`
}

type orderSaveAPIRequest struct {
	EditID                          int64  `json:"edit_id"`
	DocumentDate                    string `json:"document_date"`
	OrderDate                       string `json:"order_date"`
	CustomerID                      int64  `json:"customer_id"`
	SourceID                        int64  `json:"source_id"`
	OrderTypeID                     int64  `json:"order_type_id"`
	PayStatusID                     int64  `json:"pay_status_id"`
	PaymentMethod                   string `json:"payment_method"`
	ShipStatusID                    int64  `json:"ship_status_id"`
	ShipMethod                      string `json:"ship_method"`
	ShipTrackingNo                  string `json:"ship_tracking_no"`
	LogisticsCompanyID              int64  `json:"logistics_company_id"`
	LogisticsProductID              int64  `json:"logistics_product_id"`
	PaymentGoodsAmount              string `json:"payment_goods_amount"`
	PaymentShippingAmount           string `json:"payment_shipping_amount"`
	PaymentVoucherAssetID           int64  `json:"payment_voucher_asset_id"`
	ResponsibleType                 string `json:"responsible_type"`
	ResponsibleID                   int64  `json:"responsible_id"`
	Notes                           string `json:"notes"`
	ShippingAmount                  string `json:"shipping_amount"`
	DiscountAmount                  string `json:"discount_amount"`
	RoundToInt                      string `json:"round_to_int"`
	ExpressFee                      string `json:"express_fee"`
	OutsourceMaterialFee            string `json:"outsource_material_fee"`
	OutsourceRoastFee               string `json:"outsource_roast_fee"`
	OutsourcePackagingFee           string `json:"outsource_packaging_fee"`
	OutsourceManualFee              string `json:"outsource_manual_fee"`
	OutsourceTaxFee                 string `json:"outsource_tax_fee"`
	OutsourceOtherFee               string `json:"outsource_other_fee"`
	StockBatchDecision              string `json:"stock_batch_decision"`
	BeanListPublicationID           int64  `json:"bean_list_publication_id"`
	CommercialBeanListPublicationID int64  `json:"commercial_bean_list_publication_id"`
	GreenBeanListPublicationID      int64  `json:"green_bean_list_publication_id"`
	DripBeanListPublicationID       int64  `json:"drip_bean_list_publication_id"`
	ReceiverName                    string `json:"receiver_name"`
	ReceiverPhone                   string `json:"receiver_phone"`
	ReceiverAddress                 string `json:"receiver_address"`
	ReceiverCompany                 string `json:"receiver_company"`
	PortalServiceCode               string `json:"portal_service_code"`
	OrdersScope                     string `json:"orders_scope"`

	ProductID                          []string `json:"product_id"`
	ParentProductID                    []string `json:"parent_product_id"`
	ItemParentProductID                []string `json:"item_parent_product_id"`
	BomSpecID                          []string `json:"bom_spec_id"`
	BomVariantID                       []string `json:"bom_variant_id"`
	CustomerProductAliasID             []string `json:"customer_product_alias_id"`
	CustomerProductReferenceID         []string `json:"customer_product_reference_id"`
	MaterialSourceMode                 []string `json:"material_source_mode"`
	CustomerProductDisplayNameSnapshot []string `json:"customer_product_display_name_snapshot"`
	CustomerItemCodeSnapshot           []string `json:"customer_item_code_snapshot"`
	BrandNameSnapshot                  []string `json:"brand_name_snapshot"`
	ProductCodeSnapshot                []string `json:"product_code_snapshot"`
	ProductNameSnapshot                []string `json:"product_name_snapshot"`
	ItemBeanListPublicationID          []string `json:"item_bean_list_publication_id"`
	ItemBeanListVersionNo              []string `json:"item_bean_list_version_no"`
	PriceSourceJSON                    []string `json:"price_source_json"`
	TierID                             []string `json:"tier_id"`
	UnitPrice                          []string `json:"unit_price"`
	ItemName                           []string `json:"item_name"`
	ItemNote                           []string `json:"item_note"`
	Qty                                []string `json:"qty"`
	Unit                               []string `json:"unit"`
	Spec                               []string `json:"spec"`
	ProductKind                        []string `json:"product_kind"`
	SalesUnit                          []string `json:"sales_unit"`
	UnitBagCount                       []string `json:"unit_bag_count"`
	UnitBeanG                          []string `json:"unit_bean_g"`
	DiscountType                       []string `json:"discount_type"`
	DiscountValue                      []string `json:"discount_value"`
}

type orderVoidAPIRequest struct {
	Reason string `json:"reason"`
}

type orderVoidManyAPIRequest struct {
	OrderIDs []int64 `json:"order_ids"`
	Reason   string  `json:"reason"`
}

func registerOrderAPI(e *echo.Echo, salesSvc *salesapp.Service, messages MessagePublisher, assetDirs ...any) {
	assetDir := "/app/data/assets"
	var customerScope CustomerScopeResolver
	for _, value := range assetDirs {
		switch item := value.(type) {
		case string:
			if strings.TrimSpace(item) != "" {
				assetDir = strings.TrimSpace(item)
			}
		case CustomerScopeResolver:
			customerScope = item
		}
	}
	h := orderAPIHandler{
		sales:         salesSvc,
		messages:      messages,
		assetDir:      assetDir,
		customerScope: customerScope,
	}
	e.GET("/api/orders", h.list)
	e.GET("/api/orders/:id/detail", h.detail)
	e.POST("/api/orders/:id/void", h.void)
	e.POST("/api/orders/void", h.voidMany)
	e.GET("/api/order/form", h.form)
	e.POST("/api/order/stock-batch-preview", h.stockBatchPreview)
	e.POST("/api/order/payment-vouchers", h.uploadPaymentVoucher)
	e.POST("/api/order", h.save)
}

func (h orderAPIHandler) list(c echo.Context) error {
	query, ok := ordersQueryFromContext(c)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid scope"})
	}
	result, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{
		Q:                     query.Q,
		From:                  query.From,
		To:                    query.To,
		Void:                  query.Void,
		Scope:                 query.Scope,
		EmployeeID:            query.EmployeeID,
		FulfillmentEmployeeID: query.FulfillmentEmployeeID,
		OrderID:               query.OrderID,
		CustomerID:            query.CustomerID,
		PayStatusID:           query.PayStatusID,
		ShipStatusID:          query.ShipStatusID,
		ProcessStatusID:       query.ProcessStatusID,
		UnproducedOnly:        query.UnproducedOnly,
		CompletedOnly:         query.CompletedOnly,
		ShipReadyOnly:         query.ShipReadyOnly,
		Limit:                 query.Limit,
		Offset:                query.Offset,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	total := result.Summary.Orders
	totalPages := orderAPITotalPages(total, query.Limit)

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
		"total":            total,
		"total_pages":      totalPages,
		"has_prev":         query.Offset > 0,
		"has_next":         query.Page < totalPages,
	})
}

func orderAPITotalPages(total, limit int) int {
	if limit <= 0 {
		limit = 10
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
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
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		if h.customerScope == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer scope unavailable"})
		}
		boundID, resolveErr := h.customerScope.BoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if resolveErr != nil || boundID <= 0 {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
		}
		if filterByCustomer && customerID != boundID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer scope mismatch"})
		}
		customerID = boundID
		filterByCustomer = true
	}
	if filterByCustomer {
		data.Products = filterOrderProductsForCustomer(data.Products, customerID, data.BeanListVersionOptions, data.CustomerPublicUsages)
	}

	resp := orderFormAPIResponse{
		Today:                  data.Today,
		Customers:              apiCustomerOptions(data.Customers),
		Employees:              apiEmployeeOptions(data.Employees),
		Sources:                apiOptions(data.Sources),
		ShipStatuses:           apiOptions(data.ShipStatuses),
		PayStatuses:            apiOptions(data.PayStatuses),
		OrderTypes:             apiOptions(data.OrderTypes),
		Products:               apiProducts(data.Products),
		ProductFamilies:        apiProductFamilies(data.Products),
		ProductBOMSpecOptions:  data.ProductBOMSpecOptions,
		Logistics:              data.LogisticsCompanies,
		BeanListVersionOptions: data.BeanListVersionOptions,
		CustomerPublicUsages:   data.CustomerPublicUsages,
		CustomerProductUsages:  data.CustomerProductUsages,
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

func (h orderAPIHandler) detail(c echo.Context) error {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	data, err := h.sales.OrderForm(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if data.EditData == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		if err := h.ensureFulfillmentOrderDetailAccess(c, id, data.EditData.CustomerID); err != nil {
			return err
		}
	}
	return c.JSON(http.StatusOK, orderFormAPIResponse{
		Today:                 data.EditData.OrderDate,
		ProductBOMSpecOptions: data.ProductBOMSpecOptions,
		EditMode:              true,
		EditID:                id,
		EditData:              editDataForAPI(data.EditData),
	})
}

func (h orderAPIHandler) void(c echo.Context) error {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req orderVoidAPIRequest
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
	}
	if err := h.sales.Void(c.Request().Context(), id, support.ActorOf(c), strings.TrimSpace(req.Reason)); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"order_id": id,
		"is_void":  true,
	})
}

func (h orderAPIHandler) voidMany(c echo.Context) error {
	var req orderVoidManyAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}
	count, err := h.sales.VoidMany(c.Request().Context(), req.OrderIDs, support.ActorOf(c), strings.TrimSpace(req.Reason))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"order_ids": req.OrderIDs,
		"voided":    count,
		"is_void":   true,
	})
}

func (h orderAPIHandler) ensureFulfillmentOrderDetailAccess(c echo.Context, orderID int64, customerID int64) error {
	employeeID := support.CurrentEmployeeID(c)
	if orderID <= 0 || customerID <= 0 || employeeID <= 0 {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied"})
	}
	result, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{
		OrderID:               orderID,
		Scope:                 "fulfillment",
		CustomerID:            customerID,
		EmployeeID:            employeeID,
		FulfillmentEmployeeID: employeeID,
		Void:                  "all",
		Limit:                 1,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	for _, row := range result.Rows {
		if row.ID == orderID {
			return nil
		}
	}
	return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied"})
}

type ordersAPIQuery struct {
	Q                     string
	From                  string
	To                    string
	Void                  string
	Scope                 string
	EmployeeID            int64
	FulfillmentEmployeeID int64
	OrderID               int64
	CustomerID            int64
	PayStatusID           int64
	ShipStatusID          int64
	ProcessStatusID       int64
	UnproducedOnly        bool
	CompletedOnly         bool
	ShipReadyOnly         bool
	Limit                 int
	Offset                int
	Page                  int
}

func ordersQueryFromContext(c echo.Context) (ordersAPIQuery, bool) {
	q := ordersAPIQuery{
		Q:     strings.TrimSpace(c.QueryParam("q")),
		From:  strings.TrimSpace(c.QueryParam("from")),
		To:    strings.TrimSpace(c.QueryParam("to")),
		Void:  strings.TrimSpace(c.QueryParam("void")),
		Scope: strings.TrimSpace(c.QueryParam("scope")),
		Limit: support.IntParam(c, "limit", 10),
	}
	if !validOrderListScope(q.Scope) {
		return q, false
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
	q.OrderID = int64(support.IntParam(c, "order_id", 0))
	q.EmployeeID = support.CurrentEmployeeID(c)
	q.PayStatusID = int64(support.IntParam(c, "pay_status_id", 0))
	q.ShipStatusID = int64(support.IntParam(c, "ship_status_id", 0))
	q.ProcessStatusID = int64(support.IntParam(c, "process_status_id", 0))
	q.UnproducedOnly = strings.TrimSpace(c.QueryParam("preset")) == "unprod"
	q.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
	q.ShipReadyOnly = strings.TrimSpace(c.QueryParam("ship_ready")) == "1"
	if q.Scope == "fulfillment" && support.CustomerFulfillmentOrderScopeLimited(c) {
		q.FulfillmentEmployeeID = q.EmployeeID
	}
	if q.Void == "" {
		q.Void = "normal"
	}
	return q, true
}

func validOrderListScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case "", "all", "mine", "fulfillment":
		return true
	default:
		return false
	}
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
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		if h.customerScope == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer scope unavailable"})
		}
		boundID, resolveErr := h.customerScope.BoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if resolveErr != nil || boundID <= 0 || cmd.CustomerID != boundID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer scope mismatch"})
		}
	}
	res, err := h.sales.SaveOrder(c.Request().Context(), cmd)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if !res.Edited {
		h.publishOrderCreated(c, res)
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

func (h orderAPIHandler) publishOrderCreated(c echo.Context, res salesapp.SaveOrderResult) {
	if h.messages == nil || res.OrderID <= 0 {
		return
	}
	_, _ = h.messages.Publish(c.Request().Context(), messagecenterapp.PublishCommand{
		EventKey:   "order.created." + strconv.FormatInt(res.OrderID, 10),
		Topic:      "orders",
		EventType:  "order.created",
		SourceType: "order",
		SourceID:   res.OrderID,
		Actor:      support.ActorOf(c),
		Title:      "新订单 " + strings.TrimSpace(res.OrderNo),
		Body:       "ERP 订单已创建",
		Tone:       "success",
		Payload: map[string]any{
			"order_id":           res.OrderID,
			"order_no":           res.OrderNo,
			"orders_scope":       "all",
			"highlight_order_id": res.OrderID,
		},
		Deliveries: []messagecenterapp.DeliveryCommand{{
			Channel:    messagecenterapp.ChannelERPPlatform,
			TargetType: "permission",
			TargetKey:  "orders.read",
		}},
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

func (h orderAPIHandler) uploadPaymentVoucher(c echo.Context) error {
	if err := support.RequireEmployeeBound(c); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	asset, err := saveUploadedSalesOrderAsset(c, h.sales, h.assetDir, "payment_voucher")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"asset": asset})
}

func (r orderSaveAPIRequest) toCreateRequest() CreateOrderRequest {
	return CreateOrderRequest{
		DocumentDate:                       r.DocumentDate,
		OrderDate:                          r.OrderDate,
		CustomerID:                         r.CustomerID,
		SourceID:                           r.SourceID,
		OrderTypeID:                        r.OrderTypeID,
		PayStatusID:                        r.PayStatusID,
		PaymentMethod:                      r.PaymentMethod,
		ShipStatusID:                       r.ShipStatusID,
		ShipMethod:                         r.ShipMethod,
		ShipTrackingNo:                     r.ShipTrackingNo,
		LogisticsCompanyID:                 r.LogisticsCompanyID,
		LogisticsProductID:                 r.LogisticsProductID,
		PaymentGoodsAmount:                 r.PaymentGoodsAmount,
		PaymentShippingAmount:              r.PaymentShippingAmount,
		PaymentVoucherAssetID:              r.PaymentVoucherAssetID,
		ResponsibleType:                    r.ResponsibleType,
		ResponsibleID:                      r.ResponsibleID,
		Notes:                              r.Notes,
		ShippingAmount:                     r.ShippingAmount,
		DiscountAmount:                     r.DiscountAmount,
		RoundToInt:                         r.RoundToInt,
		ExpressFee:                         r.ExpressFee,
		OutsourceMaterialFee:               r.OutsourceMaterialFee,
		OutsourceRoastFee:                  r.OutsourceRoastFee,
		OutsourcePackagingFee:              r.OutsourcePackagingFee,
		OutsourceManualFee:                 r.OutsourceManualFee,
		OutsourceTaxFee:                    r.OutsourceTaxFee,
		OutsourceOtherFee:                  r.OutsourceOtherFee,
		StockBatchDecision:                 r.StockBatchDecision,
		BeanListPublicationID:              r.BeanListPublicationID,
		CommercialBeanListPublicationID:    r.CommercialBeanListPublicationID,
		GreenBeanListPublicationID:         r.GreenBeanListPublicationID,
		DripBeanListPublicationID:          r.DripBeanListPublicationID,
		ReceiverName:                       r.ReceiverName,
		ReceiverPhone:                      r.ReceiverPhone,
		ReceiverAddress:                    r.ReceiverAddress,
		ReceiverCompany:                    r.ReceiverCompany,
		PortalServiceCode:                  r.PortalServiceCode,
		OrdersScope:                        r.OrdersScope,
		ProductID:                          r.ProductID,
		ParentProductID:                    r.ParentProductID,
		ItemParentProductID:                r.ItemParentProductID,
		BomSpecID:                          r.BomSpecID,
		BomVariantID:                       r.BomVariantID,
		CustomerProductAliasID:             r.CustomerProductAliasID,
		CustomerProductReferenceID:         r.CustomerProductReferenceID,
		CustomerProductDisplayNameSnapshot: r.CustomerProductDisplayNameSnapshot,
		CustomerItemCodeSnapshot:           r.CustomerItemCodeSnapshot,
		BrandNameSnapshot:                  r.BrandNameSnapshot,
		ProductCodeSnapshot:                r.ProductCodeSnapshot,
		ProductNameSnapshot:                r.ProductNameSnapshot,
		ItemBeanListPublicationID:          r.ItemBeanListPublicationID,
		ItemBeanListVersionNo:              r.ItemBeanListVersionNo,
		PriceSourceJSON:                    r.PriceSourceJSON,
		TierID:                             r.TierID,
		UnitPrice:                          r.UnitPrice,
		ItemName:                           r.ItemName,
		ItemNote:                           r.ItemNote,
		Qty:                                r.Qty,
		Unit:                               r.Unit,
		Spec:                               r.Spec,
		ProductKind:                        r.ProductKind,
		SalesUnit:                          r.SalesUnit,
		UnitBagCount:                       r.UnitBagCount,
		UnitBeanG:                          r.UnitBeanG,
		DiscountType:                       r.DiscountType,
		DiscountValue:                      r.DiscountValue,
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
			ID:                      item.ID,
			Name:                    item.Name,
			CustomerType:            item.CustomerType,
			Contact:                 item.Contact,
			Phone:                   item.Phone,
			Address:                 item.Address,
			CompanyName:             item.CompanyName,
			CompanyAddress:          item.CompanyAddress,
			CompanyPhone:            item.CompanyPhone,
			Py:                      support.PinyinFull(item.Name),
			Pyi:                     support.PinyinInitials(item.Name),
			DefaultSourceID:         item.DefaultSourceID,
			DefaultOrderTypeID:      item.DefaultOrderTypeID,
			ResponsibleEmployeeID:   item.ResponsibleEmployeeID,
			ResponsibleEmployeeName: item.ResponsibleEmployeeName,
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

func apiProducts(ps []ProductOption) []map[string]any {
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		jp := map[string]any{
			"id":                                   p.ID,
			"name":                                 p.Name,
			"product_code":                         p.ProductCode,
			"product_name_snapshot":                firstNonEmpty(p.ProductRecordName, p.Name),
			"customer_product_alias_id":            p.CustomerProductAliasID,
			"customer_product_reference_id":        p.CustomerProductReferenceID,
			"customer_product_display_name":        p.CustomerProductDisplayName,
			"customer_item_code":                   p.CustomerItemCode,
			"brand_name":                           p.BrandName,
			"customer_alias_display_category_id":   p.CustomerAliasDisplayCategoryID,
			"customer_alias_display_category_name": p.CustomerAliasDisplayCategoryName,
			"py":                                   support.PinyinFull(p.Name),
			"pyi":                                  support.PinyinInitials(p.Name),
			"retail_price_100g":                    p.RetailPrice100G,
			"retail_price_200g":                    p.RetailPrice200G,
			"retail_price_227g":                    p.RetailPrice227G,
			"retail_price_250g":                    p.RetailPrice250G,
			"customer_id":                          p.CustomerID,
			"base_product_id":                      p.BaseProductID,
			"visibility":                           productVisibilityForAPI(p.Visibility, p.CustomerID),
			"custom_type":                          p.CustomType,
			"product_kind":                         p.ProductKind,
			"drip_bag_grams":                       p.DripBagGrams,
			"drip_box_bag_count":                   p.DripBoxBagCount,
			"sales_units":                          p.SalesUnits,
			"retail_specs":                         p.RetailSpecs,
			"product_type_category_id":             p.ProductTypeCategoryID,
			"product_subtype_category_id":          p.ProductSubtypeCategoryID,
			"product_type_name":                    p.ProductTypeName,
			"product_subtype_name":                 p.ProductSubtypeName,
		}
		tiers := make([]map[string]any, 0, len(p.Tiers))
		for _, t := range p.Tiers {
			tiers = append(tiers, apiProductTier(t))
		}
		jp["tiers"] = tiers
		out = append(out, jp)
	}
	return out
}

func apiProductTier(t ProductTierOption) map[string]any {
	tier := map[string]any{
		"id":                     t.ID,
		"spec_g":                 t.SpecG,
		"min":                    t.MinQty,
		"max":                    t.MaxQty,
		"min_qty":                t.MinQty,
		"max_qty":                t.MaxQty,
		"unit_price":             t.UnitPrice,
		"price":                  t.UnitPrice,
		"product_kind":           t.ProductKind,
		"sales_unit":             t.SalesUnit,
		"unit_bag_count":         t.UnitBagCount,
		"price_source_json":      t.PriceSourceJSON,
		"publication_id":         t.PublicationID,
		"publication_version_no": t.PublicationVersionNo,
		"version_no":             t.PublicationVersionNo,
		"list_type":              t.ListType,
		"parent_product_id":      t.ParentProductID,
		"bom_spec_id":            t.BomSpecID,
		"bom_variant_id":         t.BomVariantID,
	}
	if t.QuantityBasis != "" {
		tier["quantity_basis"] = t.QuantityBasis
	}
	if t.TierQuantityUnit != "" {
		tier["tier_quantity_unit"] = t.TierQuantityUnit
	}
	if len(t.EffectiveSalesSpec) > 0 {
		tier["effective_sales_spec"] = t.EffectiveSalesSpec
	}
	if t.DisplayUnit != "" {
		tier["display_unit"] = t.DisplayUnit
	}
	return tier
}

func apiProductFamilies(products []ProductOption) []map[string]any {
	return salesapp.BuildOrderProductFamilies(products)
}

func effectiveOrderSKUID(product ProductOption) int64 {
	if product.SKUID > 0 {
		return product.SKUID
	}
	return product.ID
}

func effectiveOrderParentProductID(product ProductOption) int64 {
	if product.ParentProductID > 0 {
		return product.ParentProductID
	}
	return product.ID
}

func effectiveOrderParentProductName(product ProductOption) string {
	return firstNonEmpty(product.ParentProductName, product.ProductRecordName, product.Name)
}

func effectiveOrderSKUName(product ProductOption) string {
	return firstNonEmpty(product.SKUName, product.SpecLabel, "默认规格")
}

func filterOrderProductsForCustomer(products []ProductOption, customerID int64, versionOptions []salesapp.BeanListVersionOption, publicUsages ...[]salesapp.CustomerPublicUsageOption) []ProductOption {
	return salesapp.FilterOrderProductsForCustomer(products, customerID, versionOptions, publicUsages...)
}

func orderProductBeanListType(productKind string) string {
	switch strings.TrimSpace(productKind) {
	case "green_bean":
		return "green"
	case "drip_bag":
		return "drip"
	default:
		return "commercial"
	}
}

func normalizeOrderBeanListType(listType string) string {
	switch strings.TrimSpace(listType) {
	case "green":
		return "green"
	case "drip":
		return "drip"
	default:
		return "commercial"
	}
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

func orderQuoteSourceTrace(ed *OrderEditData) []map[string]any {
	if ed == nil {
		return nil
	}
	rows := make([]map[string]any, 0, len(ed.Items))
	for _, it := range ed.Items {
		source := orderPriceSourceSnapshot(it.PriceSourceJSON)
		version := orderTraceString(source["version"])
		if version == "" {
			version = it.BeanListVersionNo
		}
		publicationID := int64(orderTraceNumber(source["publication_id"]))
		if publicationID <= 0 {
			publicationID = it.BeanListPublicationID
		}
		rows = append(rows, map[string]any{
			"product_id":                it.ProductID,
			"product_name":              it.Product,
			"price_list_publication_id": publicationID,
			"price_list_version":        version,
			"tier_label":                orderTraceString(source["tier_label"]),
			"price_unit":                orderTraceString(source["price_unit"]),
			"final_unit_price":          orderTraceNumber(source["final_unit_price"]),
			"pricing_rule_version":      orderTraceString(source["pricing_rule_version"]),
			"manual_adjusted":           orderTraceBool(source["manual_adjusted"]),
			"source_label":              "已发布商品价格表快照",
		})
	}
	return rows
}

func orderProductionSourceTrace(ed *OrderEditData) []map[string]any {
	if ed == nil {
		return nil
	}
	rows := make([]map[string]any, 0, len(ed.Items))
	for _, it := range ed.Items {
		source := orderPriceSourceSnapshot(it.PriceSourceJSON)
		costSource := orderTraceObject(source["cost_source_snapshot"])
		if len(costSource) == 0 {
			costSource = orderTraceObject(source["production_source_snapshot"])
		}
		if len(costSource) == 0 {
			continue
		}
		row := map[string]any{
			"product_id":    it.ProductID,
			"product_name":  it.Product,
			"source_label":  "工单/工序卡冻结快照",
			"work_order_no": orderTraceString(costSource["work_order_no"]),
		}
		for _, key := range []string{"bom_version_no", "bom_version_id", "process_route_name", "process_card_no", "material_batch_no"} {
			if value, ok := costSource[key]; ok {
				row[key] = value
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func orderPriceSourceSnapshot(value string) map[string]any {
	source := map[string]any{}
	if strings.TrimSpace(value) == "" {
		return source
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &source); err != nil {
		return map[string]any{}
	}
	return source
}

func orderTraceObject(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case string:
		out := map[string]any{}
		if strings.TrimSpace(v) == "" {
			return out
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(v)), &out); err != nil {
			return map[string]any{}
		}
		return out
	default:
		return map[string]any{}
	}
}

func orderTraceString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64, float32, int, int64:
		return strings.TrimSpace(strconv.FormatFloat(orderTraceNumber(value), 'f', -1, 64))
	default:
		return ""
	}
}

func orderTraceNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func orderTraceBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	default:
		return orderTraceNumber(value) != 0
	}
}

func editDataForAPI(ed *OrderEditData) map[string]any {
	type editItem struct {
		ProductID                          int64  `json:"product_id"`
		BomSpecID                          int64  `json:"bom_spec_id,omitempty"`
		BomVariantID                       int64  `json:"bom_variant_id,omitempty"`
		BomSpecKey                         string `json:"bom_spec_key,omitempty"`
		BomSpecName                        string `json:"bom_spec_name,omitempty"`
		ProductName                        string `json:"product_name"`
		CustomerProductAliasID             int64  `json:"customer_product_alias_id"`
		CustomerProductDisplayNameSnapshot string `json:"customer_product_display_name_snapshot"`
		CustomerItemCodeSnapshot           string `json:"customer_item_code_snapshot"`
		BrandNameSnapshot                  string `json:"brand_name_snapshot"`
		ProductCodeSnapshot                string `json:"product_code_snapshot"`
		ProductNameSnapshot                string `json:"product_name_snapshot"`
		Note                               string `json:"note"`
		TierID                             string `json:"tier_id"`
		PriceOverride                      bool   `json:"price_override"`
		UnitPrice                          string `json:"unit_price"`
		Qty                                string `json:"qty"`
		Unit                               string `json:"unit"`
		Spec                               string `json:"spec"`
		BeanListPublicationID              int64  `json:"bean_list_publication_id"`
		BeanListVersionNo                  string `json:"bean_list_version_no"`
		DiscountType                       string `json:"discount_type"`
		DiscountValue                      string `json:"discount_value"`
		DiscountAmount                     string `json:"discount_amount"`
		ProductKind                        string `json:"product_kind"`
		SalesUnit                          string `json:"sales_unit"`
		UnitBagCount                       int64  `json:"unit_bag_count"`
		UnitBeanG                          string `json:"unit_bean_g"`
		MatchedPriceQty                    string `json:"matched_price_qty"`
		UnitConversionLabel                string `json:"unit_conversion_label"`
		PriceSourceJSON                    string `json:"price_source_json"`
	}
	items := make([]editItem, 0, len(ed.Items))
	for _, it := range ed.Items {
		spec := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(it.Spec)), "g")
		tierID := "auto"
		if it.PriceOverride {
			tierID = "manual"
		} else if it.PriceTierID > 0 {
			tierID = strconv.FormatInt(it.PriceTierID, 10)
		}
		items = append(items, editItem{
			ProductID:                          it.ProductID,
			BomSpecID:                          it.BomSpecID,
			BomVariantID:                       it.BomVariantID,
			BomSpecKey:                         it.BomSpecKey,
			BomSpecName:                        it.BomSpecName,
			ProductName:                        it.Product,
			CustomerProductAliasID:             it.CustomerProductAliasID,
			CustomerProductDisplayNameSnapshot: it.CustomerProductDisplayNameSnapshot,
			CustomerItemCodeSnapshot:           it.CustomerItemCodeSnapshot,
			BrandNameSnapshot:                  it.BrandNameSnapshot,
			ProductCodeSnapshot:                it.ProductCodeSnapshot,
			ProductNameSnapshot:                it.ProductNameSnapshot,
			Note:                               it.Note,
			TierID:                             tierID,
			PriceOverride:                      it.PriceOverride,
			UnitPrice:                          it.UnitPrice,
			Qty:                                it.Qty,
			Unit:                               it.Unit,
			Spec:                               spec,
			BeanListPublicationID:              it.BeanListPublicationID,
			BeanListVersionNo:                  it.BeanListVersionNo,
			DiscountType:                       it.DiscountType,
			DiscountValue:                      it.DiscountValue,
			DiscountAmount:                     it.DiscountAmount,
			ProductKind:                        it.ProductKind,
			SalesUnit:                          it.SalesUnit,
			UnitBagCount:                       it.UnitBagCount,
			UnitBeanG:                          it.UnitBeanG,
			MatchedPriceQty:                    it.MatchedPriceQty,
			UnitConversionLabel:                it.UnitConversionLabel,
			PriceSourceJSON:                    it.PriceSourceJSON,
		})
	}
	itemPublicationByType := func(listType string) (int64, string, bool) {
		publicationID := int64(0)
		versionNo := ""
		for _, it := range ed.Items {
			itemType := orderEditItemBeanListType(it)
			if itemType != listType || it.BeanListPublicationID <= 0 {
				continue
			}
			if publicationID > 0 && publicationID != it.BeanListPublicationID {
				return 0, "", true
			}
			publicationID = it.BeanListPublicationID
			if versionNo == "" {
				versionNo = strings.TrimSpace(it.BeanListVersionNo)
			}
		}
		return publicationID, versionNo, false
	}
	commercialItemPublicationID, commercialItemVersionNo, commercialPublicationAmbiguous := itemPublicationByType("commercial")
	commercialPublicationID := ed.BeanListPublicationID
	beanListPublicationID := ed.BeanListPublicationID
	beanListVersionNo := ed.BeanListVersionNo
	if commercialPublicationAmbiguous {
		commercialPublicationID = 0
		beanListPublicationID = 0
		beanListVersionNo = ""
	} else if commercialItemPublicationID > 0 {
		commercialPublicationID = commercialItemPublicationID
		beanListPublicationID = commercialItemPublicationID
		if commercialItemVersionNo != "" {
			beanListVersionNo = commercialItemVersionNo
		} else if ed.BeanListPublicationID != commercialItemPublicationID {
			beanListVersionNo = ""
		}
	}
	greenPublicationID, _, _ := itemPublicationByType("green")
	dripPublicationID, _, _ := itemPublicationByType("drip")
	return map[string]any{
		"document_date":                       ed.DocumentDate,
		"order_date":                          ed.OrderDate,
		"customer_id":                         strconv.FormatInt(ed.CustomerID, 10),
		"source_id":                           strconv.FormatInt(ed.SourceID, 10),
		"order_type_id":                       strconv.FormatInt(ed.OrderTypeID, 10),
		"pay_status_id":                       strconv.FormatInt(ed.PayStatusID, 10),
		"payment_method":                      ed.PaymentMethod,
		"ship_status_id":                      strconv.FormatInt(ed.ShipStatusID, 10),
		"ship_method":                         ed.ShipMethod,
		"ship_tracking_no":                    ed.ShipTrackingNo,
		"logistics_company_id":                ed.LogisticsCompanyID,
		"logistics_product_id":                ed.LogisticsProductID,
		"payment_goods_amount":                ed.PaymentGoodsAmount,
		"payment_shipping_amount":             ed.PaymentShippingAmount,
		"payment_voucher_asset_id":            ed.PaymentVoucherAssetID,
		"payment_voucher":                     ed.PaymentVoucher,
		"responsible_type":                    ed.ResponsibleType,
		"responsible_id":                      ed.ResponsibleID,
		"responsible_name":                    ed.ResponsibleName,
		"receiver_name":                       ed.ReceiverName,
		"receiver_phone":                      ed.ReceiverPhone,
		"receiver_address":                    ed.ReceiverAddress,
		"receiver_company":                    ed.ReceiverCompany,
		"portal_service_code":                 ed.PortalServiceCode,
		"source_warehouse":                    ed.SourceWarehouse,
		"bean_list_publication_id":            beanListPublicationID,
		"commercial_bean_list_publication_id": commercialPublicationID,
		"green_bean_list_publication_id":      greenPublicationID,
		"drip_bean_list_publication_id":       dripPublicationID,
		"bean_list_version_no":                beanListVersionNo,
		"notes":                               ed.Notes,
		"shipping_amount":                     ed.ShippingAmount,
		"discount_amount":                     ed.DiscountAmount,
		"round_to_int":                        ed.RoundToInt,
		"express_fee":                         ed.ExpressFee,
		"outsource_material_fee":              ed.OutsourceMaterialFee,
		"outsource_roast_fee":                 ed.OutsourceRoastFee,
		"outsource_packaging_fee":             ed.OutsourcePackagingFee,
		"outsource_manual_fee":                ed.OutsourceManualFee,
		"outsource_tax_fee":                   ed.OutsourceTaxFee,
		"outsource_other_fee":                 ed.OutsourceOtherFee,
		"outsource_total_fee":                 ed.OutsourceTotalFee,
		"items":                               items,
		"quote_source_trace":                  orderQuoteSourceTrace(ed),
		"production_source_trace":             orderProductionSourceTrace(ed),
	}
}

func orderEditItemBeanListType(item OrderEditItem) string {
	var source struct {
		ListType string `json:"list_type"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(item.PriceSourceJSON)), &source); err == nil && strings.TrimSpace(source.ListType) != "" {
		return normalizeOrderBeanListType(source.ListType)
	}
	return orderProductBeanListType(item.ProductKind)
}
