package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
	infra "orderapp/internal/infrastructure/postgres"
	"reflect"
	"sort"
	"strings"
)

type pendingMaterialSupply struct {
	ID, PlanItemID, OwnerID int64
	No, Warehouse           string
	G, Units                int64
}
type manufacturingExpansion struct {
	Plan     domain.ManufacturingPlan
	Bases    map[string]manufacturingOutputBOMPlanBasis
	Roots    map[string]app.ProductionPlanItem
	Specs    map[string]int64
	Supplies map[string][]pendingMaterialSupply
	Owners   map[string]int64
}

const supplyRecheckMarker = "[[SUPPLY_RECHECK_REQUIRED]]"

func productionSupplyChangeSummary(before, after []app.ProductionPlanItem) string {
	oldItems, newItems := map[int64]app.ProductionPlanItem{}, map[int64]app.ProductionPlanItem{}
	for _, item := range before {
		oldItems[item.ID] = item
	}
	for _, item := range after {
		newItems[item.ID] = item
	}
	added, removed, changed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	operationLabels := func(item app.ProductionPlanItem, out map[string]bool) {
		name := firstNonEmpty(item.OutputName, item.ProductName)
		for _, op := range productionPlanPreviewOperations(item, nil) {
			if op.Operation != "" {
				out[fmt.Sprintf("%s（%s）", op.Operation, name)] = true
			}
		}
	}
	for id, item := range newItems {
		old, exists := oldItems[id]
		if !exists {
			operationLabels(item, added)
			continue
		}
		if math.Abs(old.OutputQty-item.OutputQty) > 0.000001 || old.OutputUnit != item.OutputUnit {
			changed[fmt.Sprintf("%s %s%s→%s%s", firstNonEmpty(item.OutputName, item.ProductName), formatOperationQuantity(old.OutputQty), old.OutputUnit, formatOperationQuantity(item.OutputQty), item.OutputUnit)] = true
		}
	}
	for id, item := range oldItems {
		if _, exists := newItems[id]; !exists {
			operationLabels(item, removed)
		}
	}
	parts := []string{}
	for _, group := range []struct {
		label  string
		values map[string]bool
	}{{"新增工序", added}, {"减少工序", removed}, {"数量变化", changed}} {
		list := make([]string, 0, len(group.values))
		for value := range group.values {
			list = append(list, value)
		}
		sort.Strings(list)
		if len(list) > 0 {
			parts = append(parts, group.label+"："+strings.Join(list, "、"))
		}
	}
	if len(parts) == 0 {
		return "供应来源已更新，请核对生产安排"
	}
	return strings.Join(parts, "；") + "，请核对受影响的工位安排"
}

// productionPlanSupplyChangesTx compares a freshly calculated supply graph to
// the saved draft without modifying either the plan or its reservations.
// The caller holds the plan row lock before using this as a submission guard.
func productionPlanSupplyChangesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) (bool, error) {
	items, err := loadProductionPlanItemsTx(ctx, tx, schema, planID)
	if err != nil {
		return false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT d.depends_on_plan_item_id FROM %[1]s.production_plan_item_dependencies d JOIN %[1]s.production_plan_items pi ON pi.id=d.depends_on_plan_item_id AND pi.production_plan_id=$1 WHERE d.production_plan_id=$1`, schema), planID)
	if err != nil {
		return false, err
	}
	children := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		children[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	roots := make([]app.ProductionPlanItem, 0, len(items))
	for _, item := range items {
		if !children[item.ID] {
			roots = append(roots, item)
		}
	}
	expansion, err := buildManufacturingExpansionTx(ctx, tx, schema, roots)
	if err != nil {
		return false, err
	}
	var stored []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT supply_graph_json FROM %s.production_plans WHERE id=$1`, schema), planID).Scan(&stored); err != nil {
		return false, err
	}
	if len(stored) == 0 || string(stored) == "null" {
		for _, edge := range expansion.Plan.Edges {
			if edge.ShortageQty > 0 || edge.InflightCoveredQty > 0 {
				return true, nil
			}
		}
		for _, node := range expansion.Plan.Nodes {
			if node.InflightCoveredQty > 0 {
				return true, nil
			}
			if node.Action == domain.ManufacturingSupplyManufacture && node.ShortageQty > 0 {
				if _, root := expansion.Roots[node.Item.Key()]; !root {
					return true, nil
				}
			}
		}
		return false, nil
	}
	var oldGraph any
	if err := json.Unmarshal(stored, &oldGraph); err != nil {
		return false, err
	}
	raw, err := json.Marshal(expansion.Plan)
	if err != nil {
		return false, err
	}
	var newGraph any
	if err := json.Unmarshal(raw, &newGraph); err != nil {
		return false, err
	}
	if !reflect.DeepEqual(oldGraph, newGraph) {
		return true, nil
	}
	return productionPlanInflightAllocationsChangedTx(ctx, tx, schema, planID, items, expansion)
}

type supplyIdentity struct {
	Consumer, Supplier, Material, Owner int64
	Warehouse                           string
}
type supplyAmounts struct{ G, Units int64 }

// Equal totals do not imply equal source work orders. Reconstruct the proposed
// allocation with the same ordering and depletion rules used when applying.
func productionPlanInflightAllocationsChangedTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []app.ProductionPlanItem, expansion manufacturingExpansion) (bool, error) {
	itemIDs := map[string]int64{}
	for key, root := range expansion.Roots {
		itemIDs[key] = root.ID
	}
	for _, item := range items {
		if _, root := expansion.Roots[fmt.Sprintf("%s::root:%d", manufacturingItemIdentityKey(item.OutputType, item.OutputProductID, item.BomSpecID), item.ID)]; root {
			continue
		}
		typ, id := item.OutputType, item.OutputProductID
		if typ == "material" {
			id = item.OutputMaterialID
		}
		itemIDs[manufacturingItemIdentityKey(typ, id, item.BomSpecID)] = item.ID
	}
	refs := map[string]domain.ManufacturingItemRef{}
	for _, node := range expansion.Plan.Nodes {
		refs[node.Item.Key()] = node.Item
	}
	want := map[supplyIdentity]supplyAmounts{}
	for _, edge := range expansion.Plan.Edges {
		consumer := itemIDs[edge.ConsumerKey]
		if consumer == 0 {
			continue
		}
		ref := refs[edge.SupplierKey]
		remaining := edge.InflightCoveredQty
		options := expansion.Supplies[edge.SupplierKey]
		for i := range options {
			s := &options[i]
			q := math.Min(remaining, manufacturingQtyFromCanonical(s.G, s.Units, ref.Unit))
			if q <= 0 {
				continue
			}
			g, n := canonicalFromManufacturingQty(q, ref.Unit)
			s.G -= g
			s.Units -= n
			remaining -= q
			key := supplyIdentity{consumer, s.ID, ref.ID, s.OwnerID, s.Warehouse}
			amount := want[key]
			amount.G += g
			amount.Units += n
			want[key] = amount
		}
		expansion.Supplies[edge.SupplierKey] = options
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT production_plan_item_id,supplier_work_order_id,material_id,owner_customer_id,warehouse,required_g,required_units FROM %s.production_supply_allocations WHERE production_plan_id=$1`, schema), planID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	got := map[supplyIdentity]supplyAmounts{}
	for rows.Next() {
		var key supplyIdentity
		var amount supplyAmounts
		if err = rows.Scan(&key.Consumer, &key.Supplier, &key.Material, &key.Owner, &key.Warehouse, &amount.G, &amount.Units); err != nil {
			return false, err
		}
		prior := got[key]
		prior.G += amount.G
		prior.Units += amount.Units
		got[key] = prior
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	return !reflect.DeepEqual(want, got), nil
}

func requestProductionKeyTx(ctx context.Context, tx pgx.Tx, schema, action, actor, key string, payload any) (int64, json.RawMessage, error) {
	if key == "" {
		return 0, nil, nil
	}
	if len(key) > 160 {
		return 0, nil, fmt.Errorf("request_id too long")
	}
	raw, _ := json.Marshal(payload)
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_request_keys(action,actor,request_id,payload_hash) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, schema), action, actor, key, hash); err != nil {
		return 0, nil, err
	}
	var previous string
	var id int64
	var result []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT payload_hash,result_id,result_json FROM %s.production_request_keys WHERE action=$1 AND actor=$2 AND request_id=$3 FOR UPDATE`, schema), action, actor, key).Scan(&previous, &id, &result); err != nil {
		return 0, nil, err
	}
	if previous != hash {
		return 0, nil, fmt.Errorf("相同请求标识的内容已改变，请重新操作")
	}
	return id, result, nil
}
func finishProductionKeyTx(ctx context.Context, tx pgx.Tx, schema, action, actor, key string, id int64, result any) error {
	if key == "" {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_request_keys SET result_id=$4,result_json=$5 WHERE action=$1 AND actor=$2 AND request_id=$3`, schema), action, actor, key, id, raw)
	return err
}

func pendingMaterialSuppliesTx(ctx context.Context, tx pgx.Tx, schema string, basis manufacturingOutputBOMPlanBasis, warehouse string, owner int64) ([]pendingMaterialSupply, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
 SELECT w.id,w.production_plan_item_id,w.work_order_no,w.target_warehouse,
 GREATEST(0,w.planned_output_g-COALESCE(received.g,0)-COALESCE(bound.g,0))::bigint,
 GREATEST(0,CASE WHEN lower(w.output_unit) IN ('g','kg','lb') THEN 0 ELSE floor(w.output_qty)::bigint END-COALESCE(received.units,0)-COALESCE(bound.units,0))::bigint
 FROM %[1]s.work_orders w JOIN %[1]s.production_plan_items pi ON pi.id=w.production_plan_item_id
 LEFT JOIN LATERAL (SELECT SUM(finished_g)::bigint g,SUM(finished_units)::bigint units FROM %[1]s.production_material_receipts r WHERE r.work_order_id=w.id) received ON true
 LEFT JOIN LATERAL (
 SELECT SUM(GREATEST(0,d.required_g-d.delivered_g))::bigint g,SUM(GREATEST(0,d.required_units-d.delivered_units))::bigint units
 FROM %[1]s.work_order_dependencies d JOIN %[1]s.work_orders consumer ON consumer.id=d.work_order_id
 WHERE d.depends_on_work_order_id=w.id AND consumer.status NOT IN ('cancelled','completed')
 ) bound ON true
 JOIN %[1]s.materials m ON m.id=w.output_material_id AND m.deprecated_at IS NULL AND COALESCE(m.owner_customer_id,0)=$5
 WHERE w.output_type='material' AND w.output_material_id=$1 AND w.bom_version_id=$2 AND pi.process_route_id=$3
 AND ($4='' OR w.target_warehouse=$4) AND w.status IN ('released','running','partially_completed')
 ORDER BY w.created_at,w.work_order_no,w.id`, schema), basis.OutputID, basis.VersionID, basis.ProcessRouteID, warehouse, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pendingMaterialSupply{}
	for rows.Next() {
		var s pendingMaterialSupply
		s.OwnerID = owner
		if err := rows.Scan(&s.ID, &s.PlanItemID, &s.No, &s.Warehouse, &s.G, &s.Units); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// The same frozen-root expansion is used by read-only previews and draft creation.
func buildManufacturingExpansionTx(ctx context.Context, tx pgx.Tx, schema string, roots []app.ProductionPlanItem, newOrderPreview ...bool) (manufacturingExpansion, error) {
	result := manufacturingExpansion{Roots: map[string]app.ProductionPlanItem{}, Supplies: map[string][]pendingMaterialSupply{}, Owners: map[string]int64{}}
	policy := ""
	planID := int64(0)
	if len(roots) > 0 {
		planID = roots[0].PlanID
	}
	if planID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT semi_finished_stock_policy FROM %s.production_plans WHERE id=$1`, schema), planID).Scan(&policy); err != nil {
			return result, err
		}
	} else if len(newOrderPreview) > 0 && newOrderPreview[0] {
		policy = "manual"
	}
	rootNeeds := map[int64][]materialConsumptionNeed{}
	allNeeds := []materialConsumptionNeed{}
	for i, root := range roots {
		if root.ID == 0 {
			root.ID = -int64(i + 1)
		}
		roots[i] = root
		needs, err := productionPlanItemConsumptionNeeds(root)
		if err != nil {
			return result, err
		}
		rootNeeds[root.ID] = needs
		allNeeds = append(allNeeds, needs...)
	}
	bases, boms, specs, err := loadDefaultManufacturingOutputBOMsForPlanningTx(ctx, tx, schema, allNeeds)
	if err != nil {
		return result, err
	}
	// Recalculate an existing draft from its frozen upstream recipe and route,
	// even after another version becomes the default. Roots are handled below.
	if len(roots) > 0 && roots[0].PlanID > 0 {
		planID := roots[0].PlanID
		storedItems, e := loadProductionPlanItemsTx(ctx, tx, schema, planID)
		if e != nil {
			return result, e
		}
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT d.depends_on_plan_item_id FROM %[1]s.production_plan_item_dependencies d JOIN %[1]s.production_plan_items i ON i.id=d.depends_on_plan_item_id AND i.production_plan_id=$1 WHERE d.production_plan_id=$1`, schema), planID)
		if e != nil {
			return result, e
		}
		childIDs := map[int64]bool{}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return result, e
			}
			childIDs[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return result, e
		}
		frozen := []app.ProductionPlanItem{}
		var frozenNeeds []materialConsumptionNeed
		liveFrozenKeys := map[string]bool{}
		for _, item := range storedItems {
			if !childIDs[item.ID] {
				continue
			}
			needs, e := productionPlanItemConsumptionNeeds(item)
			if e != nil {
				return result, e
			}
			frozen = append(frozen, item)
			frozenNeeds = append(frozenNeeds, needs...)
			typ, id := item.OutputType, item.OutputProductID
			if typ == "material" {
				id = item.OutputMaterialID
			}
			liveFrozenKeys[manufacturingItemIdentityKey(typ, id, item.BomSpecID)] = true
		}
		if policy == "manual" {
			archiveRows, e := tx.Query(ctx, fmt.Sprintf(`SELECT item_key,item_snapshot FROM %s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=$1 ORDER BY item_key`, schema), planID)
			if e != nil {
				return result, e
			}
			for archiveRows.Next() {
				var key string
				var raw []byte
				if e = archiveRows.Scan(&key, &raw); e != nil {
					archiveRows.Close()
					return result, e
				}
				if liveFrozenKeys[key] {
					continue
				}
				var item app.ProductionPlanItem
				if e = json.Unmarshal(raw, &item); e != nil {
					archiveRows.Close()
					return result, e
				}
				needs, loadErr := productionPlanItemConsumptionNeeds(item)
				if loadErr != nil {
					archiveRows.Close()
					return result, loadErr
				}
				frozen = append(frozen, item)
				frozenNeeds = append(frozenNeeds, needs...)
			}
			e = archiveRows.Err()
			archiveRows.Close()
			if e != nil {
				return result, e
			}
		}
		if len(frozenNeeds) > 0 {
			extraBases, extraBOMs, extraSpecs, e := loadDefaultManufacturingOutputBOMsForPlanningTx(ctx, tx, schema, frozenNeeds)
			if e != nil {
				return result, e
			}
			for key, value := range extraBases {
				if _, exists := bases[key]; !exists {
					bases[key] = value
				}
			}
			for key, value := range extraSpecs {
				if _, exists := specs[key]; !exists {
					specs[key] = value
				}
			}
			known := map[string]bool{}
			for _, bom := range boms {
				known[bom.Output.Key()] = true
			}
			for _, bom := range extraBOMs {
				if !known[bom.Output.Key()] {
					boms = append(boms, bom)
					known[bom.Output.Key()] = true
				}
			}
		}
		seen := map[string]bool{}
		for _, item := range frozen {
			typ, outputID := item.OutputType, item.OutputProductID
			if typ == "material" {
				outputID = item.OutputMaterialID
			}
			ref := domain.ManufacturingItemRef{Type: typ, ID: outputID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, Name: item.OutputName, Unit: item.OutputUnit}
			key := ref.Key()
			if seen[key] {
				return result, fmt.Errorf("上游任务身份重复，请先核对生产安排")
			}
			seen[key] = true
			if item.OutputQty <= 0 || item.MaterialSnapshot == "" || item.ProcessSnapshotJSON == "" {
				return result, fmt.Errorf("%s 的冻结生产资料不完整", item.OutputName)
			}
			var snapshot []materialSnapshotRow
			if e = json.Unmarshal([]byte(item.MaterialSnapshot), &snapshot); e != nil || len(snapshot) == 0 {
				return result, fmt.Errorf("%s 的冻结配方不完整", item.OutputName)
			}
			basisQty, basisUnit := snapshot[0].OutputQty, snapshot[0].OutputUnit
			if basisQty <= 0 {
				basisQty = 1
			}
			if strings.TrimSpace(basisUnit) == "" {
				basisUnit = item.OutputUnit
			}
			outputG, outputUnits := canonicalOutputBasis(basisQty, basisUnit)
			basisInventoryQty := manufacturingQtyFromCanonical(outputG, outputUnits, item.OutputUnit)
			if basisInventoryQty <= 0 {
				return result, fmt.Errorf("%s 的冻结产出单位不兼容", item.OutputName)
			}
			calculationSnapshot, e := manufacturingOutputCalculationSnapshot(item.MaterialSnapshot)
			if e != nil {
				return result, e
			}
			run := ProduceRunRow{Product: item.OutputName, ProductID: outputID, SpecG: item.SpecG, NeedG: outputG, InputG: outputG, MaterialSnapshot: calculationSnapshot}
			needs, ok, e := materialSnapshotNeedsTx(run, InvQty{Units: outputUnits, LooseG: outputG})
			if e != nil {
				return result, e
			}
			if !ok {
				return result, fmt.Errorf("%s 的冻结配方缺少物料", item.OutputName)
			}
			components := []domain.ManufacturingBOMComponent{}
			for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
				t, id, sg := manufacturingNeedIdentity(need)
				bs, bv := manufacturingNeedBOMSpecIdentity(need)
				unit := need.Unit
				if basis, ok := bases[manufacturingItemIdentityKey(t, id, bs)]; ok {
					unit = basis.InventoryUnit
				}
				g, n := manufacturingNeedCanonicalQuantities(need)
				qty := manufacturingQtyFromCanonical(g, n, unit)
				if qty > 0 {
					components = append(components, domain.ManufacturingBOMComponent{Item: domain.ManufacturingItemRef{Type: t, ID: id, BomSpecID: bs, BomVariantID: bv, Name: need.MaterialName, Unit: unit}, Qty: qty})
					specs[manufacturingItemIdentityKey(t, id, bs)] = sg
				}
			}
			basis := manufacturingOutputBOMPlanBasis{VersionID: item.BomVersionID, OutputType: typ, OutputID: outputID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, OutputName: item.OutputName, InventoryUnit: item.OutputUnit, OutputSpecG: item.SpecG, OutputQty: basisQty, OutputUnit: basisUnit, ProcessRouteID: item.ProcessRouteID, FrozenProcessRouteSnapshot: item.ProcessSnapshotJSON, Snapshot: item.MaterialSnapshot}
			bases[key] = basis
			bom := domain.ManufacturingBOM{VersionID: item.BomVersionID, Output: ref, OutputQty: basisInventoryQty, Components: components}
			configureManufacturingGrossRatioBOM(item.MaterialSnapshot, outputG, item.OutputUnit, &bom)
			replaced := false
			for i := range boms {
				if boms[i].Output.Key() == key {
					boms[i] = bom
					replaced = true
					break
				}
			}
			if !replaced {
				boms = append(boms, bom)
			}
		}
	}
	result.Bases = bases
	result.Specs = specs
	demands := []domain.ManufacturingDemand{}
	for _, root := range roots {
		typ, id := root.OutputType, root.OutputProductID
		if typ == "material" {
			id = root.OutputMaterialID
		}
		if typ == "" {
			typ = "product"
			id = root.ProductID
		}
		unit := firstNonEmpty(root.OutputUnit, root.InventoryUnit, "g")
		qty := root.OutputQty
		if qty <= 0 {
			qty = manufacturingQtyFromCanonical(root.PlannedOutputG, int64(root.SalesSpecCount), unit)
		}
		ref := domain.ManufacturingItemRef{Type: typ, ID: id, BomSpecID: root.BomSpecID, Name: firstNonEmpty(root.OutputName, root.ProductName), Unit: unit, Scope: fmt.Sprintf("root:%d", root.ID)}
		components := []domain.ManufacturingBOMComponent{}
		for _, need := range rootNeeds[root.ID] {
			t, id, sg := manufacturingNeedIdentity(need)
			bs, bv := manufacturingNeedBOMSpecIdentity(need)
			key := manufacturingItemIdentityKey(t, id, bs)
			u := need.Unit
			if basis, ok := bases[key]; ok {
				u = basis.InventoryUnit
			}
			if t == "product" && bases[key].InventoryUnit == "" {
				u = "g"
			}
			g, n := manufacturingNeedCanonicalQuantities(need)
			components = append(components, domain.ManufacturingBOMComponent{Item: domain.ManufacturingItemRef{Type: t, ID: id, BomSpecID: bs, BomVariantID: bv, Name: need.MaterialName, Unit: u}, Qty: manufacturingQtyFromCanonical(g, n, u)})
			specs[key] = sg
		}
		boms = append(boms, domain.ManufacturingBOM{VersionID: root.BomVersionID, Output: ref, OutputQty: qty, Components: components})
		demands = append(demands, domain.ManufacturingDemand{Item: ref, Qty: qty, TargetWarehouse: root.TargetWarehouse})
		result.Roots[ref.Key()] = root
	}
	refs := map[string]domain.ManufacturingItemRef{}
	for _, b := range boms {
		for _, c := range b.Components {
			refs[c.Item.Key()] = c.Item
		}
	}
	keys := []string{}
	for key := range refs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	available := map[string]float64{}
	inflight := map[string]float64{}
	for _, key := range keys {
		ref := refs[key]
		owner := int64(0)
		if ref.Type == "material" {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(owner_customer_id,0) FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, schema), ref.ID).Scan(&owner); err != nil {
				return result, err
			}
		}
		result.Owners[key] = owner
		opts, err := componentSourceOptionsTx(ctx, tx, schema, app.ProductionPlanComponentSource{ComponentType: ref.Type, ComponentID: ref.ID, ComponentBOMSpecID: ref.BomSpecID, ComponentSpecG: specs[key]}, owner)
		if err != nil {
			return result, err
		}
		eligibleWarehouses := map[string]bool{}
		var g, n int64
		for _, o := range opts {
			eligibleWarehouses[o.Warehouse] = true
			if o.OwnerCustomerID == owner {
				g += o.AvailableG
				n += o.AvailableUnits
			}
		}
		available[key] = manufacturingQtyFromCanonical(g, n, ref.Unit)
		if policy == "manual" && ref.Type == "material" {
			if _, isManufactured := bases[key]; isManufactured {
				var isSemi bool
				if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(is_semi_finished,false) FROM %s.materials WHERE id=$1`, schema), ref.ID).Scan(&isSemi); err != nil {
					return result, err
				}
				if isSemi {
					var selectedG, selectedUnits int64
					if planID > 0 {
						if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0),COALESCE(SUM(qty_units),0) FROM %s.production_plan_semi_finished_selections WHERE production_plan_id=$1 AND item_key=$2`, schema), planID, key).Scan(&selectedG, &selectedUnits); err != nil {
							return result, err
						}
					}
					selected := manufacturingQtyFromCanonical(selectedG, selectedUnits, ref.Unit)
					available[key], err = semiFinishedStockBudget(policy, true, available[key], selected)
					if err != nil {
						return result, fmt.Errorf("%s：%w", ref.Name, err)
					}
				}
			}
		}
		if ref.Type == "material" {
			if basis, ok := bases[key]; ok {
				supplies, err := pendingMaterialSuppliesTx(ctx, tx, schema, basis, "", owner)
				if err != nil {
					return result, err
				}
				for _, s := range supplies {
					if !eligibleWarehouses[s.Warehouse] {
						continue
					}
					result.Supplies[key] = append(result.Supplies[key], s)
					inflight[key] += manufacturingQtyFromCanonical(s.G, s.Units, ref.Unit)
				}
			}
		}
	}
	for _, bom := range boms {
		consumerOwner := result.Owners[bom.Output.Key()]
		if root, ok := result.Roots[bom.Output.Key()]; ok {
			consumerOwner = root.CustomerID
		}
		for _, component := range bom.Components {
			if owner := result.Owners[component.Item.Key()]; owner > 0 && owner != consumerOwner {
				return result, fmt.Errorf("组件「%s」属于其他货主，不能用于本生产任务", component.Item.Name)
			}
		}
	}
	result.Plan, err = domain.BuildMultilevelManufacturingPlanWithSupply(demands, boms, available, inflight)
	return result, err
}

// Walk only new-production edges: stock-covered and unrelated orders do not
// belong to the trace of a newly manufactured upstream task.
func manufacturingUpstreamDemandRoots(expansion manufacturingExpansion, key string) []app.ProductionPlanItem {
	seen := map[string]bool{}
	roots := []app.ProductionPlanItem{}
	var visit func(string)
	visit = func(k string) {
		if seen[k] {
			return
		}
		seen[k] = true
		if root, ok := expansion.Roots[k]; ok {
			roots = append(roots, root)
			return
		}
		for _, edge := range expansion.Plan.Edges {
			if edge.SupplierKey == k && edge.ShortageQty > 0 {
				visit(edge.ConsumerKey)
			}
		}
	}
	visit(key)
	sort.Slice(roots, func(i, j int) bool { return roots[i].ID < roots[j].ID })
	return roots
}

func createExpandedProductionItemsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, roots []app.ProductionPlanItem) (app.ProductionPlanPreview, error) {
	return createExpandedProductionItemsReusingTx(ctx, tx, schema, planID, roots, nil, nil)
}

func createExpandedProductionItemsReusingTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, roots []app.ProductionPlanItem, reuse map[string]app.ProductionPlanItem, prepared *manufacturingExpansion) (app.ProductionPlanPreview, error) {
	var expansion manufacturingExpansion
	var err error
	if prepared == nil {
		expansion, err = buildManufacturingExpansionTx(ctx, tx, schema, roots)
	} else {
		expansion = *prepared
	}
	if err != nil {
		return app.ProductionPlanPreview{}, err
	}
	result := app.ProductionPlanPreview{Items: append([]app.ProductionPlanItem{}, roots...), ManufacturingPlan: expansion.Plan, SupplyAllocations: []app.ProductionSupplyAllocation{}}
	items := map[string]app.ProductionPlanItem{}
	for k, v := range expansion.Roots {
		items[k] = v
	}
	for _, node := range expansion.Plan.Nodes {
		if _, root := items[node.Item.Key()]; root {
			continue
		}
		if node.Action != domain.ManufacturingSupplyManufacture || node.ShortageQty <= 0 {
			continue
		}
		basis := expansion.Bases[node.Item.Key()]
		previous, exists := reuse[node.Item.Key()]
		if exists && (previous.BomVersionID != basis.VersionID || previous.ProcessRouteID != basis.ProcessRouteID || previous.BomVariantID != basis.BomVariantID) {
			return result, fmt.Errorf("%s 的冻结配方或工艺与当前配置不同，请先核对生产配置", previous.OutputName)
		}
		insertPlanID := planID
		if exists {
			insertPlanID = 0
		}
		item, err := insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, insertPlanID, basis, node.ShortageQty, manufacturingUpstreamDemandRoots(expansion, node.Item.Key()))
		if err != nil {
			return result, err
		}
		if exists {
			item.ID, item.PlanID = previous.ID, planID
			item.TargetWarehouse = previous.TargetWarehouse
			item.MaterialSnapshot, err = freezeManufacturingOutputMaterialSnapshot(previous.MaterialSnapshot)
			if err != nil {
				return result, err
			}
			item.ProcessSnapshotJSON = previous.ProcessSnapshotJSON
			_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET output_qty=$2,planned_inventory_qty=$2,planned_g=$3,planned_output_g=$4,gap_g=$4,sales_spec_count=$5,order_nos=$6,component_snapshot_json=$7::jsonb WHERE id=$1`, schema), item.ID, item.OutputQty, item.PlannedG, item.PlannedOutputG, item.SalesSpecCount, item.OrderNos, item.MaterialSnapshot)
			if err != nil {
				return result, err
			}
			if previous.PlannedG != item.PlannedG || previous.PlannedOutputG != item.PlannedOutputG || math.Abs(previous.OutputQty-item.OutputQty) > 0.000001 {
				_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_operation_splits SET note=CASE WHEN strpos(COALESCE(note,''),$2)>0 THEN note ELSE COALESCE(note,'') || $2 END WHERE production_plan_item_id=$1`, schema), item.ID, supplyRecheckMarker)
				if err != nil {
					return result, err
				}
			}
			delete(reuse, node.Item.Key())
		}
		if item.ID == 0 {
			item.ID = -int64(len(items) + 1)
		}
		item.CustomerID = expansion.Owners[node.Item.Key()]
		if planID > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET customer_id=$2 WHERE id=$1`, schema), item.ID, item.CustomerID); err != nil {
				return result, err
			}
		}
		items[node.Item.Key()] = item
		result.Items = append(result.Items, item)
	}
	for _, edge := range expansion.Plan.Edges {
		consumer, exists := items[edge.ConsumerKey]
		if !exists {
			continue
		}
		var ref domain.ManufacturingItemRef
		for _, n := range expansion.Plan.Nodes {
			if n.Item.Key() == edge.SupplierKey {
				ref = n.Item
				break
			}
		}
		sg := expansion.Specs[edge.SupplierKey]
		if edge.ShortageQty > 0 {
			g, n := canonicalFromManufacturingQty(edge.ShortageQty, ref.Unit)
			if supplier, ok := items[edge.SupplierKey]; ok {
				if planID > 0 {
					if err := insertProductionPlanItemDependencyTx(ctx, tx, schema, planID, consumer.ID, supplier.ID, ref.Type, ref.ID, ref.BomSpecID, ref.BomVariantID, sg, g, n); err != nil {
						return result, err
					}
				}
			} else if planID > 0 {
				if err := insertProductionPlanSupplyGapTx(ctx, tx, schema, planID, consumer.ID, ref.Type, ref.ID, ref.Name, g, n, "no_default_"+ref.Type+"_bom"); err != nil {
					return result, err
				}
			}
		}
		remaining := edge.InflightCoveredQty
		options := expansion.Supplies[edge.SupplierKey]
		for i := range options {
			s := &options[i]
			q := math.Min(remaining, manufacturingQtyFromCanonical(s.G, s.Units, ref.Unit))
			if q <= 0 {
				continue
			}
			g, n := canonicalFromManufacturingQty(q, ref.Unit)
			s.G -= g
			s.Units -= n
			remaining -= q
			allocation := app.ProductionSupplyAllocation{PlanItemID: consumer.ID, SupplierWorkOrderID: s.ID, SupplierWorkOrderNo: s.No, MaterialID: ref.ID, MaterialName: ref.Name, Warehouse: s.Warehouse, OwnerCustomerID: s.OwnerID, RequiredG: g, RequiredUnits: n, Status: "proposed"}
			if planID > 0 {
				if err := insertProductionPlanItemDependencyTx(ctx, tx, schema, planID, consumer.ID, s.PlanItemID, ref.Type, ref.ID, ref.BomSpecID, ref.BomVariantID, sg, g, n); err != nil {
					return result, err
				}
				if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_supply_allocations(production_plan_id,production_plan_item_id,supplier_work_order_id,supplier_plan_item_id,material_id,warehouse,owner_customer_id,required_g,required_units) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, schema), planID, consumer.ID, s.ID, s.PlanItemID, ref.ID, s.Warehouse, s.OwnerID, g, n).Scan(&allocation.ID); err != nil {
					return result, err
				}
			}
			result.SupplyAllocations = append(result.SupplyAllocations, allocation)
		}
		expansion.Supplies[edge.SupplierKey] = options
	}
	if planID == 0 {
		result.PickingVersion = 1
		keys := []string{}
		for key := range items {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			item := items[key]
			needs, err := productionPlanItemConsumptionNeeds(item)
			if err != nil {
				return result, err
			}
			for _, need := range needs {
				typ, id, sg := manufacturingNeedIdentity(need)
				bs, bv := manufacturingNeedBOMSpecIdentity(need)
				g, n := manufacturingNeedCanonicalQuantities(need)
				source := app.ProductionPlanComponentSource{ID: -int64(len(result.ComponentSources) + 1), ProductionPlanItemID: item.ID, BOMVersionID: item.BomVersionID, ComponentType: typ, ComponentID: id, ComponentBOMSpecID: bs, ComponentBOMVariantID: bv, ComponentSpecG: sg, ComponentName: need.MaterialName, Unit: need.Unit, RequiredG: g, RequiredUnits: n, AllocationMode: "auto"}
				for _, edge := range expansion.Plan.Edges {
					if edge.ConsumerKey != key {
						continue
					}
					ref, ok := func() (domain.ManufacturingItemRef, bool) {
						for _, node := range expansion.Plan.Nodes {
							if node.Item.Key() == edge.SupplierKey {
								return node.Item, true
							}
						}
						return domain.ManufacturingItemRef{}, false
					}()
					if !ok || ref.Type != typ || ref.ID != id || ref.BomSpecID != bs {
						continue
					}
					pending := edge.InflightCoveredQty
					if _, exists := items[edge.SupplierKey]; exists {
						pending += edge.ShortageQty
					}
					dg, dn := canonicalFromManufacturingQty(pending, need.Unit)
					source.UpstreamG += dg
					source.UpstreamUnits += dn
				}
				source.RequiredG = nonnegativeQuantity(g - source.UpstreamG)
				source.RequiredUnits = nonnegativeQuantity(n - source.UpstreamUnits)
				source.Options, err = componentSourceOptionsTx(ctx, tx, schema, source, item.CustomerID)
				if err != nil {
					return result, err
				}
				result.ComponentSources = append(result.ComponentSources, source)
			}
		}
		if err := preparePickingSourcesTx(ctx, tx, schema, 0, result.ComponentSources, "draft"); err != nil {
			return result, err
		}
	}

	result.MaterialSummary = aggregateProductionPlanMaterialSummary(result.Items)
	if planID > 0 {
		// Freeze every reachable manufacturing recipe, including one completely
		// covered by stock or in-flight supply and therefore without a live task.
		var policy string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT semi_finished_stock_policy FROM %s.production_plans WHERE id=$1`, schema), planID).Scan(&policy); err != nil {
			return result, err
		}
		if policy == "manual" {
			keys := make([]string, 0, len(expansion.Bases))
			for key, basis := range expansion.Bases {
				if basis.Snapshot != "" {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				basis := expansion.Bases[key]
				var exists bool
				if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=$1 AND item_key=$2)`, schema), planID, key).Scan(&exists); err != nil {
					return result, err
				}
				if exists {
					continue
				}
				item, ok := items[key]
				if !ok {
					item, err = insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, 0, basis, 1, nil)
					if err != nil {
						return result, fmt.Errorf("%s 的冻结生产资料不完整：%w", basis.OutputName, err)
					}
				}
				item.PlanID = planID
				var snapshot []byte
				if snapshot, err = json.Marshal(item); err != nil {
					return result, err
				}
				if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_plan_frozen_manufacturing_outputs(production_plan_id,item_key,item_snapshot) VALUES($1,$2,$3::jsonb) ON CONFLICT DO NOTHING`, schema), planID, key, snapshot); err != nil {
					return result, err
				}
			}
		}
		raw, _ := json.Marshal(expansion.Plan)
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET supply_graph_json=$2 WHERE id=$1`, schema), planID, raw)
	}
	return result, err
}

func buildStockProductionRootsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, targets []app.StockProductionTarget) ([]app.ProductionPlanItem, error) {
	roots := []app.ProductionPlanItem{}
	merged := []app.StockProductionTarget{}
	byTarget := map[string]int{}
	for _, t := range targets {
		key := fmt.Sprintf("%d:%s", t.OutputMaterialID, firstNonEmpty(t.TargetWarehouse, "wip"))
		if i, ok := byTarget[key]; ok {
			merged[i].OutputQty += t.OutputQty
		} else {
			byTarget[key] = len(merged)
			merged = append(merged, t)
		}
	}
	for _, target := range merged {
		var active, semi bool
		var owner int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (deprecated_at IS NULL),is_semi_finished,COALESCE(owner_customer_id,0) FROM %s.materials WHERE id=$1`, schema), target.OutputMaterialID).Scan(&active, &semi, &owner); err != nil {
			return nil, err
		}
		if !active || !semi {
			return nil, fmt.Errorf("备货产出必须是有效自制物料")
		}
		bases, _, _, err := loadDefaultManufacturingOutputBOMsForPlanningTx(ctx, tx, schema, []materialConsumptionNeed{{MaterialID: target.OutputMaterialID}})
		if err != nil {
			return nil, err
		}
		basis, ok := bases[manufacturingMaterialKey(target.OutputMaterialID)]
		if !ok {
			return nil, fmt.Errorf("自制物料尚未配置默认已发布 BOM")
		}
		item, err := insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, planID, basis, target.OutputQty, nil)
		if err != nil {
			return nil, err
		}
		warehouse := firstNonEmpty(strings.TrimSpace(target.TargetWarehouse), "wip")
		var whOwner int64
		var kind string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0),kind FROM %s.warehouses WHERE code=$1 AND active=true`, schema), warehouse).Scan(&whOwner, &kind); err != nil {
			return nil, fmt.Errorf("请选择有效目标仓库")
		}
		if whOwner != owner && !(whOwner == 0 && kind == "wip") {
			return nil, fmt.Errorf("目标仓库与物料货主不一致")
		}
		item.TargetWarehouse = warehouse
		item.CustomerID = owner
		if planID > 0 {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET target_warehouse=$2,customer_id=$3 WHERE id=$1`, schema), item.ID, warehouse, owner); err != nil {
				return nil, err
			}
		}
		roots = append(roots, item)
	}
	return roots, nil
}

func (r Repository) PreviewProductionPlan(ctx context.Context, cmd app.CreateProductionPlanCommand) (app.ProductionPlanPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return app.ProductionPlanPreview{}, err
	}
	defer tx.Rollback(ctx)
	roots := []app.ProductionPlanItem{}
	if cmd.SourceType == "stock" {
		roots, err = buildStockProductionRootsTx(ctx, tx, r.schema, 0, cmd.Items)
	} else {
		var needs []app.StartNeed
		needs, err = r.productionPlanSelectedNeeds(ctx, tx, cmd)
		if err == nil {
			for _, group := range groupStartNeedsForRuns(needs, cmd.InputByKey) {
				var item app.ProductionPlanItem
				item, err = createProductionPlanItemForGroupTx(ctx, tx, r.schema, 0, group)
				if err != nil {
					break
				}
				roots = append(roots, item)
			}
		}
	}
	if err != nil {
		return app.ProductionPlanPreview{}, err
	}
	if len(roots) == 0 {
		return app.ProductionPlanPreview{}, fmt.Errorf("请选择待计划需求")
	}
	expansion, err := buildManufacturingExpansionTx(ctx, tx, r.schema, roots, cmd.SourceType != "stock")
	if err != nil {
		return app.ProductionPlanPreview{}, err
	}
	return createExpandedProductionItemsReusingTx(ctx, tx, r.schema, 0, roots, nil, &expansion)
}

func (r Repository) createStockProductionPlan(ctx context.Context, cmd app.CreateProductionPlanCommand) (app.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	defer tx.Rollback(ctx)
	id, _, err := requestProductionKeyTx(ctx, tx, r.schema, "create_plan", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if id > 0 {
		return loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_plans(plan_no,source_type,status,customer_id,created_by,picking_version) VALUES('PP-TMP-'||nextval('%s.production_plans_id_seq')::text,'stock','draft',$1,$2,1) RETURNING id`, r.schema, r.schema), cmd.CustomerID, cmd.Operator).Scan(&id); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET plan_no=$2 WHERE id=$1`, r.schema), id, productionPlanNo(id)); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	roots, err := buildStockProductionRootsTx(ctx, tx, r.schema, id, cmd.Items)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	preview, err := createExpandedProductionItemsTx(ctx, tx, r.schema, id, roots)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, id, preview.Items); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &id, "create", infra.StrPtr("status"), nil, infra.StrPtr("draft"), infra.AuditMeta{"source_type": "stock", "items": cmd.Items}); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	result, err := loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "create_plan", cmd.Operator, cmd.RequestID, id, nil); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// Supplier rows serialize submission with receipts and cancellation. Draft proposals do not reserve supply.
func validateInflightSupplyAtSubmitTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT supplier_work_order_id,SUM(required_g)::bigint,SUM(required_units)::bigint FROM %s.production_supply_allocations WHERE production_plan_id=$1 AND status='proposed' GROUP BY supplier_work_order_id ORDER BY supplier_work_order_id`, schema), planID)
	if err != nil {
		return err
	}
	type demand struct{ id, g, n int64 }
	var needs []demand
	for rows.Next() {
		var d demand
		if err = rows.Scan(&d.id, &d.g, &d.n); err != nil {
			rows.Close()
			return err
		}
		needs = append(needs, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range needs {
		var b manufacturingOutputBOMPlanBasis
		var wh, status string
		var owner int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.output_material_id,w.bom_version_id,pi.process_route_id,w.target_warehouse,w.status,COALESCE(m.owner_customer_id,0) FROM %[1]s.work_orders w JOIN %[1]s.production_plan_items pi ON pi.id=w.production_plan_item_id JOIN %[1]s.materials m ON m.id=w.output_material_id WHERE w.id=$1 FOR UPDATE OF w`, schema), d.id).Scan(&b.OutputID, &b.VersionID, &b.ProcessRouteID, &wh, &status, &owner)
		if err != nil {
			return err
		}
		options, err := pendingMaterialSuppliesTx(ctx, tx, schema, b, wh, owner)
		if err != nil {
			return err
		}
		valid := false
		for _, o := range options {
			if o.ID == d.id && o.G >= d.g && o.Units >= d.n {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("在途供应余量已变化，请刷新草稿后重新安排生产")
		}
	}
	return nil
}

func loadProductionSupplyAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]app.ProductionSupplyAllocation, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT a.id,a.production_plan_item_id,a.work_order_id,a.supplier_work_order_id,w.work_order_no,a.material_id,COALESCE(m.name,''),a.warehouse,a.owner_customer_id,a.required_g,a.required_units,COALESCE(d.delivered_g,0),COALESCE(d.delivered_units,0),CASE WHEN consumer.status='cancelled' THEN 'released' WHEN w.status='completed' AND (COALESCE(d.delivered_g,0)<a.required_g OR COALESCE(d.delivered_units,0)<a.required_units) THEN 'shortfall' ELSE a.status END
 FROM %[1]s.production_supply_allocations a JOIN %[1]s.work_orders w ON w.id=a.supplier_work_order_id LEFT JOIN %[1]s.materials m ON m.id=a.material_id
 LEFT JOIN %[1]s.work_orders consumer ON consumer.id=a.work_order_id
 LEFT JOIN %[1]s.work_order_dependencies d ON d.work_order_id=a.work_order_id AND d.depends_on_work_order_id=a.supplier_work_order_id AND d.material_id=a.material_id
 WHERE a.production_plan_id=$1 ORDER BY a.id`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.ProductionSupplyAllocation{}
	for rows.Next() {
		var a app.ProductionSupplyAllocation
		if err = rows.Scan(&a.ID, &a.PlanItemID, &a.WorkOrderID, &a.SupplierWorkOrderID, &a.SupplierWorkOrderNo, &a.MaterialID, &a.MaterialName, &a.Warehouse, &a.OwnerCustomerID, &a.RequiredG, &a.RequiredUnits, &a.DeliveredG, &a.DeliveredUnits, &a.Status); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func guardSupplierCancellationTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID, excludedPlanID int64) error {
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=$1 FOR UPDATE`, schema), workOrderID).Scan(&id); err != nil {
		return err
	}
	var count int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %[1]s.work_order_dependencies d JOIN %[1]s.work_orders c ON c.id=d.work_order_id WHERE d.depends_on_work_order_id=$1 AND c.status NOT IN ('cancelled','completed') AND ($2=0 OR c.production_plan_id<>$2) AND (d.required_g>d.delivered_g OR d.required_units>d.delivered_units)`, schema), workOrderID, excludedPlanID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该上游工单仍有未兑现的下游供应分配，请先取消或解除下游关联")
	}
	return nil
}

func (r Repository) RefreshProductionPlanSupply(ctx context.Context, id int64, operator string) (app.ProductionPlanDetail, error) {
	result, err := r.SyncProductionPlanSupply(ctx, id, operator, "", false)
	return result.Plan, err
}

func (r Repository) SyncProductionPlanSupply(ctx context.Context, id int64, operator, token string, checkOnly bool) (app.ProductionPlanSupplySyncResult, error) {
	result := app.ProductionPlanSupplySyncResult{}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&status); err != nil {
		return result, err
	}
	if status != "draft" {
		return result, fmt.Errorf("仅草稿可更新供应；已提交工单继续使用冻结快照")
	}
	var workOrders int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.work_orders WHERE production_plan_id=$1`, r.schema), id).Scan(&workOrders); err != nil {
		return result, err
	}
	if workOrders > 0 {
		return result, fmt.Errorf("草稿已有工单，无法自动更新供应")
	}
	current, err := loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	if token != "" && token != current.DraftToken {
		return result, fmt.Errorf("生产计划草稿版本已变化，请重新核对")
	}
	changed, err := productionPlanSupplyChangesTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	result.Plan, result.Changed = current, changed
	if !changed {
		result.Summary = "已按最新供应核对"
		return result, nil
	}
	result.Summary = "库存或在产供应已变化，生产安排需要更新"
	if checkOnly {
		return result, nil
	}
	// An order draft created before manual selection must keep its original
	// half-finished allocation, including an original zero-stock allocation.
	var legacyCovered bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(p.semi_finished_stock_policy,'')='' AND COALESCE(p.source_type,'')<>'stock' AND (EXISTS (
 SELECT 1 FROM jsonb_array_elements(COALESCE(p.supply_graph_json->'nodes','[]'::jsonb)) n
 JOIN %[1]s.materials m ON m.id=CASE WHEN n->'item'->>'type'='material' THEN COALESCE(NULLIF(n->'item'->>'id',''),'0')::bigint ELSE 0 END
 WHERE COALESCE(m.is_semi_finished,false) AND %[2]s
	 ) OR (p.supply_graph_json IS NULL AND EXISTS (
	 SELECT 1 FROM %[1]s.production_plan_component_sources s JOIN %[1]s.materials m ON m.id=s.component_id
	 WHERE s.production_plan_id=$1 AND s.component_type='material' AND COALESCE(m.is_semi_finished,false) AND %[2]s
	 ))) FROM %[1]s.production_plans p WHERE p.id=$1`, r.schema, legacySemiFinishedManufacturableSQL(r.schema, "p.id", "m.id")), id).Scan(&legacyCovered); err != nil {
		return result, err
	}
	if legacyCovered {
		result.Summary = "原计划包含半成品，供应发生变化；请在半成品选择中核对后再调整"
		return result, nil
	}
	items, err := loadProductionPlanItemsTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	adjustments, err := preservedPickingAdjustmentsTx(ctx, tx, r.schema, id, items)
	if err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT d.depends_on_plan_item_id FROM %[1]s.production_plan_item_dependencies d JOIN %[1]s.production_plan_items pi ON pi.id=d.depends_on_plan_item_id AND pi.production_plan_id=$1 WHERE d.production_plan_id=$1`, r.schema), id)
	if err != nil {
		return result, err
	}
	children := map[int64]bool{}
	for rows.Next() {
		var child int64
		if err = rows.Scan(&child); err != nil {
			rows.Close()
			return result, err
		}
		children[child] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	roots := []app.ProductionPlanItem{}
	reuse := map[string]app.ProductionPlanItem{}
	for _, item := range items {
		if !children[item.ID] {
			roots = append(roots, item)
			continue
		}
		typ, outputID := item.OutputType, item.OutputProductID
		if typ == "material" {
			outputID = item.OutputMaterialID
		}
		key := manufacturingItemIdentityKey(typ, outputID, item.BomSpecID)
		if _, exists := reuse[key]; exists {
			return result, fmt.Errorf("存在多个相同产出任务，请先核对生产安排")
		}
		reuse[key] = item
	}
	expansion, err := buildManufacturingExpansionTx(ctx, tx, r.schema, roots)
	if err != nil {
		return result, err
	}
	for _, table := range []string{"production_supply_allocations", "production_plan_item_dependencies", "production_plan_supply_gaps"} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_id=$1`, r.schema, table), id); err != nil {
			return result, err
		}
	}
	expanded, err := createExpandedProductionItemsReusingTx(ctx, tx, r.schema, id, roots, reuse, &expansion)
	if err != nil {
		return result, err
	}
	for _, item := range reuse {
		if err = archiveSemiFinishedCapacityTx(ctx, tx, r.schema, id, item); err != nil {
			return result, err
		}
		for _, table := range []string{"production_plan_operation_splits", "production_plan_component_sources"} {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_item_id=$1`, r.schema, table), item.ID); err != nil {
				return result, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_items WHERE id=$1`, r.schema), item.ID); err != nil {
			return result, err
		}
	}
	if err = syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, id, expanded.Items); err != nil {
		return result, err
	}
	if err = restorePickingAdjustmentsTx(ctx, tx, r.schema, id, expanded.Items, adjustments); err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET picking_version=1,revision=revision+1 WHERE id=$1`, r.schema), id); err != nil {
		return result, err
	}
	result.Summary = productionSupplyChangeSummary(current.Items, expanded.Items)
	if err = infra.AuditInsertTx(ctx, tx, r.schema, operator, "production_plan", &id, "refresh_supply", nil, nil, nil, infra.AuditMeta{"item_count": len(expanded.Items), "changes": result.Summary}); err != nil {
		return result, err
	}
	result.Plan, err = loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return app.ProductionPlanSupplySyncResult{}, err
	}
	return result, nil
}

func (r Repository) MaterialReceiptTotals(ctx context.Context, id int64) (int64, int64, error) {
	var g, n int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(finished_g),0)::bigint,COALESCE(SUM(finished_units),0)::bigint FROM %s.production_material_receipts WHERE work_order_id=$1`, r.schema), id).Scan(&g, &n)
	return g, n, err
}
