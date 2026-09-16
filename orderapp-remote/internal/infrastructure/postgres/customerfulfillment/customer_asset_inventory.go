package customerfulfillment

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
)

type customerAssetState struct {
	row        app.CustomerAssetInventory
	warehouses map[string]int
}

func (r *Repository) ListCustomerAssetInventory(ctx context.Context, query app.CustomerAssetInventoryQuery) ([]app.CustomerAssetInventory, error) {
	states := make([]*customerAssetState, 0)
	if query.InventoryType == "" || query.InventoryType == "finished_product" {
		finished, err := r.customerFinishedAssetInventory(ctx, query.CustomerID)
		if err != nil {
			return nil, err
		}
		states = append(states, finished...)
	}
	if query.InventoryType == "" || query.InventoryType == "green_bean" || query.InventoryType == "packaging" || query.InventoryType == "semi_finished" {
		materials, err := r.customerMaterialAssetInventory(ctx, query.CustomerID, query.InventoryType)
		if err != nil {
			return nil, err
		}
		states = append(states, materials...)
		legacy, err := r.customerLegacyCustodyAssetInventory(ctx, query.CustomerID, query.InventoryType, states)
		if err != nil {
			return nil, err
		}
		states = append(states, legacy...)
	}
	needle := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(query.Q)), ""))
	out := make([]app.CustomerAssetInventory, 0, len(states))
	for _, state := range states {
		if needle != "" {
			search := strings.ToLower(strings.Join(strings.Fields(state.row.ItemName+state.row.ItemCode+state.row.Spec), ""))
			if !strings.Contains(search, needle) {
				continue
			}
		}
		sort.SliceStable(state.row.Warehouses, func(i, j int) bool {
			return state.row.Warehouses[i].WarehouseName < state.row.Warehouses[j].WarehouseName
		})
		sort.SliceStable(state.row.Batches, func(i, j int) bool {
			if state.row.Batches[i].InboundAt != state.row.Batches[j].InboundAt {
				return state.row.Batches[i].InboundAt > state.row.Batches[j].InboundAt
			}
			return state.row.Batches[i].BatchID > state.row.Batches[j].BatchID
		})
		out = append(out, state.row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InventoryType != out[j].InventoryType {
			return customerAssetTypeOrder(out[i].InventoryType) < customerAssetTypeOrder(out[j].InventoryType)
		}
		return out[i].ItemName < out[j].ItemName
	})
	return out, nil
}

func (r *Repository) customerFinishedAssetInventory(ctx context.Context, customerID int64) ([]*customerAssetState, error) {
	snapshot, err := r.loadMiniCustomerFinishedStock(ctx, r.pool, customerID, false)
	if err != nil {
		return nil, err
	}
	inProduction := map[string]int64{}
	if relationExists(ctx, r.pool, fmt.Sprintf("%s.processing_job_request_items", r.schema)) {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT i.product_id,COALESCE(i.bom_spec_id,0),i.spec_g,
			       SUM(GREATEST(i.target_qty-COALESCE(receipt.actual_inbound_qty,0),0))::bigint
			FROM %s.processing_job_request_items i
			JOIN %s.processing_job_requests request ON request.id=i.request_id
			LEFT JOIN LATERAL (
				SELECT SUM(CASE WHEN si.qty_units>0 THEN si.qty_units
				                    WHEN si.spec_g>0 THEN si.qty_g/si.spec_g ELSE 0 END)::bigint AS actual_inbound_qty
				FROM %s.stock_entries se
				JOIN %s.stock_entry_items si ON si.stock_entry_id=se.id
				WHERE se.work_order_id=i.linked_work_order_id AND se.status='submitted'
				  AND (se.purpose='manufacture' OR (se.entry_type='finished_receipt' AND se.source_type='work_order_complete'))
				  AND COALESCE(se.is_return,false)=false
				  AND si.item_type='finished_product' AND si.product_id=i.product_id
				  AND (COALESCE(i.bom_spec_id,0)=0 OR si.bom_spec_id=i.bom_spec_id)
			) receipt ON true
			WHERE request.customer_id=$1
			  AND request.status NOT IN ('completed','cancelled','rejected')
			  AND i.status NOT IN ('completed','cancelled')
			  AND i.linked_work_order_id>0
			GROUP BY i.product_id,COALESCE(i.bom_spec_id,0),i.spec_g
		`, r.schema, r.schema, r.schema, r.schema), customerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var productID, bomSpecID, specG, qty int64
			if err := rows.Scan(&productID, &bomSpecID, &specG, &qty); err != nil {
				rows.Close()
				return nil, err
			}
			inProduction[miniStockKey(productID, bomSpecID, specG)] += qty
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	availableByWarehouse := map[string]int64{}
	for _, candidate := range snapshot.Candidates {
		availableByWarehouse[miniWarehouseStockKey(candidate.ProductID, candidate.BomSpecID, candidate.SpecG, candidate.Warehouse)] += candidate.AvailableQty
	}
	byKey := map[string]*customerAssetState{}
	states := make([]*customerAssetState, 0)
	for _, inventory := range snapshot.Inventory {
		key := miniStockKey(inventory.ProductID, inventory.BomSpecID, inventory.SpecG)
		state := byKey[key]
		if state == nil {
			unit := strings.TrimSpace(inventory.InventoryUnit)
			if unit == "" {
				unit = "件"
			}
			spec := inventory.BomSpecName
			if spec == "" && inventory.SpecG > 0 {
				spec = fmt.Sprintf("%dg", inventory.SpecG)
			}
			state = &customerAssetState{row: app.CustomerAssetInventory{
				InventoryType: "finished_product", ItemID: inventory.ProductID, ProductID: inventory.ProductID,
				BomSpecID: inventory.BomSpecID, BomVariantID: inventory.BomVariantID, SpecG: inventory.SpecG,
				ItemCode: inventory.SKUCode, ItemName: inventory.ProductName, Spec: spec, Unit: unit,
				QualityStatus: "available", Warehouses: []app.CustomerAssetWarehouseBalance{}, Batches: []app.CustomerAssetInventoryBatch{},
			}, warehouses: map[string]int{}}
			state.row.InProductionQty = float64(inProduction[key])
			byKey[key] = state
			states = append(states, state)
		}
		available := availableByWarehouse[miniWarehouseStockKey(inventory.ProductID, inventory.BomSpecID, inventory.SpecG, inventory.Warehouse)]
		state.row.TotalQty += float64(inventory.TotalQty)
		state.row.AvailableQty += float64(available)
		state.row.OccupiedQty += float64(inventory.ReservedQty)
		idx, ok := state.warehouses[inventory.Warehouse]
		if !ok {
			idx = len(state.row.Warehouses)
			state.warehouses[inventory.Warehouse] = idx
			state.row.Warehouses = append(state.row.Warehouses, app.CustomerAssetWarehouseBalance{WarehouseCode: inventory.Warehouse, WarehouseName: inventory.WarehouseName})
		}
		state.row.Warehouses[idx].AvailableQty += float64(available)
		state.row.Warehouses[idx].OccupiedQty += float64(inventory.ReservedQty)
	}
	for _, batch := range snapshot.Batches {
		key := miniStockKey(batch.ProductID, batch.BomSpecID, batch.SpecG)
		state := byKey[key]
		if state == nil {
			continue
		}
		available := batch.PhysicalQty - batch.ReservedQty
		if available < 0 || !miniDirectShipQualityAvailable(batch.QualityStatus) {
			available = 0
		}
		inbound := ""
		if batch.InboundAt != nil {
			inbound = batch.InboundAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04:05")
		}
		state.row.Batches = append(state.row.Batches, app.CustomerAssetInventoryBatch{
			BatchID: batch.BatchID, BatchNo: batch.BatchCode, WarehouseCode: batch.Warehouse,
			WarehouseName: batch.WarehouseName, TotalQty: float64(batch.PhysicalQty), AvailableQty: float64(available),
			OccupiedQty: float64(batch.ReservedQty), QualityStatus: batch.QualityStatus, InboundAt: inbound,
		})
		if !miniDirectShipQualityAvailable(batch.QualityStatus) {
			state.row.QualityStatus = batch.QualityStatus
		}
	}
	productIDs := make([]int64, 0, len(states))
	for _, state := range states {
		productIDs = append(productIDs, state.row.ProductID)
	}
	if len(productIDs) > 0 {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT id,true
			FROM %s.products
			WHERE id=ANY($1::bigint[]) AND active=true
			  AND COALESCE(is_processing_product,false)=true
			  AND COALESCE(customer_id,0)=$2
		`, r.schema), productIDs, customerID)
		if err != nil {
			return nil, err
		}
		processing := map[int64]bool{}
		for rows.Next() {
			var id int64
			var allowed bool
			if err := rows.Scan(&id, &allowed); err != nil {
				rows.Close()
				return nil, err
			}
			processing[id] = allowed
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		for _, state := range states {
			state.row.CanCreateProcessingRequest = processing[state.row.ProductID]
		}
	}
	return states, nil
}

func (r *Repository) customerMaterialAssetInventory(ctx context.Context, customerID int64, inventoryType string) ([]*customerAssetState, error) {
	if !relationExists(ctx, r.pool, fmt.Sprintf("%s.material_batches", r.schema)) || !relationExists(ctx, r.pool, fmt.Sprintf("%s.material_batch_locations", r.schema)) {
		return []*customerAssetState{}, nil
	}
	reserved := map[int64][2]int64{}
	if relationExists(ctx, r.pool, fmt.Sprintf("%s.customer_processing_material_reservations", r.schema)) {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT material_id,
			       SUM(GREATEST(reserved_g-consumed_g-returned_g,0))::bigint,
			       SUM(GREATEST(reserved_units-consumed_units-returned_units,0))::bigint
			FROM %s.customer_processing_material_reservations
			WHERE source_customer_id=$1 AND status='reserved' AND component_type='material'
			GROUP BY material_id
		`, r.schema), customerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, qtyG, qtyUnits int64
			if err := rows.Scan(&id, &qtyG, &qtyUnits); err != nil {
				rows.Close()
				return nil, err
			}
			reserved[id] = [2]int64{qtyG, qtyUnits}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id,COALESCE(m.code,''),m.name,COALESCE(NULLIF(m.unit,''),'kg'),COALESCE(m.kind,''),COALESCE(m.is_semi_finished,false),
		       b.id,b.batch_code,COALESCE(NULLIF(b.quality_status,''),'unchecked'),
		       to_char(b.received_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),
		       l.warehouse,COALESCE(NULLIF(w.name,''),l.warehouse),GREATEST(l.qty_g,0),GREATEST(l.qty_units,0)
		FROM %s.material_batches b
		JOIN %s.materials m ON m.id=b.material_id AND m.deprecated_at IS NULL
		JOIN %s.material_batch_locations l ON l.material_batch_id=b.id AND (l.qty_g>0 OR l.qty_units>0)
		LEFT JOIN %s.warehouses w ON w.code=l.warehouse
		WHERE COALESCE(NULLIF(b.owner_customer_id,0),m.owner_customer_id)=$1
		ORDER BY m.name,b.received_at DESC,b.id DESC,l.warehouse
	`, r.schema, r.schema, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]*customerAssetState{}
	states := make([]*customerAssetState, 0)
	for rows.Next() {
		var materialID, batchID, qtyG, qtyUnits int64
		var code, name, unit, kind, batchNo, quality, inbound, warehouse, warehouseName string
		var semi bool
		if err := rows.Scan(&materialID, &code, &name, &unit, &kind, &semi, &batchID, &batchNo, &quality, &inbound, &warehouse, &warehouseName, &qtyG, &qtyUnits); err != nil {
			return nil, err
		}
		typeKey := customerMaterialAssetType(kind, semi)
		if typeKey == "" {
			continue
		}
		if inventoryType != "" && inventoryType != typeKey {
			continue
		}
		state := byID[materialID]
		if state == nil {
			state = &customerAssetState{row: app.CustomerAssetInventory{
				InventoryType: typeKey, ItemID: materialID, ItemCode: code, ItemName: name, Unit: customerAssetDisplayUnit(unit),
				QualityStatus: "available", Warehouses: []app.CustomerAssetWarehouseBalance{}, Batches: []app.CustomerAssetInventoryBatch{},
			}, warehouses: map[string]int{}}
			byID[materialID] = state
			states = append(states, state)
		}
		qty := customerAssetQuantity(unit, qtyG, qtyUnits)
		inProduction := float64(0)
		available := qty
		if strings.EqualFold(strings.TrimSpace(warehouse), "wip") {
			inProduction = qty
			available = 0
		} else if !miniDirectShipQualityAvailable(quality) {
			available = 0
		}
		state.row.TotalQty += qty
		state.row.AvailableQty += available
		state.row.InProductionQty += inProduction
		idx, ok := state.warehouses[warehouse]
		if !ok {
			idx = len(state.row.Warehouses)
			state.warehouses[warehouse] = idx
			state.row.Warehouses = append(state.row.Warehouses, app.CustomerAssetWarehouseBalance{WarehouseCode: warehouse, WarehouseName: warehouseName})
		}
		state.row.Warehouses[idx].AvailableQty += available
		state.row.Warehouses[idx].InProductionQty += inProduction
		state.row.Batches = append(state.row.Batches, app.CustomerAssetInventoryBatch{
			BatchID: batchID, BatchNo: batchNo, WarehouseCode: warehouse, WarehouseName: warehouseName,
			TotalQty: qty, AvailableQty: available, InProductionQty: inProduction, QualityStatus: quality, InboundAt: inbound,
		})
		if !miniDirectShipQualityAvailable(quality) {
			state.row.QualityStatus = quality
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for materialID, state := range byID {
		reservedRaw := reserved[materialID]
		occupied := customerAssetQuantity(state.row.Unit, reservedRaw[0], reservedRaw[1])
		if occupied > state.row.AvailableQty {
			occupied = state.row.AvailableQty
		}
		state.row.OccupiedQty = occupied
		state.row.AvailableQty -= occupied
		remaining := occupied
		for idx := range state.row.Batches {
			if remaining <= 0 || state.row.Batches[idx].AvailableQty <= 0 {
				continue
			}
			take := min(remaining, state.row.Batches[idx].AvailableQty)
			state.row.Batches[idx].AvailableQty -= take
			state.row.Batches[idx].OccupiedQty += take
			remaining -= take
			for warehouseIdx := range state.row.Warehouses {
				warehouse := &state.row.Warehouses[warehouseIdx]
				if warehouse.WarehouseCode != state.row.Batches[idx].WarehouseCode {
					continue
				}
				warehouse.AvailableQty -= take
				warehouse.OccupiedQty += take
				break
			}
		}
	}
	return states, nil
}

func (r *Repository) customerLegacyCustodyAssetInventory(ctx context.Context, customerID int64, inventoryType string, existing []*customerAssetState) ([]*customerAssetState, error) {
	if !relationExists(ctx, r.pool, fmt.Sprintf("%s.customer_custody_balances", r.schema)) {
		return []*customerAssetState{}, nil
	}
	seen := map[string]bool{}
	for _, state := range existing {
		seen[state.row.InventoryType+":"+strings.ToLower(strings.TrimSpace(state.row.ItemName))] = true
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.item_id,b.item_type,b.item_name,b.spec,b.quantity_g,b.quantity_units,COALESCE(NULLIF(i.unit,''),'')
		FROM %s.customer_custody_balances b
		LEFT JOIN %s.customer_custody_items i ON i.customer_id=b.customer_id AND i.id=b.item_id
		WHERE b.customer_id=$1 AND (b.quantity_g<>0 OR b.quantity_units<>0)
		ORDER BY b.item_type,b.item_name,b.item_id
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*customerAssetState, 0)
	for rows.Next() {
		var itemID, qtyG, qtyUnits int64
		var itemType, name, spec, unit string
		if err := rows.Scan(&itemID, &itemType, &name, &spec, &qtyG, &qtyUnits, &unit); err != nil {
			return nil, err
		}
		typeKey := ""
		switch strings.ToLower(strings.TrimSpace(itemType)) {
		case "raw_bean", "green_bean", "bean":
			typeKey = "green_bean"
		case "pack", "packaging":
			typeKey = "packaging"
		case "semi_finished":
			typeKey = "semi_finished"
		}
		if typeKey == "" {
			continue
		}
		if inventoryType != "" && inventoryType != typeKey || seen[typeKey+":"+strings.ToLower(strings.TrimSpace(name))] {
			continue
		}
		if unit == "" {
			if qtyUnits != 0 {
				unit = "件"
			} else {
				unit = "g"
			}
		}
		qty := customerAssetQuantity(unit, qtyG, qtyUnits)
		out = append(out, &customerAssetState{row: app.CustomerAssetInventory{
			InventoryType: typeKey, ItemID: itemID, ItemName: name, Spec: spec, Unit: customerAssetDisplayUnit(unit),
			TotalQty: qty, AvailableQty: qty, QualityStatus: "historical", Legacy: true,
			Warehouses: []app.CustomerAssetWarehouseBalance{}, Batches: []app.CustomerAssetInventoryBatch{},
		}, warehouses: map[string]int{}})
	}
	return out, rows.Err()
}

func (r *Repository) ListCustomerAssetInventoryLedger(ctx context.Context, query app.CustomerAssetInventoryLedgerQuery) ([]app.CustomerAssetInventoryLedgerEntry, error) {
	if query.InventoryType == "finished_product" {
		return r.customerFinishedAssetLedger(ctx, query)
	}
	return r.customerMaterialAssetLedger(ctx, query)
}

func (r *Repository) customerMaterialAssetLedger(ctx context.Context, query app.CustomerAssetInventoryLedgerQuery) ([]app.CustomerAssetInventoryLedgerEntry, error) {
	unit := "kg"
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(m.unit,''),'kg') FROM %s.materials m
		WHERE m.id=$1 AND (
		  m.owner_customer_id=$2 OR EXISTS(
		    SELECT 1 FROM %s.material_batches b WHERE b.material_id=m.id AND b.owner_customer_id=$2
		  )
		)
	`, r.schema, r.schema), query.ItemID, query.CustomerID).Scan(&unit); err != nil {
		if legacy, legacyErr := r.customerLegacyCustodyLedger(ctx, query); legacyErr == nil && len(legacy) > 0 {
			return legacy, nil
		}
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.id,to_char(l.created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(l.source_doc_type,''),COALESCE(l.source_doc_type,''),COALESCE(l.source_doc_id,0),
		       COALESCE(l.source_batch_code,''),COALESCE(l.warehouse,''),COALESCE(w.name,''),
		       COALESCE(l.qty_change_g,0),COALESCE(l.qty_change_units,0),COALESCE(l.note,'')
		FROM %s.stock_ledger_entries l
		LEFT JOIN %s.warehouses w ON w.code=l.warehouse
		WHERE l.item_type='material' AND l.item_id=$1 AND l.owner_customer_id=$2
		ORDER BY l.created_at DESC,l.id DESC LIMIT $3
	`, r.schema, r.schema), query.ItemID, query.CustomerID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustomerAssetInventoryLedgerEntry, 0)
	for rows.Next() {
		var row app.CustomerAssetInventoryLedgerEntry
		var qtyG, qtyUnits int64
		if err := rows.Scan(&row.ID, &row.OccurredAt, &row.MovementType, &row.SourceType, &row.SourceID, &row.BatchNo,
			&row.WarehouseCode, &row.WarehouseName, &qtyG, &qtyUnits, &row.Note); err != nil {
			return nil, err
		}
		row.Unit = customerAssetDisplayUnit(unit)
		row.QuantityDelta = customerAssetQuantity(unit, qtyG, qtyUnits)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) customerLegacyCustodyLedger(ctx context.Context, query app.CustomerAssetInventoryLedgerQuery) ([]app.CustomerAssetInventoryLedgerEntry, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.id,to_char(l.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(l.movement_type,''),COALESCE(l.source_type,''),l.source_id,
		       l.qty_g_delta,l.qty_units_delta,COALESCE(l.note,''),COALESCE(NULLIF(i.unit,''),'')
		FROM %s.customer_custody_ledger_entries l
		LEFT JOIN %s.customer_custody_items i ON i.customer_id=l.customer_id AND i.id=l.item_id
		WHERE l.customer_id=$1 AND l.item_id=$2 ORDER BY l.occurred_at DESC,l.id DESC LIMIT $3
	`, r.schema, r.schema), query.CustomerID, query.ItemID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustomerAssetInventoryLedgerEntry, 0)
	for rows.Next() {
		var row app.CustomerAssetInventoryLedgerEntry
		var qtyG, qtyUnits int64
		var unit string
		if err := rows.Scan(&row.ID, &row.OccurredAt, &row.MovementType, &row.SourceType, &row.SourceID, &qtyG, &qtyUnits, &row.Note, &unit); err != nil {
			return nil, err
		}
		if unit == "" {
			if qtyUnits != 0 {
				unit = "件"
			} else {
				unit = "g"
			}
		}
		row.Unit = customerAssetDisplayUnit(unit)
		row.QuantityDelta = customerAssetQuantity(unit, qtyG, qtyUnits)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) customerFinishedAssetLedger(ctx context.Context, query app.CustomerAssetInventoryLedgerQuery) ([]app.CustomerAssetInventoryLedgerEntry, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.id,to_char(l.created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(l.source_doc_type,''),COALESCE(l.source_doc_type,''),COALESCE(l.source_doc_id,0),
		       COALESCE(l.source_batch_code,''),COALESCE(l.warehouse,''),COALESCE(w.name,''),
		       COALESCE(l.qty_change_g,0),COALESCE(l.qty_change_units,0),COALESCE(l.note,'')
		FROM %s.stock_ledger_entries l
		LEFT JOIN %s.warehouses w ON w.code=l.warehouse
		WHERE l.item_type='finished_product' AND l.item_id=$1 AND l.owner_customer_id=$2
		  AND ($3::bigint=0 OR l.bom_spec_id=$3)
		  AND ($4::bigint=0 OR l.spec_g=$4)
		ORDER BY l.created_at DESC,l.id DESC LIMIT $5
	`, r.schema, r.schema), query.ItemID, query.CustomerID, query.BomSpecID, query.SpecG, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustomerAssetInventoryLedgerEntry, 0)
	for rows.Next() {
		var row app.CustomerAssetInventoryLedgerEntry
		var qtyG, qtyUnits int64
		if err := rows.Scan(&row.ID, &row.OccurredAt, &row.MovementType, &row.SourceType, &row.SourceID, &row.BatchNo,
			&row.WarehouseCode, &row.WarehouseName, &qtyG, &qtyUnits, &row.Note); err != nil {
			return nil, err
		}
		row.Unit = "件"
		if query.BomSpecID > 0 || qtyUnits != 0 {
			row.QuantityDelta = float64(qtyUnits)
		} else if query.SpecG > 0 {
			row.QuantityDelta = float64(qtyG) / float64(query.SpecG)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func customerMaterialAssetType(kind string, semi bool) string {
	if semi {
		return "semi_finished"
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pack", "packaging", "package", "packing":
		return "packaging"
	case "bean", "raw_bean", "raw-bean", "green_bean", "green-bean":
		return "green_bean"
	default:
		return ""
	}
}

func customerAssetDisplayUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "公斤":
		return "kg"
	case "g", "克":
		return "g"
	case "unit", "units", "piece", "pcs":
		return "件"
	default:
		if strings.TrimSpace(unit) == "" {
			return "件"
		}
		return strings.TrimSpace(unit)
	}
}

func customerAssetQuantity(unit string, qtyG, qtyUnits int64) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "公斤":
		return float64(qtyG) / 1000
	case "g", "克":
		return float64(qtyG)
	default:
		if qtyUnits != 0 {
			return float64(qtyUnits)
		}
		return float64(qtyG)
	}
}

func customerAssetTypeOrder(value string) int {
	switch value {
	case "finished_product":
		return 1
	case "green_bean":
		return 2
	case "packaging":
		return 3
	case "semi_finished":
		return 4
	default:
		return 9
	}
}
