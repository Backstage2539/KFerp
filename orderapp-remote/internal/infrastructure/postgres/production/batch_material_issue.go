package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"sort"
)

type batchIssueTask struct {
	id, work                               int64
	no, station, status, workStatus, start string
	sequence, first, priority              int
	input                                  float64
	selected                               bool
	draft                                  int64
}
type batchIssueReservation struct {
	id, work, item, bomSpec, bomVariant, spec, owner, required, consumed int64
	kind, name, unit, basis, warehouse                                   string
}
type batchIssueBinding struct {
	id, reservation, work, materialBatch, stockBatch, owner, remaining int64
	warehouse, batch                                                   string
}
type batchIssueStock struct {
	physical, reserved int64
	quality            string
	signature          string
}

func (r Repository) PreviewBatchMaterialIssue(ctx context.Context, cmd app.BatchMaterialIssueCommand) (app.BatchMaterialIssuePreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.BatchMaterialIssuePreview{}, err
	}
	defer tx.Rollback(ctx)
	result, err := r.previewBatchMaterialIssueTx(ctx, tx, cmd)
	if err != nil {
		return result, err
	}
	err = tx.Commit(ctx)
	return result, err
}

func (r Repository) batchIssueTasksTx(ctx context.Context, tx pgx.Tx, ids []int64) ([]batchIssueTask, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT j.id,j.work_order_id,w.work_order_no,j.workstation,j.status,w.status,j.sequence_no,
 (SELECT MIN(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=w.id AND status<>'cancelled'),j.priority,COALESCE(to_char(j.planned_start_at,'YYYY-MM-DD HH24:MI:SS'),'9999'),j.planned_input_qty::float8,
 COALESCE((SELECT MIN(id) FROM %[1]s.stock_entries WHERE work_order_id=w.id AND purpose='material_transfer_for_manufacture' AND is_return=false AND status='draft'),0)
 FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id
 WHERE j.work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1)) ORDER BY j.id`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	selected := map[int64]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	found := 0
	out := []batchIssueTask{}
	station := ""
	for rows.Next() {
		var t batchIssueTask
		if err := rows.Scan(&t.id, &t.work, &t.no, &t.station, &t.status, &t.workStatus, &t.sequence, &t.first, &t.priority, &t.start, &t.input, &t.draft); err != nil {
			return nil, err
		}
		t.selected = selected[t.id]
		if t.selected {
			found++
			if station != "" && station != t.station {
				return nil, fmt.Errorf("请只选择同一工位的任务")
			}
			station = t.station
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if found != len(ids) {
		return nil, fmt.Errorf("所选任务不存在，请刷新工位")
	}
	return out, nil
}
func batchTaskReason(t batchIssueTask) string {
	if t.workStatus == "completed" || t.workStatus == "cancelled" || t.status == "completed" || t.status == "cancelled" {
		return "任务已完成或取消"
	}
	if t.sequence != t.first {
		return "后续工序承接前道实际产出，无需重复领料"
	}
	if t.input <= 0 {
		return "本批数量待核对，请先确认任务拆分"
	}
	if t.draft > 0 {
		return "已有未提交领料草稿，请先处理原草稿"
	}
	return ""
}

func (r Repository) previewBatchMaterialIssueTx(ctx context.Context, tx pgx.Tx, cmd app.BatchMaterialIssueCommand) (app.BatchMaterialIssuePreview, error) {
	out := app.BatchMaterialIssuePreview{JobCardIDs: cmd.JobCardIDs, Groups: []app.BatchMaterialIssueGroup{}, Blocked: []app.BatchMaterialIssueBlocked{}}
	tasks, err := r.batchIssueTasksTx(ctx, tx, cmd.JobCardIDs)
	if err != nil {
		return out, err
	}
	workIDs := []int64{}
	workSeen := map[int64]bool{}
	eligible := map[int64]bool{}
	for _, t := range tasks {
		if t.selected {
			reason := batchTaskReason(t)
			if reason != "" {
				out.Blocked = append(out.Blocked, app.BatchMaterialIssueBlocked{JobCardID: t.id, WorkOrderNo: t.no, Reason: reason, DraftID: t.draft})
				continue
			}
			eligible[t.id] = true
			if !workSeen[t.work] {
				workSeen[t.work] = true
				workIDs = append(workIDs, t.work)
			}
		}
	}
	if len(workIDs) == 0 {
		out.Version = app.BatchMaterialIssueHash(out)
		return out, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,r.work_order_id,r.component_type,CASE WHEN r.component_id>0 THEN r.component_id ELSE r.material_id END,r.component_bom_spec_id,r.component_bom_variant_id,r.component_spec_g,
 COALESCE(NULLIF(r.material_name,''),m.name,''),COALESCE(NULLIF(r.unit,''),m.unit,''),r.source_owner_customer_id,r.source_warehouse,
 GREATEST(r.required_g,r.reserved_g),GREATEST(r.required_units,r.reserved_units),r.consumed_g,r.consumed_units
 FROM %[1]s.work_order_material_reservations r LEFT JOIN %[1]s.materials m ON m.id=r.material_id WHERE r.work_order_id=ANY($1) AND r.status IN('reserved','consumed') ORDER BY r.id`, r.schema), workIDs)
	if err != nil {
		return out, err
	}
	reservations := []batchIssueReservation{}
	for rows.Next() {
		var rr batchIssueReservation
		var g, n, cg, cn int64
		if err := rows.Scan(&rr.id, &rr.work, &rr.kind, &rr.item, &rr.bomSpec, &rr.bomVariant, &rr.spec, &rr.name, &rr.unit, &rr.owner, &rr.warehouse, &g, &n, &cg, &cn); err != nil {
			rows.Close()
			return out, err
		}
		rr.required, rr.consumed, rr.basis = n, cn, "count"
		if n == 0 && g > 0 {
			rr.required, rr.consumed, rr.basis = g, cg, "weight"
		}
		reservations = append(reservations, rr)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	groups := map[string]*app.BatchMaterialIssueGroup{}
	signatures := map[string][]string{}
	stockCache := map[string]batchIssueStock{}
	freeUsed := map[string]int64{}
	ownUsed := map[string]int64{}
	for _, rr := range reservations {
		bindings, err := r.batchIssueBindingsTx(ctx, tx, rr)
		if err != nil {
			return out, err
		}
		siblings := []batchIssueTask{}
		denominator := 0.0
		for _, t := range tasks {
			if t.work == rr.work && t.sequence == t.first && t.status != "cancelled" {
				siblings = append(siblings, t)
				denominator += t.input
			}
		}
		if denominator <= 0 {
			continue
		}
		// Submitted task documents identify which split was supplied, including out-of-order issues.
		// Pre-existing WIP/consumption without a task document is assigned once to the earliest split.
		posted, err := r.batchIssuePostedTx(ctx, tx, rr)
		if err != nil {
			return out, err
		}
		fulfilled := rr.consumed
		for _, b := range bindings {
			if b.warehouse == "wip" {
				fulfilled += b.remaining
			}
		}
		before := 0.0
		claimed := int64(0)
		reqByTask := map[int64]int64{}
		for _, t := range siblings {
			q := taskQuantitySlice(rr.required, before, t.input, denominator)
			before += t.input
			reqByTask[t.id] = q
			claimed += min(q, posted[t.id])
		}
		inherited := max(int64(0), fulfilled-claimed)
		needed := map[int64]int64{}
		covered := map[int64]int64{}
		for _, t := range siblings {
			q := reqByTask[t.id]
			own := min(q, posted[t.id])
			extra := min(q-own, inherited)
			inherited -= extra
			needed[t.id] = q - own - extra
			covered[t.id] = own + extra
		}
		// Stable priority controls both scarce inventory and edited quantities.
		sort.SliceStable(siblings, func(i, j int) bool {
			a, b := siblings[i], siblings[j]
			if a.priority != b.priority {
				return a.priority > b.priority
			}
			if a.start != b.start {
				return a.start < b.start
			}
			return a.id < b.id
		})
		bindingUsed := map[int64]int64{}
		for _, t := range siblings {
			if !eligible[t.id] || needed[t.id] <= 0 {
				continue
			}
			left := needed[t.id]
			first := true
			for _, b := range bindings {
				if b.warehouse == "wip" || b.remaining <= 0 || b.materialBatch == 0 && b.stockBatch == 0 {
					continue
				}
				need := min(left, max(int64(0), b.remaining-bindingUsed[b.id]))
				if need == 0 {
					continue
				}
				bindingUsed[b.id] += need
				left -= need
				g := batchIssueGroup(rr, b.warehouse, b.owner)
				key := g.Key
				if groups[key] == nil {
					groups[key] = &g
				}
				target := groups[key]
				stockKey := fmt.Sprintf("%d:%d:%s:%d:%s", b.materialBatch, b.stockBatch, b.warehouse, b.owner, rr.basis)
				st, ok := stockCache[stockKey]
				if !ok {
					st, err = r.batchIssueStockTx(ctx, tx, rr, b)
					if err != nil {
						return out, err
					}
					stockCache[stockKey] = st
				}
				ownKey := fmt.Sprintf("%s:%d", stockKey, t.work)
				ownReserved := int64(0)
				for _, other := range bindings {
					if other.materialBatch == b.materialBatch && other.stockBatch == b.stockBatch && other.warehouse == b.warehouse && other.owner == b.owner {
						ownReserved += other.remaining
					}
				}
				capacity := max(int64(0), min(st.physical, st.physical-st.reserved+ownReserved)-ownUsed[ownKey])
				available := min(need, capacity)
				if st.quality == "hold" || st.quality == "reject" {
					available = 0
				}
				ownUsed[ownKey] += available
				extra := max(int64(0), st.physical-st.reserved-freeUsed[stockKey])
				if available < need || st.quality == "hold" || st.quality == "reject" {
					extra = 0
				}
				// Free stock is offered once, to the first eligible allocation. Reserved stock is never borrowed.
				freeUsed[stockKey] += extra
				a := app.BatchMaterialIssueAllocation{JobCardID: t.id, WorkOrderID: t.work, WorkOrderNo: t.no, ReservationID: rr.id, BindingID: b.id, BatchCode: b.batch, Required: need, Remaining: need, Available: available, ExtraAvailable: extra, Priority: t.priority, PlannedStart: t.start}
				if first {
					a.Required += covered[t.id]
					a.WIP = covered[t.id]
					first = false
				}
				target.Allocations = append(target.Allocations, a)
				target.Required += a.Required
				target.WIP += a.WIP
				target.Remaining += need
				target.Available += available
				target.MaxQuantity += available + extra
				signatures[key] = append(signatures[key], st.signature)
				if available < need {
					target.Reason = "部分库存不足、质检冻结或被其他工单预留"
				}
				if left == 0 {
					break
				}
			}
			if left > 0 {
				g := batchIssueGroup(rr, "", rr.owner)
				if groups[g.Key] == nil {
					groups[g.Key] = &g
				}
				target := groups[g.Key]
				a := app.BatchMaterialIssueAllocation{JobCardID: t.id, WorkOrderID: t.work, WorkOrderNo: t.no, ReservationID: rr.id, Required: left, Remaining: left, Priority: t.priority, PlannedStart: t.start}
				if first {
					a.Required += covered[t.id]
					a.WIP = covered[t.id]
				}
				target.Required += a.Required
				target.WIP += a.WIP
				target.Remaining += left
				target.Reason = "等待前道产出或冻结来源资料待完善"
				target.Allocations = append(target.Allocations, a)
			}
		}
	}
	for _, t := range tasks {
		if eligible[t.id] {
			has := false
			for _, rr := range reservations {
				if rr.work == t.work {
					has = true
					break
				}
			}
			if !has {
				out.Blocked = append(out.Blocked, app.BatchMaterialIssueBlocked{JobCardID: t.id, WorkOrderNo: t.no, Reason: "冻结用料资料待完善"})
			}
		}
	}
	for _, g := range groups {
		if g.OwnerCustomerID > 0 {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), g.OwnerCustomerID).Scan(&g.OwnerName); err != nil {
				return out, err
			}
		} else {
			g.OwnerName = "工厂自有"
		}
		sort.SliceStable(g.Allocations, func(i, j int) bool {
			a, b := g.Allocations[i], g.Allocations[j]
			if a.Priority != b.Priority {
				return a.Priority > b.Priority
			}
			if a.PlannedStart != b.PlannedStart {
				return a.PlannedStart < b.PlannedStart
			}
			return a.JobCardID < b.JobCardID
		})
		// Assign the one free-stock budget to the highest-priority eligible task.
		extraByBatch := map[string]int64{}
		for i := range g.Allocations {
			a := &g.Allocations[i]
			extraByBatch[a.BatchCode] += a.ExtraAvailable
			a.ExtraAvailable = 0
		}
		for i := range g.Allocations {
			a := &g.Allocations[i]
			if a.Available > 0 && a.Available == a.Remaining {
				a.ExtraAvailable = extraByBatch[a.BatchCode]
				extraByBatch[a.BatchCode] = 0
			}
		}
		g.Version = app.BatchMaterialIssueHash(struct {
			Group app.BatchMaterialIssueGroup
			Stock []string
		}{*g, signatures[g.Key]})
		out.Groups = append(out.Groups, *g)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i], out.Groups[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Key < b.Key
	})
	out.Version = app.BatchMaterialIssueHash(out)
	return out, nil
}

func batchIssueGroup(rr batchIssueReservation, warehouse string, owner int64) app.BatchMaterialIssueGroup {
	kind := "material"
	if rr.kind == "product" {
		kind = "finished_product"
	}
	g := app.BatchMaterialIssueGroup{ItemType: kind, ItemID: rr.item, Name: rr.name, BomSpecID: rr.bomSpec, BomVariantID: rr.bomVariant, SpecG: rr.spec, OwnerCustomerID: owner, Unit: rr.unit, QuantityBasis: rr.basis, FromWarehouse: warehouse, ToWarehouse: "wip", Allocations: []app.BatchMaterialIssueAllocation{}}
	g.Key = app.BatchMaterialIssueHash([]any{kind, rr.item, rr.bomSpec, rr.bomVariant, rr.spec, owner, rr.unit, rr.basis, warehouse, "wip"})
	return g
}
func (r Repository) batchIssueBindingsTx(ctx context.Context, tx pgx.Tx, rr batchIssueReservation) ([]batchIssueBinding, error) {
	q := "GREATEST(0,reserved_units-consumed_units-returned_units)"
	if rr.basis == "weight" {
		q = "GREATEST(0,reserved_g-consumed_g-returned_g)"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,reservation_id,work_order_id,material_batch_id,stock_batch_id,owner_customer_id,warehouse,batch_code,%s FROM %s.work_order_material_reservation_batches WHERE reservation_id=$1 AND status IN('reserved','consumed') ORDER BY id`, q, r.schema), rr.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []batchIssueBinding{}
	for rows.Next() {
		var b batchIssueBinding
		if err := rows.Scan(&b.id, &b.reservation, &b.work, &b.materialBatch, &b.stockBatch, &b.owner, &b.warehouse, &b.batch, &b.remaining); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r Repository) batchIssuePostedTx(ctx context.Context, tx pgx.Tx, rr batchIssueReservation) (map[int64]int64, error) {
	qty := "i.qty_units"
	if rr.basis == "weight" {
		qty = "i.qty_g"
	}
	kind, idColumn := "material", "i.material_id"
	if rr.kind == "product" {
		kind, idColumn = "finished_product", "i.product_id"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT s.job_card_id,COALESCE(SUM(CASE WHEN s.is_return THEN -%[2]s ELSE %[2]s END),0)::bigint FROM %[1]s.stock_entries s JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=s.id WHERE s.work_order_id=$1 AND s.status='submitted' AND s.purpose='material_transfer_for_manufacture' AND i.item_type=$2 AND %[3]s=$3 AND i.bom_spec_id=$4 AND i.bom_variant_id=$5 AND i.spec_g=$6 AND i.owner_customer_id=$7 GROUP BY s.job_card_id`, r.schema, qty, idColumn), rr.work, kind, rr.item, rr.bomSpec, rr.bomVariant, rr.spec, rr.owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, q int64
		if err := rows.Scan(&id, &q); err != nil {
			return nil, err
		}
		out[id] = max(int64(0), q)
	}
	return out, rows.Err()
}
func (r Repository) batchIssueStockTx(ctx context.Context, tx pgx.Tx, rr batchIssueReservation, b batchIssueBinding) (batchIssueStock, error) {
	var st batchIssueStock
	var g, n, rg, rn int64
	var err error
	if b.materialBatch > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(l.qty_g,0),COALESCE(l.qty_units,0),mb.quality_status FROM %[1]s.material_batches mb LEFT JOIN %[1]s.material_batch_locations l ON l.material_batch_id=mb.id AND l.warehouse=$2 WHERE mb.id=$1 AND mb.owner_customer_id=$3`, r.schema), b.materialBatch, b.warehouse, b.owner).Scan(&g, &n, &st.quality)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units,quality_status FROM %s.stock_batches WHERE id=$1 AND owner_customer_id=$2`, r.schema), b.stockBatch, b.owner).Scan(&g, &n, &st.quality)
	}
	if err == pgx.ErrNoRows {
		st.quality = "hold"
	} else if err != nil {
		return st, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)),0)::bigint,COALESCE(SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)),0)::bigint FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id WHERE b.material_batch_id=$1 AND b.stock_batch_id=$2 AND b.warehouse=$3 AND b.owner_customer_id=$4 AND b.status='reserved' AND r.status='reserved'`, r.schema), b.materialBatch, b.stockBatch, b.warehouse, b.owner).Scan(&rg, &rn)
	if err != nil {
		return st, err
	}
	st.physical, st.reserved = n, rn
	if rr.basis == "weight" {
		st.physical, st.reserved = g, rg
	}
	st.signature = app.BatchMaterialIssueHash([]any{b.materialBatch, b.stockBatch, b.warehouse, b.owner, g, n, rg, rn, st.quality})
	return st, nil
}

// Locks follow the same work-order -> reservation -> inventory order as single issue.
// Inventory is re-read after the locks; previews themselves never write or reserve stock.
func (r Repository) lockBatchMaterialIssueTx(ctx context.Context, tx pgx.Tx, ids []int64) error {
	queries := []string{
		`SELECT id FROM %[1]s.work_orders WHERE id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1)) ORDER BY id FOR UPDATE`,
		`SELECT id FROM %[1]s.job_cards WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1)) ORDER BY id FOR UPDATE`,
		`SELECT id FROM %[1]s.work_order_material_reservations WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1)) ORDER BY id FOR UPDATE`,
		`SELECT id FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1)) ORDER BY id FOR UPDATE`,
		`SELECT id FROM %[1]s.material_batches WHERE id IN(SELECT material_batch_id FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1))) ORDER BY id FOR UPDATE`,
		`SELECT material_batch_id FROM %[1]s.material_batch_locations WHERE material_batch_id IN(SELECT material_batch_id FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1))) ORDER BY material_batch_id,warehouse FOR UPDATE`,
		`SELECT id FROM %[1]s.stock_batches WHERE id IN(SELECT stock_batch_id FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id IN(SELECT work_order_id FROM %[1]s.job_cards WHERE id=ANY($1))) ORDER BY id FOR UPDATE`,
	}
	for _, q := range queries {
		rows, err := tx.Query(ctx, fmt.Sprintf(q, r.schema), ids)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func (r Repository) SubmitBatchMaterialIssue(ctx context.Context, cmd app.BatchMaterialIssueCommand) (app.BatchMaterialIssueResult, error) {
	result := app.BatchMaterialIssueResult{RequestID: cmd.RequestID, Groups: []app.BatchMaterialIssueGroupResult{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	// Reuse the existing audit log as the durable request receipt, under an advisory lock.
	// The receipt and all successful groups commit together, so an uncertain retry is safe.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":batch-material-issue:"+cmd.RequestID); err != nil {
		return result, err
	}
	fingerprint := app.BatchMaterialIssueHash(struct {
		Command app.BatchMaterialIssueCommand
		Actor   string
	}{cmd, cmd.Operator})
	var receipt []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta FROM %s.audit_logs WHERE entity_type='workstation_material_issue' AND action='submit' AND meta->>'request_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), cmd.RequestID).Scan(&receipt)
	if err == nil {
		var stored struct {
			Hash   string                       `json:"hash"`
			Result app.BatchMaterialIssueResult `json:"result"`
		}
		if err = json.Unmarshal(receipt, &stored); err != nil {
			return result, err
		}
		if stored.Hash != fingerprint {
			return result, fmt.Errorf("请求标识已用于另一份领料内容，请刷新后重新提交")
		}
		return stored.Result, nil
	}
	if err != pgx.ErrNoRows {
		return result, err
	}
	if err = r.lockBatchMaterialIssueTx(ctx, tx, cmd.JobCardIDs); err != nil {
		return result, err
	}
	preview, err := r.previewBatchMaterialIssueTx(ctx, tx, cmd)
	if err != nil {
		return result, err
	}
	byKey := map[string]app.BatchMaterialIssueGroup{}
	for _, g := range preview.Groups {
		byKey[g.Key] = g
	}
	stockRepo := postgresstock.NewRepository(r.pool, r.schema)
	for _, selection := range cmd.Groups {
		g, ok := byKey[selection.Key]
		row := app.BatchMaterialIssueGroupResult{Key: selection.Key, Name: g.Name, Unit: g.Unit, QuantityBasis: g.QuantityBasis, Status: "failed", Remaining: g.Remaining, Documents: []app.BatchMaterialIssueDocument{}}
		if !ok || g.Version != selection.Version {
			row.Error = "库存、预留或任务已变化，请刷新领料建议"
		} else {
			qty, qerr := app.BatchMaterialIssueBaseQuantity(g, selection.Quantity)
			var allocations []app.BatchMaterialIssueAllocation
			if qerr == nil {
				allocations, qerr = app.AllocateBatchMaterialIssue(g, qty)
			}
			if qerr != nil {
				row.Error = qerr.Error()
			} else {
				sub, suberr := tx.Begin(ctx)
				if suberr != nil {
					return result, suberr
				}
				commands := map[int64]*stockapp.StockDocumentCommand{}
				order := []int64{}
				// Post reserved demand before extra WIP, so one task cannot take another's reservation.
				for _, a := range allocations {
					if a.Quantity <= 0 {
						continue
					}
					doc := commands[a.JobCardID]
					if doc == nil {
						doc = &stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, EntryType: "material_issue_to_wip", WorkOrderID: a.WorkOrderID, JobCardID: a.JobCardID, SourceType: "work_order", SourceID: a.WorkOrderID, ReturnSource: "workstation_batch", Operator: cmd.Operator, Note: "工位合并领料", IdempotencyKey: "batch-issue:" + cmd.RequestID + ":" + g.Key + fmt.Sprintf(":%d", a.JobCardID)}
						commands[a.JobCardID] = doc
						order = append(order, a.JobCardID)
					}
					item := stockapp.StockDocumentItemCommand{ItemType: g.ItemType, ItemName: g.Name, BomSpecID: g.BomSpecID, BomVariantID: g.BomVariantID, SpecG: g.SpecG, InventoryUnit: g.Unit, FromWarehouse: g.FromWarehouse, ToWarehouse: g.ToWarehouse, OwnerCustomerID: g.OwnerCustomerID, BatchCode: a.BatchCode}
					if g.ItemType == "material" {
						item.MaterialID = g.ItemID
					} else {
						item.ProductID = g.ItemID
					}
					if g.QuantityBasis == "weight" {
						item.QtyG = a.Quantity
					} else {
						item.QtyUnits = a.Quantity
					}
					doc.Items = append(doc.Items, item)
					row.Extra += a.Extra
				}
				// The extra quantity comes exclusively from unreserved stock, so canonical posting
				// can keep enforcing every other work order's reservations unchanged.
				for _, id := range order {
					doc, e := stockRepo.CreateAndSubmitStockDocumentTx(ctx, sub, *commands[id])
					if e != nil {
						suberr = e
						break
					}
					row.Documents = append(row.Documents, app.BatchMaterialIssueDocument{ID: doc.ID, EntryNo: doc.EntryNo, WorkOrderID: doc.WorkOrderID, JobCardID: doc.JobCardID})
				}
				if suberr == nil {
					suberr = sub.Commit(ctx)
				}
				if suberr != nil {
					_ = sub.Rollback(ctx)
					row.Error = suberr.Error()
					row.Documents = []app.BatchMaterialIssueDocument{}
					row.Extra = 0
				} else {
					row.Status = "submitted"
					row.Quantity = qty
					row.Remaining = max(int64(0), g.Remaining-(qty-row.Extra))
				}
			}
		}
		if row.Status == "submitted" {
			result.SuccessCount++
		} else {
			result.FailedCount++
		}
		result.Groups = append(result.Groups, row)
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "workstation_material_issue", nil, "submit", nil, nil, nil, postgresinfra.AuditMeta{"request_id": cmd.RequestID, "hash": fingerprint, "job_card_ids": cmd.JobCardIDs, "result": result}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
