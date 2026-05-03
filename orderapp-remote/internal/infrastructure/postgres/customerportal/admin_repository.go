package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListPortalAdminCustomers(ctx context.Context, query customerportalapp.PortalAdminCustomerQuery) ([]customerportalapp.PortalAdminCustomer, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := strings.TrimSpace(query.Query)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT c.id,
		       COALESCE(c.name,''),
		       COALESCE(c.phone,''),
		       COALESCE(c.company_name,''),
		       COALESCE(p.display_name,''),
		       COALESCE(p.enabled,true),
		       COALESCE(p.status,'active'),
		       COUNT(b.id) FILTER (WHERE b.status='approved')::int
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		LEFT JOIN %s.customer_portal_user_bindings b ON b.customer_id=c.id
		WHERE c.active=true
		  AND ($1='' OR c.name ILIKE '%%' || $1 || '%%' OR c.phone ILIKE '%%' || $1 || '%%' OR c.company_name ILIKE '%%' || $1 || '%%')
		GROUP BY c.id, c.name, c.phone, c.company_name, p.display_name, p.enabled, p.status
		ORDER BY c.name, c.id
		LIMIT $2
	`, r.schema, r.schema, r.schema), q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.PortalAdminCustomer, 0)
	for rows.Next() {
		var row customerportalapp.PortalAdminCustomer
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.PortalEnabled, &row.PortalStatus, &row.BindingCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) PortalAdminDetail(ctx context.Context, customerID int64) (customerportalapp.PortalAdminDetail, error) {
	customer, err := r.portalAdminCustomer(ctx, customerID)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	bindings, err := r.portalAdminBindings(ctx, customerID)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	capabilities, err := r.portalAdminCapabilities(ctx, customerID)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	return customerportalapp.PortalAdminDetail{
		Customer:     customer,
		Bindings:     bindings,
		Capabilities: capabilities,
	}, nil
}

func (r Repository) UpdatePortalVisibility(ctx context.Context, cmd customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT 1 FROM %s.customers WHERE id=$1`, r.schema), cmd.CustomerID).Scan(&exists); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
		}
		return customerportalapp.PortalAdminDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, enabled, status, updated_at, updated_by)
		VALUES($1,$2,$3,'active',now(),$4)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=excluded.display_name,
			enabled=excluded.enabled,
			status='active',
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.CustomerID, strings.TrimSpace(cmd.DisplayName), cmd.Enabled, strings.TrimSpace(cmd.UpdatedBy)); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	for _, capability := range cmd.Capabilities {
		raw, err := json.Marshal(map[string]any{})
		if err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
		if capability.Config != nil {
			raw, err = json.Marshal(capability.Config)
			if err != nil {
				return customerportalapp.PortalAdminDetail{}, err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled, config_json, updated_at)
			VALUES($1,$2,$3,$4::jsonb,now())
			ON CONFLICT(customer_id, capability_code) DO UPDATE SET
				enabled=excluded.enabled,
				config_json=excluded.config_json,
				updated_at=now()
		`, r.schema), cmd.CustomerID, capability.Code, capability.Enabled, string(raw)); err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	return r.PortalAdminDetail(ctx, cmd.CustomerID)
}

func (r Repository) portalAdminCustomer(ctx context.Context, customerID int64) (customerportalapp.PortalAdminCustomer, error) {
	var row customerportalapp.PortalAdminCustomer
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.id,
		       COALESCE(c.name,''),
		       COALESCE(c.phone,''),
		       COALESCE(c.company_name,''),
		       COALESCE(p.display_name,''),
		       COALESCE(p.enabled,true),
		       COALESCE(p.status,'active'),
		       COALESCE((SELECT COUNT(*)::int FROM %s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.status='approved'),0)
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1
	`, r.schema, r.schema, r.schema), customerID).Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.PortalEnabled, &row.PortalStatus, &row.BindingCount)
	if err == pgx.ErrNoRows {
		return customerportalapp.PortalAdminCustomer{}, customerportalapp.ErrPortalCustomerNotFound
	}
	return row, err
}

func (r Repository) portalAdminBindings(ctx context.Context, customerID int64) ([]customerportalapp.PortalUserBinding, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.mini_user_id,
		       COALESCE(u.openid,''),
		       COALESCE(u.phone,''),
		       COALESCE(u.nickname,''),
		       b.role,
		       b.status,
		       to_char(b.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_portal_user_bindings b
		JOIN %s.mini_users u ON u.id=b.mini_user_id
		WHERE b.customer_id=$1
		ORDER BY b.status, b.id
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.PortalUserBinding, 0)
	for rows.Next() {
		var row customerportalapp.PortalUserBinding
		if err := rows.Scan(&row.MiniUserID, &row.OpenID, &row.Phone, &row.Nickname, &row.Role, &row.Status, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) portalAdminCapabilities(ctx context.Context, customerID int64) ([]customerportalapp.CapabilityOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT capability_code, enabled, config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1
		ORDER BY capability_code
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CapabilityOption, 0)
	for rows.Next() {
		var row customerportalapp.CapabilityOption
		var raw []byte
		if err := rows.Scan(&row.Code, &row.Enabled, &raw); err != nil {
			return nil, err
		}
		row.Config = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &row.Config)
			if row.Config == nil {
				row.Config = map[string]any{}
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
