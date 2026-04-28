package production

import (
	"context"
	"fmt"
	"math"
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
	yieldByProductID, err := r.loadProductYieldRateMap(ctx)
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
		if len(query.Selected) > 0 && !query.Selected[key] {
			continue
		}
		if row.GapG <= 0 {
			continue
		}
		yieldRate := normalizeYieldRate(yieldByProductID[row.ProductID])
		inputG := query.InputByKey[key]
		if inputG <= 0 {
			inputG = int64(math.Ceil(float64(row.GapG) / yieldRate))
		}
		plan := runningInventoryPlan(row.SpecG, row.GapG, inputG, yieldRate)
		run := ProduceRunRow{
			Product:      row.Product,
			ProductID:    row.ProductID,
			SpecG:        row.SpecG,
			NeedG:        row.GapG,
			InputG:       inputG,
			BomYieldRate: yieldRate,
			PlanUnits:    plan.Units,
			PlanLooseG:   plan.LooseG,
		}
		needs, err := currentMaterialNeedsTx(ctx, tx, r.schema, run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
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
		shortageG := need.DeductG - availableG - rawG
		if shortageG < 0 {
			shortageG = 0
		}
		purchaseSuggestionG := shortageG
		out = append(out, productionapp.MaterialPlanRow{
			MaterialID:          materialID,
			MaterialName:        need.MaterialName,
			Unit:                strings.TrimSpace(need.Unit),
			RequiredG:           need.DeductG,
			RequiredUnits:       need.DeductUnits,
			WIPG:                wipG,
			RawG:                rawG,
			ReservedG:           reservedG,
			ShortageG:           shortageG,
			PurchaseSuggestionG: purchaseSuggestionG,
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
	`, schema, schema), materialID, warehouse).Scan(&qtyG)
	if err != nil {
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") {
			return 0, nil
		}
		return 0, err
	}
	return qtyG, nil
}
