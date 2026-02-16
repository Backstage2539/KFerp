package main

import (
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
		}
		return c.Render(http.StatusOK, "unprod_summary.html", data)
	})
}
