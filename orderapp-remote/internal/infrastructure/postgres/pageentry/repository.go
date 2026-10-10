package pageentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	app "orderapp/internal/application/pageentry"
	pg "orderapp/internal/infrastructure/postgres"
	portal "orderapp/internal/infrastructure/postgres/customerportal"
	"strings"
)

type Repository struct {
	Pool   *pgxpool.Pool
	Schema string
	Portal portal.Repository
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool, schema, portal.NewRepository(pool, schema)}
}
func (r Repository) q(s string) string { return fmt.Sprintf(s, r.Schema) }

const columns = `entry_key,draft,published,revision,published_revision,enabled,has_draft,deleted_at IS NOT NULL,updated_at`

func scan(row pgx.Row) (e app.Entry, err error) {
	var d, p []byte
	err = row.Scan(&e.Key, &d, &p, &e.Revision, &e.PublishedRevision, &e.Enabled, &e.HasDraft, &e.Deleted, &e.UpdatedAt)
	if err != nil {
		return
	}
	err = json.Unmarshal(d, &e.Draft)
	if err == nil && len(p) > 0 {
		err = json.Unmarshal(p, &e.Published)
	}
	return
}
func (r Repository) Entry(ctx context.Context, key string) (app.Entry, error) {
	return scan(r.Pool.QueryRow(ctx, r.q(`SELECT `+columns+` FROM %[1]s.page_entries WHERE entry_key=$1`), key))
}
func (r Repository) List(ctx context.Context) ([]app.Entry, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT `+columns+` FROM %[1]s.page_entries WHERE deleted_at IS NULL ORDER BY updated_at DESC,entry_key`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Entry{}
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r Repository) audit(ctx context.Context, tx pgx.Tx, e app.Entry, action, actor string) error {
	raw, _ := json.Marshal(e.Draft)
	_, err := tx.Exec(ctx, r.q(`INSERT INTO %[1]s.page_entry_revisions(entry_key,revision,action,document,actor) VALUES($1,$2,$3,$4,$5)`), e.Key, e.Revision, action, raw, actor)
	if err != nil {
		return err
	}
	return pg.AuditInsertTx(ctx, tx, r.Schema, actor, "page_entry", nil, action, nil, nil, nil, pg.AuditMeta{"entry_key": e.Key, "name": e.Draft.Name, "kind": e.Draft.Kind, "revision": e.Revision})
}
func (r Repository) Create(ctx context.Context, d app.Document, actor string) (app.Entry, error) {
	d = app.Normalize(d)
	if err := app.Validate(d, false); err != nil {
		return app.Entry{}, err
	}
	key, err := app.NewKey()
	if err != nil {
		return app.Entry{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Entry{}, err
	}
	defer tx.Rollback(ctx)
	if err = r.validateTarget(ctx, tx, key, d, false); err != nil {
		return app.Entry{}, err
	}
	raw, _ := json.Marshal(d)
	e, err := scan(tx.QueryRow(ctx, r.q(`INSERT INTO %[1]s.page_entries(entry_key,draft) VALUES($1,$2) RETURNING `+columns), key, raw))
	if err != nil {
		return e, err
	}
	if err = r.audit(ctx, tx, e, "create", actor); err == nil {
		err = tx.Commit(ctx)
	}
	return e, err
}
func (r Repository) validateTarget(ctx context.Context, tx pgx.Tx, key string, d app.Document, publish bool) error {
	if err := app.Validate(d, publish); err != nil {
		return err
	}
	if d.Kind == "price" && d.PublicationID > 0 {
		var owner, status string
		var deleted bool
		err := tx.QueryRow(ctx, r.q(`SELECT owner_type,status,deleted_at IS NOT NULL FROM %[1]s.bean_list_publications WHERE id=$1 FOR SHARE`), d.PublicationID).Scan(&owner, &status, &deleted)
		if err != nil || status != "published" || deleted {
			return app.ErrUnavailable
		}
		if owner != "official" && (d.Visibility == "public" || d.Visibility == "registered" || d.BeanCenter) {
			return app.ErrDenied
		}
		if publish {
			if _, err = r.Portal.LoadEntryPublication(ctx, d.PublicationID); err != nil {
				return app.ErrUnavailable
			}
		}
	}
	for _, b := range d.Blocks {
		if b.Kind != "image" {
			continue
		}
		var exists bool
		err := tx.QueryRow(ctx, r.q(`SELECT EXISTS(SELECT 1 FROM %[1]s.page_entry_assets WHERE asset_key=$1 AND entry_key=$2)`), b.AssetID, key).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return app.ErrInvalid
		}
	}
	return nil
}
func (r Repository) Change(ctx context.Context, key string, revision int64, action string, d app.Document, actor string) (app.Entry, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Entry{}, err
	}
	defer tx.Rollback(ctx)
	e, err := scan(tx.QueryRow(ctx, r.q(`SELECT `+columns+` FROM %[1]s.page_entries WHERE entry_key=$1 FOR UPDATE`), key))
	if err != nil || e.Deleted {
		return e, app.ErrUnavailable
	}
	if e.Revision != revision {
		return e, app.ErrConflict
	}
	switch action {
	case "save":
		d = app.Normalize(d)
		if err = r.validateTarget(ctx, tx, key, d, false); err != nil {
			return e, err
		}
		e.Draft = d
		e.HasDraft = true
	case "publish":
		if err = r.validateTarget(ctx, tx, key, e.Draft, true); err != nil {
			return e, err
		}
		p := e.Draft
		e.Published = &p
		e.Enabled = true
		e.HasDraft = false
		e.PublishedRevision = e.Revision + 1
	case "disable":
		e.Enabled = false
	case "delete":
		e.Enabled = false
		e.Deleted = true
	default:
		return e, app.ErrInvalid
	}
	e.Revision++
	draft, _ := json.Marshal(e.Draft)
	var published any
	if e.Published != nil {
		published, _ = json.Marshal(e.Published)
	}
	e, err = scan(tx.QueryRow(ctx, r.q(`UPDATE %[1]s.page_entries SET draft=$2,published=$3,revision=$4,published_revision=$5,enabled=$6,has_draft=$7,deleted_at=CASE WHEN $8 THEN now() ELSE NULL END,updated_at=now() WHERE entry_key=$1 RETURNING `+columns), key, draft, published, e.Revision, e.PublishedRevision, e.Enabled, e.HasDraft, e.Deleted))
	if err != nil {
		return e, err
	}
	if err = r.audit(ctx, tx, e, action, actor); err == nil {
		err = tx.Commit(ctx)
	}
	return e, err
}
func (r Repository) Targets(ctx context.Context) ([]app.Target, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT p.id,%[1]s.wechat_price_scope(p),COALESCE(NULLIF(p.publication_table_name,''),NULLIF(p.config_json->'publication_batch'->>'table_name',''),NULLIF(p.config_json->>'title',''),'价格表'),COALESCE(NULLIF(p.classification_template_name,''),NULLIF(p.product_type_name,''),p.list_type),p.owner_type,p.owner_key,p.version_no FROM %[1]s.bean_list_publications p WHERE p.status='published' AND p.deleted_at IS NULL AND p.publication_purpose='factory_supply' ORDER BY p.published_at DESC,p.id DESC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Target{}
	for rows.Next() {
		var t app.Target
		if err = rows.Scan(&t.ID, &t.TableScope, &t.Name, &t.TypeName, &t.OwnerType, &t.OwnerKey, &t.Version); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func referencesKey(v any, key string) []string {
	out := []string{}
	switch x := v.(type) {
	case map[string]any:
		for _, field := range []string{"pagepath", "url"} {
			if path, ok := x[field].(string); ok {
				u, err := url.Parse(path)
				if err == nil && (strings.TrimPrefix(u.Path, "/") == "pages/page-entry/page-entry" && u.Query().Get("entry") == key || strings.TrimSuffix(u.Path, "/") == "/app/p/"+key) {
					name, _ := x["name"].(string)
					out = append(out, name)
				}
			}
		}
		for _, item := range x {
			out = append(out, referencesKey(item, key)...)
		}
	case []any:
		for _, item := range x {
			out = append(out, referencesKey(item, key)...)
		}
	}
	return out
}
func (r Repository) References(ctx context.Context, key string) ([]app.Reference, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT id,status,menu_json FROM (SELECT DISTINCT ON(app_id,status) id,app_id,status,menu_json FROM %[1]s.wechat_menu_versions ORDER BY app_id,status,id DESC) current_menus`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Reference{}
	for rows.Next() {
		var id int64
		var status string
		var raw []byte
		if err = rows.Scan(&id, &status, &raw); err != nil {
			return nil, err
		}
		var v any
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		for _, name := range referencesKey(v, key) {
			out = append(out, app.Reference{ID: id, Status: status, Name: name})
		}
	}
	return out, rows.Err()
}
func (r Repository) SaveAsset(ctx context.Context, key, mime string, data []byte, actor string) (string, error) {
	asset, err := app.NewKey()
	if err != nil {
		return "", err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var found string
	if err = tx.QueryRow(ctx, r.q(`SELECT entry_key FROM %[1]s.page_entries WHERE entry_key=$1 AND deleted_at IS NULL FOR SHARE`), key).Scan(&found); err != nil {
		return "", app.ErrUnavailable
	}
	if _, err = tx.Exec(ctx, r.q(`INSERT INTO %[1]s.page_entry_assets(asset_key,entry_key,mime,data) VALUES($1,$2,$3,$4)`), asset, key, mime, data); err != nil {
		return "", err
	}
	if err = pg.AuditInsertTx(ctx, tx, r.Schema, actor, "page_entry", nil, "upload_image", nil, nil, nil, pg.AuditMeta{"entry_key": key, "asset_key": asset, "size": len(data)}); err != nil {
		return "", err
	}
	return asset, tx.Commit(ctx)
}
func (r Repository) Asset(ctx context.Context, key, asset string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := r.Pool.QueryRow(ctx, r.q(`SELECT data,mime FROM %[1]s.page_entry_assets WHERE entry_key=$1 AND asset_key=$2`), key, asset).Scan(&data, &mime)
	return data, mime, err
}
func IsMissing(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
