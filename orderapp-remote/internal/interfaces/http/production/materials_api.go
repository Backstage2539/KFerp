package production

import (
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	materialsapp "orderapp/internal/application/materials"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type MaterialListResponse struct {
	Rows []materialsapp.Material `json:"rows"`
}

func registerMaterialsAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	materialsSvc := materialsapp.NewService(postgresMaterialsRepository{pool: pool, schema: schema})

	e.GET("/api/materials", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 200)
		rows, err := materialsSvc.List(c.Request().Context(), materialsapp.ListCommand{
			Query: strings.TrimSpace(c.QueryParam("q")),
			Limit: limit,
		})
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
		var req materialsapp.MaterialInput
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.Update(c.Request().Context(), materialsapp.UpdateCommand{
			Actor: support.ActorOf(c),
			ID:    id,
			Input: req,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
}
