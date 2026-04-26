package appmain

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerProducePlanPages(e *echo.Echo, _ *pgxpool.Pool, _ string) {
	redirect := func(c echo.Context) error {
		target := "/vue-shell?view=producePlan"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	}
	e.GET("/produce/plan", redirect)
	e.GET("/app/produce/plan", redirect)
}
