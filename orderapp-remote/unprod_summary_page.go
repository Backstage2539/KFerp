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
	From       string
	To         string
	CustomerID int64
	Rows       []UnprodNeedRow
	PlanRows   []UnprodNeedRow
	Materials  []MaterialNeed
	RoastSplits []RoastSplitRow
	Selected   map[string]bool
	Error      string
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
			planRows := make([]UnprodNeedRow, 0)
			for _, r := range rows {
				if r.GapG <= 0 {
					continue
				}
				k := fmt.Sprintf("%d-%d", r.ProductID, r.SpecG)
				if len(data.Selected) == 0 || data.Selected[k] {
					planRows = append(planRows, r)
				}
			}
			data.PlanRows = planRows
			params := defaultProducePlanParams()
			if mappings, err := listBagSpecMappings(c.Request().Context(), pool, schema); err == nil {
				params.BagNameBySpecG = mappingNameBySpec(mappings)
			}
			data.Materials = calcProducePlanMaterials(planRows, params)
			if ms, err := loadActiveMachines(c.Request().Context(), pool, schema); err == nil {
				data.RoastSplits = calcRoastSplits(planRows, ms)
			}
		}
		return c.Render(http.StatusOK, "unprod_summary.html", data)
	})
}
