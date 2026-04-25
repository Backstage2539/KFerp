package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProduceStartAPIRequest struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	CustomerID int64            `json:"customer_id"`
	Selected   []string         `json:"selected"`
	InputByKey map[string]int64 `json:"input_by_key"`
}

type ProduceStartAPIResponse struct {
	OK      bool   `json:"ok"`
	BatchID string `json:"batch_id"`
}

func registerProductionFlowPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		selected, sel := parseSelectedCSV(strings.TrimSpace(c.FormValue("selected")))
		from := strings.TrimSpace(c.FormValue("from"))
		to := strings.TrimSpace(c.FormValue("to"))
		var cid int64
		if v := strings.TrimSpace(c.FormValue("customer_id")); v != "" {
			cid, _ = strconv.ParseInt(v, 10, 64)
		}
		if sel == "" {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("请先生成计划并选择项目"))
		}
		inputByKey := map[string]int64{}
		rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, from, to, cid)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		for _, r := range rows {
			if !selected[producePlanKey(r.ProductID, r.SpecG)] || r.GapG <= 0 {
				continue
			}
			formKey := fmt.Sprintf("%d_%d", r.ProductID, r.SpecG)
			inputG, _, err := parseOptionalInt64(c.FormValue("input_g_" + formKey))
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("投料数格式不正确"))
			}
			if inputG <= 0 {
				return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("投料数必须大于0"))
			}
			inputByKey[producePlanKey(r.ProductID, r.SpecG)] = inputG
		}
		operator := actorOf(c)
		if _, err := startProductionWithInputs(c.Request().Context(), pool, schema, from, to, cid, selected, inputByKey, operator); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})

	e.POST("/api/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req ProduceStartAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if len(req.Selected) == 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请先生成计划并选择项目"})
		}
		selected := map[string]bool{}
		for _, key := range req.Selected {
			key = strings.TrimSpace(key)
			if key != "" {
				selected[key] = true
			}
		}
		batchID, err := startProductionWithInputs(c.Request().Context(), pool, schema, req.From, req.To, req.CustomerID, selected, req.InputByKey, actorOf(c))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceStartAPIResponse{OK: true, BatchID: batchID})
	})

	e.GET("/produce/running", func(c echo.Context) error {
		data := ProduceRunPageData{
			Ok:    strings.TrimSpace(c.QueryParam("ok")) == "1",
			Error: strings.TrimSpace(c.QueryParam("err")),
		}
		rows, err := listRunningItems(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "produce_running.html", data)
	})

	e.POST("/produce/running/finish", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		if id <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("invalid id"))
		}
		finishedUnits, hasFinishedUnits, err := parseOptionalInt64(c.FormValue("finished_units"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("完成件数格式不正确"))
		}
		finishedLooseG, hasFinishedLooseG, err := parseOptionalInt64(c.FormValue("finished_loose_g"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("散装余量格式不正确"))
		}
		if err := finishRunningItem(c.Request().Context(), pool, schema, id, finishedUnits, finishedLooseG, hasFinishedUnits || hasFinishedLooseG, actorOf(c)); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})

	e.POST("/produce/running/cancel", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		if id <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("invalid id"))
		}
		if err := cancelRunningItem(c.Request().Context(), pool, schema, id, actorOf(c)); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})
}

func parseSelectedCSV(sel string) (map[string]bool, string) {
	selected := map[string]bool{}
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return selected, ""
	}
	for _, key := range strings.Split(sel, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			selected[key] = true
		}
	}
	return selected, sel
}

func parseOptionalInt64(v string) (int64, bool, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}
