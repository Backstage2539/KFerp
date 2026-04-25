package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type MaterialListResponse struct {
	Rows []MaterialRow `json:"rows"`
}

func registerMaterialsAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/api/materials", func(c echo.Context) error {
		limit := intParam(c, "limit", 200)
		rows, err := listMaterials(c.Request().Context(), pool, schema, strings.TrimSpace(c.QueryParam("q")), limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, MaterialListResponse{Rows: rows})
	})

	e.POST("/api/materials/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		var req MaterialInput
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := updateMaterialInline(c.Request().Context(), pool, schema, actorOf(c), id, req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
}
