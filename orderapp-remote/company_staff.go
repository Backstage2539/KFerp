package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type DepartmentItem struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type EmployeeItem struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	DepartmentID int64  `json:"department_id"`
	Department   string `json:"department"`
	Active       bool   `json:"active"`
}

type EmployeeUpsertReq struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	DepartmentID int64  `json:"department_id"`
	Active       *bool  `json:"active"`
}

type DepartmentUpsertReq struct {
	Name   string `json:"name"`
	Active *bool  `json:"active"`
}

type DepartmentPageData struct {
	Items []DepartmentItem
	Error string
	Ok    bool
}

type EmployeePageData struct {
	Items        []EmployeeItem
	Departments  []DepartmentItem
	DepartmentID int64
	Error        string
	Ok           bool
}

var phoneBasic = regexp.MustCompile(`^[0-9+\-\s]{6,32}$`)

func ensureCompanyStaffTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.company_departments (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.company_employees (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	phone TEXT NOT NULL,
	department_id BIGINT NOT NULL REFERENCES %s.company_departments(id),
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS company_employees_phone_uq ON %s.company_employees(phone);
`, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	seed := fmt.Sprintf(`
INSERT INTO %s.company_departments(name,active) VALUES
('销售', true),('生产', true),('财务', true)
ON CONFLICT (name) DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, seed)
	return err
}

func registerCompanyStaffPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/company/departments", func(c echo.Context) error {
		if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
			return vueShellRedirect(c, "departments")
		}
		rows, err := pool.Query(c.Request().Context(), "SELECT id,name,active FROM "+schema+".company_departments ORDER BY id")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		defer rows.Close()
		items := make([]DepartmentItem, 0)
		for rows.Next() {
			var d DepartmentItem
			if err := rows.Scan(&d.ID, &d.Name, &d.Active); err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			items = append(items, d)
		}
		data := DepartmentPageData{Items: items, Ok: strings.TrimSpace(c.QueryParam("ok")) == "1", Error: strings.TrimSpace(c.QueryParam("err"))}
		return c.Render(http.StatusOK, "company_departments.html", data)
	})

	e.GET("/company/employees", func(c echo.Context) error {
		if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
			return vueShellRedirect(c, "employees")
		}
		depRows, err := pool.Query(c.Request().Context(), "SELECT id,name,active FROM "+schema+".company_departments ORDER BY id")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		defer depRows.Close()
		deps := make([]DepartmentItem, 0)
		for depRows.Next() {
			var d DepartmentItem
			if err := depRows.Scan(&d.ID, &d.Name, &d.Active); err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			deps = append(deps, d)
		}

		depID := int64(0)
		where := ""
		args := []any{}
		if v := strings.TrimSpace(c.QueryParam("department_id")); v != "" {
			if x, err := strconv.ParseInt(v, 10, 64); err == nil && x > 0 {
				depID = x
				where = " WHERE e.department_id=$1"
				args = append(args, depID)
			}
		}
		q := "SELECT e.id,e.name,e.phone,e.department_id,COALESCE(d.name,''),e.active FROM " + schema + ".company_employees e JOIN " + schema + ".company_departments d ON d.id=e.department_id" + where + " ORDER BY e.id DESC"
		rows, err := pool.Query(c.Request().Context(), q, args...)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		defer rows.Close()
		items := make([]EmployeeItem, 0)
		for rows.Next() {
			var x EmployeeItem
			if err := rows.Scan(&x.ID, &x.Name, &x.Phone, &x.DepartmentID, &x.Department, &x.Active); err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			items = append(items, x)
		}
		data := EmployeePageData{Items: items, Departments: deps, DepartmentID: depID, Ok: strings.TrimSpace(c.QueryParam("ok")) == "1", Error: strings.TrimSpace(c.QueryParam("err"))}
		return c.Render(http.StatusOK, "company_employees.html", data)
	})
}

func registerCompanyStaffAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/api/company/departments", func(c echo.Context) error {
		rows, err := pool.Query(c.Request().Context(), "SELECT id,name,active FROM "+schema+".company_departments ORDER BY id")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer rows.Close()
		out := make([]DepartmentItem, 0)
		for rows.Next() {
			var d DepartmentItem
			if err := rows.Scan(&d.ID, &d.Name, &d.Active); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			out = append(out, d)
		}
		return c.JSON(http.StatusOK, out)
	})
	// DEV-050: department maintenance API
	e.POST("/api/company/departments", func(c echo.Context) error {
		var req DepartmentUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name required"})
		}
		active := true
		if req.Active != nil {
			active = *req.Active
		}
		var id int64
		if err := pool.QueryRow(c.Request().Context(),
			"INSERT INTO "+schema+".company_departments(name,active) VALUES($1,$2) RETURNING id", name, active,
		).Scan(&id); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"id": id})
	})
	e.PUT("/api/company/departments/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "id required"})
		}
		var req DepartmentUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name required"})
		}
		active := true
		if req.Active != nil {
			active = *req.Active
		}
		tag, err := pool.Exec(c.Request().Context(),
			"UPDATE "+schema+".company_departments SET name=$1,active=$2 WHERE id=$3", name, active, id,
		)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if tag.RowsAffected() == 0 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "department not found"})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/company/employees", func(c echo.Context) error {
		rows, err := pool.Query(c.Request().Context(),
			"SELECT e.id,e.name,e.phone,e.department_id,COALESCE(d.name,''),e.active FROM "+schema+".company_employees e JOIN "+schema+".company_departments d ON d.id=e.department_id ORDER BY e.id DESC")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer rows.Close()
		out := make([]EmployeeItem, 0)
		for rows.Next() {
			var x EmployeeItem
			if err := rows.Scan(&x.ID, &x.Name, &x.Phone, &x.DepartmentID, &x.Department, &x.Active); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			out = append(out, x)
		}
		return c.JSON(http.StatusOK, out)
	})

	e.POST("/api/company/employees", func(c echo.Context) error {
		var req EmployeeUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		name := strings.TrimSpace(req.Name)
		phone := strings.TrimSpace(req.Phone)
		if name == "" || req.DepartmentID <= 0 || phone == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name/phone/department_id required"})
		}
		if !phoneBasic.MatchString(phone) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid phone format"})
		}
		active := true
		if req.Active != nil {
			active = *req.Active
		}
		var id int64
		err := pool.QueryRow(c.Request().Context(),
			"INSERT INTO "+schema+".company_employees(name,phone,department_id,active) VALUES($1,$2,$3,$4) RETURNING id",
			name, phone, req.DepartmentID, active,
		).Scan(&id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"id": id})
	})

	e.PUT("/api/company/employees/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "id required"})
		}
		var req EmployeeUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		name := strings.TrimSpace(req.Name)
		phone := strings.TrimSpace(req.Phone)
		if name == "" || req.DepartmentID <= 0 || phone == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name/phone/department_id required"})
		}
		if !phoneBasic.MatchString(phone) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid phone format"})
		}
		active := true
		if req.Active != nil {
			active = *req.Active
		}
		tag, err := pool.Exec(c.Request().Context(),
			"UPDATE "+schema+".company_employees SET name=$1,phone=$2,department_id=$3,active=$4,updated_at=now() WHERE id=$5",
			name, phone, req.DepartmentID, active, id,
		)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if tag.RowsAffected() == 0 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "employee not found"})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
