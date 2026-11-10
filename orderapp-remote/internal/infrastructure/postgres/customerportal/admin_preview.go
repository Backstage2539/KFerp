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
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT c.id,COALESCE(c.name,''),COALESCE(c.contact,''),COALESCE(c.phone,''),COALESCE(c.address,''),COALESCE(e.name,''),COALESCE(e.phone,'') FROM %s.customers c LEFT JOIN %s.company_employees e ON e.id=NULLIF(c.responsible_employee_id,0) WHERE c.id=$1 AND c.active=true`, r.schema, r.schema), customerID).Scan(&current.CurrentCustomerID, &current.CurrentCustomerName, &current.CurrentCustomerContact, &current.CurrentCustomerPhone, &current.CurrentCustomerAddress, &current.BusinessContactName, &current.BusinessContactPhone)
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
