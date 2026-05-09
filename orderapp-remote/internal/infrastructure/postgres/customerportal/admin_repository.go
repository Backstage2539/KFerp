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
		       COALESCE(p.processing_warehouse_code,''),
		       COALESCE(p.default_sender_id,0),
		       COALESCE(p.enabled,true),
		       COALESCE(p.status,'active'),
		       COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
		       COALESCE(NULLIF(p.miniapp_entry_mode,''),'services'),
		       COALESCE(p.capability_template_key,''),
		       COUNT(b.id) FILTER (WHERE b.status='approved')::int
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		LEFT JOIN %s.customer_portal_user_bindings b ON b.customer_id=c.id
		WHERE c.active=true
		  AND ($1='' OR c.name ILIKE '%%' || $1 || '%%' OR c.phone ILIKE '%%' || $1 || '%%' OR c.company_name ILIKE '%%' || $1 || '%%')
		GROUP BY c.id, c.name, c.phone, c.company_name, p.display_name, p.processing_warehouse_code, p.default_sender_id, p.enabled, p.status, p.theme_key, p.miniapp_entry_mode, p.capability_template_key
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
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.ProcessingWarehouseCode, &row.DefaultSenderID, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.MiniappEntryMode, &row.CapabilityTemplateKey, &row.BindingCount); err != nil {
			return nil, err
		}
		row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
		row.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(row.MiniappEntryMode)
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
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, processing_warehouse_code, default_sender_id, enabled, status, theme_key, miniapp_entry_mode, capability_template_key, updated_at, updated_by)
		VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8,now(),$9)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=excluded.display_name,
			processing_warehouse_code=excluded.processing_warehouse_code,
			default_sender_id=excluded.default_sender_id,
			enabled=excluded.enabled,
			status='active',
			theme_key=excluded.theme_key,
			miniapp_entry_mode=excluded.miniapp_entry_mode,
			capability_template_key=excluded.capability_template_key,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.CustomerID, strings.TrimSpace(cmd.DisplayName), warehouseCode, cmd.DefaultSenderID, cmd.Enabled, customerportalapp.NormalizePortalThemeKey(cmd.ThemeKey), customerportalapp.NormalizeMiniappEntryMode(cmd.MiniappEntryMode), customerportalapp.NormalizeCapabilityTemplateKey(cmd.CapabilityTemplateKey), strings.TrimSpace(cmd.UpdatedBy)); err != nil {
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
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.id,
		       COALESCE(c.name,''),
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
		       COALESCE((SELECT COUNT(*)::int FROM %s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.status='approved'),0)
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1
	`, r.schema, r.schema, r.schema), customerID).Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.ProcessingWarehouseCode, &row.DefaultSenderID, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.MiniappEntryMode, &row.CapabilityTemplateKey, &row.BindingCount)
	if err == nil {
		row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
		row.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(row.MiniappEntryMode)
	}
	if err == pgx.ErrNoRows {
		return customerportalapp.PortalAdminCustomer{}, customerportalapp.ErrPortalCustomerNotFound
	}
	return row, err
}

func (r Repository) ListMallProducts(ctx context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
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
		if err := rows.Scan(&row.ID, &row.ProductID, &row.ProductName, &row.Title, &row.Subtitle, &row.Description, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt); err != nil {
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
		SELECT id, COALESCE(name,''), COALESCE(default_price,0)
		FROM %s.products
		WHERE active=true
		ORDER BY name, id
	`, r.schema))
	if err != nil {
		return nil, nil, err
	}
	defer optionRows.Close()
	// product_options for the admin API are returned alongside mall rows.
	productOptions := make([]customerportalapp.MallProductOption, 0)
	for optionRows.Next() {
		var row customerportalapp.MallProductOption
		if err := optionRows.Scan(&row.ID, &row.Name, &row.DefaultPrice); err != nil {
			return nil, nil, err
		}
		productOptions = append(productOptions, row)
	}
	return mallRows, productOptions, optionRows.Err()
}

func (r Repository) SaveMallProduct(ctx context.Context, cmd customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error) {
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
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
		       m.image_url, m.spec_g, m.unit_price, m.template_key, m.status, m.sort_order,
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE m.id=$1
	`, r.schema, r.schema), id).Scan(&row.ID, &row.ProductID, &row.ProductName, &row.Title, &row.Subtitle, &row.Description, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt)
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
