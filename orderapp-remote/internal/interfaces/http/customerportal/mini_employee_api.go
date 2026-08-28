package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	catalogdomain "orderapp/internal/domain/catalog"

	"github.com/labstack/echo/v4"
)

type miniEmployeeOrderDetailResponse struct {
	CanEdit         bool                          `json:"can_edit"`
	EditBlockReason string                        `json:"edit_block_reason"`
	Order           miniEmployeeOrderDetailDTO    `json:"order"`
	Documents       miniEmployeeOrderDocumentsDTO `json:"documents"`
}

type miniEmployeeOrderDetailDTO struct {
	ID                              int64                             `json:"id"`
	EditRevision                    string                            `json:"edit_revision"`
	OrderNo                         string                            `json:"order_no"`
	DocumentDate                    string                            `json:"document_date"`
	OrderDate                       string                            `json:"order_date"`
	CustomerID                      int64                             `json:"customer_id"`
	Customer                        string                            `json:"customer"`
	SourceID                        int64                             `json:"source_id"`
	Source                          string                            `json:"source"`
	OrderTypeID                     int64                             `json:"order_type_id"`
	OrderType                       string                            `json:"order_type"`
	PayStatusID                     int64                             `json:"pay_status_id"`
	PayStatus                       string                            `json:"pay_status"`
	PaymentMethod                   string                            `json:"payment_method"`
	ShipStatusID                    int64                             `json:"ship_status_id"`
	ShipStatus                      string                            `json:"ship_status"`
	ProcessStatusID                 int64                             `json:"process_status_id"`
	ProcessStatus                   string                            `json:"process_status"`
	ShipMethod                      string                            `json:"ship_method"`
	ShipTrackingNo                  string                            `json:"ship_tracking_no"`
	LogisticsCompanyID              int64                             `json:"logistics_company_id"`
	LogisticsCompany                string                            `json:"logistics_company"`
	LogisticsProductID              int64                             `json:"logistics_product_id"`
	LogisticsProduct                string                            `json:"logistics_product"`
	SenderID                        int64                             `json:"sender_id"`
	SenderLabel                     string                            `json:"sender_label"`
	SenderName                      string                            `json:"sender_name"`
	PaymentGoodsAmount              string                            `json:"payment_goods_amount"`
	PaymentShippingAmount           string                            `json:"payment_shipping_amount"`
	PaymentVoucherAssetID           int64                             `json:"payment_voucher_asset_id"`
	PaymentVoucher                  *miniEmployeeOrderAssetDTO        `json:"payment_voucher,omitempty"`
	ResponsibleType                 string                            `json:"responsible_type"`
	ResponsibleID                   int64                             `json:"responsible_id"`
	ResponsibleName                 string                            `json:"responsible_name"`
	ReceiverName                    string                            `json:"receiver_name"`
	ReceiverPhone                   string                            `json:"receiver_phone"`
	ReceiverAddress                 string                            `json:"receiver_address"`
	ReceiverCompany                 string                            `json:"receiver_company"`
	PortalServiceCode               string                            `json:"portal_service_code"`
	SourceWarehouse                 string                            `json:"source_warehouse"`
	BeanListPublicationID           int64                             `json:"bean_list_publication_id"`
	CommercialBeanListPublicationID int64                             `json:"commercial_bean_list_publication_id"`
	GreenBeanListPublicationID      int64                             `json:"green_bean_list_publication_id"`
	DripBeanListPublicationID       int64                             `json:"drip_bean_list_publication_id"`
	BeanListVersionNo               string                            `json:"bean_list_version_no"`
	Notes                           string                            `json:"notes"`
	TotalAmount                     string                            `json:"total_amount"`
	ShippingAmount                  string                            `json:"shipping_amount"`
	DiscountAmount                  string                            `json:"discount_amount"`
	OrderDiscountAmount             string                            `json:"order_discount_amount"`
	RoundToInt                      bool                              `json:"round_to_int"`
	RoundingAmount                  string                            `json:"rounding_amount"`
	GrandTotal                      string                            `json:"grand_total"`
	ExpressFee                      string                            `json:"express_fee"`
	OutsourceMaterialFee            string                            `json:"outsource_material_fee"`
	OutsourceRoastFee               string                            `json:"outsource_roast_fee"`
	OutsourcePackagingFee           string                            `json:"outsource_packaging_fee"`
	OutsourceManualFee              string                            `json:"outsource_manual_fee"`
	OutsourceTaxFee                 string                            `json:"outsource_tax_fee"`
	OutsourceOtherFee               string                            `json:"outsource_other_fee"`
	OutsourceTotalFee               string                            `json:"outsource_total_fee"`
	ProductKindSummary              string                            `json:"product_kind_summary"`
	CreatedByEmployee               string                            `json:"created_by_employee"`
	IsVoid                          bool                              `json:"is_void"`
	VoidedAt                        *string                           `json:"voided_at,omitempty"`
	VoidReason                      *string                           `json:"void_reason,omitempty"`
	InvoiceStatus                   string                            `json:"invoice_status"`
	InvoiceFilename                 string                            `json:"invoice_filename"`
	InvoiceFileURL                  string                            `json:"invoice_file_url"`
	Items                           []miniEmployeeOrderItemDetailDTO  `json:"items"`
	QuoteSourceTrace                []miniEmployeeQuoteSourceTraceDTO `json:"quote_source_trace"`
	ProductionSourceTrace           []miniEmployeeProductionTraceDTO  `json:"production_source_trace"`
}

type miniEmployeeOrderAssetDTO struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	URL         string `json:"url"`
}

type miniEmployeeOrderItemDetailDTO struct {
	ItemID                             int64  `json:"item_id"`
	LineNo                             int    `json:"line_no"`
	ProductID                          int64  `json:"product_id"`
	ParentProductID                    int64  `json:"parent_product_id,omitempty"`
	MigrationState                     string `json:"migration_state,omitempty"`
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
	LineTotal                          string `json:"line_total"`
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

type miniEmployeeQuoteSourceTraceDTO struct {
	ProductID              int64   `json:"product_id"`
	ProductName            string  `json:"product_name"`
	PriceListPublicationID int64   `json:"price_list_publication_id"`
	PriceListVersion       string  `json:"price_list_version"`
	TierLabel              string  `json:"tier_label"`
	PriceUnit              string  `json:"price_unit"`
	FinalUnitPrice         float64 `json:"final_unit_price"`
	PricingRuleVersion     string  `json:"pricing_rule_version"`
	ManualAdjusted         bool    `json:"manual_adjusted"`
	SourceLabel            string  `json:"source_label"`
}

type miniEmployeeProductionTraceDTO struct {
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	SourceLabel      string `json:"source_label"`
	WorkOrderNo      string `json:"work_order_no"`
	BOMVersionNo     string `json:"bom_version_no,omitempty"`
	BOMVersionID     string `json:"bom_version_id,omitempty"`
	ProcessRouteName string `json:"process_route_name,omitempty"`
	ProcessCardNo    string `json:"process_card_no,omitempty"`
	MaterialBatchNo  string `json:"material_batch_no,omitempty"`
}

type miniEmployeeOrderDocumentsDTO struct {
	SalesOrderPDF   miniEmployeeOrderDocumentDTO `json:"sales_order_pdf"`
	SalesOrderPNG   miniEmployeeOrderDocumentDTO `json:"sales_order_png"`
	DeliveryNotePDF miniEmployeeOrderDocumentDTO `json:"delivery_note_pdf"`
	DeliveryNotePNG miniEmployeeOrderDocumentDTO `json:"delivery_note_png"`
}

type miniEmployeeOrderDocumentDTO struct {
	Available   bool   `json:"available"`
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename,omitempty"`
	VersionNo   int    `json:"version_no,omitempty"`
}

type miniEmployeeOrderDocumentMutationResponse struct {
	Document miniEmployeeGeneratedDocumentDTO `json:"document"`
}

type miniEmployeeGeneratedDocumentDTO struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename,omitempty"`
	VersionNo   int    `json:"version_no,omitempty"`
	Generated   bool   `json:"generated"`
}

type miniEmployeeDocumentMetadata struct {
	Filename  string
	VersionNo int
}

type miniEmployeeOrderItemRequest struct {
	ItemID                 int64   `json:"item_id"`
	ProductID              int64   `json:"product_id"`
	ParentProductID        int64   `json:"parent_product_id"`
	BomSpecID              int64   `json:"bom_spec_id"`
	BomVariantID           int64   `json:"bom_variant_id"`
	CustomerProductAliasID int64   `json:"customer_product_alias_id"`
	BeanListPublicationID  int64   `json:"bean_list_publication_id"`
	BeanListVersionNo      string  `json:"bean_list_version_no"`
	PriceSourceJSON        string  `json:"price_source_json"`
	PriceOverride          bool    `json:"price_override"`
	Name                   string  `json:"name"`
	Qty                    int64   `json:"qty"`
	Unit                   string  `json:"unit"`
	SpecG                  int64   `json:"spec_g"`
	ProductKind            string  `json:"product_kind"`
	SalesUnit              string  `json:"sales_unit"`
	UnitBagCount           int64   `json:"unit_bag_count"`
	UnitBeanG              float64 `json:"unit_bean_g"`
	UnitPrice              float64 `json:"unit_price"`
}

type miniEmployeeOrderRequest struct {
	EditRevision    string                         `json:"edit_revision"`
	OrderDate       string                         `json:"order_date"`
	CustomerID      int64                          `json:"customer_id"`
	SourceID        int64                          `json:"source_id"`
	OrderTypeID     int64                          `json:"order_type_id"`
	PayStatusID     int64                          `json:"pay_status_id"`
	PaymentMethod   string                         `json:"payment_method"`
	ShipStatusID    int64                          `json:"ship_status_id"`
	ShipMethod      string                         `json:"ship_method"`
	ShipTrackingNo  string                         `json:"ship_tracking_no"`
	ShippingAmount  float64                        `json:"shipping_amount"`
	DiscountAmount  float64                        `json:"discount_amount"`
	ReceiverName    string                         `json:"receiver_name"`
	ReceiverPhone   string                         `json:"receiver_phone"`
	ReceiverAddress string                         `json:"receiver_address"`
	ReceiverCompany string                         `json:"receiver_company"`
	Notes           string                         `json:"notes"`
	Items           []miniEmployeeOrderItemRequest `json:"items"`
}

type miniEmployeeCustomerRequest struct {
	Name                  string `json:"name"`
	CustomerType          string `json:"customer_type"`
	CompanyName           string `json:"company_name"`
	CompanyAddress        string `json:"company_address"`
	CompanyPhone          string `json:"company_phone"`
	Contact               string `json:"contact"`
	Phone                 string `json:"phone"`
	Address               string `json:"address"`
	DefaultSourceID       int64  `json:"default_source_id"`
	DefaultOrderTypeID    int64  `json:"default_order_type_id"`
	ResponsibleEmployeeID int64  `json:"responsible_employee_id"`
	Active                *bool  `json:"active"`
	PortalEnabled         *bool  `json:"portal_enabled"`
	CapabilityTemplateKey string `json:"capability_template_key"`
}

type miniEmployeeOrderDraftRequest struct {
	Payload json.RawMessage `json:"payload"`
}

func registerMiniEmployeeAPI(e *echo.Echo, portal Service, sales EmployeeSales, customerServices ...CustomerMaintenance) {
	var customers CustomerMaintenance
	if len(customerServices) > 0 {
		customers = customerServices[0]
	}
	orders := miniEmployeeOrderHandler{portal: portal, sales: sales}
	e.GET("/api/mini/employee/order-form", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		form, err := sales.OrderForm(c.Request().Context(), 0)
		if err != nil {
			return miniInternalError(c)
		}
		customerID := int64(0)
		retailOrder := false
		if rawCustomerID := strings.TrimSpace(c.QueryParam("customer_id")); rawCustomerID != "" {
			customerID, err = strconv.ParseInt(rawCustomerID, 10, 64)
			if err != nil || customerID <= 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "客户编号不正确"})
			}
			customer, found := miniEmployeeOrderCustomer(form, customerID)
			if !found {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "客户不存在"})
			}
			retailOrder = miniEmployeeUsesRetailCatalog(form, customer, 0)
		}
		customers := make([]map[string]any, 0, len(form.Customers))
		for _, customer := range form.Customers {
			canMaintain := containsMiniRole(employee.Permissions, "customers.read") &&
				containsMiniRole(employee.Permissions, "customers.write") &&
				(containsMiniRole(employee.Roles, "admin") || customer.ResponsibleEmployeeID == employee.EmployeeID)
			customers = append(customers, map[string]any{
				"id": customer.ID, "name": customer.Name, "customer_type": customer.CustomerType,
				"py": catalogdomain.SearchPinyin(customer.Name), "pyi": catalogdomain.SearchInitials(customer.Name),
				"default_source_id": customer.DefaultSourceID, "default_order_type_id": customer.DefaultOrderTypeID,
				"receiver_name":             firstMiniOrderValue(customer.Contact, customer.Name),
				"receiver_phone":            firstMiniOrderValue(customer.Phone, customer.CompanyPhone),
				"receiver_address":          firstMiniOrderValue(customer.Address, customer.CompanyAddress),
				"receiver_company":          firstMiniOrderValue(customer.CompanyName, customer.Name),
				"responsible_employee_id":   customer.ResponsibleEmployeeID,
				"responsible_employee_name": customer.ResponsibleEmployeeName,
				"can_maintain":              canMaintain,
			})
		}
		catalog := miniEmployeeOrderCatalogResult{Products: []salesapp.ProductOption{}, Families: []map[string]any{}, BOMSpecOptions: []salesapp.ProductBOMSpecOption{}}
		if customerID > 0 {
			catalog = miniEmployeeOrderCatalog(form, customerID, retailOrder)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"today": form.Today, "customers": customers, "sources": form.Sources,
			"order_types": form.OrderTypes, "pay_statuses": form.PayStatuses,
			"ship_statuses": form.ShipStatuses, "products": catalog.Products, "product_families": catalog.Families,
			"product_bom_spec_options": catalog.BOMSpecOptions,
		})
	})

	e.GET("/api/mini/employee/customers", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "customers.read")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if customers == nil {
			return miniInternalError(c)
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		if limit <= 0 {
			limit = 200
		}
		page, _ := strconv.Atoi(c.QueryParam("page"))
		if page <= 0 {
			page = 1
		}
		result, err := customers.ListManaged(c.Request().Context(), miniCustomerPrincipal(employee), customerapp.ListQuery{
			Query: strings.TrimSpace(c.QueryParam("q")), Limit: limit, Offset: (page - 1) * limit,
			CustomerType: strings.TrimSpace(c.QueryParam("customer_type")),
		})
		if err != nil {
			return miniCustomerError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"rows": result.Rows, "sources": miniCustomerOptions(result.Sources), "order_types": miniCustomerOptions(result.OrderTypes),
			"employees": miniCustomerOptions(result.Employees), "customer_type_options": result.CustomerTypeOptions,
			"is_admin": containsMiniRole(employee.Roles, "admin"), "total": result.Total, "has_next": result.HasNext,
		})
	})

	e.GET("/api/mini/employee/customers/:id", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "customers.read")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if customers == nil {
			return miniInternalError(c)
		}
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "客户编号不正确"})
		}
		data, err := customers.EditorManaged(c.Request().Context(), miniCustomerPrincipal(employee), id)
		if err != nil {
			return miniCustomerError(c, err)
		}
		if data == nil {
			return miniCustomerError(c, customerapp.ErrCustomerNotFound)
		}
		return c.JSON(http.StatusOK, map[string]any{"customer": miniCustomerEditResponse(data.Customer)})
	})

	e.POST("/api/mini/employee/customers", func(c echo.Context) error {
		employee, err := requireMiniEmployeeCustomerWriter(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal)
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if customers == nil {
			return miniInternalError(c)
		}
		var req miniEmployeeCustomerRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}
		principal := miniCustomerPrincipal(employee)
		id, err := customers.UpsertManaged(c.Request().Context(), principal, nil, miniCustomerUpsertCommand(req, nil))
		if err != nil {
			return miniCustomerError(c, err)
		}
		response, err := miniCustomerResponseAfterWrite(c.Request().Context(), customers, principal, id, req, nil)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusCreated, map[string]any{"customer": response})
	})

	e.PUT("/api/mini/employee/customers/:id", func(c echo.Context) error {
		employee, err := requireMiniEmployeeCustomerWriter(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal)
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if customers == nil {
			return miniInternalError(c)
		}
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "客户编号不正确"})
		}
		var req miniEmployeeCustomerRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}
		principal := miniCustomerPrincipal(employee)
		existing, err := customers.EditorManaged(c.Request().Context(), principal, id)
		if err != nil {
			return miniCustomerError(c, err)
		}
		if existing == nil {
			return miniCustomerError(c, customerapp.ErrCustomerNotFound)
		}
		if _, err := customers.UpsertManaged(c.Request().Context(), principal, &id, miniCustomerUpsertCommand(req, &existing.Customer)); err != nil {
			return miniCustomerError(c, err)
		}
		response, err := miniCustomerResponseAfterWrite(c.Request().Context(), customers, principal, id, req, &existing.Customer)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, map[string]any{"customer": response})
	})

	e.GET("/api/mini/employee/order-draft", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		draft, err := sales.GetEmployeeOrderDraft(c.Request().Context(), employee.EmployeeID)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, map[string]any{"draft": draft})
	})

	e.PUT("/api/mini/employee/order-draft", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		var req miniEmployeeOrderDraftRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}
		draft, err := sales.SaveEmployeeOrderDraft(c.Request().Context(), salesapp.SaveEmployeeOrderDraftCommand{
			EmployeeID: employee.EmployeeID, Actor: miniEmployeeActor(employee), Payload: req.Payload,
		})
		if err != nil {
			if message, ok := salesapp.EmployeeOrderDraftValidationMessage(err); ok {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
			}
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, map[string]any{"draft": draft})
	})

	e.DELETE("/api/mini/employee/order-draft", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		if _, err := sales.DeleteEmployeeOrderDraft(c.Request().Context(), employee.EmployeeID, miniEmployeeActor(employee)); err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, map[string]any{"deleted": true})
	})

	e.GET("/api/mini/employee/orders", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.read")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		if limit <= 0 {
			limit = 30
		}
		query := salesapp.OrderListQuery{Q: strings.TrimSpace(c.QueryParam("q")), Void: "normal", Limit: limit}
		if !containsMiniRole(employee.Roles, "admin") {
			query.Scope = "mine"
			query.EmployeeID = employee.EmployeeID
		}
		result, err := sales.ListOrders(c.Request().Context(), query)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": result.Rows, "summary": result.Summary, "has_next": result.HasNext})
	})

	e.GET("/api/mini/employee/orders/:id", orders.detail)
	e.GET("/api/mini/employee/orders/:id/documents/sales-order.pdf", orders.downloadSalesOrderPDF)
	e.GET("/api/mini/employee/orders/:id/documents/sales-order.png", orders.downloadSalesOrderPNG)
	e.GET("/api/mini/employee/orders/:id/documents/delivery-note.pdf", orders.downloadDeliveryNotePDF)
	e.GET("/api/mini/employee/orders/:id/documents/delivery-note.png", orders.downloadDeliveryNotePNG)
	e.POST("/api/mini/employee/orders/:id/documents/sales-order.pdf", orders.generateSalesOrderPDF)
	e.POST("/api/mini/employee/orders/:id/documents/sales-order.png", orders.generateSalesOrderPNG)
	e.POST("/api/mini/employee/orders/:id/documents/delivery-note.pdf", orders.generateDeliveryNotePDF)
	e.POST("/api/mini/employee/orders/:id/documents/delivery-note.png", orders.generateDeliveryNotePNG)
	e.PUT("/api/mini/employee/orders/:id", orders.update)

	e.POST("/api/mini/employee/orders", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if sales == nil {
			return miniInternalError(c)
		}
		var req miniEmployeeOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		}
		cmd, err := miniEmployeeSaveOrderCommand(req, miniEmployeeActor(employee), 0)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单日期不正确"})
		}
		if message, err := miniEmployeePrepareCurrentCatalog(c.Request().Context(), sales, &cmd); err != nil {
			return miniInternalError(c)
		} else if message != "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
		}
		cmd.DraftEmployeeID = employee.EmployeeID
		cmd.RequireCurrentDefaultPublications = true
		result, err := sales.SaveOrder(c.Request().Context(), cmd)
		if err != nil {
			if message, conflict := salesapp.OrderEditConflictMessage(err); conflict {
				return c.JSON(http.StatusConflict, map[string]string{"error": message})
			}
			if message, known := miniOrderKnownBusinessErrorText(err); known {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
			}
			return miniInternalError(c)
		}
		return c.JSON(http.StatusCreated, map[string]any{"order_id": result.OrderID, "order_no": result.OrderNo})
	})
}

type miniEmployeeOrderHandler struct {
	portal Service
	sales  EmployeeSales
}

type miniEmployeeOrderAccess struct {
	employee customerportalapp.CurrentContext
	orderID  int64
	row      salesapp.OrderRow
}

func miniEmployeeSaveOrderCommand(req miniEmployeeOrderRequest, actor string, editID int64) (salesapp.SaveOrderCommand, error) {
	orderDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.OrderDate))
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	items := make([]salesapp.OrderItemCommand, 0, len(req.Items))
	for _, item := range req.Items {
		productID := item.ProductID
		command := salesapp.OrderItemCommand{
			ItemID:    item.ItemID,
			ProductID: &productID, ParentProductID: item.ParentProductID,
			BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID,
			CustomerProductAliasID: item.CustomerProductAliasID,
			BeanListPublicationID:  item.BeanListPublicationID,
			BeanListVersionNo:      strings.TrimSpace(item.BeanListVersionNo),
			PriceSourceJSON:        strings.TrimSpace(item.PriceSourceJSON),
			Name:                   strings.TrimSpace(item.Name),
			Units:                  item.Qty,
			Unit:                   strings.TrimSpace(item.Unit),
			SpecG:                  item.SpecG,
			ProductKind:            strings.TrimSpace(item.ProductKind),
			SalesUnit:              strings.TrimSpace(item.SalesUnit),
			UnitBagCount:           item.UnitBagCount,
			UnitBeanG:              item.UnitBeanG,
		}
		if item.PriceOverride {
			price := item.UnitPrice
			command.ManualPrice = &price
		}
		items = append(items, command)
	}
	return salesapp.SaveOrderCommand{
		Actor: actor, EditID: editID, DocumentDate: orderDate, OrderDate: orderDate,
		CustomerID: req.CustomerID, SourceID: req.SourceID, OrderTypeID: req.OrderTypeID,
		PayStatusID: req.PayStatusID, PaymentMethod: strings.TrimSpace(req.PaymentMethod),
		ShipStatusID: req.ShipStatusID, ShipMethod: strings.TrimSpace(req.ShipMethod), ShipTrackingNo: strings.TrimSpace(req.ShipTrackingNo),
		ShippingAmount: req.ShippingAmount, DiscountAmount: req.DiscountAmount,
		ReceiverName: strings.TrimSpace(req.ReceiverName), ReceiverPhone: strings.TrimSpace(req.ReceiverPhone),
		ReceiverAddress: strings.TrimSpace(req.ReceiverAddress), ReceiverCompany: strings.TrimSpace(req.ReceiverCompany),
		Notes: strings.TrimSpace(req.Notes), Items: items,
	}, nil
}

func miniEmployeeOrderCustomer(form salesapp.OrderFormData, customerID int64) (salesapp.CustomerOption, bool) {
	for _, customer := range form.Customers {
		if customer.ID == customerID {
			return customer, true
		}
	}
	return salesapp.CustomerOption{}, false
}

func miniEmployeeUsesRetailCatalog(form salesapp.OrderFormData, customer salesapp.CustomerOption, fallbackOrderTypeID int64) bool {
	orderTypeID := customer.DefaultOrderTypeID
	if orderTypeID <= 0 {
		orderTypeID = fallbackOrderTypeID
	}
	for _, orderType := range form.OrderTypes {
		if orderType.ID != orderTypeID {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(orderType.Name))
		return strings.Contains(name, "零售") || strings.Contains(name, "retail")
	}
	return false
}

func miniEmployeePrepareCurrentCatalog(ctx context.Context, sales EmployeeSales, cmd *salesapp.SaveOrderCommand) (string, error) {
	form, err := sales.OrderForm(ctx, 0)
	if err != nil {
		return "", err
	}
	return miniEmployeePrepareCurrentCatalogFromForm(form, cmd)
}

func miniEmployeePrepareCurrentCatalogFromForm(form salesapp.OrderFormData, cmd *salesapp.SaveOrderCommand) (string, error) {
	customer, found := miniEmployeeOrderCustomer(form, cmd.CustomerID)
	if !found {
		return "客户不存在", nil
	}
	retailOrder := miniEmployeeUsesRetailCatalog(form, customer, cmd.OrderTypeID)
	products := salesapp.FilterOrderProductsForDefaultPublications(form.Products, cmd.CustomerID, form.BeanListVersionOptions, form.CustomerPublicUsages, retailOrder)
	cutoverOptions := make([]salesapp.ProductBOMSpecOption, 0)
	for _, option := range form.ProductBOMSpecOptions {
		if option.MigrationState == "cutover" && option.Published && option.ParentProductID > 0 && option.BomSpecID > 0 && option.BomVariantID > 0 {
			cutoverOptions = append(cutoverOptions, option)
		}
	}
	cutoverProducts, cutoverOptionBySpec := miniEmployeeBOMSpecFilterProducts(form.Products, cutoverOptions)
	cutoverProducts = salesapp.FilterOrderProductsForDefaultPublications(cutoverProducts, cmd.CustomerID, form.BeanListVersionOptions, form.CustomerPublicUsages, retailOrder)
	for index := range cmd.Items {
		item := &cmd.Items[index]
		if item.ProductID == nil || *item.ProductID <= 0 {
			return "请选择该客户当前价格表中的商品规格", nil
		}
		if item.BomSpecID > 0 || item.BomVariantID > 0 {
			option, optionFound := cutoverOptionBySpec[item.BomSpecID]
			product, productFound := miniEmployeeCurrentBOMSpecCatalogProduct(cutoverProducts, *item.ProductID, item.BomSpecID, item.BomVariantID, item.CustomerProductAliasID)
			if !optionFound || !productFound || option.ParentProductID != *item.ProductID || option.BomVariantID != item.BomVariantID {
				return "所选商品或BOM规格不在该客户当前价格表中，请重新选择", nil
			}
			tier, found := miniEmployeeCurrentCatalogTier(product.Tiers, item.BeanListPublicationID, item.BeanListVersionNo, item.PriceSourceJSON)
			if !found {
				return "所选商品或BOM规格不在该客户当前价格表中，请重新选择", nil
			}
			item.ParentProductID = option.ParentProductID
			item.BomSpecID = option.BomSpecID
			item.BomVariantID = option.BomVariantID
			item.CustomerProductAliasID = product.CustomerProductAliasID
			item.BeanListPublicationID = tier.PublicationID
			item.BeanListVersionNo = tier.PublicationVersionNo
			item.PriceSourceJSON = tier.PriceSourceJSON
			item.ProductKind = product.ProductKind
			item.ProductCodeSnapshot = option.SpecCode
			item.SpecG = 0
			item.Unit = option.InventoryUnit
			item.SalesUnit = option.InventoryUnit
			continue
		}
		product, found := miniEmployeeCurrentCatalogProduct(products, *item.ProductID, item.CustomerProductAliasID)
		if !found {
			return "所选商品或规格不在该客户当前价格表中，请重新选择", nil
		}
		if item.ParentProductID > 0 && item.ParentProductID != miniEmployeeCatalogParentProductID(product) {
			return "所选商品规格与当前价格表不一致，请重新选择", nil
		}
		if item.CustomerProductAliasID > 0 && item.CustomerProductAliasID != product.CustomerProductAliasID {
			return "所选客户商品与当前价格表不一致，请重新选择", nil
		}
		tier, found := miniEmployeeCurrentCatalogTier(product.Tiers, item.BeanListPublicationID, item.BeanListVersionNo, item.PriceSourceJSON)
		if !found {
			return "所选商品或规格不在该客户当前价格表中，请重新选择", nil
		}
		item.ParentProductID = miniEmployeeCatalogParentProductID(product)
		item.CustomerProductAliasID = product.CustomerProductAliasID
		item.BeanListPublicationID = tier.PublicationID
		item.BeanListVersionNo = tier.PublicationVersionNo
		item.PriceSourceJSON = tier.PriceSourceJSON
		item.ProductKind = product.ProductKind
	}
	return "", nil
}

func miniEmployeeCurrentBOMSpecCatalogProduct(products []salesapp.ProductOption, parentProductID, bomSpecID, bomVariantID, aliasID int64) (salesapp.ProductOption, bool) {
	var fallback *salesapp.ProductOption
	for index := range products {
		product := products[index]
		if product.ID != parentProductID || product.SKUID != bomSpecID {
			continue
		}
		variantMatches := false
		for _, tier := range product.Tiers {
			if tier.BomVariantID == bomVariantID {
				variantMatches = true
				break
			}
		}
		if !variantMatches {
			continue
		}
		if aliasID > 0 {
			if product.CustomerProductAliasID == aliasID {
				return product, true
			}
			continue
		}
		if product.CustomerProductAliasID == 0 {
			return product, true
		}
		if fallback == nil {
			copyProduct := product
			fallback = &copyProduct
		}
	}
	if aliasID <= 0 && fallback != nil {
		return *fallback, true
	}
	return salesapp.ProductOption{}, false
}

func miniEmployeeCatalogParentProductID(product salesapp.ProductOption) int64 {
	if product.ParentProductID > 0 {
		return product.ParentProductID
	}
	return product.ID
}

func miniEmployeeCurrentCatalogProduct(products []salesapp.ProductOption, productID, aliasID int64) (salesapp.ProductOption, bool) {
	var fallback *salesapp.ProductOption
	aliasCandidateCount := 0
	for index := range products {
		product := products[index]
		if product.ID != productID {
			continue
		}
		if aliasID > 0 {
			if product.CustomerProductAliasID == aliasID {
				return product, true
			}
			continue
		}
		if product.CustomerProductAliasID == 0 {
			return product, true
		}
		aliasCandidateCount++
		if fallback == nil {
			copyProduct := product
			fallback = &copyProduct
		}
	}
	if fallback != nil && aliasCandidateCount == 1 {
		return *fallback, true
	}
	return salesapp.ProductOption{}, false
}

func miniEmployeeCurrentCatalogTier(tiers []salesapp.ProductTierOption, publicationID int64, versionNo, sourceJSON string) (salesapp.ProductTierOption, bool) {
	versionNo = strings.TrimSpace(versionNo)
	sourcePublicationID := miniEmployeePriceSourcePublicationID(sourceJSON)
	for _, tier := range tiers {
		if publicationID > 0 && tier.PublicationID != publicationID {
			continue
		}
		if sourcePublicationID > 0 && tier.PublicationID != sourcePublicationID {
			continue
		}
		if versionNo != "" && strings.TrimSpace(tier.PublicationVersionNo) != versionNo {
			continue
		}
		return tier, true
	}
	return salesapp.ProductTierOption{}, false
}

func miniEmployeePriceSourcePublicationID(raw string) int64 {
	var source map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &source) != nil {
		return 0
	}
	for _, key := range []string{"publication_id", "bean_list_publication_id"} {
		switch value := source[key].(type) {
		case float64:
			if value > 0 {
				return int64(value)
			}
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func miniEmployeePreserveHiddenOrderFields(cmd *salesapp.SaveOrderCommand, existing *salesapp.OrderEditData) error {
	if cmd == nil || existing == nil {
		return fmt.Errorf("existing order required")
	}
	paymentGoodsAmount, err := miniEmployeeStoredOrderAmount(existing.PaymentGoodsAmount)
	if err != nil {
		return err
	}
	paymentShippingAmount, err := miniEmployeeStoredOrderAmount(existing.PaymentShippingAmount)
	if err != nil {
		return err
	}
	materialFee, err := miniEmployeeStoredOrderAmount(existing.OutsourceMaterialFee)
	if err != nil {
		return err
	}
	roastFee, err := miniEmployeeStoredOrderAmount(existing.OutsourceRoastFee)
	if err != nil {
		return err
	}
	packagingFee, err := miniEmployeeStoredOrderAmount(existing.OutsourcePackagingFee)
	if err != nil {
		return err
	}
	manualFee, err := miniEmployeeStoredOrderAmount(existing.OutsourceManualFee)
	if err != nil {
		return err
	}
	taxFee, err := miniEmployeeStoredOrderAmount(existing.OutsourceTaxFee)
	if err != nil {
		return err
	}
	otherFee, err := miniEmployeeStoredOrderAmount(existing.OutsourceOtherFee)
	if err != nil {
		return err
	}
	beanListID, _, commercialID, greenID, dripID := miniEmployeeOrderPublicationFields(existing)
	documentDate := strings.TrimSpace(existing.DocumentDate)
	if documentDate == "" {
		documentDate = strings.TrimSpace(existing.OrderDate)
	}
	if documentDate != "" {
		parsedDocumentDate, err := time.Parse("2006-01-02", documentDate)
		if err != nil {
			return fmt.Errorf("invalid stored document date")
		}
		cmd.DocumentDate = parsedDocumentDate
	}
	cmd.SourceID = existing.SourceID
	cmd.OrderTypeID = existing.OrderTypeID
	cmd.PayStatusID = existing.PayStatusID
	cmd.PaymentMethod = existing.PaymentMethod
	cmd.ShipStatusID = existing.ShipStatusID
	cmd.ShipMethod = existing.ShipMethod
	cmd.ShipTrackingNo = existing.ShipTrackingNo
	cmd.LogisticsCompanyID = existing.LogisticsCompanyID
	cmd.LogisticsProductID = existing.LogisticsProductID
	cmd.PaymentGoodsAmount = paymentGoodsAmount
	cmd.PaymentShippingAmount = paymentShippingAmount
	cmd.PaymentVoucherAssetID = existing.PaymentVoucherAssetID
	cmd.ResponsibleType = existing.ResponsibleType
	cmd.ResponsibleID = existing.ResponsibleID
	cmd.ResponsibleName = existing.ResponsibleName
	cmd.RoundToInt = existing.RoundToInt
	cmd.ExpressFee = existing.ExpressFee
	cmd.OutsourceMaterialFee = materialFee
	cmd.OutsourceRoastFee = roastFee
	cmd.OutsourcePackagingFee = packagingFee
	cmd.OutsourceManualFee = manualFee
	cmd.OutsourceTaxFee = taxFee
	cmd.OutsourceOtherFee = otherFee
	cmd.BeanListPublicationID = beanListID
	cmd.CommercialBeanListPublicationID = commercialID
	cmd.GreenBeanListPublicationID = greenID
	cmd.DripBeanListPublicationID = dripID
	cmd.PortalServiceCode = existing.PortalServiceCode
	cmd.SourceWarehouse = existing.SourceWarehouse
	cmd.PreserveResponsibleSnapshot = true
	cmd.PreserveFulfillmentSnapshot = true
	return nil
}

func miniEmployeeStoredOrderAmount(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid stored order amount")
	}
	return value, nil
}

func miniEmployeePreserveHiddenOrderItemFields(requestItems []miniEmployeeOrderItemRequest, commandItems []salesapp.OrderItemCommand, existingItems []salesapp.OrderEditItem) error {
	matchedExisting := make([]bool, len(existingItems))
	matches := make([]int, len(commandItems))
	for index := range matches {
		matches[index] = -1
	}
	for index := range commandItems {
		if index >= len(requestItems) || requestItems[index].ItemID <= 0 {
			continue
		}
		for existingIndex := range existingItems {
			if !matchedExisting[existingIndex] && existingItems[existingIndex].ItemID == requestItems[index].ItemID {
				matches[index] = existingIndex
				matchedExisting[existingIndex] = true
				break
			}
		}
	}
	for index := range commandItems {
		if matches[index] >= 0 {
			continue
		}
		for existingIndex := range existingItems {
			if matchedExisting[existingIndex] || !miniEmployeeOrderItemIdentityMatches(commandItems[index], existingItems[existingIndex]) {
				continue
			}
			matches[index] = existingIndex
			matchedExisting[existingIndex] = true
			break
		}
	}
	if len(commandItems) == len(existingItems) {
		for index := range commandItems {
			if matches[index] >= 0 || matchedExisting[index] {
				continue
			}
			matches[index] = index
			matchedExisting[index] = true
		}
	}
	for index, existingIndex := range matches {
		if existingIndex < 0 {
			continue
		}
		existing := existingItems[existingIndex]
		discountValue, err := miniEmployeeStoredOrderAmount(existing.DiscountValue)
		if err != nil {
			return err
		}
		commandItems[index].Note = existing.Note
		commandItems[index].DiscountType = existing.DiscountType
		commandItems[index].DiscountValue = discountValue
		commandItems[index].CustomerProductDisplayNameSnapshot = existing.CustomerProductDisplayNameSnapshot
		commandItems[index].CustomerItemCodeSnapshot = existing.CustomerItemCodeSnapshot
		commandItems[index].BrandNameSnapshot = existing.BrandNameSnapshot
		commandItems[index].ProductCodeSnapshot = existing.ProductCodeSnapshot
		commandItems[index].ProductNameSnapshot = existing.ProductNameSnapshot
	}
	return nil
}

func miniEmployeeOrderItemIdentityMatches(command salesapp.OrderItemCommand, existing salesapp.OrderEditItem) bool {
	if command.ProductID == nil || *command.ProductID <= 0 || *command.ProductID != existing.ProductID {
		return false
	}
	if command.CustomerProductAliasID > 0 || existing.CustomerProductAliasID > 0 {
		if command.CustomerProductAliasID != existing.CustomerProductAliasID {
			return false
		}
	}
	if command.BomSpecID > 0 || existing.BomSpecID > 0 {
		if command.BomSpecID != existing.BomSpecID || command.BomVariantID != existing.BomVariantID {
			return false
		}
	}
	if command.BeanListPublicationID > 0 && existing.BeanListPublicationID > 0 && command.BeanListPublicationID != existing.BeanListPublicationID {
		return false
	}
	return true
}

func (h miniEmployeeOrderHandler) update(c echo.Context) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.write")
	if !ok {
		return err
	}
	var req miniEmployeeOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
	}
	if req.CustomerID != access.row.CustomerID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "编辑订单不能更换客户"})
	}
	form, err := h.sales.OrderForm(c.Request().Context(), access.orderID)
	if err != nil {
		return miniInternalError(c)
	}
	if form.EditData == nil || form.EditData.ID != access.orderID || form.EditData.CustomerID != access.row.CustomerID {
		return miniEmployeeOrderNotFound(c)
	}
	if currentRevision := strings.TrimSpace(form.EditData.EditRevision); currentRevision != "" && strings.TrimSpace(req.EditRevision) != currentRevision {
		return c.JSON(http.StatusConflict, map[string]string{"error": "订单已被其他操作修改，请重新打开后再编辑"})
	}
	cmd, err := miniEmployeeSaveOrderCommand(req, miniEmployeeActor(access.employee), access.orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单日期不正确"})
	}
	if err := miniEmployeePreserveHiddenOrderFields(&cmd, form.EditData); err != nil {
		return miniInternalError(c)
	}
	if message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd); err != nil {
		return miniInternalError(c)
	} else if message != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
	}
	if err := miniEmployeePreserveHiddenOrderItemFields(req.Items, cmd.Items, form.EditData.Items); err != nil {
		return miniInternalError(c)
	}
	cmd.RequirePreProductionEdit = true
	cmd.RequireCurrentDefaultPublications = true
	cmd.ExpectedEditRevision = strings.TrimSpace(req.EditRevision)
	result, err := h.sales.SaveOrder(c.Request().Context(), cmd)
	if err != nil {
		if message, conflict := salesapp.OrderEditConflictMessage(err); conflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": message})
		}
		if message, known := miniOrderKnownBusinessErrorText(err); known {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
		}
		return miniInternalError(c)
	}
	return c.JSON(http.StatusOK, map[string]any{"order_id": result.OrderID, "order_no": result.OrderNo})
}

func (h miniEmployeeOrderHandler) detail(c echo.Context) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.read")
	if !ok {
		return err
	}
	form, err := h.sales.OrderForm(c.Request().Context(), access.orderID)
	if err != nil {
		return miniInternalError(c)
	}
	if form.EditData == nil || form.EditData.ID != access.orderID {
		return miniEmployeeOrderNotFound(c)
	}
	editability := salesapp.OrderEditability{BlockReason: "当前账号没有编辑订单权限"}
	if containsMiniRole(access.employee.Permissions, "orders.write") {
		editability, err = h.sales.OrderEditability(c.Request().Context(), access.orderID)
		if err != nil {
			return miniInternalError(c)
		}
	}
	documents, err := h.documentSummary(c.Request().Context(), access.orderID)
	if err != nil {
		return miniInternalError(c)
	}
	return c.JSON(http.StatusOK, miniEmployeeOrderDetailResponse{
		CanEdit: editability.CanEdit, EditBlockReason: editability.BlockReason,
		Order: miniEmployeeOrderDetail(access.row, form), Documents: documents,
	})
}

func (h miniEmployeeOrderHandler) downloadSalesOrderPDF(c echo.Context) error {
	return h.downloadDocument(c, "sales-order.pdf")
}

func (h miniEmployeeOrderHandler) downloadSalesOrderPNG(c echo.Context) error {
	return h.downloadDocument(c, "sales-order.png")
}

func (h miniEmployeeOrderHandler) downloadDeliveryNotePDF(c echo.Context) error {
	return h.downloadDocument(c, "delivery-note.pdf")
}

func (h miniEmployeeOrderHandler) downloadDeliveryNotePNG(c echo.Context) error {
	return h.downloadDocument(c, "delivery-note.png")
}

func (h miniEmployeeOrderHandler) generateSalesOrderPDF(c echo.Context) error {
	return h.generateDocument(c, "sales-order.pdf")
}

func (h miniEmployeeOrderHandler) generateSalesOrderPNG(c echo.Context) error {
	return h.generateDocument(c, "sales-order.png")
}

func (h miniEmployeeOrderHandler) generateDeliveryNotePDF(c echo.Context) error {
	return h.generateDocument(c, "delivery-note.pdf")
}

func (h miniEmployeeOrderHandler) generateDeliveryNotePNG(c echo.Context) error {
	return h.generateDocument(c, "delivery-note.png")
}

func (h miniEmployeeOrderHandler) ensureMiniEmployeeOrderAccess(c echo.Context, permission string) (miniEmployeeOrderAccess, bool, error) {
	employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), h.portal, permission)
	if err != nil {
		return miniEmployeeOrderAccess{}, false, miniEmployeeAuthError(c, err)
	}
	if h.sales == nil {
		return miniEmployeeOrderAccess{}, false, miniInternalError(c)
	}
	orderID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || orderID <= 0 {
		return miniEmployeeOrderAccess{}, false, c.JSON(http.StatusBadRequest, map[string]string{"error": "订单编号不正确"})
	}
	query := salesapp.OrderListQuery{OrderID: orderID, Void: "all", Limit: 1}
	if !containsMiniRole(employee.Roles, "admin") {
		query.Scope = "mine"
		query.EmployeeID = employee.EmployeeID
	}
	result, err := h.sales.ListOrders(c.Request().Context(), query)
	if err != nil {
		return miniEmployeeOrderAccess{}, false, miniInternalError(c)
	}
	for _, row := range result.Rows {
		if row.ID == orderID {
			return miniEmployeeOrderAccess{employee: employee, orderID: orderID, row: row}, true, nil
		}
	}
	return miniEmployeeOrderAccess{}, false, miniEmployeeOrderNotFound(c)
}

func (h miniEmployeeOrderHandler) documentSummary(ctx context.Context, orderID int64) (miniEmployeeOrderDocumentsDTO, error) {
	salesDocuments, err := h.sales.ListSalesOrderDocuments(ctx, orderID)
	if err != nil {
		return miniEmployeeOrderDocumentsDTO{}, err
	}
	salesImages, err := h.sales.ListSalesOrderImageDocuments(ctx, orderID)
	if err != nil {
		return miniEmployeeOrderDocumentsDTO{}, err
	}
	deliveryDocuments, err := h.sales.ListDeliveryNoteDocuments(ctx, orderID)
	if err != nil {
		return miniEmployeeOrderDocumentsDTO{}, err
	}
	salesPDFAvailable := miniEmployeeLatestSalesOrderPDFAvailable(salesDocuments)
	salesPNGAvailable := miniEmployeeLatestSalesOrderPNGAvailable(salesImages)
	deliveryPDFAvailable, deliveryPNGAvailable := miniEmployeeLatestDeliveryNoteAvailability(deliveryDocuments)
	var salesPDFMetadata, salesPNGMetadata, deliveryPDFMetadata, deliveryPNGMetadata miniEmployeeDocumentMetadata
	if salesPDFAvailable {
		salesPDFMetadata, salesPDFAvailable = h.latestDocumentMetadata(ctx, orderID, "sales-order.pdf")
	}
	if salesPNGAvailable {
		salesPNGMetadata, salesPNGAvailable = h.latestDocumentMetadata(ctx, orderID, "sales-order.png")
	}
	if deliveryPDFAvailable {
		deliveryPDFMetadata, deliveryPDFAvailable = h.latestDocumentMetadata(ctx, orderID, "delivery-note.pdf")
	}
	if deliveryPNGAvailable {
		deliveryPNGMetadata, deliveryPNGAvailable = h.latestDocumentMetadata(ctx, orderID, "delivery-note.png")
	}
	return miniEmployeeOrderDocumentsDTO{
		SalesOrderPDF:   miniEmployeeOrderDocumentDescriptor(orderID, "sales-order.pdf", "application/pdf", salesPDFAvailable, salesPDFMetadata),
		SalesOrderPNG:   miniEmployeeOrderDocumentDescriptor(orderID, "sales-order.png", "image/png", salesPNGAvailable, salesPNGMetadata),
		DeliveryNotePDF: miniEmployeeOrderDocumentDescriptor(orderID, "delivery-note.pdf", "application/pdf", deliveryPDFAvailable, deliveryPDFMetadata),
		DeliveryNotePNG: miniEmployeeOrderDocumentDescriptor(orderID, "delivery-note.png", "image/png", deliveryPNGAvailable, deliveryPNGMetadata),
	}, nil
}

func miniEmployeeLatestSalesOrderPDFAvailable(documents []salesapp.SalesOrderDocument) bool {
	for _, document := range documents {
		if document.IsLatest {
			return document.PDFAssetID > 0
		}
	}
	return false
}

func miniEmployeeLatestSalesOrderPNGAvailable(documents []salesapp.SalesOrderImageDocument) bool {
	for _, document := range documents {
		if document.IsLatest {
			return document.ImageAssetID > 0
		}
	}
	return false
}

func miniEmployeeLatestDeliveryNoteAvailability(documents []salesapp.DeliveryNoteDocument) (bool, bool) {
	for _, document := range documents {
		if document.IsLatest {
			return document.PDFAssetID > 0, document.ImageAssetID > 0
		}
	}
	return false, false
}

func (h miniEmployeeOrderHandler) downloadDocument(c echo.Context, kind string) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.read")
	if !ok {
		return err
	}
	var filePath, filename, contentType string
	switch kind {
	case "sales-order.pdf":
		file, loadErr := h.sales.LoadSalesOrderDocumentFile(c.Request().Context(), access.orderID, 0, true)
		if loadErr != nil {
			return miniEmployeeDocumentNotFound(c)
		}
		filePath, filename, contentType = file.Path, file.Filename, "application/pdf"
	case "sales-order.png":
		file, loadErr := h.sales.LoadSalesOrderImageFile(c.Request().Context(), access.orderID, 0, true)
		if loadErr != nil {
			return miniEmployeeDocumentNotFound(c)
		}
		filePath, filename, contentType = file.Path, file.Filename, "image/png"
	case "delivery-note.pdf":
		file, loadErr := h.sales.LoadDeliveryNoteDocumentFile(c.Request().Context(), access.orderID, 0, true)
		if loadErr != nil {
			return miniEmployeeDocumentNotFound(c)
		}
		filePath, filename, contentType = file.Path, file.Filename, "application/pdf"
	case "delivery-note.png":
		file, loadErr := h.sales.LoadDeliveryNoteImageFile(c.Request().Context(), access.orderID, 0, true)
		if loadErr != nil {
			return miniEmployeeDocumentNotFound(c)
		}
		filePath, filename, contentType = file.Path, file.Filename, "image/png"
	default:
		return miniEmployeeDocumentNotFound(c)
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return miniEmployeeDocumentNotFound(c)
	}
	if info, statErr := os.Stat(filePath); statErr != nil || info.IsDir() {
		return miniEmployeeDocumentNotFound(c)
	}
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "" {
		filename = miniEmployeeDocumentFilename(access.row.OrderNo, kind, 0)
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, disposition)
	return c.File(filePath)
}

func (h miniEmployeeOrderHandler) generateDocument(c echo.Context, kind string) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.write")
	if !ok {
		return err
	}
	metadata, existing := h.latestDocumentMetadata(c.Request().Context(), access.orderID, kind)
	if existing {
		if metadata.Filename == "." || metadata.Filename == "" {
			metadata.Filename = miniEmployeeDocumentFilename(access.row.OrderNo, kind, metadata.VersionNo)
		}
		return c.JSON(http.StatusOK, miniEmployeeOrderDocumentMutationResponse{Document: miniEmployeeGeneratedDocumentDTO{
			Path: miniEmployeeDocumentPath(access.orderID, kind), ContentType: miniEmployeeDocumentContentType(kind),
			Filename: metadata.Filename, VersionNo: metadata.VersionNo,
		}})
	}
	actor := miniEmployeeActor(access.employee)
	versionNo := 0
	switch kind {
	case "sales-order.pdf":
		result, err := h.sales.GenerateSalesOrderDocument(c.Request().Context(), salesapp.GenerateSalesOrderDocumentCommand{Actor: actor, OrderID: access.orderID})
		if err != nil {
			return miniEmployeeDocumentGenerateError(c, err)
		}
		versionNo = result.Document.VersionNo
	case "sales-order.png":
		result, err := h.sales.GenerateSalesOrderImage(c.Request().Context(), salesapp.GenerateSalesOrderImageCommand{Actor: actor, OrderID: access.orderID})
		if err != nil {
			return miniEmployeeDocumentGenerateError(c, err)
		}
		versionNo = result.Document.VersionNo
	case "delivery-note.pdf", "delivery-note.png":
		result, err := h.sales.GenerateDeliveryNoteDocument(c.Request().Context(), salesapp.GenerateDeliveryNoteDocumentCommand{Actor: actor, OrderID: access.orderID})
		if err != nil {
			return miniEmployeeDocumentGenerateError(c, err)
		}
		versionNo = result.Document.VersionNo
	default:
		return miniEmployeeDocumentNotFound(c)
	}
	return c.JSON(http.StatusOK, miniEmployeeOrderDocumentMutationResponse{Document: miniEmployeeGeneratedDocumentDTO{
		Path: miniEmployeeDocumentPath(access.orderID, kind), ContentType: miniEmployeeDocumentContentType(kind),
		Filename: miniEmployeeDocumentFilename(access.row.OrderNo, kind, versionNo), VersionNo: versionNo, Generated: true,
	}})
}

func (h miniEmployeeOrderHandler) latestDocumentMetadata(ctx context.Context, orderID int64, kind string) (miniEmployeeDocumentMetadata, bool) {
	switch kind {
	case "sales-order.pdf":
		file, err := h.sales.LoadSalesOrderDocumentFile(ctx, orderID, 0, true)
		return miniEmployeeDocumentMetadata{Filename: filepath.Base(strings.TrimSpace(file.Filename)), VersionNo: file.Document.VersionNo}, err == nil && miniEmployeeDocumentFileExists(file.Path)
	case "sales-order.png":
		file, err := h.sales.LoadSalesOrderImageFile(ctx, orderID, 0, true)
		return miniEmployeeDocumentMetadata{Filename: filepath.Base(strings.TrimSpace(file.Filename)), VersionNo: file.Document.VersionNo}, err == nil && miniEmployeeDocumentFileExists(file.Path)
	case "delivery-note.pdf":
		file, err := h.sales.LoadDeliveryNoteDocumentFile(ctx, orderID, 0, true)
		return miniEmployeeDocumentMetadata{Filename: filepath.Base(strings.TrimSpace(file.Filename)), VersionNo: file.Document.VersionNo}, err == nil && miniEmployeeDocumentFileExists(file.Path)
	case "delivery-note.png":
		file, err := h.sales.LoadDeliveryNoteImageFile(ctx, orderID, 0, true)
		return miniEmployeeDocumentMetadata{Filename: filepath.Base(strings.TrimSpace(file.Filename)), VersionNo: file.Document.VersionNo}, err == nil && miniEmployeeDocumentFileExists(file.Path)
	default:
		return miniEmployeeDocumentMetadata{}, false
	}
}

func miniEmployeeDocumentFileExists(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && !info.IsDir()
}

func miniEmployeeOrderDetail(row salesapp.OrderRow, form salesapp.OrderFormData) miniEmployeeOrderDetailDTO {
	ed := form.EditData
	items := make([]miniEmployeeOrderItemDetailDTO, 0, len(ed.Items))
	for _, item := range ed.Items {
		tierID := "auto"
		if item.PriceOverride {
			tierID = "manual"
		} else if item.PriceTierID > 0 {
			tierID = strconv.FormatInt(item.PriceTierID, 10)
		}
		items = append(items, miniEmployeeOrderItemDetailDTO{
			ItemID: item.ItemID, LineNo: item.LineNo, ProductID: item.ProductID, ProductName: item.Product,
			ParentProductID: item.ProductID, MigrationState: map[bool]string{true: "cutover", false: "legacy"}[item.BomSpecID > 0],
			BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, BomSpecKey: item.BomSpecKey, BomSpecName: item.BomSpecName,
			CustomerProductAliasID: item.CustomerProductAliasID, CustomerProductDisplayNameSnapshot: item.CustomerProductDisplayNameSnapshot,
			CustomerItemCodeSnapshot: item.CustomerItemCodeSnapshot, BrandNameSnapshot: item.BrandNameSnapshot,
			ProductCodeSnapshot: item.ProductCodeSnapshot, ProductNameSnapshot: item.ProductNameSnapshot, Note: item.Note,
			TierID: tierID, PriceOverride: item.PriceOverride, UnitPrice: item.UnitPrice, Qty: item.Qty, Unit: item.Unit,
			Spec: strings.TrimSuffix(strings.TrimSpace(strings.ToLower(item.Spec)), "g"), LineTotal: item.LineTotal,
			BeanListPublicationID: item.BeanListPublicationID, BeanListVersionNo: item.BeanListVersionNo,
			DiscountType: item.DiscountType, DiscountValue: item.DiscountValue, DiscountAmount: item.DiscountAmount,
			ProductKind: item.ProductKind, SalesUnit: item.SalesUnit, UnitBagCount: item.UnitBagCount, UnitBeanG: item.UnitBeanG,
			MatchedPriceQty: item.MatchedPriceQty, UnitConversionLabel: item.UnitConversionLabel, PriceSourceJSON: item.PriceSourceJSON,
		})
	}
	beanListPublicationID, beanListVersionNo, commercialPublicationID, greenPublicationID, dripPublicationID := miniEmployeeOrderPublicationFields(ed)
	detail := miniEmployeeOrderDetailDTO{
		ID: row.ID, EditRevision: ed.EditRevision, OrderNo: firstMiniOrderValue(row.OrderNo, ed.OrderNo),
		DocumentDate: firstMiniOrderValue(ed.DocumentDate, row.DocumentDate), OrderDate: firstMiniOrderValue(ed.OrderDate, row.OrderDate),
		CustomerID: ed.CustomerID, Customer: row.Customer, SourceID: ed.SourceID, Source: miniEmployeeOptionName(form.Sources, ed.SourceID),
		OrderTypeID: ed.OrderTypeID, OrderType: firstMiniOrderValue(row.OrderType, miniEmployeeOptionName(form.OrderTypes, ed.OrderTypeID)),
		PayStatusID: ed.PayStatusID, PayStatus: firstMiniOrderValue(row.PayStatus, miniEmployeeOptionName(form.PayStatuses, ed.PayStatusID)), PaymentMethod: ed.PaymentMethod,
		ShipStatusID: ed.ShipStatusID, ShipStatus: firstMiniOrderValue(row.ShipStatus, miniEmployeeOptionName(form.ShipStatuses, ed.ShipStatusID)),
		ProcessStatusID: row.ProcessStatusID, ProcessStatus: row.ProcessStatus,
		ShipMethod: ed.ShipMethod, ShipTrackingNo: firstMiniOrderValue(ed.ShipTrackingNo, row.ShipTrackingNo),
		LogisticsCompanyID: ed.LogisticsCompanyID, LogisticsProductID: ed.LogisticsProductID,
		SenderID: row.SenderID, SenderLabel: row.SenderLabel, SenderName: row.SenderName,
		PaymentGoodsAmount: ed.PaymentGoodsAmount, PaymentShippingAmount: ed.PaymentShippingAmount,
		PaymentVoucherAssetID: ed.PaymentVoucherAssetID, PaymentVoucher: miniEmployeeOrderAsset(ed.PaymentVoucher),
		ResponsibleType: firstMiniOrderValue(ed.ResponsibleType, row.ResponsibleType), ResponsibleID: ed.ResponsibleID,
		ResponsibleName: firstMiniOrderValue(ed.ResponsibleName, row.ResponsibleName),
		ReceiverName:    firstMiniOrderValue(ed.ReceiverName, row.ReceiverName), ReceiverPhone: firstMiniOrderValue(ed.ReceiverPhone, row.ReceiverPhone),
		ReceiverAddress: firstMiniOrderValue(ed.ReceiverAddress, row.ReceiverAddress), ReceiverCompany: firstMiniOrderValue(ed.ReceiverCompany, row.ReceiverCompany),
		PortalServiceCode: firstMiniOrderValue(ed.PortalServiceCode, row.PortalServiceCode), SourceWarehouse: firstMiniOrderValue(ed.SourceWarehouse, row.SourceWarehouse),
		BeanListPublicationID: beanListPublicationID, CommercialBeanListPublicationID: commercialPublicationID,
		GreenBeanListPublicationID: greenPublicationID, DripBeanListPublicationID: dripPublicationID, BeanListVersionNo: beanListVersionNo,
		Notes: firstMiniOrderValue(ed.Notes, row.Notes), TotalAmount: firstMiniOrderValue(ed.TotalAmount, row.TotalAmount),
		ShippingAmount: firstMiniOrderValue(ed.ShippingAmount, row.ShippingAmount), DiscountAmount: firstMiniOrderValue(ed.DiscountAmount, row.DiscountAmount),
		OrderDiscountAmount: miniEmployeeHeaderDiscountAmount(ed, row.DiscountAmount),
		RoundToInt:          ed.RoundToInt, RoundingAmount: ed.RoundingAmount, GrandTotal: firstMiniOrderValue(ed.GrandTotal, row.GrandTotal),
		ExpressFee: firstMiniOrderValue(ed.ExpressFee, row.ExpressFee), OutsourceMaterialFee: firstMiniOrderValue(ed.OutsourceMaterialFee, row.OutsourceMaterialFee),
		OutsourceRoastFee: firstMiniOrderValue(ed.OutsourceRoastFee, row.OutsourceRoastFee), OutsourcePackagingFee: firstMiniOrderValue(ed.OutsourcePackagingFee, row.OutsourcePackagingFee),
		OutsourceManualFee: firstMiniOrderValue(ed.OutsourceManualFee, row.OutsourceManualFee), OutsourceTaxFee: firstMiniOrderValue(ed.OutsourceTaxFee, row.OutsourceTaxFee),
		OutsourceOtherFee: firstMiniOrderValue(ed.OutsourceOtherFee, row.OutsourceOtherFee), OutsourceTotalFee: firstMiniOrderValue(ed.OutsourceTotalFee, row.OutsourceTotalFee),
		ProductKindSummary: row.ProductKindSummary, CreatedByEmployee: row.CreatedByEmployee,
		IsVoid: ed.IsVoid || row.IsVoid, VoidedAt: ed.VoidedAt, VoidReason: ed.VoidReason,
		InvoiceStatus: row.InvoiceStatus, InvoiceFilename: row.InvoiceFilename, InvoiceFileURL: row.InvoiceFileURL,
		Items: items, QuoteSourceTrace: miniEmployeeQuoteSourceTrace(ed), ProductionSourceTrace: miniEmployeeProductionSourceTrace(ed),
	}
	if detail.ID <= 0 {
		detail.ID = ed.ID
	}
	if detail.CustomerID <= 0 {
		detail.CustomerID = row.CustomerID
	}
	if detail.OrderTypeID <= 0 {
		detail.OrderTypeID = row.OrderTypeID
	}
	if detail.PayStatusID <= 0 {
		detail.PayStatusID = row.PayStatusID
	}
	if detail.ShipStatusID <= 0 {
		detail.ShipStatusID = row.ShipStatusID
	}
	if detail.ResponsibleID <= 0 {
		detail.ResponsibleID = row.ResponsibleID
	}
	detail.LogisticsCompany, detail.LogisticsProduct = miniEmployeeLogisticsNames(form.LogisticsCompanies, ed.LogisticsCompanyID, ed.LogisticsProductID)
	return detail
}

func miniEmployeeOrderAsset(asset *salesapp.SalesOrderAsset) *miniEmployeeOrderAssetDTO {
	if asset == nil {
		return nil
	}
	return &miniEmployeeOrderAssetDTO{
		ID: asset.ID, Kind: asset.Kind, Filename: asset.Filename, ContentType: asset.ContentType,
		Bytes: asset.Bytes, CreatedAt: asset.CreatedAt, CreatedBy: asset.CreatedBy, URL: asset.URL,
	}
}

func miniEmployeeOptionName(options []salesapp.Option, id int64) string {
	for _, option := range options {
		if option.ID == id {
			return option.Name
		}
	}
	return ""
}

func miniEmployeeLogisticsNames(companies []salesapp.LogisticsCompany, companyID, productID int64) (string, string) {
	for _, company := range companies {
		if company.ID != companyID {
			continue
		}
		productName := ""
		for _, product := range company.Products {
			if product.ID == productID {
				productName = product.Name
				break
			}
		}
		return company.Name, productName
	}
	return "", ""
}

func miniEmployeeOrderPublicationFields(ed *salesapp.OrderEditData) (int64, string, int64, int64, int64) {
	publicationByType := func(listType string) (int64, string, bool) {
		publicationID := int64(0)
		versionNo := ""
		for _, item := range ed.Items {
			if miniEmployeeOrderBeanListType(item) != listType || item.BeanListPublicationID <= 0 {
				continue
			}
			if publicationID > 0 && publicationID != item.BeanListPublicationID {
				return 0, "", true
			}
			publicationID = item.BeanListPublicationID
			if versionNo == "" {
				versionNo = strings.TrimSpace(item.BeanListVersionNo)
			}
		}
		return publicationID, versionNo, false
	}
	commercialItemID, commercialItemVersion, ambiguous := publicationByType("commercial")
	beanListID, beanListVersion, commercialID := ed.BeanListPublicationID, ed.BeanListVersionNo, ed.BeanListPublicationID
	if ambiguous {
		beanListID, beanListVersion, commercialID = 0, "", 0
	} else if commercialItemID > 0 {
		beanListID, commercialID = commercialItemID, commercialItemID
		if commercialItemVersion != "" {
			beanListVersion = commercialItemVersion
		} else if ed.BeanListPublicationID != commercialItemID {
			beanListVersion = ""
		}
	}
	greenID, _, _ := publicationByType("green")
	dripID, _, _ := publicationByType("drip")
	return beanListID, beanListVersion, commercialID, greenID, dripID
}

func miniEmployeeOrderBeanListType(item salesapp.OrderEditItem) string {
	source := miniEmployeeTraceSnapshot(item.PriceSourceJSON)
	if listType := strings.ToLower(miniEmployeeTraceString(source["list_type"])); listType != "" {
		switch listType {
		case "green", "green_bean", "green-bean":
			return "green"
		case "drip", "drip_bag", "drip-bag":
			return "drip"
		default:
			return "commercial"
		}
	}
	switch strings.ToLower(strings.TrimSpace(item.ProductKind)) {
	case "green", "green_bean", "green-bean":
		return "green"
	case "drip", "drip_bag", "drip-bag":
		return "drip"
	default:
		return "commercial"
	}
}

func miniEmployeeQuoteSourceTrace(ed *salesapp.OrderEditData) []miniEmployeeQuoteSourceTraceDTO {
	rows := make([]miniEmployeeQuoteSourceTraceDTO, 0, len(ed.Items))
	for _, item := range ed.Items {
		source := miniEmployeeTraceSnapshot(item.PriceSourceJSON)
		version := miniEmployeeTraceString(source["version"])
		if version == "" {
			version = item.BeanListVersionNo
		}
		publicationID := miniEmployeeTraceInt64(source["publication_id"])
		if publicationID <= 0 {
			publicationID = item.BeanListPublicationID
		}
		rows = append(rows, miniEmployeeQuoteSourceTraceDTO{
			ProductID: item.ProductID, ProductName: item.Product, PriceListPublicationID: publicationID,
			PriceListVersion: version, TierLabel: miniEmployeeTraceString(source["tier_label"]),
			PriceUnit: miniEmployeeTraceString(source["price_unit"]), FinalUnitPrice: miniEmployeeTraceNumber(source["final_unit_price"]),
			PricingRuleVersion: miniEmployeeTraceString(source["pricing_rule_version"]), ManualAdjusted: miniEmployeeTraceBool(source["manual_adjusted"]),
			SourceLabel: "已发布商品价格表快照",
		})
	}
	return rows
}

func miniEmployeeProductionSourceTrace(ed *salesapp.OrderEditData) []miniEmployeeProductionTraceDTO {
	rows := make([]miniEmployeeProductionTraceDTO, 0, len(ed.Items))
	for _, item := range ed.Items {
		source := miniEmployeeTraceSnapshot(item.PriceSourceJSON)
		costSource := miniEmployeeTraceObject(source["cost_source_snapshot"])
		if len(costSource) == 0 {
			costSource = miniEmployeeTraceObject(source["production_source_snapshot"])
		}
		if len(costSource) == 0 {
			continue
		}
		rows = append(rows, miniEmployeeProductionTraceDTO{
			ProductID: item.ProductID, ProductName: item.Product, SourceLabel: "工单/工序卡冻结快照",
			WorkOrderNo: miniEmployeeTraceString(costSource["work_order_no"]), BOMVersionNo: miniEmployeeTraceString(costSource["bom_version_no"]),
			BOMVersionID: miniEmployeeTraceString(costSource["bom_version_id"]), ProcessRouteName: miniEmployeeTraceString(costSource["process_route_name"]),
			ProcessCardNo: miniEmployeeTraceString(costSource["process_card_no"]), MaterialBatchNo: miniEmployeeTraceString(costSource["material_batch_no"]),
		})
	}
	return rows
}

func miniEmployeeTraceSnapshot(value string) map[string]any {
	result := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(value)))
	decoder.UseNumber()
	if strings.TrimSpace(value) == "" || decoder.Decode(&result) != nil {
		return map[string]any{}
	}
	return result
}

func miniEmployeeTraceObject(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		return miniEmployeeTraceSnapshot(typed)
	default:
		return map[string]any{}
	}
}

func miniEmployeeTraceString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func miniEmployeeTraceNumber(value any) float64 {
	switch typed := value.(type) {
	case json.Number:
		number, _ := typed.Float64()
		return number
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		number, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number
	default:
		return 0
	}
}

func miniEmployeeTraceInt64(value any) int64 {
	return int64(miniEmployeeTraceNumber(value))
}

func miniEmployeeTraceBool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	if typed, ok := value.(string); ok {
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	}
	return miniEmployeeTraceNumber(value) != 0
}

func miniEmployeeOrderDocumentDescriptor(orderID int64, kind, contentType string, available bool, metadata miniEmployeeDocumentMetadata) miniEmployeeOrderDocumentDTO {
	return miniEmployeeOrderDocumentDTO{
		Available: available, Path: miniEmployeeDocumentPath(orderID, kind), ContentType: contentType,
		Filename: metadata.Filename, VersionNo: metadata.VersionNo,
	}
}

func miniEmployeeDocumentPath(orderID int64, kind string) string {
	return fmt.Sprintf("/api/mini/employee/orders/%d/documents/%s", orderID, kind)
}

func miniEmployeeDocumentContentType(kind string) string {
	if strings.HasSuffix(kind, ".png") {
		return "image/png"
	}
	return "application/pdf"
}

func miniEmployeeDocumentFilename(orderNo, kind string, versionNo int) string {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		orderNo = "order"
	}
	prefix := "销售单"
	if strings.HasPrefix(kind, "delivery-note") {
		prefix = "发货单"
	}
	version := ""
	if versionNo > 0 {
		version = fmt.Sprintf("-V%d", versionNo)
	}
	extension := filepath.Ext(kind)
	return prefix + "-" + orderNo + version + extension
}

func miniEmployeeOrderNotFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在"})
}

func miniEmployeeDocumentNotFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]string{"error": "订单或单据不存在"})
}

func miniEmployeeDocumentGenerateError(c echo.Context, err error) error {
	message := "单据生成失败"
	if strings.Contains(strings.ToLower(err.Error()), "shipped") {
		message = "订单尚未发货，不能生成发货单"
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
}

func requireMiniEmployeeCustomerWriter(ctx context.Context, authorization string, portal Service) (customerportalapp.CurrentContext, error) {
	employee, err := requireMiniEmployee(ctx, authorization, portal, "customers.read")
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if !containsMiniRole(employee.Permissions, "customers.write") {
		return customerportalapp.CurrentContext{}, errMiniEmployeeForbidden
	}
	return employee, nil
}

func miniCustomerOptions(options []customerapp.Option) []map[string]any {
	rows := make([]map[string]any, 0, len(options))
	for _, option := range options {
		rows = append(rows, map[string]any{"id": option.ID, "name": option.Name})
	}
	return rows
}

func miniCustomerPrincipal(employee customerportalapp.CurrentContext) customerapp.MaintenancePrincipal {
	return customerapp.MaintenancePrincipal{
		EmployeeID: employee.EmployeeID, EmployeeName: employee.EmployeeName,
		IsAdmin: containsMiniRole(employee.Roles, "admin"),
	}
}

func miniEmployeeActor(employee customerportalapp.CurrentContext) string {
	name := strings.TrimSpace(employee.EmployeeName)
	if name == "" {
		return "mini-employee:" + strconv.FormatInt(employee.EmployeeID, 10)
	}
	return "mini-employee:" + strconv.FormatInt(employee.EmployeeID, 10) + ":" + name
}

func miniCustomerUpsertCommand(req miniEmployeeCustomerRequest, existing *customerapp.CustomerEditData) customerapp.UpsertCommand {
	active := true
	if existing != nil {
		active = existing.Active
	}
	if req.Active != nil {
		active = *req.Active
	}
	responsibleEmployeeID := req.ResponsibleEmployeeID
	if responsibleEmployeeID <= 0 && existing != nil {
		responsibleEmployeeID, _ = strconv.ParseInt(existing.ResponsibleEmployeeID, 10, 64)
	}
	activeValue := ""
	if active {
		activeValue = "on"
	}
	return customerapp.UpsertCommand{
		Name: strings.TrimSpace(req.Name), RawName: strings.TrimSpace(req.Name),
		CustomerType: strings.TrimSpace(req.CustomerType), CompanyName: strings.TrimSpace(req.CompanyName),
		CompanyAddress: strings.TrimSpace(req.CompanyAddress), CompanyPhone: strings.TrimSpace(req.CompanyPhone),
		Contact: strings.TrimSpace(req.Contact), Phone: strings.TrimSpace(req.Phone), Address: strings.TrimSpace(req.Address),
		DefaultSourceID: strconv.FormatInt(req.DefaultSourceID, 10), DefaultOrderTypeID: strconv.FormatInt(req.DefaultOrderTypeID, 10),
		ResponsibleEmployeeID: strconv.FormatInt(responsibleEmployeeID, 10), Active: activeValue,
		PortalEnabled: req.PortalEnabled, CapabilityTemplateKey: strings.TrimSpace(req.CapabilityTemplateKey),
	}
}

func miniCustomerEditResponse(customer customerapp.CustomerEditData) map[string]any {
	responsibleEmployeeID, _ := strconv.ParseInt(strings.TrimSpace(customer.ResponsibleEmployeeID), 10, 64)
	defaultSourceID, _ := strconv.ParseInt(strings.TrimSpace(customer.DefaultSourceID), 10, 64)
	defaultOrderTypeID, _ := strconv.ParseInt(strings.TrimSpace(customer.DefaultOrderTypeID), 10, 64)
	return map[string]any{
		"id": customer.ID, "name": customer.Name, "customer_type": customer.CustomerType,
		"company_name": customer.CompanyName, "company_address": customer.CompanyAddress, "company_phone": customer.CompanyPhone,
		"contact": customer.Contact, "phone": customer.Phone, "address": customer.Address,
		"default_source_id": defaultSourceID, "default_order_type_id": defaultOrderTypeID,
		"responsible_employee_id": responsibleEmployeeID, "responsible_employee_name": customer.ResponsibleEmployeeName,
		"active": customer.Active, "portal_enabled": customer.PortalEnabled, "capability_template_key": customer.CapabilityTemplateKey,
		"receiver_name":    firstMiniOrderValue(customer.Contact, customer.Name),
		"receiver_phone":   firstMiniOrderValue(customer.Phone, customer.CompanyPhone),
		"receiver_address": firstMiniOrderValue(customer.Address, customer.CompanyAddress),
		"receiver_company": firstMiniOrderValue(customer.CompanyName, customer.Name),
	}
}

func miniCustomerResponseAfterWrite(ctx context.Context, customers CustomerMaintenance, principal customerapp.MaintenancePrincipal, id int64, req miniEmployeeCustomerRequest, existing *customerapp.CustomerEditData) (map[string]any, error) {
	data, err := customers.EditorManaged(ctx, principal, id)
	if err == nil && data != nil {
		return miniCustomerEditResponse(data.Customer), nil
	}
	if err != nil && !errors.Is(err, customerapp.ErrCustomerMaintenanceForbidden) && !errors.Is(err, customerapp.ErrCustomerNotFound) {
		return nil, err
	}

	active := true
	portalEnabled := false
	capabilityTemplateKey := ""
	responsibleEmployeeID := principal.EmployeeID
	responsibleEmployeeName := principal.EmployeeName
	if existing != nil {
		active = existing.Active
		portalEnabled = existing.PortalEnabled
		capabilityTemplateKey = existing.CapabilityTemplateKey
		if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(existing.ResponsibleEmployeeID), 10, 64); parseErr == nil && parsed > 0 {
			responsibleEmployeeID = parsed
			responsibleEmployeeName = existing.ResponsibleEmployeeName
		}
	}
	if principal.IsAdmin {
		if req.Active != nil {
			active = *req.Active
		}
		if req.PortalEnabled != nil {
			portalEnabled = *req.PortalEnabled
		}
		if req.ResponsibleEmployeeID > 0 {
			responsibleEmployeeID = req.ResponsibleEmployeeID
			if existing == nil || responsibleEmployeeID != parseMiniCustomerID(existing.ResponsibleEmployeeID) {
				responsibleEmployeeName = ""
			}
		}
	}

	return miniCustomerEditResponse(customerapp.CustomerEditData{
		ID: id, Name: strings.TrimSpace(req.Name), RawName: strings.TrimSpace(req.Name),
		CustomerType: customerapp.NormalizeCustomerType(req.CustomerType),
		CompanyName:  strings.TrimSpace(req.CompanyName), CompanyAddress: strings.TrimSpace(req.CompanyAddress), CompanyPhone: strings.TrimSpace(req.CompanyPhone),
		Contact: strings.TrimSpace(req.Contact), Phone: strings.TrimSpace(req.Phone), Address: strings.TrimSpace(req.Address),
		DefaultSourceID: strconv.FormatInt(req.DefaultSourceID, 10), DefaultOrderTypeID: strconv.FormatInt(req.DefaultOrderTypeID, 10),
		ResponsibleEmployeeID: strconv.FormatInt(responsibleEmployeeID, 10), ResponsibleEmployeeName: responsibleEmployeeName,
		PortalEnabled: portalEnabled, CapabilityTemplateKey: capabilityTemplateKey, Active: active,
	}), nil
}

func parseMiniCustomerID(raw string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id
}

func miniCustomerError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, customerapp.ErrCustomerMaintenanceForbidden):
		return c.JSON(http.StatusForbidden, map[string]string{"error": "员工只能维护自己负责的客户"})
	case errors.Is(err, customerapp.ErrCustomerNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "客户不存在"})
	}
	if message, ok := customerapp.MaintenanceValidationMessage(err); ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
	}
	return miniInternalError(c)
}

func firstMiniOrderValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

var (
	errMiniEmployeeLoginRequired = errors.New("mini employee login required")
	errMiniEmployeeForbidden     = errors.New("mini employee permission denied")
	errMiniEmployeeUnavailable   = errors.New("mini employee service unavailable")
)

func requireMiniEmployee(ctx context.Context, authorization string, portal Service, permission string) (customerportalapp.CurrentContext, error) {
	if portal == nil {
		return customerportalapp.CurrentContext{}, errMiniEmployeeUnavailable
	}
	token := miniTokenFromHeader(authorization)
	if token == "" {
		return customerportalapp.CurrentContext{}, errMiniEmployeeLoginRequired
	}
	current, err := portal.Me(ctx, token)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if current.AccountType != "employee" || (!containsMiniRole(current.Roles, "sales") && !containsMiniRole(current.Roles, "admin")) || !containsMiniRole(current.Permissions, permission) {
		return customerportalapp.CurrentContext{}, errMiniEmployeeForbidden
	}
	return current, nil
}

func miniEmployeeAuthError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, errMiniEmployeeLoginRequired):
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "请先登录"})
	case errors.Is(err, errMiniEmployeeForbidden):
		return c.JSON(http.StatusForbidden, map[string]string{"error": "当前员工无此权限"})
	case errors.Is(err, errMiniEmployeeUnavailable):
		return miniInternalError(c)
	default:
		return miniSessionError(c, err)
	}
}

func containsMiniRole(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func miniOrderKnownBusinessErrorText(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, salesapp.ErrInvalidShippingAmount) {
		return "运费金额不能小于 0", true
	}
	if errors.Is(err, salesapp.ErrInvalidDiscountAmount) {
		return "整单优惠不能小于 0", true
	}
	switch strings.TrimSpace(err.Error()) {
	case "customer required":
		return "请选择客户", true
	case "at least one item required":
		return "请至少添加一个商品", true
	case "product required":
		return "请选择商品", true
	case "spec required":
		return "请填写规格克重", true
	case "qty required":
		return "请填写正确数量", true
	case "customer product price unpublished":
		return "客户商品价格尚未发布", true
	case "invalid pay_status_id":
		return "付款状态不正确", true
	case "payment_method required":
		return "请选择付款方式", true
	case "invalid ship_status_id":
		return "发货状态不正确", true
	case "logistics_company_id required":
		return "请选择物流公司", true
	case "logistics_product_id required", "invalid logistics product":
		return "请选择正确的物流产品", true
	case "payment_goods_amount required":
		return "请填写货款金额", true
	case "invalid payment_shipping_amount":
		return "运费金额不正确", true
	case "payment_voucher_asset_id required", "invalid payment_voucher_asset_id":
		return "付款凭证不正确", true
	}
	message := strings.TrimSpace(err.Error())
	for _, prefix := range []string{
		"客户资料缺少", "客户商品别名", "商品规格", "所选价格表", "价格表", "提交的商品",
		"订单行", "手动单价", "缺少", "具体 SKU", "解析订单价格来源快照失败", "生成订单",
	} {
		if strings.HasPrefix(message, prefix) {
			return message, true
		}
	}
	return "", false
}

func miniEmployeeHeaderDiscountAmount(ed *salesapp.OrderEditData, fallbackTotal ...string) string {
	if ed == nil {
		return "0.00"
	}
	rawTotal := strings.TrimSpace(ed.DiscountAmount)
	if rawTotal == "" && len(fallbackTotal) > 0 {
		rawTotal = strings.TrimSpace(fallbackTotal[0])
	}
	total, err := strconv.ParseFloat(rawTotal, 64)
	if err != nil {
		return "0.00"
	}
	for _, item := range ed.Items {
		lineDiscount, parseErr := strconv.ParseFloat(strings.TrimSpace(item.DiscountAmount), 64)
		if parseErr == nil {
			total -= lineDiscount
		}
	}
	if total < 0 || total < 0.005 {
		total = 0
	}
	return fmt.Sprintf("%.2f", total)
}
