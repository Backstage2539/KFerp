package appmain

import (
	companyhttp "orderapp/internal/interfaces/http/company"
	customerhttp "orderapp/internal/interfaces/http/customer"
	productionhttp "orderapp/internal/interfaces/http/production"
	saleshttp "orderapp/internal/interfaces/http/sales"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerAppRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, assetDir string) {
	supporthttp.RegisterRoutes(e, pool, schema)
	productionhttp.RegisterRoutes(e, pool, schema)
	companyhttp.RegisterRoutes(e, pool, schema)
	customerhttp.RegisterRoutes(e, pool, schema, assetDir)
	saleshttp.RegisterRoutes(e, pool, schema)
}
