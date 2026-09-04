package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	productiondomain "orderapp/internal/domain/production"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type multilevelRootShortage struct {
	planItemID            int64
	componentType         string
	componentID           int64
	componentBomSpecID    int64
	componentBomVariantID int64
	componentSpecG        int64
	requiredG             int64
	requiredUnits         int64
}

type manufacturingOutputBOMPlanBasis struct {
	BOMID            int64
	VersionID        int64
	VersionNo        string
	OutputType       string
	OutputID         int64
	BomSpecID        int64
	BomVariantID     int64
	OutputName       string
	InventoryUnit    string
	OutputSpecG      int64
	OutputQty        float64
	OutputUnit       string
	ProcessRouteID   int64
	ProcessRouteName string
	Snapshot         string
}

func createMultilevelProductionPlanItemsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, roots []productionapp.ProductionPlanItem) error {
	if len(roots) == 0 {
		return nil
	}
	for index := range roots {
		root := &roots[index]
		root.OutputType = "product"
		root.OutputProductID = root.ProductID
		root.OutputName = root.ProductName
		root.OutputQty = root.PlannedInventoryQty
		root.OutputUnit = root.InventoryUnit
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_plan_items
			SET output_type='product',output_product_id=product_id,output_material_id=0,
			    output_name=product_name,output_qty=planned_inventory_qty,output_unit=inventory_unit
			WHERE id=$1
		`, schema), root.ID); err != nil {
			return err
		}
	}

	type rootNeed struct {
		item productionapp.ProductionPlanItem
		need materialConsumptionNeed
	}
	rootNeeds := make([]rootNeed, 0)
	for _, item := range roots {
		plan := plannedFinishedInventoryAddition(item.SpecG, item.PlannedOutputG)
		if item.SalesSpecCount > 0 {
			plan.Units = int64(math.Ceil(item.SalesSpecCount))
			if item.SpecG > 0 {
				plan.LooseG = nonnegativeQuantity(item.PlannedOutputG - plan.Units*item.SpecG)
			}
		}
		run := ProduceRunRow{
			Product: item.ProductName, ProductID: item.ProductID, SpecG: item.SpecG,
			NeedG: item.PlannedOutputG, InputG: item.PlannedG,
			PlanUnits: plan.Units, PlanLooseG: plan.LooseG, MaterialSnapshot: item.MaterialSnapshot,
		}
		needs, ok, err := materialSnapshotNeedsTx(run, plan)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
			rootNeeds = append(rootNeeds, rootNeed{item: item, need: need})
		}
	}
	if len(rootNeeds) == 0 {
		return nil
	}

	rootComponents := make([]materialConsumptionNeed, 0, len(rootNeeds))
	for _, rootNeed := range rootNeeds {
		rootComponents = append(rootComponents, rootNeed.need)
	}
	bases, domainBOMs, componentSpecs, err := loadDefaultManufacturingOutputBOMsForPlanningTx(ctx, tx, schema, rootComponents)
	if err != nil {
		return err
	}
	for _, rootNeed := range rootNeeds {
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(rootNeed.need)
		componentBomSpecID, _ := manufacturingNeedBOMSpecIdentity(rootNeed.need)
		if componentType == "product" {
			componentSpecs[manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)] = componentSpecG
		}
	}
	ownerCustomerID := int64(0)
	for _, root := range roots {
		if root.CustomerID > 0 {
			ownerCustomerID = root.CustomerID
			break
		}
	}
	domainAvailable, err := loadManufacturingPlanAvailabilityTx(ctx, tx, schema, domainBOMs, componentSpecs, rootComponents, ownerCustomerID)
	if err != nil {
		return err
	}
	for _, rootNeed := range rootNeeds {
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(rootNeed.need)
		componentBomSpecID, _ := manufacturingNeedBOMSpecIdentity(rootNeed.need)
		key := manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)
		if _, exists := domainAvailable[key]; exists {
			continue
		}
		unit := rootNeed.need.Unit
		if componentType == "product" {
			if basis, ok := bases[key]; ok {
				unit = basis.InventoryUnit
			} else {
				unit = "g"
			}
			availableG, availableUnits, availableErr := finishedProductAvailableForPlanningIdentityTx(ctx, tx, schema, componentID, componentBomSpecID, componentSpecG, 0)
			if availableErr != nil {
				return availableErr
			}
			domainAvailable[key] = manufacturingQtyFromCanonical(availableG, availableUnits, unit)
			continue
		}
		coverage, coverageErr := workOrderWIPCoverageForNeedsTx(ctx, tx, schema, 0, []materialConsumptionNeed{rootNeed.need})
		if coverageErr != nil {
			return coverageErr
		}
		if len(coverage) > 0 {
			domainAvailable[key] = manufacturingQtyFromCanonical(coverage[0].AvailableG, coverage[0].AvailableUnits, unit)
		}
	}
	demands := make([]productiondomain.ManufacturingDemand, 0)
	shortages := make([]multilevelRootShortage, 0)
	shortageByItem := map[string]productiondomain.ManufacturingDemand{}
	for _, rootNeed := range rootNeeds {
		need := rootNeed.need
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
		componentBomSpecID, componentBomVariantID := manufacturingNeedBOMSpecIdentity(need)
		key := manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)
		unit := strings.TrimSpace(need.Unit)
		if componentType == "product" {
			if basis, ok := bases[key]; ok {
				unit = basis.InventoryUnit
			} else {
				unit = "g"
			}
		}
		requiredG, requiredUnits := manufacturingNeedCanonicalQuantities(need)
		requiredQty := manufacturingQtyFromCanonical(requiredG, requiredUnits, unit)
		coveredQty := math.Min(requiredQty, math.Max(0, domainAvailable[key]))
		shortageQty := math.Max(0, requiredQty-coveredQty)
		domainAvailable[key] = math.Max(0, domainAvailable[key]-coveredQty)
		shortageG, shortageUnits := canonicalFromManufacturingQty(shortageQty, unit)
		if shortageG <= 0 && shortageUnits <= 0 {
			continue
		}
		shortages = append(shortages, multilevelRootShortage{
			planItemID: rootNeed.item.ID, componentType: componentType, componentID: componentID,
			componentBomSpecID: componentBomSpecID, componentBomVariantID: componentBomVariantID, componentSpecG: componentSpecG,
			requiredG: shortageG, requiredUnits: shortageUnits,
		})
		basis, ok := bases[key]
		if !ok {
			if err := insertProductionPlanSupplyGapTx(ctx, tx, schema, planID, rootNeed.item.ID, componentType, componentID, need.MaterialName, shortageG, shortageUnits, "no_default_"+componentType+"_bom"); err != nil {
				return err
			}
			continue
		}
		qty := manufacturingQtyFromCanonical(shortageG, shortageUnits, basis.InventoryUnit)
		if qty <= 0 {
			continue
		}
		demand := shortageByItem[key]
		if demand.Item.ID == 0 {
			demand = productiondomain.ManufacturingDemand{
				Item:            productiondomain.ManufacturingItemRef{Type: basis.OutputType, ID: basis.OutputID, BomSpecID: basis.BomSpecID, BomVariantID: basis.BomVariantID, Name: basis.OutputName, Unit: basis.InventoryUnit},
				TargetWarehouse: defaultManufacturingTargetWarehouse(basis.OutputType),
			}
		}
		demand.Qty += qty
		shortageByItem[key] = demand
	}
	itemKeys := make([]string, 0, len(shortageByItem))
	for key := range shortageByItem {
		itemKeys = append(itemKeys, key)
	}
	sort.Strings(itemKeys)
	for _, key := range itemKeys {
		demands = append(demands, shortageByItem[key])
	}
	if len(demands) == 0 {
		return nil
	}

	plan, err := productiondomain.BuildMultilevelManufacturingPlan(demands, domainBOMs, domainAvailable)
	if err != nil {
		return err
	}
	itemsByKey := map[string]productionapp.ProductionPlanItem{}
	for _, node := range plan.Nodes {
		if node.Action != productiondomain.ManufacturingSupplyManufacture || node.ShortageQty <= 0 {
			continue
		}
		basis, ok := bases[node.Item.Key()]
		if !ok {
			return fmt.Errorf("default manufacturing BOM snapshot unavailable: %s", node.Item.Name)
		}
		item, err := insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, planID, basis, node.ShortageQty, roots)
		if err != nil {
			return err
		}
		itemsByKey[node.Item.Key()] = item
	}

	for _, shortage := range shortages {
		upstream, ok := itemsByKey[manufacturingItemIdentityKey(shortage.componentType, shortage.componentID, shortage.componentBomSpecID)]
		if !ok {
			continue
		}
		if err := insertProductionPlanItemDependencyTx(ctx, tx, schema, planID, shortage.planItemID, upstream.ID,
			shortage.componentType, shortage.componentID, shortage.componentBomSpecID, shortage.componentBomVariantID,
			shortage.componentSpecG, shortage.requiredG, shortage.requiredUnits); err != nil {
			return err
		}
	}
	for _, edge := range plan.Edges {
		consumer, consumerOK := itemsByKey[edge.ConsumerKey]
		supplier, supplierOK := itemsByKey[edge.SupplierKey]
		if !consumerOK || !supplierOK {
			continue
		}
		basis := bases[edge.SupplierKey]
		if edge.ShortageQty <= 0 {
			continue
		}
		requiredG, requiredUnits := canonicalFromManufacturingQty(edge.ShortageQty, basis.InventoryUnit)
		if err := insertProductionPlanItemDependencyTx(ctx, tx, schema, planID, consumer.ID, supplier.ID,
			basis.OutputType, basis.OutputID, basis.BomSpecID, basis.BomVariantID,
			componentSpecs[edge.SupplierKey], requiredG, requiredUnits); err != nil {
			return err
		}
	}
	type plannedSupplyGap struct {
		consumerItemID int64
		outputType     string
		outputID       int64
		outputName     string
		requiredG      int64
		requiredUnits  int64
	}
	purchaseNodes := map[string]productiondomain.ManufacturingPlanNode{}
	for _, node := range plan.Nodes {
		if node.Blocking && node.Action == productiondomain.ManufacturingSupplyPurchase && node.ShortageQty > 0 {
			purchaseNodes[node.Item.Key()] = node
		}
	}
	gapsByConsumer := map[string]plannedSupplyGap{}
	matchedPurchaseNodes := map[string]bool{}
	for _, edge := range plan.Edges {
		node, isPurchase := purchaseNodes[edge.SupplierKey]
		consumer, hasConsumer := itemsByKey[edge.ConsumerKey]
		if !isPurchase || !hasConsumer || edge.ShortageQty <= 0 {
			continue
		}
		basis, hasBasis := bases[node.Item.Key()]
		unit := node.Item.Unit
		outputType := node.Item.Type
		outputID := node.Item.ID
		outputName := node.Item.Name
		if hasBasis {
			unit = basis.InventoryUnit
			outputType = basis.OutputType
			outputID = basis.OutputID
			outputName = basis.OutputName
		}
		requiredG, requiredUnits := canonicalFromManufacturingQty(edge.ShortageQty, unit)
		key := fmt.Sprintf("%d:%s:%d", consumer.ID, outputType, outputID)
		gap := gapsByConsumer[key]
		gap.consumerItemID = consumer.ID
		gap.outputType = outputType
		gap.outputID = outputID
		gap.outputName = outputName
		gap.requiredG += requiredG
		gap.requiredUnits += requiredUnits
		gapsByConsumer[key] = gap
		matchedPurchaseNodes[node.Item.Key()] = true
	}
	for key, node := range purchaseNodes {
		if matchedPurchaseNodes[key] {
			continue
		}
		basis, hasBasis := bases[key]
		unit := node.Item.Unit
		outputType := node.Item.Type
		outputID := node.Item.ID
		outputName := node.Item.Name
		if hasBasis {
			unit = basis.InventoryUnit
			outputType = basis.OutputType
			outputID = basis.OutputID
			outputName = basis.OutputName
		}
		requiredG, requiredUnits := canonicalFromManufacturingQty(node.ShortageQty, unit)
		gapsByConsumer[fmt.Sprintf("0:%s:%d", outputType, outputID)] = plannedSupplyGap{
			outputType: outputType, outputID: outputID, outputName: outputName,
			requiredG: requiredG, requiredUnits: requiredUnits,
		}
	}
	gapKeys := make([]string, 0, len(gapsByConsumer))
	for key := range gapsByConsumer {
		gapKeys = append(gapKeys, key)
	}
	sort.Strings(gapKeys)
	for _, key := range gapKeys {
		gap := gapsByConsumer[key]
		if err := insertProductionPlanSupplyGapTx(ctx, tx, schema, planID, gap.consumerItemID, gap.outputType, gap.outputID, gap.outputName, gap.requiredG, gap.requiredUnits, "no_default_"+gap.outputType+"_bom"); err != nil {
			return err
		}
	}
	return nil
}

func loadManufacturingPlanAvailabilityTx(ctx context.Context, tx pgx.Tx, schema string, boms []productiondomain.ManufacturingBOM, componentSpecs map[string]int64, rootComponents []materialConsumptionNeed, ownerCustomerID int64) (map[string]float64, error) {
	refs := map[string]productiondomain.ManufacturingItemRef{}
	for _, need := range rootComponents {
		if need.MaterialID <= 0 || need.Source == "finished_product" || need.ComponentType == "finished_product" {
			continue
		}
		refs[manufacturingMaterialKey(need.MaterialID)] = productiondomain.ManufacturingItemRef{
			Type: "material", ID: need.MaterialID, Name: need.MaterialName, Unit: need.Unit,
		}
	}
	for _, bom := range boms {
		refs[bom.Output.Key()] = bom.Output
		for _, component := range bom.Components {
			refs[component.Item.Key()] = component.Item
		}
	}
	keys := make([]string, 0, len(refs))
	for key := range refs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := map[string]float64{}
	for _, key := range keys {
		ref := refs[key]
		if strings.EqualFold(ref.Type, "product") {
			availableG, availableUnits, err := finishedProductAvailableForPlanningIdentityTx(ctx, tx, schema, ref.ID, ref.BomSpecID, componentSpecs[key], 0)
			if err != nil {
				return nil, err
			}
			out[key] = manufacturingQtyFromCanonical(availableG, availableUnits, ref.Unit)
			continue
		}
		availableG, availableUnits, err := materialAvailableForPlanningOwnerTx(ctx, tx, schema, ref.ID, ownerCustomerID)
		if err != nil {
			return nil, err
		}
		out[key] = manufacturingQtyFromCanonical(availableG, availableUnits, ref.Unit)
	}
	return out, nil
}

// Planning availability includes both raw and WIP locations. The owner is
// frozen by the production demand: customer supplied demand uses that
// customer's batches only, while ordinary factory demand uses owner 0.
func materialAvailableForPlanningOwnerTx(ctx context.Context, tx pgx.Tx, schema string, materialID, ownerCustomerID int64) (int64, int64, error) {
	var availableG, availableUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(l.qty_g),0)::bigint, COALESCE(SUM(l.qty_units),0)::bigint
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1
		  AND (l.qty_g>0 OR l.qty_units>0)
		  AND b.status='active'
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.owner_customer_id,0)=$2
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
	`, schema, schema), materialID, ownerCustomerID).Scan(&availableG, &availableUnits)
	if err != nil {
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") || strings.Contains(err.Error(), "quality_status") {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return availableG, availableUnits, nil
}

func manufacturingMaterialKey(materialID int64) string {
	return manufacturingItemKey("material", materialID)
}

func manufacturingItemKey(itemType string, itemID int64) string {
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(itemType)), itemID)
}

func manufacturingItemIdentityKey(itemType string, itemID, bomSpecID int64) string {
	if strings.EqualFold(strings.TrimSpace(itemType), "product") && bomSpecID > 0 {
		return fmt.Sprintf("product:%d:bom_spec:%d", itemID, bomSpecID)
	}
	return manufacturingItemKey(itemType, itemID)
}

func defaultManufacturingTargetWarehouse(itemType string) string {
	if strings.EqualFold(strings.TrimSpace(itemType), "product") {
		return "finished_goods"
	}
	return "wip"
}

func manufacturingNeedIdentity(need materialConsumptionNeed) (string, int64, int64) {
	if need.Source == "finished_product" || need.ComponentType == "finished_product" {
		productID := need.ComponentProductID
		if productID <= 0 {
			productID = need.MaterialID
		}
		return "product", productID, need.ComponentSpecG
	}
	return "material", need.MaterialID, 0
}

func manufacturingNeedBOMSpecIdentity(need materialConsumptionNeed) (int64, int64) {
	if need.Source == "finished_product" || need.ComponentType == "finished_product" {
		return need.ComponentBomSpecID, need.ComponentBomVariantID
	}
	return 0, 0
}

func manufacturingNeedCanonicalQuantities(need materialConsumptionNeed) (int64, int64) {
	componentType, _, componentSpecG := manufacturingNeedIdentity(need)
	if componentType != "product" {
		return need.DeductG, need.DeductUnits
	}
	consumeUnit := normalizeBomConsumeUnit(need.ConsumeUnit)
	if consumeUnit == "unit_per_bag" || consumeUnit == "unit_per_box" || consumeUnit == "unit" {
		units := nonnegativeQuantity(need.Qty)
		if componentSpecG > 0 {
			return units * componentSpecG, units
		}
		return 0, units
	}
	return need.DeductG, need.DeductUnits
}

func aggregateManufacturingConsumptionNeeds(needs []materialConsumptionNeed) []materialConsumptionNeed {
	byKey := map[string]materialConsumptionNeed{}
	order := make([]string, 0, len(needs))
	for _, need := range needs {
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
		if componentID <= 0 {
			continue
		}
		componentBomSpecID, _ := manufacturingNeedBOMSpecIdentity(need)
		key := fmt.Sprintf("%s:%d:%d:%d", componentType, componentID, componentBomSpecID, componentSpecG)
		row, ok := byKey[key]
		if !ok {
			row = need
			row.Qty = 0
			row.QtyDecimal = 0
			row.DeductG = 0
			row.DeductUnits = 0
			order = append(order, key)
		}
		row.Qty += need.Qty
		row.QtyDecimal += need.QtyDecimal
		row.DeductG += need.DeductG
		row.DeductUnits += need.DeductUnits
		byKey[key] = row
	}
	out := make([]materialConsumptionNeed, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

func finishedProductAvailableForPlanningTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG, excludedWorkOrderID int64) (int64, int64, error) {
	return finishedProductAvailableForPlanningIdentityTx(ctx, tx, schema, productID, 0, specG, excludedWorkOrderID)
}

func finishedProductAvailableForPlanningIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, specG, excludedWorkOrderID int64) (int64, int64, error) {
	hasBatches, err := schemaColumnExistsTx(ctx, tx, schema, "stock_batches", "id")
	if err != nil {
		return 0, 0, err
	}
	if !hasBatches {
		var units, looseG int64
		hasBomSpec, columnErr := schemaColumnExistsTx(ctx, tx, schema, "finished_inventory", "bom_spec_id")
		if columnErr != nil {
			return 0, 0, columnErr
		}
		if hasBomSpec {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(onhand_units),0)::bigint,COALESCE(SUM(onhand_loose_g),0)::bigint
				FROM %s.finished_inventory
				WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND warehouse='finished_goods'
			`, schema), productID, bomSpecID, specG).Scan(&units, &looseG)
		} else {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(onhand_units),0)::bigint,COALESCE(SUM(onhand_loose_g),0)::bigint
				FROM %s.finished_inventory
				WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods'
			`, schema), productID, specG).Scan(&units, &looseG)
		}
		if err != nil {
			return 0, 0, err
		}
		return finishedComponentTotalG(specG, units, looseG), units, nil
	}
	hasCustomerReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "finished_stock_batch_id")
	if err != nil {
		return 0, 0, err
	}
	hasTypedWorkReservations, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "component_type")
	if err != nil {
		return 0, 0, err
	}
	customerJoin := `LEFT JOIN LATERAL (SELECT 0::bigint AS reserved_g,0::bigint AS reserved_units) customer_reserved ON true`
	if hasCustomerReservations {
		customerJoin = fmt.Sprintf(`
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
				FROM %s.customer_processing_material_reservations r
				WHERE r.finished_stock_batch_id=b.id AND r.status='reserved'
			) customer_reserved ON true`, schema)
	}
	workJoin := `LEFT JOIN LATERAL (SELECT 0::bigint AS reserved_g,0::bigint AS reserved_units) work_reserved ON true`
	if hasTypedWorkReservations {
		hasComponentBomSpec, componentErr := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "component_bom_spec_id")
		if componentErr != nil {
			return 0, 0, componentErr
		}
		bomSpecPredicate := ""
		if hasComponentBomSpec {
			bomSpecPredicate = " AND rb.component_bom_spec_id=$2"
		}
		workJoin = fmt.Sprintf(`
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
				FROM %s.work_order_material_reservation_batches rb
				JOIN %s.work_order_material_reservations r ON r.id=rb.reservation_id AND r.status='reserved'
				WHERE rb.component_type='product' AND rb.component_id=$1 %s AND rb.component_spec_g=$3
				  AND rb.stock_batch_id=b.id AND rb.status='reserved'
				  AND ($4::bigint=0 OR rb.work_order_id<>$4)
			) work_reserved ON true`, schema, schema, bomSpecPredicate)
	}
	var availableG, availableUnits int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,b.remaining_g-COALESCE(customer_reserved.reserved_g,0)-COALESCE(work_reserved.reserved_g,0))),0)::bigint,
		       COALESCE(SUM(GREATEST(0,b.remaining_units-COALESCE(customer_reserved.reserved_units,0)-COALESCE(work_reserved.reserved_units,0))),0)::bigint
		FROM %s.stock_batches b
		%s
		%s
		WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
	`, schema, customerJoin, workJoin), productID, bomSpecID, specG, excludedWorkOrderID).Scan(&availableG, &availableUnits)
	if err != nil {
		return 0, 0, err
	}
	hasTypedReservationRows, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "component_type")
	if err != nil {
		return 0, 0, err
	}
	if hasTypedReservationRows {
		hasComponentBomSpec, componentErr := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "component_bom_spec_id")
		if componentErr != nil {
			return 0, 0, componentErr
		}
		bomSpecPredicate := ""
		if hasComponentBomSpec {
			bomSpecPredicate = " AND r.component_bom_spec_id=$2"
		}
		var unboundG, unboundUnits int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
			FROM %s.work_order_material_reservations r
			LEFT JOIN %s.work_orders wo ON wo.id=r.work_order_id
			WHERE r.component_type='product' AND r.component_id=$1 %s AND r.component_spec_g=$3
			  AND r.status='reserved' AND ($4::bigint=0 OR r.work_order_id<>$4)
			  AND (wo.id IS NULL OR wo.status IN ('released','running','partially_completed','paused'))
			  AND NOT EXISTS (
				SELECT 1 FROM %s.work_order_material_reservation_batches rb
				WHERE rb.reservation_id=r.id AND rb.status='reserved'
			  )
		`, schema, schema, bomSpecPredicate, schema), productID, bomSpecID, specG, excludedWorkOrderID).Scan(&unboundG, &unboundUnits)
		if err != nil {
			return 0, 0, err
		}
		availableG = nonnegativeQuantity(availableG - unboundG)
		availableUnits = nonnegativeQuantity(availableUnits - unboundUnits)
	}
	availableG, availableUnits = normalizeCustomerProcessingFinishedQuantity(specG, availableG, availableUnits)
	return availableG, availableUnits, nil
}

func manufacturingQtyFromCanonical(qtyG, qtyUnits int64, unit string) float64 {
	if factor := productionWeightUnitGrams(unit); factor > 0 {
		return float64(qtyG) / factor
	}
	return float64(qtyUnits)
}

func canonicalFromManufacturingQty(qty float64, unit string) (int64, int64) {
	if qty <= 0 {
		return 0, 0
	}
	if factor := productionWeightUnitGrams(unit); factor > 0 {
		return int64(math.Ceil(qty * factor)), 0
	}
	return 0, int64(math.Ceil(qty))
}

func loadDefaultManufacturingOutputBOMsForPlanningTx(ctx context.Context, tx pgx.Tx, schema string, rootComponents []materialConsumptionNeed) (map[string]manufacturingOutputBOMPlanBasis, []productiondomain.ManufacturingBOM, map[string]int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,v.id,COALESCE(v.version_no,''),binding.output_type,binding.output_id,
		       COALESCE(spec.id,0),COALESCE(variant.id,0),
		       CASE WHEN binding.output_type='material' THEN COALESCE(m.name,'')
		            WHEN COALESCE(spec.id,0)>0 THEN btrim(COALESCE(p.name,'') || ' ' || COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''))
		            ELSE COALESCE(p.name,'') END,
		       CASE WHEN binding.output_type='material' THEN COALESCE(NULLIF(m.unit,''),'g') ELSE
		         COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),NULLIF(LOWER(p.unit_rule_override_json->>'inventory_unit'),''),
		           CASE LOWER(COALESCE(v.output_unit,''))
		             WHEN 'kg' THEN 'kg' WHEN 'lb' THEN 'lb' WHEN 'g' THEN 'g' ELSE 'unit' END)
		       END,
		       CASE WHEN binding.output_type='product' AND COALESCE(spec.id,0)=0 THEN CASE LOWER(COALESCE(p.net_content_unit,''))
		         WHEN 'kg' THEN CEIL(COALESCE(p.net_content_qty,0)*1000)::bigint
		         WHEN 'lb' THEN CEIL(COALESCE(p.net_content_qty,0)*453.59237)::bigint
		         WHEN 'g' THEN CEIL(COALESCE(p.net_content_qty,0))::bigint ELSE 0 END ELSE 0 END,
		       CASE WHEN COALESCE(spec.id,0)>0 THEN 1::float8 ELSE COALESCE(NULLIF(v.output_qty,0),1)::float8 END,
		       CASE WHEN COALESCE(spec.id,0)>0 THEN COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),'unit') ELSE COALESCE(NULLIF(v.output_unit,''),'unit') END,
		       COALESCE(NULLIF(variant.process_route_id,0),v.process_route_id,0),COALESCE(NULLIF(variant_route.name,''),pr.name,'')
		FROM %s.production_bom_output_bindings binding
		JOIN %s.production_boms b ON b.id=binding.bom_id
		JOIN %s.production_bom_versions v ON v.id=binding.bom_version_id AND v.bom_id=b.id
		LEFT JOIN %s.materials m ON binding.output_type='material' AND m.id=binding.output_id
		LEFT JOIN %s.products p ON binding.output_type='product' AND p.id=binding.output_id
		LEFT JOIN %s.production_bom_version_variants variant
		  ON binding.output_type='product' AND variant.version_id=v.id
		LEFT JOIN %s.production_bom_specs spec
		  ON spec.id=variant.bom_spec_id AND spec.bom_id=b.id
		LEFT JOIN %s.process_routes pr ON pr.id=v.process_route_id AND pr.status='active'
		LEFT JOIN %s.process_routes variant_route ON variant_route.id=variant.process_route_id AND variant_route.status='active'
		WHERE binding.output_type IN ('material','product') AND binding.is_default=true
		  AND b.output_type=binding.output_type
		  AND ((binding.output_type='material' AND b.output_material_id=binding.output_id)
		    OR (binding.output_type='product' AND b.output_product_id=binding.output_id))
		  AND COALESCE(NULLIF(b.status,''),'active')='active'
		  AND v.status='published'
		ORDER BY binding.output_type,binding.output_id
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	if err != nil {
		return nil, nil, nil, err
	}
	bases := map[string]manufacturingOutputBOMPlanBasis{}
	for rows.Next() {
		var basis manufacturingOutputBOMPlanBasis
		if err := rows.Scan(&basis.BOMID, &basis.VersionID, &basis.VersionNo, &basis.OutputType, &basis.OutputID,
			&basis.BomSpecID, &basis.BomVariantID, &basis.OutputName, &basis.InventoryUnit, &basis.OutputSpecG,
			&basis.OutputQty, &basis.OutputUnit, &basis.ProcessRouteID, &basis.ProcessRouteName); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		bases[manufacturingItemIdentityKey(basis.OutputType, basis.OutputID, basis.BomSpecID)] = basis
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, nil, err
	}
	rows.Close()
	requestedSet := map[string]bool{}
	for _, component := range rootComponents {
		componentType, componentID, _ := manufacturingNeedIdentity(component)
		componentBomSpecID, _ := manufacturingNeedBOMSpecIdentity(component)
		if componentID <= 0 {
			continue
		}
		requestedSet[manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)] = true
	}
	requestedKeys := make([]string, 0, len(rootComponents))
	for key := range requestedSet {
		requestedKeys = append(requestedKeys, key)
	}
	sort.Strings(requestedKeys)

	boms := make([]productiondomain.ManufacturingBOM, 0, len(requestedKeys))
	componentSpecs := map[string]int64{}
	visited := map[string]bool{}
	for len(requestedKeys) > 0 {
		key := requestedKeys[0]
		requestedKeys = requestedKeys[1:]
		if visited[key] {
			continue
		}
		visited[key] = true
		basis, exists := bases[key]
		if !exists {
			continue
		}
		snapshot, err := buildMaterialSnapshotForBomVersionVariantTx(ctx, tx, schema,
			ProduceRunRow{Product: basis.OutputName, ProductID: basis.OutputID, SpecG: basis.OutputSpecG},
			basis.VersionID, basis.BomVariantID, false)
		if err != nil {
			return nil, nil, nil, err
		}
		basis.Snapshot = string(snapshot)
		bases[key] = basis
		outputG, outputUnits := canonicalOutputBasis(basis.OutputQty, basis.OutputUnit)
		if basis.OutputType == "product" && outputG <= 0 && outputUnits > 0 && basis.OutputSpecG > 0 {
			outputG = outputUnits * basis.OutputSpecG
			outputUnits = 0
		}
		outputQty := manufacturingQtyFromCanonical(outputG, outputUnits, basis.InventoryUnit)
		if outputQty <= 0 {
			return nil, nil, nil, fmt.Errorf("BOM output unit incompatible with inventory unit: %s", basis.OutputName)
		}
		run := ProduceRunRow{Product: basis.OutputName, ProductID: basis.OutputID, SpecG: basis.OutputSpecG, NeedG: outputG, InputG: outputG, MaterialSnapshot: basis.Snapshot}
		needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: outputUnits, LooseG: outputG})
		if err != nil || !ok {
			if err == nil {
				err = fmt.Errorf("BOM has no frozen component snapshot: %s", basis.OutputName)
			}
			return nil, nil, nil, err
		}
		components := make([]productiondomain.ManufacturingBOMComponent, 0, len(needs))
		for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
			componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
			componentBomSpecID, componentBomVariantID := manufacturingNeedBOMSpecIdentity(need)
			unit := need.Unit
			if componentType == "product" {
				if componentBasis, ok := bases[manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)]; ok {
					unit = componentBasis.InventoryUnit
				} else {
					unit = firstNonEmpty(need.Unit, "g")
				}
			}
			requiredG, requiredUnits := manufacturingNeedCanonicalQuantities(need)
			qty := manufacturingQtyFromCanonical(requiredG, requiredUnits, unit)
			if qty <= 0 {
				continue
			}
			componentKey := manufacturingItemIdentityKey(componentType, componentID, componentBomSpecID)
			if componentType == "product" {
				componentSpecs[componentKey] = componentSpecG
			}
			if _, hasDefaultBOM := bases[componentKey]; hasDefaultBOM && !visited[componentKey] {
				requestedKeys = append(requestedKeys, componentKey)
			}
			components = append(components, productiondomain.ManufacturingBOMComponent{
				Item: productiondomain.ManufacturingItemRef{Type: componentType, ID: componentID, BomSpecID: componentBomSpecID, BomVariantID: componentBomVariantID, Name: need.MaterialName, Unit: unit},
				Qty:  qty,
			})
		}
		componentSpecs[key] = basis.OutputSpecG
		boms = append(boms, productiondomain.ManufacturingBOM{
			VersionID:  basis.VersionID,
			Output:     productiondomain.ManufacturingItemRef{Type: basis.OutputType, ID: basis.OutputID, BomSpecID: basis.BomSpecID, BomVariantID: basis.BomVariantID, Name: basis.OutputName, Unit: basis.InventoryUnit},
			OutputQty:  outputQty,
			Components: components,
		})
		sort.Strings(requestedKeys)
	}
	return bases, boms, componentSpecs, nil
}

func canonicalOutputBasis(qty float64, unit string) (int64, int64) {
	if factor := productionWeightUnitGrams(unit); factor > 0 {
		return int64(math.Ceil(qty * factor)), 0
	}
	return 0, int64(math.Ceil(qty))
}

func insertManufacturingOutputProductionPlanItemTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, basis manufacturingOutputBOMPlanBasis, outputQty float64, roots []productionapp.ProductionPlanItem) (productionapp.ProductionPlanItem, error) {
	outputG, outputUnits := canonicalFromManufacturingQty(outputQty, basis.InventoryUnit)
	run := ProduceRunRow{Product: basis.OutputName, ProductID: basis.OutputID, SpecG: basis.OutputSpecG, NeedG: outputG, InputG: outputG, MaterialSnapshot: basis.Snapshot}
	needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: outputUnits, LooseG: outputG})
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	if !ok {
		return productionapp.ProductionPlanItem{}, fmt.Errorf("BOM has no frozen component snapshot: %s", basis.OutputName)
	}
	plannedInputG := int64(0)
	for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
		plannedInputG += need.DeductG
	}
	productID := int64(0)
	if basis.OutputType == "product" {
		productID = basis.OutputID
	}
	processSnapshot, processJSON, err := loadProcessRouteSnapshotByIDTx(ctx, tx, schema, basis.ProcessRouteID, productID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	if processSnapshot == nil {
		return productionapp.ProductionPlanItem{}, fmt.Errorf("BOM version has no active process route: %s", basis.OutputName)
	}
	processSnapshot.ProductID = productID
	processSnapshot.ProductName = basis.OutputName
	processSnapshot.BomVersionID = basis.VersionID
	processSnapshot.BomVersionNo = basis.VersionNo
	processSnapshot.YieldRate = 1
	processSnapshot.RouteID = basis.ProcessRouteID
	processSnapshot.RouteName = basis.ProcessRouteName
	processJSON, err = json.Marshal(processSnapshot)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	orderSet := map[string]bool{}
	for _, root := range roots {
		for _, orderNo := range splitOrderNos(root.OrderNos) {
			orderSet[orderNo] = true
		}
	}
	orderNos := make([]string, 0, len(orderSet))
	for orderNo := range orderSet {
		orderNos = append(orderNos, orderNo)
	}
	sort.Strings(orderNos)
	targetWarehouse := defaultManufacturingTargetWarehouse(basis.OutputType)
	outputProductID, outputMaterialID := int64(0), int64(0)
	if basis.OutputType == "product" {
		outputProductID = basis.OutputID
	} else {
		outputMaterialID = basis.OutputID
	}
	salesSpecCount := float64(0)
	if basis.OutputType == "product" {
		if basis.BomSpecID > 0 {
			salesSpecCount = outputQty
		} else if basis.OutputSpecG > 0 && outputG > 0 {
			salesSpecCount = float64(outputG) / float64(basis.OutputSpecG)
		}
	}
	var item productionapp.ProductionPlanItem
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			production_plan_id,product_id,parent_product_id,bom_spec_id,bom_variant_id,product_name,spec_g,sales_spec_count,inventory_unit,planned_inventory_qty,
			planned_g,planned_output_g,gap_g,order_nos,bom_version_id,process_route_id,
			component_snapshot_json,process_route_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,
			target_warehouse,output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,created_at
		) VALUES($1,$2,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11,$12,$13,$14,$15::jsonb,$16::jsonb,'{}'::jsonb,'[]'::jsonb,$17,$18,$19,$20,$5,$9,$8,now())
		RETURNING id
	`, schema), planID, productID, basis.BomSpecID, basis.BomVariantID, basis.OutputName, basis.OutputSpecG,
		salesSpecCount, basis.InventoryUnit, outputQty, plannedInputG, outputG, strings.Join(orderNos, ","), basis.VersionID,
		basis.ProcessRouteID, basis.Snapshot, processJSON, targetWarehouse, basis.OutputType, outputProductID, outputMaterialID).Scan(&item.ID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	item.PlanID = planID
	item.OutputType = basis.OutputType
	item.OutputProductID = outputProductID
	item.OutputMaterialID = outputMaterialID
	item.OutputName = basis.OutputName
	item.OutputQty = outputQty
	item.OutputUnit = basis.InventoryUnit
	item.ProductID = productID
	item.ParentProductID = productID
	item.BomSpecID = basis.BomSpecID
	item.BomVariantID = basis.BomVariantID
	item.ProductName = basis.OutputName
	item.SpecG = basis.OutputSpecG
	item.SalesSpecCount = salesSpecCount
	item.InventoryUnit = basis.InventoryUnit
	item.PlannedInventoryQty = outputQty
	item.PlannedG = plannedInputG
	item.PlannedOutputG = outputG
	item.GapG = outputG
	item.OrderNos = strings.Join(orderNos, ",")
	item.BomVersionID = basis.VersionID
	item.ProcessRouteID = basis.ProcessRouteID
	item.MaterialSnapshot = basis.Snapshot
	item.ProcessSnapshotJSON = string(processJSON)
	item.ProductionConfigSnapshotJSON = "{}"
	item.CustomerProductSnapshotJSON = "[]"
	item.TargetWarehouse = targetWarehouse
	return item, nil
}

func insertProductionPlanItemDependencyTx(ctx context.Context, tx pgx.Tx, schema string, planID, itemID, dependsOnItemID int64,
	componentType string, componentID, componentBomSpecID, componentBomVariantID, componentSpecG, requiredG, requiredUnits int64) error {
	if itemID <= 0 || dependsOnItemID <= 0 || itemID == dependsOnItemID {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_item_dependencies(
			production_plan_id,production_plan_item_id,depends_on_plan_item_id,material_id,
			component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,
			required_g,required_units,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
		ON CONFLICT(production_plan_item_id,depends_on_plan_item_id,component_type,component_id,component_bom_spec_id,component_spec_g) DO UPDATE SET
			required_g=production_plan_item_dependencies.required_g+excluded.required_g,
			required_units=production_plan_item_dependencies.required_units+excluded.required_units
	`, schema), planID, itemID, dependsOnItemID, func() int64 {
		if componentType == "material" {
			return componentID
		}
		return 0
	}(), componentType, componentID, componentBomSpecID, componentBomVariantID, componentSpecG, requiredG, requiredUnits)
	return err
}

func insertProductionPlanSupplyGapTx(ctx context.Context, tx pgx.Tx, schema string, planID, planItemID int64, itemType string, itemID int64, itemName string, requiredG, requiredUnits int64, reason string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_supply_gaps(
			production_plan_id,production_plan_item_id,item_type,item_id,item_name,
			required_g,required_units,reason,status,created_at,updated_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'unresolved',now(),now())
	`, schema), planID, planItemID, itemType, itemID, strings.TrimSpace(itemName), requiredG, requiredUnits, reason)
	return err
}

func loadProductionPlanSupplyGapsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanSupplyGap, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,production_plan_item_id,item_type,item_id,item_name,
		       required_g,required_units,reason,status
		FROM %s.production_plan_supply_gaps
		WHERE production_plan_id=$1
		ORDER BY id
	`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionPlanSupplyGap, 0)
	for rows.Next() {
		var row productionapp.ProductionPlanSupplyGap
		if err := rows.Scan(&row.ID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.ItemType, &row.ItemID, &row.ItemName, &row.RequiredG, &row.RequiredUnits, &row.Reason, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
