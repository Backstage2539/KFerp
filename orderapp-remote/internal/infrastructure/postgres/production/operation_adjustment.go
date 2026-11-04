package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strings"
)

func (r Repository) AdjustOperationMaterials(ctx context.Context, c app.OperationAdjustmentCommand) (app.JobCardActionResult, error) {
	return retryOperationTransaction(ctx, func() (app.JobCardActionResult, error) {
		var out app.JobCardActionResult
		if c.EmployeeID <= 0 || strings.TrimSpace(c.Reason) == "" || c.RequestID == "" || c.ExpectedVersion == "" {
			return out, fmt.Errorf("请确认员工、原因和当前版本后重试")
		}
		if c.Kind != "material_add" && c.Kind != "material_return" && c.Kind != "start_reversal" {
			return out, fmt.Errorf("不支持的用料调整")
		}
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return out, err
		}
		defer tx.Rollback(ctx)
		week, _, err := app.NormalizeProductionRosterWeek(productionWorkDate(), nil)
		if err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_roster:"+week); err != nil {
			return out, err
		}
		var work, run int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), c.ID).Scan(&work, &run); err != nil {
			return out, err
		}
		if run > 0 {
			if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, run); err != nil {
				return out, err
			}
		}
		if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, work); err != nil {
			return out, err
		}
		rid, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_adjustment", c.Operator, c.RequestID, c)
		if err != nil {
			return out, err
		}
		if rid > 0 {
			err = json.Unmarshal(replay, &out)
			return out, err
		}
		p, wo, err := r.operationCompletionContextTx(ctx, tx, c.ID, true)
		if err != nil {
			return out, err
		}
		if p.MaterialConsumptionPolicy != "bom_start_v1" {
			return out, fmt.Errorf("旧工单请沿用原执行流程")
		}
		if p.Version != c.ExpectedVersion {
			return out, fmt.Errorf("任务或库存已变化，请刷新；本次未调整")
		}
		var status, station string
		var stationID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,workstation,workstation_id FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), c.ID).Scan(&status, &station, &stationID); err != nil {
			return out, err
		}
		if (status != "running" && status != "paused") || wo.Status == "completed" || wo.Status == "cancelled" {
			return out, fmt.Errorf("只有运行中或暂停的批次可以调整用料")
		}
		owner, roster, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
		if err != nil {
			return out, err
		}
		if roster && owner.EmployeeID != c.EmployeeID {
			return out, fmt.Errorf("请由今日工位负责人调整用料")
		}
		if c.Kind != "start_reversal" && p.PredecessorRequired {
			return out, fmt.Errorf("后续工序承接前序产出，不调整本 BOM 原料")
		}
		if c.Kind == "start_reversal" {
			var used bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted')`, r.schema), c.ID).Scan(&used); err != nil {
				return out, err
			}
			if used {
				return out, fmt.Errorf("本批已有报工，请通过生产记录更正并先处理下游依赖")
			}
		}
		if c.Kind != "material_add" && !c.UnprocessedConfirmed {
			return out, fmt.Errorf("请确认所退物料尚未投入加工")
		}
		payload, err := json.Marshal(c)
		if err != nil {
			return out, err
		}
		var event int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,request_id,expected_version,operator,input_unit,output_unit,warehouse,difference_reason,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb) RETURNING id`, r.schema), wo.ID, c.ID, c.Kind, c.RequestID, c.ExpectedVersion, c.Operator, p.InputUnit, p.OutputUnit, p.Warehouse, c.Reason, string(payload)).Scan(&event); err != nil {
			return out, err
		}
		materialCost := 0.0
		if c.Kind == "material_add" {
			if c.InputUnit != p.InputUnit || c.ActualInputQty <= 0 {
				return out, fmt.Errorf("请填写补料增加的实际处理投入及单位")
			}
			if err = app.ValidateOperationQuantity(c.ActualInputQty, c.InputUnit); err != nil {
				return out, err
			}
			if err = app.ValidateStartedInputMaterials(p.Materials, c.Materials, c.Reason); err != nil {
				return out, err
			}
			if err = app.ValidateStartedInputTotals(c.Materials, c.ActualInputQty, c.InputUnit, p.RecipeRatioPct > 0); err != nil {
				return out, err
			}
			cmd := app.JobCardActionCommand{ID: c.ID, RequestID: c.RequestID, Operator: c.Operator, ActualInputQty: c.ActualInputQty, InputUnit: c.InputUnit, Materials: c.Materials}
			doc, e := r.consumeStartedMaterialsTx(ctx, tx, wo, p, cmd, event, c.Kind)
			if e != nil {
				return out, e
			}
			materialCost = doc.TotalCost
		} else {
			inputs, err := r.startedInputsTx(ctx, tx, c.ID)
			if err != nil {
				return out, err
			}
			found := c.Kind == "start_reversal"
			for _, input := range inputs {
				if c.Kind != "start_reversal" && input.ID != c.SourceInputID {
					continue
				}
				found = true
				returns := c.Returns
				reduce := c.ActualInputQty
				if c.Kind == "start_reversal" {
					returns = nil
					reduce = input.RemainingQty
					for _, component := range input.Components {
						if component.RemainingQty > 0 {
							returns = append(returns, app.ComponentReturn{ID: component.ID, Qty: component.RemainingQty})
						}
					}
				}
				if len(returns) == 0 && c.Kind == "start_reversal" && input.RemainingQty == 0 {
					if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET status='reversed' WHERE id=$1`, r.schema), input.ID); err != nil {
						return out, err
					}
					continue
				}
				if len(returns) == 0 {
					return out, fmt.Errorf("请选择本次退回的物料")
				}
				if reduce < 0 || reduce > input.RemainingQty || math.IsNaN(reduce) || math.IsInf(reduce, 0) {
					return out, fmt.Errorf("退料减少的处理投入超过余额")
				}
				if err = app.ValidateOperationQuantity(reduce, input.Unit); err != nil {
					return out, err
				}
				ids := map[int64]bool{}
				totalG, returnedCost := 0.0, 0.0
				for _, row := range returns {
					valid := false
					for _, component := range input.Components {
						if row.ID == component.ID {
							valid = true
							if component.Unit == "g" {
								totalG += row.Qty
							}
						}
					}
					if !valid || ids[row.ID] {
						return out, fmt.Errorf("退料来源无效或重复")
					}
					ids[row.ID] = true
					amount, e := stockrepo.NewRepository(r.pool, r.schema).ReturnStartedComponentTx(ctx, tx, row.ID, event, row.Qty, c.Operator)
					if e != nil {
						return out, e
					}
					returnedCost += amount
				}
				if p.RecipeRatioPct > 0 && math.Abs(reduce*app.OperationUnitFactor(input.Unit)-totalG) > 1e-8 {
					return out, fmt.Errorf("比例配方退料合计必须等于减少的实际投入")
				}
				if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3,status=CASE WHEN $4 THEN 'reversed' ELSE status END WHERE id=$1`, r.schema), input.ID, reduce, returnedCost, c.Kind == "start_reversal"); err != nil {
					return out, err
				}
				var stranded bool
				if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.remaining_qty=0 AND EXISTS(SELECT 1 FROM %[1]s.production_started_input_components c WHERE c.input_id=b.id AND c.remaining_qty>0) FROM %[1]s.production_started_inputs b WHERE b.id=$1`, r.schema), input.ID).Scan(&stranded); err != nil {
					return out, err
				}
				if stranded {
					return out, fmt.Errorf("处理投入归零前，请先处理该投入记录的全部剩余物料")
				}
				materialCost -= returnedCost
			}
			if !found {
				return out, fmt.Errorf("原投入记录不存在")
			}
		}
		if c.Kind != "material_add" {
			if err = r.recordStartedInputReturnLogsTx(ctx, tx, wo, event, c.Operator); err != nil {
				return out, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2 WHERE id=$1`, r.schema), event, materialCost); err != nil {
			return out, err
		}
		if c.Kind == "start_reversal" {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_handoffs SET remaining_qty=0,status='reversed' WHERE job_card_id=$1 AND status='posted'`, r.schema), c.ID); err != nil {
				return out, err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',started_at=NULL,resumed_at=NULL,paused_at=NULL,metrics_json=metrics_json-'timer_elapsed_seconds' WHERE id=$1`, r.schema), c.ID); err != nil {
				return out, err
			}
		} else {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET operator=$2 WHERE id=$1`, r.schema), c.ID, c.Operator); err != nil {
				return out, err
			}
		}
		if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "job_card", &c.ID, c.Kind, nil, nil, nil, infra.AuditMeta{"event_id": event, "reason": c.Reason, "material_cost": materialCost, "source_input_id": c.SourceInputID}); err != nil {
			return out, err
		}
		out.JobCard, err = loadJobCardRowTx(ctx, tx, r.schema, c.ID)
		if err != nil {
			return out, err
		}
		out.WorkOrder, err = loadWorkOrderExecutionRowTx(ctx, tx, r.schema, wo.ID)
		if err != nil {
			return out, err
		}
		if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_adjustment", c.Operator, c.RequestID, c.ID, out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	})
}

func (r Repository) startedInputsTx(ctx context.Context, tx pgx.Tx, card int64) ([]app.StartedInputDetail, error) {
	out := []app.StartedInputDetail{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,remaining_qty::float8,input_unit FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted' ORDER BY id`, r.schema), card)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var i app.StartedInputDetail
		if err = rows.Scan(&i.ID, &i.RemainingQty, &i.Unit); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for index := range out {
		rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT c.id,i.item_name,COALESCE(m.batch_code,s.batch_code,''),c.warehouse,c.owner_customer_id,c.unit,c.remaining_qty::float8,c.remaining_cost::float8 FROM %[1]s.production_started_input_components c JOIN %[1]s.stock_entry_items i ON i.id=c.stock_entry_item_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches s ON s.id=c.stock_batch_id WHERE c.input_id=$1 ORDER BY c.id`, r.schema), out[index].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var c app.StartedComponentBalance
			if err = rows.Scan(&c.ID, &c.Name, &c.BatchCode, &c.Warehouse, &c.OwnerCustomerID, &c.Unit, &c.RemainingQty, &c.RemainingCost); err != nil {
				rows.Close()
				return nil, err
			}
			out[index].Components = append(out[index].Components, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
