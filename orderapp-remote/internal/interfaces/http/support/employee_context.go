package support

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func EmployeeContextMiddleware(pool *pgxpool.Pool, schema string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if u, _, ok := c.Request().BasicAuth(); ok {
				if eid, err := resolveEmployeeIDByLogin(c, pool, schema, u); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename, err := resolveEmployeeNameByID(c, pool, schema, eid); err == nil && strings.TrimSpace(ename) != "" {
						c.Set("operator_employee", strings.TrimSpace(ename))
					}
				}
			}
			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename != "" {
						c.Set("operator_employee", ename)
					}
				}
			}
			return next(c)
		}
	}
}

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

func resolveEmployeeBySessionToken(ctx echo.Context, pool *pgxpool.Pool, schema, token string) (int64, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, "", nil
	}
	var id int64
	var name string
	err := pool.QueryRow(ctx.Request().Context(),
		"SELECT e.id,COALESCE(e.name,'') FROM "+schema+".login_sessions s JOIN "+schema+".company_employees e ON e.id=s.employee_id WHERE s.token=$1 AND s.expire_at>now() AND e.active=true LIMIT 1",
		token,
	).Scan(&id, &name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", err
	}
	return id, strings.TrimSpace(name), nil
}

func RequireEmployeeBound(c echo.Context) error {
	if currentEmployeeID(c) <= 0 {
		return fmt.Errorf("employee binding required")
	}
	return nil
}
