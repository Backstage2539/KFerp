package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

func (r Repository) productionStopSources(ctx context.Context, q querier, c *domain.Context) error {
	for n := range c.Dependencies {
		d := &c.Dependencies[n]
		if d.Kind != "production" || one(d.Status, "completed", "cancelled", "stopped") {
			continue
		}
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT jsonb_build_object('work',to_jsonb(w),
 'cards',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.job_cards x WHERE x.work_order_id=w.id),'[]'),
 'inputs',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.production_started_inputs x WHERE x.work_order_id=w.id),'[]'),
 'components',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.production_started_input_components x JOIN %[1]s.production_started_inputs i ON i.id=x.input_id WHERE i.work_order_id=w.id),'[]'),
 'events',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.production_operation_events x WHERE x.work_order_id=w.id),'[]'),
 'balances',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.production_operation_balances x WHERE x.work_order_id=w.id),'[]'),
 'handoffs',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.production_started_handoffs x JOIN %[1]s.job_cards j ON j.id=x.job_card_id WHERE j.work_order_id=w.id),'[]'),
 'reservations',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.work_order_material_reservations x WHERE x.work_order_id=w.id OR (w.running_item_id>0 AND x.running_item_id=w.running_item_id)),'[]'),
 'bindings',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.work_order_material_reservation_batches x WHERE x.work_order_id=w.id),'[]'),
 'supply',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.production_supply_allocations x WHERE x.work_order_id=w.id OR x.supplier_work_order_id=w.id),'[]'),
 'dependencies',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.work_order_dependencies x WHERE x.work_order_id=w.id OR x.depends_on_work_order_id=w.id),'[]'),
 'outputs',COALESCE((SELECT jsonb_agg(to_jsonb(x)||jsonb_build_object('warehouse',COALESCE((SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.item_type=x.item_type AND l.item_id=x.item_id AND l.bom_spec_id=x.bom_spec_id AND l.bom_variant_id=x.bom_variant_id AND l.owner_customer_id=x.owner_customer_id AND l.source_batch_code=x.batch_code ORDER BY l.id DESC LIMIT 1),'')) ORDER BY x.id) FROM %[1]s.stock_batches x WHERE (x.source_doc_type='production_run' AND x.source_doc_id=w.running_item_id AND w.running_item_id>0) OR (x.source_doc_type IN('production_operation_event','operation_completion') AND x.source_doc_id IN(SELECT id FROM %[1]s.production_operation_events WHERE work_order_id=w.id))),'[]'),
 'run',COALESCE((SELECT to_jsonb(x) FROM %[1]s.produce_running_items x WHERE x.id=w.running_item_id),'{}'))
 FROM %[1]s.work_orders w WHERE w.production_plan_id=$1 AND w.status NOT IN('completed','cancelled','stopped') ORDER BY w.id`, r.schema), d.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			var snap map[string]json.RawMessage
			if err = json.Unmarshal(raw, &snap); err != nil {
				rows.Close()
				return err
			}
			var w map[string]json.RawMessage
			_ = json.Unmarshal(snap["work"], &w)
			s := domain.StopWorkOrder{WorkOrderID: int64(num(w, "id")), Label: str(w, "work_order_no"), Version: hash(raw), Remainders: []domain.ProductionRemainder{}}
			if strings.TrimSpace(str(w, "order_nos")) != c.OrderNo {
				s.Issue = "工单还关联其他需求或缺少本单来源，请先拆分并核对，不能整批停工"
			}
			if str(w, "material_consumption_policy") != "bom_start_v1" {
				s.Issue = "旧生产缺少逐批开工余料记录，请先核对生产历史，不能自动停工"
			}
			if num(w, "processing_request_item_id") > 0 {
				s.Issue = "客户来料加工须先核对加工委托及专属预留，不能在销售单直接停工"
			}
			var linked []json.RawMessage
			_ = json.Unmarshal(snap["dependencies"], &linked)
			if len(linked) > 0 {
				s.Issue = "工单存在上下游工序依赖，请先拆分并核对关联生产，不能整单停工"
			}
			for _, key := range []string{"inputs", "components", "balances", "reservations", "outputs", "supply"} {
				var items []map[string]json.RawMessage
				_ = json.Unmarshal(snap[key], &items)
				for _, v := range items {
					row := domain.ProductionRemainder{Kind: key, ID: int64(num(v, "id")), Warehouse: str(v, "warehouse"), OwnerCustomerID: int64(num(v, "owner_customer_id")), Unit: str(v, "unit"), Quantity: num(v, "remaining_qty"), Cost: num(v, "remaining_cost")}
					switch key {
					case "inputs":
						if num(v, "remaining_qty") > 0 && str(v, "status") == "posted" {
							s.HasMaterials = true
						}
						continue
					case "components":
						s.HasMaterials = s.HasMaterials || row.Quantity > 0
						row.Label = fmt.Sprintf("已投余料 %s #%d · 原批次 #%d", str(v, "component_type"), int64(num(v, "component_id")), int64(num(v, "material_batch_id")+num(v, "stock_batch_id")))
					case "balances":
						s.HasWIP = s.HasWIP || row.Quantity > 0
						row.Label = fmt.Sprintf("在制品 · 工序 %d · %s", int64(num(v, "sequence_no")), str(v, "quality_status"))
					case "reservations":
						if str(v, "status") != "reserved" {
							continue
						}
						row.Quantity = max(0, num(v, "reserved_g")-num(v, "consumed_g")-num(v, "returned_g"))
						row.Unit = "g"
						if row.Quantity == 0 {
							row.Quantity = max(0, num(v, "reserved_units")-num(v, "consumed_units")-num(v, "returned_units"))
							row.Unit = str(v, "unit")
						}
						row.Label = "未耗用预留 · " + str(v, "material_name")
						row.Warehouse = str(v, "source_warehouse")
						row.OwnerCustomerID = int64(num(v, "source_owner_customer_id"))
						s.HasReservations = s.HasReservations || row.Quantity > 0
					case "outputs":
						s.HasOutputs = true
						row.Label = "已完成产出 · " + str(v, "batch_code")
						row.Quantity = num(v, "remaining_units")
						row.Unit = str(w, "inventory_unit")
						row.Cost = row.Quantity * num(v, "unit_cost")
						if num(v, "remaining_g") > 0 {
							row.Cost = num(v, "remaining_g") / 1000 * num(v, "unit_cost")
						}
						if row.Quantity == 0 {
							row.Quantity = num(v, "remaining_g")
							row.Unit = "g"
						}
					case "supply":
						if !one(str(v, "status"), "released", "cancelled") {
							s.Issue = "工单存在上下游供料关系，请先拆分并核对关联生产，不能整单停工"
						}
						continue
					}
					if row.Quantity > 0 || key == "outputs" {
						s.Remainders = append(s.Remainders, row)
					}
				}
			}
			if s.HasMaterials {
				found := false
				for _, row := range s.Remainders {
					if row.Kind == "components" && row.Quantity > 0 {
						found = true
					}
				}
				if !found {
					s.Issue = "已投余料缺少逐批数量、货权及位置记录，请先核对来源"
				}
			}
			d.StopWorkOrders = append(d.StopWorkOrders, s)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(d.StopWorkOrders)
		c.Version = hash([]byte(c.Version + string(raw)))
	}
	return nil
}

func verifyProductionStopIdentity(original, current domain.Context, cmd domain.Command) error {
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "stop" {
			continue
		}
		find := func(c domain.Context) []domain.StopWorkOrder {
			for _, dep := range c.Dependencies {
				if dep.Kind == d.Kind && dep.ID == d.ID {
					return dep.StopWorkOrders
				}
			}
			return nil
		}
		before, after := find(original), find(current)
		if len(before) == 0 || len(before) != len(after) {
			return fmt.Errorf("%w：停工工单已变化，请重新核对", ErrStale)
		}
		for i := range before {
			if before[i].WorkOrderID != after[i].WorkOrderID || before[i].Version != after[i].Version {
				return fmt.Errorf("%w：余料、在制品、预留或工序记录已变化，请重新核对", ErrStale)
			}
		}
	}
	return nil
}

func (r Repository) applyProductionStops(ctx context.Context, tx pgx.Tx, orderID, requestID int64, cmd domain.Command, actor string) error {
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "stop" {
			continue
		}
		for _, s := range d.Stops {
			raw, _ := json.Marshal(s)
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_production_stops(request_id,order_id,plan_id,work_order_id,selection_json,snapshot_json,actor)
 SELECT $1,$2,$3,w.id,$5,jsonb_build_object('work',to_jsonb(w),'cards',(SELECT jsonb_agg(to_jsonb(j) ORDER BY id) FROM %[1]s.job_cards j WHERE j.work_order_id=w.id),'reservations',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM %[1]s.work_order_material_reservations m WHERE m.work_order_id=w.id),'run',(SELECT to_jsonb(p) FROM %[1]s.produce_running_items p WHERE p.id=w.running_item_id)),$6 FROM %[1]s.work_orders w WHERE w.id=$4 AND w.production_plan_id=$3`, r.schema), requestID, orderID, d.ID, s.WorkOrderID, raw, actor); err != nil {
				return err
			}
			if s.Reservations == "release_unconsumed" {
				for _, table := range []string{"work_order_material_reservation_batches", "work_order_material_reservations"} {
					where := "work_order_id=$1"
					if table == "work_order_material_reservations" {
						where = "(work_order_id=$1 OR (running_item_id>0 AND running_item_id=(SELECT running_item_id FROM " + r.schema + ".work_orders WHERE id=$1)))"
					}
					if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.%[2]s SET status='released',returned_g=GREATEST(0,reserved_g-consumed_g),returned_units=GREATEST(0,reserved_units-consumed_units),updated_at=now() WHERE %[3]s AND status='reserved'`, r.schema, table, where), s.WorkOrderID); err != nil {
						return err
					}
				}
			}
			for _, statement := range []string{`UPDATE %[1]s.job_cards SET status='stopped' WHERE work_order_id=$1 AND status NOT IN('completed','cancelled')`, `UPDATE %[1]s.produce_running_items SET status='stopped' WHERE id IN(SELECT running_item_id FROM %[1]s.work_orders WHERE id=$1) AND status NOT IN('done','cancelled')`, `UPDATE %[1]s.work_orders SET status='stopped' WHERE id=$1`} {
				if _, err := tx.Exec(ctx, fmt.Sprintf(statement, r.schema), s.WorkOrderID); err != nil {
					return err
				}
			}
			if err := infra.AuditInsertTx(ctx, tx, r.schema, actor, "work_order", &s.WorkOrderID, "resolution_production_stop", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "order_id": orderID, "selection": s}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET status='stopped' WHERE id=$1`, r.schema), d.ID); err != nil {
			return err
		}
	}
	return nil
}
