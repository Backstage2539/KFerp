package company

import (
	"context"
	"errors"
	"fmt"
	"strings"

	companyapp "orderapp/internal/application/company"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListDepartments(ctx context.Context) ([]companyapp.Department, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,name,active FROM "+r.schema+".company_departments ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]companyapp.Department, 0)
	for rows.Next() {
		var d companyapp.Department
		if err := rows.Scan(&d.ID, &d.Name, &d.Active); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r Repository) CreateDepartment(ctx context.Context, cmd companyapp.DepartmentCommand) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, "INSERT INTO "+r.schema+".company_departments(name,active) VALUES($1,$2) RETURNING id", cmd.Name, cmd.Active).Scan(&id)
	return id, err
}

func (r Repository) UpdateDepartment(ctx context.Context, id int64, cmd companyapp.DepartmentCommand) error {
	tag, err := r.pool.Exec(ctx, "UPDATE "+r.schema+".company_departments SET name=$1,active=$2 WHERE id=$3", cmd.Name, cmd.Active, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("department not found")
	}
	return nil
}

func (r Repository) ListEmployees(ctx context.Context, departmentID int64) ([]companyapp.Employee, error) {
	args := []any{}
	whereParts := []string{"(e.account_type='internal_employee' OR COALESCE(e.account_type,'')='')"}
	if departmentID > 0 {
		args = append(args, departmentID)
		whereParts = append(whereParts, fmt.Sprintf("e.department_id=$%d", len(args)))
	}
	where := " WHERE " + strings.Join(whereParts, " AND ")
	rows, err := r.pool.Query(ctx, "SELECT e.id,e.name,e.phone,e.department_id,COALESCE(d.name,''),e.active FROM "+r.schema+".company_employees e JOIN "+r.schema+".company_departments d ON d.id=e.department_id"+where+" ORDER BY e.id DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]companyapp.Employee, 0)
	for rows.Next() {
		var e companyapp.Employee
		if err := rows.Scan(&e.ID, &e.Name, &e.Phone, &e.DepartmentID, &e.Department, &e.Active); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r Repository) CreateEmployee(ctx context.Context, cmd companyapp.EmployeeCommand) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, "INSERT INTO "+r.schema+".company_employees(name,phone,department_id,active) VALUES($1,$2,$3,$4) RETURNING id", cmd.Name, cmd.Phone, cmd.DepartmentID, cmd.Active).Scan(&id)
	return id, mapEmployeeWriteError(err)
}

func (r Repository) UpdateEmployee(ctx context.Context, id int64, cmd companyapp.EmployeeCommand) error {
	tag, err := r.pool.Exec(ctx, "UPDATE "+r.schema+".company_employees SET name=$1,phone=$2,department_id=$3,active=$4,updated_at=now() WHERE id=$5", cmd.Name, cmd.Phone, cmd.DepartmentID, cmd.Active, id)
	if err != nil {
		return mapEmployeeWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("employee not found")
	}
	return nil
}

func mapEmployeeWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "company_employees_phone_uq" {
		return companyapp.ErrEmployeePhoneAlreadyUsed
	}
	return err
}

func (r Repository) LoadCompanyProfile(ctx context.Context) (companyapp.CompanyProfile, error) {
	var profile companyapp.CompanyProfile
	q := fmt.Sprintf(`SELECT company_name, company_address, company_phone, taxpayer_id, bank_account_name, bank_name, bank_account_no FROM %s.company_profile WHERE id=1`, r.schema)
	err := r.pool.QueryRow(ctx, q).Scan(&profile.Name, &profile.Address, &profile.Phone, &profile.TaxpayerID, &profile.BankAccountName, &profile.BankName, &profile.BankAccountNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return companyapp.CompanyProfile{}, nil
	}
	if err != nil {
		return companyapp.CompanyProfile{}, err
	}
	return profile, nil
}

func (r Repository) SaveCompanyProfile(ctx context.Context, cmd companyapp.CompanyProfileCommand) (companyapp.CompanyProfile, error) {
	q := fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address, company_phone, taxpayer_id, bank_account_name, bank_name, bank_account_no, updated_at, updated_by)
		VALUES(1,$1,$2,$3,$4,$5,$6,$7,now(),$8)
		ON CONFLICT(id) DO UPDATE SET
			company_name=excluded.company_name,
			company_address=excluded.company_address,
			company_phone=excluded.company_phone,
			taxpayer_id=excluded.taxpayer_id,
			bank_account_name=excluded.bank_account_name,
			bank_name=excluded.bank_name,
			bank_account_no=excluded.bank_account_no,
			updated_at=now(),
			updated_by=excluded.updated_by
		RETURNING company_name, company_address, company_phone, taxpayer_id, bank_account_name, bank_name, bank_account_no`, r.schema)
	var profile companyapp.CompanyProfile
	actor := cmd.Actor
	if actor == "" {
		actor = "unknown"
	}
	if err := r.pool.QueryRow(ctx, q, cmd.Name, cmd.Address, cmd.Phone, cmd.TaxpayerID, cmd.BankAccountName, cmd.BankName, cmd.BankAccountNo, actor).Scan(&profile.Name, &profile.Address, &profile.Phone, &profile.TaxpayerID, &profile.BankAccountName, &profile.BankName, &profile.BankAccountNo); err != nil {
		return companyapp.CompanyProfile{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "company_profile", nil, "update", postgresinfra.StrPtr("company_name"), nil, postgresinfra.StrPtr(profile.Name), postgresinfra.AuditMeta{"company_address": profile.Address, "company_phone": profile.Phone, "taxpayer_id": profile.TaxpayerID, "bank_account_name": profile.BankAccountName, "bank_name": profile.BankName, "bank_account_no": profile.BankAccountNo})
	return profile, nil
}
