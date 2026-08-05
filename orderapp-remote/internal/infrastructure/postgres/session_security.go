package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// SessionSecurityQueryExecer is implemented by pgx transactions and pools.
// Security writers should pass their current transaction so state changes and
// session expiry either commit or roll back together.
type SessionSecurityQueryExecer interface {
	AuditExecer
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ExpireEmployeeSecuritySessions(ctx context.Context, q SessionSecurityQueryExecer, schema string, employeeID int64) error {
	if employeeID <= 0 {
		return nil
	}
	hasMini, err := sessionSecurityRelationsExist(ctx, q, schema, "mini_sessions", "mini_users", "customer_portal_user_bindings")
	if err != nil {
		return err
	}
	if hasMini {
		if _, err := q.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.mini_sessions s
			SET expire_at=LEAST(s.expire_at,now())
			WHERE s.expire_at>now()
			  AND (
				EXISTS (
					SELECT 1
					FROM %[1]s.mini_users u
					WHERE u.id=s.mini_user_id
					  AND u.openid IN ('erp-employee:' || $1::bigint::text, 'erp-internal-employee:' || $1::bigint::text)
				)
				OR EXISTS (
					SELECT 1
					FROM %[1]s.customer_portal_user_bindings b
					WHERE b.mini_user_id=s.mini_user_id
					  AND b.approved_by IN ('erp-password-login:' || $1::bigint::text, 'phone-verify-login:' || $1::bigint::text)
				)
			  )
		`, schema), employeeID); err != nil {
			return err
		}
	}

	hasERP, err := sessionSecurityRelationsExist(ctx, q, schema, "login_sessions")
	if err != nil {
		return err
	}
	if hasERP {
		if _, err := q.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.login_sessions
			SET expire_at=LEAST(expire_at,now())
			WHERE employee_id=$1 AND expire_at>now()
		`, schema), employeeID); err != nil {
			return err
		}
	}
	return nil
}

func ExpireCustomerSecuritySessions(ctx context.Context, q SessionSecurityQueryExecer, schema string, customerID int64) error {
	if customerID <= 0 {
		return nil
	}
	hasMini, err := sessionSecurityRelationsExist(ctx, q, schema, "mini_sessions", "customer_portal_user_bindings")
	if err != nil {
		return err
	}
	if hasMini {
		if _, err := q.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.mini_sessions s
			SET expire_at=LEAST(s.expire_at,now())
			WHERE s.expire_at>now()
			  AND (
				s.current_customer_id=$1
				OR EXISTS (
					SELECT 1
					FROM %[1]s.customer_portal_user_bindings b
					WHERE b.mini_user_id=s.mini_user_id AND b.customer_id=$1 AND b.status='approved'
				)
			  )
		`, schema), customerID); err != nil {
			return err
		}
	}
	return ExpireCustomerERPSessions(ctx, q, schema, customerID)
}

func ExpireCustomerERPSessions(ctx context.Context, q SessionSecurityQueryExecer, schema string, customerID int64) error {
	if customerID <= 0 {
		return nil
	}
	hasERP, err := sessionSecurityRelationsExist(ctx, q, schema, "login_sessions", "customer_erp_user_bindings")
	if err != nil {
		return err
	}
	if !hasERP {
		return nil
	}
	_, err = q.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.login_sessions s
		SET expire_at=LEAST(s.expire_at,now())
		WHERE s.expire_at>now()
		  AND EXISTS (
			SELECT 1
			FROM %[1]s.customer_erp_user_bindings b
			WHERE b.employee_id=s.employee_id AND b.customer_id=$1 AND b.status='active'
		  )
	`, schema), customerID)
	return err
}

func ExpireCapabilityTemplateERPSessions(ctx context.Context, q SessionSecurityQueryExecer, schema, templateKey string) error {
	templateKey = strings.TrimSpace(templateKey)
	if templateKey == "" {
		return nil
	}
	hasERP, err := sessionSecurityRelationsExist(ctx, q, schema, "login_sessions", "customer_erp_user_bindings", "customer_portal_profiles")
	if err != nil {
		return err
	}
	if !hasERP {
		return nil
	}
	_, err = q.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.login_sessions s
		SET expire_at=LEAST(s.expire_at,now())
		WHERE s.expire_at>now()
		  AND EXISTS (
			SELECT 1
			FROM %[1]s.customer_erp_user_bindings b
			JOIN %[1]s.customer_portal_profiles p ON p.customer_id=b.customer_id
			WHERE b.employee_id=s.employee_id
			  AND b.status='active'
			  AND p.capability_template_key=$1
		  )
	`, schema), templateKey)
	return err
}

func sessionSecurityRelationsExist(ctx context.Context, q SessionSecurityQueryExecer, schema string, relationNames ...string) (bool, error) {
	if len(relationNames) == 0 {
		return true, nil
	}
	checks := make([]string, 0, len(relationNames))
	args := make([]any, 0, len(relationNames))
	for i, relationName := range relationNames {
		checks = append(checks, fmt.Sprintf("to_regclass($%d) IS NOT NULL", i+1))
		args = append(args, fmt.Sprintf("%s.%s", schema, strings.TrimSpace(relationName)))
	}
	var exists bool
	err := q.QueryRow(ctx, "SELECT "+strings.Join(checks, " AND "), args...).Scan(&exists)
	return exists, err
}
