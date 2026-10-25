package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
)

type completionGroupSource struct {
	cardID    int64
	workID    int64
	runID     int64
	no        string
	seq       int
	operation string
	station   string
	p         app.OperationCompletionPreview
	wo        app.WorkOrderRow
}

func (r Repository) PreviewOperationCompletionGroup(ctx context.Context, ids []int64) (app.OperationCompletionGroupPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.OperationCompletionGroupPreview{}, err
	}
	defer tx.Rollback(ctx)
	out, _, err := r.operationCompletionGroupContextTx(ctx, tx, ids, false)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (r Repository) operationCompletionGroupContextTx(ctx context.Context, tx pgx.Tx, ids []int64, lock bool) (app.OperationCompletionGroupPreview, []completionGroupSource, error) {
	out := app.OperationCompletionGroupPreview{Items: []app.OperationCompletionGroupItem{}, Materials: []app.OperationCompletionGroupMaterial{}}
	sources := make([]completionGroupSource, 0, len(ids))
	seenWorkOrders := map[int64]bool{}
	for _, cardID := range ids {
		var row completionGroupSource
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id,w.work_order_no,j.sequence_no,j.operation,j.workstation FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema, r.schema), cardID).Scan(&row.workID, &row.runID, &row.no, &row.seq, &row.operation, &row.station); err != nil {
			return out, nil, fmt.Errorf("所选包装任务已变化，请刷新")
		}
		row.cardID = cardID
		if seenWorkOrders[row.workID] {
			return out, nil, fmt.Errorf("同一工单包含多个包装任务，请逐卡处理")
		}
		seenWorkOrders[row.workID] = true
		p, wo, err := r.operationCompletionContextTx(ctx, tx, cardID, lock)
		if err != nil {
			return out, nil, err
		}
		row.p, row.wo = p, wo
		if row.p.BlockedReason != "" {
			return out, nil, fmt.Errorf("%s：%s", row.no, row.p.BlockedReason)
		}
		if !row.p.Enabled || !row.p.Final || !row.p.Execution.ReceiptEnabled || !isPackagingOperationName(taskOperationForCard(ctx, tx, r.schema, cardID)) {
			return out, nil, fmt.Errorf("请选择同一规格的可入库包装工序")
		}
		if row.wo.Status == "completed" || row.wo.Status == "cancelled" {
			return out, nil, fmt.Errorf("%s：工单已结束", row.no)
		}
		sources = append(sources, row)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].no != sources[j].no {
			return sources[i].no < sources[j].no
		}
		return sources[i].cardID < sources[j].cardID
	})
	if len(sources) < 2 {
		return out, nil, fmt.Errorf("请选择至少两张同组包装工序卡")
	}
	groupIdentity := completionGroupIdentity(sources[0])
	versionParts := make([]any, 0, len(sources))
	materialMap := map[string]*app.OperationCompletionGroupMaterial{}
	sharedFree := map[string]float64{}
	seenOwnReservations := map[string]bool{}
	out.GroupKey = fmt.Sprintf("packaging:%x", sha256.Sum256([]byte(groupIdentity)))
	out.OutputUnit = sources[0].p.OutputUnit
	out.WarehouseName = sources[0].p.WarehouseName
	out.MaxOutputQty = 0
	for _, src := range sources {
		if completionGroupIdentity(src) != groupIdentity {
			return out, nil, fmt.Errorf("所选工序的商品、规格、物料货主或目标仓不同，不能合并")
		}
		versionParts = append(versionParts, []any{src.cardID, src.p.Version, src.p.PlannedQty, src.p.CompletedOutputQty})
		item := app.OperationCompletionGroupItem{JobCardID: src.cardID, WorkOrderID: src.workID, WorkOrderNo: src.no, ProductName: src.p.ProductName, SpecG: src.p.OutputSpecG, PlannedQty: src.p.TotalPlannedQty, CompletedQty: src.p.CompletedOutputQty, RemainingQty: src.p.PlannedQty, Unit: src.p.OutputUnit, Warehouse: src.p.Warehouse, WarehouseName: src.p.WarehouseName, Version: src.p.Version}
		out.Items = append(out.Items, item)
		out.PlannedQty += src.p.TotalPlannedQty
		out.CompletedQty += src.p.CompletedOutputQty
		out.RemainingQty += src.p.PlannedQty
		out.MaxOutputQty += src.p.MaxOutputQty
		for _, material := range src.p.Materials {
			key := fmt.Sprintf("%s:%d:%s:%s:%d", material.Key, material.OwnerCustomerID, material.Warehouse, material.Unit, material.SpecG)
			grouped := materialMap[key]
			if grouped == nil {
				grouped = &app.OperationCompletionGroupMaterial{Key: key, MaterialID: material.MaterialID, Name: material.Name, Unit: material.Unit, Warehouse: material.Warehouse, WarehouseName: material.WarehouseName, OwnerCustomerID: material.OwnerCustomerID, Sources: []app.OperationMaterialUse{}}
				materialMap[key] = grouped
			}
			grouped.RequiredQty += material.Qty
			ownAvailable, freeAvailable, err := operationMaterialAvailabilityPartsTx(ctx, tx, r.schema, material.ReservationID, material.Warehouse, material.OwnerCustomerID, material.Unit)
			if err != nil {
				return out, nil, err
			}
			reservationKey := fmt.Sprintf("%s:%d", key, material.ReservationID)
			if !seenOwnReservations[reservationKey] {
				grouped.AvailableQty += ownAvailable
				seenOwnReservations[reservationKey] = true
			}
			if prior, ok := sharedFree[key]; ok {
				// Use the conservative common free balance when reservations
				// are sourced from the same inventory identity.
				sharedFree[key] = math.Min(prior, freeAvailable)
			} else {
				sharedFree[key] = freeAvailable
			}
			grouped.Sources = append(grouped.Sources, material)
		}
	}
	for _, material := range materialMap {
		material.AvailableQty += sharedFree[material.Key]
		material.ShortageQty = math.Max(0, material.RequiredQty-material.AvailableQty)
		out.Materials = append(out.Materials, *material)
	}
	out.MaxOutputQty = app.OperationGroupMaxOutputQty(out.RemainingQty, out.OutputUnit, out.Materials)
	sort.Slice(out.Materials, func(i, j int) bool { return out.Materials[i].Name < out.Materials[j].Name })
	raw, _ := json.Marshal(versionParts)
	out.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	return out, sources, nil
}

func completionGroupIdentity(src completionGroupSource) string {
	materials := make([]string, 0, len(src.p.Materials))
	for _, m := range src.p.Materials {
		planned := src.p.TotalPlannedQty
		if planned <= 0 {
			planned = src.p.PlannedQty
		}
		materials = append(materials, fmt.Sprintf("%s:%d:%s:%s:%.8f", m.Key, m.OwnerCustomerID, m.Warehouse, m.Unit, m.TotalQty/math.Max(1e-9, planned)))
	}
	sort.Strings(materials)
	productID := src.wo.OutputProductID
	if productID <= 0 {
		productID = src.wo.ProductID
	}
	return fmt.Sprintf("%d:%s:%d:%s:%s:%s:%d:%s:%s:%s", productID, src.p.ProductName, src.p.OutputSpecG, src.p.OutputUnit, src.p.Warehouse, src.p.Execution.ReceiptWarehouse, src.seq, strings.TrimSpace(src.operation), strings.TrimSpace(src.station), strings.Join(materials, ";"))
}

func taskOperationForCard(ctx context.Context, tx pgx.Tx, schema string, id int64) string {
	var operation string
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation FROM %s.job_cards WHERE id=$1`, schema), id).Scan(&operation)
	return operation
}

func (r Repository) CompleteOperationGroup(ctx context.Context, cmd app.OperationCompletionGroupCommand) (app.OperationCompletionGroupResult, error) {
	return retryOperationTransaction(ctx, func() (app.OperationCompletionGroupResult, error) { return r.completeOperationGroupOnce(ctx, cmd) })
}

func (r Repository) completeOperationGroupOnce(ctx context.Context, cmd app.OperationCompletionGroupCommand) (app.OperationCompletionGroupResult, error) {
	out := app.OperationCompletionGroupResult{RequestedQty: cmd.ActualOutputQty, OutputUnit: cmd.OutputUnit, Items: []app.JobCardActionResult{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	ids := append([]int64(nil), cmd.JobCardIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	lockSources := []completionGroupSource{}
	for _, id := range ids {
		var src completionGroupSource
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id,w.work_order_no,j.sequence_no,j.operation,j.workstation FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema, r.schema), id).Scan(&src.workID, &src.runID, &src.no, &src.seq, &src.operation, &src.station); err != nil {
			return out, fmt.Errorf("所选包装任务已变化，请刷新")
		}
		if src.runID > 0 {
			if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, src.runID); err != nil {
				return out, err
			}
		}
		lockSources = append(lockSources, src)
	}
	workOrders := map[int64]bool{}
	for _, src := range lockSources {
		if workOrders[src.workID] {
			continue
		}
		if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, src.workID); err != nil {
			return out, err
		}
		workOrders[src.workID] = true
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_complete_group", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return out, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	preview, sources, err := r.operationCompletionGroupContextTx(ctx, tx, cmd.JobCardIDs, true)
	if err != nil {
		return out, err
	}
	if preview.Version != cmd.ExpectedVersion {
		return out, fmt.Errorf("任务或库存已变化，请刷新；本次没有扣料或入库")
	}
	if cmd.OutputUnit != preview.OutputUnit {
		return out, fmt.Errorf("完成数量单位与冻结包装规格不一致")
	}
	if cmd.ActualOutputQty > preview.MaxOutputQty+1e-8 {
		return out, fmt.Errorf("当前可用物料最多支持完成%s %s，请先补齐物料或调整本次数量", strconv.FormatFloat(preview.MaxOutputQty, 'f', -1, 64), preview.OutputUnit)
	}
	if cmd.ActualOutputQty > preview.RemainingQty+1e-8 && strings.TrimSpace(cmd.DifferenceReason) == "" {
		return out, fmt.Errorf("超出剩余数量时请填写原因")
	}
	if cmd.FinishGroup && cmd.ActualOutputQty < preview.RemainingQty-1e-8 && strings.TrimSpace(cmd.DifferenceReason) == "" {
		return out, fmt.Errorf("少产结束整组工序时请填写原因")
	}
	if cmd.Materials != nil && len(cmd.Materials) != countGroupMaterialRows(sources) {
		return out, fmt.Errorf("合并用料清单已变化，请刷新")
	}
	remainingByOrder := make([]float64, len(sources))
	for i, src := range sources {
		remainingByOrder[i] = src.p.PlannedQty
	}
	allocations, err := app.AllocateOperationGroupOutput(remainingByOrder, cmd.ActualOutputQty)
	if err != nil {
		return out, err
	}
	minutesByOrder, err := app.AllocateOperationGroupMinutes(allocations, cmd.ActualMinutes)
	if err != nil {
		return out, err
	}
	for i, src := range sources {
		quantity := allocations[i]
		finish := quantity >= src.p.PlannedQty-1e-8
		if cmd.FinishGroup {
			finish = true
		}
		if quantity <= 0 && !cmd.FinishGroup {
			continue
		}
		currentPreview, currentWO, e := r.operationCompletionContextTx(ctx, tx, src.cardID, true)
		if e != nil {
			return out, e
		}
		src.p, src.wo = currentPreview, currentWO
		childMaterials := []app.OperationMaterialUse(nil)
		if cmd.Materials != nil {
			childMaterials = make([]app.OperationMaterialUse, 0, len(src.p.Materials))
			allowed := map[int64]bool{}
			for _, material := range src.p.Materials {
				allowed[material.ReservationID] = true
			}
			for _, material := range cmd.Materials {
				if allowed[material.ReservationID] && (material.JobCardID == 0 || material.JobCardID == src.cardID) {
					childMaterials = append(childMaterials, material)
				}
			}
		}
		actualInput := 0.0
		for _, material := range childMaterials {
			if factor := app.OperationUnitFactor(material.Unit); factor > 0 {
				actualInput += material.Qty * factor
			}
		}
		minutes := minutesByOrder[i]
		finishFlag := finish
		child := app.JobCardActionCommand{ID: src.cardID, EmployeeID: cmd.EmployeeID, AssignedEmployeeName: cmd.AssignedEmployeeName, Operator: cmd.Operator, Action: "complete", RequestID: fmt.Sprintf("%s:%d", cmd.RequestID, src.cardID), ExpectedVersion: src.p.Version, InputUnit: src.p.InputUnit, OutputUnit: src.p.OutputUnit, ActualInputQty: actualInput, ActualOutputQty: quantity, ActualMinutes: minutes, DifferenceReason: cmd.DifferenceReason, FinishOperation: &finishFlag, Materials: childMaterials, MetricsJSON: "{}"}
		result, e := r.completeOperationLockedTx(ctx, tx, child, src.workID, src.runID)
		if e != nil {
			return out, fmt.Errorf("%s 入库失败：%w", src.no, e)
		}
		out.Items = append(out.Items, result)
		out.CompletedQty += quantity
	}
	if out.CompletedQty+1e-8 < cmd.ActualOutputQty {
		return out, fmt.Errorf("合并完成量未能全部分配，请刷新")
	}
	if cmd.FinishGroup {
		out.RemainingQty = 0
	} else {
		out.RemainingQty = math.Max(0, preview.RemainingQty-out.CompletedQty)
	}
	var entityID int64
	if len(out.Items) > 0 && out.Items[0].Completion != nil {
		entityID = out.Items[0].Completion.EventID
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_complete_group", cmd.Operator, cmd.RequestID, entityID, out); err != nil {
		return out, err
	}
	idsRaw, _ := json.Marshal(cmd.JobCardIDs)
	if len(out.Items) > 0 {
		if err = r.auditGroupedCompletionTx(ctx, tx, cmd, out, idsRaw); err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}

func countGroupMaterialRows(sources []completionGroupSource) int {
	count := 0
	for _, src := range sources {
		count += len(src.p.Materials)
	}
	return count
}

func (r Repository) auditGroupedCompletionTx(ctx context.Context, tx pgx.Tx, cmd app.OperationCompletionGroupCommand, result app.OperationCompletionGroupResult, ids []byte) error {
	cardID := result.Items[0].JobCard.ID
	return infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cardID, "complete_packaging_group", nil, nil, nil, infra.AuditMeta{"job_card_ids": json.RawMessage(ids), "request_id": cmd.RequestID, "actual_output_qty": result.CompletedQty, "unit": result.OutputUnit, "finish_group": cmd.FinishGroup, "difference_reason": cmd.DifferenceReason})
}
