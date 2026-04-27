package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	stockdomain "orderapp/internal/domain/stock"
	"strings"

	"github.com/jackc/pgx/v5"
)

type materialConsumptionNeed struct {
	MaterialID   int64
	MaterialName string
	Unit         string
	Qty          int64
	DeductG      int64
	DeductUnits  int64
}

type materialConsumptionSummaryItem struct {
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
	Unit         string `json:"unit"`
	DeductG      int64  `json:"deduct_g"`
	DeductUnits  int64  `json:"deduct_units"`
	BatchCode    string `json:"batch_code,omitempty"`
}

func isWeightMaterialUnit(unit string) bool {
	unit = strings.ToLower(strings.TrimSpace(unit))
	return unit == "g" || unit == "kg" || unit == "克" || unit == "千克"
}

func materialNeedToDeduct(unit string, qty int64) (deductG int64, deductUnits int64) {
	if qty <= 0 {
		return 0, 0
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "kg", "千克":
		return qty * 1000, 0
	case "g", "克":
		return qty, 0
	default:
		return 0, qty
	}
}

func calcRunningItemMaterialNeedsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finished InvQty) ([]materialConsumptionNeed, error) {
	if r.ProductID <= 0 || r.SpecG <= 0 || r.NeedG <= 0 {
		return nil, nil
	}
	q := fmt.Sprintf(`
		SELECT bi.material_id,
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0),
		       COALESCE(p.roast_level,''),
		       COALESCE(pb.yield_rate,0)
		FROM %s.product_bom_items bi
		LEFT JOIN %s.products p ON p.id=bi.product_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=bi.product_id
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		WHERE bi.product_id=$1
		ORDER BY bi.id
	`, schema, schema, schema, schema)
	rows, err := tx.Query(ctx, q, r.ProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bomRow struct {
		materialID int64
		name       string
		unit       string
		ratio      float64
		roastLevel string
		yieldRate  float64
	}
	bomRows := make([]bomRow, 0)
	for rows.Next() {
		var x bomRow
		if err := rows.Scan(&x.materialID, &x.name, &x.unit, &x.ratio, &x.roastLevel, &x.yieldRate); err != nil {
			return nil, err
		}
		x.ratio = bomdomain.NormalizeRatioPct(x.ratio)
		if x.materialID <= 0 || strings.TrimSpace(x.name) == "" || x.ratio <= 0 {
			continue
		}
		bomRows = append(bomRows, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(bomRows) == 0 {
		return nil, fmt.Errorf("product BOM not configured: %s", r.Product)
	}

	yield := catalogdomain.ResolveYieldRate(bomRows[0].roastLevel, bomRows[0].yieldRate)
	rawG := r.InputG
	if rawG <= 0 {
		rawG = int64(math.Ceil(float64(r.NeedG) / yield))
	}
	packedUnits := finished.Units
	if packedUnits < 0 {
		packedUnits = 0
	}

	needs := make([]materialConsumptionNeed, 0, len(bomRows)+1)
	for _, bi := range bomRows {
		unit := strings.TrimSpace(bi.unit)
		if unit == "" {
			unit = "g"
		}
		qty := int64(0)
		if isWeightMaterialUnit(unit) {
			if strings.EqualFold(unit, "kg") || unit == "千克" {
				qty = int64(math.Ceil((float64(rawG) * bi.ratio / 100.0) / 1000.0))
			} else {
				qty = int64(math.Ceil(float64(rawG) * bi.ratio / 100.0))
			}
		} else {
			qty = int64(math.Ceil(float64(packedUnits) * bi.ratio / 100.0))
		}
		deductG, deductUnits := materialNeedToDeduct(unit, qty)
		needs = append(needs, materialConsumptionNeed{
			MaterialID:   bi.materialID,
			MaterialName: strings.TrimSpace(bi.name),
			Unit:         unit,
			Qty:          qty,
			DeductG:      deductG,
			DeductUnits:  deductUnits,
		})
	}

	var bagID int64
	var bagName, bagUnit string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT m.material_id, COALESCE(mat.name,''), COALESCE(NULLIF(mat.unit,''),'个')
		FROM %s.packaging_spec_material_map m
		LEFT JOIN %s.materials mat ON mat.id=m.material_id
		WHERE m.spec_g=$1
	`, schema, schema), r.SpecG).Scan(&bagID, &bagName, &bagUnit)
	if err == nil && bagID > 0 && strings.TrimSpace(bagName) != "" {
		deductG, deductUnits := materialNeedToDeduct(bagUnit, packedUnits)
		needs = append(needs, materialConsumptionNeed{
			MaterialID:   bagID,
			MaterialName: strings.TrimSpace(bagName),
			Unit:         strings.TrimSpace(bagUnit),
			Qty:          packedUnits,
			DeductG:      deductG,
			DeductUnits:  deductUnits,
		})
	} else if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	return needs, nil
}

func deductMaterialsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finished InvQty, operator string) error {
	needs, err := calcRunningItemMaterialNeedsTx(ctx, tx, schema, r, finished)
	if err != nil {
		return err
	}
	for _, need := range needs {
		if need.DeductG <= 0 && need.DeductUnits <= 0 {
			continue
		}
		var beforeG, beforeUnits int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, schema), need.MaterialID).Scan(&beforeG, &beforeUnits); err != nil {
			return err
		}
		afterG := beforeG - need.DeductG
		afterUnits := beforeUnits - need.DeductUnits
		if afterG < 0 || afterUnits < 0 {
			return fmt.Errorf("material stock insufficient: %s", need.MaterialName)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, schema), need.MaterialID, afterG, afterUnits); err != nil {
			return err
		}
		allocations, err := materialBatchAllocationsTx(ctx, tx, schema, need.MaterialID, need.DeductG)
		if err != nil {
			return err
		}
		if len(allocations) == 0 {
			allocations = []stockdomain.BatchAllocation{{QtyG: need.DeductG}}
		}
		for _, alloc := range allocations {
			logDeductG := alloc.QtyG
			logDeductUnits := int64(0)
			if need.DeductG == 0 {
				logDeductUnits = need.DeductUnits
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.material_consumption_logs(
					running_item_id,batch_id,product_id,product_name,spec_g,
					material_id,material_name,unit,deduct_g,deduct_units,
					before_g,after_g,before_units,after_units,operator,
					material_batch_id,material_batch_code
				) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
			`, schema),
				r.ID, r.BatchID, r.ProductID, r.Product, r.SpecG,
				need.MaterialID, need.MaterialName, need.Unit, logDeductG, logDeductUnits,
				beforeG, afterG, beforeUnits, afterUnits, operator,
				alloc.BatchID, alloc.BatchCode,
			); err != nil {
				return err
			}
			qty := stockLedgerQty{
				BeforeG:     beforeG,
				ChangeG:     -logDeductG,
				AfterG:      afterG,
				BeforeUnits: beforeUnits,
				ChangeUnits: -logDeductUnits,
				AfterUnits:  afterUnits,
			}
			if err := insertStockLedgerEntryTx(ctx, tx, schema,
				stockItemTypeMaterial, need.MaterialID, need.MaterialName, 0, stockdomain.WarehouseWIP,
				stockSourceProductionRun, r.ID, alloc.BatchCode, r.BatchID,
				qty, operator,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func materialBatchAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, deductG int64) ([]stockdomain.BatchAllocation, error) {
	if deductG <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,l.qty_g
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1
		  AND l.warehouse=$2
		  AND l.qty_g > 0
		  AND b.status='active'
		  AND b.remaining_g > 0
		ORDER BY b.received_at, b.id
		FOR UPDATE OF l,b
	`, schema, schema), materialID, stockdomain.WarehouseWIP)
	if err != nil {
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	available := make([]stockdomain.BatchAvailability, 0)
	for rows.Next() {
		var batch stockdomain.BatchAvailability
		if err := rows.Scan(&batch.BatchID, &batch.BatchCode, &batch.AvailableG); err != nil {
			return nil, err
		}
		available = append(available, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("WIP stock insufficient for material %d", materialID)
	}
	allocations, err := stockdomain.AllocateFIFO(available, deductG)
	if err != nil {
		return nil, err
	}
	for _, alloc := range allocations {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batch_locations
			SET qty_g=qty_g-$2,
			    updated_at=now()
			WHERE material_batch_id=$1 AND warehouse=$3
		`, schema), alloc.BatchID, alloc.QtyG, stockdomain.WarehouseWIP); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batches
			SET remaining_g=remaining_g-$2,
			    status=CASE WHEN remaining_g-$2 <= 0 THEN 'consumed' ELSE status END
			WHERE id=$1
		`, schema), alloc.BatchID, alloc.QtyG); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches
			SET remaining_g=GREATEST(0, remaining_g-$2)
			WHERE batch_code=$1
		`, schema), alloc.BatchCode, alloc.QtyG); err != nil {
			return nil, err
		}
	}
	return allocations, nil
}

func listMaterialConsumptionSummaryTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) ([]materialConsumptionSummaryItem, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT material_id,
		       COALESCE(material_name,''),
		       COALESCE(unit,''),
		       SUM(deduct_g)::bigint,
		       SUM(deduct_units)::bigint,
		       COALESCE(NULLIF(material_batch_code,''),'')
		FROM %s.material_consumption_logs
		WHERE running_item_id=$1
		GROUP BY material_id, material_name, unit, COALESCE(NULLIF(material_batch_code,''),'')
		ORDER BY material_name, material_id, COALESCE(NULLIF(material_batch_code,''),'')
	`, schema), runningItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]materialConsumptionSummaryItem, 0)
	for rows.Next() {
		var item materialConsumptionSummaryItem
		if err := rows.Scan(&item.MaterialID, &item.MaterialName, &item.Unit, &item.DeductG, &item.DeductUnits, &item.BatchCode); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func marshalMaterialConsumptionSummary(items []materialConsumptionSummaryItem) ([]byte, error) {
	if len(items) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(items)
}
