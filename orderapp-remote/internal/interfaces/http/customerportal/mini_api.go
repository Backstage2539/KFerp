package customerportal

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniLoginRequest struct {
	Code     string `json:"code"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
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
	ShippingAmount   float64 `json:"shipping_amount"`
	Note             string  `json:"note"`
}

func registerMiniAPI(e *echo.Echo, svc Service, beanListPDFRenderer BeanListPDFRenderer) {
	e.POST("/api/mini/login", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		var req miniLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.Login(c.Request().Context(), customerportalapp.LoginCommand{Code: req.Code, Phone: req.Phone, Nickname: req.Nickname})
		if err != nil {
			return miniLoginError(c, err)
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
			UnitPrice:         req.UnitPrice,
			ShippingAmount:    req.ShippingAmount,
			Note:              req.Note,
		})
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
}

func miniLoginError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniLoginDisabled) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "mini login disabled"})
	}
	if errors.Is(err, customerportalapp.ErrMiniUserDisabled) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "mini user disabled"})
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
	return miniInternalError(c)
}

func miniSwitchCustomerError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired mini token"})
	}
	if errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
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
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return miniInternalError(c)
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
		"service key invalid", "source_name required", "total_rows invalid", "input_material required",
		"input_qty required", "target_product required", "target_spec required", "target_qty required",
		"bean_list required":
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
