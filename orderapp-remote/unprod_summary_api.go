package main

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerUnprodSummaryAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/api/produce/unproduced", func(c echo.Context) error {
		data, err := loadUnprodSummaryData(c.Request().Context(), pool, schema, parseUnprodSummaryQuery(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, data)
	})
}
