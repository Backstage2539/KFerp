package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	companyapp "orderapp/internal/application/company"

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

func registerCompanyStaffPages(e *echo.Echo, _ *pgxpool.Pool, _ string) {
	e.GET("/company/departments", func(c echo.Context) error {
		return vueShellRedirect(c, "departments")
	})

	e.GET("/company/employees", func(c echo.Context) error {
		return vueShellRedirect(c, "employees")
	})
}

func registerCompanyStaffAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	companySvc := companyapp.NewService(postgresCompanyRepository{pool: pool, schema: schema})

	e.GET("/api/company/departments", func(c echo.Context) error {
		rows, err := companySvc.ListDepartments(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, departmentItemsFromApp(rows))
	})
	// DEV-050: department maintenance API
	e.POST("/api/company/departments", func(c echo.Context) error {
		var req DepartmentUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		id, err := companySvc.CreateDepartment(c.Request().Context(), companyapp.DepartmentCommand{
			Name:   req.Name,
			Active: activeOrDefault(req.Active),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"id": id})
	})
	e.PUT("/api/company/departments/:id", func(c echo.Context) error {
		id, err := parseCompanyIDParam(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		var req DepartmentUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		err = companySvc.UpdateDepartment(c.Request().Context(), id, companyapp.DepartmentCommand{
			Name:   req.Name,
			Active: activeOrDefault(req.Active),
		})
		if err != nil && strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/company/employees", func(c echo.Context) error {
		departmentID := int64(0)
		if v := strings.TrimSpace(c.QueryParam("department_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				departmentID = n
			}
		}
		rows, err := companySvc.ListEmployees(c.Request().Context(), departmentID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, employeeItemsFromApp(rows))
	})

	e.POST("/api/company/employees", func(c echo.Context) error {
		var req EmployeeUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		id, err := companySvc.CreateEmployee(c.Request().Context(), companyapp.EmployeeCommand{
			Name:         req.Name,
			Phone:        req.Phone,
			DepartmentID: req.DepartmentID,
			Active:       activeOrDefault(req.Active),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"id": id})
	})

	e.PUT("/api/company/employees/:id", func(c echo.Context) error {
		id, err := parseCompanyIDParam(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		var req EmployeeUpsertReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		err = companySvc.UpdateEmployee(c.Request().Context(), id, companyapp.EmployeeCommand{
			Name:         req.Name,
			Phone:        req.Phone,
			DepartmentID: req.DepartmentID,
			Active:       activeOrDefault(req.Active),
		})
		if err != nil && strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func activeOrDefault(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func parseCompanyIDParam(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id required")
	}
	return id, nil
}

func departmentItemsFromApp(rows []companyapp.Department) []DepartmentItem {
	out := make([]DepartmentItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, DepartmentItem{ID: row.ID, Name: row.Name, Active: row.Active})
	}
	return out
}

func employeeItemsFromApp(rows []companyapp.Employee) []EmployeeItem {
	out := make([]EmployeeItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, EmployeeItem{
			ID:           row.ID,
			Name:         row.Name,
			Phone:        row.Phone,
			DepartmentID: row.DepartmentID,
			Department:   row.Department,
			Active:       row.Active,
		})
	}
	return out
}
