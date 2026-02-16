package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ReqPageData struct {
	Title string
	Rows  []ReqRow
	Error string
	Ok    bool

	Limit   int
	Offset  int
	Page    int
	HasPrev bool
	HasNext bool
}

func registerRequirementPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	// 需求管理（5张表/页面）——先跑通体系：列表 + 新增（最小可用）。
	reg := func(path, tpl, title, table string) {
		e.GET(path, func(c echo.Context) error {
			data := ReqPageData{Title: title}
			data.Limit = intParam(c, "limit", 20)
			if data.Limit <= 0 || data.Limit > 200 {
				data.Limit = 20
			}
			data.Page = intParam(c, "page", 1)
			if data.Page <= 0 {
				data.Page = 1
			}
			data.Offset = (data.Page - 1) * data.Limit
			data.HasPrev = data.Page > 1

			rows, hasNext, err := listReqRows(c.Request().Context(), pool, schema, table, data.Limit, data.Offset)
			if err != nil {
				data.Error = err.Error()
			} else {
				data.Rows = rows
				data.HasNext = hasNext
			}
			if qerr := strings.TrimSpace(c.QueryParam("err")); qerr != "" {
				data.Error = qerr
			}
			data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
			return c.Render(http.StatusOK, tpl, data)
		})
		e.POST(path+"/new", func(c echo.Context) error {
			code := strings.TrimSpace(c.FormValue("code"))
			title2 := strings.TrimSpace(c.FormValue("title"))
			status := strings.TrimSpace(c.FormValue("status"))
			assignee := strings.TrimSpace(c.FormValue("assignee"))
			if err := createReqRow(c.Request().Context(), pool, schema, table, code, title2, status, assignee); err != nil {
				return c.Redirect(http.StatusSeeOther, path+"?err="+url.QueryEscape(err.Error()))
			}
			return c.Redirect(http.StatusSeeOther, path+"?ok=1")
		})
	}

	reg("/req/product", "req_product.html", "产品需求表", "req_product")
	reg("/req/dev", "req_dev.html", "开发需求表", "req_dev")
	reg("/req/unit", "req_unit.html", "单元测试表", "req_unit")
	reg("/req/api", "req_api.html", "API 测试表", "req_api")
	// Review page needs special create/update behavior.

	// Review: list
	e.GET("/req/review", func(c echo.Context) error {
		data := ReqPageData{Title: "需求审核表"}
		data.Limit = intParam(c, "limit", 20)
		if data.Limit <= 0 || data.Limit > 200 {
			data.Limit = 20
		}
		data.Page = intParam(c, "page", 1)
		if data.Page <= 0 {
			data.Page = 1
		}
		data.Offset = (data.Page - 1) * data.Limit
		data.HasPrev = data.Page > 1

		rows, hasNext, err := listReqRows(c.Request().Context(), pool, schema, "req_review", data.Limit, data.Offset)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
			data.HasNext = hasNext
		}
		if qerr := strings.TrimSpace(c.QueryParam("err")); qerr != "" {
			data.Error = qerr
		}
		data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
		return c.Render(http.StatusOK, "req_review.html", data)
	})
	// Review: create
	e.POST("/req/review/new", func(c echo.Context) error {
		code := strings.TrimSpace(c.FormValue("code"))
		prCode := strings.TrimSpace(c.FormValue("pr_code"))
		title2 := strings.TrimSpace(c.FormValue("title"))
		status := strings.TrimSpace(c.FormValue("status"))
		assignee := strings.TrimSpace(c.FormValue("assignee"))
		if err := createReviewRow(c.Request().Context(), pool, schema, code, prCode, title2, status, assignee); err != nil {
			return c.Redirect(http.StatusSeeOther, "/req/review?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/req/review?ok=1")
	})
	// Review: update status
	e.POST("/req/review/update", func(c echo.Context) error {
		code := strings.TrimSpace(c.FormValue("code"))
		status := strings.TrimSpace(c.FormValue("status"))
		if err := updateReviewStatusAndCascade(c.Request().Context(), pool, schema, code, status); err != nil {
			return c.Redirect(http.StatusSeeOther, "/req/review?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/req/review?ok=1")
	})
}

