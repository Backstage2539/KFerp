package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type UnprodSummaryPageData struct {
	From        string
	To          string
	CustomerID  int64
	Rows        []UnprodNeedRow
	PlanRows    []UnprodNeedRow
	Materials   []MaterialNeed
	RoastSplits []RoastSplitRow
	Selected    map[string]bool
	PlanReady   bool
	StockTip    string
	Error       string
}

func registerUnprodSummaryPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/unproduced", func(c echo.Context) error {
		data := UnprodSummaryPageData{
			From: strings.TrimSpace(c.QueryParam("from")),
			To:   strings.TrimSpace(c.QueryParam("to")),
		}
		if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				data.CustomerID = n
			}
		}
		rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, data.From, data.To, data.CustomerID)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
			data.Selected = map[string]bool{}
			if sel := strings.TrimSpace(c.QueryParam("selected")); sel != "" {
				for _, k := range strings.Split(sel, ",") {
					k = strings.TrimSpace(k)
					if k != "" {
						data.Selected[k] = true
					}
				}
			}
			if strings.TrimSpace(c.QueryParam("plan")) == "1" && len(data.Selected) > 0 {
				data.PlanReady = true
				planRows := make([]UnprodNeedRow, 0)
				selectedCount := 0
				for _, r := range rows {
					k := fmt.Sprintf("%d-%d", r.ProductID, r.SpecG)
					if !data.Selected[k] {
						continue
					}
					selectedCount++
					if r.GapG <= 0 {
						continue
					}
					planRows = append(planRows, r)
				}
				data.PlanRows = planRows
				if selectedCount > 0 && len(planRows) == 0 {
					data.StockTip = "库存充足：当前已选商品库存均可满足，无需补产。"
				}
				params := defaultProducePlanParams()
				if mappings, err := listBagSpecMappings(c.Request().Context(), pool, schema); err == nil {
					params.BagNameBySpecG = mappingNameBySpec(mappings)
				}
				if bomMap, err := loadProducePlanBomMap(c.Request().Context(), pool, schema); err == nil {
					params.BomByProductID = bomMap
				}
				data.Materials = calcProducePlanMaterialsWithBOM(c.Request().Context(), pool, schema, planRows, params)
				if ms, err := loadActiveMachines(c.Request().Context(), pool, schema); err == nil {
					data.RoastSplits = calcRoastSplits(planRows, ms, params.YieldRate)
				}
			}
		}
		return c.Render(http.StatusOK, "unprod_summary.html", data)
	})
}
