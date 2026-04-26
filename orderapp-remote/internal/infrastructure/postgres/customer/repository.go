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

func upsertCustomer(ctx context.Context, pool *pgxpool.Pool, schema string, actor string, id *int64, req upsertRequest) (int64, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	raw := strings.TrimSpace(req.RawName)
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
		q := fmt.Sprintf(`INSERT INTO %s.customers(name, raw_name, contact, phone, address, active, default_source_id, default_order_type_id, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now(), now()) RETURNING id`, schema)
		if err := tx.QueryRow(ctx, q, name, raw, nullText(contact), nullText(phone), nullText(address), active, ds, dt).Scan(&newID); err != nil {
			return 0, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer", &newID, "create", nil, nil, nil, postgresinfra.AuditMeta{"name": name}); err != nil {
			return 0, err
		}
	} else {
		newID = *id
		var oldName, oldRaw, oldContact, oldPhone, oldAddr string
		var oldActive bool
		var oldDS, oldDT *int
		q0 := fmt.Sprintf(`SELECT name, COALESCE(raw_name,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id FROM %s.customers WHERE id=$1`, schema)
		if err := tx.QueryRow(ctx, q0, newID).Scan(&oldName, &oldRaw, &oldContact, &oldPhone, &oldAddr, &oldActive, &oldDS, &oldDT); err != nil {
			return 0, err
		}
		q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, raw_name=$3, contact=$4, phone=$5, address=$6, active=$7,
			default_source_id=$8, default_order_type_id=$9, updated_at=$10 WHERE id=$1`, schema)
		if _, err := tx.Exec(ctx, q, newID, name, raw, nullText(contact), nullText(phone), nullText(address), active, ds, dt, time.Now()); err != nil {
			return 0, err
		}
		if err := auditCustomerDiffs(ctx, tx, schema, actor, newID, customerSnapshot{oldName, oldContact, oldPhone, oldAddr, oldActive, oldDS, oldDT}, customerSnapshot{name, contact, phone, address, active, ds, dt}); err != nil {
			return 0, err
		}
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
		name:     name,
		contact:  strings.TrimSpace(req.Contact),
		phone:    strings.TrimSpace(req.Phone),
		address:  strings.TrimSpace(req.Address),
		active:   strings.TrimSpace(req.Active) != "",
		sourceID: parseOptionalInt(req.DefaultSourceID),
		typeID:   parseOptionalInt(req.DefaultOrderTypeID),
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
	q0 := fmt.Sprintf(`SELECT name, COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id
		FROM %s.customers WHERE id=$1`, schema)
	if err := tx.QueryRow(ctx, q0, id).Scan(&old.name, &old.contact, &old.phone, &old.address, &old.active, &old.sourceID, &old.typeID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("not found")
		}
		return err
	}
	q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, contact=$3, phone=$4, address=$5, active=$6,
		default_source_id=$7, default_order_type_id=$8, updated_at=$9 WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.name, nullText(next.contact), nullText(next.phone), nullText(next.address), next.active, next.sourceID, next.typeID, time.Now()); err != nil {
		return err
	}
	if err := auditCustomerDiffs(ctx, tx, schema, actor, id, old, next); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	name     string
	contact  string
	phone    string
	address  string
	active   bool
	sourceID *int
	typeID   *int
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
