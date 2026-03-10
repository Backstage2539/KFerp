package main

import (
	"net/http"
	"sort"
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
	Warning    string
	Error      string

	// material params
	YieldRate   string
	DripExtraG  string
	DripBoxSpec string

	Materials   []MaterialNeed
	RoastSplits []RoastSplitRow
}

func registerProducePlanPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := func(c echo.Context) error {
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
		if strings.TrimSpace(c.QueryParam("warning")) == "low_inventory" {
			data.Warning = "库存已低于警戒线：本次允许扣减，请尽快补货。"
		}
		if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				data.CustomerID = n
			}
		}
		rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, data.From, data.To, data.CustomerID)
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "produce_plan.html", data)
		}
		if mappings, err := listBagSpecMappings(c.Request().Context(), pool, schema); err == nil {
			params.BagNameBySpecG = mappingNameBySpec(mappings)
		}
		if bomMap, err := loadProducePlanBomMap(c.Request().Context(), pool, schema); err == nil {
			params.BomByProductID = bomMap
		}
		// only gap>0
		out := make([]UnprodNeedRow, 0, len(rows))
		for _, r := range rows {
			if r.GapG > 0 {
				out = append(out, r)
			}
		}
		// DEV-034: present summary in stable SKU/spec order.
		sort.Slice(out, func(i, j int) bool {
			if out[i].ProductID != out[j].ProductID {
				return out[i].ProductID < out[j].ProductID
			}
			if out[i].SpecG != out[j].SpecG {
				return out[i].SpecG < out[j].SpecG
			}
			return out[i].Product < out[j].Product
		})
		data.Rows = out
		data.Materials = calcProducePlanMaterialsWithBOM(c.Request().Context(), pool, schema, out, params)
		if ms, err := loadActiveMachines(c.Request().Context(), pool, schema); err == nil {
			data.RoastSplits = calcRoastSplits(out, ms, params.YieldRate)
		}
		return c.Render(http.StatusOK, "produce_plan.html", data)
	}
	e.GET("/produce/plan", h)
	// Backward/alternate path mentioned in requirements.
	e.GET("/app/produce/plan", h)
}
