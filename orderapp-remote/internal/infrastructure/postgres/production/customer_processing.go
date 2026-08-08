package production

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CustomerProcessingTarget is the customer-visible output request. Inputs are
// deliberately absent: the authoritative production BOM resolves them.
type CustomerProcessingTarget struct {
	ProductID int64
	SpecG     int64
	Qty       int64
}

type CustomerProcessingNeed struct {
	MaterialID         int64
	MaterialName       string
	Unit               string
	RequiredG          int64
	RequiredUnits      int64
	ComponentType      string
	ComponentProductID int64
	ComponentSpecG     int64
}

type CustomerProcessingResolvedItem struct {
	LineNo             int
	ProductID          int64
	ParentProductID    int64
	ProductName        string
	SpecG              int64
	Qty                int64
	NeedG              int64
	InputG             int64
	BomVersionID       int64
	BomVersionNo       string
	BomSourceProductID int64
	BomInherited       bool
	MaterialSnapshot   string
	Materials          []CustomerProcessingNeed
}

// HasUsableCustomerProcessingBomTx applies the same strict BOM and process
// route resolution used when a customer processing request is previewed. It is
// intentionally exported so the customer catalog can omit targets that would
// fail immediately after selection without duplicating production rules.
func HasUsableCustomerProcessingBomTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (bool, error) {
	if productID <= 0 {
		return false, nil
	}
	var parentProductID int64
	var productName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT CASE WHEN COALESCE(parent_product_id,0)>0 THEN parent_product_id ELSE id END,
		       COALESCE(name,'')
		FROM %s.products
		WHERE id=$1 AND active=true
	`, schema), productID).Scan(&parentProductID, &productName); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if _, err := resolveProductionBomForDemandProductTx(ctx, tx, schema, productID, parentProductID, productName); err != nil {
		if isProductionBomConfigurationError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ResolveCustomerProcessingTargetsTx reuses the same published BOM resolution,
// loss conversion and material snapshot calculation as formal production plans.
func ResolveCustomerProcessingTargetsTx(ctx context.Context, tx pgx.Tx, schema string, targets []CustomerProcessingTarget) ([]CustomerProcessingResolvedItem, error) {
	out := make([]CustomerProcessingResolvedItem, 0, len(targets))
	for index, target := range targets {
		if target.ProductID <= 0 || target.SpecG <= 0 || target.Qty <= 0 {
			return nil, fmt.Errorf("invalid customer processing target")
		}
		if target.Qty > math.MaxInt64/target.SpecG {
			return nil, fmt.Errorf("target_qty invalid")
		}
		var parentProductID int64
		var productName string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT CASE WHEN COALESCE(parent_product_id,0)>0 THEN parent_product_id ELSE id END,
			       COALESCE(name,'')
			FROM %s.products
			WHERE id=$1 AND active=true
		`, schema), target.ProductID).Scan(&parentProductID, &productName); err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("target product unavailable")
			}
			return nil, err
		}
		bom, err := resolveProductionBomForDemandProductTx(ctx, tx, schema, target.ProductID, parentProductID, productName)
		if err != nil {
			return nil, err
		}
		needG := target.SpecG * target.Qty
		lossRate := productionPlanBomMaterialLossRate(bom)
		inputG := productionInputGFromBomMaterialLoss(needG, lossRate)
		plan := plannedFinishedInventoryAddition(target.SpecG, needG)
		run := ProduceRunRow{
			Product:      productName,
			ProductID:    target.ProductID,
			SpecG:        target.SpecG,
			NeedG:        needG,
			InputG:       inputG,
			BomYieldRate: 1,
			PlanUnits:    plan.Units,
			PlanLooseG:   plan.LooseG,
		}
		snapshot, err := buildMaterialSnapshotForBomVersionTx(
			ctx, tx, schema, run, bom.BomVersionID, lossRate > 0,
		)
		if err != nil {
			return nil, err
		}
		run.MaterialSnapshot = strings.TrimSpace(string(snapshot))
		needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
		if err != nil {
			return nil, err
		}
		if !ok || len(needs) == 0 {
			return nil, fmt.Errorf("production BOM version has no material lines: %s", productName)
		}
		aggregated := aggregateMaterialConsumptionNeeds(needs)
		materialRows := make([]CustomerProcessingNeed, 0, len(aggregated))
		for _, need := range aggregated {
			if need.MaterialID <= 0 || (need.DeductG <= 0 && need.DeductUnits <= 0) {
				continue
			}
			materialRows = append(materialRows, CustomerProcessingNeed{
				MaterialID:         need.MaterialID,
				MaterialName:       strings.TrimSpace(need.MaterialName),
				Unit:               strings.TrimSpace(need.Unit),
				RequiredG:          need.DeductG,
				RequiredUnits:      need.DeductUnits,
				ComponentType:      firstNonEmpty(strings.TrimSpace(need.ComponentType), "material"),
				ComponentProductID: need.ComponentProductID,
				ComponentSpecG:     need.ComponentSpecG,
			})
		}
		if len(materialRows) == 0 {
			return nil, fmt.Errorf("production BOM version has no material lines: %s", productName)
		}
		out = append(out, CustomerProcessingResolvedItem{
			LineNo: index + 1, ProductID: target.ProductID, ParentProductID: parentProductID,
			ProductName: productName, SpecG: target.SpecG, Qty: target.Qty, NeedG: needG, InputG: inputG,
			BomVersionID: bom.BomVersionID, BomVersionNo: bom.BomVersionNo,
			BomSourceProductID: bom.BomSourceProductID, BomInherited: bom.BomInherited,
			MaterialSnapshot: strings.TrimSpace(string(snapshot)), Materials: materialRows,
		})
	}
	return out, nil
}
