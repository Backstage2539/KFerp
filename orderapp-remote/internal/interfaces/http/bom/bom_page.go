package bom

import (
	"net/http"
	"net/url"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

func registerBomPages(e *echo.Echo) {
	e.GET("/bom", func(c echo.Context) error {
		target := "/vue-shell?view=productionConfig&tab=bom"
		if productID := strings.TrimSpace(c.QueryParam("product_id")); productID != "" {
			target += "&product_id=" + url.QueryEscape(productID)
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
}
