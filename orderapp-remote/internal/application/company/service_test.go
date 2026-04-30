package company

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	department DepartmentCommand
	employee   EmployeeCommand
	profile    CompanyProfileCommand
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

func (r *fakeRepo) LoadCompanyProfile(ctx context.Context) (CompanyProfile, error) {
	return CompanyProfile{Name: "棵凡咖啡", Address: "昆明市人民东路", Phone: "0871-12345678"}, nil
}

func (r *fakeRepo) SaveCompanyProfile(ctx context.Context, cmd CompanyProfileCommand) (CompanyProfile, error) {
	r.profile = cmd
	return CompanyProfile{Name: cmd.Name, Address: cmd.Address, Phone: cmd.Phone}, nil
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

func TestServiceValidatesAndNormalizesCompanyProfile(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.SaveCompanyProfile(context.Background(), CompanyProfileCommand{
		Actor:   " 设置员 ",
		Name:    " 棵凡咖啡 ",
		Address: " 昆明市人民东路 ",
		Phone:   " 0871-12345678 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "棵凡咖啡" || repo.profile.Actor != "设置员" || repo.profile.Name != "棵凡咖啡" || repo.profile.Address != "昆明市人民东路" || repo.profile.Phone != "0871-12345678" {
		t.Fatalf("profile=%+v command=%+v", got, repo.profile)
	}
	if _, err := svc.SaveCompanyProfile(context.Background(), CompanyProfileCommand{}); err == nil || !strings.Contains(err.Error(), "company_name required") {
		t.Fatalf("empty company profile error = %v", err)
	}
	loaded, err := svc.LoadCompanyProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "棵凡咖啡" {
		t.Fatalf("LoadCompanyProfile() = %+v", loaded)
	}
}
