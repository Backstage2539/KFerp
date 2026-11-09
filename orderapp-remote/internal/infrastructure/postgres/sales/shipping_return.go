package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	app "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	freight "orderapp/internal/infrastructure/postgres/freight"
	progresspg "orderapp/internal/infrastructure/postgres/orderprogress"
	"strconv"
	"strings"
)

func ensureShippingReturnSchema(ctx context.Context, p *pgxpool.Pool, s string) error {
	_, e := p.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.shipping_return_templates(singleton bool PRIMARY KEY DEFAULT true CHECK(singleton),templates jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS %[1]s.shipping_return_batches(id bigserial PRIMARY KEY,file_key text UNIQUE NOT NULL,filename text NOT NULL,template jsonb NOT NULL,created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.shipping_return_rows(id bigserial PRIMARY KEY,batch_id bigint NOT NULL REFERENCES %[1]s.shipping_return_batches(id),payload jsonb NOT NULL,status text NOT NULL,selected_order_id bigint REFERENCES %[1]s.orders(id),result text NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS %[1]s.shipping_return_candidates(row_id bigint NOT NULL REFERENCES %[1]s.shipping_return_rows(id),order_id bigint NOT NULL REFERENCES %[1]s.orders(id),version text NOT NULL,PRIMARY KEY(row_id,order_id));
 CREATE INDEX IF NOT EXISTS shipping_return_candidates_order ON %[1]s.shipping_return_candidates(order_id);
 `, s))
	return e
}
func (r Repository) ShippingReturnTemplates(ctx context.Context) ([]d.Template, error) {
	var raw []byte
	e := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT templates FROM %s.shipping_return_templates WHERE singleton`, r.schema)).Scan(&raw)
	if e == pgx.ErrNoRows {
		t := d.DefaultTemplate()
		_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.logistics_companies WHERE name LIKE '%%顺丰%%' ORDER BY id LIMIT 1`, r.schema)).Scan(&t.CarrierID)
		return []d.Template{t}, nil
	}
	if e != nil {
		return nil, e
	}
	var out []d.Template
	e = json.Unmarshal(raw, &out)
	if e == nil {
		defaults := d.DefaultTemplate()
		for i := range out {
			if out[i].Columns == nil {
				out[i].Columns = map[string]d.Column{}
			}
			for _, key := range []string{"settlement_weight", "pickup_time"} {
				if _, ok := out[i].Columns[key]; !ok {
					out[i].Columns[key] = defaults.Columns[key]
				}
			}
			if strings.TrimSpace(out[i].WeightUnit) == "" {
				out[i].WeightUnit = defaults.WeightUnit
			}
		}
	}
	return out, e
}
func (r Repository) SaveShippingReturnTemplates(ctx context.Context, actor string, templates []d.Template) error {
	if len(templates) == 0 || len(templates) > 30 {
		return fmt.Errorf("请保留 1–30 个模板")
	}
	seen := map[string]bool{}
	for _, t := range templates {
		if e := t.Validate(); e != nil {
			return e
		}
		if t.CarrierID <= 0 || seen[t.ID] {
			return fmt.Errorf("请选择快递公司，模板编号不能重复")
		}
		seen[t.ID] = true
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('shipping-return-templates'))`); e != nil {
		return e
	}
	var old []byte
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT templates FROM %s.shipping_return_templates WHERE singleton`, r.schema)).Scan(&old)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	var previous []d.Template
	_ = json.Unmarshal(old, &previous)
	revs := map[string]int64{"sf-recipient": 1}
	for _, t := range previous {
		revs[t.ID] = t.Revision
	}
	for _, t := range templates {
		var exists bool
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.logistics_companies WHERE id=$1 AND active)`, r.schema), t.CarrierID).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return fmt.Errorf("请选择有效且启用的快递公司")
		}
	}
	for i := range templates {
		if revs[templates[i].ID] > 0 && templates[i].Revision != revs[templates[i].ID] {
			return fmt.Errorf("模板已更新，请刷新后再保存")
		}
		templates[i].Revision = revs[templates[i].ID] + 1
	}
	raw, _ := json.Marshal(templates)
	_, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_templates(templates)VALUES($1)ON CONFLICT(singleton)DO UPDATE SET templates=excluded.templates`, r.schema), raw)
	if e != nil {
		return e
	}
	if e = freight.NewRepository(r.pool, r.schema).Audit(ctx, tx, actor, "shipping_return_template", 0, "save", json.RawMessage(old), templates); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type returnOrder struct {
	id                   int64
	name, phone, version string
}

func returnOrderSQL(s string) string {
	where := []string{
		"NOT o.is_void",
		"COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted'",
		"COALESCE(to_jsonb(o)->>'delivery_mode','ship')<>'customer_stock'",
		"COALESCE(to_jsonb(o)->>'fulfillment_paused','false')<>'true'",
		"COALESCE(to_jsonb(o)->>'resolution_status','open') NOT IN ('closed','cancelled','canceled')",
		fmt.Sprintf("NOT EXISTS (SELECT 1 FROM %s.ship_statuses ss WHERE ss.id=o.ship_status_id AND (ss.name LIKE '%%已发货%%' OR ss.name IN ('部分发货','已签收','已出库','已收货','已完成')))", s),
	}
	return fmt.Sprintf(`SELECT o.id,CASE WHEN o.portal_service_code IN ('direct_ship','product_order','processing_ship') THEN coalesce(o.receiver_name,'') ELSE coalesce(nullif(o.receiver_name,''),nullif(c.contact,''),c.name,'') END,CASE WHEN o.portal_service_code IN ('direct_ship','product_order','processing_ship') THEN coalesce(o.receiver_phone,'') ELSE coalesce(nullif(o.receiver_phone,''),c.phone,'') END,md5(to_jsonb(o)::text) FROM %[1]s.orders o LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE %[2]s AND coalesce(o.ship_tracking_no,'')='' AND NOT EXISTS(SELECT 1 FROM %[1]s.order_shipping_trackings t WHERE t.order_id=o.id) AND coalesce(o.ship_method,'') NOT IN ('pickup','self_pickup','local_delivery')`, s, strings.Join(where, " AND "))
}
func (r Repository) ImportShippingReturn(ctx context.Context, actor, name, hash string, t d.Template, items []d.Row) (d.Batch, error) {
	var b d.Batch
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return b, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`); e != nil {
		return b, e
	}
	key := fmt.Sprintf("%s:%s:%d", hash, t.ID, t.Revision)
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.shipping_return_batches WHERE file_key=$1`, r.schema), key).Scan(&b.ID)
	if e == nil {
		backfilled, skipped, err := r.backfillReturnWeightsTx(ctx, tx, actor, b.ID, items)
		if err != nil {
			return b, err
		}
		if err = r.rematchShippingReturnTx(ctx, tx, actor, b.ID, nil); err != nil {
			return b, err
		}
		if err = tx.Commit(ctx); err != nil {
			return b, err
		}
		out, err := r.returnBatch(ctx, b.ID)
		out.Rematched, out.OrderIDs = true, nil
		out.WeightBackfilled, out.WeightSkipped = backfilled, skipped
		return out, err
	}
	if e != pgx.ErrNoRows {
		return b, e
	}
	var differentTemplate bool
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.shipping_return_batches WHERE split_part(file_key, ':', 1)=$1 AND file_key<>$2)`, r.schema), hash, key).Scan(&differentTemplate); e != nil {
		return b, e
	}
	if differentTemplate {
		return b, fmt.Errorf("该文件已使用其他模板或版本导入，请使用原模板版本；未重复发货或收费")
	}
	raw, _ := json.Marshal(t)
	e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_batches(file_key,filename,template,created_by)VALUES($1,$2,$3,$4)RETURNING id`, r.schema), key, name, raw, actor).Scan(&b.ID)
	if e != nil {
		return b, e
	}
	orders, e := tx.Query(ctx, returnOrderSQL(r.schema))
	if e != nil {
		return b, e
	}
	targets := map[string][]returnOrder{}
	for orders.Next() {
		var o returnOrder
		if e = orders.Scan(&o.id, &o.name, &o.phone, &o.version); e != nil {
			orders.Close()
			return b, e
		}
		key := d.PhoneMatchKey(o.phone)
		if key != "" {
			targets[key] = append(targets[key], o)
		}
	}
	e = orders.Err()
	orders.Close()
	if e != nil {
		return b, e
	}
	// Duplicate waybills collapse only if their recipient and cost agree; conflicts never auto-apply.
	first := map[string]int{}
	for i := range items {
		v := &items[i]
		if v.Error != "" {
			continue
		}
		if j, ok := first[v.TrackingNo]; ok {
			other := &items[j]
			if other.ActualCents != v.ActualCents || other.Receiver != v.Receiver || other.Phone != v.Phone {
				other.Error = "同运单记录冲突"
				other.Status = "skipped"
				v.Error = other.Error
			} else {
				v.Error = "重复运单"
			}
			v.Status = "skipped"
		} else {
			first[v.TrackingNo] = i
		}
	}
	for _, v := range items {
		raw, _ = json.Marshal(v)
		var rid int64
		e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_rows(batch_id,payload,status)VALUES($1,$2,$3)RETURNING id`, r.schema), b.ID, raw, v.Status).Scan(&rid)
		if e != nil {
			return b, e
		}
		if v.Error != "" {
			continue
		}
		for _, o := range targets[d.PhoneMatchKey(v.Phone)] {
			if d.RecipientMatches(v.Receiver, v.Phone, o.name, o.phone) {
				_, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_candidates(row_id,order_id,version)VALUES($1,$2,$3)`, r.schema), rid, o.id, o.version)
				if e != nil {
					return b, e
				}
			}
		}
	}
	if e = freight.NewRepository(r.pool, r.schema).Audit(ctx, tx, actor, "shipping_return", b.ID, "import", nil, map[string]any{"file": name, "hash": hash, "rows": len(items)}); e != nil {
		return b, e
	}
	if e = tx.Commit(ctx); e != nil {
		return b, e
	}
	// Only mutually unique matches can be assigned automatically, across all persisted pending batches.
	matches, e := r.pool.Query(ctx, fmt.Sprintf(`SELECT c.row_id,c.order_id,c.version FROM %[1]s.shipping_return_candidates c JOIN %[1]s.shipping_return_rows r ON r.id=c.row_id WHERE r.batch_id=$1 AND r.status='pending' AND (SELECT count(*) FROM %[1]s.shipping_return_candidates x WHERE x.row_id=c.row_id)=1 AND (SELECT count(*) FROM %[1]s.shipping_return_candidates x JOIN %[1]s.shipping_return_rows y ON y.id=x.row_id WHERE x.order_id=c.order_id AND y.status='pending')=1`, r.schema), b.ID)
	if e != nil {
		return b, e
	}
	type selection struct {
		rid, oid int64
		version  string
	}
	auto := []selection{}
	for matches.Next() {
		var a selection
		if e = matches.Scan(&a.rid, &a.oid, &a.version); e != nil {
			matches.Close()
			return b, e
		}
		auto = append(auto, a)
	}
	e = matches.Err()
	matches.Close()
	if e != nil {
		return b, e
	}
	for _, a := range auto {
		_, err := r.selectShippingReturn(ctx, actor, a.oid, a.rid, a.version, true)
		if err != nil {
			_, e = r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET result=$2 WHERE id=$1`, r.schema), a.rid, err.Error())
			if e != nil {
				return b, e
			}
		}
	}
	return r.returnBatch(ctx, b.ID)
}

// A repeated upload uses the same hash, template ID and revision. Only fill
// missing metadata on uniquely matching file rows; never rerun shipment/fees.
func (r Repository) backfillReturnWeightsTx(ctx context.Context, tx pgx.Tx, actor string, batchID int64, incoming []d.Row) (int, int, error) {
	type existingRow struct {
		id     int64
		status string
		row    d.Row
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,status,payload FROM %s.shipping_return_rows WHERE batch_id=$1 ORDER BY id FOR UPDATE`, r.schema), batchID)
	if err != nil {
		return 0, 0, err
	}
	stored := []existingRow{}
	storedRows, storedTracks := map[int]int{}, map[string]int{}
	for rows.Next() {
		var v existingRow
		var raw []byte
		if err = rows.Scan(&v.id, &v.status, &raw); err == nil {
			err = json.Unmarshal(raw, &v.row)
		}
		if err != nil {
			rows.Close()
			return 0, 0, err
		}
		stored = append(stored, v)
		storedRows[v.row.RowNo]++
		storedTracks[v.row.TrackingNo]++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, err
	}
	newRows, newTracks := map[int]int{}, map[string]int{}
	byRow := map[int]d.Row{}
	for _, v := range incoming {
		newRows[v.RowNo]++
		newTracks[v.TrackingNo]++
		byRow[v.RowNo] = v
	}
	changes := []map[string]any{}
	skipped := 0
	for _, old := range stored {
		v, found := byRow[old.row.RowNo]
		if !found {
			if old.row.SettlementWeightKG == "" && (old.status == "applied" || old.status == "pending") {
				skipped++
			}
			continue
		}
		if strings.TrimSpace(v.SettlementWeightKG) == "" {
			if v.Error != "" {
				skipped++
			}
			continue
		}
		weight, parseErr := strconv.ParseFloat(v.SettlementWeightKG, 64)
		valid := parseErr == nil && !math.IsNaN(weight) && !math.IsInf(weight, 0) && weight >= 0 && weight <= 1000
		if !valid || v.Error != "" || v.Status != "pending" || (old.status != "applied" && old.status != "pending") || old.row.Error != "" || v.RowNo <= 1 || v.TrackingNo == "" || v.TrackingNo != old.row.TrackingNo || v.ActualCents != old.row.ActualCents || newRows[v.RowNo] != 1 || newTracks[v.TrackingNo] != 1 || storedRows[old.row.RowNo] != 1 || storedTracks[old.row.TrackingNo] != 1 {
			skipped++
			continue
		}
		if old.row.SettlementWeightKG != "" {
			if old.row.SettlementWeightKG != v.SettlementWeightKG {
				skipped++
			}
			continue
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET payload=jsonb_set(payload,'{settlement_weight_kg}',to_jsonb($2::text),true) WHERE id=$1`, r.schema), old.id, v.SettlementWeightKG); err != nil {
			return 0, 0, err
		}
		changes = append(changes, map[string]any{"row_id": old.id, "row_no": v.RowNo, "tracking_no": v.TrackingNo, "weight_kg": v.SettlementWeightKG})
	}
	if len(changes) > 0 {
		if err = freight.NewRepository(r.pool, r.schema).Audit(ctx, tx, actor, "shipping_return", batchID, "backfill_settlement_weight", nil, map[string]any{"rows": changes, "skipped": skipped}); err != nil {
			return 0, 0, err
		}
	}
	return len(changes), skipped, nil
}
func (r Repository) returnBatch(ctx context.Context, id int64) (d.Batch, error) {
	b := d.Batch{ID: id, Rows: []d.Row{}}
	rows, e := r.pool.Query(ctx, fmt.Sprintf(`SELECT r.id,r.payload,r.status,coalesce(r.selected_order_id,0),r.result,(SELECT count(*) FROM %[1]s.shipping_return_candidates c WHERE c.row_id=r.id) FROM %[1]s.shipping_return_rows r WHERE batch_id=$1 ORDER BY id`, r.schema), id)
	if e != nil {
		return b, e
	}
	defer rows.Close()
	for rows.Next() {
		var v d.Row
		var raw []byte
		var reason, status string
		var oid, rid, n int64
		if e = rows.Scan(&rid, &raw, &status, &oid, &reason, &n); e != nil {
			return b, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return b, e
		}
		v.ID = rid
		v.Status = status
		v.OrderID = oid
		if reason != "" {
			v.Error = reason
		}
		switch {
		case status == "applied":
			b.Updated++
			b.OrderIDs = append(b.OrderIDs, oid)
		case status == "pending" && n > 0:
			b.Pending++
		case status == "pending":
			v.Status = "unmatched"
			if v.Error == "" {
				v.Error = "未匹配待回填订单"
			}
			b.Skipped++
		default:
			b.Skipped++
		}
		b.Rows = append(b.Rows, v)
	}
	b.Total = len(b.Rows)
	return b, rows.Err()
}
func (r Repository) ShippingReturnCandidates(ctx context.Context, oid int64) ([]d.Row, error) {
	return r.liveShippingReturnCandidates(ctx, r.pool, oid)
}

// Read from current orders, including matches whose inventory is not yet ready.
// A GET neither reserves stock nor persists stale recipient-match snapshots.
func (r Repository) liveShippingReturnCandidates(ctx context.Context, q progresspg.Querier, oid int64) ([]d.Row, error) {
	query := returnOrderSQL(r.schema)
	args := []any{}
	if oid > 0 {
		query += " AND o.id=$1"
		args = append(args, oid)
	}
	orders, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	targets := map[string][]returnOrder{}
	ids := []int64{}
	for orders.Next() {
		var o returnOrder
		if err = orders.Scan(&o.id, &o.name, &o.phone, &o.version); err != nil {
			orders.Close()
			return nil, err
		}
		if key := d.PhoneMatchKey(o.phone); key != "" {
			targets[key] = append(targets[key], o)
			ids = append(ids, o.id)
		}
	}
	err = orders.Err()
	orders.Close()
	if err != nil {
		return nil, err
	}
	progress, err := progresspg.Load(ctx, q, r.schema, ids)
	if err != nil {
		return nil, err
	}
	dates := map[int64][2]string{}
	meta, err := q.Query(ctx, fmt.Sprintf(`SELECT id,order_no,coalesce(to_char(order_date,'YYYY-MM-DD'),'') FROM %s.orders WHERE id=ANY($1)`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	for meta.Next() {
		var id int64
		var no, date string
		if err = meta.Scan(&id, &no, &date); err != nil {
			meta.Close()
			return nil, err
		}
		dates[id] = [2]string{no, date}
	}
	err = meta.Err()
	meta.Close()
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT r.id,r.payload FROM %[1]s.shipping_return_rows r
      WHERE r.status='pending' AND NOT EXISTS(SELECT 1 FROM %[1]s.orders o WHERE o.ship_tracking_no=r.payload->>'tracking_no'
        UNION ALL SELECT 1 FROM %[1]s.order_shipping_trackings t WHERE t.tracking_no=r.payload->>'tracking_no') ORDER BY r.id DESC`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []d.Row{}
	for rows.Next() {
		var item d.Row
		var raw []byte
		var id int64
		if err = rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		if item.Error != "" {
			continue
		}
		for _, o := range targets[d.PhoneMatchKey(item.Phone)] {
			if !d.RecipientMatches(item.Receiver, item.Phone, o.name, o.phone) {
				continue
			}
			v := item
			v.ID, v.OrderID, v.Version = id, o.id, o.version
			v.OrderNo, v.OrderDate = dates[o.id][0], dates[o.id][1]
			v.BlockReason = "订单尚未满足发货条件"
			if p := progress[o.id]; p != nil {
				v.CanSelect = p.CanShip
				v.BlockReason = p.BlockReason
			}
			out = append(out, v)
		}
	}
	return out, rows.Err()
}
func (r Repository) SelectShippingReturn(ctx context.Context, actor string, oid, rid int64, version string) (app.FillShipmentTrackingResult, error) {
	return r.selectShippingReturn(ctx, actor, oid, rid, version, false)
}
func (r Repository) selectShippingReturn(ctx context.Context, actor string, oid, rid int64, version string, automatic bool) (app.FillShipmentTrackingResult, error) {
	out := app.FillShipmentTrackingResult{}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`); e != nil {
		return out, e
	}
	var raw, cfg []byte
	var status string
	var selected int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT r.payload,b.template,r.status,coalesce(r.selected_order_id,0) FROM %[1]s.shipping_return_rows r JOIN %[1]s.shipping_return_batches b ON b.id=r.batch_id WHERE r.id=$1 FOR UPDATE OF r`, r.schema), rid).Scan(&raw, &cfg, &status, &selected)
	if e != nil {
		return out, e
	}
	if status == "applied" && selected == oid {
		return out, nil
	}
	if status != "pending" {
		return out, fmt.Errorf("快递单已被使用")
	}
	// Import and selection share this lock. Recheck uniqueness inside it because
	// another uploaded batch may have introduced candidates after the initial scan.
	if automatic {
		matches, err := r.liveShippingReturnCandidates(ctx, tx, 0)
		if err != nil {
			return out, err
		}
		rowCount, orderCount := 0, 0
		for _, c := range matches {
			if c.ID == rid {
				rowCount++
			}
			if c.OrderID == oid {
				orderCount++
			}
		}
		if rowCount != 1 || orderCount != 1 {
			return out, fmt.Errorf("出现多个匹配，请选择快递单")
		}
	}
	var item d.Row
	var t d.Template
	if e = json.Unmarshal(raw, &item); e != nil {
		return out, e
	}
	if e = json.Unmarshal(cfg, &t); e != nil {
		return out, e
	}
	var current string
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT md5(to_jsonb(o)::text) FROM %s.orders o WHERE id=$1 FOR UPDATE`, r.schema), oid).Scan(&current)
	if e != nil {
		return out, e
	}
	if current != version {
		return out, fmt.Errorf("订单已变化，请刷新后重新选择快递单")
	}
	var eligible returnOrder
	e = tx.QueryRow(ctx, returnOrderSQL(r.schema)+" AND o.id=$1", oid).Scan(&eligible.id, &eligible.name, &eligible.phone, &eligible.version)
	if e != nil {
		return out, fmt.Errorf("订单不再符合回填条件")
	}
	progress, err := progresspg.Load(ctx, tx, r.schema, []int64{oid})
	if err != nil {
		return out, err
	}
	if progress[oid] == nil || !progress[oid].CanShip {
		return out, fmt.Errorf("本单实物库存尚未全部分配或履约状态不允许发货，请先处理订单库存后刷新")
	}
	if !d.RecipientMatches(item.Receiver, item.Phone, eligible.name, eligible.phone) {
		return out, fmt.Errorf("收件信息已变化")
	}
	if t.CarrierID <= 0 {
		return out, fmt.Errorf("请在物流回传模板中选择快递公司")
	}
	var occupied bool
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.orders o LEFT JOIN %[1]s.order_shipping_trackings x ON x.order_id=o.id WHERE o.id<>$1 AND (x.tracking_no=$2 OR o.ship_tracking_no=$2))`, r.schema), oid, item.TrackingNo).Scan(&occupied)
	if e != nil {
		return out, e
	}
	if occupied {
		return out, fmt.Errorf("该运单已绑定其他订单，请核对")
	}
	if _, e = appendOrderTrackingNumbersTx(ctx, tx, r.schema, oid, item.TrackingNo, "shipping_return", actor); e != nil {
		return out, e
	}
	shipped := lookupDefaultStatusID(ctx, tx, r.schema, "ship_statuses", "已发货")
	if shipped <= 0 {
		return out, fmt.Errorf("未配置已发货状态")
	}
	_, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=$2,logistics_company_id=$3 WHERE id=$1`, r.schema), oid, shipped, t.CarrierID)
	if e != nil {
		return out, e
	}
	reason, e := freight.NewRepository(r.pool, r.schema).ConfirmActualCostTx(ctx, tx, oid, t.CarrierID, item.TrackingNo, item.ActualCents, fmt.Sprintf("shipping-return:%d", rid), actor)
	if e != nil {
		return out, e
	}
	_, e = tx.Exec(ctx, fmt.Sprintf(`WITH changed AS(UPDATE %[1]s.order_shipment_orders SET tracking_no=$2,shipped_at=now() WHERE id=(SELECT id FROM %[1]s.order_shipment_orders WHERE order_id=$1 ORDER BY id DESC LIMIT 1)RETURNING shipment_id) UPDATE %[1]s.order_shipments SET status='shipped' WHERE id IN(SELECT shipment_id FROM changed)`, r.schema), oid, item.TrackingNo)
	if e != nil {
		return out, e
	}
	if e = r.deductOrderAllocatedStockTx(ctx, tx, oid, actor); e != nil {
		return out, e
	}
	if e = r.insertShippingAuditTx(ctx, tx, actor, oid, "shipping_return", item.TrackingNo+" · "+reason); e != nil {
		return out, e
	}
	_, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET status='applied',selected_order_id=$2,result=$3 WHERE id=$1`, r.schema), rid, oid, reason)
	if e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	out.Updated = 1
	out.Total = 1
	out.OrderIDs = []int64{oid}
	return out, nil
}
