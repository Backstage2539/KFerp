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
func buildManufacturingExpansionTx(ctx context.Context, tx pgx.Tx, schema string, roots []app.ProductionPlanItem) (manufacturingExpansion, error) {
	result := manufacturingExpansion{Roots: map[string]app.ProductionPlanItem{}, Supplies: map[string][]pendingMaterialSupply{}, Owners: map[string]int64{}}
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
	expansion, err := buildManufacturingExpansionTx(ctx, tx, schema, roots)
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
		item, err := insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, planID, basis, node.ShortageQty, manufacturingUpstreamDemandRoots(expansion, node.Item.Key()))
		if err != nil {
			return result, err
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
	return createExpandedProductionItemsTx(ctx, tx, r.schema, 0, roots)
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&status); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if status != "draft" {
		return app.ProductionPlanDetail{}, fmt.Errorf("仅草稿可刷新供应；已提交工单继续使用冻结快照")
	}
	items, err := loadProductionPlanItemsTx(ctx, tx, r.schema, id)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	adjustments, err := preservedPickingAdjustmentsTx(ctx, tx, r.schema, id, items)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET picking_version=1 WHERE id=$1`, r.schema), id); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT d.depends_on_plan_item_id FROM %[1]s.production_plan_item_dependencies d JOIN %[1]s.production_plan_items pi ON pi.id=d.depends_on_plan_item_id AND pi.production_plan_id=$1 WHERE d.production_plan_id=$1`, r.schema), id)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	children := []int64{}
	childSet := map[int64]bool{}
	for rows.Next() {
		var child int64
		if err = rows.Scan(&child); err != nil {
			rows.Close()
			return app.ProductionPlanDetail{}, err
		}
		children = append(children, child)
		childSet[child] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	roots := []app.ProductionPlanItem{}
	for _, item := range items {
		if !childSet[item.ID] {
			roots = append(roots, item)
		}
	}
	// Preserve final output identity, frozen BOM, order sources and root capacity splits.
	for _, table := range []string{"production_supply_allocations", "production_plan_item_dependencies", "production_plan_supply_gaps"} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_id=$1`, r.schema, table), id); err != nil {
			return app.ProductionPlanDetail{}, err
		}
	}
	for _, table := range []string{"production_plan_operation_splits", "production_plan_component_sources"} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_item_id=ANY($1::bigint[])`, r.schema, table), children); err != nil {
			return app.ProductionPlanDetail{}, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_items WHERE id=ANY($1::bigint[])`, r.schema), children); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	expanded, err := createExpandedProductionItemsTx(ctx, tx, r.schema, id, roots)
	if err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, id, expanded.Items); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = restorePickingAdjustmentsTx(ctx, tx, r.schema, id, expanded.Items, adjustments); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, operator, "production_plan", &id, "refresh_supply", nil, nil, nil, infra.AuditMeta{"replaced_upstream_items": children, "item_count": len(expanded.Items)}); err != nil {
		return app.ProductionPlanDetail{}, err
	}
	result, err := loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func (r Repository) MaterialReceiptTotals(ctx context.Context, id int64) (int64, int64, error) {
	var g, n int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(finished_g),0)::bigint,COALESCE(SUM(finished_units),0)::bigint FROM %s.production_material_receipts WHERE work_order_id=$1`, r.schema), id).Scan(&g, &n)
	return g, n, err
}
