package customerportal

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	catalogdomain "orderapp/internal/domain/catalog"

	"github.com/labstack/echo/v4"
)

type miniEmployeeOrderItemRequest struct {
	ProductID    int64   `json:"product_id"`
	Name         string  `json:"name"`
	Qty          int64   `json:"qty"`
	Unit         string  `json:"unit"`
	SpecG        int64   `json:"spec_g"`
	ProductKind  string  `json:"product_kind"`
	SalesUnit    string  `json:"sales_unit"`
	UnitBagCount int64   `json:"unit_bag_count"`
	UnitBeanG    float64 `json:"unit_bean_g"`
	UnitPrice    float64 `json:"unit_price"`
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

func registerMiniEmployeeAPI(e *echo.Echo, portal Service, sales EmployeeSales) {
	e.GET("/api/mini/employee/order-form", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		_ = employee
		if sales == nil {
			return miniInternalError(c)
		}
		form, err := sales.OrderForm(c.Request().Context(), 0)
		if err != nil {
			return miniInternalError(c)
		}
		customers := make([]map[string]any, 0, len(form.Customers))
		for _, customer := range form.Customers {
			customers = append(customers, map[string]any{
				"id": customer.ID, "name": customer.Name, "customer_type": customer.CustomerType,
				"py": catalogdomain.SearchPinyin(customer.Name), "pyi": catalogdomain.SearchInitials(customer.Name),
				"default_source_id": customer.DefaultSourceID, "default_order_type_id": customer.DefaultOrderTypeID,
				"receiver_name":    firstMiniOrderValue(customer.Contact, customer.Name),
				"receiver_phone":   firstMiniOrderValue(customer.Phone, customer.CompanyPhone),
				"receiver_address": firstMiniOrderValue(customer.Address, customer.CompanyAddress),
				"receiver_company": firstMiniOrderValue(customer.CompanyName, customer.Name),
			})
		}
		families := salesapp.BuildOrderProductFamilies(form.Products)
		return c.JSON(http.StatusOK, map[string]any{
			"today": form.Today, "customers": customers, "sources": form.Sources,
			"order_types": form.OrderTypes, "pay_statuses": form.PayStatuses,
			"ship_statuses": form.ShipStatuses, "products": form.Products, "product_families": families,
		})
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
				ProductID: &productID, Name: strings.TrimSpace(item.Name), Units: item.Qty,
				Unit: strings.TrimSpace(item.Unit), SpecG: item.SpecG, ProductKind: strings.TrimSpace(item.ProductKind),
				SalesUnit: strings.TrimSpace(item.SalesUnit), UnitBagCount: item.UnitBagCount,
				UnitBeanG: item.UnitBeanG, ManualPrice: &price,
			})
		}
		result, err := sales.SaveOrder(c.Request().Context(), salesapp.SaveOrderCommand{
			Actor: "mini-employee:" + employee.EmployeeName, DocumentDate: orderDate, OrderDate: orderDate,
			CustomerID: req.CustomerID, SourceID: req.SourceID, OrderTypeID: req.OrderTypeID,
			PayStatusID: req.PayStatusID, ShipStatusID: req.ShipStatusID,
			ResponsibleType: "employee", ResponsibleID: employee.EmployeeID,
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
