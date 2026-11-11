package customerportal

import (
	"context"
	"fmt"
	"strconv"

	portal "orderapp/internal/application/customerportal"
)

// AdminPreviewCustomer derives the customer's own portal configuration without
// creating a customer user, binding, or changing any session.
func (r Repository) AdminPreviewCustomer(ctx context.Context, customerID int64) (portal.CurrentContext, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return portal.CurrentContext{}, err
	}
	defer tx.Rollback(ctx)
	var current portal.CurrentContext
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT c.id,COALESCE(NULLIF(p.display_name,''),c.name,''),COALESCE(c.contact,''),COALESCE(c.phone,''),COALESCE(c.address,''),COALESCE(e.name,''),COALESCE(e.phone,'') FROM %s.customers c JOIN %s.customer_portal_profiles p ON p.customer_id=c.id AND p.enabled=true LEFT JOIN %s.company_employees e ON e.id=NULLIF(c.responsible_employee_id,0) WHERE c.id=$1 AND c.active=true`, r.schema, r.schema, r.schema), customerID).Scan(&current.CurrentCustomerID, &current.CurrentCustomerName, &current.CurrentCustomerContact, &current.CurrentCustomerPhone, &current.CurrentCustomerAddress, &current.BusinessContactName, &current.BusinessContactPhone)
	if err != nil {
		return portal.CurrentContext{}, err
	}
	current.Capabilities, err = r.capabilitiesForCustomerTx(ctx, tx, customerID)
	if err != nil {
		return portal.CurrentContext{}, err
	}
	current.ThemeKey, err = r.themeForCustomerTx(ctx, tx, customerID)
	if err != nil {
		return portal.CurrentContext{}, err
	}
	current.MiniappEntryMode, err = r.miniappEntryModeForCustomerTx(ctx, tx, customerID)
	if err != nil {
		return portal.CurrentContext{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return portal.CurrentContext{}, err
	}
	return current, nil
}

// The user-visible audit log records employee and customer IDs but no tokens.
func (r Repository) AdminPreviewAudit(ctx context.Context, employeeID, customerID int64, action string) error {
	if employeeID <= 0 || customerID <= 0 || (action != "enter" && action != "exit") {
		return portal.ErrAdminPreviewDenied
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.audit_logs(actor,entity_type,entity_id,action,field,new_value,meta) VALUES($1,'customer_preview',$2,$3,'customer_id',$4,jsonb_build_object('employee_id',$5::bigint,'customer_id',$2::bigint))`, r.schema), "employee:"+strconv.FormatInt(employeeID, 10), customerID, action, strconv.FormatInt(customerID, 10), employeeID)
	return err
}

// xmin captures intervening row updates even when a customer or employee is
// disabled and re-enabled between requests. Changes fail closed; this value
// is kept only in server memory and is never returned to the client.
func (r Repository) AdminPreviewSecurityVersion(ctx context.Context, employeeID, customerID int64) (string, error) {
	var version string
	q := fmt.Sprintf(`SELECT c.xmin::text||':'||p.xmin::text||':'||e.xmin::text||':'||
	 COALESCE((SELECT string_agg(er.role_code||':'||er.xmin::text,',' ORDER BY er.role_code) FROM %[1]s.employee_roles er WHERE er.employee_id=e.id),'')||':'||
	 COALESCE((SELECT string_agg(rp.role_code||':'||rp.permission_code||':'||rp.xmin::text,',' ORDER BY rp.role_code,rp.permission_code) FROM %[1]s.auth_role_permissions rp JOIN %[1]s.employee_roles er ON er.role_code=rp.role_code WHERE er.employee_id=e.id),'')
	 FROM %[1]s.customers c JOIN %[1]s.customer_portal_profiles p ON p.customer_id=c.id AND p.enabled=true JOIN %[1]s.company_employees e ON e.id=$1 AND e.active=true WHERE c.id=$2 AND c.active=true`, r.schema)
	err := r.pool.QueryRow(ctx, q, employeeID, customerID).Scan(&version)
	return version, err
}

// Only the supplied employee mini session is expired; other devices and
// customer sessions are unaffected. The session change and audit are atomic.
func (r Repository) LogoutPreviewEmployee(ctx context.Context, token string, employeeID int64) error {
	if token == "" || employeeID <= 0 {
		return portal.ErrAdminPreviewDenied
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var miniUserID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.mini_sessions SET expire_at=now() WHERE token=$1 AND expire_at>now() RETURNING mini_user_id`, r.schema), token).Scan(&miniUserID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.audit_logs(actor,entity_type,entity_id,action,meta) VALUES($1,'mini_employee_session',$2,'logout',jsonb_build_object('employee_id',$2::bigint,'mini_user_id',$3::bigint))`, r.schema), "employee:"+strconv.FormatInt(employeeID, 10), employeeID, miniUserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
