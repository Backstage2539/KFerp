package authz

import (
	"context"
	"fmt"

	authzapp "orderapp/internal/application/authz"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error) {
	var actor authzapp.Actor
	var accountType string
	err := r.pool.QueryRow(ctx,
		"SELECT id,COALESCE(name,''),COALESCE(NULLIF(account_type,''),'internal_employee') FROM "+r.schema+".company_employees WHERE id=$1 AND active=true LIMIT 1",
		employeeID,
	).Scan(&actor.EmployeeID, &actor.Name, &accountType)
	if err != nil {
		if err == pgx.ErrNoRows {
			return authzapp.Actor{}, fmt.Errorf("employee not found")
		}
		return authzapp.Actor{}, err
	}
	roles, err := r.rolesForEmployee(ctx, employeeID)
	if err != nil {
		return authzapp.Actor{}, err
	}
	views, err := r.viewPermissions(ctx)
	if err != nil {
		return authzapp.Actor{}, err
	}
	if accountType == "channel_customer" {
		roles = nil
		actor.Permissions = []string{"customer_processing.read", "customer_processing.submit"}
	}
	actor.AccountType = accountType
	actor.Roles = roles
	actor.ViewPermissions = views
	return actor, nil
}

func (r Repository) ListRoles(ctx context.Context) ([]authzapp.Role, error) {
	rows, err := r.pool.Query(ctx, `
SELECT r.code,r.name,COALESCE(p.code,'')
FROM `+r.schema+`.auth_roles r
LEFT JOIN `+r.schema+`.auth_role_permissions rp ON rp.role_code=r.code
LEFT JOIN `+r.schema+`.auth_permissions p ON p.code=rp.permission_code
WHERE r.code NOT IN ('customer_processing_customer','customer_direct_ship_customer')
ORDER BY r.code,p.code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r Repository) AssignEmployeeRoles(ctx context.Context, cmd authzapp.AssignmentCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var employeeID int64
	var accountType string
	err = tx.QueryRow(ctx, "SELECT id,COALESCE(NULLIF(account_type,''),'internal_employee') FROM "+r.schema+".company_employees WHERE id=$1 AND active=true LIMIT 1 FOR UPDATE", cmd.EmployeeID).Scan(&employeeID, &accountType)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("employee not found")
		}
		return err
	}
	if accountType == "channel_customer" && len(cmd.RoleCodes) > 0 {
		return fmt.Errorf("channel customer accounts cannot assign roles")
	}
	oldRows, err := tx.Query(ctx, "SELECT role_code FROM "+r.schema+".employee_roles WHERE employee_id=$1 ORDER BY role_code", cmd.EmployeeID)
	if err != nil {
		return err
	}
	oldRoleCodes := make([]string, 0)
	for oldRows.Next() {
		var roleCode string
		if err := oldRows.Scan(&roleCode); err != nil {
			oldRows.Close()
			return err
		}
		oldRoleCodes = append(oldRoleCodes, roleCode)
	}
	if err := oldRows.Err(); err != nil {
		oldRows.Close()
		return err
	}
	oldRows.Close()
	if _, err := tx.Exec(ctx, "DELETE FROM "+r.schema+".employee_roles WHERE employee_id=$1", cmd.EmployeeID); err != nil {
		return err
	}
	for _, roleCode := range cmd.RoleCodes {
		tag, err := tx.Exec(ctx, "INSERT INTO "+r.schema+".employee_roles(employee_id,role_code) SELECT $1,code FROM "+r.schema+".auth_roles WHERE code=$2", cmd.EmployeeID, roleCode)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("unknown role: %s", roleCode)
		}
	}
	if !sameEmployeeRoleSet(oldRoleCodes, cmd.RoleCodes) {
		if err := postgresinfra.ExpireEmployeeSecuritySessions(ctx, tx, r.schema, cmd.EmployeeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func sameEmployeeRoleSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		if counts[value] == 0 {
			return false
		}
		counts[value]--
	}
	return true
}

func (r Repository) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT employee_id,role_code FROM "+r.schema+".employee_roles ORDER BY employee_id,role_code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var employeeID int64
		var roleCode string
		if err := rows.Scan(&employeeID, &roleCode); err != nil {
			return nil, err
		}
		out[employeeID] = append(out[employeeID], roleCode)
	}
	return out, rows.Err()
}

func (r Repository) rolesForEmployee(ctx context.Context, employeeID int64) ([]authzapp.Role, error) {
	rows, err := r.pool.Query(ctx, `
SELECT r.code,r.name,COALESCE(p.code,'')
FROM `+r.schema+`.employee_roles er
JOIN `+r.schema+`.auth_roles r ON r.code=er.role_code
LEFT JOIN `+r.schema+`.auth_role_permissions rp ON rp.role_code=r.code
LEFT JOIN `+r.schema+`.auth_permissions p ON p.code=rp.permission_code
WHERE er.employee_id=$1
ORDER BY r.code,p.code`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r Repository) viewPermissions(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT view_key,permission_code FROM "+r.schema+".auth_view_permissions ORDER BY view_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var viewKey, permission string
		if err := rows.Scan(&viewKey, &permission); err != nil {
			return nil, err
		}
		out[viewKey] = permission
	}
	return out, rows.Err()
}

func scanRoles(rows pgx.Rows) ([]authzapp.Role, error) {
	byCode := map[string]*authzapp.Role{}
	order := []string{}
	for rows.Next() {
		var code, name, permission string
		if err := rows.Scan(&code, &name, &permission); err != nil {
			return nil, err
		}
		role := byCode[code]
		if role == nil {
			role = &authzapp.Role{Code: code, Name: name}
			byCode[code] = role
			order = append(order, code)
		}
		if permission != "" {
			role.Permissions = append(role.Permissions, permission)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]authzapp.Role, 0, len(order))
	for _, code := range order {
		out = append(out, *byCode[code])
	}
	return out, nil
}
