package company

import (
	"context"
	"fmt"

	companyapp "orderapp/internal/application/company"

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
	where := ""
	if departmentID > 0 {
		where = " WHERE e.department_id=$1"
		args = append(args, departmentID)
	}
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
	return id, err
}

func (r Repository) UpdateEmployee(ctx context.Context, id int64, cmd companyapp.EmployeeCommand) error {
	tag, err := r.pool.Exec(ctx, "UPDATE "+r.schema+".company_employees SET name=$1,phone=$2,department_id=$3,active=$4,updated_at=now() WHERE id=$5", cmd.Name, cmd.Phone, cmd.DepartmentID, cmd.Active, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("employee not found")
	}
	return nil
}
