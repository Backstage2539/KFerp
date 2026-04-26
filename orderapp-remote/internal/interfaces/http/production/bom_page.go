package production

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerBomPages(e *echo.Echo, _ *pgxpool.Pool, _ string) {
	e.GET("/bom", func(c echo.Context) error {
		target := "/vue-shell?view=bom"
		if productID := strings.TrimSpace(c.QueryParam("product_id")); productID != "" {
			target += "&product_id=" + url.QueryEscape(productID)
		}
		return c.Redirect(http.StatusFound, target)
	})
}
