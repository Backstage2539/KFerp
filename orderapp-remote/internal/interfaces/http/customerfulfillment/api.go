package customerfulfillment

import (
	"fmt"
	"net/http"
	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type api struct {
	svc       Service
	customers CustomerDirectory
}

func (a api) listCustomers(c echo.Context) error {
	if a.customers == nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, fmt.Errorf("customer directory unavailable"))
	}
	q := strings.TrimSpace(c.QueryParam("q"))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	if limit <= 0 {
		limit = 80
	}
	if limit > 200 {
		limit = 200
	}
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	if offset < 0 {
		offset = 0
	}
	result, err := a.customers.List(c.Request().Context(), customerapp.ListQuery{Query: q, Limit: limit, Offset: offset})
	if err != nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, err)
	}
	rows := make([]customerapp.CustomerRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		if row.Active {
			rows = append(rows, row)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"customers": rows,
		"limit":     limit,
		"offset":    offset,
		"has_next":  result.HasNext,
	})
}

func (a api) parseImport(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	importType := app.ImportType(strings.TrimSpace(c.FormValue("import_type")))
	file, err := c.FormFile("file")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("file required"))
	}
	src, err := file.Open()
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	defer src.Close()
	batch, err := a.svc.ParseImport(c.Request().Context(), app.ParseImportCommand{
		CustomerID:     customerID,
		ImportType:     importType,
		SourceFilename: strings.TrimSpace(file.Filename),
		Reader:         src,
		CreatedBy:      currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, importBatchResponse(batch))
}

func (a api) applyImport(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	result, err := a.svc.ApplyImport(c.Request().Context(), app.ApplyImportCommand{
		BatchID: batchID,
		Actor:   currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, result)
}

func (a api) listImportRows(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	query := app.ListImportRowsQuery{
		BatchID: batchID,
		Status:  c.QueryParam("status"),
		Limit:   limit,
		Offset:  offset,
	}
	rows, err := a.svc.ListImportRows(c.Request().Context(), query)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"rows":   rows,
		"limit":  limit,
		"offset": offset,
	})
}

func (a api) importPreview(c echo.Context) error {
	batchID, err := parseID(c.Param("batch_id"), "batch")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	preview, err := a.svc.ImportPreview(c.Request().Context(), app.ImportPreviewQuery{BatchID: batchID})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, preview)
}

func (a api) overview(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	overview, err := a.svc.Overview(c.Request().Context(), app.OverviewQuery{CustomerID: customerID})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, overview)
}

func (a api) listImports(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	imports, err := a.svc.ListImports(c.Request().Context(), app.ListImportsQuery{CustomerID: customerID, Limit: limit, Offset: offset})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"imports": imports})
}

func (a api) createSettlement(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req struct {
		PeriodFrom string `json:"period_from"`
		PeriodTo   string `json:"period_to"`
	}
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	req.PeriodFrom = strings.TrimSpace(req.PeriodFrom)
	req.PeriodTo = strings.TrimSpace(req.PeriodTo)
	if req.PeriodFrom == "" || req.PeriodTo == "" {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("period required"))
	}
	result, err := a.svc.CreateSettlement(c.Request().Context(), app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: req.PeriodFrom,
		PeriodTo:   req.PeriodTo,
		CreatedBy:  currentCustomerFulfillmentActor(c),
	})
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, result)
}

func importBatchResponse(batch app.ImportBatch) map[string]any {
	return map[string]any{
		"batch":              batch,
		"batch_id":           batch.ID,
		"customer_id":        batch.CustomerID,
		"import_type":        batch.ImportType,
		"source_filename":    batch.SourceFilename,
		"status":             batch.Status,
		"summary":            batch.Summary,
		"total_rows":         batch.Summary.TotalRows,
		"valid_rows":         batch.Summary.ValidRows,
		"invalid_rows":       batch.Summary.InvalidRows,
		"direct_ship_orders": batch.Summary.DirectShipOrders,
		"processing_orders":  batch.Summary.ProcessingOrders,
		"fee_items":          batch.Summary.FeeItems,
	}
}

func parseID(value, name string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s id invalid", name)
	}
	return id, nil
}

func currentCustomerFulfillmentActor(c echo.Context) string {
	if v := strings.TrimSpace(c.Request().Header.Get("X-User")); v != "" {
		return v
	}
	return "erp"
}

func customerFulfillmentError(c echo.Context, status int, err error) error {
	return c.JSON(status, map[string]any{"error": err.Error()})
}
