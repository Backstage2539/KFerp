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
	ID            int64
	Name          string
	CapacityG     int64
	AllowedSpecs  string
	MinRoastG     int64
	Active        bool
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

func registerMachineCapacityPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/machines", func(c echo.Context) error {
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
		specs := strings.TrimSpace(c.FormValue("allowed_specs"))
		minRoast, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("min_roast_g")), 10, 64)
		active := strings.TrimSpace(c.FormValue("active")) != "0"
		if name == "" || capacity <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/machines")
		}
		if id > 0 {
			_, _ = pool.Exec(c.Request().Context(), "UPDATE "+schema+".roast_machines SET name=$2,capacity_g=$3,allowed_specs=$4,min_roast_g=$5,active=$6,updated_at=now() WHERE id=$1", id, name, capacity, specs, minRoast, active)
		} else {
			_, _ = pool.Exec(c.Request().Context(), "INSERT INTO "+schema+".roast_machines(name,capacity_g,allowed_specs,min_roast_g,active,updated_at) VALUES($1,$2,$3,$4,$5,now())", name, capacity, specs, minRoast, active)
		}
		return c.Redirect(http.StatusSeeOther, "/produce/machines?ok=1")
	})
}
