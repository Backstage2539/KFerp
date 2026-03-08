package main

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func resolveEmployeeIDByLogin(ctx echo.Context, pool *pgxpool.Pool, schema, login string) (int64, error) {
	if login == "" {
		return 0, nil
	}
	var id int64
	err := pool.QueryRow(ctx.Request().Context(),
		"SELECT id FROM "+schema+".company_employees WHERE active=true AND (phone=$1 OR name=$1) ORDER BY id LIMIT 1",
		login,
	).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}

func resolveEmployeeNameByID(ctx echo.Context, pool *pgxpool.Pool, schema string, id int64) (string, error) {
	if id <= 0 {
		return "", nil
	}
	var name string
	err := pool.QueryRow(ctx.Request().Context(),
		"SELECT COALESCE(name,'') FROM "+schema+".company_employees WHERE id=$1 LIMIT 1",
		id,
	).Scan(&name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return name, nil
}

func currentEmployeeID(c echo.Context) int64 {
	if v := c.Get("employee_id"); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

func requireEmployeeBound(c echo.Context) error {
	if currentEmployeeID(c) <= 0 {
		return fmt.Errorf("employee binding required")
	}
	return nil
}
