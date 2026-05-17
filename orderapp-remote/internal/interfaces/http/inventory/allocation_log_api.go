package inventory

import (
	"net/http"
	"strconv"
	"strings"

	inventoryapp "orderapp/internal/application/inventory"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type AllocationLogViewRow = inventoryapp.AllocationLogRow
type AllocationBatchRow = inventoryapp.AllocationBatchRow

type AllocationLogPageData struct {
	BatchID  string
	Rows     []AllocationLogViewRow
	Batches  []AllocationBatchRow
	Page     int
	PerPage  int
	HasNext  bool
	PrevPage int
	NextPage int
	Error    string
}

type AllocationLogAPIResponse struct {
	BatchID    string                 `json:"batch_id"`
	Rows       []AllocationLogViewRow `json:"rows"`
	Batches    []AllocationBatchRow   `json:"batches"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"per_page"`
	Limit      int                    `json:"limit"`
	Total      int                    `json:"total"`
	TotalPages int                    `json:"total_pages"`
	HasNext    bool                   `json:"has_next"`
	HasPrev    bool                   `json:"has_prev"`
	PrevPage   int                    `json:"prev_page"`
	NextPage   int                    `json:"next_page"`
}

func registerAllocationLogPages(e *echo.Echo, inventorySvc *inventoryapp.Service) {
	e.GET("/produce/allocations", func(c echo.Context) error {
		target := "/vue-shell?view=allocationLogs"
		if raw := strings.TrimSpace(c.QueryString()); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})

	e.GET("/api/produce/allocations", func(c echo.Context) error {
		page := 1
		per := 20
		if v := strings.TrimSpace(c.QueryParam("page")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				page = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("per_page")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				per = n
			}
		}

		offset := (page - 1) * per
		result, err := inventorySvc.ListAllocations(c.Request().Context(), inventoryapp.AllocationLogQuery{
			BatchID: strings.TrimSpace(c.QueryParam("batch")),
			Limit:   per,
			Offset:  offset,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		totalPages := allocationPageCount(result.Total, per)
		prevPage := 0
		if page > 1 {
			prevPage = page - 1
		}
		return c.JSON(http.StatusOK, AllocationLogAPIResponse{
			BatchID:    result.BatchID,
			Rows:       result.Rows,
			Batches:    result.Batches,
			Page:       page,
			PerPage:    per,
			Limit:      per,
			Total:      result.Total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
			PrevPage:   prevPage,
			NextPage:   page + 1,
		})
	})
}

func allocationPageCount(total, limit int) int {
	if limit <= 0 {
		limit = 20
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}
