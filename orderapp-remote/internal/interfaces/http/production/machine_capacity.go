package production

import (
	"context"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	support "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type RoastMachine = productionapp.RoastMachine

func ensureMachineCapacityTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.roast_machines (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		capacity_g BIGINT NOT NULL,
		allowed_specs TEXT NOT NULL DEFAULT '',
		min_roast_g BIGINT NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func registerMachineCapacityPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	e.GET("/produce/machines", func(c echo.Context) error {
		return support.VueShellRedirect(c, "machines")
	})
	e.GET("/api/produce/machines", func(c echo.Context) error {
		rows, err := productionSvc.ListMachines(c.Request().Context(), false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/produce/machines", func(c echo.Context) error {
		var req productionapp.RoastMachineCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := productionSvc.SaveMachine(c.Request().Context(), req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
