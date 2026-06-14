package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type stockEntryRequest struct {
	EntryType     string                                `json:"entry_type"`
	Purpose       string                                `json:"purpose"`
	WorkOrderID   int64                                 `json:"work_order_id"`
	JobCardID     int64                                 `json:"job_card_id"`
	RunningItemID int64                                 `json:"running_item_id"`
	SourceType    string                                `json:"source_type"`
	SourceID      int64                                 `json:"source_id"`
	Note          string                                `json:"note"`
	Items         []productionapp.StockEntryItemCommand `json:"items"`
}

// Accepted purpose values include material_transfer_for_manufacture, material_return_from_manufacture,
// material_consumption_for_manufacture, manufacture, stock_adjustment, and finished_transfer.
// PR-479 compatibility markers: material_issue_to_wip, finished_receipt.
func registerStockEntryAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	createStockDocument := func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req stockEntryRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		detail, err := productionSvc.CreateStockEntry(c.Request().Context(), productionapp.StockEntryCommand{
			EntryType:     req.EntryType,
			Purpose:       req.Purpose,
			WorkOrderID:   req.WorkOrderID,
			JobCardID:     req.JobCardID,
			RunningItemID: req.RunningItemID,
			SourceType:    req.SourceType,
			SourceID:      req.SourceID,
			Operator:      support.ActorOf(c),
			Note:          req.Note,
			Items:         req.Items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	}
	listStockDocuments := func(c echo.Context) error {
		rows, err := productionSvc.ListStockEntries(c.Request().Context(), productionapp.StockEntryQuery{
			EntryType:   strings.TrimSpace(c.QueryParam("entry_type")),
			Purpose:     strings.TrimSpace(c.QueryParam("purpose")),
			Status:      strings.TrimSpace(c.QueryParam("status")),
			WorkOrderID: parseInt64(c.QueryParam("work_order_id")),
			JobCardID:   parseInt64(c.QueryParam("job_card_id")),
			Limit:       support.IntParam(c, "limit", 200),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	}
	getStockDocument := func(c echo.Context) error {
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
	e.POST("/api/stock-entries", createStockDocument)
	e.GET("/api/stock-entries", listStockDocuments)
	e.GET("/api/stock-entries/:id", getStockDocument)
	e.POST("/api/stock-documents", createStockDocument)
	e.GET("/api/stock-documents", listStockDocuments)
	e.GET("/api/stock-documents/:id", getStockDocument)
}
