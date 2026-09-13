package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type ProductionLogRow = productionapp.ProductionLogRow
type productionProductOption = productionapp.ProductionLogProductOption

type ProductionLogsPageData struct {
	From          string
	To            string
	ProductID     int64
	BatchID       string
	Operator      string
	RunningItemID int64
	Products      []productionProductOption
	Rows          []ProductionLogRow
	Error         string
}

type ProductionLogsAPIResponse struct {
	Products   []productionProductOption `json:"products"`
	Rows       []ProductionLogRow        `json:"rows"`
	Total      int                       `json:"total"`
	Page       int                       `json:"page"`
	Limit      int                       `json:"limit"`
	TotalPages int                       `json:"total_pages"`
}

func registerProductionLogPages(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/produce/logs", func(c echo.Context) error {
		target := "/vue-shell?view=produceLogs"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})

	e.GET("/api/produce/logs", func(c echo.Context) error {
		query := parseProductionLogsQuery(c)
		if err := productionapp.ValidateProductionLogsQuery(query); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		result, err := productionSvc.ListProductionLogs(c.Request().Context(), query)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		page, limit := result.Page, result.Limit
		if page < 1 {
			page = query.Page
		}
		if limit < 1 {
			limit = query.Limit
		}
		totalPages := (result.Total + limit - 1) / limit
		if totalPages < 1 {
			totalPages = 1
		}
		return c.JSON(http.StatusOK, ProductionLogsAPIResponse{Products: result.Products, Rows: result.Rows, Total: result.Total, Page: page, Limit: limit, TotalPages: totalPages})
	})
}

func parseProductionLogsQuery(c echo.Context) productionapp.ProductionLogsQuery {
	data := productionapp.ProductionLogsQuery{
		Search:   strings.TrimSpace(c.QueryParam("q")),
		Page:     support.IntParam(c, "page", 1),
		Limit:    support.IntParam(c, "limit", 200),
		From:     strings.TrimSpace(c.QueryParam("from")),
		To:       strings.TrimSpace(c.QueryParam("to")),
		BatchID:  strings.TrimSpace(c.QueryParam("batch_id")),
		Operator: strings.TrimSpace(c.QueryParam("operator")),
	}
	if v := strings.TrimSpace(c.QueryParam("product_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.ProductID = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("running_item_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.RunningItemID = n
		}
	}
	if data.Page < 1 {
		data.Page = 1
	}
	if data.Limit < 1 || data.Limit > 500 {
		data.Limit = 200
	}
	data.WorkOrderID, _ = strconv.ParseInt(c.QueryParam("work_order_id"), 10, 64)
	return data
}
