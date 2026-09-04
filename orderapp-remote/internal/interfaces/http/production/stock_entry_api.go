package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type stockEntryRequest struct {
	EntryType      string                              `json:"entry_type"`
	Purpose        string                              `json:"purpose"`
	WorkOrderID    int64                               `json:"work_order_id"`
	JobCardID      int64                               `json:"job_card_id"`
	RunningItemID  int64                               `json:"running_item_id"`
	SourceType     string                              `json:"source_type"`
	SourceID       int64                               `json:"source_id"`
	Note           string                              `json:"note"`
	IsReturn       bool                                `json:"is_return"`
	ReturnSource   string                              `json:"return_source"`
	IdempotencyKey string                              `json:"idempotency_key"`
	CustomerID     int64                               `json:"customer_id"`
	Items          []stockapp.StockDocumentItemCommand `json:"items"`
}

// Accepted new-write purpose values include material_issue, material_transfer,
// material_transfer_for_manufacture, material_return_from_manufacture,
// material_consumption_for_manufacture, manufacture, and the legacy finished_transfer alias.
// Inventory reconciliation remains a separate stock-adjustment workflow.
// PR-479 compatibility markers: material_issue_to_wip, finished_receipt.
func registerStockEntryAPI(e *echo.Echo, productionSvc *productionapp.Service, stockServices ...*stockapp.Service) {
	var stockSvc *stockapp.Service
	if len(stockServices) > 0 {
		stockSvc = stockServices[0]
	}
	bindStockDocumentCommand := func(c echo.Context) (stockapp.StockDocumentCommand, error) {
		var req stockEntryRequest
		if err := c.Bind(&req); err != nil {
			return stockapp.StockDocumentCommand{}, err
		}
		return stockapp.StockDocumentCommand{
			EntryType:      req.EntryType,
			Purpose:        req.Purpose,
			IsReturn:       req.IsReturn,
			WorkOrderID:    req.WorkOrderID,
			JobCardID:      req.JobCardID,
			RunningItemID:  req.RunningItemID,
			SourceType:     req.SourceType,
			SourceID:       req.SourceID,
			ReturnSource:   req.ReturnSource,
			Operator:       support.ActorOf(c),
			Note:           req.Note,
			IdempotencyKey: req.IdempotencyKey,
			CustomerID:     req.CustomerID,
			Items:          req.Items,
		}, nil
	}
	isRetiredMaterialReceipt := func(cmd stockapp.StockDocumentCommand) bool {
		return strings.TrimSpace(cmd.Purpose) == stockapp.PurposeMaterialReceipt || strings.TrimSpace(cmd.EntryType) == stockapp.PurposeMaterialReceipt
	}
	createStockDocument := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		cmd, err := bindStockDocumentCommand(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if isRetiredMaterialReceipt(cmd) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "普通原料入库已停用，请前往采购入库重建"})
		}
		detail, err := stockSvc.CreateStockDocumentDraft(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	updateStockDocument := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid stock_entry_id"})
		}
		cmd, err := bindStockDocumentCommand(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if isRetiredMaterialReceipt(cmd) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "历史原料入库草稿不能继续编辑，请前往采购入库重建"})
		}
		detail, err := stockSvc.UpdateStockDocumentDraft(c.Request().Context(), id, cmd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	submitStockDocument := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid stock_entry_id"})
		}
		detail, err := stockSvc.GetStockDocument(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if detail.Status == "submitted" {
			return c.JSON(http.StatusOK, detail)
		}
		if detail.Purpose == stockapp.PurposeMaterialReceipt {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "历史原料入库草稿不能继续提交，请前往采购入库重建"})
		}
		if detail.Purpose == stockapp.PurposeManufacture && detail.WorkOrderID > 0 {
			workOrderDetail, err := productionSvc.GetWorkOrderDetail(c.Request().Context(), detail.WorkOrderID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			if len(detail.Items) != 1 {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "manufacture stock document must contain exactly one frozen output item"})
			}
			item := detail.Items[0]
			complete := productionapp.WorkOrderCompleteCommand{
				ID: detail.WorkOrderID, StockDocumentID: detail.ID,
				Warehouse: item.ToWarehouse, Operator: support.ActorOf(c), Note: detail.Note,
			}
			if strings.EqualFold(strings.TrimSpace(workOrderDetail.WorkOrder.OutputType), "material") {
				complete.FinishedQtyG = item.QtyG
				complete.FinishedQtyUnits = item.QtyUnits
			} else {
				complete.FinishedUnits = item.QtyUnits
				complete.FinishedLooseG = item.QtyG
			}
			if _, err := productionSvc.CompleteWorkOrder(c.Request().Context(), complete); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			detail, err = stockSvc.GetStockDocument(c.Request().Context(), id)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusOK, detail)
		}
		detail, err = stockSvc.SubmitStockDocument(c.Request().Context(), id, support.ActorOf(c))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	cancelStockDocument := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid stock_entry_id"})
		}
		detail, err := stockSvc.GetStockDocument(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if detail.Purpose == stockapp.PurposeMaterialReceipt {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "历史原料入库单据只读，不能取消"})
		}
		detail, err = stockSvc.CancelStockDocument(c.Request().Context(), id, support.ActorOf(c))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	listStockDocuments := func(c echo.Context) error {
		result, err := stockSvc.ListStockDocuments(c.Request().Context(), stockapp.StockDocumentQuery{
			Q:           strings.TrimSpace(c.QueryParam("q")),
			Purpose:     strings.TrimSpace(c.QueryParam("purpose")),
			Status:      strings.TrimSpace(c.QueryParam("status")),
			WorkOrderID: parseInt64(c.QueryParam("work_order_id")),
			JobCardID:   parseInt64(c.QueryParam("job_card_id")),
			Limit:       support.IntParam(c, "limit", 200),
			Offset:      support.IntParam(c, "offset", 0),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	}
	getStockDocument := func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid stock_entry_id"})
		}
		detail, err := stockSvc.GetStockDocument(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	if stockSvc != nil {
		e.POST("/api/stock-entries", func(c echo.Context) error {
			if err := support.RequireEmployeeBound(c); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			cmd, err := bindStockDocumentCommand(c)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			}
			if isRetiredMaterialReceipt(cmd) {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "普通原料入库已停用，请前往采购入库重建"})
			}
			detail, err := stockSvc.CreateAndSubmitStockDocument(c.Request().Context(), cmd)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusOK, detail)
		})
		e.GET("/api/stock-entries", listStockDocuments)
		e.GET("/api/stock-entries/:id", getStockDocument)
		e.POST("/api/stock-documents", createStockDocument)
		e.PUT("/api/stock-documents/:id", updateStockDocument)
		e.POST("/api/stock-documents/:id/submit", submitStockDocument)
		e.POST("/api/stock-documents/:id/cancel", cancelStockDocument)
		e.GET("/api/stock-documents", listStockDocuments)
		e.GET("/api/stock-documents/:id", getStockDocument)
		return
	}
	createLegacy := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req stockEntryRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		items := make([]productionapp.StockEntryItemCommand, 0, len(req.Items))
		for _, item := range req.Items {
			items = append(items, productionapp.StockEntryItemCommand{
				MaterialID: item.MaterialID, ProductID: item.ProductID, ItemType: item.ItemType, ItemName: item.ItemName,
				SpecG: item.SpecG, FromWarehouse: item.FromWarehouse, ToWarehouse: item.ToWarehouse,
				QtyG: item.QtyG, QtyUnits: item.QtyUnits, BatchCode: item.BatchCode, UnitCost: item.UnitCost,
			})
		}
		detail, err := productionSvc.CreateStockEntry(c.Request().Context(), productionapp.StockEntryCommand{
			EntryType: req.EntryType, Purpose: req.Purpose, WorkOrderID: req.WorkOrderID, JobCardID: req.JobCardID,
			RunningItemID: req.RunningItemID, SourceType: req.SourceType, SourceID: req.SourceID,
			Operator: support.ActorOf(c), Note: req.Note, Items: items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	listLegacy := func(c echo.Context) error {
		rows, err := productionSvc.ListStockEntries(c.Request().Context(), productionapp.StockEntryQuery{
			EntryType: strings.TrimSpace(c.QueryParam("entry_type")), Purpose: strings.TrimSpace(c.QueryParam("purpose")),
			Status: strings.TrimSpace(c.QueryParam("status")), WorkOrderID: parseInt64(c.QueryParam("work_order_id")),
			JobCardID: parseInt64(c.QueryParam("job_card_id")), Limit: support.IntParam(c, "limit", 200),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	}
	getLegacy := func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid stock_entry_id"})
		}
		detail, err := productionSvc.GetStockEntry(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	e.POST("/api/stock-entries", createLegacy)
	e.GET("/api/stock-entries", listLegacy)
	e.GET("/api/stock-entries/:id", getLegacy)
	e.POST("/api/stock-documents", createLegacy)
	e.GET("/api/stock-documents", listLegacy)
	e.GET("/api/stock-documents/:id", getLegacy)
}
