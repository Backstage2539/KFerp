package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type ProductionLogRow = productionapp.ProductionLogRow
type productionProductOption = productionapp.ProductionLogProductOption

type ProductionLogsPageData struct {
	From      string
	To        string
	ProductID int64
	BatchID   string
	Operator  string
	Products  []productionProductOption
	Rows      []ProductionLogRow
	Error     string
}

type ProductionLogsAPIResponse struct {
	Products []productionProductOption `json:"products"`
	Rows     []ProductionLogRow        `json:"rows"`
}

func registerProductionLogPages(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/produce/logs", func(c echo.Context) error {
		target := "/vue-shell?view=produceLogs"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})

	e.GET("/api/produce/logs", func(c echo.Context) error {
		query := parseProductionLogsQuery(c)
		result, err := productionSvc.ListProductionLogs(c.Request().Context(), productionapp.ProductionLogsQuery{
			From:      query.From,
			To:        query.To,
			ProductID: query.ProductID,
			BatchID:   query.BatchID,
			Operator:  query.Operator,
			Limit:     200,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProductionLogsAPIResponse{Products: result.Products, Rows: result.Rows})
	})
}

func parseProductionLogsQuery(c echo.Context) ProductionLogsPageData {
	data := ProductionLogsPageData{
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
	return data
}
