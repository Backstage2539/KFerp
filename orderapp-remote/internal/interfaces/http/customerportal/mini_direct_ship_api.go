package customerportal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type MiniCustomerFulfillment interface {
	MiniDirectShipCatalog(context.Context, customerfulfillmentapp.MiniDirectShipCatalogQuery) (customerfulfillmentapp.MiniDirectShipCatalog, error)
	PreviewMiniDirectShip(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipPreview, error)
	SubmitMiniDirectShip(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipRequest, error)
	ListMiniDirectShipRequests(context.Context, int64, int) ([]customerfulfillmentapp.MiniDirectShipRequest, error)
	GetMiniDirectShipRequest(context.Context, int64, int64) (customerfulfillmentapp.MiniDirectShipRequest, error)
	CancelMiniDirectShipRequest(context.Context, int64, int64, string) (customerfulfillmentapp.MiniDirectShipRequest, error)
	ListCustomerCentralInventory(context.Context, int64) ([]customerfulfillmentapp.CustomerInventorySummary, error)
	ListCustomerCentralInventoryBatches(context.Context, int64, int64, int64) ([]customerfulfillmentapp.CustomerInventoryBatch, error)
}

func registerMiniCustomerFulfillmentAPI(e *echo.Echo, portal Service, fulfillment MiniCustomerFulfillment) {
	e.GET("/api/mini/direct-ship/catalog", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		result, err := fulfillment.MiniDirectShipCatalog(c.Request().Context(), customerfulfillmentapp.MiniDirectShipCatalogQuery{
			CustomerID: current.CurrentCustomerID,
			Q:          c.QueryParam("q"),
			Category:   c.QueryParam("category"),
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/direct-ship/preview", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := fulfillment.PreviewMiniDirectShip(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/direct-ship/requests", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := fulfillment.SubmitMiniDirectShip(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusCreated, result)
	})

	e.GET("/api/mini/direct-ship/requests", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		rows, err := fulfillment.ListMiniDirectShipRequests(c.Request().Context(), current.CurrentCustomerID, limit)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/mini/direct-ship/requests/:id", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || requestID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		row, err := fulfillment.GetMiniDirectShipRequest(c.Request().Context(), current.CurrentCustomerID, requestID)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"request": row})
	})

	e.POST("/api/mini/direct-ship/requests/:id/cancel", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || requestID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		row, err := fulfillment.CancelMiniDirectShipRequest(c.Request().Context(), current.CurrentCustomerID, requestID, miniCustomerFulfillmentActor(current))
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/mini/customer-inventory", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rows, err := fulfillment.ListCustomerCentralInventory(c.Request().Context(), current.CurrentCustomerID)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/mini/customer-inventory/:product_id/batches", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		productID, err := strconv.ParseInt(strings.TrimSpace(c.Param("product_id")), 10, 64)
		if err != nil || productID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		specG := int64(0)
		if raw := strings.TrimSpace(c.QueryParam("spec_g")); raw != "" {
			specG, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || specG <= 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
			}
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rows, err := fulfillment.ListCustomerCentralInventoryBatches(c.Request().Context(), current.CurrentCustomerID, productID, specG)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
}

func requireMiniCustomerFulfillmentContext(c echo.Context, portal Service, capabilities ...string) (customerportalapp.CurrentContext, bool, error) {
	if portal == nil {
		return customerportalapp.CurrentContext{}, false, miniInternalError(c)
	}
	token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
	}
	current, err := portal.Me(c.Request().Context(), token)
	if err != nil {
		return customerportalapp.CurrentContext{}, false, miniSessionError(c, err)
	}
	if current.CurrentCustomerID <= 0 {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	if !current.HasAnyCapability(capabilities) {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusForbidden, map[string]string{"error": "capability not enabled"})
	}
	return current, true, nil
}

func bindMiniDirectShipPrincipal(c echo.Context, current customerportalapp.CurrentContext, cmd *customerfulfillmentapp.MiniDirectShipCommand) {
	cmd.CustomerID = current.CurrentCustomerID
	cmd.EmployeeID = current.EmployeeID
	cmd.MiniUserID = current.MiniUserID
	cmd.Actor = miniCustomerFulfillmentActor(current)
	if cmd.IdempotencyKey == "" {
		cmd.IdempotencyKey = strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
	}
}

func miniCustomerFulfillmentActor(current customerportalapp.CurrentContext) string {
	if current.EmployeeID > 0 {
		return fmt.Sprintf("mini_employee:%d", current.EmployeeID)
	}
	return fmt.Sprintf("mini_user:%d", current.MiniUserID)
}

func miniCustomerFulfillmentError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipStockInsufficient):
		return c.JSON(http.StatusConflict, map[string]string{"error": "当前客户成品仓库存不足，无法提交发货"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipIdempotency):
		return c.JSON(http.StatusConflict, map[string]string{"error": "该发货请求已提交，不能使用同一请求编号修改内容"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipCannotCancel):
		return c.JSON(http.StatusConflict, map[string]string{"error": "包裹已发货，不能取消发货申请"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipRequestNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "未找到该发货申请"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipUnavailable):
		return miniInternalError(c)
	case miniCustomerFulfillmentValidationError(err):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "发货信息不完整，请检查收件信息和商品数量"})
	default:
		return miniInternalError(c)
	}
}

func miniCustomerFulfillmentValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, " required") || strings.Contains(message, " invalid") || strings.Contains(message, " too long") || strings.Contains(message, " unavailable")
}
