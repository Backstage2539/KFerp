package customer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	customerapp "orderapp/internal/application/customer"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool     *pgxpool.Pool
	schema   string
	assetDir string
}

type upsertRequest struct {
	Name                  string
	RawName               string
	CustomerType          string
	CompanyName           string
	CompanyAddress        string
	CompanyPhone          string
	Contact               string
	Phone                 string
	Address               string
	DefaultSourceID       string
	DefaultOrderTypeID    string
	ResponsibleEmployeeID string
	Active                string
	PortalEnabled         *bool
	CapabilityTemplateKey string
}

type inlineRequest = upsertRequest

type prefs struct {
	ID              int64
	DefaultSourceID *int
	SourceName      *string
	DefaultTypeID   *int
	TypeName        *string
	Address         *string
}

func NewRepository(pool *pgxpool.Pool, schema, assetDir string) Repository {
	return Repository{pool: pool, schema: schema, assetDir: assetDir}
}

func (r Repository) Upsert(ctx context.Context, actor string, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	return upsertCustomer(ctx, r.pool, r.schema, actor, id, upsertRequestFromCommand(cmd), nil)
}

func upsertRequestFromCommand(cmd customerapp.UpsertCommand) upsertRequest {
	return upsertRequest{
		Name:                  cmd.Name,
		RawName:               cmd.RawName,
		CustomerType:          cmd.CustomerType,
		CompanyName:           cmd.CompanyName,
		CompanyAddress:        cmd.CompanyAddress,
		CompanyPhone:          cmd.CompanyPhone,
		Contact:               cmd.Contact,
		Phone:                 cmd.Phone,
		Address:               cmd.Address,
		DefaultSourceID:       cmd.DefaultSourceID,
		DefaultOrderTypeID:    cmd.DefaultOrderTypeID,
		ResponsibleEmployeeID: cmd.ResponsibleEmployeeID,
		Active:                cmd.Active,
		PortalEnabled:         cmd.PortalEnabled,
		CapabilityTemplateKey: cmd.CapabilityTemplateKey,
	}
}

func (r Repository) UpsertManaged(ctx context.Context, principal customerapp.MaintenancePrincipal, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	return upsertCustomer(ctx, r.pool, r.schema, miniEmployeeCustomerActor(principal), id, upsertRequestFromCommand(cmd), &principal)
}

func (r Repository) Prefs(ctx context.Context, id int64) (*customerapp.Prefs, error) {
	p, err := fetchCustomerPrefs(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	return &customerapp.Prefs{
		ID:              p.ID,
		DefaultSourceID: p.DefaultSourceID,
		SourceName:      p.SourceName,
		DefaultTypeID:   p.DefaultTypeID,
		TypeName:        p.TypeName,
		Address:         p.Address,
	}, nil
}

func (r Repository) SaveAsset(ctx context.Context, cmd customerapp.SaveAssetCommand) (customerapp.SaveAssetResult, error) {
	obj, size, sha, err := saveAssetFile(r.assetDir, cmd.CustomerID, cmd.Kind, cmd.Reader, cmd.ContentType, cmd.MaxBytes, cmd.Filename)
	if err != nil {
		return customerapp.SaveAssetResult{}, err
	}
	if _, err := insertCustomerAsset(ctx, r.pool, r.schema, cmd.Actor, cmd.CustomerID, cmd.Kind, obj, cmd.ContentType, size, sha); err != nil {
		cleanupCustomerAssetFile(r.assetDir, obj)
		return customerapp.SaveAssetResult{}, err
	}
	return customerapp.SaveAssetResult{CustomerID: cmd.CustomerID, ObjectKey: obj, Bytes: size, SHA256: sha}, nil
}

func (r Repository) DeleteAsset(ctx context.Context, actor string, assetID int64) (customerapp.DeleteAssetResult, error) {
	cid, _, obj, err := deleteCustomerAssetByID(ctx, r.pool, r.schema, actor, assetID)
	if err != nil {
		return customerapp.DeleteAssetResult{}, err
	}
	if obj != "" {
		_ = os.Remove(filepath.Join(r.assetDir, obj))
	}
	return customerapp.DeleteAssetResult{CustomerID: cid, ObjectKey: obj}, nil
}

func (r Repository) InlineUpdate(ctx context.Context, actor string, id int64, cmd customerapp.InlineUpdateCommand) error {
	req := inlineRequest{
		Name:                  cmd.Name,
		CustomerType:          cmd.CustomerType,
		CompanyName:           cmd.CompanyName,
		CompanyAddress:        cmd.CompanyAddress,
		CompanyPhone:          cmd.CompanyPhone,
		Contact:               cmd.Contact,
		Phone:                 cmd.Phone,
		Address:               cmd.Address,
		DefaultSourceID:       cmd.DefaultSourceID,
		DefaultOrderTypeID:    cmd.DefaultOrderTypeID,
		ResponsibleEmployeeID: cmd.ResponsibleEmployeeID,
		Active:                cmd.Active,
	}
	return inlineUpdateCustomer(ctx, r.pool, r.schema, actor, id, req)
}

func (r Repository) Delete(ctx context.Context, actor string, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var oldActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.customers WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&oldActive); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	q := fmt.Sprintf(`UPDATE %s.customers SET active=false, updated_at=$2 WHERE id=$1`, r.schema)
	if _, err := tx.Exec(ctx, q, id, time.Now()); err != nil {
		return err
	}
	if oldActive {
		if err := postgresinfra.ExpireCustomerSecuritySessions(ctx, tx, r.schema, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "customer", &id, "delete", nil, nil, nil, nil)
	return nil
}

func (r Repository) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	rows, total, hasNext, err := fetchCustomers(ctx, r.pool, r.schema, query)
	if err != nil {
		return customerapp.ListResult{}, err
	}
	sources, err := fetchOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", r.schema))
	if err != nil {
		return customerapp.ListResult{}, err
	}
	orderTypes, err := fetchOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", r.schema))
	if err != nil {
		return customerapp.ListResult{}, err
	}
	employees, err := fetchCustomerResponsibleEmployees(ctx, r.pool, r.schema)
	if err != nil {
		return customerapp.ListResult{}, err
	}
	customerTypes, err := r.ListCustomerTypeOptions(ctx)
	if err != nil {
		return customerapp.ListResult{}, err
	}
	return customerapp.ListResult{Rows: rows, Sources: sources, OrderTypes: orderTypes, Employees: employees, CustomerTypeOptions: customerTypes, Total: total, HasNext: hasNext}, nil
}

func (r Repository) ListManaged(ctx context.Context, principal customerapp.MaintenancePrincipal, query customerapp.ListQuery) (customerapp.ListResult, error) {
	if !principal.IsAdmin {
		query.ResponsibleEmployeeID = principal.EmployeeID
	}
	return r.List(ctx, query)
}

func (r Repository) Editor(ctx context.Context, id int64) (*customerapp.EditorData, error) {
	return r.editor(ctx, id, nil)
}

func (r Repository) EditorManaged(ctx context.Context, principal customerapp.MaintenancePrincipal, id int64) (*customerapp.EditorData, error) {
	if principal.IsAdmin {
		data, err := r.editor(ctx, id, nil)
		if err == nil && data == nil {
			return nil, customerapp.ErrCustomerNotFound
		}
		return data, err
	}
	data, err := r.editor(ctx, id, &principal.EmployeeID)
	if err != nil || data != nil {
		return data, err
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1)`, r.schema), id).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, customerapp.ErrCustomerMaintenanceForbidden
	}
	return nil, customerapp.ErrCustomerNotFound
}

func (r Repository) editor(ctx context.Context, id int64, responsibleEmployeeID *int64) (*customerapp.EditorData, error) {
	customer, err := fetchCustomerByID(ctx, r.pool, r.schema, id, responsibleEmployeeID)
	if err != nil || customer == nil {
		return nil, err
	}
	sources, err := fetchOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", r.schema))
	if err != nil {
		return nil, err
	}
	orderTypes, err := fetchOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", r.schema))
	if err != nil {
		return nil, err
	}
	employees, err := fetchCustomerResponsibleEmployees(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	customerTypes, err := r.ListCustomerTypeOptions(ctx)
	if err != nil {
		return nil, err
	}
	assets, err := fetchCustomerAssets(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	dashboard, err := fetchCustomerDashboard(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	return &customerapp.EditorData{
		Customer:            *customer,
		Sources:             sources,
		OrderTypes:          orderTypes,
		Employees:           employees,
		CustomerTypeOptions: customerTypes,
		Assets:              assets,
		Dashboard:           dashboard,
	}, nil
}

func miniEmployeeCustomerActor(principal customerapp.MaintenancePrincipal) string {
	name := strings.TrimSpace(principal.EmployeeName)
	if name == "" {
		return fmt.Sprintf("mini-employee:%d", principal.EmployeeID)
	}
	return fmt.Sprintf("mini-employee:%d:%s", principal.EmployeeID, name)
}

func (r Repository) AssetObject(ctx context.Context, assetID int64) (customerapp.AssetObject, error) {
	var obj string
	var contentType string
	q := fmt.Sprintf(`SELECT object_key, content_type FROM %s.customer_assets WHERE id=$1`, r.schema)
	if err := r.pool.QueryRow(ctx, q, assetID).Scan(&obj, &contentType); err != nil {
		return customerapp.AssetObject{}, err
	}
	return customerapp.AssetObject{ObjectKey: obj, ContentType: contentType}, nil
}

func (r Repository) ListCustomerTypeOptions(ctx context.Context) ([]customerapp.CustomerTypeOption, error) {
	if err := ensureCustomerTypeOptionTable(ctx, r.pool, r.schema); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT value,label
		FROM %s.customer_type_options
		WHERE active=true
		ORDER BY sort_order, label, value
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	merged := mergeCustomerTypeOptions(customerapp.DefaultCustomerTypeOptions(), nil)
	for rows.Next() {
		var option customerapp.CustomerTypeOption
		if err := rows.Scan(&option.Value, &option.Label); err != nil {
			return nil, err
		}
		merged = mergeCustomerTypeOptions(merged, []customerapp.CustomerTypeOption{option})
	}
	return merged, rows.Err()
}

func (r Repository) CreateCustomerTypeOption(ctx context.Context, actor string, cmd customerapp.CreateCustomerTypeCommand) (customerapp.CustomerTypeOption, error) {
	if err := ensureCustomerTypeOptionTable(ctx, r.pool, r.schema); err != nil {
		return customerapp.CustomerTypeOption{}, err
	}
	label := strings.TrimSpace(cmd.Label)
	if label == "" {
		return customerapp.CustomerTypeOption{}, fmt.Errorf("label required")
	}
	value := sanitizeCustomerTypeValue(cmd.Value)
	if value == "" {
		value = sanitizeCustomerTypeValue(label)
	}
	if value == "" {
		value = fmt.Sprintf("custom_%d", time.Now().UnixNano())
	}
	var existing customerapp.CustomerTypeOption
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT value,label
		FROM %s.customer_type_options
		WHERE active=true AND (value=$1 OR label=$2)
		ORDER BY CASE WHEN value=$1 THEN 0 ELSE 1 END
		LIMIT 1
	`, r.schema), value, label).Scan(&existing.Value, &existing.Label)
	if err == nil {
		return existing, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return customerapp.CustomerTypeOption{}, err
	}
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_type_options(value,label,active,sort_order,created_at,created_by)
		VALUES($1,$2,true,100,now(),$3)
		ON CONFLICT(value) DO UPDATE SET label=excluded.label, active=true
	`, r.schema), value, label, strings.TrimSpace(actor)); err != nil {
		return customerapp.CustomerTypeOption{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "customer_type_option", nil, "create", postgresinfra.StrPtr("label"), nil, postgresinfra.StrPtr(label), postgresinfra.AuditMeta{"value": value})
	return customerapp.CustomerTypeOption{Value: value, Label: label}, nil
}

func (r Repository) CreateOrderTypeOption(ctx context.Context, actor string, cmd customerapp.CreateOrderTypeCommand) (customerapp.Option, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return customerapp.Option{}, fmt.Errorf("name required")
	}
	var option customerapp.Option
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,name FROM %s.order_types WHERE name=$1 LIMIT 1`, r.schema), name).Scan(&option.ID, &option.Name)
	if err == nil {
		return option, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return customerapp.Option{}, err
	}
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_types(name) VALUES($1) RETURNING id,name`, r.schema), name).Scan(&option.ID, &option.Name); err != nil {
		return customerapp.Option{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order_type", &option.ID, "create", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(name), nil)
	return option, nil
}

func fetchCustomers(ctx context.Context, pool *pgxpool.Pool, schema string, query customerapp.ListQuery) (rows []customerapp.CustomerRow, total int, hasNext bool, err error) {
	q := strings.TrimSpace(query.Query)
	limit := query.Limit
	offset := query.Offset
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := make([]any, 0)
	where := ""
	if q != "" {
		where = "WHERE (c.name ILIKE $1 OR COALESCE(c.company_name,'') ILIKE $1 OR COALESCE(c.company_phone,'') ILIKE $1 OR COALESCE(c.company_address,'') ILIKE $1 OR COALESCE(c.contact,'') ILIKE $1 OR COALESCE(c.phone,'') ILIKE $1 OR COALESCE(c.address,'') ILIKE $1)"
		args = append(args, "%"+strings.TrimSpace(q)+"%")
	}
	if t := strings.TrimSpace(query.CustomerType); t != "" {
		if len(args) > 0 {
			where += " AND c.customer_type = $" + strconv.Itoa(len(args)+1)
		} else {
			where = "WHERE c.customer_type = $1"
		}
		args = append(args, t)
	}
	if query.Active != nil {
		if len(args) > 0 {
			where += " AND c.active = $" + strconv.Itoa(len(args)+1)
		} else {
			where = "WHERE c.active = $1"
		}
		args = append(args, *query.Active)
	}
	if query.ResponsibleEmployeeID > 0 {
		if len(args) > 0 {
			where += " AND c.responsible_employee_id = $" + strconv.Itoa(len(args)+1)
		} else {
			where = "WHERE c.responsible_employee_id = $1"
		}
		args = append(args, query.ResponsibleEmployeeID)
	}
	countArgs := append([]any(nil), args...)
	countSQL := fmt.Sprintf(`SELECT count(*)::int FROM %s.customers c %s`, schema, where)
	if err := pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	args = append(args, limit+1, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	orderBy := "c.name"
	if query.SortBy == "updated" {
		orderBy = "c.updated_at"
	}
	orderDir := "ASC"
	if strings.EqualFold(query.SortDirection, "desc") {
		orderDir = "DESC"
	}

	portalProfiles := relationExists(ctx, pool, fmt.Sprintf("%s.customer_portal_profiles", schema))
	portalSelect := "false, ''"
	portalJoin := ""
	if portalProfiles {
		portalSelect = "COALESCE(p.enabled,false), COALESCE(p.capability_template_key,'')"
		portalJoin = fmt.Sprintf("LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id", schema)
	}
	sql := fmt.Sprintf(`
		SELECT c.id, c.name, COALESCE(NULLIF(c.customer_type,''),'retail'), COALESCE(c.company_name,''), COALESCE(c.company_address,''), COALESCE(c.company_phone,''), c.contact, c.phone, c.address, c.active, c.default_source_id, c.default_order_type_id,
			NULLIF(COALESCE(c.responsible_employee_id,0),0)::int, COALESCE(e.name,''),
			%s,
			to_char(c.updated_at,'YYYY-MM-DD HH24:MI') AS updated
		FROM %s.customers c
		LEFT JOIN %s.company_employees e ON e.id=c.responsible_employee_id
		%s
		%s
            ORDER BY %s %s, c.id %s
            LIMIT $%d OFFSET $%d
        `, portalSelect, schema, schema, portalJoin, where, orderBy, orderDir, orderDir, limitArg, offsetArg)

	dbRows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer dbRows.Close()

	out := make([]customerapp.CustomerRow, 0)
	for dbRows.Next() {
		var r customerapp.CustomerRow
		if err := dbRows.Scan(&r.ID, &r.Name, &r.CustomerType, &r.CompanyName, &r.CompanyAddress, &r.CompanyPhone, &r.Contact, &r.Phone, &r.Address, &r.Active, &r.DefaultSourceID, &r.DefaultOrderTypeID, &r.ResponsibleEmployeeID, &r.ResponsibleEmployeeName, &r.PortalEnabled, &r.CapabilityTemplateKey, &r.Updated); err != nil {
			return nil, 0, false, err
		}
		r.CustomerType = customerapp.NormalizeCustomerType(r.CustomerType)
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, 0, false, err
	}

	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, total, hasNext, nil
}

func fetchCustomerByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64, responsibleEmployeeID *int64) (*customerapp.CustomerEditData, error) {
	portalProfiles := relationExists(ctx, pool, fmt.Sprintf("%s.customer_portal_profiles", schema))
	portalSelect := "false, ''"
	portalJoin := ""
	if portalProfiles {
		portalSelect = "COALESCE(p.enabled,false), COALESCE(p.capability_template_key,'')"
		portalJoin = fmt.Sprintf("LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id", schema)
	}
	where := "WHERE c.id=$1"
	args := []any{id}
	if responsibleEmployeeID != nil {
		where += " AND c.responsible_employee_id=$2"
		args = append(args, *responsibleEmployeeID)
	}
	q := fmt.Sprintf(`SELECT c.id, c.name, COALESCE(c.raw_name,''), COALESCE(NULLIF(c.customer_type,''),'retail'), COALESCE(c.company_name,''), COALESCE(c.company_address,''), COALESCE(c.company_phone,''), COALESCE(c.contact,''), COALESCE(c.phone,''), COALESCE(c.address,''),
		COALESCE(c.default_source_id::text,''), COALESCE(c.default_order_type_id::text,''), COALESCE(NULLIF(c.responsible_employee_id,0)::text,''), COALESCE(e.name,''), %s, c.active
		FROM %s.customers c
		LEFT JOIN %s.company_employees e ON e.id=c.responsible_employee_id
		%s
		%s`, portalSelect, schema, schema, portalJoin, where)
	var d customerapp.CustomerEditData
	err := pool.QueryRow(ctx, q, args...).Scan(&d.ID, &d.Name, &d.RawName, &d.CustomerType, &d.CompanyName, &d.CompanyAddress, &d.CompanyPhone, &d.Contact, &d.Phone, &d.Address, &d.DefaultSourceID, &d.DefaultOrderTypeID, &d.ResponsibleEmployeeID, &d.ResponsibleEmployeeName, &d.PortalEnabled, &d.CapabilityTemplateKey, &d.Active)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	d.CustomerType = customerapp.NormalizeCustomerType(d.CustomerType)
	return &d, nil
}

func fetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]customerapp.Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]customerapp.Option, 0)
	for rows.Next() {
		var o customerapp.Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func fetchCustomerResponsibleEmployees(ctx context.Context, pool *pgxpool.Pool, schema string) ([]customerapp.Option, error) {
	q := fmt.Sprintf(`
		SELECT id, name
		FROM %s.company_employees
		WHERE active=true AND (account_type='internal_employee' OR COALESCE(account_type,'')='')
		ORDER BY id DESC
	`, schema)
	return fetchOptions(ctx, pool, q)
}

func ensureCustomerTypeOptionTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.customer_type_options (
			value TEXT PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			sort_order INTEGER NOT NULL DEFAULT 100,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT ''
		)
	`, schema)); err != nil {
		return err
	}
	for index, option := range customerapp.DefaultCustomerTypeOptions() {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_type_options(value,label,active,sort_order)
			VALUES($1,$2,true,$3)
			ON CONFLICT(value) DO UPDATE SET label=excluded.label, active=true, sort_order=excluded.sort_order
		`, schema), option.Value, option.Label, (index+1)*10); err != nil {
			return err
		}
	}
	return nil
}

func mergeCustomerTypeOptions(base, extra []customerapp.CustomerTypeOption) []customerapp.CustomerTypeOption {
	out := make([]customerapp.CustomerTypeOption, 0, len(base)+len(extra))
	seen := map[string]bool{}
	for _, option := range append(base, extra...) {
		option.Value = strings.TrimSpace(option.Value)
		option.Label = strings.TrimSpace(option.Label)
		if option.Value == "" || option.Label == "" || seen[option.Value] {
			continue
		}
		seen[option.Value] = true
		out = append(out, option)
	}
	return out
}

func sanitizeCustomerTypeValue(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if r == '_' || r == '-' || unicode.IsSpace(r) {
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func fetchCustomerAssets(ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64) ([]customerapp.CustomerAsset, error) {
	q := fmt.Sprintf(`
		SELECT id, customer_id, kind, object_key, content_type, bytes, sha256,
			to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_assets
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
	`, schema)
	rows, err := pool.Query(ctx, q, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]customerapp.CustomerAsset, 0)
	for rows.Next() {
		var a customerapp.CustomerAsset
		if err := rows.Scan(&a.ID, &a.CustomerID, &a.Kind, &a.ObjectKey, &a.ContentType, &a.Bytes, &a.Sha256, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func fetchCustomerDashboard(ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64) (customerapp.CustomerDashboard, error) {
	var dashboard customerapp.CustomerDashboard
	var productionStatusID int64
	var shippingStatusID int64
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1),0)`, schema)).Scan(&productionStatusID)
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT id FROM %s.order_process_statuses WHERE name='发货中' LIMIT 1),0)`, schema)).Scan(&shippingStatusID)

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0) <> 2 THEN 1 ELSE 0 END),0) AS unpaid,
			COALESCE(SUM(CASE WHEN COALESCE(o.ship_status_id,0) IN (0,1,2) THEN 1 ELSE 0 END),0) AS unshipped,
			COALESCE(SUM(CASE WHEN $2>0 AND COALESCE(o.process_status_id,0) = $2 THEN 1 ELSE 0 END),0) AS in_prod,
			COALESCE(SUM(CASE WHEN $3>0 AND COALESCE(o.process_status_id,0) = $3 THEN 1 ELSE 0 END),0) AS in_ship,
			COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4) THEN 1 ELSE 0 END),0) AS completed
		FROM %s.orders o
		WHERE o.customer_id=$1 AND COALESCE(o.is_void,false)=false
	`, schema)

	if err := pool.QueryRow(ctx, q, customerID, productionStatusID, shippingStatusID).Scan(
		&dashboard.TotalOrders,
		&dashboard.UnpaidOrders,
		&dashboard.UnshippedOrders,
		&dashboard.InProduction,
		&dashboard.InShipping,
		&dashboard.Completed,
	); err != nil {
		return customerapp.CustomerDashboard{}, err
	}
	return dashboard, nil
}

func upsertCustomer(ctx context.Context, pool *pgxpool.Pool, schema string, actor string, id *int64, req upsertRequest, principal *customerapp.MaintenancePrincipal) (int64, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return 0, customerapp.NewMaintenanceValidationError("请填写客户名称")
	}
	raw := strings.TrimSpace(req.RawName)
	customerType := customerapp.NormalizeCustomerType(req.CustomerType)
	companyName := strings.TrimSpace(req.CompanyName)
	companyAddress := strings.TrimSpace(req.CompanyAddress)
	companyPhone := strings.TrimSpace(req.CompanyPhone)
	contact := strings.TrimSpace(req.Contact)
	phone := strings.TrimSpace(req.Phone)
	address := strings.TrimSpace(req.Address)
	active := strings.TrimSpace(req.Active) != ""
	ds := parseOptionalInt(req.DefaultSourceID)
	dt := parseOptionalInt(req.DefaultOrderTypeID)
	if raw == "" {
		raw = name
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if principal != nil {
		if err := validateCustomerProfileOptionsTx(ctx, tx, schema, ds, dt); err != nil {
			return 0, err
		}
	}

	var newID int64
	if id == nil {
		if principal != nil && !principal.IsAdmin {
			req.ResponsibleEmployeeID = strconv.FormatInt(principal.EmployeeID, 10)
			active = true
			req.PortalEnabled = nil
			req.CapabilityTemplateKey = ""
		}
		responsibleEmployeeID, err := resolveCustomerResponsibleEmployeeTx(ctx, tx, schema, req.ResponsibleEmployeeID)
		if err != nil {
			return 0, err
		}
		q := fmt.Sprintf(`INSERT INTO %s.customers(name, raw_name, customer_type, company_name, company_address, company_phone, contact, phone, address, active, default_source_id, default_order_type_id, responsible_employee_id, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, now(), now()) RETURNING id`, schema)
		if err := tx.QueryRow(ctx, q, name, raw, customerType, companyName, companyAddress, companyPhone, contact, phone, address, active, ds, dt, responsibleEmployeeID).Scan(&newID); err != nil {
			return 0, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &newID, "create", nil, nil, nil, postgresinfra.AuditMeta{"name": name}); err != nil {
			return 0, err
		}
	} else {
		newID = *id
		var oldName, oldRaw, oldCustomerType, oldCompanyName, oldCompanyAddress, oldCompanyPhone, oldContact, oldPhone, oldAddr string
		var oldActive bool
		var oldDS, oldDT, oldResponsibleEmployeeID *int
		q0 := fmt.Sprintf(`SELECT name, COALESCE(raw_name,''), COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id, NULLIF(COALESCE(responsible_employee_id,0),0)::int FROM %s.customers WHERE id=$1 FOR UPDATE`, schema)
		if err := tx.QueryRow(ctx, q0, newID).Scan(&oldName, &oldRaw, &oldCustomerType, &oldCompanyName, &oldCompanyAddress, &oldCompanyPhone, &oldContact, &oldPhone, &oldAddr, &oldActive, &oldDS, &oldDT, &oldResponsibleEmployeeID); err != nil {
			if err == pgx.ErrNoRows && principal != nil {
				return 0, customerapp.ErrCustomerNotFound
			}
			return 0, err
		}
		if principal != nil && !principal.IsAdmin {
			if oldResponsibleEmployeeID == nil || int64(*oldResponsibleEmployeeID) != principal.EmployeeID {
				return 0, customerapp.ErrCustomerMaintenanceForbidden
			}
			active = oldActive
			req.PortalEnabled = nil
			req.CapabilityTemplateKey = ""
		}
		var responsibleEmployeeID *int
		if principal != nil && !principal.IsAdmin {
			responsibleEmployeeID = oldResponsibleEmployeeID
		} else {
			responsibleEmployeeID, err = resolveCustomerResponsibleEmployeeTx(ctx, tx, schema, req.ResponsibleEmployeeID)
			if err != nil {
				return 0, err
			}
		}
		q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, raw_name=$3, customer_type=$4, company_name=$5, company_address=$6, company_phone=$7, contact=$8, phone=$9, address=$10, active=$11,
			default_source_id=$12, default_order_type_id=$13, responsible_employee_id=$14, updated_at=$15 WHERE id=$1`, schema)
		if _, err := tx.Exec(ctx, q, newID, name, raw, customerType, companyName, companyAddress, companyPhone, contact, phone, address, active, ds, dt, responsibleEmployeeID, time.Now()); err != nil {
			return 0, err
		}
		if oldActive != active {
			if err := postgresinfra.ExpireCustomerSecuritySessions(ctx, tx, schema, newID); err != nil {
				return 0, err
			}
		}
		if err := auditCustomerDiffs(ctx, tx, schema, actor, newID, customerSnapshot{oldName, customerapp.NormalizeCustomerType(oldCustomerType), oldCompanyName, oldCompanyAddress, oldCompanyPhone, oldContact, oldPhone, oldAddr, oldActive, oldDS, oldDT, oldResponsibleEmployeeID}, customerSnapshot{name, customerType, companyName, companyAddress, companyPhone, contact, phone, address, active, ds, dt, responsibleEmployeeID}); err != nil {
			return 0, err
		}
	}
	if err := syncCustomerPortalProfileTx(ctx, tx, schema, actor, newID, name, req.PortalEnabled, req.CapabilityTemplateKey); err != nil {
		return 0, err
	}
	if err := ensureDefaultRetailPortalTx(ctx, tx, schema, newID, name, actor, customerType, active); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newID, nil
}

func resolveCustomerResponsibleEmployeeTx(ctx context.Context, tx pgx.Tx, schema, raw string) (*int, error) {
	employeeID := parseOptionalInt(raw)
	if employeeID == nil {
		return nil, customerapp.NewMaintenanceValidationError("请选择负责人")
	}
	var activeID int
	q := fmt.Sprintf(`
		SELECT id
		FROM %s.company_employees
		WHERE id=$1 AND active=true AND (account_type='internal_employee' OR COALESCE(account_type,'')='')
	`, schema)
	if err := tx.QueryRow(ctx, q, *employeeID).Scan(&activeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, customerapp.NewMaintenanceValidationError("负责人不存在或已停用，请重新选择")
		}
		return nil, err
	}
	return &activeID, nil
}

func validateCustomerProfileOptionsTx(ctx context.Context, tx pgx.Tx, schema string, sourceID, orderTypeID *int) error {
	if sourceID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sources WHERE id=$1)`, schema), *sourceID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return customerapp.NewMaintenanceValidationError("来源不存在，请重新选择")
		}
	}
	if orderTypeID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_types WHERE id=$1)`, schema), *orderTypeID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return customerapp.NewMaintenanceValidationError("订单类型不存在，请重新选择")
		}
	}
	return nil
}

func inlineUpdateCustomer(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64, req inlineRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("name required")
	}
	next := customerSnapshot{
		name:           name,
		customerType:   customerapp.NormalizeCustomerType(req.CustomerType),
		companyName:    strings.TrimSpace(req.CompanyName),
		companyAddress: strings.TrimSpace(req.CompanyAddress),
		companyPhone:   strings.TrimSpace(req.CompanyPhone),
		contact:        strings.TrimSpace(req.Contact),
		phone:          strings.TrimSpace(req.Phone),
		address:        strings.TrimSpace(req.Address),
		active:         strings.TrimSpace(req.Active) != "",
		sourceID:       parseOptionalInt(req.DefaultSourceID),
		typeID:         parseOptionalInt(req.DefaultOrderTypeID),
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var old customerSnapshot
	q0 := fmt.Sprintf(`SELECT name, COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id, NULLIF(COALESCE(responsible_employee_id,0),0)::int
		FROM %s.customers WHERE id=$1`, schema)
	if err := tx.QueryRow(ctx, q0, id).Scan(&old.name, &old.customerType, &old.companyName, &old.companyAddress, &old.companyPhone, &old.contact, &old.phone, &old.address, &old.active, &old.sourceID, &old.typeID, &old.responsibleEmployeeID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("not found")
		}
		return err
	}
	old.customerType = customerapp.NormalizeCustomerType(old.customerType)
	if strings.TrimSpace(req.ResponsibleEmployeeID) != "" {
		responsibleEmployeeID, err := resolveCustomerResponsibleEmployeeTx(ctx, tx, schema, req.ResponsibleEmployeeID)
		if err != nil {
			return err
		}
		next.responsibleEmployeeID = responsibleEmployeeID
	} else {
		if old.responsibleEmployeeID == nil {
			return fmt.Errorf("responsible_employee_id required")
		}
		next.responsibleEmployeeID = old.responsibleEmployeeID
	}
	q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, customer_type=$3, company_name=$4, company_address=$5, company_phone=$6, contact=$7, phone=$8, address=$9, active=$10,
		default_source_id=$11, default_order_type_id=$12, responsible_employee_id=$13, updated_at=$14 WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.name, next.customerType, next.companyName, next.companyAddress, next.companyPhone, next.contact, next.phone, next.address, next.active, next.sourceID, next.typeID, next.responsibleEmployeeID, time.Now()); err != nil {
		return err
	}
	if old.active != next.active {
		if err := postgresinfra.ExpireCustomerSecuritySessions(ctx, tx, schema, id); err != nil {
			return err
		}
	}
	if err := auditCustomerDiffs(ctx, tx, schema, actor, id, old, next); err != nil {
		return err
	}
	if err := syncCustomerPortalProfileTx(ctx, tx, schema, actor, id, next.name, req.PortalEnabled, req.CapabilityTemplateKey); err != nil {
		return err
	}
	if err := ensureDefaultRetailPortalTx(ctx, tx, schema, id, next.name, actor, next.customerType, next.active); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ensureDefaultRetailPortalTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, _ string, actor, customerType string, active bool) error {
	return nil
}

func syncCustomerPortalProfileTx(ctx context.Context, tx pgx.Tx, schema, actor string, customerID int64, customerName string, portalEnabled *bool, templateKey string) error {
	templateKey = strings.TrimSpace(templateKey)
	if portalEnabled == nil && templateKey == "" {
		return nil
	}
	var hasProfiles bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_portal_profiles", schema)).Scan(&hasProfiles); err != nil {
		return err
	}
	if !hasProfiles {
		return nil
	}

	oldEnabled := false
	oldTemplateKey := ""
	hadProfile := true
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(enabled,false), COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, schema), customerID).Scan(&oldEnabled, &oldTemplateKey)
	if err == pgx.ErrNoRows {
		hadProfile = false
		err = nil
	}
	if err != nil {
		return err
	}

	nextEnabled := oldEnabled
	if portalEnabled != nil {
		nextEnabled = *portalEnabled
	}
	nextTemplateKey := templateKey
	if nextTemplateKey == "" {
		nextTemplateKey = oldTemplateKey
	}
	if !hadProfile && !nextEnabled && nextTemplateKey == "" {
		return nil
	}

	displayName := strings.TrimSpace(customerName)
	updatedBy := strings.TrimSpace(actor)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, enabled, status, capability_template_key, updated_at, updated_by)
		VALUES($1,$2,$3,'active',$4,now(),$5)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=COALESCE(NULLIF(%s.customer_portal_profiles.display_name,''), excluded.display_name),
			enabled=excluded.enabled,
			status='active',
			capability_template_key=excluded.capability_template_key,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, schema, schema), customerID, displayName, nextEnabled, nextTemplateKey, updatedBy); err != nil {
		return err
	}
	if hadProfile && oldEnabled != nextEnabled {
		if err := postgresinfra.ExpireCustomerSecuritySessions(ctx, tx, schema, customerID); err != nil {
			return err
		}
	}
	if hadProfile && oldTemplateKey != nextTemplateKey {
		if err := postgresinfra.ExpireCustomerERPSessions(ctx, tx, schema, customerID); err != nil {
			return err
		}
	}
	if portalEnabled != nil && oldEnabled != nextEnabled {
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &customerID, "update", postgresinfra.StrPtr("portal_enabled"), postgresinfra.StrPtr(fmt.Sprintf("%v", oldEnabled)), postgresinfra.StrPtr(fmt.Sprintf("%v", nextEnabled)), nil); err != nil {
			return err
		}
	}
	if oldTemplateKey != nextTemplateKey {
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &customerID, "update", postgresinfra.StrPtr("capability_template_key"), postgresinfra.StrPtr(oldTemplateKey), postgresinfra.StrPtr(nextTemplateKey), nil); err != nil {
			return err
		}
	}
	return nil
}

func deactivateRetailERPWorkbenchBindingsTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, actor string) error {
	var hasBindings bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`,
		fmt.Sprintf("%s.customer_erp_user_bindings", schema),
	).Scan(&hasBindings); err != nil {
		return err
	}
	if !hasBindings {
		return nil
	}
	updatedBy := strings.TrimSpace(actor)
	if updatedBy == "" {
		updatedBy = "system:retail-template"
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_erp_user_bindings
		SET status='inactive',
			updated_at=now(),
			updated_by=$2
		WHERE customer_id=$1 AND status='active'
	`, schema), customerID, updatedBy)
	return err
}

func fetchCustomerPrefs(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*prefs, error) {
	q := fmt.Sprintf(`
		SELECT c.id, c.default_source_id, s.name, c.default_order_type_id, t.name, c.address
		FROM %s.customers c
		LEFT JOIN %s.sources s ON s.id = c.default_source_id
		LEFT JOIN %s.order_types t ON t.id = c.default_order_type_id
		WHERE c.id=$1
	`, schema, schema, schema)
	var p prefs
	if err := pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.DefaultSourceID, &p.SourceName, &p.DefaultTypeID, &p.TypeName, &p.Address); err != nil {
		return nil, err
	}
	return &p, nil
}

func insertCustomerAsset(ctx context.Context, pool *pgxpool.Pool, schema, actor string, customerID int64, kind string, objectKey string, contentType string, bytes int64, sha string) (int64, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return 0, fmt.Errorf("kind required")
	}
	q := fmt.Sprintf(`
		INSERT INTO %s.customer_assets(customer_id, kind, object_key, content_type, bytes, sha256, created_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6, now(), $7)
		RETURNING id
	`, schema)
	var id int64
	if err := pool.QueryRow(ctx, q, customerID, kind, objectKey, contentType, bytes, sha, actor).Scan(&id); err != nil {
		return 0, err
	}
	postgresinfra.AuditInsert(ctx, pool, schema, actor, "customer_asset", &customerID, "upload", postgresinfra.StrPtr("kind"), nil, postgresinfra.StrPtr(kind), postgresinfra.AuditMeta{"asset_id": id, "object_key": objectKey, "bytes": bytes, "content_type": contentType})
	return id, nil
}

func deleteCustomerAssetByID(ctx context.Context, pool *pgxpool.Pool, schema, actor string, assetID int64) (customerID int64, kind string, objectKey string, err error) {
	q0 := fmt.Sprintf(`SELECT customer_id, kind, object_key FROM %s.customer_assets WHERE id=$1`, schema)
	if err := pool.QueryRow(ctx, q0, assetID).Scan(&customerID, &kind, &objectKey); err != nil {
		return 0, "", "", err
	}
	q := fmt.Sprintf(`DELETE FROM %s.customer_assets WHERE id=$1`, schema)
	if _, err := pool.Exec(ctx, q, assetID); err != nil {
		return 0, "", "", err
	}
	postgresinfra.AuditInsert(ctx, pool, schema, actor, "customer_asset", &customerID, "delete", postgresinfra.StrPtr("asset_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", assetID)), nil, postgresinfra.AuditMeta{"kind": kind})
	return customerID, kind, objectKey, nil
}

func saveAssetFile(assetDir string, customerID int64, kind string, r io.Reader, contentType string, maxBytes int64, filename string) (objectKey string, size int64, sha string, err error) {
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	if !allowedImageTypes[contentType] {
		fn := strings.ToLower(strings.TrimSpace(filename))
		if strings.HasSuffix(fn, ".heic") || strings.HasSuffix(fn, ".heif") {
			return "", 0, "", fmt.Errorf("不支持 HEIC 图片：请在 iPhone 分享→选项→格式选“最兼容”(JPEG)，或先转换成 JPG/PNG 后再上传")
		}
		return "", 0, "", fmt.Errorf("不支持的图片格式（仅支持 JPG/PNG/WebP）")
	}
	ext := extByContentType(contentType)
	if ext == "" {
		return "", 0, "", fmt.Errorf("unknown file type")
	}
	base := fmt.Sprintf("customers/%d/%s/%d%s", customerID, kind, time.Now().UnixNano(), ext)
	path := filepath.Join(assetDir, base)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", 0, "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", 0, "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	mw := io.MultiWriter(f, h)
	lr := &io.LimitedReader{R: r, N: maxBytes + 1}
	n, err := io.Copy(mw, lr)
	if err != nil {
		_ = os.Remove(path)
		return "", 0, "", err
	}
	if n > maxBytes {
		_ = os.Remove(path)
		return "", 0, "", fmt.Errorf("file too large")
	}
	return base, n, hex.EncodeToString(h.Sum(nil)), nil
}

func cleanupCustomerAssetFile(assetDir string, objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return
	}
	assetDir = filepath.Clean(assetDir)
	path := filepath.Join(assetDir, objectKey)
	if err := os.Remove(path); err != nil {
		return
	}
	for dir := filepath.Dir(path); dir != "." && dir != assetDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

func extByContentType(ct string) string {
	exts, _ := mime.ExtensionsByType(ct)
	for _, ext := range exts {
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			return ext
		}
	}
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

type customerSnapshot struct {
	name                  string
	customerType          string
	companyName           string
	companyAddress        string
	companyPhone          string
	contact               string
	phone                 string
	address               string
	active                bool
	sourceID              *int
	typeID                *int
	responsibleEmployeeID *int
}

func auditCustomerDiffs(ctx context.Context, tx pgx.Tx, schema, actor string, id int64, old, next customerSnapshot) error {
	log := func(field, oldValue, newValue string) error {
		if oldValue == newValue {
			return nil
		}
		return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &id, "update", postgresinfra.StrPtr(field), postgresinfra.StrPtr(oldValue), postgresinfra.StrPtr(newValue), nil)
	}
	if err := log("name", old.name, next.name); err != nil {
		return err
	}
	if err := log("customer_type", customerapp.NormalizeCustomerType(old.customerType), customerapp.NormalizeCustomerType(next.customerType)); err != nil {
		return err
	}
	if err := log("company_name", old.companyName, next.companyName); err != nil {
		return err
	}
	if err := log("company_address", old.companyAddress, next.companyAddress); err != nil {
		return err
	}
	if err := log("company_phone", old.companyPhone, next.companyPhone); err != nil {
		return err
	}
	if err := log("contact", old.contact, next.contact); err != nil {
		return err
	}
	if err := log("phone", old.phone, next.phone); err != nil {
		return err
	}
	if err := log("address", old.address, next.address); err != nil {
		return err
	}
	if old.active != next.active {
		if err := log("active", fmt.Sprintf("%v", old.active), fmt.Sprintf("%v", next.active)); err != nil {
			return err
		}
	}
	if err := log("default_source_id", intPtrString(old.sourceID), intPtrString(next.sourceID)); err != nil {
		return err
	}
	if err := log("default_order_type_id", intPtrString(old.typeID), intPtrString(next.typeID)); err != nil {
		return err
	}
	return log("responsible_employee_id", intPtrString(old.responsibleEmployeeID), intPtrString(next.responsibleEmployeeID))
}

func parseOptionalInt(v string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func intPtrString(v *int) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func relationExists(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, relation string) bool {
	var ok bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, relation).Scan(&ok); err != nil {
		return false
	}
	return ok
}
