package customer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	Name               string
	RawName            string
	CustomerType       string
	CompanyName        string
	CompanyAddress     string
	CompanyPhone       string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             string
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
	req := upsertRequest{
		Name:               cmd.Name,
		RawName:            cmd.RawName,
		CustomerType:       cmd.CustomerType,
		CompanyName:        cmd.CompanyName,
		CompanyAddress:     cmd.CompanyAddress,
		CompanyPhone:       cmd.CompanyPhone,
		Contact:            cmd.Contact,
		Phone:              cmd.Phone,
		Address:            cmd.Address,
		DefaultSourceID:    cmd.DefaultSourceID,
		DefaultOrderTypeID: cmd.DefaultOrderTypeID,
		Active:             cmd.Active,
	}
	return upsertCustomer(ctx, r.pool, r.schema, actor, id, req)
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
		Name:               cmd.Name,
		CustomerType:       cmd.CustomerType,
		CompanyName:        cmd.CompanyName,
		CompanyAddress:     cmd.CompanyAddress,
		CompanyPhone:       cmd.CompanyPhone,
		Contact:            cmd.Contact,
		Phone:              cmd.Phone,
		Address:            cmd.Address,
		DefaultSourceID:    cmd.DefaultSourceID,
		DefaultOrderTypeID: cmd.DefaultOrderTypeID,
		Active:             cmd.Active,
	}
	return inlineUpdateCustomer(ctx, r.pool, r.schema, actor, id, req)
}

func (r Repository) Delete(ctx context.Context, actor string, id int64) error {
	q := fmt.Sprintf(`UPDATE %s.customers SET active=false, updated_at=$2 WHERE id=$1`, r.schema)
	if _, err := r.pool.Exec(ctx, q, id, time.Now()); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "customer", &id, "delete", nil, nil, nil, nil)
	return nil
}

func (r Repository) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	rows, hasNext, err := fetchCustomers(ctx, r.pool, r.schema, query.Query, query.Limit, query.Offset)
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
	return customerapp.ListResult{Rows: rows, Sources: sources, OrderTypes: orderTypes, HasNext: hasNext}, nil
}

func (r Repository) Editor(ctx context.Context, id int64) (*customerapp.EditorData, error) {
	customer, err := fetchCustomerByID(ctx, r.pool, r.schema, id)
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
	assets, err := fetchCustomerAssets(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	dashboard, err := fetchCustomerDashboard(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	return &customerapp.EditorData{
		Customer:   *customer,
		Sources:    sources,
		OrderTypes: orderTypes,
		Assets:     assets,
		Dashboard:  dashboard,
	}, nil
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

func fetchCustomers(ctx context.Context, pool *pgxpool.Pool, schema, q string, limit, offset int) (rows []customerapp.CustomerRow, hasNext bool, err error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := make([]any, 0)
	where := ""
	if strings.TrimSpace(q) != "" {
		where = "WHERE name ILIKE $1 OR COALESCE(company_name,'') ILIKE $1 OR COALESCE(company_phone,'') ILIKE $1 OR COALESCE(company_address,'') ILIKE $1 OR COALESCE(contact,'') ILIKE $1 OR COALESCE(phone,'') ILIKE $1 OR COALESCE(address,'') ILIKE $1"
		args = append(args, "%"+strings.TrimSpace(q)+"%")
	}
	args = append(args, limit+1, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	sql := fmt.Sprintf(`
		SELECT id, name, COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), contact, phone, address, active, default_source_id, default_order_type_id,
			to_char(updated_at,'YYYY-MM-DD HH24:MI') AS updated
		FROM %s.customers
		%s
		ORDER BY active DESC, name ASC
		LIMIT $%d OFFSET $%d
	`, schema, where, limitArg, offsetArg)

	dbRows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer dbRows.Close()

	out := make([]customerapp.CustomerRow, 0)
	for dbRows.Next() {
		var r customerapp.CustomerRow
		if err := dbRows.Scan(&r.ID, &r.Name, &r.CustomerType, &r.CompanyName, &r.CompanyAddress, &r.CompanyPhone, &r.Contact, &r.Phone, &r.Address, &r.Active, &r.DefaultSourceID, &r.DefaultOrderTypeID, &r.Updated); err != nil {
			return nil, false, err
		}
		r.CustomerType = customerapp.NormalizeCustomerType(r.CustomerType)
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, false, err
	}

	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func fetchCustomerByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*customerapp.CustomerEditData, error) {
	q := fmt.Sprintf(`SELECT id, name, COALESCE(raw_name,''), COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''),
		COALESCE(default_source_id::text,''), COALESCE(default_order_type_id::text,''), active
		FROM %s.customers WHERE id=$1`, schema)
	var d customerapp.CustomerEditData
	err := pool.QueryRow(ctx, q, id).Scan(&d.ID, &d.Name, &d.RawName, &d.CustomerType, &d.CompanyName, &d.CompanyAddress, &d.CompanyPhone, &d.Contact, &d.Phone, &d.Address, &d.DefaultSourceID, &d.DefaultOrderTypeID, &d.Active)
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

func upsertCustomer(ctx context.Context, pool *pgxpool.Pool, schema string, actor string, id *int64, req upsertRequest) (int64, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return 0, fmt.Errorf("name required")
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

	var newID int64
	if id == nil {
		q := fmt.Sprintf(`INSERT INTO %s.customers(name, raw_name, customer_type, company_name, company_address, company_phone, contact, phone, address, active, default_source_id, default_order_type_id, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now(), now()) RETURNING id`, schema)
		if err := tx.QueryRow(ctx, q, name, raw, customerType, companyName, companyAddress, companyPhone, nullText(contact), nullText(phone), nullText(address), active, ds, dt).Scan(&newID); err != nil {
			return 0, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &newID, "create", nil, nil, nil, postgresinfra.AuditMeta{"name": name}); err != nil {
			return 0, err
		}
	} else {
		newID = *id
		var oldName, oldRaw, oldCustomerType, oldCompanyName, oldCompanyAddress, oldCompanyPhone, oldContact, oldPhone, oldAddr string
		var oldActive bool
		var oldDS, oldDT *int
		q0 := fmt.Sprintf(`SELECT name, COALESCE(raw_name,''), COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id FROM %s.customers WHERE id=$1`, schema)
		if err := tx.QueryRow(ctx, q0, newID).Scan(&oldName, &oldRaw, &oldCustomerType, &oldCompanyName, &oldCompanyAddress, &oldCompanyPhone, &oldContact, &oldPhone, &oldAddr, &oldActive, &oldDS, &oldDT); err != nil {
			return 0, err
		}
		q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, raw_name=$3, customer_type=$4, company_name=$5, company_address=$6, company_phone=$7, contact=$8, phone=$9, address=$10, active=$11,
			default_source_id=$12, default_order_type_id=$13, updated_at=$14 WHERE id=$1`, schema)
		if _, err := tx.Exec(ctx, q, newID, name, raw, customerType, companyName, companyAddress, companyPhone, nullText(contact), nullText(phone), nullText(address), active, ds, dt, time.Now()); err != nil {
			return 0, err
		}
		if err := auditCustomerDiffs(ctx, tx, schema, actor, newID, customerSnapshot{oldName, customerapp.NormalizeCustomerType(oldCustomerType), oldCompanyName, oldCompanyAddress, oldCompanyPhone, oldContact, oldPhone, oldAddr, oldActive, oldDS, oldDT}, customerSnapshot{name, customerType, companyName, companyAddress, companyPhone, contact, phone, address, active, ds, dt}); err != nil {
			return 0, err
		}
	}
	if err := ensureDefaultRetailPortalTx(ctx, tx, schema, newID, name, actor, customerType, active); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newID, nil
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
	q0 := fmt.Sprintf(`SELECT name, COALESCE(NULLIF(customer_type,''),'retail'), COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(company_phone,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id
		FROM %s.customers WHERE id=$1`, schema)
	if err := tx.QueryRow(ctx, q0, id).Scan(&old.name, &old.customerType, &old.companyName, &old.companyAddress, &old.companyPhone, &old.contact, &old.phone, &old.address, &old.active, &old.sourceID, &old.typeID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("not found")
		}
		return err
	}
	old.customerType = customerapp.NormalizeCustomerType(old.customerType)
	q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, customer_type=$3, company_name=$4, company_address=$5, company_phone=$6, contact=$7, phone=$8, address=$9, active=$10,
		default_source_id=$11, default_order_type_id=$12, updated_at=$13 WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.name, next.customerType, next.companyName, next.companyAddress, next.companyPhone, nullText(next.contact), nullText(next.phone), nullText(next.address), next.active, next.sourceID, next.typeID, time.Now()); err != nil {
		return err
	}
	if err := auditCustomerDiffs(ctx, tx, schema, actor, id, old, next); err != nil {
		return err
	}
	if err := ensureDefaultRetailPortalTx(ctx, tx, schema, id, next.name, actor, next.customerType, next.active); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ensureDefaultRetailPortalTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, customerName, actor, customerType string, active bool) error {
	if !active {
		return nil
	}
	customerType = customerapp.NormalizeCustomerType(customerType)
	if customerType != customerapp.CustomerTypeRetail && customerType != customerapp.CustomerTypeEcommerce {
		return nil
	}
	var hasProfiles, hasCapabilities bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL`,
		fmt.Sprintf("%s.customer_portal_profiles", schema),
		fmt.Sprintf("%s.customer_service_capabilities", schema),
	).Scan(&hasProfiles, &hasCapabilities); err != nil {
		return err
	}
	if !hasProfiles || !hasCapabilities {
		return nil
	}
	displayName := strings.TrimSpace(customerName)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, display_name, enabled, status, theme_key, miniapp_entry_mode, capability_template_key, updated_at, updated_by)
		VALUES($1,$2,true,'active','clean_ops','mall','retail_mall',now(),$3)
		ON CONFLICT(customer_id) DO UPDATE SET
			display_name=COALESCE(NULLIF(%s.customer_portal_profiles.display_name,''), excluded.display_name),
			enabled=true,
			status='active',
			theme_key=excluded.theme_key,
			miniapp_entry_mode='mall',
			capability_template_key='retail_mall',
			updated_at=now(),
			updated_by=excluded.updated_by
	`, schema, schema), customerID, displayName, strings.TrimSpace(actor)); err != nil {
		return err
	}
	for _, capability := range []string{
		"bean_list",
		"mall",
		"product_order",
		"direct_ship",
		"processing",
		"inventory_custody",
		"settlement",
		"shipping_query",
	} {
		enabled := capability == "mall"
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled, config_json, updated_at)
			VALUES($1,$2,$3,'{}'::jsonb,now())
			ON CONFLICT(customer_id, capability_code) DO UPDATE SET
				enabled=excluded.enabled,
				config_json=excluded.config_json,
				updated_at=now()
		`, schema), customerID, capability, enabled); err != nil {
			return err
		}
	}
	return nil
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
	name           string
	customerType   string
	companyName    string
	companyAddress string
	companyPhone   string
	contact        string
	phone          string
	address        string
	active         bool
	sourceID       *int
	typeID         *int
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
	return log("default_order_type_id", intPtrString(old.typeID), intPtrString(next.typeID))
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

func nullText(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}
