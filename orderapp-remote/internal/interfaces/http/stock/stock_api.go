package stock

import (
	"net/http"
	"net/url"
	stockapp "orderapp/internal/application/stock"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

type errorResponse struct {
	Error string `json:"error"`
}

func registerStockPages(e *echo.Echo) {
	for _, route := range []struct {
		path string
		view string
	}{
		{"/stock/ledger", "stockLedger"},
		{"/stock/batches", "stockBatches"},
		{"/stock/wip", "stockOperations&tab=stockEntries&action=issue"},
		{"/stock/material-receipts", "stockOperations&tab=stockEntries&action=receipt"},
		{"/stock/material-batches", "materialBatches"},
		{"/stock/adjustments", "stockOperations&tab=adjustments"},
		{"/stock/outbound-logs", "stockOutboundLogs"},
	} {
		path, view := route.path, route.view
		e.GET(path, func(c echo.Context) error {
			target := "/vue-shell?view=" + view
			if raw := c.QueryString(); raw != "" {
				target += "&" + raw
			}
			return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
		})
	}
}

func registerStockAPI(e *echo.Echo, stockSvc *stockapp.Service) {
	e.GET("/api/stock/ledger", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListLedger(c.Request().Context(), stockapp.LedgerQuery{
			Q:             strings.TrimSpace(c.QueryParam("q")),
			ItemType:      strings.TrimSpace(c.QueryParam("item_type")),
			Warehouse:     strings.TrimSpace(c.QueryParam("warehouse")),
			SourceDocType: strings.TrimSpace(c.QueryParam("source_doc_type")),
			SourceBatch:   strings.TrimSpace(c.QueryParam("source_batch")),
			From:          strings.TrimSpace(c.QueryParam("from")),
			To:            strings.TrimSpace(c.QueryParam("to")),
			Limit:         limit,
			Offset:        offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, limit, offset)
		result.Limit = limit
		result.Offset = offset
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/batches", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListBatches(c.Request().Context(), stockapp.BatchQuery{
			Q:        strings.TrimSpace(c.QueryParam("q")),
			ItemType: strings.TrimSpace(c.QueryParam("item_type")),
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, limit, offset)
		result.Limit = limit
		result.Offset = offset
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/material-batches", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListMaterialBatches(c.Request().Context(), stockapp.MaterialBatchQuery{
			Q:          strings.TrimSpace(c.QueryParam("q")),
			MaterialID: int64(support.IntParam(c, "material_id", 0)),
			ActiveOnly: strings.TrimSpace(c.QueryParam("active_only")) == "1",
			Limit:      limit,
			Offset:     offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, limit, offset)
		result.Limit = limit
		result.Offset = offset
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/warehouses", func(c echo.Context) error {
		rows, err := stockSvc.ListWarehouses(c.Request().Context(), stockapp.WarehouseListQuery{
			CustomerID: int64(support.IntParam(c, "customer_id", 0)),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.PUT("/api/stock/warehouses/:code/customer", func(c echo.Context) error {
		var req struct {
			CustomerID int64 `json:"customer_id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		row, err := stockSvc.BindWarehouseCustomer(c.Request().Context(), stockapp.BindWarehouseCustomerCommand{
			WarehouseCode: strings.TrimSpace(c.Param("code")),
			CustomerID:    req.CustomerID,
			Actor:         support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/stock/material-batch-locations", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListMaterialBatchLocations(c.Request().Context(), stockapp.MaterialBatchLocationQuery{
			Q:          strings.TrimSpace(c.QueryParam("q")),
			MaterialID: int64(support.IntParam(c, "material_id", 0)),
			Warehouse:  strings.TrimSpace(c.QueryParam("warehouse")),
			ActiveOnly: strings.TrimSpace(c.QueryParam("active_only")) == "1",
			Limit:      limit,
			Offset:     offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, limit, offset)
		result.Limit = limit
		result.Offset = offset
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/warehouse-inventory", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListWarehouseInventory(c.Request().Context(), stockapp.WarehouseInventoryQuery{
			Q:           strings.TrimSpace(c.QueryParam("q")),
			Warehouse:   strings.TrimSpace(c.QueryParam("warehouse")),
			ItemType:    strings.TrimSpace(c.QueryParam("item_type")),
			CustomerID:  int64(support.IntParam(c, "customer_id", 0)),
			GroupID:     int64(support.IntParam(c, "group_id", 0)),
			GroupItemID: int64(support.IntParam(c, "group_item_id", 0)),
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		result.Page, result.TotalPages, result.HasNext = stockPaginationValues(result.Total, limit, offset)
		result.Limit = limit
		result.Offset = offset
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/outbound-logs", func(c echo.Context) error {
		limit := stockLimit(c)
		offset := stockOffsetForLimit(c, limit)
		result, err := stockSvc.ListOutboundLogs(c.Request().Context(), stockapp.OutboundLogQuery{
			Q:      strings.TrimSpace(c.QueryParam("q")),
			From:   strings.TrimSpace(c.QueryParam("from")),
			To:     strings.TrimSpace(c.QueryParam("to")),
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		page, totalPages, hasNext := stockPaginationValues(result.Total, limit, offset)
		result.Page = page
		result.Limit = limit
		result.Offset = offset
		result.TotalPages = totalPages
		result.HasNext = hasNext
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/trace", func(c echo.Context) error {
		result, err := stockSvc.GetStockTrace(c.Request().Context(), stockapp.StockTraceQuery{
			BatchCode: strings.TrimSpace(c.QueryParam("batch")),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/stock/material-receipts", func(c echo.Context) error {
		var req struct {
			MaterialID                int64   `json:"material_id"`
			Supplier                  string  `json:"supplier"`
			Qty                       float64 `json:"qty"`
			UnitCode                  string  `json:"unit_code"`
			QtyG                      int64   `json:"qty_g"`
			UnitCost                  float64 `json:"unit_cost"`
			CropSeason                string  `json:"crop_season"`
			Origin                    string  `json:"origin"`
			ProducerFlavorDescription string  `json:"producer_flavor_description"`
			Note                      string  `json:"note"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.ReceiveMaterial(c.Request().Context(), stockapp.MaterialReceiptCommand{
			MaterialID:                req.MaterialID,
			Supplier:                  req.Supplier,
			Qty:                       req.Qty,
			UnitCode:                  req.UnitCode,
			QtyG:                      req.QtyG,
			UnitCost:                  req.UnitCost,
			CropSeason:                req.CropSeason,
			Origin:                    req.Origin,
			ProducerFlavorDescription: req.ProducerFlavorDescription,
			Note:                      req.Note,
			Operator:                  support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/stock/adjustments", func(c echo.Context) error {
		var req struct {
			AdjustmentType  string   `json:"adjustment_type"`
			ItemType        string   `json:"item_type"`
			ItemID          int64    `json:"item_id"`
			SpecG           int64    `json:"spec_g"`
			Warehouse       string   `json:"warehouse"`
			TargetG         int64    `json:"target_g"`
			TargetUnits     int64    `json:"target_units"`
			TargetQty       *float64 `json:"target_qty"`
			UnitCode        string   `json:"unit_code"`
			MaterialBatchID int64    `json:"material_batch_id"`
			TargetUnitCost  float64  `json:"target_unit_cost"`
			Reason          string   `json:"reason"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.CreateAdjustment(c.Request().Context(), stockapp.StockAdjustmentCommand{
			AdjustmentType:  req.AdjustmentType,
			ItemType:        req.ItemType,
			ItemID:          req.ItemID,
			SpecG:           req.SpecG,
			Warehouse:       req.Warehouse,
			TargetG:         req.TargetG,
			TargetUnits:     req.TargetUnits,
			TargetQty:       valueOfFloat64(req.TargetQty),
			HasTargetQty:    req.TargetQty != nil,
			UnitCode:        req.UnitCode,
			MaterialBatchID: req.MaterialBatchID,
			TargetUnitCost:  req.TargetUnitCost,
			Reason:          req.Reason,
			Operator:        support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/stock/finished-transfers", func(c echo.Context) error {
		var req struct {
			ProductID      int64  `json:"product_id"`
			SpecG          int64  `json:"spec_g"`
			FromWarehouse  string `json:"from_warehouse"`
			ToWarehouse    string `json:"to_warehouse"`
			QtyUnits       int64  `json:"qty_units"`
			QtyLooseG      int64  `json:"qty_loose_g"`
			Note           string `json:"note"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.TransferFinishedProduct(c.Request().Context(), stockapp.FinishedProductTransferCommand{
			ProductID:      req.ProductID,
			SpecG:          req.SpecG,
			FromWarehouse:  req.FromWarehouse,
			ToWarehouse:    req.ToWarehouse,
			QtyUnits:       req.QtyUnits,
			QtyLooseG:      req.QtyLooseG,
			Note:           req.Note,
			Operator:       support.ActorOf(c),
			IdempotencyKey: req.IdempotencyKey,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/stock/material-transfers", func(c echo.Context) error {
		var req struct {
			MaterialID     int64  `json:"material_id"`
			FromWarehouse  string `json:"from_warehouse"`
			ToWarehouse    string `json:"to_warehouse"`
			QtyG           int64  `json:"qty_g"`
			Note           string `json:"note"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.TransferMaterial(c.Request().Context(), stockapp.MaterialTransferCommand{
			MaterialID:     req.MaterialID,
			FromWarehouse:  req.FromWarehouse,
			ToWarehouse:    req.ToWarehouse,
			QtyG:           req.QtyG,
			Note:           req.Note,
			Operator:       support.ActorOf(c),
			IdempotencyKey: req.IdempotencyKey,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})
}

func stockOffset(c echo.Context) int {
	limit := support.IntParam(c, "limit", 100)
	return stockOffsetForLimit(c, limit)
}

func valueOfFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func stockLimit(c echo.Context) int {
	limit := support.IntParam(c, "limit", 100)
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func stockOffsetForLimit(c echo.Context, limit int) int {
	if limit <= 0 {
		limit = 100
	}
	offset := support.IntParam(c, "offset", 0)
	if page := support.IntParam(c, "page", 0); page > 0 {
		offset = (page - 1) * limit
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func stockPageCount(total, limit int) int {
	if limit <= 0 {
		limit = 100
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}

func stockPaginationValues(total, limit, offset int) (int, int, bool) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	page := (offset / limit) + 1
	totalPages := stockPageCount(total, limit)
	return page, totalPages, page < totalPages
}

func vueStockURL(view string, values url.Values) string {
	target := "/vue-shell?view=" + url.QueryEscape(view)
	for key, vals := range values {
		for _, val := range vals {
			target += "&" + url.QueryEscape(key) + "=" + url.QueryEscape(val)
		}
	}
	return target
}
