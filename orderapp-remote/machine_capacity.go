package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type RoastMachine struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CapacityG    int64  `json:"capacity_g"`
	AllowedSpecs string `json:"allowed_specs"`
	MinRoastG    int64  `json:"min_roast_g"`
	Active       bool   `json:"active"`
}

type MachinePageData struct {
	Rows  []RoastMachine
	Error string
	Ok    bool
}

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

func normalizeLoadSettings(raw string, minRoastG, maxRoastG int64) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	if minRoastG <= 0 {
		minRoastG = 1
	}
	if maxRoastG < minRoastG {
		return "", false
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil || v < minRoastG || v > maxRoastG {
			return "", false
		}
		out = append(out, strconv.FormatInt(v, 10))
	}
	return strings.Join(out, ","), true
}

func registerMachineCapacityPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/machines", func(c echo.Context) error {
		if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
			return vueShellRedirect(c, "machines")
		}
		data := MachinePageData{Ok: strings.TrimSpace(c.QueryParam("ok")) == "1"}
		q := "SELECT id,COALESCE(name,''),COALESCE(capacity_g,0),COALESCE(allowed_specs,''),COALESCE(min_roast_g,0),COALESCE(active,true) FROM " + schema + ".roast_machines ORDER BY id"
		rows, err := pool.Query(c.Request().Context(), q)
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "produce_machines.html", data)
		}
		defer rows.Close()
		for rows.Next() {
			var r RoastMachine
			if err := rows.Scan(&r.ID, &r.Name, &r.CapacityG, &r.AllowedSpecs, &r.MinRoastG, &r.Active); err == nil {
				data.Rows = append(data.Rows, r)
			}
		}
		return c.Render(http.StatusOK, "produce_machines.html", data)
	})

	e.POST("/produce/machines/save", func(c echo.Context) error {
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		name := strings.TrimSpace(c.FormValue("name"))
		capacity, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("capacity_g")), 10, 64)
		loadSettingsRaw := strings.TrimSpace(c.FormValue("allowed_specs"))
		minRoast, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("min_roast_g")), 10, 64)
		active := strings.TrimSpace(c.FormValue("active")) != "0"
		if name == "" || capacity <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/machines")
		}
		if minRoast <= 0 {
			minRoast = 1000
		}
		if minRoast > capacity {
			return c.Redirect(http.StatusSeeOther, "/produce/machines")
		}
		loadSettings, ok := normalizeLoadSettings(loadSettingsRaw, minRoast, capacity)
		if !ok {
			return c.Redirect(http.StatusSeeOther, "/produce/machines")
		}
		if id > 0 {
			_, _ = pool.Exec(c.Request().Context(), "UPDATE "+schema+".roast_machines SET name=$2,capacity_g=$3,allowed_specs=$4,min_roast_g=$5,active=$6,updated_at=now() WHERE id=$1", id, name, capacity, loadSettings, minRoast, active)
		} else {
			_, _ = pool.Exec(c.Request().Context(), "INSERT INTO "+schema+".roast_machines(name,capacity_g,allowed_specs,min_roast_g,active,updated_at) VALUES($1,$2,$3,$4,$5,now())", name, capacity, loadSettings, minRoast, active)
		}
		return c.Redirect(http.StatusSeeOther, "/produce/machines?ok=1")
	})
	e.GET("/api/produce/machines", func(c echo.Context) error {
		rows, err := listRoastMachines(c.Request().Context(), pool, schema)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/produce/machines", func(c echo.Context) error {
		var req RoastMachine
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := saveRoastMachine(c.Request().Context(), pool, schema, req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func listRoastMachines(ctx context.Context, pool *pgxpool.Pool, schema string) ([]RoastMachine, error) {
	q := "SELECT id,COALESCE(name,''),COALESCE(capacity_g,0),COALESCE(allowed_specs,''),COALESCE(min_roast_g,0),COALESCE(active,true) FROM " + schema + ".roast_machines ORDER BY id"
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RoastMachine, 0)
	for rows.Next() {
		var r RoastMachine
		if err := rows.Scan(&r.ID, &r.Name, &r.CapacityG, &r.AllowedSpecs, &r.MinRoastG, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func saveRoastMachine(ctx context.Context, pool *pgxpool.Pool, schema string, req RoastMachine) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.CapacityG <= 0 {
		return fmt.Errorf("name and capacity_g required")
	}
	if req.MinRoastG <= 0 {
		req.MinRoastG = 1000
	}
	if req.MinRoastG > req.CapacityG {
		return fmt.Errorf("min_roast_g must be <= capacity_g")
	}
	loadSettings, ok := normalizeLoadSettings(req.AllowedSpecs, req.MinRoastG, req.CapacityG)
	if !ok {
		return fmt.Errorf("invalid allowed_specs")
	}
	if req.ID > 0 {
		tag, err := pool.Exec(ctx, "UPDATE "+schema+".roast_machines SET name=$2,capacity_g=$3,allowed_specs=$4,min_roast_g=$5,active=$6,updated_at=now() WHERE id=$1", req.ID, req.Name, req.CapacityG, loadSettings, req.MinRoastG, req.Active)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("machine not found")
		}
		return nil
	}
	_, err := pool.Exec(ctx, "INSERT INTO "+schema+".roast_machines(name,capacity_g,allowed_specs,min_roast_g,active,updated_at) VALUES($1,$2,$3,$4,$5,now())", req.Name, req.CapacityG, loadSettings, req.MinRoastG, req.Active)
	return err
}
