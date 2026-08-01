package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	catalogdomain "orderapp/internal/domain/catalog"

	"github.com/labstack/echo/v4"
)

type miniEmployeeOrderItemRequest struct {
	ProductID              int64   `json:"product_id"`
	CustomerProductAliasID int64   `json:"customer_product_alias_id"`
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
	OrderDate       string                         `json:"order_date"`
	CustomerID      int64                          `json:"customer_id"`
	SourceID        int64                          `json:"source_id"`
	OrderTypeID     int64                          `json:"order_type_id"`
	PayStatusID     int64                          `json:"pay_status_id"`
	ShipStatusID    int64                          `json:"ship_status_id"`
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
	e.GET("/api/mini/employee/order-form", func(c echo.Context) error {
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
		families := salesapp.BuildOrderProductFamilies(form.Products)
		return c.JSON(http.StatusOK, map[string]any{
			"today": form.Today, "customers": customers, "sources": form.Sources,
			"order_types": form.OrderTypes, "pay_statuses": form.PayStatuses,
			"ship_statuses": form.ShipStatuses, "products": form.Products, "product_families": families,
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
		orderDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.OrderDate))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单日期不正确"})
		}
		items := make([]salesapp.OrderItemCommand, 0, len(req.Items))
		for _, item := range req.Items {
			productID := item.ProductID
			price := item.UnitPrice
			items = append(items, salesapp.OrderItemCommand{
				ProductID: &productID, CustomerProductAliasID: item.CustomerProductAliasID,
				Name: strings.TrimSpace(item.Name), Units: item.Qty,
				Unit: strings.TrimSpace(item.Unit), SpecG: item.SpecG, ProductKind: strings.TrimSpace(item.ProductKind),
				SalesUnit: strings.TrimSpace(item.SalesUnit), UnitBagCount: item.UnitBagCount,
				UnitBeanG: item.UnitBeanG, ManualPrice: &price,
			})
		}
		result, err := sales.SaveOrder(c.Request().Context(), salesapp.SaveOrderCommand{
			Actor: miniEmployeeActor(employee), DraftEmployeeID: employee.EmployeeID, DocumentDate: orderDate, OrderDate: orderDate,
			CustomerID: req.CustomerID, SourceID: req.SourceID, OrderTypeID: req.OrderTypeID,
			PayStatusID: req.PayStatusID, ShipStatusID: req.ShipStatusID,
			ReceiverName: req.ReceiverName, ReceiverPhone: req.ReceiverPhone,
			ReceiverAddress: req.ReceiverAddress, ReceiverCompany: req.ReceiverCompany,
			Notes: req.Notes, Items: items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": miniOrderErrorText(err)})
		}
		return c.JSON(http.StatusCreated, map[string]any{"order_id": result.OrderID, "order_no": result.OrderNo})
	})
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

func miniOrderErrorText(err error) string {
	switch strings.TrimSpace(err.Error()) {
	case "customer required":
		return "请选择客户"
	case "at least one item required":
		return "请至少添加一个商品"
	case "product required":
		return "请选择商品"
	case "spec required":
		return "请填写规格克重"
	case "qty required":
		return "请填写正确数量"
	default:
		return err.Error()
	}
}
