package company

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var phonePattern = regexp.MustCompile(`^[0-9+\-\s]{6,32}$`)

var ErrEmployeePhoneAlreadyUsed = errors.New("employee phone already used")

type Department struct {
	ID     int64
	Name   string
	Active bool
}

type Employee struct {
	ID           int64
	Name         string
	Phone        string
	DepartmentID int64
	Department   string
	Active       bool
}

type CompanyProfile struct {
	Name            string `json:"company_name"`
	Address         string `json:"company_address"`
	Phone           string `json:"company_phone"`
	TaxpayerID      string `json:"taxpayer_id"`
	BankAccountName string `json:"bank_account_name"`
	BankName        string `json:"bank_name"`
	BankAccountNo   string `json:"bank_account_no"`
}

type DepartmentCommand struct {
	Name   string
	Active bool
}

type EmployeeCommand struct {
	Name         string
	Phone        string
	DepartmentID int64
	Active       bool
}

type CompanyProfileCommand struct {
	Actor           string `json:"actor"`
	Name            string `json:"company_name"`
	Address         string `json:"company_address"`
	Phone           string `json:"company_phone"`
	TaxpayerID      string `json:"taxpayer_id"`
	BankAccountName string `json:"bank_account_name"`
	BankName        string `json:"bank_name"`
	BankAccountNo   string `json:"bank_account_no"`
}

type Repository interface {
	ListDepartments(ctx context.Context) ([]Department, error)
	CreateDepartment(ctx context.Context, cmd DepartmentCommand) (int64, error)
	UpdateDepartment(ctx context.Context, id int64, cmd DepartmentCommand) error
	ListEmployees(ctx context.Context, departmentID int64) ([]Employee, error)
	CreateEmployee(ctx context.Context, cmd EmployeeCommand) (int64, error)
	UpdateEmployee(ctx context.Context, id int64, cmd EmployeeCommand) error
	LoadCompanyProfile(ctx context.Context) (CompanyProfile, error)
	SaveCompanyProfile(ctx context.Context, cmd CompanyProfileCommand) (CompanyProfile, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListDepartments(ctx context.Context) ([]Department, error) {
	return s.repo.ListDepartments(ctx)
}

func (s *Service) CreateDepartment(ctx context.Context, cmd DepartmentCommand) (int64, error) {
	cmd, err := normalizeDepartmentCommand(cmd)
	if err != nil {
		return 0, err
	}
	return s.repo.CreateDepartment(ctx, cmd)
}

func (s *Service) UpdateDepartment(ctx context.Context, id int64, cmd DepartmentCommand) error {
	if id <= 0 {
		return fmt.Errorf("id required")
	}
	cmd, err := normalizeDepartmentCommand(cmd)
	if err != nil {
		return err
	}
	return s.repo.UpdateDepartment(ctx, id, cmd)
}

func (s *Service) ListEmployees(ctx context.Context, departmentID int64) ([]Employee, error) {
	return s.repo.ListEmployees(ctx, departmentID)
}

func (s *Service) CreateEmployee(ctx context.Context, cmd EmployeeCommand) (int64, error) {
	cmd, err := normalizeEmployeeCommand(cmd)
	if err != nil {
		return 0, err
	}
	return s.repo.CreateEmployee(ctx, cmd)
}

func (s *Service) UpdateEmployee(ctx context.Context, id int64, cmd EmployeeCommand) error {
	if id <= 0 {
		return fmt.Errorf("id required")
	}
	cmd, err := normalizeEmployeeCommand(cmd)
	if err != nil {
		return err
	}
	return s.repo.UpdateEmployee(ctx, id, cmd)
}

func (s *Service) LoadCompanyProfile(ctx context.Context) (CompanyProfile, error) {
	return s.repo.LoadCompanyProfile(ctx)
}

func (s *Service) SaveCompanyProfile(ctx context.Context, cmd CompanyProfileCommand) (CompanyProfile, error) {
	cmd, err := normalizeCompanyProfileCommand(cmd)
	if err != nil {
		return CompanyProfile{}, err
	}
	return s.repo.SaveCompanyProfile(ctx, cmd)
}

func normalizeDepartmentCommand(cmd DepartmentCommand) (DepartmentCommand, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return DepartmentCommand{}, fmt.Errorf("name required")
	}
	return cmd, nil
}

func normalizeEmployeeCommand(cmd EmployeeCommand) (EmployeeCommand, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Phone = strings.TrimSpace(cmd.Phone)
	if cmd.Name == "" || cmd.Phone == "" || cmd.DepartmentID <= 0 {
		return EmployeeCommand{}, fmt.Errorf("name/phone/department_id required")
	}
	if !phonePattern.MatchString(cmd.Phone) {
		return EmployeeCommand{}, fmt.Errorf("invalid phone format")
	}
	return cmd, nil
}

func normalizeCompanyProfileCommand(cmd CompanyProfileCommand) (CompanyProfileCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Address = strings.TrimSpace(cmd.Address)
	cmd.Phone = strings.TrimSpace(cmd.Phone)
	cmd.TaxpayerID = strings.TrimSpace(cmd.TaxpayerID)
	cmd.BankAccountName = strings.TrimSpace(cmd.BankAccountName)
	cmd.BankName = strings.TrimSpace(cmd.BankName)
	cmd.BankAccountNo = strings.TrimSpace(cmd.BankAccountNo)
	if cmd.Name == "" {
		return CompanyProfileCommand{}, fmt.Errorf("company_name required")
	}
	return cmd, nil
}
