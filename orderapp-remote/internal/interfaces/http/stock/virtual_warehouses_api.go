package stock

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	stockapp "orderapp/internal/application/stock"
	support "orderapp/internal/interfaces/http/support"
)

func virtualWarehouseError(c echo.Context, err error) error {
	status := http.StatusBadRequest
	if errors.Is(err, stockapp.ErrWarehouseConflict) {
		status = http.StatusConflict
	}
	if errors.Is(err, stockapp.ErrVirtualWarehouseNotFound) {
		status = http.StatusNotFound
	}
	return c.JSON(status, errorResponse{Error: err.Error()})
}

func registerVirtualWarehouseAPI(e *echo.Echo, svc *stockapp.Service) {
	e.GET("/api/stock/virtual-warehouses", func(c echo.Context) error {
		var customerID *int64
		if values, present := c.QueryParams()["customer_id"]; present {
			if len(values) != 1 {
				return c.JSON(http.StatusBadRequest, errorResponse{Error: "客户编号必须为非负整数"})
			}
			id, err := strconv.ParseInt(strings.TrimSpace(values[0]), 10, 64)
			if err != nil || id < 0 {
				return c.JSON(http.StatusBadRequest, errorResponse{Error: "客户编号必须为非负整数"})
			}
			customerID = &id
		}
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := svc.ListVirtualWarehouses(c.Request().Context(), stockapp.VirtualWarehouseQuery{Q: c.QueryParam("q"), CustomerID: customerID, Limit: limit, Offset: offset})
		if err != nil {
			return virtualWarehouseError(c, err)
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, result.Limit, result.Offset)
		return c.JSON(http.StatusOK, result)
	})
	e.POST("/api/stock/virtual-warehouses", func(c echo.Context) error {
		var req struct {
			Name        string `json:"name"`
			Kind        string `json:"kind"`
			Description string `json:"description"`
			CustomerID  int64  `json:"customer_id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		row, err := svc.CreateVirtualWarehouse(c.Request().Context(), stockapp.CreateVirtualWarehouseCommand{Name: req.Name, Kind: req.Kind, Description: req.Description, CustomerID: req.CustomerID, Actor: support.ActorOf(c)})
		if err != nil {
			return virtualWarehouseError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.PUT("/api/stock/virtual-warehouses/:code", func(c echo.Context) error {
		var req struct {
			Name             string  `json:"name"`
			Kind             *string `json:"kind"`
			Code             *string `json:"code"`
			IsVirtual        *bool   `json:"is_virtual"`
			Description      string  `json:"description"`
			CustomerID       int64   `json:"customer_id"`
			ExpectedRevision int64   `json:"expected_revision"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		if req.Kind != nil || req.Code != nil || req.IsVirtual != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "仓库编号、类型及虚拟分类不可修改"})
		}
		row, err := svc.UpdateVirtualWarehouse(c.Request().Context(), stockapp.UpdateVirtualWarehouseCommand{Code: c.Param("code"), Name: req.Name, Description: req.Description, CustomerID: req.CustomerID, ExpectedRevision: req.ExpectedRevision, Actor: support.ActorOf(c)})
		if err != nil {
			return virtualWarehouseError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.DELETE("/api/stock/virtual-warehouses/:code", func(c echo.Context) error {
		revision, err := strconv.ParseInt(c.QueryParam("expected_revision"), 10, 64)
		if err != nil || revision < 1 {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "配置版本必填"})
		}
		row, err := svc.DeleteVirtualWarehouse(c.Request().Context(), stockapp.DeleteVirtualWarehouseCommand{Code: c.Param("code"), ExpectedRevision: revision, Actor: support.ActorOf(c)})
		if err != nil {
			return virtualWarehouseError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
}
