package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	stockdomain "orderapp/internal/domain/stock"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r Repository) MaterialPlan(ctx context.Context, query productionapp.MaterialPlanQuery) (productionapp.MaterialPlanResult, error) {
	rows, err := fetchUnproducedNeeds(ctx, r.pool, r.schema, query.From, query.To, query.CustomerID)
	if err != nil {
		return productionapp.MaterialPlanResult{}, err
	}
	rows, err = r.splitUnproducedNeedsByProductionPlan(ctx, rows)
	if err != nil {
		return productionapp.MaterialPlanResult{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return productionapp.MaterialPlanResult{}, err
	}
	defer tx.Rollback(ctx)

	byMaterial := map[int64]materialConsumptionNeed{}
	order := make([]int64, 0)
	for _, row := range rows {
		key := producePlanKey(row.ProductID, row.SpecG)
		demandKey := producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)
		if len(query.Selected) > 0 && !query.Selected[key] {
			continue
		}
		if len(query.IncludedDemandKeys) > 0 && !query.IncludedDemandKeys[demandKey] {
			continue
		}
		if row.DemandStatus != "unplanned" {
			continue
		}
		if query.SkipDemandKeys[demandKey] {
			continue
		}
		if row.GapG <= 0 {
			continue
		}
		bomVersionID := query.BomVersionByDemandKey[demandKey]
		bomLossRate := query.BomLossByDemandKey[demandKey]
		if bomVersionID <= 0 {
			resolved, resolveErr := resolveProductionBomForDemandProductPreviewTx(
				ctx,
				tx,
				r.schema,
				row.ProductID,
				row.ParentProductID,
				row.Product,
			)
			switch {
			case resolveErr == nil:
				bomVersionID = resolved.BomVersionID
				bomLossRate = productionPlanBomMaterialLossRate(resolved)
			case isProductionBomNotConfiguredError(resolveErr):
				// Legacy product_bom remains the compatibility path when no
				// formal production BOM exists.
			default:
				return productionapp.MaterialPlanResult{}, resolveErr
			}
		}

		inputG, hasResolvedInput := query.InputByDemandKey[demandKey]
		if !hasResolvedInput {
			inputG = query.InputByKey[key]
		}
		if inputG <= 0 && bomVersionID > 0 {
			inputG = productionInputGFromBomMaterialLoss(row.GapG, bomLossRate)
		}
		if inputG <= 0 {
			inputG = row.GapG
		}
		plan := plannedFinishedInventoryAddition(row.SpecG, row.GapG)
		run := ProduceRunRow{
			Product:      row.Product,
			ProductID:    row.ProductID,
			SpecG:        row.SpecG,
			NeedG:        row.GapG,
			InputG:       inputG,
			BomYieldRate: 1,
			PlanUnits:    plan.Units,
			PlanLooseG:   plan.LooseG,
		}
		var needs []materialConsumptionNeed
		if bomVersionID > 0 {
			snapshot, snapshotErr := buildMaterialSnapshotForBomVersionTx(
				ctx,
				tx,
				r.schema,
				run,
				bomVersionID,
				bomLossRate > 0,
			)
			if snapshotErr != nil {
				return productionapp.MaterialPlanResult{}, snapshotErr
			}
			run.MaterialSnapshot = string(snapshot)
			var ok bool
			needs, ok, err = materialSnapshotNeedsTx(run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
			if err != nil {
				return productionapp.MaterialPlanResult{}, err
			}
			if !ok {
				return productionapp.MaterialPlanResult{}, fmt.Errorf("production BOM version has no material lines: %s", row.Product)
			}
		} else {
			needs, err = currentMaterialNeedsTx(ctx, tx, r.schema, run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
		}
		if err != nil {
			return productionapp.MaterialPlanResult{}, err
		}
		for _, need := range aggregateMaterialConsumptionNeeds(needs) {
			if need.MaterialID <= 0 {
				continue
			}
			cur, ok := byMaterial[need.MaterialID]
			if !ok {
				cur = materialConsumptionNeed{
					MaterialID:   need.MaterialID,
					MaterialName: need.MaterialName,
					Unit:         need.Unit,
				}
				order = append(order, need.MaterialID)
			}
			cur.RequiredAdd(need)
			byMaterial[need.MaterialID] = cur
		}
	}

	out := make([]productionapp.MaterialPlanRow, 0, len(order))
	for _, materialID := range order {
		need := byMaterial[materialID]
		wipG, err := materialWarehouseGTx(ctx, tx, r.schema, materialID, stockdomain.WarehouseWIP)
		if err != nil {
			return productionapp.MaterialPlanResult{}, err
		}
		rawG, err := materialWarehouseGTx(ctx, tx, r.schema, materialID, stockdomain.WarehouseRawMaterials)
		if err != nil {
			return productionapp.MaterialPlanResult{}, err
		}
		reservedG, err := reservedWIPGForMaterialTx(ctx, tx, r.schema, materialID)
		if err != nil {
			return productionapp.MaterialPlanResult{}, err
		}
		availableG := wipG - reservedG
		if availableG < 0 {
			availableG = 0
		}
		wipTransferSuggestionG := need.DeductG - availableG
		if wipTransferSuggestionG < 0 {
			wipTransferSuggestionG = 0
		}
		if wipTransferSuggestionG > rawG {
			wipTransferSuggestionG = rawG
		}
		shortageG := need.DeductG - availableG - rawG
		if shortageG < 0 {
			shortageG = 0
		}
		purchaseSuggestionG := shortageG
		out = append(out, productionapp.MaterialPlanRow{
			MaterialID:             materialID,
			MaterialName:           need.MaterialName,
			Unit:                   strings.TrimSpace(need.Unit),
			RequiredG:              need.DeductG,
			RequiredUnits:          need.DeductUnits,
			WIPG:                   wipG,
			AvailableG:             availableG,
			RawG:                   rawG,
			ReservedG:              reservedG,
			WIPTransferSuggestionG: wipTransferSuggestionG,
			ShortageG:              shortageG,
			PurchaseSuggestionG:    purchaseSuggestionG,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.MaterialPlanResult{}, err
	}
	return productionapp.MaterialPlanResult{Rows: out}, nil
}

func (n *materialConsumptionNeed) RequiredAdd(other materialConsumptionNeed) {
	if n.MaterialName == "" {
		n.MaterialName = other.MaterialName
	}
	if n.Unit == "" {
		n.Unit = other.Unit
	}
	n.Qty += other.Qty
	n.DeductG += other.DeductG
	n.DeductUnits += other.DeductUnits
}

func materialWarehouseGTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, warehouse string) (int64, error) {
	var qtyG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(l.qty_g),0)::bigint
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1
		  AND l.warehouse=$2
		  AND l.qty_g > 0
		  AND b.status='active'
		  AND b.remaining_g > 0
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
	`, schema, schema), materialID, warehouse).Scan(&qtyG)
	if err != nil {
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") || strings.Contains(err.Error(), "quality_status") {
			return 0, nil
		}
		return 0, err
	}
	return qtyG, nil
}
