package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerProducePlanAllocate(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/produce/plan/commit", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/plan?err="+url.QueryEscape(err.Error()))
		}
		from := strings.TrimSpace(c.FormValue("from"))
		to := strings.TrimSpace(c.FormValue("to"))
		var cid int64
		if v := strings.TrimSpace(c.FormValue("customer_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				cid = n
			}
		}
		operator := "order"
		batch, _, lowWarn, err := allocateUnproducedBySummary(c.Request().Context(), pool, schema, from, to, cid, operator)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/plan?err="+url.QueryEscape(err.Error()))
		}
		next := "/produce/plan?ok=1&batch=" + url.QueryEscape(batch) + "&from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to) + "&customer_id=" + strconv.FormatInt(cid, 10)
		if lowWarn {
			next += "&warning=low_inventory"
		}
		return c.Redirect(http.StatusSeeOther, next)
	})
}
