package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"sort"
)

func autoPickingPlanTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (bool, error) {
	var version int
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT picking_version FROM %s.production_plans WHERE id=$1`, schema), id).Scan(&version)
	return version > 0, err
}
func autoPickingWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (bool, error) {
	exists, err := schemaColumnExistsTx(ctx, tx, schema, "production_plans", "picking_version")
	if err != nil || !exists {
		return false, err
	}
	var enabled bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_plans p ON p.id=w.production_plan_id WHERE w.id=$1 AND p.picking_version>0)`, schema), id).Scan(&enabled)
	return enabled, err
}
func pickingKey(s app.ProductionPlanComponentSource, warehouse string, owner int64) string {
	return fmt.Sprintf("%s:%d:%d:%d:%s:%d", s.ComponentType, s.ComponentID, s.ComponentBOMSpecID, s.ComponentSpecG, warehouse, owner)
}

// Draft allocation is a projection, never a reservation. The shared budget prevents
// two rows of the same plan from promising the same stock.
func calculatePickingSources(sources []app.ProductionPlanComponentSource) {
	used := map[string][2]int64{}
	// Honour valid manual quantities across the entire plan before auto-filling.
	for i := range sources {
		s := &sources[i]
		s.Allocations = []app.ProductionPlanSourceAllocation{}
		s.ShortageG, s.ShortageUnits = s.RequiredG, s.RequiredUnits
		s.WIPCoveredG, s.WIPCoveredUnits, s.TransferG, s.TransferUnits = 0, 0, 0, 0
		sort.SliceStable(s.Options, func(i, j int) bool {
			a, b := s.Options[i], s.Options[j]
			if (a.Warehouse == "wip") != (b.Warehouse == "wip") {
				return a.Warehouse == "wip"
			}
			if a.SortOrder != b.SortOrder {
				return a.SortOrder < b.SortOrder
			}
			if a.Warehouse != b.Warehouse {
				return a.Warehouse < b.Warehouse
			}
			return a.OwnerCustomerID < b.OwnerCustomerID
		})
		if s.AllocationMode != "manual" {
			continue
		}
		for _, m := range s.ManualAllocations {
			found := false
			for _, o := range s.Options {
				if o.Warehouse != m.Warehouse || o.OwnerCustomerID != m.OwnerCustomerID {
					continue
				}
				found = true
				g, n := addPickingAllocation(s, o, minInt64(m.QtyG, s.ShortageG), minInt64(m.QtyUnits, s.ShortageUnits), used)
				if g != m.QtyG || n != m.QtyUnits {
					s.AdjustmentMessage = "部分手工分配的可用量已变化，已保留有效数量并重新补齐"
				}
				break
			}
			if !found {
				s.AdjustmentMessage = "部分手工来源已停用或货主不符，已重新生成建议"
			}
		}
	}
	for i := range sources {
		s := &sources[i]
		for _, o := range s.Options {
			addPickingAllocation(s, o, s.ShortageG, s.ShortageUnits, used)
		}
		s.Selected = s.ShortageG == 0 && s.ShortageUnits == 0
		s.SourceWarehouse = "wip"
		s.DemandG, s.DemandUnits = s.RequiredG+s.UpstreamG, s.RequiredUnits+s.UpstreamUnits
		s.PreparationStatus = "ready"
		if s.UpstreamG > 0 || s.UpstreamUnits > 0 {
			s.PreparationStatus = "awaiting_production"
		}
		if s.TransferG > 0 || s.TransferUnits > 0 {
			s.PreparationStatus = "awaiting_transfer"
		}
		if !s.Selected {
			s.PreparationStatus = "shortage"
		}
	}
}
func addPickingAllocation(s *app.ProductionPlanComponentSource, o app.ProductionPlanComponentSourceOption, g, n int64, used map[string][2]int64) (int64, int64) {
	key := pickingKey(*s, o.Warehouse, o.OwnerCustomerID)
	consumed := used[key]
	g = minInt64(g, nonnegativeQuantity(o.AvailableG-consumed[0]))
	n = minInt64(n, nonnegativeQuantity(o.AvailableUnits-consumed[1]))
	if g <= 0 && n <= 0 {
		return 0, 0
	}
	a := app.ProductionPlanSourceAllocation{Warehouse: o.Warehouse, WarehouseName: o.WarehouseName, OwnerCustomerID: o.OwnerCustomerID, OwnerName: o.OwnerName, QtyG: g, QtyUnits: n}
	merged := false
	for i := range s.Allocations {
		if s.Allocations[i].Warehouse == o.Warehouse && s.Allocations[i].OwnerCustomerID == o.OwnerCustomerID {
			s.Allocations[i].QtyG += g
			s.Allocations[i].QtyUnits += n
			merged = true
			break
		}
	}
	if !merged {
		s.Allocations = append(s.Allocations, a)
	}
	consumed[0] += g
	consumed[1] += n
	used[key] = consumed
	s.ShortageG -= g
	s.ShortageUnits -= n
	if o.Warehouse == "wip" {
		s.WIPCoveredG += g
		s.WIPCoveredUnits += n
	} else {
		s.TransferG += g
		s.TransferUnits += n
	}
	return g, n
}

func preparePickingSourcesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, sources []app.ProductionPlanComponentSource, status string) error {
	for i := range sources {
		s := &sources[i]
		s.PickingVersion = 1
		if s.AllocationMode == "manual" {
			s.ManualAllocations = append([]app.ProductionPlanSourceAllocation{}, s.Allocations...)
		}
		if status == "draft" && s.ComponentType == "material" {
			var active bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(owner_customer_id,0),unit,(deprecated_at IS NULL) FROM %s.materials WHERE id=$1`, schema), s.ComponentID).Scan(&s.SourceOwnerCustomerID, &s.Unit, &active); err != nil {
				return err
			}
			if !active {
				s.Options = nil
				s.AdjustmentMessage = "物料已停用，请检查物料档案"
			}
		} else if status == "draft" {
			s.SourceOwnerCustomerID = 0
		}
		allowed := []app.ProductionPlanComponentSourceOption{}
		for _, o := range s.Options {
			if o.OwnerCustomerID == s.SourceOwnerCustomerID {
				allowed = append(allowed, o)
			}
		}
		s.Options = allowed
		if planID > 0 {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(required_g),0)::bigint,COALESCE(SUM(required_units),0)::bigint FROM %s.production_plan_item_dependencies WHERE production_plan_item_id=$1 AND component_type=$2 AND component_id=$3 AND component_bom_spec_id=$4 AND component_spec_g=$5`, schema), s.ProductionPlanItemID, s.ComponentType, s.ComponentID, s.ComponentBOMSpecID, s.ComponentSpecG).Scan(&s.UpstreamG, &s.UpstreamUnits); err != nil {
				return err
			}
		}
	}
	if status == "draft" {
		calculatePickingSources(sources)
		return fillPickingBatchSuggestionsTx(ctx, tx, schema, sources)
	}
	return loadFrozenPickingProgressTx(ctx, tx, schema, sources)
}

func savePickingAdjustmentTx(ctx context.Context, tx pgx.Tx, schema string, stored, requested app.ProductionPlanComponentSource, customerID int64, operator string) error {
	mode := requested.AllocationMode
	if mode == "" {
		mode = "manual"
	}
	if mode != "auto" && mode != "manual" {
		return fmt.Errorf("请选择自动建议或手工调整")
	}
	allocations := requested.ManualAllocations
	if allocations == nil {
		allocations = requested.Allocations
	}
	if requested.AllocationMode == "" && mode == "manual" && len(allocations) == 0 && requested.SourceWarehouse != "" {
		allocations = []app.ProductionPlanSourceAllocation{{Warehouse: requested.SourceWarehouse, OwnerCustomerID: requested.SourceOwnerCustomerID, QtyG: stored.RequiredG, QtyUnits: stored.RequiredUnits}}
	}
	if mode == "auto" {
		allocations = []app.ProductionPlanSourceAllocation{}
	}
	var totalG, totalN int64
	seen := map[string]bool{}
	for i := range allocations {
		a := &allocations[i]
		if a.QtyG < 0 || a.QtyUnits < 0 {
			return fmt.Errorf("领料数量不能为负数")
		}
		whOwner, err := warehouseCustomerID(ctx, tx, schema, a.Warehouse)
		if err != nil {
			return err
		}
		owner, err := validateComponentSourceOwner(customerID, whOwner, a.OwnerCustomerID)
		if err != nil {
			return err
		}
		if err = validateMaterialComponentSourceOwnerTx(ctx, tx, schema, stored.ComponentType, stored.ComponentID, owner); err != nil {
			return err
		}
		if stored.ComponentType == "product" && owner != 0 {
			return fmt.Errorf("商品组件来源必须与冻结组件货主一致")
		}
		if stored.ComponentType == "product" && stored.ComponentSpecG > 0 && stored.RequiredUnits > 0 {
			if a.QtyUnits == 0 && a.QtyG > 0 {
				a.QtyUnits = a.QtyG / stored.ComponentSpecG
			}
			if a.QtyG == 0 && a.QtyUnits > 0 {
				a.QtyG = a.QtyUnits * stored.ComponentSpecG
			}
			if a.QtyG != a.QtyUnits*stored.ComponentSpecG {
				return fmt.Errorf("商品组件数量须符合冻结规格")
			}
		}
		a.OwnerCustomerID = owner
		a.Batches = nil
		key := fmt.Sprintf("%s:%d", a.Warehouse, owner)
		if seen[key] {
			return fmt.Errorf("同一来源仓和货主不能重复设置")
		}
		seen[key] = true
		if a.QtyG > stored.RequiredG-totalG || a.QtyUnits > stored.RequiredUnits-totalN {
			return fmt.Errorf("手工分配数量不能超过本项待落实需求")
		}
		totalG += a.QtyG
		totalN += a.QtyUnits
	}
	if totalG > stored.RequiredG || totalN > stored.RequiredUnits {
		return fmt.Errorf("手工分配数量不能超过本项待落实需求")
	}
	raw, _ := json.Marshal(allocations)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_component_sources SET allocation_mode=$2,allocations_json=$3,selected_by=$4,selected_at=now(),updated_at=now() WHERE id=$1`, schema), stored.ID, mode, raw, operator); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "production_plan_component_source", &stored.ID, "adjust_picking", nil, nil, nil, postgresinfra.AuditMeta{"production_plan_id": stored.ProductionPlanID, "mode": mode, "allocations": allocations})
}

func validateAutoPickingAtSubmitTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	// Lock physical batches in a global order before reading availability. This also
	// serializes different plans competing for different locations of one batch.
	for _, q := range []string{
		`SELECT b.id FROM %[1]s.material_batches b WHERE b.material_id IN(SELECT component_id FROM %[1]s.production_plan_component_sources WHERE production_plan_id=$1 AND component_type='material') ORDER BY b.id FOR UPDATE`,
		`SELECT b.id FROM %[1]s.stock_batches b WHERE b.item_type='finished_product' AND b.item_id IN(SELECT component_id FROM %[1]s.production_plan_component_sources WHERE production_plan_id=$1 AND component_type='product') ORDER BY b.id FOR UPDATE`,
	} {
		rows, err := tx.Query(ctx, fmt.Sprintf(q, schema), planID)
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
	sources, err := loadProductionPlanComponentSourcesTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	for _, s := range sources {
		if s.ShortageG > 0 || s.ShortageUnits > 0 {
			return fmt.Errorf("%s 供给不足或已变化，缺少 %dg / %d件；草稿已保留，请刷新供应", s.ComponentName, s.ShortageG, s.ShortageUnits)
		}
		raw, _ := json.Marshal(s.Allocations)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_component_sources SET allocations_json=$2,source_warehouse='wip',source_owner_customer_id=$3,available_g_snapshot=$4,available_units_snapshot=$5,unit=$6 WHERE id=$1`, schema), s.ID, raw, s.SourceOwnerCustomerID, s.RequiredG, s.RequiredUnits, s.Unit); err != nil {
			return err
		}
	}
	return nil
}

func fillPickingBatchSuggestionsTx(ctx context.Context, tx pgx.Tx, schema string, sources []app.ProductionPlanComponentSource) error {
	used := map[string][2]int64{}
	for i := range sources {
		s := &sources[i]
		for j := range s.Allocations {
			a := &s.Allocations[j]
			var rows pgx.Rows
			var err error
			if s.ComponentType == "product" {
				rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,GREATEST(0,b.remaining_g-COALESCE(r.g,0))::bigint,GREATEST(0,b.remaining_units-COALESCE(r.n,0))::bigint
 FROM %[1]s.stock_batches b
 LEFT JOIN LATERAL(SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code) ORDER BY l.id DESC LIMIT 1) location ON true
 LEFT JOIN LATERAL(SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE stock_batch_id=b.id AND status='reserved') r ON true
 WHERE b.item_type='finished_product' AND b.item_id=$1 AND COALESCE(NULLIF(location.warehouse,''),'finished_goods')=$2 AND COALESCE(b.owner_customer_id,0)=$3 AND b.bom_spec_id=$4 AND b.spec_g=$5 AND COALESCE(b.quality_status,'unchecked') NOT IN('hold','reject') ORDER BY b.created_at,b.id`, schema), s.ComponentID, a.Warehouse, a.OwnerCustomerID, s.ComponentBOMSpecID, s.ComponentSpecG)
			} else {
				rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,GREATEST(0,l.qty_g-COALESCE(r.g,0))::bigint,GREATEST(0,l.qty_units-COALESCE(r.n,0))::bigint
   FROM %[1]s.material_batch_locations l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id
   LEFT JOIN LATERAL(SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse=l.warehouse AND status='reserved') r ON true
   WHERE l.material_id=$1 AND l.warehouse=$2 AND b.owner_customer_id=$3 AND b.status='active' AND b.quality_status NOT IN('hold','reject') ORDER BY b.received_at,b.id`, schema), s.ComponentID, a.Warehouse, a.OwnerCustomerID)
			}
			if err != nil {
				return err
			}
			g, n := a.QtyG, a.QtyUnits
			for rows.Next() {
				var b app.ProductionPlanSourceBatch
				var bg, bn int64
				if err = rows.Scan(&b.BatchID, &b.BatchCode, &bg, &bn); err != nil {
					rows.Close()
					return err
				}
				key := fmt.Sprintf("%s:%d:%s", s.ComponentType, b.BatchID, a.Warehouse)
				u := used[key]
				b.QtyG = minInt64(g, nonnegativeQuantity(bg-u[0]))
				b.QtyUnits = minInt64(n, nonnegativeQuantity(bn-u[1]))
				if b.QtyG == 0 && b.QtyUnits == 0 {
					continue
				}
				a.Batches = append(a.Batches, b)
				g -= b.QtyG
				n -= b.QtyUnits
				used[key] = [2]int64{u[0] + b.QtyG, u[1] + b.QtyUnits}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func loadFrozenPickingProgressTx(ctx context.Context, tx pgx.Tx, schema string, sources []app.ProductionPlanComponentSource) error {
	for i := range sources {
		s := &sources[i]
		s.WIPCoveredG = 0
		s.WIPCoveredUnits = 0
		s.TransferG = 0
		s.TransferUnits = 0
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.warehouse,COALESCE(w.name,b.warehouse),b.owner_customer_id,COALESCE(c.name,'工厂'),b.material_batch_id,b.stock_batch_id,b.batch_code,GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g),GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)
  FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_orders wo ON wo.id=b.work_order_id LEFT JOIN %[1]s.warehouses w ON w.code=b.warehouse LEFT JOIN %[1]s.customers c ON c.id=b.owner_customer_id
  WHERE wo.production_plan_item_id=$1 AND b.component_type=$2 AND b.component_id=$3 AND b.component_bom_spec_id=$4 AND b.component_spec_g=$5 AND b.status='reserved' ORDER BY b.warehouse,b.id`, schema), s.ProductionPlanItemID, s.ComponentType, s.ComponentID, s.ComponentBOMSpecID, s.ComponentSpecG)
		if err != nil {
			return err
		}
		s.Allocations = []app.ProductionPlanSourceAllocation{}
		for rows.Next() {
			var a app.ProductionPlanSourceAllocation
			var b app.ProductionPlanSourceBatch
			var stockID int64
			if err = rows.Scan(&a.Warehouse, &a.WarehouseName, &a.OwnerCustomerID, &a.OwnerName, &b.BatchID, &stockID, &b.BatchCode, &a.QtyG, &a.QtyUnits); err != nil {
				rows.Close()
				return err
			}
			if b.BatchID == 0 {
				b.BatchID = stockID
			}
			b.QtyG = a.QtyG
			b.QtyUnits = a.QtyUnits
			a.Batches = []app.ProductionPlanSourceBatch{b}
			s.Allocations = append(s.Allocations, a)
			if a.Warehouse == "wip" {
				s.WIPCoveredG += a.QtyG
				s.WIPCoveredUnits += a.QtyUnits
			} else {
				s.TransferG += a.QtyG
				s.TransferUnits += a.QtyUnits
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		s.DemandG = s.RequiredG + s.UpstreamG
		s.DemandUnits = s.RequiredUnits + s.UpstreamUnits
		s.Selected = true
		s.PreparationStatus = "ready"
		if s.TransferG > 0 || s.TransferUnits > 0 {
			s.PreparationStatus = "awaiting_transfer"
		} else if s.WIPCoveredG < s.DemandG || s.WIPCoveredUnits < s.DemandUnits {
			s.PreparationStatus = "awaiting_production"
		}
	}
	return nil
}

func pickingAdjustmentIdentity(item app.ProductionPlanItem, s app.ProductionPlanComponentSource) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%s:%s:%d:%d:%d", item.OutputType, item.OutputProductID, item.OutputMaterialID, item.BomVersionID, item.CustomerID, item.TargetWarehouse, s.ComponentType, s.ComponentID, s.ComponentBOMSpecID, s.ComponentSpecG)
}
func preservedPickingAdjustmentsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []app.ProductionPlanItem) (map[string][]app.ProductionPlanSourceAllocation, error) {
	sources, err := loadProductionPlanComponentSourcesTx(ctx, tx, schema, planID)
	if err != nil {
		return nil, err
	}
	byID := map[int64]app.ProductionPlanItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	out := map[string][]app.ProductionPlanSourceAllocation{}
	for _, s := range sources {
		allocations := s.ManualAllocations
		if s.PickingVersion == 0 && s.Selected {
			allocations = []app.ProductionPlanSourceAllocation{{Warehouse: s.SourceWarehouse, OwnerCustomerID: s.SourceOwnerCustomerID, QtyG: s.RequiredG, QtyUnits: s.RequiredUnits}}
		}
		if len(allocations) > 0 {
			out[pickingAdjustmentIdentity(byID[s.ProductionPlanItemID], s)] = allocations
		}
	}
	return out, nil
}
func restorePickingAdjustmentsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []app.ProductionPlanItem, adjustments map[string][]app.ProductionPlanSourceAllocation) error {
	sources, err := loadProductionPlanComponentSourcesTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	byID := map[int64]app.ProductionPlanItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	for _, s := range sources {
		if a, ok := adjustments[pickingAdjustmentIdentity(byID[s.ProductionPlanItemID], s)]; ok {
			raw, _ := json.Marshal(a)
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_component_sources SET allocation_mode='manual',allocations_json=$2 WHERE id=$1`, schema), s.ID, raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// The execution hub and the start/consume guard use this same owned, qualified,
// frozen WIP quantity. Stock reserved elsewhere cannot make a work order ready.
func automaticWorkOrderWIPStatusTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (app.ProductionWIPStatus, error) {
	out := app.ProductionWIPStatus{DataComplete: true, Status: "ok", Materials: []app.WIPReservationRow{}}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,r.material_id,r.material_name,r.unit,r.required_g,r.required_units,r.consumed_g,r.consumed_units,r.reserved_g,r.reserved_units,COALESCE(q.g,0)::bigint,COALESCE(q.n,0)::bigint
 FROM %[1]s.work_order_material_reservations r
 LEFT JOIN LATERAL(
  SELECT SUM(LEAST(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g),GREATEST(0,CASE WHEN b.component_type='product' THEN sb.remaining_g ELSE l.qty_g END))) g,
  SUM(LEAST(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units),GREATEST(0,CASE WHEN b.component_type='product' THEN sb.remaining_units ELSE l.qty_units END))) n
  FROM %[1]s.work_order_material_reservation_batches b
  LEFT JOIN %[1]s.material_batches mb ON mb.id=b.material_batch_id AND mb.status='active' AND mb.owner_customer_id=b.owner_customer_id AND mb.quality_status NOT IN('hold','reject')
  LEFT JOIN %[1]s.material_batch_locations l ON l.material_batch_id=mb.id AND l.warehouse='wip'
  LEFT JOIN %[1]s.stock_batches sb ON sb.id=b.stock_batch_id AND sb.owner_customer_id=b.owner_customer_id AND sb.quality_status NOT IN('hold','reject')
  WHERE b.reservation_id=r.id AND b.status='reserved' AND b.warehouse='wip' AND b.owner_customer_id=r.source_owner_customer_id
 ) q ON true
 WHERE r.work_order_id=$1 AND r.status='reserved' ORDER BY r.id`, schema), id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var row app.WIPReservationRow
		if err = rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.Unit, &row.RequiredG, &row.RequiredUnits, &row.ConsumedG, &row.ConsumedUnits, &row.ReservedG, &row.ReservedUnits, &row.AvailableG, &row.AvailableUnits); err != nil {
			return out, err
		}
		row.WorkOrderID = id
		row.WIPG = row.AvailableG
		row.WIPUnits = row.AvailableUnits
		row.InventoryUnit = row.Unit
		row.ShortageG = nonnegativeQuantity(row.RequiredG - row.ConsumedG - row.AvailableG)
		row.ShortageUnits = nonnegativeQuantity(row.RequiredUnits - row.ConsumedUnits - row.AvailableUnits)
		row.Status = "reserved"
		row.QuantityBasis = "count"
		row.RequiredQty = float64(row.RequiredUnits)
		row.AvailableQty = float64(row.AvailableUnits)
		row.ShortageQty = float64(row.ShortageUnits)
		if row.RequiredG > 0 {
			row.QuantityBasis = "weight"
			row.RequiredQty = productionInventoryQuantity(row.RequiredG, row.Unit)
			row.AvailableQty = productionInventoryQuantity(row.AvailableG, row.Unit)
			row.ShortageQty = productionInventoryQuantity(row.ShortageG, row.Unit)
		}
		row.RemainingReservedG = nonnegativeQuantity(row.ReservedG - row.ConsumedG)
		out.Materials = append(out.Materials, row)
		out.RequiredG += row.RequiredG
		out.RequiredUnits += row.RequiredUnits
		out.AvailableG += row.AvailableG
		out.AvailableUnits += row.AvailableUnits
		out.ShortageG += row.ShortageG
		out.ShortageUnits += row.ShortageUnits
		out.ReservedG += row.ReservedG
		out.ConsumedG += row.ConsumedG
		out.RemainingG += row.RemainingReservedG
		if row.ShortageG > 0 || row.ShortageUnits > 0 {
			out.Status = "blocked"
			if out.BlockingReason == "" {
				out.BlockingReason = fmt.Sprintf("%s 尚未领齐到 WIP，待领或待入库 %g %s", row.MaterialName, row.ShortageQty, row.Unit)
			}
		}
	}
	return out, rows.Err()
}
