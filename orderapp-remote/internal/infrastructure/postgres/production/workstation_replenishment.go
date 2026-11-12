package production

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"math"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
)

type replenishmentReceipt struct {
	Hash    string                        `json:"hash"`
	Command app.BatchMaterialIssueCommand `json:"command"`
	Result  app.BatchMaterialIssueResult  `json:"result"`
}

func (r Repository) draftBatchMaterialIssueOnce(ctx context.Context, cmd app.BatchMaterialIssueCommand) (app.BatchMaterialIssueResult, error) {
	result := app.BatchMaterialIssueResult{RequestID: cmd.RequestID, Groups: []app.BatchMaterialIssueGroupResult{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":workstation-replenishment:"+cmd.RequestID); err != nil {
		return result, err
	}
	fingerprint := app.BatchMaterialIssueHash(struct {
		Command app.BatchMaterialIssueCommand
		Actor   string
	}{cmd, cmd.Operator})
	var raw []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta FROM %s.audit_logs WHERE entity_type='workstation_replenishment' AND action='draft' AND meta->>'request_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), cmd.RequestID).Scan(&raw)
	if err == nil {
		var cancelled bool
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.audit_logs WHERE entity_type='workstation_replenishment' AND action='cancel' AND meta->>'request_id'=$1)`, r.schema), cmd.RequestID).Scan(&cancelled); e != nil {
			return result, e
		}
		if cancelled {
			return result, fmt.Errorf("原补料草稿已取消，请重新生成")
		}
		var stored replenishmentReceipt
		if e := json.Unmarshal(raw, &stored); e != nil {
			return result, e
		}
		if stored.Hash != fingerprint {
			return result, fmt.Errorf("请求标识已用于其他补料")
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
	if cmd.PreviewVersion != preview.Version {
		return result, fmt.Errorf("库存或任务已变化，请重新核对")
	}
	limits, err := r.replenishmentActualLimitsTx(ctx, tx, cmd)
	if err != nil {
		return result, err
	}
	byKey := map[string]app.BatchMaterialIssueGroup{}
	for _, g := range preview.Groups {
		byKey[g.Key] = g
	}
	groupTasks := map[string]map[int64]bool{}
	commands := map[int64]*stockapp.StockDocumentCommand{}
	order := []int64{}
	for _, sel := range cmd.Groups {
		g, ok := byKey[sel.Key]
		if !ok || g.Version != sel.Version {
			return result, fmt.Errorf("库存或任务已变化，请重新核对")
		}
		qty, e := app.BatchMaterialIssueBaseQuantity(g, sel.Quantity)
		if e != nil {
			return result, e
		}
		if cmd.Workflow != "replenishment" && limits != nil {
			key := replenishmentGroupKey(g)
			if qty > limits[key] {
				return result, fmt.Errorf("补料数量超过实际缺口")
			}
			limits[key] -= qty
		} else if cmd.Workflow != "replenishment" && qty > g.Remaining {
			return result, fmt.Errorf("补料数量超过当前缺口")
		}
		allocations, e := app.AllocateBatchMaterialIssue(g, qty)
		if e != nil {
			return result, e
		}
		row := app.BatchMaterialIssueGroupResult{Key: g.Key, Name: g.Name, Unit: g.Unit, QuantityBasis: g.QuantityBasis, Status: "draft", Quantity: qty, Remaining: max(int64(0), g.Remaining-qty), Documents: []app.BatchMaterialIssueDocument{}}
		for _, a := range allocations {
			if a.Quantity <= 0 {
				continue
			}
			if groupTasks[g.Key] == nil {
				groupTasks[g.Key] = map[int64]bool{}
			}
			groupTasks[g.Key][a.JobCardID] = true
			doc := commands[a.JobCardID]
			if doc == nil {
				doc = &stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, EntryType: "material_issue_to_wip", WorkOrderID: a.WorkOrderID, JobCardID: a.JobCardID, SourceType: "work_order", SourceID: a.WorkOrderID, ReturnSource: "workstation_replenishment", Operator: cmd.Operator, Note: "工位补料", IdempotencyKey: fmt.Sprintf("replenishment:%s:%d", cmd.RequestID, a.JobCardID)}
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
		}
		result.Groups = append(result.Groups, row)
	}
	repo := postgresstock.NewRepository(r.pool, r.schema)
	for _, id := range order {
		doc, e := repo.CreateWorkstationReplenishmentDraftTx(ctx, tx, *commands[id])
		if e != nil {
			return result, e
		}
		for i := range result.Groups {
			if !groupTasks[result.Groups[i].Key][id] {
				continue
			}
			result.Groups[i].Documents = append(result.Groups[i].Documents, app.BatchMaterialIssueDocument{ID: doc.ID, EntryNo: doc.EntryNo, WorkOrderID: doc.WorkOrderID, JobCardID: doc.JobCardID})
		}
	}
	result.SuccessCount = len(result.Groups)
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "workstation_replenishment", nil, "draft", nil, nil, nil, infra.AuditMeta{"request_id": cmd.RequestID, "hash": fingerprint, "command": cmd, "result": result}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
func (r Repository) confirmBatchMaterialIssueOnce(ctx context.Context, requestID, actor string) (app.BatchMaterialIssueResult, error) {
	var result app.BatchMaterialIssueResult
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":workstation-replenishment:"+requestID); err != nil {
		return result, err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta FROM %s.audit_logs WHERE entity_type='workstation_replenishment' AND action='draft' AND meta->>'request_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), requestID).Scan(&raw); err != nil {
		return result, fmt.Errorf("补料草稿不存在")
	}
	var receipt replenishmentReceipt
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return result, err
	}
	result = receipt.Result
	var confirmed bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.audit_logs WHERE entity_type='workstation_replenishment' AND action='confirm' AND meta->>'request_id'=$1)`, r.schema), requestID).Scan(&confirmed); err != nil {
		return result, err
	}
	if confirmed {
		for i := range result.Groups {
			result.Groups[i].Status = "submitted"
		}
		return result, nil
	}
	if err = r.lockBatchMaterialIssueTx(ctx, tx, receipt.Command.JobCardIDs); err != nil {
		return result, err
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, g := range result.Groups {
		for _, d := range g.Documents {
			if !seen[d.ID] {
				ids = append(ids, d.ID)
				seen[d.ID] = true
			}
		}
	}
	preview, err := r.previewBatchMaterialIssueIgnoringDraftsTx(ctx, tx, receipt.Command, ids)
	if err != nil {
		return result, err
	}
	if preview.Version != receipt.Command.PreviewVersion {
		return result, fmt.Errorf("库存、预留或缺口已变化，请处理草稿后重新核对")
	}
	limits, err := r.replenishmentActualLimitsTx(ctx, tx, receipt.Command)
	if err != nil {
		return result, err
	}
	if receipt.Command.Workflow != "replenishment" && limits != nil {
		groups := map[string]app.BatchMaterialIssueGroup{}
		for _, g := range preview.Groups {
			groups[g.Key] = g
		}
		for _, sel := range receipt.Command.Groups {
			g, ok := groups[sel.Key]
			if !ok {
				return result, fmt.Errorf("补料来源已变化")
			}
			qty, e := app.BatchMaterialIssueBaseQuantity(g, sel.Quantity)
			if e != nil {
				return result, e
			}
			key := replenishmentGroupKey(g)
			if qty > limits[key] {
				return result, fmt.Errorf("实际缺口已变化，请处理草稿后重新核对")
			}
			limits[key] -= qty
		}
	}
	repo := postgresstock.NewRepository(r.pool, r.schema)
	for _, id := range ids {
		if _, err = repo.SubmitWorkstationReplenishmentTx(ctx, tx, id, actor, receipt.Command.Workflow == "replenishment"); err != nil {
			return result, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "workstation_replenishment", nil, "confirm", nil, nil, nil, infra.AuditMeta{"request_id": requestID, "document_ids": ids}); err != nil {
		return result, err
	}
	for i := range result.Groups {
		result.Groups[i].Status = "submitted"
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// Replenishment limits are derived again from the server's WIP availability and
// validated operator-entered start quantities, never from a browser shortage.
func (r Repository) replenishmentActualLimitsTx(ctx context.Context, tx pgx.Tx, cmd app.BatchMaterialIssueCommand) (map[string]int64, error) {
	if cmd.StartInput == nil {
		limits := map[string]int64{}
		freeUsed := map[string]float64{}
		ownUsed := map[int64]float64{}
		productUsed := map[int64]int64{}
		productFreeUsed := map[int64]int64{}
		for _, id := range cmd.JobCardIDs {
			p, _, err := r.operationCompletionContextTx(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			if p.PredecessorRequired {
				return nil, fmt.Errorf("后续工序承接前序产出，不支持补料转仓")
			}
			for _, m := range p.Materials {
				if m.Warehouse != "wip" {
					continue
				}
				key := replenishmentMaterialKey(m.Key, m.BomVariantID, m.OwnerCustomerID, m.Unit)
				if m.ProductID > 0 && len(cmd.JobCardIDs) > 1 {
					covered, e := r.replenishmentProductCoverageTx(ctx, tx, m, productUsed, productFreeUsed, ownUsed)
					if e != nil {
						return nil, e
					}
					short := m.Qty - covered
					if short > 1e-8 {
						g := app.BatchMaterialIssueGroup{Unit: m.Unit, QuantityBasis: "count"}
						if app.OperationUnitFactor(m.Unit) > 0 {
							g.QuantityBasis = "weight"
						}
						q, e := app.BatchMaterialIssueBaseQuantity(g, short)
						if e != nil {
							return nil, e
						}
						limits[key] += q
					}
					continue
				}
				factor := app.OperationUnitFactor(m.Unit)
				if factor <= 0 {
					factor = 1
				}
				inventoryKey := fmt.Sprintf("%d/%d", m.MaterialID, m.OwnerCustomerID)
				own := m.AvailableQty
				if len(cmd.JobCardIDs) > 1 {
					own, err = r.replenishmentOwnWIPTx(ctx, tx, m)
					if err != nil {
						return nil, err
					}
				}
				own *= factor
				available := m.AvailableQty * factor
				demand := m.Qty * factor
				ownLeft := math.Max(0, own-ownUsed[m.ReservationID])
				free := math.Max(0, available-own-freeUsed[inventoryKey])
				covered := math.Min(demand, ownLeft+free)
				usedOwn := math.Min(covered, ownLeft)
				ownUsed[m.ReservationID] += usedOwn
				freeUsed[inventoryKey] += covered - usedOwn
				short := (demand - covered) / factor
				if short <= 1e-8 {
					continue
				}
				g := app.BatchMaterialIssueGroup{Unit: m.Unit, QuantityBasis: "count"}
				if app.OperationUnitFactor(m.Unit) > 0 {
					g.QuantityBasis = "weight"
				}
				qty, e := app.BatchMaterialIssueBaseQuantity(g, short)
				if e != nil {
					return nil, e
				}
				limits[key] += qty
			}
		}
		return limits, nil
	}
	if len(cmd.JobCardIDs) != 1 {
		return nil, fmt.Errorf("按实际投入补料只支持单个任务")
	}
	in := cmd.StartInput
	p, _, err := r.operationCompletionContextTx(ctx, tx, cmd.JobCardIDs[0], false)
	if err != nil {
		return nil, err
	}
	if p.Version != in.ExpectedVersion || p.PredecessorRequired || p.InputUnit != in.InputUnit {
		return nil, fmt.Errorf("任务或用料已变化，请重新核对")
	}
	if err = app.ValidateOperationQuantity(in.ActualInputQty, in.InputUnit); err != nil || in.ActualInputQty <= 0 {
		return nil, fmt.Errorf("请填写有效投入")
	}
	if math.Round(in.ActualInputQty*1e8) != math.Round(p.InputQty*1e8) && strings.TrimSpace(in.DifferenceReason) == "" {
		return nil, fmt.Errorf("实际投入与建议数量不一致，请填写差异原因")
	}

	expected := append([]app.OperationMaterialUse(nil), p.Materials...)
	if p.RecipeRatioPct > 0 {
		expected, err = app.ResolveBOMStartMaterialUses(expected, in.ActualInputQty, p.InputUnit)
		if err != nil {
			return nil, err
		}
	} else {
		if p.InputQty <= 0 {
			return nil, fmt.Errorf("本批计划数量无效")
		}
		for i := range expected {
			expected[i].Qty = operationLedgerQuantity(expected[i].Qty*in.ActualInputQty/p.InputQty, expected[i].Unit)
		}
	}
	if err = app.ValidateStartedInputMaterials(expected, in.Materials, in.DifferenceReason); err != nil {
		return nil, err
	}
	if err = app.ValidateStartedInputTotals(in.Materials, in.ActualInputQty, in.InputUnit, p.RecipeRatioPct > 0); err != nil {
		return nil, err
	}
	availability := map[string]app.OperationMaterialUse{}
	for _, m := range p.Materials {
		availability[m.Key] = m
	}
	out := map[string]int64{}
	for _, m := range in.Materials {
		v := availability[m.Key]
		if m.Warehouse != "wip" {
			continue
		}
		short := m.Qty - v.AvailableQty
		if short <= 1e-8 {
			continue
		}
		g := app.BatchMaterialIssueGroup{Unit: m.Unit, QuantityBasis: "count"}
		if app.OperationUnitFactor(m.Unit) > 0 {
			g.QuantityBasis = "weight"
		}
		qty, e := app.BatchMaterialIssueBaseQuantity(g, short)
		if e != nil {
			return nil, e
		}
		out[replenishmentMaterialKey(m.Key, m.BomVariantID, m.OwnerCustomerID, m.Unit)] += qty
	}
	return out, nil
}
func replenishmentMaterialKey(key string, variant, owner int64, unit string) string {
	return fmt.Sprintf("%s/%d/%d/%s", key, variant, owner, unit)
}
func replenishmentGroupKey(g app.BatchMaterialIssueGroup) string {
	kind := g.ItemType
	if kind == "finished_product" {
		kind = "product"
	}
	return replenishmentMaterialKey(fmt.Sprintf("%s:%d:%d:%d", kind, g.ItemID, g.BomSpecID, g.SpecG), g.BomVariantID, g.OwnerCustomerID, g.Unit)
}

func (r Repository) FindBatchReplenishment(ctx context.Context, jobCardID int64) (app.BatchReplenishmentRecovery, error) {
	out := app.BatchReplenishmentRecovery{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT a.meta FROM %[1]s.audit_logs a WHERE a.entity_type='workstation_replenishment' AND a.action='draft' AND a.meta->'command'->'job_card_ids' @> to_jsonb(ARRAY[$1]::bigint[]) AND NOT EXISTS(SELECT 1 FROM %[1]s.audit_logs c WHERE c.entity_type='workstation_replenishment' AND c.action='confirm' AND c.meta->>'request_id'=a.meta->>'request_id') ORDER BY a.id DESC`, r.schema), jobCardID)
	if err != nil {
		return out, err
	}
	var receipts []replenishmentReceipt
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return out, err
		}
		var receipt replenishmentReceipt
		if err = json.Unmarshal(raw, &receipt); err != nil {
			rows.Close()
			return out, err
		}
		receipts = append(receipts, receipt)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, receipt := range receipts {
		seen := map[int64]bool{}
		drafts, other := 0, 0
		for _, g := range receipt.Result.Groups {
			for _, doc := range g.Documents {
				if seen[doc.ID] {
					continue
				}
				seen[doc.ID] = true
				var status string
				e := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1`, r.schema), doc.ID).Scan(&status)
				if e != nil || status != "draft" {
					other++
				} else {
					drafts++
				}
			}
		}
		if drafts == 0 {
			continue
		}
		if other > 0 {
			return out, fmt.Errorf("补料组包含已变更单据，请先处理原单")
		}
		if out.Found {
			return out, fmt.Errorf("存在多个补料草稿组，请先处理原单")
		}
		cmd := receipt.Command
		result := receipt.Result
		out = app.BatchReplenishmentRecovery{Found: true, Command: &cmd, Result: &result}
	}
	return out, nil
}

func (r Repository) CancelBatchReplenishment(ctx context.Context, requestID, actor string) (app.BatchReplenishmentCancelResult, error) {
	out := app.BatchReplenishmentCancelResult{RequestID: requestID}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":workstation-replenishment:"+requestID); err != nil {
		return out, err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta FROM %s.audit_logs WHERE entity_type='workstation_replenishment' AND action='draft' AND meta->>'request_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), requestID).Scan(&raw); err != nil {
		return out, fmt.Errorf("补料草稿不存在")
	}
	var rec replenishmentReceipt
	if err = json.Unmarshal(raw, &rec); err != nil {
		return out, err
	}
	var confirmed, cancelled bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.audit_logs WHERE entity_type='workstation_replenishment' AND action='confirm' AND meta->>'request_id'=$1),EXISTS(SELECT 1 FROM %[1]s.audit_logs WHERE entity_type='workstation_replenishment' AND action='cancel' AND meta->>'request_id'=$1)`, r.schema), requestID).Scan(&confirmed, &cancelled); err != nil {
		return out, err
	}
	if confirmed {
		return out, fmt.Errorf("补料已过账，不能取消草稿")
	}
	if cancelled {
		out.Cancelled = true
		return out, nil
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, g := range rec.Result.Groups {
		for _, d := range g.Documents {
			if !seen[d.ID] {
				seen[d.ID] = true
				ids = append(ids, d.ID)
			}
		}
	}
	repo := postgresstock.NewRepository(r.pool, r.schema)
	for _, id := range ids {
		if err = repo.CancelWorkstationReplenishmentDraftTx(ctx, tx, id, actor); err != nil {
			return out, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "workstation_replenishment", nil, "cancel", nil, nil, nil, infra.AuditMeta{"request_id": requestID, "document_ids": ids}); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Cancelled = true
	return out, nil
}

// Reproduce the operation availability rules for the reservation-owned portion;
// only the free portion is shared among tasks in the same draft group.
func (r Repository) replenishmentOwnWIPTx(ctx context.Context, tx pgx.Tx, m app.OperationMaterialUse) (float64, error) {
	if m.ReservationID <= 0 {
		return 0, nil
	}
	if m.ProductID > 0 {
		return 0, fmt.Errorf("共享成品 WIP 请分别处理")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT mb.id,l.qty_g,l.qty_units,COALESCE(own.g,0)::bigint,COALESCE(own.units,0)::bigint,COALESCE(all_reserved.g,0)::bigint,COALESCE(all_reserved.units,0)::bigint,r.work_order_id,r.material_id
 FROM %[1]s.work_order_material_reservations r JOIN %[1]s.material_batches mb ON mb.material_id=r.material_id AND mb.status='active' AND mb.quality_status NOT IN('hold','reject') AND COALESCE(mb.owner_customer_id,0)=$2
 JOIN %[1]s.material_batch_locations l ON l.material_batch_id=mb.id AND l.warehouse='wip'
 LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)) g,SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)) units FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations rr ON rr.id=b.reservation_id AND rr.status='reserved' WHERE b.reservation_id=$1 AND b.material_batch_id=mb.id AND b.warehouse='wip' AND b.owner_customer_id=$2 AND b.status='reserved') own ON true
 LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)) g,SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)) units FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations rr ON rr.id=b.reservation_id AND rr.status='reserved' WHERE b.material_batch_id=mb.id AND b.warehouse='wip' AND b.owner_customer_id=$2 AND b.status='reserved') all_reserved ON true
 WHERE r.id=$1 AND r.status IN('reserved','consumed') AND r.source_owner_customer_id=$2`, r.schema), m.ReservationID, m.OwnerCustomerID)
	if err != nil {
		return 0, err
	}
	type entry struct{ id, physicalG, physicalN, ownG, ownN, reservedG, reservedN, work, material int64 }
	var list []entry
	for rows.Next() {
		var v entry
		if err = rows.Scan(&v.id, &v.physicalG, &v.physicalN, &v.ownG, &v.ownN, &v.reservedG, &v.reservedN, &v.work, &v.material); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	var grams, units int64
	for _, v := range list {
		pg, pn, e := postgresstock.OperationOtherProcessingReservedTx(ctx, tx, r.schema, v.work, v.id, v.material, m.OwnerCustomerID, "wip", false)
		if e != nil {
			return 0, e
		}
		grams += min(v.ownG, max(int64(0), v.physicalG-max(int64(0), v.reservedG-v.ownG)-pg))
		units += min(v.ownN, max(int64(0), v.physicalN-max(int64(0), v.reservedN-v.ownN)-pn))
	}
	if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
		return float64(grams) / factor, nil
	}
	return float64(units), nil
}

func retryableReplenishment(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && (pg.Code == "40001" || pg.Code == "40P01")
}
func (r Repository) DraftBatchMaterialIssue(ctx context.Context, cmd app.BatchMaterialIssueCommand) (app.BatchMaterialIssueResult, error) {
	var out app.BatchMaterialIssueResult
	var err error
	for i := 0; i < 3; i++ {
		out, err = r.draftBatchMaterialIssueOnce(ctx, cmd)
		if !retryableReplenishment(err) {
			return out, err
		}
	}
	return out, err
}
func (r Repository) ConfirmBatchMaterialIssue(ctx context.Context, id, actor string) (app.BatchMaterialIssueResult, error) {
	var out app.BatchMaterialIssueResult
	var err error
	for i := 0; i < 3; i++ {
		out, err = r.confirmBatchMaterialIssueOnce(ctx, id, actor)
		if !retryableReplenishment(err) {
			return out, err
		}
	}
	return out, err
}

func (r Repository) replenishmentProductCoverageTx(ctx context.Context, tx pgx.Tx, m app.OperationMaterialUse, batchUsed map[int64]int64, freeUsed map[int64]int64, ownUsed map[int64]float64) (float64, error) {
	factor := app.OperationUnitFactor(m.Unit)
	if factor <= 0 {
		factor = 1
	}
	var hasBindings bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.work_order_material_reservation_batches WHERE reservation_id=$1 AND status='reserved')`, r.schema), m.ReservationID).Scan(&hasBindings); err != nil {
		return 0, err
	}
	if !hasBindings {
		remaining := int64(math.Round(m.Qty * factor))
		var covered int64
		for _, b := range m.EligibleBatches {
			available := int64(math.Round(b.AvailableQty * factor))
			take := min(remaining, max(int64(0), available-freeUsed[b.BatchID]))
			remaining -= take
			covered += take
			freeUsed[b.BatchID] += take
			batchUsed[b.BatchID] += take
			if remaining <= 0 {
				break
			}
		}
		return float64(covered) / factor, nil
	}
	remaining := int64(math.Round(m.Qty * factor))
	usedReservation := int64(math.Round(ownUsed[m.ReservationID]))
	var covered int64
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT sb.id,GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g),GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units),GREATEST(0,sb.remaining_g),GREATEST(0,sb.remaining_units) FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.stock_batches sb ON sb.id=b.stock_batch_id AND sb.quality_status NOT IN('hold','reject') WHERE b.reservation_id=$1 AND b.status='reserved' AND b.warehouse='wip' AND b.owner_customer_id=$2 AND COALESCE(sb.owner_customer_id,0)=$2 ORDER BY sb.id,b.id`, r.schema), m.ReservationID, m.OwnerCustomerID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, rg, rn, pg, pn int64
		if err = rows.Scan(&id, &rg, &rn, &pg, &pn); err != nil {
			return 0, err
		}
		res, physical := rn, pn
		if app.OperationUnitFactor(m.Unit) > 0 {
			res, physical = rg, pg
		}
		skip := min(res, usedReservation)
		usedReservation -= skip
		res -= skip
		qty := min(remaining, min(res, max(int64(0), physical-batchUsed[id])))
		covered += qty
		remaining -= qty
		batchUsed[id] += qty
		if remaining <= 0 {
			break
		}
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	ownUsed[m.ReservationID] += float64(covered)
	return float64(covered) / factor, nil
}
