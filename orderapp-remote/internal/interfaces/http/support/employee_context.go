package support

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func EmployeeContextMiddleware(pool *pgxpool.Pool, schema string, eligibility ...ERPWorkbenchLoginEligibility) echo.MiddlewareFunc {
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
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token, eligibility...); err == nil && eid > 0 {
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

func CurrentEmployeeID(c echo.Context) int64 {
	return currentEmployeeID(c)
}

func resolveEmployeeBySessionToken(ctx echo.Context, pool *pgxpool.Pool, schema, token string, eligibility ...ERPWorkbenchLoginEligibility) (int64, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, "", nil
	}
	requestCtx := ctx.Request().Context()
	tx, err := pool.Begin(requestCtx)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = tx.Rollback(requestCtx) }()

	var id int64
	var name, accountType string
	var sessionCreatedAt time.Time
	var passwordUpdatedAt *time.Time
	var unexpired, employeeActive, loginDisabled bool
	err = tx.QueryRow(requestCtx, fmt.Sprintf(`
		SELECT e.id,
		       COALESCE(e.name,''),
		       COALESCE(NULLIF(TRIM(e.account_type),''),'internal_employee'),
		       s.created_at,
		       s.expire_at > now(),
		       e.active,
		       COALESCE(p.login_disabled,false),
		       p.updated_at
		FROM %s.login_sessions s
		JOIN %s.company_employees e ON e.id=s.employee_id
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE s.token=$1
		FOR UPDATE OF s
	`, schema, schema, schema), token).Scan(
		&id,
		&name,
		&accountType,
		&sessionCreatedAt,
		&unexpired,
		&employeeActive,
		&loginDisabled,
		&passwordUpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", err
	}
	revoke := func(reason error) (int64, string, error) {
		if err := revokeERPLoginSessionTx(requestCtx, tx, schema, token); err != nil {
			return 0, "", err
		}
		if err := tx.Commit(requestCtx); err != nil {
			return 0, "", err
		}
		return 0, "", reason
	}
	if !unexpired {
		return revoke(nil)
	}
	if !employeeActive || loginDisabled {
		return revoke(fmt.Errorf("ERP login unavailable"))
	}
	if passwordUpdatedAt != nil && sessionCreatedAt.Before(*passwordUpdatedAt) {
		return revoke(fmt.Errorf("ERP login session stale"))
	}

	accountType = strings.TrimSpace(accountType)
	if accountType == AccountTypeChannelCustomer {
		var channelSecurityUpdatedAt time.Time
		err := tx.QueryRow(requestCtx, fmt.Sprintf(`
			SELECT GREATEST(b.updated_at, COALESCE(lp.updated_at,b.updated_at))
			FROM %s.customer_erp_user_bindings b
			JOIN %s.company_employees e ON e.id=b.employee_id
			LEFT JOIN %s.employee_login_passwords lp ON lp.employee_id=e.id
			JOIN %s.customers c ON c.id=b.customer_id
			JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
			WHERE b.employee_id=$1
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type=$2
			  AND COALESCE(lp.login_disabled,false)=false
			  AND c.active=true
			  AND p.enabled=true
			ORDER BY b.id
			LIMIT 1
		`, schema, schema, schema, schema, schema), id, AccountTypeChannelCustomer).Scan(&channelSecurityUpdatedAt)
		if err == pgx.ErrNoRows {
			return revoke(fmt.Errorf("ERP login unavailable"))
		}
		if err != nil {
			return 0, "", err
		}
		if sessionCreatedAt.Before(channelSecurityUpdatedAt) {
			return revoke(fmt.Errorf("ERP login session stale"))
		}
	}
	if err := requireERPWorkbenchLoginEligibilityForAccount(requestCtx, id, accountType, eligibility...); err != nil {
		return revoke(err)
	}
	if err := tx.Commit(requestCtx); err != nil {
		return 0, "", err
	}
	return id, strings.TrimSpace(name), nil
}

func revokeERPLoginSessionTx(ctx context.Context, tx pgx.Tx, schema, token string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.login_sessions WHERE token=$1`, schema), strings.TrimSpace(token))
	return err
}

func requireERPWorkbenchLoginEligibility(ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64, eligibility ...ERPWorkbenchLoginEligibility) error {
	var accountType string
	if err := pool.QueryRow(ctx,
		"SELECT COALESCE(NULLIF(TRIM(account_type),''),'internal_employee') FROM "+schema+".company_employees WHERE id=$1 AND active=true LIMIT 1",
		employeeID,
	).Scan(&accountType); err != nil {
		return err
	}
	return requireERPWorkbenchLoginEligibilityForAccount(ctx, employeeID, accountType, eligibility...)
}

func requireERPWorkbenchLoginEligibilityForAccount(ctx context.Context, employeeID int64, accountType string, eligibility ...ERPWorkbenchLoginEligibility) error {
	switch strings.TrimSpace(accountType) {
	case AccountTypeInternalEmployee:
		return nil
	case AccountTypeChannelCustomer:
		if len(eligibility) == 0 || eligibility[0] == nil {
			return fmt.Errorf("ERP login eligibility unavailable")
		}
		if err := eligibility[0].RequireERPWorkbenchLogin(ctx, employeeID); err != nil {
			return fmt.Errorf("ERP login unavailable: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("ERP login unavailable for account type")
	}
}

func RequireEmployeeBound(c echo.Context) error {
	if currentEmployeeID(c) <= 0 {
		return fmt.Errorf("employee binding required")
	}
	return nil
}
