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
		{"/stock/wip", "wipMaterials"},
		{"/stock/material-receipts", "materialReceipts"},
		{"/stock/material-batches", "materialBatches"},
		{"/stock/adjustments", "stockAdjustments"},
		{"/stock/outbound-logs", "stockOutboundLogs"},
	} {
		path, view := route.path, route.view
		e.GET(path, func(c echo.Context) error {
			target := "/vue-shell?view=" + view
			if raw := c.QueryString(); raw != "" {
				target += "&" + raw
			}
			return c.Redirect(http.StatusFound, target)
		})
	}
}

func registerStockAPI(e *echo.Echo, stockSvc *stockapp.Service) {
	e.GET("/api/stock/ledger", func(c echo.Context) error {
		result, err := stockSvc.ListLedger(c.Request().Context(), stockapp.LedgerQuery{
			Q:             strings.TrimSpace(c.QueryParam("q")),
			ItemType:      strings.TrimSpace(c.QueryParam("item_type")),
			Warehouse:     strings.TrimSpace(c.QueryParam("warehouse")),
			SourceDocType: strings.TrimSpace(c.QueryParam("source_doc_type")),
			SourceBatch:   strings.TrimSpace(c.QueryParam("source_batch")),
			From:          strings.TrimSpace(c.QueryParam("from")),
			To:            strings.TrimSpace(c.QueryParam("to")),
			Limit:         support.IntParam(c, "limit", 100),
			Offset:        stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/batches", func(c echo.Context) error {
		result, err := stockSvc.ListBatches(c.Request().Context(), stockapp.BatchQuery{
			Q:        strings.TrimSpace(c.QueryParam("q")),
			ItemType: strings.TrimSpace(c.QueryParam("item_type")),
			Limit:    support.IntParam(c, "limit", 100),
			Offset:   stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/material-batches", func(c echo.Context) error {
		result, err := stockSvc.ListMaterialBatches(c.Request().Context(), stockapp.MaterialBatchQuery{
			Q:          strings.TrimSpace(c.QueryParam("q")),
			MaterialID: int64(support.IntParam(c, "material_id", 0)),
			ActiveOnly: strings.TrimSpace(c.QueryParam("active_only")) == "1",
			Limit:      support.IntParam(c, "limit", 100),
			Offset:     stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/warehouses", func(c echo.Context) error {
		rows, err := stockSvc.ListWarehouses(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/stock/material-batch-locations", func(c echo.Context) error {
		result, err := stockSvc.ListMaterialBatchLocations(c.Request().Context(), stockapp.MaterialBatchLocationQuery{
			Q:          strings.TrimSpace(c.QueryParam("q")),
			MaterialID: int64(support.IntParam(c, "material_id", 0)),
			Warehouse:  strings.TrimSpace(c.QueryParam("warehouse")),
			ActiveOnly: strings.TrimSpace(c.QueryParam("active_only")) == "1",
			Limit:      support.IntParam(c, "limit", 100),
			Offset:     stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/warehouse-inventory", func(c echo.Context) error {
		result, err := stockSvc.ListWarehouseInventory(c.Request().Context(), stockapp.WarehouseInventoryQuery{
			Q:         strings.TrimSpace(c.QueryParam("q")),
			Warehouse: strings.TrimSpace(c.QueryParam("warehouse")),
			ItemType:  strings.TrimSpace(c.QueryParam("item_type")),
			Limit:     support.IntParam(c, "limit", 100),
			Offset:    stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/stock/outbound-logs", func(c echo.Context) error {
		result, err := stockSvc.ListOutboundLogs(c.Request().Context(), stockapp.OutboundLogQuery{
			Q:      strings.TrimSpace(c.QueryParam("q")),
			From:   strings.TrimSpace(c.QueryParam("from")),
			To:     strings.TrimSpace(c.QueryParam("to")),
			Limit:  support.IntParam(c, "limit", 100),
			Offset: stockOffset(c),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
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
			MaterialID int64   `json:"material_id"`
			Supplier   string  `json:"supplier"`
			QtyG       int64   `json:"qty_g"`
			UnitCost   float64 `json:"unit_cost"`
			Note       string  `json:"note"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.ReceiveMaterial(c.Request().Context(), stockapp.MaterialReceiptCommand{
			MaterialID: req.MaterialID,
			Supplier:   req.Supplier,
			QtyG:       req.QtyG,
			UnitCost:   req.UnitCost,
			Note:       req.Note,
			Operator:   support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/stock/adjustments", func(c echo.Context) error {
		var req struct {
			ItemType    string `json:"item_type"`
			ItemID      int64  `json:"item_id"`
			SpecG       int64  `json:"spec_g"`
			Warehouse   string `json:"warehouse"`
			TargetG     int64  `json:"target_g"`
			TargetUnits int64  `json:"target_units"`
			Reason      string `json:"reason"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		result, err := stockSvc.CreateAdjustment(c.Request().Context(), stockapp.StockAdjustmentCommand{
			ItemType:    req.ItemType,
			ItemID:      req.ItemID,
			SpecG:       req.SpecG,
			Warehouse:   req.Warehouse,
			TargetG:     req.TargetG,
			TargetUnits: req.TargetUnits,
			Reason:      req.Reason,
			Operator:    support.ActorOf(c),
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
	offset := support.IntParam(c, "offset", 0)
	if page := support.IntParam(c, "page", 0); page > 0 {
		offset = (page - 1) * limit
	}
	if offset < 0 {
		return 0
	}
	return offset
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
