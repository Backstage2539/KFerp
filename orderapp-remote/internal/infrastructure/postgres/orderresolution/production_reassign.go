package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) reassignSnapshot(ctx context.Context, q querier, workID int64) (map[string]json.RawMessage, []byte, error) {
	var raw []byte
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('work',to_jsonb(w),'plan_item',to_jsonb(i),'plan',to_jsonb(p),'run',COALESCE((SELECT to_jsonb(run) FROM %[1]s.produce_running_items run WHERE run.id=w.running_item_id),'{}')) FROM %[1]s.work_orders w JOIN %[1]s.production_plan_items i ON i.id=w.production_plan_item_id JOIN %[1]s.production_plans p ON p.id=w.production_plan_id WHERE w.id=$1`, r.schema), workID).Scan(&raw)
	out := map[string]json.RawMessage{}
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, raw, err
}
func (r Repository) productionReassignSources(ctx context.Context, q querier, c *domain.Context, items []map[string]json.RawMessage) error {
	for n := range c.Dependencies {
		d := &c.Dependencies[n]
		if d.Kind != "production" || one(d.Status, "completed", "cancelled", "stopped") {
			continue
		}
		for _, stop := range d.StopWorkOrders {
			s := domain.ReassignWorkOrder{WorkOrderID: stop.WorkOrderID, Label: stop.Label, Issue: stop.Issue, Targets: []domain.CompanyStockTarget{}}
			snap, raw, err := r.reassignSnapshot(ctx, q, stop.WorkOrderID)
			if err != nil {
				return err
			}
			var w, pi map[string]json.RawMessage
			_ = json.Unmarshal(snap["work"], &w)
			_ = json.Unmarshal(snap["plan_item"], &pi)
			s.Unit = str(w, "inventory_unit")
			qty := num(w, "planned_inventory_qty")
			s.Quantity = int64(qty)
			s.Version = hash(append(raw, []byte(stop.Version)...))
			if d.Shared || len(d.StopWorkOrders) != 1 {
				s.Issue = "共享或多工单生产请先拆分并核对，不能整批转供"
			}
			if d.OwnershipIssue != "" {
				s.Issue = d.OwnershipIssue
			}
			if stop.HasOutputs {
				s.Issue = "本工单已有成品产出，请分别处理已完成批次及剩余生产，不能整批改供"
			}
			if qty <= 0 || math.Trunc(qty) != qty || num(w, "spec_g") != 0 || s.Unit == "" || num(pi, "planned_inventory_qty") != qty || str(pi, "inventory_unit") != s.Unit {
				s.Issue = "原生产数量、库存单位或换算不完整，请先核对工单"
			}
			if str(w, "output_type") != "product" || num(w, "output_product_id") != num(w, "product_id") || num(pi, "output_product_id") != num(w, "product_id") || num(w, "output_qty") != qty || num(pi, "output_qty") != qty || str(w, "output_unit") != s.Unit || str(pi, "output_unit") != s.Unit {
				s.Issue = "原工单及计划产出身份、数量或单位不完整，请先核对"
			}
			if str(w, "target_warehouse") == "" || str(pi, "target_warehouse") != str(w, "target_warehouse") {
				s.Issue = "原生产目的仓未明确或不一致，请先核对"
			}
			if reassignHasCustomerIdentity(w["customer_product_snapshot_json"]) || reassignHasCustomerIdentity(pi["customer_product_snapshot_json"]) {
				s.Issue = "生产含客户专属要求，请先由生产核对目标要求，不能直接转供"
			}
			var counts, prior int
			if err = q.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.work_orders WHERE production_plan_id=$1)+(SELECT count(*) FROM %[1]s.production_plan_items WHERE production_plan_id=$1),(SELECT count(*) FROM %[1]s.order_resolution_production_reassignments WHERE work_order_id=$2)`, r.schema), d.ID, s.WorkOrderID).Scan(&counts, &prior); err != nil {
				return err
			}
			if counts != 2 {
				s.Issue = "计划含其他工单或需求行，请先拆分核对"
			}
			if prior > 0 {
				s.Issue = "该工单已有转供记录，再次转供须另办生产更正"
			}
			for _, item := range items {
				if num(item, "product_id") == num(w, "product_id") && num(item, "bom_spec_id") == num(w, "bom_spec_id") && num(item, "bom_variant_id") == num(w, "bom_variant_id") && str(item, "unit") == s.Unit {
					if s.SourceItemID > 0 {
						s.Issue = "原订单有重复规格商品行，请先核对来源"
					}
					s.SourceItemID = int64(num(item, "id"))
					if str(item, "item_note") != "" {
						s.Issue = "原商品行含加工说明，请先核对目标要求"
					}
				}
			}
			if s.SourceItemID == 0 {
				s.Issue = "原订单商品与冻结生产身份不一致"
			}
			for _, line := range c.Lines {
				if line.ID == s.SourceItemID && line.Quantity-line.Closed-line.Delivered < float64(s.Quantity) {
					s.Issue = "工单数量超过本单可关闭数量，请先拆分需求"
				}
			}
			if s.Issue == "" {
				targets, err := r.companyTargets(ctx, q, *c, companyBatch{ProductID: int64(num(w, "product_id")), SpecID: int64(num(w, "bom_spec_id")), VariantID: int64(num(w, "bom_variant_id")), Warehouse: str(w, "target_warehouse")}, s.Unit)
				if err != nil {
					return err
				}
				targets, err = r.reassignEligibleTargets(ctx, q, targets)
				if err != nil {
					return err
				}
				for _, t := range targets {
					if t.RemainingQuantity == s.Quantity {
						s.Targets = append(s.Targets, t)
					}
				}
			}
			d.ReassignWorkOrders = append(d.ReassignWorkOrders, s)
		}
		raw, _ := json.Marshal(d.ReassignWorkOrders)
		c.Version = hash([]byte(c.Version + string(raw)))
	}
	return nil
}
func verifyProductionReassignIdentity(original, current domain.Context, cmd domain.Command) error {
	find := func(c domain.Context, planID int64, s domain.ReassignSelection) string {
		for _, d := range c.Dependencies {
			if d.Kind == "production" && d.ID == planID {
				for _, w := range d.ReassignWorkOrders {
					if w.WorkOrderID == s.WorkOrderID && w.Issue == "" {
						for _, t := range w.Targets {
							if t.OrderItemID == s.TargetItemID {
								return w.Version + "|" + t.Version
							}
						}
					}
				}
			}
		}
		return ""
	}
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "reassign" {
			continue
		}
		for _, s := range d.Reassignments {
			a, b := find(original, d.ID, s), find(current, d.ID, s)
			if a == "" || a != b {
				return fmt.Errorf("%w：原生产或目标需求已变化，请刷新后重新确认转供", ErrStale)
			}
		}
	}
	return nil
}
func (r Repository) applyProductionReassignments(ctx context.Context, tx pgx.Tx, orderID, requestID int64, cmd domain.Command, actor string) error {
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "reassign" {
			continue
		}
		for _, s := range d.Reassignments {
			snap, raw, err := r.reassignSnapshot(ctx, tx, s.WorkOrderID)
			if err != nil {
				return err
			}
			var w map[string]json.RawMessage
			_ = json.Unmarshal(snap["work"], &w)
			var targetID, customer int64
			var targetNo, unit string
			var targetRaw []byte
			err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,COALESCE(o.customer_id,0),o.order_no,i.unit,jsonb_build_object('order',to_jsonb(o),'item',to_jsonb(i)) FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE i.id=$1 FOR UPDATE OF o,i`, r.schema), s.TargetItemID).Scan(&targetID, &customer, &targetNo, &unit, &targetRaw)
			if err != nil {
				return err
			}
			var id int64
			err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_production_reassignments(request_id,source_order_id,target_order_id,target_item_id,plan_id,plan_item_id,work_order_id,running_item_id,quantity,unit,snapshot_json,target_json,actor)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)RETURNING id`, r.schema), requestID, orderID, targetID, s.TargetItemID, d.ID, int64(num(w, "production_plan_item_id")), s.WorkOrderID, int64(num(w, "running_item_id")), s.Quantity, unit, raw, targetRaw, actor).Scan(&id)
			if err != nil {
				return err
			}
			sources, _ := json.Marshal([]map[string]any{{"order_id": targetID, "order_item_id": s.TargetItemID, "order_no": targetNo, "customer_id": customer, "quantity": s.Quantity, "sales_unit": unit, "force_produce": false}})
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET order_nos=$2,customer_id=$3,demand_sources_json=$4 WHERE id=$1`, r.schema), int64(num(w, "production_plan_item_id")), targetNo, customer, sources); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET order_nos=$2,customer_id=$3 WHERE id=$1`, r.schema), s.WorkOrderID, targetNo, customer); err != nil {
				return err
			}
			if num(w, "running_item_id") > 0 {
				if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET order_nos=$2 WHERE id=$1`, r.schema), int64(num(w, "running_item_id")), targetNo); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_production_reassignments SET status='applied' WHERE id=$1`, r.schema), id); err != nil {
				return err
			}
			if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "production_plan", &d.ID, "resolution_production_reassign", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "source_order_id": orderID, "target_order_id": targetID, "target_item_id": s.TargetItemID, "work_order_id": s.WorkOrderID, "quantity": s.Quantity, "preserved_inputs": true}); err != nil {
				return err
			}
		}
	}
	return nil
}

func reassignTargetIDs(cmd domain.Command) ([]int64, []int64) {
	items, works := []int64{}, []int64{}
	for _, d := range cmd.Dispositions {
		if d.Kind == "production" && d.Choice == "reassign" {
			for _, s := range d.Reassignments {
				items = append(items, s.TargetItemID)
				works = append(works, s.WorkOrderID)
			}
		}
	}
	return items, works
}

// Immutable receipt reads the original and chosen target identities from posting
// snapshots, not from subsequently edited display names on live orders.
type ProductionReassignRecord struct {
	ID            int64  `json:"id"`
	WorkOrderID   int64  `json:"work_order_id"`
	SourceOrderID int64  `json:"source_order_id"`
	TargetOrderID int64  `json:"target_order_id"`
	TargetItemID  int64  `json:"target_item_id"`
	SourceOrderNo string `json:"source_order_no"`
	TargetOrderNo string `json:"target_order_no"`
	Quantity      int64  `json:"quantity"`
	Unit          string `json:"unit"`
}

func (r Repository) productionReassignRecords(ctx context.Context, q querier, requestID int64) ([]ProductionReassignRecord, error) {
	out := []ProductionReassignRecord{}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT id,work_order_id,source_order_id,target_order_id,target_item_id,snapshot_json->'work'->>'order_nos',target_json->'order'->>'order_no',quantity,unit FROM %s.order_resolution_production_reassignments WHERE request_id=$1 AND status='applied' ORDER BY id`, r.schema), requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v ProductionReassignRecord
		if err = rows.Scan(&v.ID, &v.WorkOrderID, &v.SourceOrderID, &v.TargetOrderID, &v.TargetItemID, &v.SourceOrderNo, &v.TargetOrderNo, &v.Quantity, &v.Unit); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func reassignHasCustomerIdentity(raw json.RawMessage) bool {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(raw, &rows) != nil {
		return true
	}
	for _, row := range rows {
		if num(row, "customer_product_alias_id") > 0 || str(row, "customer_product_display_name_snapshot") != "" || str(row, "customer_item_code_snapshot") != "" {
			return true
		}
	}
	return false
}
func (r Repository) reassignEligibleTargets(ctx context.Context, q querier, targets []domain.CompanyStockTarget) ([]domain.CompanyStockTarget, error) {
	ids := []int64{}
	for _, v := range targets {
		ids = append(ids, v.OrderItemID)
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT i.id FROM %[1]s.order_items i JOIN %[1]s.orders o ON o.id=i.order_id WHERE i.id=ANY($1::bigint[]) AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted' AND COALESCE((to_jsonb(i)->>'customer_product_alias_id')::bigint,0)=0 AND COALESCE(to_jsonb(i)->>'customer_product_display_name_snapshot','')='' AND COALESCE(to_jsonb(i)->>'customer_item_code_snapshot','')='' AND btrim(COALESCE(to_jsonb(i)->>'item_note',''))=''`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allowed := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		allowed[id] = true
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []domain.CompanyStockTarget{}
	for _, v := range targets {
		if allowed[v.OrderItemID] {
			out = append(out, v)
		}
	}
	return out, nil
}
