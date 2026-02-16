package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type ReqPageData struct {
	Title string
}

func registerRequirementPages(e *echo.Echo) {
	// 需求管理（5张表/页面）——先保证可访问，后续再接入真实数据模型。
	e.GET("/req/product", func(c echo.Context) error {
		return c.Render(http.StatusOK, "req_product.html", ReqPageData{Title: "产品需求表"})
	})
	e.GET("/req/dev", func(c echo.Context) error {
		return c.Render(http.StatusOK, "req_dev.html", ReqPageData{Title: "开发需求表"})
	})
	e.GET("/req/unit", func(c echo.Context) error {
		return c.Render(http.StatusOK, "req_unit.html", ReqPageData{Title: "单元测试表"})
	})
	e.GET("/req/api", func(c echo.Context) error {
		return c.Render(http.StatusOK, "req_api.html", ReqPageData{Title: "API 测试表"})
	})
	e.GET("/req/review", func(c echo.Context) error {
		return c.Render(http.StatusOK, "req_review.html", ReqPageData{Title: "需求审核表"})
	})
}
