package main

import (
	"net/http"
	"net/url"
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
	Ok         bool
	BatchID    string
	Error      string
}

func registerUnprodSummaryPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/unproduced", func(c echo.Context) error {
		data := UnprodSummaryPageData{
			From: strings.TrimSpace(c.QueryParam("from")),
			To:   strings.TrimSpace(c.QueryParam("to")),
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
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "unprod_summary.html", data)
	})

	e.POST("/produce/unproduced/allocate", func(c echo.Context) error {
		from := strings.TrimSpace(c.FormValue("from"))
		to := strings.TrimSpace(c.FormValue("to"))
		var cid int64
		if v := strings.TrimSpace(c.FormValue("customer_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				cid = n
			}
		}
		operator := "order"
		batch, _, err := allocateUnproducedBySummary(c.Request().Context(), pool, schema, from, to, cid, operator)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/unproduced?ok=1&batch="+url.QueryEscape(batch)+"&from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to)+"&customer_id="+strconv.FormatInt(cid, 10))
	})
}
