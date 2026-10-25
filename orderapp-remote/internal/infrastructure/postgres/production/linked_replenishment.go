package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	app "orderapp/internal/application/production"
	"orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type linkedReplenishmentSource struct {
	ID, MaterialID, OwnerID, VersionID, ProcessRouteID, SpecG, BomSpecID, BomVariantID int64
	No, Name, Unit, Warehouse, Status, Snapshot, ProcessSnapshot                       string
	OutputQty                                                                          float64
}

func linkedReplenishmentIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func loadLinkedReplenishmentSourceTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (linkedReplenishmentSource, error) {
	var src linkedReplenishmentSource
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.output_material_id,COALESCE(m.owner_customer_id,0),
		       COALESCE(NULLIF(m.name,''),wo.output_name),COALESCE(NULLIF(m.unit,''),NULLIF(wo.output_unit,''),'kg'),
	       COALESCE(NULLIF(wo.target_warehouse,''),'wip'),wo.status,wo.bom_version_id,
	       pi.process_route_id,pi.spec_g,pi.bom_spec_id,pi.bom_variant_id,
	       COALESCE(pi.component_snapshot_json,'[]'::jsonb)::text,
	       COALESCE(pi.process_route_snapshot_json,'{}'::jsonb)::text,wo.output_qty::float8
		FROM %s.work_orders wo
		JOIN %s.production_plan_items pi ON pi.id=wo.production_plan_item_id
		JOIN %s.materials m ON m.id=wo.output_material_id AND m.deprecated_at IS NULL
		WHERE wo.id=$1 AND wo.output_type='material' AND wo.output_material_id>0
	`, schema, schema, schema), id).Scan(
		&src.ID, &src.No, &src.MaterialID, &src.OwnerID, &src.Name, &src.Unit, &src.Warehouse, &src.Status,
		&src.VersionID, &src.ProcessRouteID, &src.SpecG, &src.BomSpecID, &src.BomVariantID,
		&src.Snapshot, &src.ProcessSnapshot, &src.OutputQty,
	)
	if err == pgx.ErrNoRows {
		return src, fmt.Errorf("原上游工单不存在，或不是有效自制物料工单")
	}
	if err != nil {
		return src, err
	}
	if src.Status != "completed" && src.Status != "partially_completed" {
		return src, fmt.Errorf("原上游工单尚未结束，当前状态不能创建关联补产")
	}
	if strings.TrimSpace(src.Snapshot) == "" || strings.TrimSpace(src.Snapshot) == "[]" || !json.Valid([]byte(src.Snapshot)) || !json.Valid([]byte(src.ProcessSnapshot)) || strings.TrimSpace(src.ProcessSnapshot) == "{}" {
		return src, fmt.Errorf("原上游工单缺少完整冻结配方或工艺路线，需先核对工单资料")
	}
	return src, nil
}

func linkedReplenishmentAvailableTx(ctx context.Context, tx pgx.Tx, schema string, src linkedReplenishmentSource) (int64, int64, error) {
	var physicalG, physicalUnits, boundG, boundUnits, genericG, genericUnits, processingG, processingUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(l.qty_g),0)::bigint,COALESCE(SUM(l.qty_units),0)::bigint
		FROM %s.material_batch_locations l JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1 AND l.warehouse=$2 AND COALESCE(b.owner_customer_id,0)=$3
		  AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
	`, schema, schema), src.MaterialID, src.Warehouse, src.OwnerID).Scan(&physicalG, &physicalUnits)
	if err != nil {
		return 0, 0, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)),0)::bigint,
		       COALESCE(SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)),0)::bigint
		FROM %s.work_order_material_reservation_batches b
		JOIN %s.material_batches mb ON mb.id=b.material_batch_id
		JOIN %s.work_order_material_reservations r ON r.id=b.reservation_id AND r.status='reserved'
		JOIN %s.work_orders wo ON wo.id=r.work_order_id AND wo.status IN ('released','running','partially_completed','paused')
		WHERE b.material_id=$1 AND b.warehouse=$2 AND b.owner_customer_id=$3 AND b.status='reserved'
		  AND mb.status='active' AND COALESCE(mb.owner_customer_id,0)=$3
	`, schema, schema, schema, schema), src.MaterialID, src.Warehouse, src.OwnerID).Scan(&boundG, &boundUnits)
	if err != nil {
		return 0, 0, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
		       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
		FROM %s.work_order_material_reservations r
		JOIN %s.work_orders wo ON wo.id=r.work_order_id AND wo.status IN ('released','running','partially_completed','paused')
		WHERE r.material_id=$1 AND r.status='reserved' AND r.source_warehouse=$2 AND r.source_owner_customer_id=$3
		  AND NOT EXISTS (SELECT 1 FROM %s.work_order_material_reservation_batches b WHERE b.reservation_id=r.id AND b.status='reserved')
	`, schema, schema, schema), src.MaterialID, src.Warehouse, src.OwnerID).Scan(&genericG, &genericUnits)
	if err != nil {
		return 0, 0, err
	}
	// Customer-processing reservations also occupy physical stock. Subtracting all owners here is
	// intentionally conservative; the material owner check above prevents cross-owner replenishment.
	var hasProcessingReservations bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".customer_processing_material_reservations").Scan(&hasProcessingReservations); err != nil {
		return 0, 0, err
	}
	if hasProcessingReservations {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
		       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
		FROM %s.customer_processing_material_reservations r
		WHERE r.material_id=$1 AND r.status='reserved' AND r.source_warehouse_code=$2
		  AND NOT EXISTS (SELECT 1 FROM %s.work_order_material_reservations wr WHERE wr.work_order_id=r.work_order_id AND wr.material_id=r.material_id AND wr.status='reserved')
		`, schema, schema), src.MaterialID, src.Warehouse).Scan(&processingG, &processingUnits)
		if err != nil {
			return 0, 0, err
		}
	}
	availableG := maxInt64(0, physicalG-boundG-genericG-processingG)
	availableUnits := maxInt64(0, physicalUnits-boundUnits-genericUnits-processingUnits)
	return availableG, availableUnits, nil
}

func loadLinkedReplenishmentPreviewTx(ctx context.Context, tx pgx.Tx, schema string, cmd app.LinkedReplenishmentCommand, excludePlanID int64) (app.LinkedReplenishmentPreview, linkedReplenishmentSource, error) {
	src, err := loadLinkedReplenishmentSourceTx(ctx, tx, schema, cmd.SourceWorkOrderID)
	if err != nil {
		return app.LinkedReplenishmentPreview{}, src, err
	}
	ids := linkedReplenishmentIDs(cmd.ConsumerWorkOrderIDs)
	if len(ids) == 0 || len(ids) != len(cmd.ConsumerWorkOrderIDs) {
		return app.LinkedReplenishmentPreview{}, src, fmt.Errorf("请选择有效且不重复的包装工单")
	}
	availableG, availableUnits, err := linkedReplenishmentAvailableTx(ctx, tx, schema, src)
	if err != nil {
		return app.LinkedReplenishmentPreview{}, src, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,SUM(r.required_g)::bigint,SUM(r.required_units)::bigint,
	       SUM(r.reserved_g)::bigint,SUM(r.reserved_units)::bigint,SUM(r.returned_g)::bigint,SUM(r.returned_units)::bigint,
	       MIN(COALESCE(NULLIF(r.source_warehouse,''),'wip')),MAX(COALESCE(NULLIF(r.source_warehouse,''),'wip')),
	       MIN(COALESCE(r.source_owner_customer_id,0)),MAX(COALESCE(r.source_owner_customer_id,0)),
		       COALESCE(inflight.g,0)::bigint,COALESCE(inflight.units,0)::bigint,
		       COALESCE(drafts.g,0)::bigint,COALESCE(drafts.units,0)::bigint
		FROM %s.work_orders wo
		JOIN %s.work_order_material_reservations r ON r.work_order_id=wo.id
		LEFT JOIN LATERAL (
		  SELECT SUM(GREATEST(0,d.required_g-d.delivered_g))::bigint g,SUM(GREATEST(0,d.required_units-d.delivered_units))::bigint units
		  FROM %s.work_order_dependencies d JOIN %s.work_orders supplier ON supplier.id=d.depends_on_work_order_id
		  WHERE d.work_order_id=wo.id AND d.material_id=$2 AND supplier.id<>$3
		    AND supplier.status IN ('released','running','partially_completed','paused')
		) inflight ON true
		LEFT JOIN LATERAL (
		  SELECT SUM(link.required_g)::bigint g,SUM(link.required_units)::bigint units
		  FROM %s.production_replenishment_plan_links link JOIN %s.production_plans plan ON plan.id=link.production_plan_id
		  WHERE link.consumer_work_order_id=wo.id AND link.material_id=$2 AND plan.status='draft'
		    AND ($4::bigint=0 OR link.production_plan_id<>$4)
		) drafts ON true
		WHERE wo.id=ANY($1::bigint[]) AND wo.status IN ('released','running','partially_completed','paused')
		  AND r.material_id=$2 AND r.component_type='material' AND r.component_id=$2
		  AND COALESCE(r.component_spec_g,0)=$5 AND r.status='reserved'
		  AND EXISTS (SELECT 1 FROM %s.work_order_dependencies source_link
		              WHERE source_link.work_order_id=wo.id AND source_link.depends_on_work_order_id=$3
		                AND source_link.material_id=$2 AND source_link.component_type='material' AND source_link.component_id=$2
		                AND COALESCE(source_link.component_spec_g,0)=$5)
		GROUP BY wo.id,wo.work_order_no,inflight.g,inflight.units,drafts.g,drafts.units
		ORDER BY wo.work_order_no,wo.id
	`, schema, schema, schema, schema, schema, schema, schema), ids, src.MaterialID, src.ID, excludePlanID, src.SpecG)
	if err != nil {
		return app.LinkedReplenishmentPreview{}, src, err
	}
	consumers := []app.LinkedReplenishmentConsumer{}
	type versionRow struct {
		ID                                                                                                                         int64
		No, Warehouse, MaxWarehouse                                                                                                string
		Owner, MaxOwner                                                                                                            int64
		RequiredG, RequiredUnits, ReservedG, ReservedUnits, ReturnedG, ReturnedUnits, InflightG, InflightUnits, DraftG, DraftUnits int64
	}
	versionRows := []versionRow{}
	var totalG, totalUnits int64
	for rows.Next() {
		var c app.LinkedReplenishmentConsumer
		var wh, maxWarehouse string
		var owner, maxOwner int64
		var returnedG, returnedUnits int64
		var draftG, draftUnits int64
		if err := rows.Scan(&c.WorkOrderID, &c.WorkOrderNo, &c.RequiredG, &c.RequiredUnits, &c.ReservedG, &c.ReservedUnits, &returnedG, &returnedUnits, &wh, &maxWarehouse, &owner, &maxOwner, &c.InflightG, &c.InflightUnits, &draftG, &draftUnits); err != nil {
			rows.Close()
			return app.LinkedReplenishmentPreview{}, src, err
		}
		if firstNonEmpty(wh, "wip") != src.Warehouse || maxWarehouse != wh || owner != src.OwnerID || maxOwner != owner {
			rows.Close()
			return app.LinkedReplenishmentPreview{}, src, fmt.Errorf("所选工单的来源仓或货主与原上游不一致，不能关联补产")
		}
		gapG := maxInt64(0, c.RequiredG-c.ReservedG+returnedG-c.InflightG-draftG)
		gapUnits := maxInt64(0, c.RequiredUnits-c.ReservedUnits+returnedUnits-c.InflightUnits-draftUnits)
		c.ReplenishG, c.ReplenishUnits = gapG, gapUnits
		consumers = append(consumers, c)
		versionRows = append(versionRows, versionRow{ID: c.WorkOrderID, No: c.WorkOrderNo, Warehouse: wh, Owner: owner, RequiredG: c.RequiredG, RequiredUnits: c.RequiredUnits, ReservedG: c.ReservedG, ReservedUnits: c.ReservedUnits, ReturnedG: returnedG, ReturnedUnits: returnedUnits, InflightG: c.InflightG, InflightUnits: c.InflightUnits, DraftG: draftG, DraftUnits: draftUnits})
		totalG += gapG
		totalUnits += gapUnits
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return app.LinkedReplenishmentPreview{}, src, err
	}
	rows.Close()
	if len(consumers) != len(ids) {
		return app.LinkedReplenishmentPreview{}, src, fmt.Errorf("部分所选工单已完成、无此物料预留或资料不完整，请刷新后重试")
	}
	availableGUsed := minInt64(totalG, availableG)
	availableUnitsUsed := minInt64(totalUnits, availableUnits)
	remainingG := maxInt64(0, totalG-availableGUsed)
	remainingUnits := maxInt64(0, totalUnits-availableUnitsUsed)
	for i := range consumers {
		fromStockG := minInt64(consumers[i].ReplenishG, availableGUsed)
		consumers[i].ReplenishG -= fromStockG
		availableGUsed -= fromStockG
		fromStockUnits := minInt64(consumers[i].ReplenishUnits, availableUnitsUsed)
		consumers[i].ReplenishUnits -= fromStockUnits
		availableUnitsUsed -= fromStockUnits
	}
	unit := strings.TrimSpace(src.Unit)
	qty := manufacturingQtyFromCanonical(remainingG, remainingUnits, unit)
	payload, _ := json.Marshal(struct {
		Source                     linkedReplenishmentSource
		Consumers                  []versionRow
		AvailableG, AvailableUnits int64
	}{src, versionRows, availableG, availableUnits})
	version := fmt.Sprintf("%x", sha256.Sum256(payload))
	return app.LinkedReplenishmentPreview{Version: version, SourceWorkOrderID: src.ID, SourceWorkOrderNo: src.No, MaterialID: src.MaterialID, MaterialName: src.Name, Unit: unit, Warehouse: src.Warehouse, OwnerCustomerID: src.OwnerID, AvailableG: availableG, AvailableUnits: availableUnits, ReplenishG: remainingG, ReplenishUnits: remainingUnits, OutputQty: qty, Consumers: consumers}, src, nil
}

func (r Repository) PreviewLinkedReplenishment(ctx context.Context, cmd app.LinkedReplenishmentCommand) (app.LinkedReplenishmentPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return app.LinkedReplenishmentPreview{}, err
	}
	defer tx.Rollback(ctx)
	preview, _, err := loadLinkedReplenishmentPreviewTx(ctx, tx, r.schema, cmd, 0)
	return preview, err
}

func (r Repository) CreateLinkedReplenishment(ctx context.Context, cmd app.LinkedReplenishmentCommand) (app.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	defer tx.Rollback(ctx)
	planID, _, err := requestProductionKeyTx(ctx, tx, r.schema, "create_linked_replenishment", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if planID > 0 {
		return loadProductionPlanDetailTx(ctx, tx, r.schema, planID)
	}
	ids := linkedReplenishmentIDs(cmd.ConsumerWorkOrderIDs)
	lockIDs := append(append([]int64{}, ids...), cmd.SourceWorkOrderID)
	lockIDs = linkedReplenishmentIDs(lockIDs)
	for _, id := range lockIDs {
		var locked int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&locked); err != nil {
			return app.ProductionPlanDetail{}, fmt.Errorf("工单已变化，请刷新关联补产预览")
		}
	}
	preview, src, err := loadLinkedReplenishmentPreviewTx(ctx, tx, r.schema, cmd, 0)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if preview.Version != cmd.ExpectedVersion {
		return app.ProductionPlanDetail{}, fmt.Errorf("关联补产预览已过期，库存或工单已变化，请刷新后重试")
	}
	if preview.OutputQty <= 0 || (preview.ReplenishG <= 0 && preview.ReplenishUnits <= 0) {
		return app.ProductionPlanDetail{}, fmt.Errorf("可用库存和已有供给已覆盖所选缺口，无需补产")
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_plans(plan_no,source_type,status,customer_id,created_by,picking_version) VALUES('PP-TMP-'||nextval('%s.production_plans_id_seq')::text,'stock','draft',$1,$2,1) RETURNING id`, r.schema, r.schema), src.OwnerID, cmd.Operator).Scan(&planID); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET plan_no=$2 WHERE id=$1`, r.schema), planID, productionPlanNo(planID)); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	basis := manufacturingOutputBOMPlanBasis{VersionID: src.VersionID, OutputType: "material", OutputID: src.MaterialID, OutputName: src.Name, InventoryUnit: src.Unit, OutputSpecG: src.SpecG, ProcessRouteID: src.ProcessRouteID, Snapshot: src.Snapshot, FrozenProcessRouteSnapshot: src.ProcessSnapshot, BomSpecID: src.BomSpecID, BomVariantID: src.BomVariantID}
	root, err := insertManufacturingOutputProductionPlanItemTx(ctx, tx, r.schema, planID, basis, preview.OutputQty, nil)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET target_warehouse=$2,customer_id=$3 WHERE id=$1`, r.schema), root.ID, preview.Warehouse, preview.OwnerCustomerID); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	root.TargetWarehouse, root.CustomerID = preview.Warehouse, preview.OwnerCustomerID
	expanded, err := createExpandedProductionItemsTx(ctx, tx, r.schema, planID, []app.ProductionPlanItem{root})
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, planID, expanded.Items); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	for _, consumer := range preview.Consumers {
		if consumer.ReplenishG <= 0 && consumer.ReplenishUnits <= 0 {
			continue
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_replenishment_plan_links(production_plan_id,production_plan_item_id,source_work_order_id,consumer_work_order_id,material_id,required_g,required_units,warehouse,owner_customer_id,expected_version,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.schema), planID, root.ID, src.ID, consumer.WorkOrderID, src.MaterialID, consumer.ReplenishG, consumer.ReplenishUnits, preview.Warehouse, preview.OwnerCustomerID, preview.Version, cmd.Operator); err != nil {
			return app.ProductionPlanDetail{}, err
		}
	}
	if err = postgres.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &planID, "create_linked_replenishment", postgres.StrPtr("source_work_order_id"), nil, postgres.StrPtr(fmt.Sprint(src.ID)), postgres.AuditMeta{"consumer_work_order_ids": ids, "material_id": src.MaterialID, "replenish_g": preview.ReplenishG, "replenish_units": preview.ReplenishUnits, "expected_version": preview.Version, "request_id": cmd.RequestID}); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	result, err := loadProductionPlanDetailTx(ctx, tx, r.schema, planID)
	if err != nil {
		return result, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "create_linked_replenishment", cmd.Operator, cmd.RequestID, planID, nil); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func validateLinkedReplenishmentAtSubmitTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	var sourceID int64
	var ids []int64
	var version string
	var sourceCount int
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT source_work_order_id),COALESCE(MIN(source_work_order_id),0),COALESCE(array_agg(DISTINCT consumer_work_order_id ORDER BY consumer_work_order_id),'{}'::bigint[]),COALESCE(MIN(expected_version),'') FROM %s.production_replenishment_plan_links WHERE production_plan_id=$1`, schema), planID).Scan(&sourceCount, &sourceID, &ids, &version)
	if err != nil {
		return err
	}
	if sourceCount == 0 {
		return nil
	}
	if sourceCount != 1 || len(ids) == 0 {
		return fmt.Errorf("关联补产计划的来源信息不完整，请重新预览")
	}
	lockIDs := linkedReplenishmentIDs(append(append([]int64{}, ids...), sourceID))
	for _, id := range lockIDs {
		var locked int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=$1 FOR UPDATE`, schema), id).Scan(&locked); err != nil {
			return fmt.Errorf("关联补产工单已变化，请刷新后重试")
		}
	}
	cmd := app.LinkedReplenishmentCommand{SourceWorkOrderID: sourceID, ConsumerWorkOrderIDs: ids}
	preview, _, err := loadLinkedReplenishmentPreviewTx(ctx, tx, schema, cmd, planID)
	if err != nil {
		return err
	}
	if preview.Version != version {
		return fmt.Errorf("关联补产工单或库存已变化，请刷新计划并重新预览")
	}
	var linkedG, linkedUnits, linkedItemCount, linkedItemID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(required_g),0)::bigint,COALESCE(SUM(required_units),0)::bigint FROM %s.production_replenishment_plan_links WHERE production_plan_id=$1`, schema), planID).Scan(&linkedG, &linkedUnits); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT production_plan_item_id),MIN(production_plan_item_id) FROM %s.production_replenishment_plan_links WHERE production_plan_id=$1`, schema), planID).Scan(&linkedItemCount, &linkedItemID); err != nil {
		return err
	}
	if linkedItemCount != 1 {
		return fmt.Errorf("关联补产计划必须只有一个补产产出，请重新预览")
	}
	var outputQty float64
	var outputUnit string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT output_qty::float8,output_unit FROM %s.production_plan_items WHERE id=$1 AND production_plan_id=$2 AND output_type='material'`, schema), linkedItemID, planID).Scan(&outputQty, &outputUnit); err != nil {
		return fmt.Errorf("关联补产产出已变化，请重新预览")
	}
	plannedG, plannedUnits := canonicalOutputBasis(outputQty, outputUnit)
	if linkedG != plannedG || linkedUnits != plannedUnits || linkedG != preview.ReplenishG || linkedUnits != preview.ReplenishUnits {
		return fmt.Errorf("关联补产数量与当前包装工单缺口不一致，请刷新预览")
	}
	return nil
}

func createLinkedReplenishmentWorkOrderDependenciesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, workOrderByPlanItem map[int64]int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT production_plan_item_id,source_work_order_id,consumer_work_order_id,material_id,required_g,required_units FROM %s.production_replenishment_plan_links WHERE production_plan_id=$1 ORDER BY consumer_work_order_id`, schema), planID)
	if err != nil {
		return err
	}
	type link struct{ itemID, sourceID, consumerID, materialID, g, units int64 }
	links := []link{}
	for rows.Next() {
		var v link
		if err = rows.Scan(&v.itemID, &v.sourceID, &v.consumerID, &v.materialID, &v.g, &v.units); err != nil {
			rows.Close()
			return err
		}
		links = append(links, v)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, v := range links {
		supplierID := workOrderByPlanItem[v.itemID]
		if supplierID <= 0 {
			return fmt.Errorf("关联补产工单映射缺失")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_dependencies(work_order_id,depends_on_work_order_id,material_id,component_type,component_id,required_g,required_units) VALUES($1,$2,$3,'material',$3,$4,$5) ON CONFLICT(work_order_id,depends_on_work_order_id,component_type,component_id,component_bom_spec_id,component_spec_g) DO UPDATE SET required_g=excluded.required_g,required_units=excluded.required_units`, schema), v.consumerID, supplierID, v.materialID, v.g, v.units); err != nil {
			return err
		}
	}
	return nil
}
