package customerportal

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	messagecenterapp "orderapp/internal/application/messagecenter"

	"github.com/labstack/echo/v4"
)

type miniLoginRequest struct {
	Mode      string `json:"mode"`
	Code      string `json:"code"`
	Phone     string `json:"phone"`
	PhoneCode string `json:"phone_code"`
	Nickname  string `json:"nickname"`
}

type miniPasswordLoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type switchCustomerRequest struct {
	CustomerID int64 `json:"customer_id"`
}

type directShipBatchRequest struct {
	SourceName string `json:"source_name"`
	TotalRows  int    `json:"total_rows"`
	Note       string `json:"note"`
}

type processingRequestPayload struct {
	InputMaterialID int64  `json:"input_material_id"`
	InputQtyG       int64  `json:"input_qty_g"`
	TargetProductID int64  `json:"target_product_id"`
	TargetSpecG     int64  `json:"target_spec_g"`
	TargetQty       int    `json:"target_qty"`
	Note            string `json:"note"`
}

type fulfillmentOrderRequest struct {
	ServiceCode      string  `json:"service_code"`
	RecipientName    string  `json:"recipient_name"`
	RecipientPhone   string  `json:"recipient_phone"`
	RecipientAddress string  `json:"recipient_address"`
	RecipientCompany string  `json:"recipient_company"`
	ProductID        int64   `json:"product_id"`
	ProductName      string  `json:"product_name"`
	SpecG            int64   `json:"spec_g"`
	Qty              int64   `json:"qty"`
	UnitPrice        float64 `json:"unit_price"`
	Note             string  `json:"note"`
}

type mallOrderRequest struct {
	RecipientName    string                                   `json:"recipient_name"`
	RecipientPhone   string                                   `json:"recipient_phone"`
	RecipientAddress string                                   `json:"recipient_address"`
	RecipientCompany string                                   `json:"recipient_company"`
	Note             string                                   `json:"note"`
	Items            []customerportalapp.MallOrderItemCommand `json:"items"`
}

const miniCustomerConfigUpdatedMessage = "客户配置已更新，请联系管理员处理"

func registerMiniAPI(e *echo.Echo, svc Service, messages MessagePublisher, beanListPDFRenderer BeanListPDFRenderer, salesDocs SalesDocuments) {
	e.POST("/api/mini/login", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		var req miniLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.Login(c.Request().Context(), customerportalapp.LoginCommand{
			Mode:      req.Mode,
			Code:      req.Code,
			Phone:     req.Phone,
			PhoneCode: req.PhoneCode,
			Nickname:  req.Nickname,
		})
		if err != nil {
			return miniLoginError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/login/password", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		var req miniPasswordLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		login := strings.TrimSpace(req.Login)
		password := strings.TrimSpace(req.Password)
		if login == "" || password == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.LoginWithPassword(c.Request().Context(), customerportalapp.PasswordLoginCommand{Login: login, Password: password})
		if err != nil {
			return miniPasswordLoginError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/me", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		result, err := svc.Me(c.Request().Context(), token)
		if err != nil {
			return miniSessionError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/current-customer", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req switchCustomerRequest
		if err := c.Bind(&req); err != nil || req.CustomerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		result, err := svc.SwitchCurrentCustomer(c.Request().Context(), token, req.CustomerID)
		if err != nil {
			return miniSwitchCustomerError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/services/:key", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		result, err := svc.GetServicePage(c.Request().Context(), token, c.Param("key"), customerportalapp.ServicePageFilter{
			Query:         c.QueryParam("q"),
			DateFrom:      c.QueryParam("date_from"),
			DateTo:        c.QueryParam("date_to"),
			ProcessStatus: c.QueryParam("process_status"),
			PayStatus:     c.QueryParam("pay_status"),
			ShipStatus:    c.QueryParam("ship_status"),
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/mall", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		result, err := svc.GetMallPage(c.Request().Context(), token)
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/mall/orders", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req mallOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.CreateMallOrder(c.Request().Context(), token, customerportalapp.CreateMallOrderCommand{
			RecipientName:    req.RecipientName,
			RecipientPhone:   req.RecipientPhone,
			RecipientAddress: req.RecipientAddress,
			RecipientCompany: req.RecipientCompany,
			Note:             req.Note,
			Items:            req.Items,
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		publishMiniOrderCreated(c, messages, result, "mall")
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/bean-lists/:id", func(c echo.Context) error {
		if svc == nil || beanListPDFRenderer == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		publicationID, err := strconv.ParseInt(strings.TrimSuffix(c.Param("id"), ".pdf"), 10, 64)
		if err != nil || publicationID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		row, err := svc.GetBeanListPublication(c.Request().Context(), token, publicationID)
		if err != nil {
			return miniBusinessError(c, err)
		}
		body, err := beanListPDFRenderer.Render(beanListPDFDocument(row))
		if err != nil {
			return miniInternalError(c)
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, beanListPDFFilename(row)))
		return c.Blob(http.StatusOK, "application/pdf", body)
	})

	e.GET("/api/mini/orders/:id/sales-order-latest.pdf", func(c echo.Context) error {
		return miniOrderDocument(c, svc, salesDocs, "sales_order")
	})

	e.GET("/api/mini/orders/:id/delivery-note-latest.pdf", func(c echo.Context) error {
		return miniOrderDocument(c, svc, salesDocs, "delivery_note")
	})

	e.POST("/api/mini/direct-ship/batches", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req directShipBatchRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.CreateDirectShipBatch(c.Request().Context(), token, customerportalapp.CreateDirectShipBatchCommand{
			SourceName: req.SourceName,
			TotalRows:  req.TotalRows,
			Note:       req.Note,
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/processing-requests", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req processingRequestPayload
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.CreateProcessingRequest(c.Request().Context(), token, customerportalapp.CreateProcessingRequestCommand{
			InputMaterialID: req.InputMaterialID,
			InputQtyG:       req.InputQtyG,
			TargetProductID: req.TargetProductID,
			TargetSpecG:     req.TargetSpecG,
			TargetQty:       req.TargetQty,
			Note:            req.Note,
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/fulfillment-orders", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req fulfillmentOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.CreateFulfillmentOrder(c.Request().Context(), token, customerportalapp.CreateFulfillmentOrderCommand{
			PortalServiceCode: req.ServiceCode,
			RecipientName:     req.RecipientName,
			RecipientPhone:    req.RecipientPhone,
			RecipientAddress:  req.RecipientAddress,
			RecipientCompany:  req.RecipientCompany,
			ProductID:         req.ProductID,
			ProductName:       req.ProductName,
			SpecG:             req.SpecG,
			Qty:               req.Qty,
			Note:              req.Note,
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		publishMiniOrderCreated(c, messages, result, req.ServiceCode)
		return c.JSON(http.StatusOK, result)
	})
}

func miniOrderDocument(c echo.Context, svc Service, salesDocs SalesDocuments, kind string) error {
	if svc == nil || salesDocs == nil {
		return miniInternalError(c)
	}
	token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := svc.EnsureOrderAccess(c.Request().Context(), token, orderID); err != nil {
		return miniBusinessError(c, err)
	}

	var path string
	var filename string
	switch kind {
	case "sales_order":
		file, err := salesDocs.LoadSalesOrderDocumentFile(c.Request().Context(), orderID, 0, true)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "order document not found"})
		}
		path = file.Path
		filename = file.Filename
	case "delivery_note":
		file, err := salesDocs.LoadDeliveryNoteDocumentFile(c.Request().Context(), orderID, 0, true)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "order document not found"})
		}
		path = file.Path
		filename = file.Filename
	default:
		return miniInternalError(c)
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.File(path)
}

func publishMiniOrderCreated(c echo.Context, messages MessagePublisher, result customerportalapp.FulfillmentOrder, serviceCode string) {
	if messages == nil || result.OrderID <= 0 {
		return
	}
	serviceCode = strings.TrimSpace(serviceCode)
	if serviceCode == "" {
		serviceCode = strings.TrimSpace(result.PortalServiceCode)
	}
	title := "新订单 " + strings.TrimSpace(result.OrderNo)
	body := "客户门户订单已提交"
	scope := "fulfillment"
	if serviceCode == "mall" {
		body = "商城订单已提交"
		scope = "all"
	}
	_, _ = messages.Publish(c.Request().Context(), messagecenterapp.PublishCommand{
		EventKey:   fmt.Sprintf("order.created.%d", result.OrderID),
		Topic:      "orders",
		EventType:  "order.created",
		SourceType: "order",
		SourceID:   result.OrderID,
		Actor:      "customer_portal",
		Title:      title,
		Body:       body,
		Tone:       "success",
		Payload: map[string]any{
			"order_id":            result.OrderID,
			"order_no":            result.OrderNo,
			"portal_service_code": serviceCode,
			"source_warehouse":    result.SourceWarehouse,
			"orders_scope":        scope,
			"highlight_order_id":  result.OrderID,
		},
		Deliveries: []messagecenterapp.DeliveryCommand{{
			Channel:    messagecenterapp.ChannelERPPlatform,
			TargetType: "permission",
			TargetKey:  "orders.read",
		}},
	})
}

func miniLoginError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniLoginDisabled) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "mini login disabled"})
	}
	if errors.Is(err, customerportalapp.ErrMiniUserDisabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "mini user disabled"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
		return miniCustomerConfigUpdatedError(c)
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return miniInternalError(c)
}

func miniPasswordLoginError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniInvalidLogin) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid login"})
	}
	if errors.Is(err, customerportalapp.ErrMiniAccountLoginDisabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "login disabled"})
	}
	if errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	if errors.Is(err, customerportalapp.ErrMiniUserDisabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "mini user disabled"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
		return miniCustomerConfigUpdatedError(c)
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return miniInternalError(c)
}

func miniSessionError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
		return miniCustomerConfigUpdatedError(c)
	}
	return miniInternalError(c)
}

func miniSwitchCustomerError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	if errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
		return miniCustomerConfigUpdatedError(c)
	}
	return miniInternalError(c)
}

func miniBusinessError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	if errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityNotEnabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "capability not enabled"})
	}
	if errors.Is(err, customerportalapp.ErrBeanListPublicationNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "bean list publication not found"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
		return miniCustomerConfigUpdatedError(c)
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return miniInternalError(c)
}

func miniCustomerConfigUpdatedError(c echo.Context) error {
	return c.JSON(http.StatusConflict, map[string]string{"error": miniCustomerConfigUpdatedMessage})
}

func miniInternalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func isMiniValidationError(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "code required", "openid required", "customer required", "mini token required",
		"phone required", "phone_code required", "password required", "phone verification unavailable",
		"login mode invalid",
		"service key invalid", "source_name required", "total_rows invalid", "input_material required",
		"input_qty required", "target_product required", "target_spec required", "target_qty required",
		"input material unavailable", "target product unavailable",
		"bean_list required", "recipient_name required", "recipient_phone required", "recipient_address required",
		"items required", "mall_product required", "qty required", "product unavailable", "mall product unavailable":
		return true
	default:
		return false
	}
}

func miniTokenFromHeader(authz string) string {
	authz = strings.TrimSpace(authz)
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}
