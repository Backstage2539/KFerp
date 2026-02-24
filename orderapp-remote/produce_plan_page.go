package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProducePlanPageData struct {
	From       string
	To         string
	CustomerID int64
	Rows       []UnprodNeedRow
	Ok         bool
	BatchID    string
	Error      string

	// material params
	YieldRate   string
	DripExtraG  string
	DripBoxSpec string

	Materials []MaterialNeed
}

func buildProducePlanData(c echo.Context, pool *pgxpool.Pool, schema string) (ProducePlanPageData, error) {
	params := defaultProducePlanParams()
	data := ProducePlanPageData{
		From: strings.TrimSpace(c.QueryParam("from")),
		To:   strings.TrimSpace(c.QueryParam("to")),
		// defaults shown in UI
		YieldRate:   "0.8",
		DripExtraG:  "100",
		DripBoxSpec: "10",
	}
	if v := strings.TrimSpace(c.QueryParam("yield")); v != "" {
		data.YieldRate = v
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			params.YieldRate = f
		}
	}
	if v := strings.TrimSpace(c.QueryParam("drip_extra_g")); v != "" {
		data.DripExtraG = v
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.DripExtraG = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("drip_box")); v != "" {
		data.DripBoxSpec = v
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.DripBoxSpec = n
		}
	}
	data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
	data.BatchID = strings.TrimSpace(c.QueryParam("batch"))
	if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.CustomerID = n
		}
	}
	rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, data.From, data.To, data.CustomerID)
	if err != nil {
		return data, err
	}
	if mappings, err := listBagSpecMappings(c.Request().Context(), pool, schema); err == nil {
		params.BagNameBySpecG = mappingNameBySpec(mappings)
	}
	out := make([]UnprodNeedRow, 0, len(rows))
	for _, r := range rows {
		if r.GapG > 0 {
			out = append(out, r)
		}
	}
	data.Rows = out
	data.Materials = calcProducePlanMaterials(out, params)
	return data, nil
}

func registerProducePlanPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := func(c echo.Context) error {
		data, err := buildProducePlanData(c, pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		return c.Render(http.StatusOK, "produce_plan.html", data)
	}
	printH := func(c echo.Context) error {
		data, err := buildProducePlanData(c, pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		return c.Render(http.StatusOK, "produce_plan_print.html", data)
	}
	e.GET("/produce/plan", h)
	e.GET("/produce/plan/print", printH)
	// Backward/alternate path mentioned in requirements.
	e.GET("/app/produce/plan", h)
	e.GET("/app/produce/plan/print", printH)
}
