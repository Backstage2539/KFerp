package company

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	department DepartmentCommand
	employee   EmployeeCommand
}

func (r *fakeRepo) ListDepartments(ctx context.Context) ([]Department, error) {
	return []Department{{ID: 1, Name: "销售", Active: true}}, nil
}

func (r *fakeRepo) CreateDepartment(ctx context.Context, cmd DepartmentCommand) (int64, error) {
	r.department = cmd
	return 11, nil
}

func (r *fakeRepo) UpdateDepartment(ctx context.Context, id int64, cmd DepartmentCommand) error {
	r.department = cmd
	return nil
}

func (r *fakeRepo) ListEmployees(ctx context.Context, departmentID int64) ([]Employee, error) {
	return []Employee{{ID: 2, Name: "小李", DepartmentID: departmentID, Department: "销售", Active: true}}, nil
}

func (r *fakeRepo) CreateEmployee(ctx context.Context, cmd EmployeeCommand) (int64, error) {
	r.employee = cmd
	return 22, nil
}

func (r *fakeRepo) UpdateEmployee(ctx context.Context, id int64, cmd EmployeeCommand) error {
	r.employee = cmd
	return nil
}

func TestServiceValidatesAndNormalizesDepartment(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	id, err := svc.CreateDepartment(context.Background(), DepartmentCommand{Name: " 销售 ", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if id != 11 || repo.department.Name != "销售" {
		t.Fatalf("CreateDepartment() id=%d cmd=%+v", id, repo.department)
	}
	if _, err := svc.CreateDepartment(context.Background(), DepartmentCommand{}); err == nil || !strings.Contains(err.Error(), "name required") {
		t.Fatalf("empty department error = %v", err)
	}
}

func TestServiceValidatesAndNormalizesEmployee(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	id, err := svc.CreateEmployee(context.Background(), EmployeeCommand{Name: " 小王 ", Phone: " 13800138000 ", DepartmentID: 7, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if id != 22 || repo.employee.Name != "小王" || repo.employee.Phone != "13800138000" {
		t.Fatalf("CreateEmployee() id=%d cmd=%+v", id, repo.employee)
	}
	if _, err := svc.CreateEmployee(context.Background(), EmployeeCommand{Name: "小王", Phone: "abc", DepartmentID: 7}); err == nil || !strings.Contains(err.Error(), "invalid phone") {
		t.Fatalf("invalid phone error = %v", err)
	}
}
