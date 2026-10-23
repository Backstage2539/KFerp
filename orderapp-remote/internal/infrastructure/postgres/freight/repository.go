package freight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/freight"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) *Repository {
	return &Repository{pool: pool, schema: schema}
}

type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, e := r.pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.shipping_fee_templates ADD COLUMN IF NOT EXISTS config_json jsonb NOT NULL DEFAULT '{}';
 CREATE TABLE IF NOT EXISTS %[1]s.shipping_fee_template_versions(template_id bigint NOT NULL,revision bigint NOT NULL,snapshot_json jsonb NOT NULL,created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(template_id,revision));
 INSERT INTO %[1]s.shipping_fee_template_versions(template_id,revision,snapshot_json,created_by) SELECT id,revision,config_json||jsonb_build_object('id',id,'name',name,'active',active,'revision',revision,'packaging_weight_g',packaging_weight_g,'rules',rules_json),'legacy_migration' FROM %[1]s.shipping_fee_templates ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS %[1]s.system_shipping_policy(singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),revision bigint NOT NULL,template_id bigint NOT NULL DEFAULT 0,surcharge_cents bigint NOT NULL DEFAULT 0 CHECK(surcharge_cents>=0),free_shipping boolean NOT NULL DEFAULT false,updated_at timestamptz NOT NULL DEFAULT now(),updated_by text NOT NULL);
 CREATE TABLE IF NOT EXISTS %[1]s.customer_shipping_policies(customer_id bigint NOT NULL REFERENCES %[1]s.customers(id),scene text NOT NULL CHECK(scene IN ('sale','stock','on_demand')),revision bigint NOT NULL DEFAULT 1,template_id bigint NOT NULL DEFAULT 0,surcharge_cents bigint NOT NULL DEFAULT 0 CHECK(surcharge_cents>=0),free_shipping boolean NOT NULL DEFAULT false,updated_at timestamptz NOT NULL DEFAULT now(),updated_by text NOT NULL,PRIMARY KEY(customer_id,scene));
 INSERT INTO %[1]s.customer_shipping_policies(customer_id,scene,template_id,updated_by)
 SELECT customer_id,m->>'mode',(m->>'shipping_template_id')::bigint,'legacy_migration' FROM %[1]s.customer_fulfillment_settings s CROSS JOIN LATERAL jsonb_array_elements(s.modes_json) m WHERE m->>'mode' IN ('stock','on_demand') AND COALESCE((m->>'shipping_template_id')::bigint,0)>0 ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS %[1]s.order_freight_snapshots(order_id bigint PRIMARY KEY REFERENCES %[1]s.orders(id),customer_id bigint NOT NULL REFERENCES %[1]s.customers(id),snapshot_json jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),created_by text NOT NULL);
 CREATE TABLE IF NOT EXISTS %[1]s.freight_import_batches(id bigserial PRIMARY KEY,carrier_id bigint NOT NULL,file_hash text NOT NULL,filename text NOT NULL,rows_json jsonb NOT NULL,status text NOT NULL DEFAULT 'preview',created_at timestamptz NOT NULL DEFAULT now(),created_by text NOT NULL,UNIQUE(carrier_id,file_hash));
 CREATE TABLE IF NOT EXISTS %[1]s.freight_actual_shipments(id bigserial PRIMARY KEY,carrier_id bigint NOT NULL,tracking_no text NOT NULL,order_id bigint NOT NULL REFERENCES %[1]s.orders(id),customer_id bigint NOT NULL REFERENCES %[1]s.customers(id),actual_cents bigint NOT NULL DEFAULT 0,billed_cents bigint NOT NULL DEFAULT 0,revision bigint NOT NULL DEFAULT 0,legacy boolean NOT NULL DEFAULT false,UNIQUE(carrier_id,tracking_no));
 CREATE TABLE IF NOT EXISTS %[1]s.freight_actual_entries(id bigserial PRIMARY KEY,shipment_id bigint NOT NULL REFERENCES %[1]s.freight_actual_shipments(id),batch_id bigint REFERENCES %[1]s.freight_import_batches(id),request_key text NOT NULL UNIQUE,actual_cents bigint NOT NULL,target_cents bigint NOT NULL,delta_cents bigint NOT NULL,previous_actual_cents bigint NOT NULL,previous_billed_cents bigint NOT NULL,reason text NOT NULL,created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),reverses_id bigint REFERENCES %[1]s.freight_actual_entries(id));
 DROP TRIGGER IF EXISTS guard_deferred_freight ON %[1]s.orders;
 DROP TRIGGER IF EXISTS guard_order_freight_snapshot ON %[1]s.order_freight_snapshots;
 CREATE OR REPLACE FUNCTION %[1]s.guard_deferred_freight() RETURNS trigger LANGUAGE plpgsql AS $guard$
 DECLARE oid bigint; expected bigint; actual bigint; protocol text;
 BEGIN
 IF TG_TABLE_NAME='orders' THEN oid=NEW.id; ELSE oid=COALESCE(NEW.order_id,OLD.order_id); END IF;
 SELECT COALESCE((snapshot_json->>'current_receivable_cents')::bigint,0),snapshot_json->>'protocol' INTO expected,protocol FROM %[1]s.order_freight_snapshots WHERE order_id=oid;
 IF FOUND THEN
 SELECT round(COALESCE(shipping_amount,0)*100)::bigint INTO actual FROM %[1]s.orders WHERE id=oid;
 IF actual IS DISTINCT FROM expected OR protocol NOT IN ('deferred_freight_v1','order_freight_v2') OR (protocol='deferred_freight_v1' AND expected<>0) THEN RAISE EXCEPTION '订单运费与收费快照不一致' USING ERRCODE='23514'; END IF;
 END IF; RETURN NULL;
 END $guard$;
 CREATE CONSTRAINT TRIGGER guard_deferred_freight AFTER UPDATE OF shipping_amount ON %[1]s.orders DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION %[1]s.guard_deferred_freight();
 CREATE CONSTRAINT TRIGGER guard_order_freight_snapshot AFTER INSERT OR UPDATE OR DELETE ON %[1]s.order_freight_snapshots DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION %[1]s.guard_deferred_freight();

 `, r.schema))
	return e
}
func (r *Repository) LoadTemplate(ctx context.Context, q Queryer, id int64, lock bool) (domain.Template, error) {
	var t domain.Template
	var rules, config []byte
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	e := q.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,active,revision,packaging_weight_g,rules_json,config_json FROM %s.shipping_fee_templates WHERE id=$1%s`, r.schema, suffix), id).Scan(&t.ID, &t.Name, &t.Active, &t.Revision, &t.PackagingWeightG, &rules, &config)
	if e != nil {
		return t, e
	}
	if e = json.Unmarshal(rules, &t.Rules); e != nil {
		return t, e
	}
	var extra domain.Template
	if e = json.Unmarshal(config, &extra); e != nil {
		return t, e
	}
	t.Groups = extra.Groups
	t.CarrierID = extra.CarrierID
	t.ProductID = extra.ProductID
	return t, nil
}
func (r *Repository) Policies(ctx context.Context, id int64) ([]domain.Policy, error) {
	out := []domain.Policy{}
	for _, scene := range []string{"sale", "stock", "on_demand"} {
		p, e := r.loadFulfillmentPolicy(ctx, r.pool, id, scene, false)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (r *Repository) LoadPolicy(ctx context.Context, q Queryer, id int64, scene string, lock bool) (domain.Policy, error) {
	p := domain.Policy{CustomerID: id, Scene: scene}
	if !domain.ValidScene(scene) || id <= 0 {
		return p, fmt.Errorf("客户或运费场景无效")
	}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	e := q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,template_id,surcharge_cents,free_shipping FROM %s.customer_shipping_policies WHERE customer_id=$1 AND scene=$2%s`, r.schema, suffix), id, scene).Scan(&p.Revision, &p.TemplateID, &p.SurchargeCents, &p.FreeShipping)
	if errors.Is(e, pgx.ErrNoRows) {
		e = nil
	}
	return p, e
}
func (r *Repository) SavePolicy(ctx context.Context, p domain.Policy, expected int64, actor string) (domain.Policy, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return p, e
	}
	defer tx.Rollback(ctx)
	// Legacy policy editors cannot silently replace fields owned by shared defaults.
	// The unified fulfillment editor uses SavePolicyTx after explicitly choosing inheritance.
	if p.Scene == "stock" || p.Scene == "on_demand" {
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("fulfillment-settings:%d", p.CustomerID)); e != nil {
			return p, e
		}
		var raw []byte
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT modes_json FROM %s.customer_fulfillment_settings WHERE customer_id=$1 FOR UPDATE`, r.schema), p.CustomerID).Scan(&raw)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return p, e
		}
		if e == nil {
			var modes []struct {
				Mode            string   `json:"mode"`
				InheritedFields []string `json:"inherited_fields"`
			}
			if e = json.Unmarshal(raw, &modes); e != nil {
				return p, e
			}
			for _, mode := range modes {
				if mode.Mode != p.Scene {
					continue
				}
				for _, field := range mode.InheritedFields {
					if field == "shipping_template_id" || field == "shipping_surcharge_cents" || field == "shipping_free" {
						return p, fmt.Errorf("此场景运费正在使用履约默认配置，请到客户履约设置中自定义后保存")
					}
				}
			}
		}
	}
	p, e = r.SavePolicyTx(ctx, tx, p, expected, actor)
	if e != nil {
		return p, e
	}
	// Ordinary sale policies have their own quote token and are independent of
	// dropshipping configuration. Only scenario edits invalidate its editor.
	if p.Scene != "sale" {
		if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET revision=revision+1 WHERE customer_id=$1`, r.schema), p.CustomerID); e != nil {
			return p, e
		}
	}
	return p, tx.Commit(ctx)
}

// SavePolicyTx participates in the caller's transaction and audits the policy.
// It deliberately leaves the fulfillment revision and transaction commit to the
// caller, allowing multiple policies and service/warehouse settings to commit once.
func (r *Repository) SavePolicyTx(ctx context.Context, tx pgx.Tx, p domain.Policy, expected int64, actor string) (domain.Policy, error) {
	if e := domain.ValidatePolicyOverride(p); e != nil {
		return p, e
	}
	if p.CustomerID <= 0 || strings.TrimSpace(actor) == "" {
		return p, fmt.Errorf("客户及操作人必填")
	}
	// This lock is reentrant when the atomic fulfillment editor already owns it.
	_, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("fulfillment-settings:%d", p.CustomerID))
	if e != nil {
		return p, e
	}
	// Writers lock settings before policies, matching quote readers and avoiding a settings/policy lock inversion.
	if _, e = tx.Exec(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.customer_fulfillment_settings WHERE customer_id=$1 FOR UPDATE`, r.schema), p.CustomerID); e != nil {
		return p, e
	}
	var customerActive bool
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), p.CustomerID).Scan(&customerActive); e != nil {
		return p, e
	}
	if !customerActive {
		return p, fmt.Errorf("客户不存在或已停用")
	}
	old, e := r.LoadPolicy(ctx, tx, p.CustomerID, p.Scene, true)
	if e != nil {
		return p, e
	}
	if old.Revision != expected {
		return p, fmt.Errorf("运费政策已变化，请刷新")
	}
	if p.TemplateID > 0 {
		t, e := r.LoadTemplate(ctx, tx, p.TemplateID, true)
		if e != nil || !t.Active {
			return p, fmt.Errorf("运费模板不存在或停用")
		}
	}
	p.Revision = old.Revision + 1
	_, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_shipping_policies(customer_id,scene,revision,template_id,surcharge_cents,free_shipping,updated_by)VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(customer_id,scene) DO UPDATE SET revision=EXCLUDED.revision,template_id=EXCLUDED.template_id,surcharge_cents=EXCLUDED.surcharge_cents,free_shipping=EXCLUDED.free_shipping,updated_by=EXCLUDED.updated_by,updated_at=now()`, r.schema), p.CustomerID, p.Scene, p.Revision, p.TemplateID, p.SurchargeCents, p.FreeShipping, actor)
	if e != nil {
		return p, e
	}
	if e = r.Audit(ctx, tx, actor, "shipping_policy", p.CustomerID, "save", old, p); e != nil {
		return p, e
	}
	return p, nil
}
func (r *Repository) QuoteTx(ctx context.Context, tx pgx.Tx, id int64, scene string, d domain.Destination, grams int64) (domain.Snapshot, error) {
	p, e := r.ResolvePolicy(ctx, tx, id, scene, true)
	if e != nil {
		return domain.Snapshot{}, e
	}
	t, e := r.LoadTemplate(ctx, tx, p.TemplateID, true)
	if e != nil && !p.FreeShipping {
		return domain.Snapshot{}, fmt.Errorf("请配置代发、客户档案或系统默认运费模板")
	}
	return domain.Quote(p, t, d, grams)
}
func (r *Repository) SaveSnapshotTx(ctx context.Context, tx pgx.Tx, id int64, q domain.Snapshot, actor string) error {
	if (q.Protocol != domain.Protocol && q.Protocol != domain.OrderProtocol) || q.Token != domain.Token(q) || q.CurrentReceivableCents < 0 || (q.Protocol == domain.Protocol && q.CurrentReceivableCents != 0) {
		return fmt.Errorf("运费报价无效，请刷新")
	}
	var cid int64
	var shipping int64
	if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id,round(COALESCE(shipping_amount,0)*100)::bigint FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&cid, &shipping); e != nil {
		return e
	}
	if cid != q.Policy.CustomerID || shipping != q.CurrentReceivableCents {
		return fmt.Errorf("运费快照客户或订单金额不一致")
	}
	if q.CarrierID > 0 {
		var carrier, product int64
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(logistics_company_id,0),COALESCE(logistics_product_id,0) FROM %s.orders WHERE id=$1`, r.schema), id).Scan(&carrier, &product); e != nil {
			return e
		}
		if carrier > 0 && carrier != q.CarrierID || product > 0 && product != q.ProductID {
			return fmt.Errorf("订单承运商与运费模板不一致，请重新选择模板")
		}
		if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET logistics_company_id=$2,logistics_product_id=$3 WHERE id=$1`, r.schema), id, q.CarrierID, q.ProductID); e != nil {
			return e
		}
	}
	b, _ := json.Marshal(q)
	var old []byte
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, r.schema), id).Scan(&old)
	if e == nil {
		var previous domain.Snapshot
		json.Unmarshal(old, &previous)
		if previous.Token == q.Token {
			return nil
		}
		if q.Protocol != domain.OrderProtocol || q.OrderID != id || q.PreviousToken != previous.Token {
			return fmt.Errorf("订单运费已冻结或版本过期")
		}
		if e = r.EditableTx(ctx, tx, id); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_freight_snapshots SET snapshot_json=$2,created_by=$3 WHERE order_id=$1`, r.schema), id, b, actor); e != nil {
			return e
		}
		return r.Audit(ctx, tx, actor, "order_freight", id, "adjust", previous, q)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_freight_snapshots(order_id,customer_id,snapshot_json,created_by)VALUES($1,$2,$3,$4)`, r.schema), id, cid, b, actor); e != nil {
		return e
	}
	return r.Audit(ctx, tx, actor, "order_freight", id, "freeze", nil, q)
}
func (r *Repository) Audit(ctx context.Context, tx pgx.Tx, actor, entity string, id int64, action string, before, after any) error {
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	requirement := "PR-705"
	if strings.HasPrefix(entity, "shipping_return") {
		requirement = "PR-717"
	}
	if entity == "system_shipping_policy" {
		requirement = "PR-716"
	}
	if snapshot, ok := after.(domain.Snapshot); ok && snapshot.Protocol == domain.OrderProtocol {
		requirement = "PR-709"
	}
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, entity, &id, action, infra.StrPtr("freight"), infra.StrPtr(string(a)), infra.StrPtr(string(b)), infra.AuditMeta{"requirement": requirement})
}
func (r *Repository) Templates(ctx context.Context) ([]domain.Template, error) {
	rows, e := r.pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.shipping_fee_templates ORDER BY active DESC,name,id`, r.schema))
	if e != nil {
		return nil, e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []domain.Template{}
	for _, id := range ids {
		t, e := r.LoadTemplate(ctx, r.pool, id, false)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, nil
}
func (r *Repository) SaveTemplate(ctx context.Context, t domain.Template, expected int64, actor string) (domain.Template, error) {
	if e := domain.ValidateTemplate(t); e != nil {
		return t, e
	}
	if actor == "" {
		return t, fmt.Errorf("操作人必填")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return t, e
	}
	defer tx.Rollback(ctx)
	if t.CarrierID > 0 || t.ProductID > 0 {
		var ok bool
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.logistics_products p JOIN %[1]s.logistics_companies c ON c.id=p.company_id WHERE c.id=$1 AND p.id=$2 AND c.active=true AND p.active=true)`, r.schema), t.CarrierID, t.ProductID).Scan(&ok)
		if e != nil {
			return t, e
		}
		if !ok {
			return t, fmt.Errorf("物流公司或物流产品无效")
		}
	}
	old := domain.Template{}
	if t.ID > 0 {
		old, e = r.LoadTemplate(ctx, tx, t.ID, false)
		if e != nil {
			return t, e
		}
		var rev int64
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT revision FROM %s.shipping_fee_templates WHERE id=$1 FOR UPDATE`, r.schema), t.ID).Scan(&rev)
		if e != nil {
			return t, e
		}
		if rev != expected {
			return t, fmt.Errorf("运费模板已变化，请刷新")
		}
	}
	rules, _ := json.Marshal(t.Rules)
	config, _ := json.Marshal(t)
	if t.ID == 0 {
		if expected != 0 {
			return t, fmt.Errorf("模板版本无效")
		}
		e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_fee_templates(name,active,packaging_weight_g,rules_json,config_json,updated_by)VALUES($1,$2,$3,$4,$5,$6)RETURNING id,revision`, r.schema), t.Name, t.Active, t.PackagingWeightG, rules, config, actor).Scan(&t.ID, &t.Revision)
	} else {
		e = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.shipping_fee_templates SET name=$2,active=$3,revision=revision+1,packaging_weight_g=$4,rules_json=$5,config_json=$6,updated_by=$7,updated_at=now() WHERE id=$1 RETURNING revision`, r.schema), t.ID, t.Name, t.Active, t.PackagingWeightG, rules, config, actor).Scan(&t.Revision)
	}
	if e != nil {
		return t, e
	}
	version, _ := json.Marshal(t)
	if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_fee_template_versions(template_id,revision,snapshot_json,created_by)VALUES($1,$2,$3,$4)`, r.schema), t.ID, t.Revision, version, actor); e != nil {
		return t, e
	}
	if e = r.Audit(ctx, tx, actor, "shipping_fee_template", t.ID, "save", old, t); e != nil {
		return t, e
	}
	return t, tx.Commit(ctx)
}
func (r *Repository) OrderViews(ctx context.Context, q Queryer, ids []int64) (map[int64]*domain.OrderView, error) {
	out := map[int64]*domain.OrderView{}
	if !relationExists(ctx, q, r.schema+".order_freight_snapshots") {
		return out, nil
	}
	rows, e := q.Query(ctx, fmt.Sprintf(`SELECT f.order_id,f.snapshot_json,a.actual_cents,COALESCE(a.billed_cents,0),COALESCE(a.revision,0),COALESCE(service.total,0),COALESCE(service.due,0) FROM %[1]s.order_freight_snapshots f LEFT JOIN LATERAL (SELECT round(sum(fee.amount)*100)::bigint total,round(sum(CASE WHEN bill.status='paid' THEN 0 ELSE fee.amount END)*100)::bigint due FROM %[1]s.customer_fee_items fee LEFT JOIN %[1]s.customer_settlement_batches bill ON bill.id=fee.settlement_batch_id AND bill.customer_id=fee.customer_id WHERE fee.source_type='order' AND fee.source_id=f.order_id AND fee.customer_id=f.customer_id AND fee.fee_type='direct_ship_service') service ON true LEFT JOIN LATERAL (SELECT sum(actual_cents)::bigint actual_cents,sum(billed_cents)::bigint billed_cents,max(revision) revision FROM %[1]s.freight_actual_shipments WHERE order_id=f.order_id) a ON true WHERE f.order_id=ANY($1)`, r.schema), ids)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var id, rev int64
		var raw []byte
		v := &domain.OrderView{}
		if e = rows.Scan(&id, &raw, &v.ActualCostCents, &v.BilledCents, &rev, &v.ServiceFeeCents, &v.ServiceDueCents); e != nil {
			return out, e
		}
		if e = json.Unmarshal(raw, &v.Snapshot); e != nil {
			return out, e
		}
		v.BillingStatus = v.Status
		v.EditToken = v.Token
		v.EditLocked = rev > 0
		if rev > 0 && !v.Policy.FreeShipping && v.ChargeMode != "manual_current" {
			v.BillingStatus = "billed"
		}
		out[id] = v
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return out, e
	}

	actualRows, e := q.Query(ctx, fmt.Sprintf(`SELECT a.order_id,sum(a.actual_cents)::bigint,coalesce((SELECT e.reason FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments x ON x.id=e.shipment_id WHERE x.order_id=a.order_id ORDER BY e.id DESC LIMIT 1),'') FROM %[1]s.freight_actual_shipments a WHERE a.order_id=ANY($1) GROUP BY a.order_id`, r.schema), ids)
	if e != nil {
		return out, e
	}
	actualIDs := []int64{}
	for actualRows.Next() {
		var id, cost int64
		var reason string
		if e = actualRows.Scan(&id, &cost, &reason); e != nil {
			actualRows.Close()
			return out, e
		}
		v := out[id]
		if v == nil {
			v = &domain.OrderView{}
			out[id] = v
		}
		v.ActualCostCents = &cost
		v.EditLocked = true
		v.ActualConfirmationReason = reason
		actualIDs = append(actualIDs, id)
	}
	e = actualRows.Err()
	actualRows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range actualIDs {
		state, err := r.ReceivableState(ctx, q, id)
		if err != nil {
			return out, err
		}
		v := out[id]
		total := state.Current
		v.FinalReceivableCents = &total
		v.ReceivedFreightCents = state.Paid
		v.DueFreightCents = max(0, state.Current-state.Paid)
	}
	return out, nil
}

// Editor previews may display pending contents. Accounting always uses OrderViews.
func (r *Repository) OrderEditorViews(ctx context.Context, q Queryer, ids []int64) (map[int64]*domain.OrderView, error) {
	out, e := r.OrderViews(ctx, q, ids)
	if e != nil {
		return out, e
	}
	pendingRows, e := q.Query(ctx, fmt.Sprintf(`SELECT id,to_jsonb(o)->'confirmation_pending'->'freight' FROM %s.orders o WHERE id=ANY($1) AND to_jsonb(o)->>'confirmation_status'='pending' AND to_jsonb(o)->'confirmation_pending'->'freight' IS NOT NULL AND to_jsonb(o)->'confirmation_pending'->'freight'<>'null'::jsonb`, r.schema), ids)
	if e != nil {
		return out, e
	}
	defer pendingRows.Close()
	for pendingRows.Next() {
		var id int64
		var pending []byte
		if e = pendingRows.Scan(&id, &pending); e != nil {
			return out, e
		}
		var snapshot domain.Snapshot
		if e = json.Unmarshal(pending, &snapshot); e != nil {
			return out, e
		}
		view := out[id]
		if view == nil {
			view = &domain.OrderView{}
		}
		view.Snapshot = snapshot
		view.BillingStatus = snapshot.Status
		out[id] = view
	}
	if e = pendingRows.Err(); e != nil {
		return out, e
	}
	return out, nil
}
func (r *Repository) QuoteWithOverrideTx(ctx context.Context, tx pgx.Tx, id int64, scene string, d domain.Destination, grams int64, o *domain.Override, employee bool) (domain.Snapshot, error) {
	p, e := r.ResolvePolicy(ctx, tx, id, scene, true)
	if e != nil {
		return domain.Snapshot{}, e
	}
	p, e = domain.ApplyOverride(p, o, employee)
	if e != nil {
		return domain.Snapshot{}, e
	}
	t, e := r.LoadTemplate(ctx, tx, p.TemplateID, true)
	if e != nil && !p.FreeShipping {
		return domain.Snapshot{}, fmt.Errorf("请配置代发、客户档案或系统默认运费模板")
	}
	q, e := domain.Quote(p, t, d, grams)
	if e != nil {
		return q, e
	}
	if o != nil {
		q.AdjustmentReason = o.Reason
		q.Token = domain.Token(q)
	}
	return q, nil
}
