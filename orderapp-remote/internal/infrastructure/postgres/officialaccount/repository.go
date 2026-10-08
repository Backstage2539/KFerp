package officialaccount

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/officialaccount"
	pg "orderapp/internal/infrastructure/postgres"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	"time"
)

type Repository struct {
	Pool   *pgxpool.Pool
	Schema string
	Portal portalpg.Repository
}

func NewRepository(p *pgxpool.Pool, s string) Repository {
	return Repository{p, s, portalpg.NewRepository(p, s)}
}
func (r Repository) q(s string) string { return fmt.Sprintf(s, r.Schema) }
func (r Repository) audit(ctx context.Context, tx pgx.Tx, actor, action string, meta pg.AuditMeta) error {
	return pg.AuditInsertTx(ctx, tx, r.Schema, actor, "wechat_official_account", nil, action, nil, nil, nil, meta)
}

const publicationTypeKey = `CASE
 WHEN COALESCE(p.product_type_category_id,0)>0 THEN 'product-type:'||p.product_type_category_id::text
 WHEN COALESCE(p.classification_template_id,0)>0 THEN 'classification-template:'||p.classification_template_id::text
 WHEN COALESCE(p.classification_category_id,0)>0 THEN 'classification-category:'||p.classification_category_id::text
 ELSE '' END`

const entryColumns = `e.entry_key,e.scope_key,e.name,COALESCE(e.publication_id,0),e.visibility,e.enabled,e.revision,COALESCE(p.owner_type,''),COALESCE(p.owner_key,''),COALESCE(p.version_no,''),COALESCE(NULLIF(p.publication_table_name,''),NULLIF(p.config_json->'publication_batch'->>'table_name',''),NULLIF(p.product_type_name,''),''),CASE WHEN COALESCE(e.publication_id,0)=0 THEN 'unconfigured' WHEN p.deleted_at IS NULL THEN p.status ELSE 'deleted' END,e.type_key,e.type_name,e.purpose`

func scanEntry(row pgx.Row) (e app.Entry, err error) {
	err = row.Scan(&e.Key, &e.Scope, &e.Name, &e.PublicationID, &e.Visibility, &e.Enabled, &e.Revision, &e.OwnerType, &e.OwnerKey, &e.Version, &e.TableName, &e.Status, &e.TypeKey, &e.TypeName, &e.Purpose)
	e.PagePath = "pages/price-list/price-list?entry=" + e.Key
	return
}
func (r Repository) Entry(ctx context.Context, key string) (app.Entry, error) {
	return scanEntry(r.Pool.QueryRow(ctx, r.q(`SELECT `+entryColumns+` FROM %[1]s.wechat_price_entries e LEFT JOIN %[1]s.bean_list_publications p ON p.id=e.publication_id WHERE e.entry_key=$1`), key))
}
func (r Repository) Entries(ctx context.Context) ([]app.Entry, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT `+entryColumns+` FROM %[1]s.wechat_price_entries e LEFT JOIN %[1]s.bean_list_publications p ON p.id=e.publication_id WHERE e.type_key<>'' ORDER BY e.type_name,e.purpose,e.entry_key`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r Repository) Versions(ctx context.Context, key string) ([]app.Version, error) {
	query := `SELECT p.id,p.version_no,COALESCE(NULLIF(p.publication_table_name,''),NULLIF(p.config_json->'publication_batch'->>'table_name',''),NULLIF(p.product_type_name,''),p.list_type),p.status,COALESCE(p.publication_table_key,''),p.owner_type,p.owner_key
	 FROM %[1]s.bean_list_publications p JOIN %[1]s.wechat_price_entries e ON e.entry_key=$1
	 WHERE p.status='published' AND p.deleted_at IS NULL AND ((e.type_key<>'' AND e.type_key=` + publicationTypeKey + `) OR (e.type_key='' AND e.scope_key=%[1]s.wechat_price_scope(p)))
	 ORDER BY p.published_at DESC,p.id DESC`
	rows, err := r.Pool.Query(ctx, r.q(query), key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Version{}
	for rows.Next() {
		var v app.Version
		if err = rows.Scan(&v.ID, &v.Version, &v.Name, &v.Status, &v.TableKey, &v.OwnerType, &v.OwnerKey); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r Repository) SaveEntry(ctx context.Context, key string, e app.Entry, actor string) (app.Entry, error) {
	if e.Visibility != "public" && e.Visibility != "authenticated" {
		return e, errors.New("请选择可见范围")
	}
	if e.TypeKey == "" && (len(e.Name) > 160 || e.Name == "") {
		return e, errors.New("请输入入口名称（最多 160 字节）")
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return e, err
	}
	defer tx.Rollback(ctx)
	var tag pgconn.CommandTag
	var typeKey string
	if err = tx.QueryRow(ctx, r.q(`SELECT type_key FROM %[1]s.wechat_price_entries WHERE entry_key=$1 FOR UPDATE`), key).Scan(&typeKey); err != nil {
		return e, err
	}
	if typeKey != "" {
		if e.PublicationID <= 0 {
			return e, errors.New("请选择同一商品类型下的已发布价格表版本")
		}
		purposeLabel := `CASE WHEN e.purpose='wholesale' THEN ' 批发' ELSE ' 一件代发' END`
		tag, err = tx.Exec(ctx, r.q(`UPDATE %[1]s.wechat_price_entries e SET name=e.type_name||`+purposeLabel+`,publication_id=p.id,visibility=$3,enabled=$4,revision=e.revision+1,updated_at=now() FROM %[1]s.bean_list_publications p WHERE e.entry_key=$1 AND p.id=$2 AND e.revision=$5 AND e.type_key<>'' AND e.type_key=`+publicationTypeKey+` AND ((p.status='published' AND p.deleted_at IS NULL) OR ($4=false AND p.id=e.publication_id)) AND ($3<>'public' OR p.owner_type='official')`), key, e.PublicationID, e.Visibility, e.Enabled, e.Revision)
	} else {
		tag, err = tx.Exec(ctx, r.q(`UPDATE %[1]s.wechat_price_entries e SET name=$2,publication_id=p.id,visibility=$4,enabled=$5,revision=e.revision+1,updated_at=now() FROM %[1]s.bean_list_publications p WHERE e.entry_key=$1 AND p.id=$3 AND e.revision=$6 AND %[1]s.wechat_price_scope(p)=e.scope_key AND ((p.status='published' AND p.deleted_at IS NULL) OR ($5=false AND p.id=e.publication_id)) AND ($4<>'public' OR p.owner_type='official')`), key, e.Name, e.PublicationID, e.Visibility, e.Enabled, e.Revision)
	}
	if err != nil {
		return e, err
	}
	if tag.RowsAffected() != 1 {
		return e, errors.New("版本不可用、价格表归属不符或配置已更新，请刷新")
	}
	if err = r.audit(ctx, tx, actor, "entry_update", pg.AuditMeta{"entry_key": key, "publication_id": e.PublicationID, "type_key": typeKey, "purpose": e.Purpose, "visibility": e.Visibility, "enabled": e.Enabled}); err != nil {
		return e, err
	}
	if err = tx.Commit(ctx); err != nil {
		return e, err
	}
	return r.Entry(ctx, key)
}

type Binding struct {
	OpenID       string    `json:"openid"`
	MiniUserID   int64     `json:"mini_user_id"`
	CustomerID   int64     `json:"customer_id"`
	Active       bool      `json:"active"`
	BoundAt      time.Time `json:"bound_at"`
	CustomerName string    `json:"customer_name"`
}

func (r Repository) Binding(ctx context.Context, appID, openid string) (b Binding, err error) {
	err = r.Pool.QueryRow(ctx, r.q(`SELECT b.openid,b.mini_user_id,b.customer_id,b.active,b.bound_at,COALESCE(c.name,'') FROM %[1]s.wechat_bindings b LEFT JOIN %[1]s.customers c ON c.id=b.customer_id WHERE b.app_id=$1 AND b.openid=$2`), appID, openid).Scan(&b.OpenID, &b.MiniUserID, &b.CustomerID, &b.Active, &b.BoundAt, &b.CustomerName)
	return
}
func (r Repository) Bindings(ctx context.Context, appID string, userID int64) ([]Binding, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT b.openid,b.mini_user_id,b.customer_id,b.active,b.bound_at,COALESCE(c.name,'') FROM %[1]s.wechat_bindings b LEFT JOIN %[1]s.customers c ON c.id=b.customer_id WHERE b.app_id=$1 AND ($2::bigint=0 OR b.mini_user_id=$2) ORDER BY b.bound_at DESC LIMIT 500`), appID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Binding{}
	for rows.Next() {
		var b Binding
		if err = rows.Scan(&b.OpenID, &b.MiniUserID, &b.CustomerID, &b.Active, &b.BoundAt, &b.CustomerName); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r Repository) AutoBind(ctx context.Context, appID, openid, unionID string) (Binding, error) {
	if unionID == "" {
		return Binding{}, app.ErrDenied
	}
	rows, err := r.Pool.Query(ctx, r.q(`SELECT id FROM %[1]s.mini_users WHERE unionid=$1 AND active=true LIMIT 2`), unionID)
	if err != nil {
		return Binding{}, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return Binding{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Binding{}, err
	}
	if len(ids) != 1 {
		return Binding{}, app.ErrDenied
	}
	c, err := r.Portal.OfficialAccountContext(ctx, ids[0], 0, time.Now())
	if err != nil || len(c.Bindings) == 0 {
		return Binding{}, app.ErrDenied
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Binding{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, r.q(`INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`), appID, openid, ids[0], c.CurrentCustomerID)
	if err != nil {
		return Binding{}, err
	}
	if tag.RowsAffected() > 0 {
		if err = r.audit(ctx, tx, "wechat:auto", "bind", pg.AuditMeta{"openid": openid, "mini_user_id": ids[0], "customer_id": c.CurrentCustomerID}); err != nil {
			return Binding{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Binding{}, err
	}
	return r.Binding(ctx, appID, openid)
}
func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
func (r Repository) CreateCode(ctx context.Context, appID string, c portal.CurrentContext) (string, error) {
	if c.MiniUserID <= 0 || c.CurrentCustomerID <= 0 || c.AccountType == "employee" {
		return "", app.ErrDenied
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b)
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	// Serialize issuance per user; only the newest unconsumed code remains valid.
	if _, err = tx.Exec(ctx, r.q(`SELECT id FROM %[1]s.mini_users WHERE id=$1 FOR UPDATE`), c.MiniUserID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, r.q(`DELETE FROM %[1]s.wechat_binding_codes WHERE app_id=$1 AND mini_user_id=$2 OR expires_at<now()`), appID, c.MiniUserID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, r.q(`INSERT INTO %[1]s.wechat_binding_codes(code_hash,app_id,mini_user_id,customer_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '5 minutes')`), hashCode(code), appID, c.MiniUserID, c.CurrentCustomerID); err != nil {
		return "", err
	}
	if err = r.audit(ctx, tx, fmt.Sprint("mini-user:", c.MiniUserID), "binding_code_issue", pg.AuditMeta{"customer_id": c.CurrentCustomerID}); err != nil {
		return "", err
	}
	return code, tx.Commit(ctx)
}
func (r Repository) ConsumeCode(ctx context.Context, appID, openid, code string) error {
	var attempts int
	err := r.Pool.QueryRow(ctx, r.q(`INSERT INTO %[1]s.wechat_binding_attempts(app_id,openid,attempts) VALUES($1,$2,1) ON CONFLICT(app_id,openid) DO UPDATE SET attempts=CASE WHEN wechat_binding_attempts.bucket<now()-interval '5 minutes' THEN 1 ELSE wechat_binding_attempts.attempts+1 END,bucket=CASE WHEN wechat_binding_attempts.bucket<now()-interval '5 minutes' THEN now() ELSE wechat_binding_attempts.bucket END RETURNING attempts`), appID, openid).Scan(&attempts)
	if err != nil {
		return err
	}
	if attempts > 5 {
		return app.ErrRateLimited
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var uid, cid int64
	var issued time.Time
	err = tx.QueryRow(ctx, r.q(`UPDATE %[1]s.wechat_binding_codes SET consumed_at=now() WHERE code_hash=$1 AND app_id=$2 AND expires_at>now() AND consumed_at IS NULL RETURNING mini_user_id,customer_id,created_at`), hashCode(code), appID).Scan(&uid, &cid, &issued)
	if err != nil {
		return app.ErrInvalidCode
	}
	c, err := r.Portal.OfficialAccountContext(ctx, uid, cid, issued)
	if err != nil || c.CurrentCustomerID != cid {
		return app.ErrDenied
	}
	tag, err := tx.Exec(ctx, r.q(`INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id) VALUES($1,$2,$3,$4) ON CONFLICT(app_id,openid) DO UPDATE SET mini_user_id=excluded.mini_user_id,customer_id=excluded.customer_id,active=true,bound_at=now() WHERE wechat_bindings.active=false OR wechat_bindings.mini_user_id=excluded.mini_user_id`), appID, openid, uid, cid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return app.ErrBindingConflict
	}
	if err = r.audit(ctx, tx, fmt.Sprint("mini-user:", uid), "bind", pg.AuditMeta{"openid": openid, "customer_id": cid}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) ChangeBinding(ctx context.Context, appID, openid string, userID, customerID int64, active bool, actor string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, r.q(`UPDATE %[1]s.wechat_bindings SET active=$4,customer_id=CASE WHEN $4 THEN $5 ELSE customer_id END WHERE app_id=$1 AND openid=$2 AND ($3::bigint=0 OR mini_user_id=$3) AND (NOT $4 OR active=true)`), appID, openid, userID, active, customerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return app.ErrDenied
	}
	if err = r.audit(ctx, tx, actor, "binding_update", pg.AuditMeta{"openid": openid, "customer_id": customerID, "active": active}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type MenuVersion struct {
	ID        int64           `json:"id"`
	Menu      json.RawMessage `json:"menu"`
	Status    string          `json:"status"`
	Actor     string          `json:"actor"`
	CreatedAt time.Time       `json:"created_at"`
}

func (r Repository) SaveMenu(ctx context.Context, appID, status, actor string, menu json.RawMessage) (int64, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, r.q(`INSERT INTO %[1]s.wechat_menu_versions(app_id,menu_json,status,actor) VALUES($1,$2,$3,$4) RETURNING id`), appID, menu, status, actor).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = r.audit(ctx, tx, actor, "menu_"+status, pg.AuditMeta{"version_id": id}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}
func (r Repository) Menus(ctx context.Context, appID string) ([]MenuVersion, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT id,menu_json,status,actor,created_at FROM %[1]s.wechat_menu_versions WHERE app_id=$1 ORDER BY id DESC LIMIT 50`), appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MenuVersion{}
	for rows.Next() {
		var m MenuVersion
		if err = rows.Scan(&m.ID, &m.Menu, &m.Status, &m.Actor, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r Repository) FirstEvent(ctx context.Context, appID, key string) (bool, error) {
	tag, err := r.Pool.Exec(ctx, r.q(`INSERT INTO %[1]s.wechat_callback_events(app_id,event_key) VALUES($1,$2) ON CONFLICT(app_id,event_key) DO UPDATE SET expires_at=excluded.expires_at WHERE wechat_callback_events.expires_at<now()`), appID, key)
	return tag.RowsAffected() == 1, err
}

func (r Repository) EntryForPublication(ctx context.Context, id int64) (app.Entry, error) {
	return scanEntry(r.Pool.QueryRow(ctx, r.q(`SELECT `+entryColumns+` FROM %[1]s.wechat_price_entries e JOIN %[1]s.bean_list_publications p ON p.id=e.publication_id JOIN %[1]s.bean_list_publications source ON e.scope_key=%[1]s.wechat_price_scope(source) WHERE source.id=$1`), id))
}

func (r Repository) EntriesForPublication(ctx context.Context, id int64) ([]app.Entry, error) {
	rows, err := r.Pool.Query(ctx, r.q(`SELECT `+entryColumns+` FROM %[1]s.bean_list_publications source
	 JOIN %[1]s.wechat_price_entries e ON e.type_key=%[1]s.wechat_price_type_key(source) AND e.type_key<>''
	 LEFT JOIN %[1]s.bean_list_publications p ON p.id=e.publication_id
	 WHERE source.id=$1 ORDER BY e.purpose,e.entry_key`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Entry{}
	for rows.Next() {
		e, scanErr := scanEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
