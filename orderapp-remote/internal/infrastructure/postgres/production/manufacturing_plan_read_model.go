package production

import (
	"context"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	productiondomain "orderapp/internal/domain/production"
	"strings"

	"github.com/jackc/pgx/v5"
)

type frozenPlanDependency struct {
	consumerPlanItemID int64
	supplierPlanItemID int64
	componentType      string
	componentID        int64
	componentSpecG     int64
	requiredG          int64
	requiredUnits      int64
}

func loadProductionManufacturingPlanTx(ctx context.Context, tx pgx.Tx, schema string, items []productionapp.ProductionPlanItem, gaps []productionapp.ProductionPlanSupplyGap) (productionapp.ProductionManufacturingPlan, error) {
	plan := productionapp.ProductionManufacturingPlan{
		Nodes: []productionapp.ProductionManufacturingPlanNode{},
		Edges: []productionapp.ProductionManufacturingPlanEdge{},
	}
	if len(items) == 0 {
		return plan, nil
	}
	dependencies, err := loadFrozenPlanDependenciesTx(ctx, tx, schema, items[0].PlanID)
	if err != nil {
		return productionapp.ProductionManufacturingPlan{}, err
	}
	itemsByID := make(map[int64]productionapp.ProductionPlanItem, len(items))
	upstreamItemIDs := make(map[int64]bool, len(dependencies))
	dependenciesByConsumerComponent := map[string][]frozenPlanDependency{}
	for _, item := range items {
		itemsByID[item.ID] = item
	}
	for _, dependency := range dependencies {
		upstreamItemIDs[dependency.supplierPlanItemID] = true
		key := planConsumerComponentKey(dependency.consumerPlanItemID, dependency.componentType, dependency.componentID, dependency.componentSpecG)
		dependenciesByConsumerComponent[key] = append(dependenciesByConsumerComponent[key], dependency)
	}
	gapsByConsumerComponent := map[string][]productionapp.ProductionPlanSupplyGap{}
	for _, gap := range gaps {
		key := planConsumerComponentKey(gap.ProductionPlanItemID, gap.ItemType, gap.ItemID, 0)
		gapsByConsumerComponent[key] = append(gapsByConsumerComponent[key], gap)
		if gap.Status == "unresolved" {
			plan.Blocking = true
		}
	}

	nodeIndex := map[string]int{}
	visitedItems := map[int64]bool{}
	var addNode func(productionapp.ProductionManufacturingPlanNode)
	addNode = func(add productionapp.ProductionManufacturingPlanNode) {
		if index, ok := nodeIndex[add.Key]; ok {
			current := plan.Nodes[index]
			current.RequiredG += add.RequiredG
			current.RequiredUnits += add.RequiredUnits
			current.StockCoveredG += add.StockCoveredG
			current.StockCoveredUnits += add.StockCoveredUnits
			current.ShortageG += add.ShortageG
			current.ShortageUnits += add.ShortageUnits
			current.RequiredQty += add.RequiredQty
			current.StockCoveredQty += add.StockCoveredQty
			current.ShortageQty += add.ShortageQty
			if manufacturingPlanActionPriority(add.Action) > manufacturingPlanActionPriority(current.Action) {
				current.Action = add.Action
			}
			if current.PlanItemID <= 0 && add.PlanItemID > 0 {
				current.PlanItemID = add.PlanItemID
				current.BOMVersionID = add.BOMVersionID
				current.TargetWarehouse = add.TargetWarehouse
			}
			if current.ParentPlanItemID <= 0 && add.ParentPlanItemID > 0 {
				current.ParentPlanItemID = add.ParentPlanItemID
			}
			if add.Depth < current.Depth {
				current.Depth = add.Depth
			}
			current.Blocking = current.Blocking || add.Blocking
			plan.Nodes[index] = current
			return
		}
		nodeIndex[add.Key] = len(plan.Nodes)
		plan.Nodes = append(plan.Nodes, add)
	}

	var visitItem func(productionapp.ProductionPlanItem, int) error
	visitItem = func(item productionapp.ProductionPlanItem, depth int) error {
		if visitedItems[item.ID] {
			return nil
		}
		visitedItems[item.ID] = true
		finished := productionPlanItemFrozenOutput(item)
		run := ProduceRunRow{
			Product: item.OutputName, ProductID: item.ProductID, SpecG: item.SpecG,
			NeedG: item.PlannedOutputG, InputG: item.PlannedG,
			PlanUnits: finished.Units, PlanLooseG: finished.LooseG,
			MaterialSnapshot: item.MaterialSnapshot,
		}
		needs, ok, err := materialSnapshotNeedsTx(run, finished)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		consumerKey := productionPlanOutputKey(item)
		for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
			if need.MaterialID <= 0 || (need.DeductG <= 0 && need.DeductUnits <= 0) {
				continue
			}
			outputType, outputProductID, outputMaterialID := "material", int64(0), need.MaterialID
			outputID := need.MaterialID
			if need.Source == "finished_product" || need.ComponentType == "finished_product" {
				outputType = "product"
				outputProductID = need.ComponentProductID
				if outputProductID <= 0 {
					outputProductID = need.MaterialID
				}
				outputMaterialID = 0
				outputID = outputProductID
			}
			_, _, componentSpecG := manufacturingNeedIdentity(need)
			supplierKey := fmt.Sprintf("%s:%d", outputType, outputID)
			dependencyRows := dependenciesByConsumerComponent[planConsumerComponentKey(item.ID, outputType, outputID, componentSpecG)]
			var shortageG, shortageUnits, supplierPlanItemID int64
			for _, dependency := range dependencyRows {
				shortageG += dependency.requiredG
				shortageUnits += dependency.requiredUnits
				if supplierPlanItemID <= 0 {
					supplierPlanItemID = dependency.supplierPlanItemID
				}
			}
			gapRows := gapsByConsumerComponent[planConsumerComponentKey(item.ID, outputType, outputID, 0)]
			if len(gapRows) == 0 {
				gapRows = gapsByConsumerComponent[planConsumerComponentKey(0, outputType, outputID, 0)]
			}
			blocking := false
			for _, gap := range gapRows {
				if gap.Status != "unresolved" {
					continue
				}
				shortageG += gap.RequiredG
				shortageUnits += gap.RequiredUnits
				blocking = true
			}
			shortageG = minInt64(need.DeductG, shortageG)
			shortageUnits = minInt64(need.DeductUnits, shortageUnits)
			stockCoveredG := nonnegativeQuantity(need.DeductG - shortageG)
			stockCoveredUnits := nonnegativeQuantity(need.DeductUnits - shortageUnits)
			action := productiondomain.ManufacturingSupplyInventory
			var supplierItem productionapp.ProductionPlanItem
			if supplierPlanItemID > 0 {
				action = productiondomain.ManufacturingSupplyManufacture
				supplierItem = itemsByID[supplierPlanItemID]
			} else if blocking {
				action = productiondomain.ManufacturingSupplyPurchase
			}
			node := productionapp.ProductionManufacturingPlanNode{
				Key: supplierKey, PlanItemID: supplierPlanItemID, ParentPlanItemID: item.ID,
				OutputType: outputType, OutputProductID: outputProductID, OutputMaterialID: outputMaterialID,
				OutputName: need.MaterialName, OutputUnit: need.Unit,
				RequiredG: need.DeductG, RequiredUnits: need.DeductUnits,
				StockCoveredG: stockCoveredG, StockCoveredUnits: stockCoveredUnits,
				ShortageG: shortageG, ShortageUnits: shortageUnits,
				RequiredQty:     manufacturingQtyFromCanonical(need.DeductG, need.DeductUnits, need.Unit),
				StockCoveredQty: manufacturingQtyFromCanonical(stockCoveredG, stockCoveredUnits, need.Unit),
				ShortageQty:     manufacturingQtyFromCanonical(shortageG, shortageUnits, need.Unit),
				Action:          action, Blocking: blocking, Depth: depth + 1,
			}
			if supplierItem.ID > 0 {
				node.OutputName = firstNonEmpty(supplierItem.OutputName, node.OutputName)
				node.OutputUnit = firstNonEmpty(supplierItem.OutputUnit, node.OutputUnit)
				node.BOMVersionID = supplierItem.BomVersionID
				node.TargetWarehouse = supplierItem.TargetWarehouse
			}
			addNode(node)
			plan.Edges = append(plan.Edges, productionapp.ProductionManufacturingPlanEdge{
				ConsumerKey: consumerKey, SupplierKey: supplierKey,
				ConsumerPlanItemID: item.ID, SupplierPlanItemID: supplierPlanItemID,
				RequiredG: need.DeductG, RequiredUnits: need.DeductUnits,
				RequiredQty: manufacturingQtyFromCanonical(need.DeductG, need.DeductUnits, need.Unit),
			})
			if supplierItem.ID > 0 {
				if err := visitItem(supplierItem, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	roots := make([]productionapp.ProductionPlanItem, 0)
	for _, item := range items {
		if !upstreamItemIDs[item.ID] {
			roots = append(roots, item)
		}
	}
	if len(roots) == 0 {
		roots = items
	}
	for _, root := range roots {
		rootG, rootUnits := productionPlanItemCanonicalOutput(root)
		rootType := firstNonEmpty(strings.TrimSpace(root.OutputType), "product")
		rootNode := productionapp.ProductionManufacturingPlanNode{
			Key: productionPlanOutputKey(root), PlanItemID: root.ID,
			OutputType: rootType, OutputProductID: root.OutputProductID, OutputMaterialID: root.OutputMaterialID,
			OutputName: firstNonEmpty(root.OutputName, root.ProductName), OutputUnit: root.OutputUnit,
			RequiredG: rootG, RequiredUnits: rootUnits, ShortageG: rootG, ShortageUnits: rootUnits,
			RequiredQty: root.OutputQty, ShortageQty: root.OutputQty,
			Action: productiondomain.ManufacturingSupplyManufacture, Depth: 0,
			BOMVersionID: root.BomVersionID, TargetWarehouse: root.TargetWarehouse,
		}
		addNode(rootNode)
		if err := visitItem(root, 0); err != nil {
			return productionapp.ProductionManufacturingPlan{}, err
		}
	}
	return plan, nil
}

func loadFrozenPlanDependenciesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]frozenPlanDependency, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT production_plan_item_id,depends_on_plan_item_id,component_type,component_id,component_spec_g,required_g,required_units
		FROM %s.production_plan_item_dependencies
		WHERE production_plan_id=$1 ORDER BY id
	`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]frozenPlanDependency, 0)
	for rows.Next() {
		var row frozenPlanDependency
		if err := rows.Scan(&row.consumerPlanItemID, &row.supplierPlanItemID, &row.componentType, &row.componentID, &row.componentSpecG, &row.requiredG, &row.requiredUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func productionPlanItemFrozenOutput(item productionapp.ProductionPlanItem) InvQty {
	if strings.EqualFold(strings.TrimSpace(item.OutputType), "material") {
		g, units := canonicalFromManufacturingQty(item.OutputQty, item.OutputUnit)
		return InvQty{LooseG: g, Units: units}
	}
	output := plannedFinishedInventoryAddition(item.SpecG, item.PlannedOutputG)
	if item.SalesSpecCount > 0 {
		output.Units = int64(math.Ceil(item.SalesSpecCount))
		if item.SpecG > 0 {
			output.LooseG = nonnegativeQuantity(item.PlannedOutputG - output.Units*item.SpecG)
		}
	}
	return output
}

func productionPlanItemCanonicalOutput(item productionapp.ProductionPlanItem) (int64, int64) {
	if strings.EqualFold(strings.TrimSpace(item.OutputType), "material") {
		return canonicalFromManufacturingQty(item.OutputQty, item.OutputUnit)
	}
	output := productionPlanItemFrozenOutput(item)
	return finishedTotalG(item.SpecG, output.Units, output.LooseG), 0
}

func productionPlanOutputKey(item productionapp.ProductionPlanItem) string {
	outputType := firstNonEmpty(strings.ToLower(strings.TrimSpace(item.OutputType)), "product")
	outputID := item.OutputProductID
	if outputType == "material" {
		outputID = item.OutputMaterialID
	} else if outputID <= 0 {
		outputID = item.ProductID
	}
	return fmt.Sprintf("%s:%d", outputType, outputID)
}

func planConsumerComponentKey(planItemID int64, componentType string, componentID, componentSpecG int64) string {
	return fmt.Sprintf("%d:%s:%d:%d", planItemID, strings.ToLower(strings.TrimSpace(componentType)), componentID, componentSpecG)
}

func manufacturingPlanActionPriority(action string) int {
	switch action {
	case productiondomain.ManufacturingSupplyPurchase:
		return 3
	case productiondomain.ManufacturingSupplyManufacture:
		return 2
	default:
		return 1
	}
}
