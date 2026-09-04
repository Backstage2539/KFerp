package customerfulfillment

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"
	messagecenterapp "orderapp/internal/application/messagecenter"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type api struct {
	svc       Service
	customers CustomerDirectory
	messages  MessagePublisher
	sales     SalesSaver
}

const externalUsersRouteSegment = "external-users"

func (a api) listCustomers(c echo.Context) error {
	if a.customers == nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, fmt.Errorf("customer directory unavailable"))
	}
	q := strings.TrimSpace(c.QueryParam("q"))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	if limit <= 0 {
		limit = 80
	}
	if limit > 200 {
		limit = 200
	}
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	if offset < 0 {
		offset = 0
	}
	result, err := a.customers.List(c.Request().Context(), customerapp.ListQuery{Query: q, Limit: limit, Offset: offset})
	if err != nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, err)
	}
	rows := make([]customerapp.CustomerRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		if !row.Active {
			continue
		}
		available, err := a.svc.CustomerERPWorkbenchAvailable(c.Request().Context(), row.ID)
		if err != nil {
			return customerFulfillmentError(c, http.StatusInternalServerError, err)
		}
		if row.PortalEnabled || available {
			rows = append(rows, row)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"customers": rows,
		"limit":     limit,
		"offset":    offset,
		"has_next":  result.HasNext,
	})
}

func (a api) hasActiveERPBinding(c echo.Context, customerID int64) (bool, error) {
	if a.svc == nil || customerID <= 0 {
		return false, nil
	}
	users, err := a.svc.ListExternalUsers(c.Request().Context(), customerID)
	if err != nil {
		return false, err
	}
	for _, user := range users {
		if strings.TrimSpace(user.BindingStatus) != "active" {
			continue
		}
		if strings.TrimSpace(user.Phone) == "" || !user.HasPassword || !user.LoginEnabled {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (a api) parseImport(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	importType := app.ImportType(strings.TrimSpace(c.FormValue("import_type")))
	file, err := c.FormFile("file")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("file required"))
	}
	src, err := file.Open()
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	defer src.Close()
	batch, err := a.svc.ParseImport(c.Request().Context(), app.ParseImportCommand{
		CustomerID:     customerID,
		ImportType:     importType,
		SourceFilename: strings.TrimSpace(file.Filename),
		Reader:         src,
		CreatedBy:      currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, importBatchResponse(batch))
}

func (a api) applyImport(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	result, err := a.svc.ApplyImport(c.Request().Context(), app.ApplyImportCommand{
		BatchID: batchID,
		Actor:   currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, result)
}

func (a api) customerPortalOverview(c echo.Context) error {
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return customerFulfillmentError(c, http.StatusUnauthorized, fmt.Errorf("employee required"))
	}
	overview, err := a.svc.CustomerPortalOverview(c.Request().Context(), employeeID)
	if err != nil {
		return customerPortalError(c, err)
	}
	return c.JSON(http.StatusOK, overview)
}

func (a api) customerPortalOptions(c echo.Context) error {
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return customerFulfillmentError(c, http.StatusUnauthorized, fmt.Errorf("employee required"))
	}
	options, err := a.svc.CustomerPortalOptions(c.Request().Context(), employeeID)
	if err != nil {
		return customerPortalError(c, err)
	}
	return c.JSON(http.StatusOK, options)
}

func (a api) internalCustomerPortalOverview(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	overview, err := a.svc.InternalCustomerPortalOverview(c.Request().Context(), customerID)
	if err != nil {
		return customerPortalError(c, err)
	}
	return c.JSON(http.StatusOK, overview)
}

func (a api) internalCustomerPortalOptions(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	options, err := a.svc.InternalCustomerPortalOptions(c.Request().Context(), customerID)
	if err != nil {
		return customerPortalError(c, err)
	}
	return c.JSON(http.StatusOK, options)
}

func (a api) submitCustomerProcessingWorkOrder(c echo.Context) error {
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return customerFulfillmentError(c, http.StatusUnauthorized, fmt.Errorf("employee required"))
	}
	var req struct {
		ProductID          int64  `json:"product_id"`
		ProductName        string `json:"product_name"`
		RawBeanItemID      int64  `json:"raw_bean_item_id"`
		RawBeanName        string `json:"raw_bean_name"`
		InputQuantityG     int64  `json:"input_quantity_g"`
		PlannedOutputUnits int64  `json:"planned_output_units"`
		ExpectedDate       string `json:"expected_date"`
		Note               string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.SubmitCustomerProcessingWorkOrder(c.Request().Context(), app.SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         employeeID,
		ProductID:          req.ProductID,
		ProductName:        req.ProductName,
		RawBeanItemID:      req.RawBeanItemID,
		RawBeanName:        req.RawBeanName,
		InputQuantityG:     req.InputQuantityG,
		PlannedOutputUnits: req.PlannedOutputUnits,
		ExpectedDate:       req.ExpectedDate,
		Note:               req.Note,
	})
	if err != nil {
		return customerPortalError(c, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) submitCustomerDirectShipOrder(c echo.Context) error {
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return customerFulfillmentError(c, http.StatusUnauthorized, fmt.Errorf("employee required"))
	}
	var req struct {
		ReceiverName    string `json:"receiver_name"`
		ReceiverPhone   string `json:"receiver_phone"`
		ReceiverAddress string `json:"receiver_address"`
		ReceiverCompany string `json:"receiver_company"`
		ProductID       int64  `json:"product_id"`
		ProductName     string `json:"product_name"`
		Spec            string `json:"spec"`
		QuantityUnits   int64  `json:"quantity_units"`
		Items           []struct {
			ProductID                          int64  `json:"product_id"`
			BomSpecID                          int64  `json:"bom_spec_id"`
			BomVariantID                       int64  `json:"bom_variant_id"`
			BomSpecKey                         string `json:"bom_spec_key"`
			BomSpecName                        string `json:"bom_spec_name"`
			InventoryUnit                      string `json:"inventory_unit"`
			CustomerProductAliasID             int64  `json:"customer_product_alias_id"`
			CustomerProductReferenceID         int64  `json:"customer_product_reference_id"`
			MaterialSourceMode                 string `json:"material_source_mode"`
			CustomerProductDisplayNameSnapshot string `json:"customer_product_display_name_snapshot"`
			CustomerItemCodeSnapshot           string `json:"customer_item_code_snapshot"`
			ProductCodeSnapshot                string `json:"product_code_snapshot"`
			ProductNameSnapshot                string `json:"product_name_snapshot"`
			ProductName                        string `json:"product_name"`
			Spec                               string `json:"spec"`
			SpecG                              int64  `json:"spec_g"`
			SalesUnit                          string `json:"sales_unit"`
			QuantityUnits                      int64  `json:"quantity_units"`
			Note                               string `json:"note"`
		} `json:"items"`
		Note string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	items := make([]app.SubmitCustomerDirectShipOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, app.SubmitCustomerDirectShipOrderItem{
			ProductID:                          item.ProductID,
			BomSpecID:                          item.BomSpecID,
			BomVariantID:                       item.BomVariantID,
			BomSpecKey:                         item.BomSpecKey,
			BomSpecName:                        item.BomSpecName,
			InventoryUnit:                      item.InventoryUnit,
			CustomerProductAliasID:             item.CustomerProductAliasID,
			CustomerProductReferenceID:         item.CustomerProductReferenceID,
			MaterialSourceMode:                 item.MaterialSourceMode,
			CustomerProductDisplayNameSnapshot: item.CustomerProductDisplayNameSnapshot,
			CustomerItemCodeSnapshot:           item.CustomerItemCodeSnapshot,
			ProductCodeSnapshot:                item.ProductCodeSnapshot,
			ProductNameSnapshot:                item.ProductNameSnapshot,
			ProductName:                        item.ProductName,
			Spec:                               item.Spec,
			SpecG:                              item.SpecG,
			SalesUnit:                          item.SalesUnit,
			QuantityUnits:                      item.QuantityUnits,
			Note:                               item.Note,
		})
	}
	row, err := a.svc.SubmitCustomerDirectShipOrder(c.Request().Context(), app.SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      employeeID,
		ReceiverName:    req.ReceiverName,
		ReceiverPhone:   req.ReceiverPhone,
		ReceiverAddress: req.ReceiverAddress,
		ReceiverCompany: req.ReceiverCompany,
		ProductID:       req.ProductID,
		ProductName:     req.ProductName,
		Spec:            req.Spec,
		QuantityUnits:   req.QuantityUnits,
		Items:           items,
		Note:            req.Note,
		Actor:           currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerPortalError(c, err)
	}
	a.publishCustomerFulfillmentDirectShipOrderCreated(c, row)
	return c.JSON(http.StatusOK, row)
}

func (a api) publishCustomerFulfillmentDirectShipOrderCreated(c echo.Context, row app.DirectShipOrderSummary) {
	if a.messages == nil || row.OrderID <= 0 {
		return
	}
	_, _ = a.messages.Publish(c.Request().Context(), messagecenterapp.PublishCommand{
		EventKey:   "order.created." + strconv.FormatInt(row.OrderID, 10),
		Topic:      "orders",
		EventType:  "order.created",
		SourceType: "order",
		SourceID:   row.OrderID,
		Actor:      support.ActorOf(c),
		Title:      "新订单 " + strings.TrimSpace(row.OrderNo),
		Body:       "客户履约代发订单已提交",
		Tone:       "success",
		Payload: map[string]any{
			"order_id":            row.OrderID,
			"order_no":            row.OrderNo,
			"portal_service_code": "direct_ship",
			"orders_scope":        "fulfillment",
			"highlight_order_id":  row.OrderID,
		},
		Deliveries: []messagecenterapp.DeliveryCommand{{
			Channel:    messagecenterapp.ChannelERPPlatform,
			TargetType: "permission",
			TargetKey:  "orders.read",
		}},
	})
}

func (a api) submitInternalProcessingWorkOrder(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		ProductID          int64  `json:"product_id"`
		ProductName        string `json:"product_name"`
		RawBeanItemID      int64  `json:"raw_bean_item_id"`
		RawBeanName        string `json:"raw_bean_name"`
		InputQuantityG     int64  `json:"input_quantity_g"`
		PlannedOutputUnits int64  `json:"planned_output_units"`
		ExpectedDate       string `json:"expected_date"`
		Note               string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.SubmitCustomerProcessingWorkOrder(c.Request().Context(), app.SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         support.CurrentEmployeeID(c),
		CustomerID:         customerID,
		ProductID:          req.ProductID,
		ProductName:        req.ProductName,
		RawBeanItemID:      req.RawBeanItemID,
		RawBeanName:        req.RawBeanName,
		InputQuantityG:     req.InputQuantityG,
		PlannedOutputUnits: req.PlannedOutputUnits,
		ExpectedDate:       req.ExpectedDate,
		Note:               req.Note,
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) submitInternalDirectShipOrder(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		ReceiverName    string  `json:"receiver_name"`
		ReceiverPhone   string  `json:"receiver_phone"`
		ReceiverAddress string  `json:"receiver_address"`
		ReceiverCompany string  `json:"receiver_company"`
		ShippingAmount  float64 `json:"shipping_amount"`
		ProductID       int64   `json:"product_id"`
		ProductName     string  `json:"product_name"`
		Spec            string  `json:"spec"`
		QuantityUnits   int64   `json:"quantity_units"`
		Items           []struct {
			ProductID                          int64   `json:"product_id"`
			BomSpecID                          int64   `json:"bom_spec_id"`
			BomVariantID                       int64   `json:"bom_variant_id"`
			BomSpecKey                         string  `json:"bom_spec_key"`
			BomSpecName                        string  `json:"bom_spec_name"`
			InventoryUnit                      string  `json:"inventory_unit"`
			CustomerProductAliasID             int64   `json:"customer_product_alias_id"`
			CustomerProductReferenceID         int64   `json:"customer_product_reference_id"`
			MaterialSourceMode                 string  `json:"material_source_mode"`
			CustomerProductDisplayNameSnapshot string  `json:"customer_product_display_name_snapshot"`
			CustomerItemCodeSnapshot           string  `json:"customer_item_code_snapshot"`
			ProductCodeSnapshot                string  `json:"product_code_snapshot"`
			ProductNameSnapshot                string  `json:"product_name_snapshot"`
			ProductName                        string  `json:"product_name"`
			Spec                               string  `json:"spec"`
			SpecG                              int64   `json:"spec_g"`
			SalesUnit                          string  `json:"sales_unit"`
			QuantityUnits                      int64   `json:"quantity_units"`
			DiscountType                       string  `json:"discount_type"`
			DiscountValue                      float64 `json:"discount_value"`
			Note                               string  `json:"note"`
		} `json:"items"`
		Note string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	items := make([]app.SubmitCustomerDirectShipOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, app.SubmitCustomerDirectShipOrderItem{
			ProductID:                          item.ProductID,
			BomSpecID:                          item.BomSpecID,
			BomVariantID:                       item.BomVariantID,
			BomSpecKey:                         item.BomSpecKey,
			BomSpecName:                        item.BomSpecName,
			InventoryUnit:                      item.InventoryUnit,
			CustomerProductAliasID:             item.CustomerProductAliasID,
			CustomerProductReferenceID:         item.CustomerProductReferenceID,
			MaterialSourceMode:                 item.MaterialSourceMode,
			CustomerProductDisplayNameSnapshot: item.CustomerProductDisplayNameSnapshot,
			CustomerItemCodeSnapshot:           item.CustomerItemCodeSnapshot,
			ProductCodeSnapshot:                item.ProductCodeSnapshot,
			ProductNameSnapshot:                item.ProductNameSnapshot,
			ProductName:                        item.ProductName,
			Spec:                               item.Spec,
			SpecG:                              item.SpecG,
			SalesUnit:                          item.SalesUnit,
			QuantityUnits:                      item.QuantityUnits,
			DiscountType:                       item.DiscountType,
			DiscountValue:                      item.DiscountValue,
			Note:                               item.Note,
		})
	}
	row, err := a.svc.SubmitCustomerDirectShipOrder(c.Request().Context(), app.SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      support.CurrentEmployeeID(c),
		CustomerID:      customerID,
		ReceiverName:    req.ReceiverName,
		ReceiverPhone:   req.ReceiverPhone,
		ReceiverAddress: req.ReceiverAddress,
		ReceiverCompany: req.ReceiverCompany,
		ShippingAmount:  req.ShippingAmount,
		ProductID:       req.ProductID,
		ProductName:     req.ProductName,
		Spec:            req.Spec,
		QuantityUnits:   req.QuantityUnits,
		Items:           items,
		Note:            req.Note,
		Actor:           currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) listImportRows(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	query := app.ListImportRowsQuery{
		BatchID: batchID,
		Status:  c.QueryParam("status"),
		Limit:   limit,
		Offset:  offset,
	}
	rows, err := a.svc.ListImportRows(c.Request().Context(), query)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"rows":   rows,
		"limit":  limit,
		"offset": offset,
	})
}

func (a api) importPreview(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	preview, err := a.svc.ImportPreview(c.Request().Context(), app.ImportPreviewQuery{BatchID: batchID})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, preview)
}

func (a api) overview(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	overview, err := a.svc.Overview(c.Request().Context(), app.OverviewQuery{CustomerID: customerID})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, overview)
}

func (a api) options(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	options, err := a.svc.CustomerFulfillmentOptions(c.Request().Context(), customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, options)
}

func (a api) listImports(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	imports, err := a.svc.ListImports(c.Request().Context(), app.ListImportsQuery{CustomerID: customerID, Limit: limit, Offset: offset})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"imports": imports})
}

func (a api) createSettlement(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		PeriodFrom string `json:"period_from"`
		PeriodTo   string `json:"period_to"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	req.PeriodFrom = strings.TrimSpace(req.PeriodFrom)
	req.PeriodTo = strings.TrimSpace(req.PeriodTo)
	if req.PeriodFrom == "" || req.PeriodTo == "" {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("period required"))
	}
	result, err := a.svc.CreateSettlement(c.Request().Context(), app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: req.PeriodFrom,
		PeriodTo:   req.PeriodTo,
		CreatedBy:  currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, result)
}

func (a api) adjustCustodyInventory(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		ItemType           string `json:"item_type"`
		ItemName           string `json:"item_name"`
		Spec               string `json:"spec"`
		QuantityGDelta     int64  `json:"quantity_g_delta"`
		QuantityUnitsDelta int64  `json:"quantity_units_delta"`
		Note               string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.AdjustCustodyInventory(c.Request().Context(), app.AdjustCustodyInventoryCommand{
		CustomerID:         customerID,
		ItemType:           req.ItemType,
		ItemName:           req.ItemName,
		Spec:               req.Spec,
		QuantityGDelta:     req.QuantityGDelta,
		QuantityUnitsDelta: req.QuantityUnitsDelta,
		Note:               req.Note,
		Actor:              currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) listERPBindings(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	rows, err := a.svc.ListCustomerERPBindings(c.Request().Context(), customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"bindings": rows})
}

func (a api) upsertERPBinding(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		EmployeeID int64  `json:"employee_id"`
		Role       string `json:"role"`
		Status     string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.UpsertCustomerERPBinding(c.Request().Context(), app.UpsertCustomerERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: req.EmployeeID,
		Role:       req.Role,
		Status:     req.Status,
		Actor:      currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) listExternalUsers(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	rows, err := a.svc.ListExternalUsers(c.Request().Context(), customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"users": rows})
}

func (a api) CreateExternalUser(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.CreateExternalUser(c.Request().Context(), app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       req.Name,
		Phone:      req.Phone,
		Password:   req.Password,
		Actor:      currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) ResetExternalUserPassword(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	employeeID, err := parseID(c.Param("employee_id"), "employee")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.ResetExternalUserPassword(c.Request().Context(), app.ResetExternalUserPasswordCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Password:   req.Password,
		Actor:      currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) SetExternalUserLoginEnabled(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	employeeID, err := parseID(c.Param("employee_id"), "employee")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		LoginEnabled bool `json:"login_enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	row, err := a.svc.SetExternalUserLoginEnabled(c.Request().Context(), app.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   employeeID,
		LoginEnabled: req.LoginEnabled,
		Actor:        currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func importBatchResponse(batch app.ImportBatch) map[string]any {
	return map[string]any{
		"batch":              batch,
		"batch_id":           batch.ID,
		"customer_id":        batch.CustomerID,
		"import_type":        batch.ImportType,
		"source_filename":    batch.SourceFilename,
		"status":             batch.Status,
		"summary":            batch.Summary,
		"total_rows":         batch.Summary.TotalRows,
		"valid_rows":         batch.Summary.ValidRows,
		"invalid_rows":       batch.Summary.InvalidRows,
		"direct_ship_orders": batch.Summary.DirectShipOrders,
		"processing_orders":  batch.Summary.ProcessingOrders,
		"fee_items":          batch.Summary.FeeItems,
	}
}

func parseID(value, name string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s id invalid", name)
	}
	return id, nil
}

func currentCustomerFulfillmentActor(c echo.Context) string {
	return support.ActorOf(c)
}

func customerFulfillmentError(c echo.Context, status int, err error) error {
	return c.JSON(status, map[string]any{"error": err.Error()})
}

func customerPortalError(c echo.Context, err error) error {
	if err == app.ErrCustomerERPBindingNotFound {
		return customerFulfillmentError(c, http.StatusForbidden, err)
	}
	return customerFulfillmentError(c, http.StatusBadRequest, err)
}
