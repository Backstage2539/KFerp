package customerportal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	postgresinfra "orderapp/internal/infrastructure/postgres"

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
		       COALESCE(NULLIF(c.customer_type,''),'retail'),
		       COALESCE(c.phone,''),
		       COALESCE(c.company_name,''),
		       COALESCE(p.display_name,''),
		       COALESCE(p.processing_warehouse_code,''),
		       COALESCE(p.default_sender_id,0),
		       COALESCE(p.enabled,true),
		       COALESCE(p.status,'active'),
		       COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
		       COALESCE(NULLIF(p.miniapp_entry_mode,''),'services'),
		       COALESCE(p.capability_template_key,''),
		       COALESCE(NULLIF(p.bean_list_mode,''),'latest'),
		       COALESCE(p.bean_list_publication_id,0),
		       COALESCE((SELECT COUNT(*)::int FROM %s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.status='approved'),0),
		       eb.employee_id,
		       eb.employee_name,
		       eb.employee_phone,
		       eb.role,
		       eb.status,
		       eb.updated_by,
		       eb.updated_at
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		LEFT JOIN LATERAL (
			SELECT b.employee_id,
			       COALESCE(e.name,'') AS employee_name,
			       COALESCE(e.phone,'') AS employee_phone,
			       b.role,
			       b.status,
			       b.updated_by,
			       to_char(b.updated_at,'YYYY-MM-DD HH24:MI') AS updated_at
			FROM %s.customer_erp_user_bindings b
			JOIN %s.company_employees e ON e.id=b.employee_id
			LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
			WHERE b.customer_id=c.id
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type='channel_customer'
			  AND COALESCE(p.login_disabled,false)=false
			ORDER BY b.updated_at DESC, b.id DESC
			LIMIT 1
		) eb ON true
		WHERE c.active=true
		  AND COALESCE(NULLIF(c.customer_type,''),'retail')='wholesale'
		  AND ($1='' OR c.name ILIKE '%%' || $1 || '%%' OR c.phone ILIKE '%%' || $1 || '%%' OR c.company_name ILIKE '%%' || $1 || '%%')
		ORDER BY c.name, c.id
		LIMIT $2
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.PortalAdminCustomer, 0)
	for rows.Next() {
		var row customerportalapp.PortalAdminCustomer
		var employeeID sql.NullInt64
		var employeeName, employeePhone, role, status, updatedBy, updatedAt sql.NullString
		if err := rows.Scan(&row.ID, &row.Name, &row.CustomerType, &row.Phone, &row.CompanyName, &row.DisplayName, &row.ProcessingWarehouseCode, &row.DefaultSenderID, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.MiniappEntryMode, &row.CapabilityTemplateKey, &row.BeanListMode, &row.BeanListPublicationID, &row.BindingCount, &employeeID, &employeeName, &employeePhone, &role, &status, &updatedBy, &updatedAt); err != nil {
			return nil, err
		}
		row.ERPBinding = nullableERPBinding(row.ID, employeeID, employeeName, employeePhone, role, status, updatedBy, updatedAt)
		row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
		row.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(row.MiniappEntryMode)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListCapabilityTemplates(ctx context.Context) ([]customerportalapp.CapabilityTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT template_key,
		       parent_template_key,
		       label,
		       description,
		       theme_key,
		       miniapp_entry_mode,
		       erp_role_codes,
		       erp_permissions,
		       erp_view_keys,
		       capabilities_json,
		       active,
		       sort_order,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI'),
		       updated_by
		FROM %s.customer_capability_templates
		ORDER BY sort_order, parent_template_key, template_key
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CapabilityTemplate, 0)
	for rows.Next() {
		template, err := scanCapabilityTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, template)
	}
	return out, rows.Err()
}

func (r Repository) SaveCapabilityTemplate(ctx context.Context, cmd customerportalapp.SaveCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error) {
	roleCodes, err := json.Marshal(cmd.Template.ERPRoleCodes)
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	permissions, err := json.Marshal(cmd.Template.ERPPermissions)
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	viewKeys, err := json.Marshal(cmd.Template.ERPViewKeys)
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	capabilities, err := json.Marshal(cmd.Template.Capabilities)
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_capability_templates(
			template_key, parent_template_key, label, description, theme_key, miniapp_entry_mode,
			erp_role_codes, erp_permissions, erp_view_keys, capabilities_json,
			active, sort_order, updated_at, updated_by
		)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb,$10::jsonb,$11,$12,now(),$13)
		ON CONFLICT(template_key) DO UPDATE SET
			parent_template_key=excluded.parent_template_key,
			label=excluded.label,
			description=excluded.description,
			theme_key=excluded.theme_key,
			miniapp_entry_mode=excluded.miniapp_entry_mode,
			erp_role_codes=excluded.erp_role_codes,
			erp_permissions=excluded.erp_permissions,
			erp_view_keys=excluded.erp_view_keys,
			capabilities_json=excluded.capabilities_json,
			active=excluded.active,
			sort_order=excluded.sort_order,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema),
		cmd.Template.Key,
		strings.TrimSpace(cmd.Template.ParentTemplateKey),
		strings.TrimSpace(cmd.Template.Label),
		strings.TrimSpace(cmd.Template.Description),
		customerportalapp.NormalizePortalThemeKey(cmd.Template.ThemeKey),
		customerportalapp.NormalizeMiniappEntryMode(cmd.Template.MiniappEntryMode),
		string(roleCodes),
		string(permissions),
		string(viewKeys),
		string(capabilities),
		cmd.Template.Active,
		cmd.Template.SortOrder,
		strings.TrimSpace(cmd.UpdatedBy),
	); err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	return r.capabilityTemplateByKey(ctx, cmd.Template.Key)
}

func (r Repository) capabilityTemplateByKey(ctx context.Context, key string) (customerportalapp.CapabilityTemplate, error) {
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT template_key,
		       parent_template_key,
		       label,
		       description,
		       theme_key,
		       miniapp_entry_mode,
		       erp_role_codes,
		       erp_permissions,
		       erp_view_keys,
		       capabilities_json,
		       active,
		       sort_order,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI'),
		       updated_by
		FROM %s.customer_capability_templates
		WHERE template_key=$1
	`, r.schema), customerportalapp.NormalizeCapabilityTemplateKey(key))
	return scanCapabilityTemplate(row)
}

type capabilityTemplateScanner interface {
	Scan(dest ...any) error
}

func scanCapabilityTemplate(row capabilityTemplateScanner) (customerportalapp.CapabilityTemplate, error) {
	var template customerportalapp.CapabilityTemplate
	var roleCodesRaw, permissionsRaw, viewKeysRaw, capabilitiesRaw []byte
	if err := row.Scan(
		&template.Key,
		&template.ParentTemplateKey,
		&template.Label,
		&template.Description,
		&template.ThemeKey,
		&template.MiniappEntryMode,
		&roleCodesRaw,
		&permissionsRaw,
		&viewKeysRaw,
		&capabilitiesRaw,
		&template.Active,
		&template.SortOrder,
		&template.UpdatedAt,
		&template.UpdatedBy,
	); err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	template.ERPRoleCodes = decodeStringSlice(roleCodesRaw)
	template.ERPPermissions = decodeStringSlice(permissionsRaw)
	template.ERPViewKeys = decodeStringSlice(viewKeysRaw)
	if len(capabilitiesRaw) > 0 {
		_ = json.Unmarshal(capabilitiesRaw, &template.Capabilities)
	}
	if template.Capabilities == nil {
		template.Capabilities = []customerportalapp.CapabilityOption{}
	}
	return template, nil
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
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
		Customer:               customer,
		Bindings:               bindings,
		Capabilities:           capabilities,
		BeanListVersionOptions: r.portalBeanListVersionOptions(ctx, customerID),
	}, nil
}

func (r Repository) UpdatePortalVisibility(ctx context.Context, cmd customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.customers WHERE id=$1`, r.schema), cmd.CustomerID).Scan(&customerName); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
		}
		return customerportalapp.PortalAdminDetail{}, err
	}
	warehouseCode := strings.TrimSpace(cmd.ProcessingWarehouseCode)
	if warehouseCode == "" && capabilityEnabled(cmd.Capabilities, customerportalapp.CapabilityProcessing) {
		warehouseCode = defaultProcessingWarehouseCode(cmd.CustomerID)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, processing_warehouse_code, default_sender_id, enabled, status, theme_key, miniapp_entry_mode, capability_template_key, bean_list_mode, bean_list_publication_id, updated_at, updated_by)
		VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8,$9,$10,now(),$11)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=excluded.display_name,
			processing_warehouse_code=excluded.processing_warehouse_code,
			default_sender_id=excluded.default_sender_id,
			enabled=excluded.enabled,
			status='active',
			theme_key=excluded.theme_key,
			miniapp_entry_mode=excluded.miniapp_entry_mode,
			capability_template_key=excluded.capability_template_key,
			bean_list_mode=excluded.bean_list_mode,
			bean_list_publication_id=excluded.bean_list_publication_id,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.CustomerID, strings.TrimSpace(cmd.DisplayName), warehouseCode, cmd.DefaultSenderID, cmd.Enabled, customerportalapp.NormalizePortalThemeKey(cmd.ThemeKey), customerportalapp.NormalizeMiniappEntryMode(cmd.MiniappEntryMode), customerportalapp.NormalizeCapabilityTemplateKey(cmd.CapabilityTemplateKey), strings.TrimSpace(cmd.BeanListMode), cmd.BeanListPublicationID, strings.TrimSpace(cmd.UpdatedBy)); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	if warehouseCode != "" {
		if err := r.ensureProcessingWarehouseTx(ctx, tx, warehouseCode, firstNonEmpty(strings.TrimSpace(cmd.DisplayName), customerName)); err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
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
	if cmd.Template.Key != "" {
		if err := r.grantTemplateERPRolesTx(ctx, tx, cmd.CustomerID, cmd.Template); err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(cmd.UpdatedBy), "customer_portal_profile", &cmd.CustomerID, "update", postgresinfra.StrPtr("bean_list_version"), nil, postgresinfra.StrPtr(fmt.Sprintf("%s:%d", strings.TrimSpace(cmd.BeanListMode), cmd.BeanListPublicationID)), postgresinfra.AuditMeta{
		"customer_id":              cmd.CustomerID,
		"bean_list_mode":           strings.TrimSpace(cmd.BeanListMode),
		"bean_list_publication_id": cmd.BeanListPublicationID,
	}); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	return r.PortalAdminDetail(ctx, cmd.CustomerID)
}

func (r Repository) ApplyCapabilityTemplate(ctx context.Context, cmd customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerName, displayName, warehouseCode string
	var defaultSenderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(c.name,''),
		       COALESCE(p.display_name,''),
		       COALESCE(p.processing_warehouse_code,''),
		       COALESCE(p.default_sender_id,0)
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1 AND c.active=true
	`, r.schema, r.schema), cmd.CustomerID).Scan(&customerName, &displayName, &warehouseCode, &defaultSenderID); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
		}
		return customerportalapp.PortalAdminDetail{}, err
	}

	if warehouseCode == "" && capabilityEnabled(cmd.Template.Capabilities, customerportalapp.CapabilityProcessing) {
		warehouseCode = defaultProcessingWarehouseCode(cmd.CustomerID)
	}
	displayName = firstNonEmpty(displayName, customerName)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, processing_warehouse_code, default_sender_id, enabled, status, theme_key, miniapp_entry_mode, capability_template_key, updated_at, updated_by)
		VALUES($1,$2,$3,$4,true,'active',$5,$6,$7,now(),$8)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=excluded.display_name,
			processing_warehouse_code=excluded.processing_warehouse_code,
			default_sender_id=excluded.default_sender_id,
			enabled=true,
			status='active',
			theme_key=excluded.theme_key,
			miniapp_entry_mode=excluded.miniapp_entry_mode,
			capability_template_key=excluded.capability_template_key,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.CustomerID, displayName, warehouseCode, defaultSenderID, customerportalapp.NormalizePortalThemeKey(cmd.Template.ThemeKey), customerportalapp.NormalizeMiniappEntryMode(cmd.Template.MiniappEntryMode), cmd.Template.Key, strings.TrimSpace(cmd.UpdatedBy)); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	if warehouseCode != "" && capabilityEnabled(cmd.Template.Capabilities, customerportalapp.CapabilityProcessing) {
		if err := r.ensureProcessingWarehouseTx(ctx, tx, warehouseCode, displayName); err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
	}
	for _, capability := range cmd.Template.Capabilities {
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
	if err := r.grantTemplateERPRolesTx(ctx, tx, cmd.CustomerID, cmd.Template); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	return r.PortalAdminDetail(ctx, cmd.CustomerID)
}

func (r Repository) UpsertPortalERPBinding(ctx context.Context, cmd customerportalapp.UpsertPortalERPBindingCommand) (customerportalapp.PortalAdminDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customers
		WHERE id=$1 AND active=true AND COALESCE(NULLIF(customer_type,''),'retail')='wholesale'
	`, r.schema), cmd.CustomerID).Scan(&customerID); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
		}
		return customerportalapp.PortalAdminDetail{}, err
	}

	var employeeID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT e.id
		FROM %s.company_employees e
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE e.id=$1 AND e.active=true AND e.account_type='channel_customer' AND COALESCE(p.login_disabled,false)=false
	`, r.schema, r.schema), cmd.EmployeeID).Scan(&employeeID); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, fmt.Errorf("login-enabled channel customer account required")
		}
		return customerportalapp.PortalAdminDetail{}, err
	}

	status := "active"
	if strings.TrimSpace(cmd.Status) == "inactive" {
		status = "inactive"
	}
	if status == "active" {
		var otherCustomerID int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT customer_id
			FROM %s.customer_erp_user_bindings
			WHERE employee_id=$1 AND status='active' AND customer_id<>$2
			LIMIT 1
		`, r.schema), cmd.EmployeeID, customerID).Scan(&otherCustomerID)
		if err == nil && otherCustomerID > 0 {
			return customerportalapp.PortalAdminDetail{}, fmt.Errorf("erp account already bound to another customer")
		}
		if err != nil && err != pgx.ErrNoRows {
			return customerportalapp.PortalAdminDetail{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_erp_user_bindings
			SET status='inactive', updated_at=now(), updated_by=$3
			WHERE customer_id=$1 AND status='active' AND employee_id<>$2
		`, r.schema), customerID, cmd.EmployeeID, strings.TrimSpace(cmd.UpdatedBy)); err != nil {
			return customerportalapp.PortalAdminDetail{}, err
		}
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by, updated_at)
		VALUES($1,$2,'customer',$3,$4,now())
		ON CONFLICT(employee_id, customer_id) DO UPDATE SET
			role='customer',
			status=excluded.status,
			updated_by=excluded.updated_by,
			updated_at=now()
	`, r.schema), customerID, cmd.EmployeeID, status, strings.TrimSpace(cmd.UpdatedBy)); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.PortalAdminDetail{}, err
	}
	return r.PortalAdminDetail(ctx, customerID)
}

func (r Repository) grantTemplateERPRolesTx(ctx context.Context, tx pgx.Tx, customerID int64, template customerportalapp.CapabilityTemplate) error {
	roleCodes := make([]string, 0, len(template.ERPRoleCodes))
	seen := map[string]bool{}
	for _, roleCode := range template.ERPRoleCodes {
		roleCode = strings.TrimSpace(roleCode)
		if roleCode == "" || seen[roleCode] {
			continue
		}
		seen[roleCode] = true
		roleCodes = append(roleCodes, roleCode)
	}
	if len(roleCodes) == 0 {
		return nil
	}
	var hasAuthRoles, hasEmployeeRoles, hasBindings bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL, to_regclass($3) IS NOT NULL`,
		fmt.Sprintf("%s.auth_roles", r.schema),
		fmt.Sprintf("%s.employee_roles", r.schema),
		fmt.Sprintf("%s.customer_erp_user_bindings", r.schema),
	).Scan(&hasAuthRoles, &hasEmployeeRoles, &hasBindings); err != nil {
		return err
	}
	if !hasAuthRoles || !hasEmployeeRoles || !hasBindings {
		return nil
	}
	for _, roleCode := range roleCodes {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.employee_roles(employee_id, role_code)
			SELECT b.employee_id, r.code
			FROM %s.customer_erp_user_bindings b
			JOIN %s.auth_roles r ON r.code=$2
			WHERE b.customer_id=$1 AND b.status='active'
			ON CONFLICT DO NOTHING
		`, r.schema, r.schema, r.schema), customerID, roleCode); err != nil {
			return err
		}
	}
	return nil
}

func capabilityEnabled(rows []customerportalapp.CapabilityOption, code string) bool {
	for _, row := range rows {
		if row.Code == code && row.Enabled {
			return true
		}
	}
	return false
}

func defaultProcessingWarehouseCode(customerID int64) string {
	return fmt.Sprintf("cust_%d_processing", customerID)
}

func (r Repository) ensureProcessingWarehouseTx(ctx context.Context, tx pgx.Tx, code, customerName string) error {
	var hasWarehouseTable bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.warehouses", r.schema)).Scan(&hasWarehouseTable); err != nil {
		return err
	}
	if !hasWarehouseTable {
		return nil
	}
	name := strings.TrimSpace(customerName)
	if name == "" {
		name = code
	}
	name += "-代加工仓"
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,parent_code,sort_order,is_default,active,description)
		VALUES($1,$2,'customer_processing','finished_goods',60,false,true,'客户代加工成品专属仓')
		ON CONFLICT(code) DO UPDATE SET
			name=excluded.name,
			kind=excluded.kind,
			parent_code=excluded.parent_code,
			active=true,
			description=excluded.description
	`, r.schema), code, name)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (r Repository) portalAdminCustomer(ctx context.Context, customerID int64) (customerportalapp.PortalAdminCustomer, error) {
	var row customerportalapp.PortalAdminCustomer
	var employeeID sql.NullInt64
	var employeeName, employeePhone, role, status, updatedBy, updatedAt sql.NullString
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.id,
		       COALESCE(c.name,''),
		       COALESCE(NULLIF(c.customer_type,''),'retail'),
		       COALESCE(c.phone,''),
		       COALESCE(c.company_name,''),
		       COALESCE(p.display_name,''),
		       COALESCE(p.processing_warehouse_code,''),
		       COALESCE(p.default_sender_id,0),
		       COALESCE(p.enabled,true),
		       COALESCE(p.status,'active'),
		       COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
		       COALESCE(NULLIF(p.miniapp_entry_mode,''),'services'),
		       COALESCE(p.capability_template_key,''),
		       COALESCE(NULLIF(p.bean_list_mode,''),'latest'),
		       COALESCE(p.bean_list_publication_id,0),
		       COALESCE((SELECT COUNT(*)::int FROM %s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.status='approved'),0),
		       eb.employee_id,
		       eb.employee_name,
		       eb.employee_phone,
		       eb.role,
		       eb.status,
		       eb.updated_by,
		       eb.updated_at
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		LEFT JOIN LATERAL (
			SELECT b.employee_id,
			       COALESCE(e.name,'') AS employee_name,
			       COALESCE(e.phone,'') AS employee_phone,
			       b.role,
			       b.status,
			       b.updated_by,
			       to_char(b.updated_at,'YYYY-MM-DD HH24:MI') AS updated_at
			FROM %s.customer_erp_user_bindings b
			JOIN %s.company_employees e ON e.id=b.employee_id
			LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
			WHERE b.customer_id=c.id
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type='channel_customer'
			  AND COALESCE(p.login_disabled,false)=false
			ORDER BY b.updated_at DESC, b.id DESC
			LIMIT 1
		) eb ON true
		WHERE c.id=$1
		`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), customerID).Scan(&row.ID, &row.Name, &row.CustomerType, &row.Phone, &row.CompanyName, &row.DisplayName, &row.ProcessingWarehouseCode, &row.DefaultSenderID, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.MiniappEntryMode, &row.CapabilityTemplateKey, &row.BeanListMode, &row.BeanListPublicationID, &row.BindingCount, &employeeID, &employeeName, &employeePhone, &role, &status, &updatedBy, &updatedAt)
	if err == nil {
		row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
		row.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(row.MiniappEntryMode)
		row.ERPBinding = nullableERPBinding(row.ID, employeeID, employeeName, employeePhone, role, status, updatedBy, updatedAt)
	}
	if err == pgx.ErrNoRows {
		return customerportalapp.PortalAdminCustomer{}, customerportalapp.ErrPortalCustomerNotFound
	}
	return row, err
}

func (r Repository) portalBeanListVersionOptions(ctx context.Context, customerID int64) []customerportalapp.BeanListVersionOption {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, to_char(published_at,'YYYY-MM-DD HH24:MI'), config_json, content_json
		FROM %s.bean_list_publications
		WHERE owner_type='customer' AND owner_key=$1 AND status='published'
		ORDER BY published_at DESC, id DESC
	`, r.schema), fmt.Sprintf("%d", customerID))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]customerportalapp.BeanListVersionOption, 0)
	for rows.Next() {
		var row customerportalapp.BeanListSummary
		var configJSON, contentJSON []byte
		if err := rows.Scan(&row.ID, &row.ListType, &row.VersionNo, &row.PublishedAt, &configJSON, &contentJSON); err != nil {
			return nil
		}
		if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
			return nil
		}
		out = append(out, customerportalapp.BeanListVersionOption{
			ID:          row.ID,
			ListType:    row.ListType,
			VersionNo:   row.VersionNo,
			Title:       row.Title,
			PublishedAt: row.PublishedAt,
			CacheKey:    row.CacheKey,
		})
	}
	return out
}

func nullableERPBinding(customerID int64, employeeID sql.NullInt64, employeeName, employeePhone, role, status, updatedBy, updatedAt sql.NullString) *customerportalapp.PortalERPBinding {
	if !employeeID.Valid || employeeID.Int64 <= 0 {
		return nil
	}
	return &customerportalapp.PortalERPBinding{
		CustomerID:   customerID,
		EmployeeID:   employeeID.Int64,
		EmployeeName: strings.TrimSpace(employeeName.String),
		Phone:        strings.TrimSpace(employeePhone.String),
		Role:         firstNonEmpty(role.String, "customer"),
		Status:       firstNonEmpty(status.String, "active"),
		UpdatedBy:    strings.TrimSpace(updatedBy.String),
		UpdatedAt:    strings.TrimSpace(updatedAt.String),
	}
}

func (r Repository) ListMallProducts(ctx context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(p.product_kind,''),'roasted'), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
		       m.image_url, m.spec_g, m.unit_price, m.template_key, m.status, m.sort_order,
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE p.active=true
		ORDER BY m.sort_order, m.id
	`, r.schema, r.schema))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	mallRows := make([]customerportalapp.MallProduct, 0)
	for rows.Next() {
		var row customerportalapp.MallProduct
		if err := rows.Scan(&row.ID, &row.ProductID, &row.ProductName, &row.ProductKind, &row.Title, &row.Subtitle, &row.Description, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt); err != nil {
			return nil, nil, err
		}
		row.TemplateKey = customerportalapp.NormalizeMallTemplateKey(row.TemplateKey)
		row.Status = customerportalapp.NormalizeMallProductStatus(row.Status)
		mallRows = append(mallRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	optionRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(name,''), COALESCE(NULLIF(product_kind,''),'roasted'), COALESCE(default_price,0)
		FROM %s.products
		WHERE active=true
		  AND %s
		ORDER BY name, id
	`, r.schema, mallProductPublicCatalogSQL("")))
	if err != nil {
		return nil, nil, err
	}
	defer optionRows.Close()
	// product_options for the admin API are returned alongside mall rows.
	productOptions := make([]customerportalapp.MallProductOption, 0)
	for optionRows.Next() {
		var row customerportalapp.MallProductOption
		if err := optionRows.Scan(&row.ID, &row.Name, &row.ProductKind, &row.DefaultPrice); err != nil {
			return nil, nil, err
		}
		productOptions = append(productOptions, row)
	}
	return mallRows, productOptions, optionRows.Err()
}

func (r Repository) SaveMallProduct(ctx context.Context, cmd customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error) {
	if err := r.ensureMallProductPublicCatalog(ctx, cmd.ProductID); err != nil {
		return customerportalapp.MallProduct{}, err
	}
	if cmd.ID > 0 {
		if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.mall_products(id, product_id, title, subtitle, description, image_url, spec_g, unit_price, template_key, status, sort_order, updated_at, updated_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),$12)
			ON CONFLICT(id) DO UPDATE SET
				product_id=excluded.product_id,
				title=excluded.title,
				subtitle=excluded.subtitle,
				description=excluded.description,
				image_url=excluded.image_url,
				spec_g=excluded.spec_g,
				unit_price=excluded.unit_price,
				template_key=excluded.template_key,
				status=excluded.status,
				sort_order=excluded.sort_order,
				updated_at=now(),
				updated_by=excluded.updated_by
		`, r.schema), cmd.ID, cmd.ProductID, cmd.Title, cmd.Subtitle, cmd.Description, cmd.ImageURL, cmd.SpecG, cmd.UnitPrice, customerportalapp.NormalizeMallTemplateKey(cmd.TemplateKey), customerportalapp.NormalizeMallProductStatus(cmd.Status), cmd.SortOrder, strings.TrimSpace(cmd.Actor)); err != nil {
			return customerportalapp.MallProduct{}, err
		}
		return r.mallProductByID(ctx, cmd.ID)
	}

	var id int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mall_products(product_id, title, subtitle, description, image_url, spec_g, unit_price, template_key, status, sort_order, updated_at, updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(),$11)
		RETURNING id
	`, r.schema), cmd.ProductID, cmd.Title, cmd.Subtitle, cmd.Description, cmd.ImageURL, cmd.SpecG, cmd.UnitPrice, customerportalapp.NormalizeMallTemplateKey(cmd.TemplateKey), customerportalapp.NormalizeMallProductStatus(cmd.Status), cmd.SortOrder, strings.TrimSpace(cmd.Actor)).Scan(&id); err != nil {
		return customerportalapp.MallProduct{}, err
	}
	return r.mallProductByID(ctx, id)
}

func (r Repository) ensureMallProductPublicCatalog(ctx context.Context, productID int64) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.products
			WHERE id=$1 AND active=true
			  AND %s
		)
	`, r.schema, mallProductPublicCatalogSQL("")), productID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("mall product unavailable")
	}
	return nil
}

func (r Repository) UpdateMallProductImage(ctx context.Context, cmd customerportalapp.UpdateMallProductImageCommand) (customerportalapp.MallProduct, error) {
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.mall_products
		SET image_url=$2, updated_at=now(), updated_by=$3
		WHERE id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.ImageURL), strings.TrimSpace(cmd.Actor)); err != nil {
		return customerportalapp.MallProduct{}, err
	}
	return r.mallProductByID(ctx, cmd.ID)
}

func (r Repository) mallProductByID(ctx context.Context, id int64) (customerportalapp.MallProduct, error) {
	var row customerportalapp.MallProduct
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(p.product_kind,''),'roasted'), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
		       m.image_url, m.spec_g, m.unit_price, m.template_key, m.status, m.sort_order,
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE m.id=$1
	`, r.schema, r.schema), id).Scan(&row.ID, &row.ProductID, &row.ProductName, &row.ProductKind, &row.Title, &row.Subtitle, &row.Description, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt)
	if err != nil {
		return customerportalapp.MallProduct{}, err
	}
	row.TemplateKey = customerportalapp.NormalizeMallTemplateKey(row.TemplateKey)
	row.Status = customerportalapp.NormalizeMallProductStatus(row.Status)
	return row, nil
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
