package customer

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, assetDir string) {
	registerCustomerRoutes(e, pool, schema, assetDir)
}
