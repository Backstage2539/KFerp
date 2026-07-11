package company

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	companyapp "orderapp/internal/application/company"

	"github.com/labstack/echo/v4"
)

type employeePhoneConflictRepo struct{}

func (employeePhoneConflictRepo) ListDepartments(context.Context) ([]companyapp.Department, error) {
	return nil, nil
}

func (employeePhoneConflictRepo) CreateDepartment(context.Context, companyapp.DepartmentCommand) (int64, error) {
	return 0, nil
}

func (employeePhoneConflictRepo) UpdateDepartment(context.Context, int64, companyapp.DepartmentCommand) error {
	return nil
}

func (employeePhoneConflictRepo) ListEmployees(context.Context, int64) ([]companyapp.Employee, error) {
	return nil, nil
}

func (employeePhoneConflictRepo) CreateEmployee(context.Context, companyapp.EmployeeCommand) (int64, error) {
	return 0, companyapp.ErrEmployeePhoneAlreadyUsed
}

func (employeePhoneConflictRepo) UpdateEmployee(context.Context, int64, companyapp.EmployeeCommand) error {
	return companyapp.ErrEmployeePhoneAlreadyUsed
}

func (employeePhoneConflictRepo) LoadCompanyProfile(context.Context) (companyapp.CompanyProfile, error) {
	return companyapp.CompanyProfile{}, nil
}

func (employeePhoneConflictRepo) SaveCompanyProfile(context.Context, companyapp.CompanyProfileCommand) (companyapp.CompanyProfile, error) {
	return companyapp.CompanyProfile{}, nil
}

func TestEmployeeAPIMapsDuplicatePhoneToConflict(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/company/employees"},
		{name: "update", method: http.MethodPut, path: "/api/company/employees/7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			RegisterRoutes(e, Dependencies{Company: companyapp.NewService(employeePhoneConflictRepo{})})

			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"name":"测试员工","phone":"13800138000","department_id":1,"active":true}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, "该手机号已被其他员工或客户外部账号使用") {
				t.Fatalf("body = %s", body)
			}
			if strings.Contains(body, "duplicate key") || strings.Contains(body, "company_employees_phone_uq") {
				t.Fatalf("response leaked database error: %s", body)
			}
		})
	}
}
