package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
)

func ensurePlanRemainderSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainders (
 id BIGSERIAL PRIMARY KEY, plan_id BIGINT NOT NULL, product_id BIGINT NOT NULL,
 request_id TEXT NOT NULL, operator TEXT NOT NULL, command JSONB NOT NULL,
 result JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(plan_id,request_id));
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_retained (
 plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,source_id TEXT NOT NULL, quantity_g BIGINT NOT NULL CHECK(quantity_g>=0),
 item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,
 warehouse TEXT NOT NULL DEFAULT '',batch_code TEXT NOT NULL DEFAULT '',owner_customer_id BIGINT NOT NULL DEFAULT 0,ledger_version BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(plan_id,product_id,source_id)
);
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS item_type TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS item_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS batch_code TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_packaging_remainder_retained ADD COLUMN IF NOT EXISTS ledger_version BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_rechecks (
 id BIGSERIAL PRIMARY KEY,plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,request_id TEXT NOT NULL,operator TEXT NOT NULL,
 old_version TEXT NOT NULL,old_baseline JSONB NOT NULL,result JSONB NOT NULL DEFAULT '{}'::jsonb,resolved_by BIGINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(plan_id,request_id)
);
ALTER TABLE %[1]s.production_packaging_remainder_rechecks ADD COLUMN IF NOT EXISTS resolved_by BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_expected (
 plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,source_id TEXT NOT NULL,batch_code TEXT NOT NULL,warehouse TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,quantity_g BIGINT NOT NULL CHECK(quantity_g>=0),PRIMARY KEY(plan_id,product_id,source_id)
);
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_discrepancies (
 id BIGSERIAL PRIMARY KEY,plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,recheck_id BIGINT NOT NULL,source_id TEXT NOT NULL,batch_code TEXT NOT NULL,warehouse TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,quantity_g BIGINT NOT NULL CHECK(quantity_g>0),resolved_by BIGINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.production_packaging_remainder_discrepancies ADD COLUMN IF NOT EXISTS resolved_by BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_resolutions (
 id BIGSERIAL PRIMARY KEY,plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,request_id TEXT NOT NULL,operator TEXT NOT NULL,command JSONB NOT NULL,result JSONB NOT NULL DEFAULT '{}'::jsonb,confirmed_by BIGINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(plan_id,request_id)
);
CREATE TABLE IF NOT EXISTS %[1]s.production_packaging_remainder_members (
 plan_id BIGINT NOT NULL,product_id BIGINT NOT NULL,job_card_id BIGINT NOT NULL,remainder_id BIGINT NOT NULL,
 PRIMARY KEY(plan_id,product_id,job_card_id)
)`, schema))
	return err
}

type planRemainderMeta struct {
	source            app.PlanRemainderSource
	card, work, input int64
	bean              app.OperationMaterialUse
}
type planRemainderSnapshot struct {
	detail              app.PlanRemainderDetail
	meta                map[string]planRemainderMeta
	cards               map[int64]planRemainderMeta
	ledgerVersions      []string
	attributionConflict bool
}

// All membership derives from the work order's frozen product and route identity.
const planRemainderTasksSQL = `SELECT w.production_plan_id,COALESCE(NULLIF(w.parent_product_id,0),w.product_id),w.product_name,j.id,w.id,w.work_order_no,j.status
 FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id
 CROSS JOIN LATERAL jsonb_array_elements(COALESCE(w.process_snapshot_json->'operations','[]'::jsonb)) op
 WHERE w.production_plan_id>0 AND COALESCE(NULLIF(w.parent_product_id,0),w.product_id)>0
 AND (op->>'seq')::int=j.sequence_no AND COALESCE(NULLIF(op->>'execution_json','')::jsonb->>'reporting_type','')='piece'
 AND (j.status<>'cancelled' OR EXISTS (SELECT 1 FROM %[1]s.production_started_inputs si JOIN %[1]s.production_started_input_components sc ON sc.input_id=si.id WHERE si.job_card_id=j.id AND si.status='posted' AND sc.remaining_qty>0) OR EXISTS (SELECT 1 FROM %[1]s.work_order_material_reservations wr WHERE wr.work_order_id=w.id AND (wr.reserved_g>wr.consumed_g OR wr.reserved_units>wr.consumed_units)))
 AND NOT EXISTS (SELECT 1 FROM %[1]s.production_packaging_remainder_members done WHERE done.plan_id=w.production_plan_id AND done.product_id=COALESCE(NULLIF(w.parent_product_id,0),w.product_id) AND done.job_card_id=j.id)
 AND (j.status<>'completed' OR EXISTS (SELECT 1 FROM %[1]s.production_operation_events event WHERE event.job_card_id=j.id AND event.kind='complete' AND event.status='posted' AND event.payload->>'remainder_mode'='plan_transfer'))
 AND ($1::bigint=0 OR w.production_plan_id=$1) AND ($2::bigint=0 OR COALESCE(NULLIF(w.parent_product_id,0),w.product_id)=$2)
 ORDER BY w.production_plan_id,COALESCE(NULLIF(w.parent_product_id,0),w.product_id),j.id`

func (r Repository) ListPlanRemainderGroups(ctx context.Context) ([]app.PlanRemainderGroup, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, fmt.Sprintf(planRemainderTasksSQL, r.schema), int64(0), int64(0))
	if err != nil {
		return nil, err
	}
	type key struct{ plan, product int64 }
	grouped := map[key]*app.PlanRemainderGroup{}
	for rows.Next() {
		var plan, product, card, work int64
		var name, no, status string
		if err = rows.Scan(&plan, &product, &name, &card, &work, &no, &status); err != nil {
			rows.Close()
			return nil, err
		}
		k := key{plan, product}
		if grouped[k] == nil {
			grouped[k] = &app.PlanRemainderGroup{PlanID: plan, ProductID: product, ProductName: name, CanConfirm: true}
		}
		g := grouped[k]
		g.TotalTasks++
		if status == "completed" {
			g.CompletedTasks++
		} else {
			g.CanConfirm = false
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]app.PlanRemainderGroup, 0, len(grouped))
	for _, v := range grouped {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PlanID != out[j].PlanID {
			return out[i].PlanID > out[j].PlanID
		}
		return out[i].ProductID < out[j].ProductID
	})
	return out, tx.Commit(ctx)
}

func hashPlanRemainder(v any) string {
	raw, _ := json.Marshal(v)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (r Repository) GetPlanRemainder(ctx context.Context, plan, product int64) (app.PlanRemainderDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.PlanRemainderDetail{}, err
	}
	defer tx.Rollback(ctx)
	s, err := r.planRemainderSnapshotTx(ctx, tx, plan, product)
	if err != nil {
		return app.PlanRemainderDetail{}, err
	}
	return s.detail, tx.Commit(ctx)
}

func (r Repository) planRemainderSnapshotTx(ctx context.Context, tx pgx.Tx, plan, product int64) (planRemainderSnapshot, error) {
	s := planRemainderSnapshot{detail: app.PlanRemainderDetail{PlanRemainderGroup: app.PlanRemainderGroup{PlanID: plan, ProductID: product, CanConfirm: true}, Tasks: []app.PlanRemainderTask{}, Sources: []app.PlanRemainderSource{}}, meta: map[string]planRemainderMeta{}, cards: map[int64]planRemainderMeta{}}
	rows, err := tx.Query(ctx, fmt.Sprintf(planRemainderTasksSQL, r.schema), plan, product)
	if err != nil {
		return s, err
	}
	var cards []int64
	for rows.Next() {
		var pid, productID, card, work int64
		var name, no, status string
		if err = rows.Scan(&pid, &productID, &name, &card, &work, &no, &status); err != nil {
			rows.Close()
			return s, err
		}
		s.detail.ProductName = name
		s.detail.Tasks = append(s.detail.Tasks, app.PlanRemainderTask{JobCardID: card, WorkOrderNo: no, Status: status})
		cards = append(cards, card)
		s.detail.TotalTasks++
		if status == "completed" {
			s.detail.CompletedTasks++
		} else {
			s.detail.CanConfirm = false
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, err
	}
	if len(cards) == 0 {
		return s, fmt.Errorf("未找到本计划产品的包装工序")
	}
	for _, card := range cards {
		var hasStarted bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted')`, r.schema), card).Scan(&hasStarted); err != nil {
			return s, err
		}
		if !hasStarted {
			continue
		}
		p, wo, e := r.operationCompletionContextTx(ctx, tx, card, false)
		if e != nil {
			return s, e
		}
		bean, e := app.ResolvePackagingWeightComponent(p.Materials)
		if e != nil {
			var snapshot []byte
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_snapshot FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted' ORDER BY id LIMIT 1`, r.schema), card).Scan(&snapshot)
			if e != nil {
				return s, fmt.Errorf("包装用料来源尚未记录，请核对")
			}
			var materials []app.OperationMaterialUse
			if e = json.Unmarshal(snapshot, &materials); e != nil {
				return s, e
			}
			bean, e = app.ResolvePackagingWeightComponent(materials)
			if e != nil {
				return s, e
			}
		}
		s.cards[card] = planRemainderMeta{card: card, work: wo.ID, bean: bean}
		rows, e = tx.Query(ctx, fmt.Sprintf(`SELECT c.id,c.input_id,c.remaining_qty::bigint,c.warehouse,c.owner_customer_id,COALESCE(m.batch_code,b.batch_code,'') FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.job_card_id=$1 AND i.status='posted' AND c.component_key=$2 AND c.unit='g' AND c.remaining_qty>0 ORDER BY c.id`, r.schema), card, bean.Key)
		if e != nil {
			return s, e
		}
		for rows.Next() {
			var cid, input, qty, owner int64
			var warehouse, code string
			if e = rows.Scan(&cid, &input, &qty, &warehouse, &owner, &code); e != nil {
				rows.Close()
				return s, e
			}
			if code == "" {
				rows.Close()
				return s, fmt.Errorf("余料来源批次缺失，请核对")
			}
			id := fmt.Sprintf("issued:%d", cid)
			src := app.PlanRemainderSource{SourceID: id, SourceType: "issued", BatchCode: code, Warehouse: warehouse, OwnerCustomerID: owner, AvailableG: qty}
			s.meta[id] = planRemainderMeta{source: src, card: card, work: wo.ID, input: input, bean: bean}
			s.detail.Sources = append(s.detail.Sources, src)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return s, e
		}
		candidates, e := r.packagingUnissuedCandidatesTx(ctx, tx, card, bean)
		if e != nil {
			return s, e
		}
		for _, c := range candidates {
			// A source also linked to another unfinished plan cannot be attributed here.
			var foreign bool
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id JOIN %[1]s.work_orders w ON w.id=i.work_order_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE (w.production_plan_id<>$1 OR COALESCE(NULLIF(w.parent_product_id,0),w.product_id)<>$10) AND w.status<>'cancelled' AND NOT EXISTS(SELECT 1 FROM %[1]s.production_packaging_remainder_members done WHERE done.plan_id=w.production_plan_id AND done.product_id=COALESCE(NULLIF(w.parent_product_id,0),w.product_id) AND done.job_card_id=i.job_card_id) AND c.owner_customer_id=$2 AND c.warehouse=$3 AND COALESCE(m.batch_code,b.batch_code,'')=$4 AND c.component_id=$5 AND c.component_type=$9 AND c.bom_spec_id=$6 AND c.bom_variant_id=$7 AND c.spec_g=$8)`, r.schema), plan, c.OwnerCustomerID, c.Warehouse, c.BatchCode, func() int64 {
				if bean.ProductID > 0 {
					return bean.ProductID
				}
				return bean.MaterialID
			}(), bean.BomSpecID, bean.BomVariantID, bean.SpecG, func() string {
				if bean.ProductID > 0 {
					return "product"
				}
				return "material"
			}(), product).Scan(&foreign)
			if e != nil {
				return s, e
			}
			if foreign {
				s.attributionConflict = true
				s.detail.CanConfirm = false
				s.detail.BlockedReason = "同批库存还关联其他未结算或历史计划，余料归属不明确，请先核对"
				continue
			}
			identity := fmt.Sprintf("%d:%d:%d:%d:%d:%s:%s:%d", bean.MaterialID, bean.ProductID, bean.BomSpecID, bean.BomVariantID, bean.SpecG, c.Warehouse, c.BatchCode, c.OwnerCustomerID)
			id := "unissued:" + hashPlanRemainder(identity)
			var retained int64
			if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT quantity_g FROM %s.production_packaging_remainder_retained WHERE source_id=$1 AND plan_id=$2 AND product_id=$3),0)`, r.schema), id, plan, product).Scan(&retained); e != nil {
				return s, e
			}
			c.AvailableG -= retained
			if c.AvailableG <= 0 {
				continue
			}
			if _, ok := s.meta[id]; ok {
				continue
			}
			src := app.PlanRemainderSource{SourceID: id, SourceType: "unissued", BatchCode: c.BatchCode, Warehouse: c.Warehouse, OwnerCustomerID: c.OwnerCustomerID, AvailableG: c.AvailableG}
			s.meta[id] = planRemainderMeta{source: src, card: card, work: wo.ID, bean: bean}
			s.detail.Sources = append(s.detail.Sources, src)
		}
	}
	sort.Slice(s.detail.Sources, func(i, j int) bool { return s.detail.Sources[i].SourceID < s.detail.Sources[j].SourceID })
	// The optimistic version must change even when equal and opposite movements
	// leave the visible quantity unchanged.
	for _, source := range s.detail.Sources {
		m := s.meta[source.SourceID]
		kind, itemID := "material", m.bean.MaterialID
		if m.bean.ProductID > 0 {
			kind = "finished_product"
			itemID = m.bean.ProductID
		}
		var ledgerID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(max(id),0) FROM %s.stock_ledger_entries WHERE item_type=$1 AND item_id=$2 AND bom_spec_id=$3 AND bom_variant_id=$4 AND spec_g=$5 AND warehouse=$6 AND source_batch_code=$7 AND owner_customer_id=$8`, r.schema), kind, itemID, m.bean.BomSpecID, m.bean.BomVariantID, m.bean.SpecG, source.Warehouse, source.BatchCode, source.OwnerCustomerID).Scan(&ledgerID); err != nil {
			return s, err
		}
		s.ledgerVersions = append(s.ledgerVersions, fmt.Sprintf("source:%s:%d", source.SourceID, ledgerID))
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT x.source_id,x.ledger_version,COALESCE((SELECT max(l.id) FROM %[1]s.stock_ledger_entries l WHERE l.item_type=x.item_type AND l.item_id=x.item_id AND l.bom_spec_id=x.bom_spec_id AND l.bom_variant_id=x.bom_variant_id AND l.spec_g=x.spec_g AND l.warehouse=x.warehouse AND l.source_batch_code=x.batch_code AND l.owner_customer_id=x.owner_customer_id),0) FROM %[1]s.production_packaging_remainder_retained x WHERE x.plan_id=$1 AND x.product_id=$2 ORDER BY x.source_id`, r.schema), plan, product)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var id string
		var old, current int64
		if err = rows.Scan(&id, &old, &current); err != nil {
			rows.Close()
			return s, err
		}
		s.ledgerVersions = append(s.ledgerVersions, fmt.Sprintf("%s:%d:%d", id, old, current))
		if old != current {
			s.detail.CanConfirm = false
			s.detail.RecheckRequired = true
			s.detail.BlockedReason = "已处理余料的库存发生变化，请重新核对并称重"
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, err
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT id,source_id,batch_code,warehouse,owner_customer_id,quantity_g FROM %s.production_packaging_remainder_discrepancies WHERE plan_id=$1 AND product_id=$2 AND resolved_by=0 ORDER BY id`, r.schema), plan, product)
	if err != nil {
		return s, err
	}
	s.detail.Discrepancies = []app.PlanRemainderDiscrepancy{}
	for rows.Next() {
		var d app.PlanRemainderDiscrepancy
		if err = rows.Scan(&d.ID, &d.SourceID, &d.BatchCode, &d.Warehouse, &d.OwnerCustomerID, &d.QuantityG); err != nil {
			rows.Close()
			return s, err
		}
		s.detail.DiscrepancyG += d.QuantityG
		s.detail.Discrepancies = append(s.detail.Discrepancies, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, err
	}
	if s.attributionConflict {
		s.detail.CanConfirm = false
		s.detail.RecheckRequired = false
		s.detail.BlockedReason = "同批库存还关联其他未结算或历史计划，余料归属不明确，请先核对"
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_packaging_remainder_rechecks WHERE plan_id=$1 AND product_id=$2 AND resolved_by=0) OR EXISTS(SELECT 1 FROM %[1]s.production_packaging_remainder_resolutions WHERE plan_id=$1 AND product_id=$2 AND confirmed_by=0)`, r.schema), plan, product).Scan(&s.detail.ReweighRequired); err != nil {
		return s, err
	}
	s.detail.Version = hashPlanRemainder([]any{s.detail, s.ledgerVersions})
	return s, nil
}

func (r Repository) ConfirmPlanRemainder(ctx context.Context, c app.PlanRemainderCommand) (app.PlanRemainderResult, error) {
	var result app.PlanRemainderResult
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("plan-remainder:%d:%d", c.PlanID, c.ProductID)); err != nil {
		return result, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return result, err
	}
	var oldCommand, oldResult []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT command,result FROM %s.production_packaging_remainders WHERE plan_id=$1 AND request_id=$2`, r.schema), c.PlanID, c.RequestID).Scan(&oldCommand, &oldResult)
	if err == nil {
		var previous app.PlanRemainderCommand
		if json.Unmarshal(oldCommand, &previous) != nil {
			return result, fmt.Errorf("历史余料请求无效")
		}
		previous.PlanID = c.PlanID
		previous.Operator = c.Operator
		oldCanonical, _ := json.Marshal(previous)
		if string(raw) != string(oldCanonical) {
			return result, fmt.Errorf("请求编号已用于其他余料操作")
		}
		if err = json.Unmarshal(oldResult, &result); err != nil {
			return result, err
		}
		return result, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return result, err
	}
	var active bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.warehouses WHERE code=$1`, r.schema), c.TargetWarehouse).Scan(&active)
	if err != nil || !active {
		return result, fmt.Errorf("目标仓库不存在或已停用")
	}
	snapshot, err := r.planRemainderSnapshotTx(ctx, tx, c.PlanID, c.ProductID)
	if err != nil {
		return result, err
	}
	if !snapshot.detail.CanConfirm {
		if snapshot.detail.BlockedReason != "" {
			return result, fmt.Errorf("%s", snapshot.detail.BlockedReason)
		}
		return result, fmt.Errorf("请先完成本计划该产品的全部包装工序")
	}
	if snapshot.detail.Version != c.ExpectedVersion {
		return result, fmt.Errorf("包装工单或库存已变化，请重新核对")
	}
	if err = app.ValidatePlanRemainderSelection(c, snapshot.detail.Sources); err != nil {
		return result, err
	}
	if snapshot.detail.ReweighRequired && !c.ReweighConfirmed {
		return result, fmt.Errorf("请重新称重并逐项确认余料")
	}

	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainders(plan_id,product_id,request_id,operator,command) VALUES($1,$2,$3,$4,$5::jsonb) RETURNING id`, r.schema), c.PlanID, c.ProductID, c.RequestID, c.Operator, string(raw)).Scan(&result.ID); err != nil {
		return result, err
	}
	if snapshot.detail.ReweighRequired {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainder_rechecks SET resolved_by=$3 WHERE plan_id=$1 AND product_id=$2 AND resolved_by=0`, r.schema), c.PlanID, c.ProductID, result.ID); err != nil {
			return result, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainder_resolutions SET confirmed_by=$3 WHERE plan_id=$1 AND product_id=$2 AND confirmed_by=0`, r.schema), c.PlanID, c.ProductID, result.ID); err != nil {
			return result, err
		}
	}
	newDiscrepancies, err := recordExpectedShortfallsTx(ctx, tx, r.schema, c.PlanID, c.ProductID, 0, snapshot.detail.Sources)
	if err != nil {
		return result, err
	}
	for _, d := range newDiscrepancies {
		snapshot.detail.DiscrepancyG += d.QuantityG
	}
	eventByCard := map[int64]int64{}
	returnedCostByEvent := map[int64]float64{}
	var processed int64
	ensureEvent := func(m planRemainderMeta, warehouse string) (int64, error) {
		if ev := eventByCard[m.card]; ev > 0 {
			return ev, nil
		}
		var ev int64
		e := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,request_id,expected_version,operator,input_unit,output_unit,warehouse,difference_reason,payload) VALUES($1,$2,'material_return',$3,$4,$5,'g','g',$6,'包装余料统一处理',$7::jsonb) RETURNING id`, r.schema), m.work, m.card, fmt.Sprintf("packaging-plan:%d:%d", result.ID, m.card), c.ExpectedVersion, c.Operator, warehouse, string(raw)).Scan(&ev)
		if e == nil {
			eventByCard[m.card] = ev
		}
		return ev, e
	}
	for _, sel := range c.Sources {
		m := snapshot.meta[sel.SourceID]
		if sel.QuantityG == 0 {
			continue
		}
		processed += sel.QuantityG
		if m.source.SourceType == "issued" {
			ev, e := ensureEvent(m, m.source.Warehouse)
			if e != nil {
				return result, e
			}
			var cid int64
			if _, err = fmt.Sscanf(sel.SourceID, "issued:%d", &cid); err != nil || cid <= 0 {
				return result, fmt.Errorf("余料来源无效")
			}
			returnedCost, e := stockrepo.NewRepository(r.pool, r.schema).ReturnStartedComponentTx(ctx, tx, cid, ev, float64(sel.QuantityG), c.Operator)
			if e != nil {
				return result, e
			}
			returnedCostByEvent[ev] += returnedCost
			tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_cost=remaining_cost-$2 WHERE id=$1 AND remaining_cost+0.000001>=$2`, r.schema), m.input, returnedCost)
			if e != nil {
				return result, e
			}
			if tag.RowsAffected() != 1 {
				return result, fmt.Errorf("已领余料成本余额不一致")
			}
		}
		if c.TargetWarehouse == m.source.Warehouse {
			retainID := sel.SourceID
			if m.source.SourceType == "issued" {
				b := m.bean
				identity := fmt.Sprintf("%d:%d:%d:%d:%d:%s:%s:%d", b.MaterialID, b.ProductID, b.BomSpecID, b.BomVariantID, b.SpecG, m.source.Warehouse, m.source.BatchCode, m.source.OwnerCustomerID)
				retainID = "unissued:" + hashPlanRemainder(identity)
			}
			kind, itemID := "material", m.bean.MaterialID
			if m.bean.ProductID > 0 {
				kind = "finished_product"
				itemID = m.bean.ProductID
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_retained(plan_id,product_id,source_id,quantity_g,item_type,item_id,bom_spec_id,bom_variant_id,spec_g,warehouse,batch_code,owner_customer_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(plan_id,product_id,source_id) DO UPDATE SET quantity_g=production_packaging_remainder_retained.quantity_g+EXCLUDED.quantity_g`, r.schema), c.PlanID, c.ProductID, retainID, sel.QuantityG, kind, itemID, m.bean.BomSpecID, m.bean.BomVariantID, m.bean.SpecG, m.source.Warehouse, m.source.BatchCode, m.source.OwnerCustomerID)
			if err != nil {
				return result, err
			}
		}
		if c.TargetWarehouse != m.source.Warehouse {
			b := m.bean
			item := stockapp.StockDocumentItemCommand{ItemType: "material", MaterialID: b.MaterialID, ItemName: b.Name, InventoryUnit: b.Unit, BomSpecID: b.BomSpecID, BomVariantID: b.BomVariantID, SpecG: b.SpecG, FromWarehouse: m.source.Warehouse, ToWarehouse: c.TargetWarehouse, OwnerCustomerID: m.source.OwnerCustomerID, BatchCode: m.source.BatchCode, QtyG: sel.QuantityG}
			if b.ProductID > 0 {
				item.ItemType = "finished_product"
				item.ProductID = b.ProductID
				item.MaterialID = 0
			}
			doc, e := stockrepo.NewRepository(r.pool, r.schema).CreateAndSubmitStockDocumentTx(ctx, tx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, SourceType: "packaging_plan_remainder", SourceID: result.ID, Operator: c.Operator, IdempotencyKey: fmt.Sprintf("packaging-plan:%d:%s", result.ID, sel.SourceID), Items: []stockapp.StockDocumentItemCommand{item}})
			if e != nil {
				return result, e
			}
			result.StockEntryIDs = append(result.StockEntryIDs, doc.ID)
		}
	}

	// Remaining packaging components are returned to their original location,
	// never included in or moved with the weighed semi-finished quantity.
	for _, task := range snapshot.detail.Tasks {
		m, ok := snapshot.cards[task.JobCardID]
		if !ok {
			continue
		}
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT c.id,c.input_id,c.remaining_qty::float8,c.warehouse FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=$1 AND i.status='posted' AND c.component_key<>$2 AND c.remaining_qty>0 ORDER BY c.id`, r.schema), m.card, m.bean.Key)
		if e != nil {
			return result, e
		}
		type component struct {
			id, input int64
			qty       float64
			warehouse string
		}
		items := []component{}
		for rows.Next() {
			var x component
			if e = rows.Scan(&x.id, &x.input, &x.qty, &x.warehouse); e != nil {
				rows.Close()
				return result, e
			}
			items = append(items, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return result, e
		}
		for _, x := range items {
			ev, e := ensureEvent(m, x.warehouse)
			if e != nil {
				return result, e
			}
			returned, e := stockrepo.NewRepository(r.pool, r.schema).ReturnStartedComponentTx(ctx, tx, x.id, ev, x.qty, c.Operator)
			if e != nil {
				return result, e
			}
			returnedCostByEvent[ev] += returned
			tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_cost=remaining_cost-$2 WHERE id=$1 AND remaining_cost+0.000001>=$2`, r.schema), x.input, returned)
			if e != nil {
				return result, e
			}
			if tag.RowsAffected() != 1 {
				return result, fmt.Errorf("已领包材成本余额不一致")
			}
		}
	}
	// Capture the post-transaction source baseline; future unrelated stock changes
	// require explicit reweighing before this round can be finalized.
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_packaging_remainder_retained x SET ledger_version=COALESCE((SELECT max(l.id) FROM %[1]s.stock_ledger_entries l WHERE l.item_type=x.item_type AND l.item_id=x.item_id AND l.bom_spec_id=x.bom_spec_id AND l.bom_variant_id=x.bom_variant_id AND l.spec_g=x.spec_g AND l.warehouse=x.warehouse AND l.source_batch_code=x.batch_code AND l.owner_customer_id=x.owner_customer_id),0) WHERE x.plan_id=$1 AND x.product_id=$2`, r.schema), c.PlanID, c.ProductID); err != nil {
		return result, err
	}
	for _, ev := range eventByCard {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2 WHERE id=$1`, r.schema), ev, -returnedCostByEvent[ev]); err != nil {
			return result, err
		}
	}
	for _, s := range snapshot.detail.Sources {
		result.OutstandingG += s.AvailableG
	}
	result.OutstandingG -= processed
	result.DiscrepancyG = snapshot.detail.DiscrepancyG
	result.OutstandingG += result.DiscrepancyG
	// Persist the remaining per-source baseline for any later explicit recheck.
	expected := make([]app.PlanRemainderSource, 0, len(snapshot.detail.Sources))
	selected := map[string]int64{}
	for _, v := range c.Sources {
		selected[v.SourceID] = v.QuantityG
	}
	for _, v := range snapshot.detail.Sources {
		v.AvailableG -= selected[v.SourceID]
		if v.AvailableG > 0 {
			expected = append(expected, v)
		}
	}
	if err = replacePlanRemainderExpectedTx(ctx, tx, r.schema, c.PlanID, c.ProductID, expected); err != nil {
		return result, err
	}
	if result.OutstandingG == 0 {
		for _, task := range snapshot.detail.Tasks {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_members(plan_id,product_id,job_card_id,remainder_id) VALUES($1,$2,$3,$4)`, r.schema), c.PlanID, c.ProductID, task.JobCardID, result.ID); err != nil {
				return result, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_packaging_remainder_retained WHERE plan_id=$1 AND product_id=$2`, r.schema), c.PlanID, c.ProductID); err != nil {
			return result, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "production_plan", &c.PlanID, "packaging_remainder", nil, nil, nil, infra.AuditMeta{"product_id": c.ProductID, "remainder_id": result.ID, "target_warehouse": c.TargetWarehouse, "sources": c.Sources, "added_discrepancies": newDiscrepancies}); err != nil {
		return result, err
	}
	resultRaw, _ := json.Marshal(result)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainders SET result=$2::jsonb WHERE id=$1`, r.schema), result.ID, string(resultRaw)); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

// Recheck closes the former quantity baseline without changing inventory. The
// caller must recount the newly read sources before the next confirmation.
func (r Repository) RecheckPlanRemainder(ctx context.Context, c app.PlanRemainderRecheckCommand) (app.PlanRemainderDetail, error) {
	var out app.PlanRemainderDetail
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("plan-remainder:%d:%d", c.PlanID, c.ProductID)); err != nil {
		return out, err
	}
	var priorProduct int64
	var priorVersion string
	var priorResult []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,old_version,result FROM %s.production_packaging_remainder_rechecks WHERE plan_id=$1 AND request_id=$2`, r.schema), c.PlanID, c.RequestID).Scan(&priorProduct, &priorVersion, &priorResult)
	if err == nil {
		if priorProduct != c.ProductID || priorVersion != c.ExpectedVersion {
			return out, fmt.Errorf("请求编号已用于其他余料操作")
		}
		if err = json.Unmarshal(priorResult, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return out, err
	}
	snap, err := r.planRemainderSnapshotTx(ctx, tx, c.PlanID, c.ProductID)
	if err != nil {
		return out, err
	}
	if snap.detail.Version != c.ExpectedVersion {
		return out, fmt.Errorf("库存又发生变化，请重新核对")
	}
	if snap.attributionConflict {
		return out, fmt.Errorf("同批库存归属不明确，请先核对")
	}
	if !snap.detail.RecheckRequired {
		return out, fmt.Errorf("当前余料无需重新核对")
	}
	var oldBaseline []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.source_id),'[]'::jsonb) FROM %s.production_packaging_remainder_retained x WHERE x.plan_id=$1 AND x.product_id=$2`, r.schema), c.PlanID, c.ProductID).Scan(&oldBaseline); err != nil {
		return out, err
	}
	var id int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_rechecks(plan_id,product_id,request_id,operator,old_version,old_baseline) VALUES($1,$2,$3,$4,$5,$6::jsonb) RETURNING id`, r.schema), c.PlanID, c.ProductID, c.RequestID, c.Operator, c.ExpectedVersion, string(oldBaseline)).Scan(&id); err != nil {
		return out, err
	}
	// Compare before clearing retained stock so already confirmed WIP cannot
	// silently offset a shortage of the still-unconfirmed quantity.
	added, err := recordExpectedShortfallsTx(ctx, tx, r.schema, c.PlanID, c.ProductID, id, snap.detail.Sources)
	if err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_packaging_remainder_retained WHERE plan_id=$1 AND product_id=$2`, r.schema), c.PlanID, c.ProductID); err != nil {
		return out, err
	}
	next, err := r.planRemainderSnapshotTx(ctx, tx, c.PlanID, c.ProductID)
	if err != nil {
		return out, err
	}
	out = next.detail
	if err = replacePlanRemainderExpectedTx(ctx, tx, r.schema, c.PlanID, c.ProductID, next.detail.Sources); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "production_plan", &c.PlanID, "packaging_remainder_recheck", nil, nil, nil, infra.AuditMeta{"product_id": c.ProductID, "recheck_id": id, "old_version": c.ExpectedVersion, "new_version": out.Version, "old_baseline": json.RawMessage(oldBaseline), "added_discrepancies": added}); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainder_rechecks SET result=$2::jsonb WHERE id=$1`, r.schema), id, string(raw)); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func replacePlanRemainderExpectedTx(ctx context.Context, tx pgx.Tx, schema string, plan, product int64, sources []app.PlanRemainderSource) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_packaging_remainder_expected WHERE plan_id=$1 AND product_id=$2`, schema), plan, product); err != nil {
		return err
	}
	for _, v := range sources {
		if v.AvailableG <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_expected(plan_id,product_id,source_id,batch_code,warehouse,owner_customer_id,quantity_g) VALUES($1,$2,$3,$4,$5,$6,$7)`, schema), plan, product, v.SourceID, v.BatchCode, v.Warehouse, v.OwnerCustomerID, v.AvailableG); err != nil {
			return err
		}
	}
	return nil
}

func recordExpectedShortfallsTx(ctx context.Context, tx pgx.Tx, schema string, plan, product, ref int64, sources []app.PlanRemainderSource) ([]app.PlanRemainderDiscrepancy, error) {
	current := map[string]int64{}
	for _, v := range sources {
		current[v.SourceID] = v.AvailableG
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT source_id,batch_code,warehouse,owner_customer_id,quantity_g FROM %s.production_packaging_remainder_expected WHERE plan_id=$1 AND product_id=$2 ORDER BY source_id`, schema), plan, product)
	if err != nil {
		return nil, err
	}
	added := []app.PlanRemainderDiscrepancy{}
	for rows.Next() {
		var d app.PlanRemainderDiscrepancy
		var expected int64
		if err = rows.Scan(&d.SourceID, &d.BatchCode, &d.Warehouse, &d.OwnerCustomerID, &expected); err != nil {
			rows.Close()
			return nil, err
		}
		d.QuantityG = expected - current[d.SourceID]
		if d.QuantityG > 0 {
			added = append(added, d)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range added {
		d := &added[i]
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_discrepancies(plan_id,product_id,recheck_id,source_id,batch_code,warehouse,owner_customer_id,quantity_g) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), plan, product, ref, d.SourceID, d.BatchCode, d.Warehouse, d.OwnerCustomerID, d.QuantityG).Scan(&d.ID); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// ResolvePlanRemainderDiscrepancies records a human investigation, never stock
// adjustment or cost changes. A fresh, explicit recount is still required.
func (r Repository) ResolvePlanRemainderDiscrepancies(ctx context.Context, c app.PlanRemainderResolveCommand) (app.PlanRemainderDetail, error) {
	var out app.PlanRemainderDetail
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("plan-remainder:%d:%d", c.PlanID, c.ProductID)); err != nil {
		return out, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return out, err
	}
	var oldCommand, oldResult []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT command,result FROM %s.production_packaging_remainder_resolutions WHERE plan_id=$1 AND request_id=$2`, r.schema), c.PlanID, c.RequestID).Scan(&oldCommand, &oldResult)
	if err == nil {
		var previous app.PlanRemainderResolveCommand
		if json.Unmarshal(oldCommand, &previous) != nil {
			return out, fmt.Errorf("历史差额核对请求无效")
		}
		previous.PlanID = c.PlanID
		previous.Operator = c.Operator
		canonical, _ := json.Marshal(previous)
		if string(canonical) != string(raw) {
			return out, fmt.Errorf("请求编号已用于其他核对操作")
		}
		if err = json.Unmarshal(oldResult, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return out, err
	}
	snap, err := r.planRemainderSnapshotTx(ctx, tx, c.PlanID, c.ProductID)
	if err != nil {
		return out, err
	}
	if snap.detail.Version != c.ExpectedVersion {
		return out, fmt.Errorf("库存或差额已变化，请重新核对")
	}
	if snap.attributionConflict || snap.detail.RecheckRequired {
		return out, fmt.Errorf("库存来源或流水已变化，请先核对")
	}
	valid := map[int64]bool{}
	for _, d := range snap.detail.Discrepancies {
		valid[d.ID] = true
	}
	seen := map[int64]bool{}
	for _, id := range c.DiscrepancyIDs {
		if !valid[id] || seen[id] {
			return out, fmt.Errorf("待核对差额已变化，请重新读取")
		}
		seen[id] = true
	}
	var id int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_packaging_remainder_resolutions(plan_id,product_id,request_id,operator,command) VALUES($1,$2,$3,$4,$5::jsonb) RETURNING id`, r.schema), c.PlanID, c.ProductID, c.RequestID, c.Operator, string(raw)).Scan(&id); err != nil {
		return out, err
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainder_discrepancies SET resolved_by=$3 WHERE plan_id=$1 AND product_id=$2 AND id=ANY($4::bigint[]) AND resolved_by=0`, r.schema), c.PlanID, c.ProductID, id, c.DiscrepancyIDs)
	if err != nil {
		return out, err
	}
	if tag.RowsAffected() != int64(len(c.DiscrepancyIDs)) {
		return out, fmt.Errorf("待核对差额已变化，请重新读取")
	}
	next, err := r.planRemainderSnapshotTx(ctx, tx, c.PlanID, c.ProductID)
	if err != nil {
		return out, err
	}
	out = next.detail
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "production_plan", &c.PlanID, "packaging_remainder_discrepancy_resolve", nil, nil, nil, infra.AuditMeta{"product_id": c.ProductID, "resolution_id": id, "discrepancy_ids": c.DiscrepancyIDs, "reason": c.Reason}); err != nil {
		return out, err
	}
	resultRaw, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_packaging_remainder_resolutions SET result=$2::jsonb WHERE id=$1`, r.schema), id, string(resultRaw)); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
