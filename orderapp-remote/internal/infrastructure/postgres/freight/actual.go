package freight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/freight"
	"strings"
)

type ImportRow struct {
	RowNo          int    `json:"row_no"`
	TrackingNo     string `json:"tracking_no"`
	ActualCents    int64  `json:"actual_cents"`
	OrderID        int64  `json:"order_id"`
	OrderNo        string `json:"order_no"`
	CustomerID     int64  `json:"customer_id"`
	CustomerName   string `json:"customer_name"`
	SurchargeCents int64  `json:"surcharge_cents"`
	TargetCents    int64  `json:"target_cents"`
	DeltaCents     int64  `json:"delta_cents"`
	FreeShipping   bool   `json:"free_shipping"`
	Legacy         bool   `json:"legacy"`
	Revision       int64  `json:"revision"`
	SnapshotToken  string `json:"snapshot_token"`
	Error          string `json:"error"`
	EntryID        int64  `json:"entry_id"`
}
type ImportBatch struct {
	ID        int64       `json:"id"`
	CarrierID int64       `json:"carrier_id"`
	FileHash  string      `json:"file_hash"`
	Filename  string      `json:"filename"`
	Status    string      `json:"status"`
	Rows      []ImportRow `json:"rows"`
}

func (r *Repository) Batch(ctx context.Context, id int64) (ImportBatch, error) {
	return r.loadBatch(ctx, r.pool, id, false)
}
func (r *Repository) loadBatch(ctx context.Context, q Queryer, id int64, lock bool) (ImportBatch, error) {
	var b ImportBatch
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	e := q.QueryRow(ctx, fmt.Sprintf(`SELECT id,carrier_id,file_hash,filename,status,rows_json FROM %s.freight_import_batches WHERE id=$1%s`, r.schema, suffix), id).Scan(&b.ID, &b.CarrierID, &b.FileHash, &b.Filename, &b.Status, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &b.Rows)
	}
	return b, e
}
func (r *Repository) PreviewImport(ctx context.Context, carrier int64, hash, name string, input []ImportRow, actor string) (ImportBatch, error) {
	b := ImportBatch{CarrierID: carrier, FileHash: hash, Filename: name, Status: "preview", Rows: input}
	if carrier <= 0 || hash == "" || actor == "" || len(input) == 0 || len(input) > 10000 {
		return b, fmt.Errorf("请选择承运商及有效账单（最多10000行）")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return b, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("freight-file:%d:%s", carrier, hash)); e != nil {
		return b, e
	}
	var id int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.freight_import_batches WHERE carrier_id=$1 AND file_hash=$2`, r.schema), carrier, hash).Scan(&id)
	if e == nil {
		old, err := r.loadBatch(ctx, tx, id, false)
		if err != nil {
			return b, err
		}
		if old.Status == "confirmed" {
			return old, nil
		}
		b.ID = old.ID
		processed := map[int]ImportRow{}
		for _, v := range old.Rows {
			if v.EntryID > 0 {
				processed[v.RowNo] = v
			}
		}
		for i := range b.Rows {
			if v, ok := processed[b.Rows[i].RowNo]; ok {
				b.Rows[i] = v
			}
		}
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return b, e
	}
	seen := map[string]int{}
	conflicts := map[string]bool{}
	for i := range b.Rows {
		v := &b.Rows[i]
		if v.EntryID > 0 {
			continue
		}
		v.TrackingNo = strings.TrimSpace(v.TrackingNo)
		if j, ok := seen[v.TrackingNo]; ok {
			if v.ActualCents != b.Rows[j].ActualCents {
				conflicts[v.TrackingNo] = true
			}
			v.Error = "同文件重复运单，请合并为每票最终实际运费"
			b.Rows[j].Error = v.Error
		} else {
			seen[v.TrackingNo] = i
		}
	}
	for i := range b.Rows {
		v := &b.Rows[i]
		if v.EntryID > 0 {
			continue
		}
		if conflicts[v.TrackingNo] {
			v.Error = "同运单实际金额冲突，请核对"
		}
		if v.Error != "" {
			continue
		}
		resolved, e := r.resolveRow(ctx, tx, carrier, *v, false)
		if e != nil {
			return b, e
		}
		b.Rows[i] = resolved
	}
	raw, _ := json.Marshal(b.Rows)
	if b.ID > 0 {
		_, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.freight_import_batches SET rows_json=$2,created_by=$3 WHERE id=$1`, r.schema), b.ID, raw, actor)
	} else {
		e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_import_batches(carrier_id,file_hash,filename,rows_json,created_by)VALUES($1,$2,$3,$4,$5)RETURNING id`, r.schema), carrier, hash, name, raw, actor).Scan(&b.ID)
	}
	if e != nil {
		return b, e
	}
	if e = r.Audit(ctx, tx, actor, "freight_import", b.ID, "preview", nil, b); e != nil {
		return b, e
	}
	return b, tx.Commit(ctx)
}
func (r *Repository) resolveRow(ctx context.Context, tx pgx.Tx, carrier int64, v ImportRow, lock bool) (ImportRow, error) {
	v.OrderID = 0
	v.Error = ""
	if v.TrackingNo == "" || len(v.TrackingNo) > 100 || strings.ContainsAny(v.TrackingNo, "\n\r\t,，;；") || v.ActualCents < 0 || v.ActualCents > domain.MaxCents {
		v.Error = "运单号或实际金额无效"
		return v, nil
	}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT o.id FROM %[1]s.orders o LEFT JOIN %[1]s.order_shipping_trackings t ON t.order_id=o.id WHERE COALESCE(o.logistics_company_id,0)=$1 AND (trim(o.ship_tracking_no)=$2 OR t.tracking_no=$2) ORDER BY o.id`, r.schema), carrier, v.TrackingNo)
	if e != nil {
		return v, e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return v, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	if len(ids) != 1 {
		v.Error = "未匹配订单"
		if len(ids) > 1 {
			v.Error = "运单匹配多个订单，请人工核对"
		}
		return v, nil
	}
	var void bool
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF o"
	}
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,o.order_no,o.customer_id,c.name,COALESCE(o.is_void,false) FROM %[1]s.orders o JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=$1%s`, r.schema, suffix), ids[0]).Scan(&v.OrderID, &v.OrderNo, &v.CustomerID, &v.CustomerName, &void)
	if e != nil {
		return v, e
	}
	if void {
		v.Error = "订单已作废，只可留待核对"
		return v, nil
	}
	var trackingCount int
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(DISTINCT no) FROM (SELECT tracking_no AS no FROM %[1]s.order_shipping_trackings WHERE order_id=$1 UNION SELECT ship_tracking_no FROM %[1]s.orders WHERE id=$1) x WHERE trim(no)<>''`, r.schema), v.OrderID).Scan(&trackingCount)
	if e != nil {
		return v, e
	}
	if trackingCount != 1 {
		v.Error = "订单包含多票运单，整单一票规则无法自动分摊"
		return v, nil
	}
	var raw []byte
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, r.schema), v.OrderID).Scan(&raw)
	var q domain.Snapshot
	if errors.Is(e, pgx.ErrNoRows) {
		v.Legacy = true
	} else if e != nil {
		return v, e
	} else if e = json.Unmarshal(raw, &q); e != nil {
		return v, e
	}
	var billed, actual, shipmentID, shipmentOrderID, shipmentCustomerID int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,actual_cents,billed_cents,revision,order_id,customer_id FROM %s.freight_actual_shipments WHERE carrier_id=$1 AND tracking_no=$2`, r.schema), carrier, v.TrackingNo).Scan(&shipmentID, &actual, &billed, &v.Revision, &shipmentOrderID, &shipmentCustomerID)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return v, e
	}
	if shipmentID > 0 {
		if shipmentOrderID != v.OrderID || shipmentCustomerID != v.CustomerID {
			v.Error = "运单已出账关联其他订单或客户，禁止重新归属"
			return v, nil
		}
		if lock {
			if err := r.lockShipmentFees(ctx, tx, shipmentID); err != nil {
				return v, err
			}
		}
		var settled bool
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customer_fee_items f JOIN %[1]s.freight_actual_entries e ON f.source_type='freight_actual' AND f.source_id=e.id WHERE e.shipment_id=$1 AND (COALESCE(f.settlement_batch_id,0)>0 OR COALESCE(f.status,'unsettled')<>'unsettled'))`, r.schema), shipmentID).Scan(&settled)
		if e != nil {
			return v, e
		}
		if settled && actual != v.ActualCents {
			v.Error = "已有结算或关账，需先处理原结算后调整"
			return v, nil
		}
	}
	if v.Legacy {
		v.SnapshotToken = "legacy-cost-only"
		v.TargetCents = 0
		v.DeltaCents = 0
		return v, nil
	}
	if q.CarrierID > 0 && q.CarrierID != carrier {
		v.Error = "承运商与下单冻结模板不一致"
		return v, nil
	}
	a, e := domain.Settlement(q, v.ActualCents, billed)
	if e != nil {
		v.Error = e.Error()
		return v, nil
	}
	v.SnapshotToken = q.Token
	v.FreeShipping = q.Policy.FreeShipping
	v.SurchargeCents = a.SurchargeCents
	v.TargetCents = a.TargetCents
	v.DeltaCents = a.DeltaCents
	return v, nil
}
func (r *Repository) ConfirmImport(ctx context.Context, id int64, actor string) (ImportBatch, error) {
	if actor == "" {
		return ImportBatch{}, fmt.Errorf("操作人必填")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return ImportBatch{}, e
	}
	defer tx.Rollback(ctx)
	// Serialize actual shipment changes before order/fee locks, including different files for the same waybill.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`); e != nil {
		return ImportBatch{}, e
	}
	b, e := r.loadBatch(ctx, tx, id, true)
	if e != nil {
		return b, e
	}
	if b.Status == "confirmed" {
		return b, nil
	}
	posted := 0
	for i, old := range b.Rows {
		if old.Error != "" || old.EntryID > 0 {
			continue
		}
		v, e := r.resolveRow(ctx, tx, b.CarrierID, old, true)
		if e != nil {
			return b, e
		}
		if v.Error != "" {
			return b, fmt.Errorf("第%d行：%s，请重新导入预览", v.RowNo, v.Error)
		}
		if v.OrderID != old.OrderID || v.Revision != old.Revision || v.SnapshotToken != old.SnapshotToken || v.TargetCents != old.TargetCents || v.DeltaCents != old.DeltaCents {
			return b, fmt.Errorf("第%d行报价或账单已变化，请重新解析预览", v.RowNo)
		}
		var sid, prevActual, prevBilled int64
		e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(carrier_id,tracking_no,order_id,customer_id,legacy)VALUES($1,$2,$3,$4,$5) ON CONFLICT(carrier_id,tracking_no) DO UPDATE SET tracking_no=EXCLUDED.tracking_no RETURNING id,actual_cents,billed_cents`, r.schema), b.CarrierID, v.TrackingNo, v.OrderID, v.CustomerID, v.Legacy).Scan(&sid, &prevActual, &prevBilled)
		if e != nil {
			return b, e
		}
		key := fmt.Sprintf("import:%d:%s", b.ID, v.TrackingNo)
		e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(shipment_id,batch_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by)VALUES($1,$2,$3,$4,$5,$6,$7,$8,'账单确认',$9)RETURNING id`, r.schema), sid, b.ID, key, v.ActualCents, v.TargetCents, v.DeltaCents, prevActual, prevBilled, actor).Scan(&v.EntryID)
		if e != nil {
			return b, e
		}
		if v.DeltaCents != 0 {
			if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,note)VALUES($1,'freight_actual',$2,'shipping',$3,$4)`, r.schema), v.CustomerID, v.EntryID, float64(v.DeltaCents)/100, "实际运费 · "+v.OrderNo+" · "+v.TrackingNo); e != nil {
				return b, e
			}
		}
		if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.freight_actual_shipments SET actual_cents=$2,billed_cents=$3,revision=revision+1 WHERE id=$1`, r.schema), sid, v.ActualCents, v.TargetCents); e != nil {
			return b, e
		}
		b.Rows[i] = v
		posted++
	}
	if posted == 0 {
		return b, fmt.Errorf("没有可确认的行，请处理未匹配或冲突记录")
	}
	b.Status = "confirmed"
	for _, v := range b.Rows {
		if v.EntryID == 0 {
			b.Status = "partial"
			break
		}
	}
	raw, _ := json.Marshal(b.Rows)
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.freight_import_batches SET status=$3,rows_json=$2 WHERE id=$1`, r.schema), b.ID, raw, b.Status); e != nil {
		return b, e
	}
	if e = r.Audit(ctx, tx, actor, "freight_import", b.ID, "confirm", nil, b); e != nil {
		return b, e
	}
	return b, tx.Commit(ctx)
}
func (r *Repository) ReverseEntry(ctx context.Context, id int64, key, reason, actor string) error {
	if id <= 0 || strings.TrimSpace(key) == "" || strings.TrimSpace(reason) == "" || actor == "" {
		return fmt.Errorf("请选择来源并填写撤销原因")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`); e != nil {
		return e
	}
	var sid, cid, previousActual, previousBilled, actual, billed, latest int64
	var oldKey string
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_key FROM %s.freight_actual_entries WHERE request_key=$1`, r.schema), "reverse:"+key).Scan(&oldKey)
	if e == nil {
		return nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.shipment_id,s.customer_id,e.previous_actual_cents,e.previous_billed_cents,s.actual_cents,s.billed_cents,(SELECT max(id) FROM %[1]s.freight_actual_entries WHERE shipment_id=s.id) FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments s ON s.id=e.shipment_id WHERE e.id=$1 FOR UPDATE OF s`, r.schema), id).Scan(&sid, &cid, &previousActual, &previousBilled, &actual, &billed, &latest)
	if e != nil {
		return e
	}
	if latest != id {
		return fmt.Errorf("只能撤销该运单的最新记录，请先撤销后续修订")
	}
	if e = r.lockShipmentFees(ctx, tx, sid); e != nil {
		return e
	}
	var locked bool
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_fee_items WHERE source_type='freight_actual' AND source_id=$1 AND (settlement_batch_id>0 OR status<>'unsettled'))`, r.schema), id).Scan(&locked)
	if e != nil {
		return e
	}
	if locked {
		return fmt.Errorf("来源已结算或关账，禁止覆盖或撤销")
	}
	var reverseID int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by,reverses_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)RETURNING id`, r.schema), sid, "reverse:"+key, previousActual, previousBilled, previousBilled-billed, actual, billed, reason, actor, id).Scan(&reverseID)
	if e != nil {
		return e
	}
	if previousBilled != billed {
		if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,note)VALUES($1,'freight_actual',$2,'shipping',$3,$4)`, r.schema), cid, reverseID, float64(previousBilled-billed)/100, "实际运费撤销："+reason); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.freight_actual_shipments SET actual_cents=$2,billed_cents=$3,revision=revision+1 WHERE id=$1`, r.schema), sid, previousActual, previousBilled); e != nil {
		return e
	}
	if e = r.Audit(ctx, tx, actor, "freight_actual", id, "reverse", nil, map[string]any{"reason": reason, "reversal_id": reverseID}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (r *Repository) lockShipmentFees(ctx context.Context, tx pgx.Tx, sid int64) error {
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT f.id FROM %[1]s.customer_fee_items f JOIN %[1]s.freight_actual_entries e ON f.source_type='freight_actual' AND f.source_id=e.id WHERE e.shipment_id=$1 ORDER BY f.id FOR UPDATE OF f`, r.schema), sid)
	if e != nil {
		return e
	}
	for rows.Next() {
	}
	e = rows.Err()
	rows.Close()
	return e
}
