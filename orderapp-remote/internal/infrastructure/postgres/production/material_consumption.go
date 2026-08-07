package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
)

type materialConsumptionNeed struct {
	MaterialID         int64
	MaterialName       string
	Unit               string
	Qty                int64
	QtyDecimal         float64
	DeductG            int64
	DeductUnits        int64
	RatioPct           float64
	MaterialLossRate   float64
	Source             string
	ComponentType      string
	ComponentProductID int64
	ComponentSpecG     int64
	ConsumeUnit        string
	QtyPerUnit         float64
	OutputQty          float64
	OutputUnit         string
}

type materialConsumptionSummaryItem struct {
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
	Unit         string `json:"unit"`
	DeductG      int64  `json:"deduct_g"`
	DeductUnits  int64  `json:"deduct_units"`
	BatchCode    string `json:"batch_code,omitempty"`
}

type materialSnapshotRow struct {
	MaterialID                int64   `json:"material_id"`
	MaterialName              string  `json:"material_name"`
	Unit                      string  `json:"unit"`
	RatioPct                  float64 `json:"ratio_pct,omitempty"`
	MaterialLossRate          float64 `json:"material_loss_rate,omitempty"`
	InputIncludesMaterialLoss bool    `json:"input_includes_material_loss,omitempty"`
	Source                    string  `json:"source"`
	ComponentType             string  `json:"component_type,omitempty"`
	ComponentProductID        int64   `json:"component_product_id,omitempty"`
	ComponentSpecG            int64   `json:"component_spec_g,omitempty"`
	ConsumeUnit               string  `json:"consume_unit,omitempty"`
	QtyPerUnit                float64 `json:"qty_per_unit,omitempty"`
	OutputQty                 float64 `json:"output_qty,omitempty"`
	OutputUnit                string  `json:"output_unit,omitempty"`
}

func isWeightMaterialUnit(unit string) bool {
	unit = strings.ToLower(strings.TrimSpace(unit))
	return unit == "g" || unit == "kg" || unit == "lb" || unit == "克" || unit == "千克" || unit == "公斤" || unit == "磅"
}

func materialNeedToDeduct(unit string, qty int64) (deductG int64, deductUnits int64) {
	if qty <= 0 {
		return 0, 0
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "kg", "千克":
		return qty * 1000, 0
	case "lb", "磅":
		return int64(math.Ceil(float64(qty) * 453.59237)), 0
	case "g", "克":
		return qty, 0
	default:
		return 0, qty
	}
}

func componentConsumptionQty(consumeUnit string, qtyPerUnit float64, ratioPct float64, unit string, rawG int64, packedUnits int64, boxUnits int64) int64 {
	return componentConsumptionQtyWithOutputBasis(consumeUnit, qtyPerUnit, ratioPct, unit, rawG, 0, packedUnits, boxUnits, 0, "")
}

func componentConsumptionQtyWithMaterialLoss(consumeUnit string, qtyPerUnit float64, ratioPct float64, unit string, rawG int64, outputG int64, packedUnits int64, boxUnits int64, outputQty float64, outputUnit string, materialLossRate float64) int64 {
	if normalizeBomConsumeUnit(consumeUnit) != "ratio_pct" {
		return componentConsumptionQtyWithOutputBasis(consumeUnit, qtyPerUnit, ratioPct, unit, rawG, outputG, packedUnits, boxUnits, outputQty, outputUnit)
	}
	lossRate := normalizeMaterialLossRate(materialLossRate)
	if lossRate > 0 {
		factor := 1.0 / (1.0 - lossRate)
		rawG = int64(math.Ceil(float64(rawG) * factor))
		outputG = int64(math.Ceil(float64(outputG) * factor))
		packedUnits = int64(math.Ceil(float64(packedUnits) * factor))
	}
	return componentConsumptionQtyWithOutputBasis(consumeUnit, qtyPerUnit, ratioPct, unit, rawG, outputG, packedUnits, boxUnits, outputQty, outputUnit)
}

func componentConsumptionWeightGramsWithMaterialLoss(consumeUnit string, qtyPerUnit, ratioPct float64, materialUnit string, rawG, outputG, packedUnits, boxUnits int64, outputQty float64, outputUnit string, materialLossRate float64) int64 {
	normalized := normalizeBomConsumeUnit(consumeUnit)
	if normalized == "ratio_pct" {
		ratio := bomdomain.NormalizeRatioPct(ratioPct)
		if ratio <= 0 {
			return 0
		}
		lossRate := normalizeMaterialLossRate(materialLossRate)
		factor := 1.0
		if lossRate > 0 {
			factor = 1 / (1 - lossRate)
		}
		return int64(math.Ceil(float64(rawG) * ratio / 100 * factor))
	}
	outputFactor := bomOutputBasisFactor(outputG, packedUnits, outputQty, outputUnit)
	var grams float64
	switch normalized {
	case "g":
		grams = qtyPerUnit * outputFactor
	case "kg":
		grams = qtyPerUnit * outputFactor * 1000
	case "g_per_bag":
		grams = float64(packedUnits) * qtyPerUnit
	case "unit_per_bag":
		grams = float64(packedUnits) * qtyPerUnit * productionWeightUnitGrams(materialUnit)
	case "unit_per_box":
		grams = float64(boxUnits) * qtyPerUnit * productionWeightUnitGrams(materialUnit)
	default:
		grams = qtyPerUnit * outputFactor * productionWeightUnitGrams(materialUnit)
	}
	if grams <= 0 {
		return 0
	}
	return int64(math.Ceil(grams))
}

func productionWeightUnitGrams(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		return 1000
	case "lb", "磅":
		return 453.59237
	default:
		return 1
	}
}

func normalizeMaterialLossRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate >= 1 {
		return 0
	}
	return rate
}

func componentConsumptionQtyWithOutputBasis(consumeUnit string, qtyPerUnit float64, ratioPct float64, unit string, rawG int64, outputG int64, packedUnits int64, boxUnits int64, outputQty float64, outputUnit string) int64 {
	switch normalizeBomConsumeUnit(consumeUnit) {
	case "g_per_bag", "unit_per_bag":
		return int64(math.Ceil(float64(packedUnits) * qtyPerUnit))
	case "unit_per_box":
		return int64(math.Ceil(float64(boxUnits) * qtyPerUnit))
	case "g", "kg", "fixed_qty", "unit", "length", "area":
		return fixedOutputBasisConsumptionQty(consumeUnit, qtyPerUnit, unit, outputG, packedUnits, outputQty, outputUnit)
	default:
		ratio := bomdomain.NormalizeRatioPct(ratioPct)
		if ratio <= 0 {
			return 0
		}
		if isWeightMaterialUnit(unit) {
			if strings.EqualFold(unit, "kg") || unit == "千克" {
				return int64(math.Ceil((float64(rawG) * ratio / 100.0) / 1000.0))
			}
			return int64(math.Ceil(float64(rawG) * ratio / 100.0))
		}
		return int64(math.Ceil(float64(packedUnits) * ratio / 100.0))
	}
}

func fixedOutputBasisConsumptionQty(consumeUnit string, qtyPerUnit float64, unit string, outputG int64, packedUnits int64, outputQty float64, outputUnit string) int64 {
	if qtyPerUnit <= 0 {
		return 0
	}
	factor := bomOutputBasisFactor(outputG, packedUnits, outputQty, outputUnit)
	qty := qtyPerUnit * factor
	switch normalizeBomConsumeUnit(consumeUnit) {
	case "g":
		if strings.EqualFold(unit, "kg") || unit == "千克" {
			qty = qty / 1000.0
		}
	case "kg":
		if strings.EqualFold(unit, "g") || unit == "克" {
			qty = qty * 1000.0
		}
	}
	return int64(math.Ceil(qty))
}

func bomOutputBasisFactor(outputG int64, packedUnits int64, outputQty float64, outputUnit string) float64 {
	if outputQty <= 0 {
		outputQty = 1
	}
	outputUnit = strings.ToLower(strings.TrimSpace(outputUnit))
	switch outputUnit {
	case "g", "克":
		if outputG > 0 {
			return float64(outputG) / outputQty
		}
	case "kg", "千克":
		if outputG > 0 {
			return float64(outputG) / (outputQty * 1000.0)
		}
	case "lb", "磅":
		if outputG > 0 {
			return float64(outputG) / (outputQty * 453.59237)
		}
	}
	if packedUnits > 0 {
		return float64(packedUnits) / outputQty
	}
	return 1
}

func calcRunningItemMaterialNeedsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finished InvQty) ([]materialConsumptionNeed, error) {
	if needs, ok, err := materialSnapshotNeedsTx(r, finished); ok || err != nil {
		return needs, err
	}
	return currentMaterialNeedsTx(ctx, tx, schema, r, finished)
}

func currentMaterialNeedsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finished InvQty) ([]materialConsumptionNeed, error) {
	if r.ProductID <= 0 || r.SpecG <= 0 || r.NeedG <= 0 {
		return nil, nil
	}
	q := fmt.Sprintf(`
		SELECT bi.material_id,
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0),
		       COALESCE(bi.material_loss_rate,0),
		       COALESCE(p.roast_level,''),
		       COALESCE(pbv.yield_rate, pb.yield_rate, 0),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0),
		       COALESCE(pbv.output_qty,1)::float8,
		       COALESCE(NULLIF(pbv.output_unit,''),'unit'),
		       COALESCE(NULLIF(p.drip_box_bag_count,0),10)
		FROM %s.products p
		LEFT JOIN %s.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN LATERAL (
			SELECT latest.id AS bom_version_id
			FROM %s.production_boms pb
			JOIN LATERAL (
				SELECT v.id, v.published_at, v.created_at
				FROM %s.production_bom_versions v
				WHERE v.bom_id=pb.id
				  AND v.status='published'
				  AND EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)
				ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) latest ON true
			WHERE pb.output_product_id=p.id
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			ORDER BY CASE WHEN pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0) THEN 0 ELSE 1 END,
			         latest.published_at DESC NULLS LAST, latest.created_at DESC, latest.id DESC, pb.id DESC
			LIMIT 1
		) output_bom ON true
		LEFT JOIN %s.production_bom_versions pbv ON pbv.id=output_bom.bom_version_id
		JOIN LATERAL (
			SELECT pbi.id, pbi.material_id, pbi.ratio_pct, pbi.material_loss_rate, pbi.component_type, pbi.component_product_id, pbi.component_spec_g, pbi.consume_unit, pbi.qty_per_unit
			FROM %s.production_bom_version_items pbi
			WHERE COALESCE(output_bom.bom_version_id,0)>0
			  AND pbi.version_id=output_bom.bom_version_id
			UNION ALL
			SELECT lbi.id, lbi.material_id, lbi.ratio_pct, 0 AS material_loss_rate, lbi.component_type, lbi.component_product_id, lbi.component_spec_g, lbi.consume_unit, lbi.qty_per_unit
			FROM %s.product_bom_items lbi
			WHERE COALESCE(output_bom.bom_version_id,0)=0 AND lbi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
		) bi ON true
		LEFT JOIN %s.product_bom pb ON pb.product_id=CASE
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
			ELSE p.id
		END
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		LEFT JOIN %s.products cp ON cp.id=bi.component_product_id
		WHERE p.id=$1
		ORDER BY bi.id
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	rows, err := tx.Query(ctx, q, r.ProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bomRow struct {
		materialID           int64
		name                 string
		unit                 string
		ratio                float64
		materialLossRate     float64
		roastLevel           string
		yieldRate            float64
		componentType        string
		componentProductID   int64
		componentProductName string
		componentSpecG       int64
		consumeUnit          string
		qtyPerUnit           float64
		outputQty            float64
		outputUnit           string
		dripBoxBagCount      int64
	}
	bomRows := make([]bomRow, 0)
	for rows.Next() {
		var x bomRow
		if err := rows.Scan(
			&x.materialID, &x.name, &x.unit, &x.ratio, &x.materialLossRate, &x.roastLevel, &x.yieldRate,
			&x.componentType, &x.componentProductID, &x.componentProductName, &x.componentSpecG,
			&x.consumeUnit, &x.qtyPerUnit, &x.outputQty, &x.outputUnit, &x.dripBoxBagCount,
		); err != nil {
			return nil, err
		}
		x.componentType = normalizeBomComponentType(x.componentType)
		x.consumeUnit = normalizeBomConsumeUnit(x.consumeUnit)
		x.ratio = bomdomain.NormalizeRatioPct(x.ratio)
		x.materialLossRate = normalizeMaterialLossRate(x.materialLossRate)
		if x.componentType == "finished_product" {
			if x.componentProductID <= 0 || x.qtyPerUnit <= 0 {
				continue
			}
			x.materialID = x.componentProductID
			x.name = strings.TrimSpace(x.componentProductName)
			if x.name == "" {
				x.name = fmt.Sprintf("finished product %d", x.componentProductID)
			}
			x.unit = "g"
		} else if x.materialID <= 0 || strings.TrimSpace(x.name) == "" || (x.ratio <= 0 && x.qtyPerUnit <= 0) {
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
	boxUnits := int64(0)

	needs := make([]materialConsumptionNeed, 0, len(bomRows)+1)
	for _, bi := range bomRows {
		unit := strings.TrimSpace(bi.unit)
		if unit == "" {
			unit = "g"
		}
		if bi.consumeUnit == "unit_per_box" && boxUnits <= 0 {
			boxUnits = ceilDiv64(packedUnits, bi.dripBoxBagCount)
		}
		outputG := finishedTotalG(r.SpecG, packedUnits, finished.LooseG)
		if outputG <= 0 {
			outputG = r.NeedG
		}
		qty := componentConsumptionQtyWithMaterialLoss(bi.consumeUnit, bi.qtyPerUnit, bi.ratio, unit, rawG, outputG, packedUnits, boxUnits, bi.outputQty, bi.outputUnit, bi.materialLossRate)
		deductG, deductUnits := materialNeedToDeduct(unit, qty)
		source := "bom"
		componentType := bi.componentType
		if componentType == "finished_product" {
			source = "finished_product"
			deductG = qty
			deductUnits = 0
		}
		needs = append(needs, materialConsumptionNeed{
			MaterialID:         bi.materialID,
			MaterialName:       strings.TrimSpace(bi.name),
			Unit:               unit,
			Qty:                qty,
			DeductG:            deductG,
			DeductUnits:        deductUnits,
			RatioPct:           bi.ratio,
			MaterialLossRate:   bi.materialLossRate,
			Source:             source,
			ComponentType:      componentType,
			ComponentProductID: bi.componentProductID,
			ComponentSpecG:     bi.componentSpecG,
			ConsumeUnit:        bi.consumeUnit,
			QtyPerUnit:         bi.qtyPerUnit,
			OutputQty:          bi.outputQty,
			OutputUnit:         bi.outputUnit,
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
			Source:       "packaging",
		})
	} else if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	return needs, nil
}

func materialNeedsForRunOutputsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, outputs []ProduceRunOutputRow) ([]materialConsumptionNeed, error) {
	if r.ProductID <= 0 || r.NeedG <= 0 {
		return nil, nil
	}
	q := fmt.Sprintf(`
		SELECT bi.material_id,
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0),
		       COALESCE(bi.material_loss_rate,0),
		       COALESCE(p.roast_level,''),
		       COALESCE(pbv.yield_rate, pb.yield_rate, 0),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0),
		       COALESCE(pbv.output_qty,1)::float8,
		       COALESCE(NULLIF(pbv.output_unit,''),'unit'),
		       COALESCE(NULLIF(p.drip_box_bag_count,0),10)
		FROM %s.products p
		LEFT JOIN %s.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN LATERAL (
			SELECT latest.id AS bom_version_id
			FROM %s.production_boms pb
			JOIN LATERAL (
				SELECT v.id, v.published_at, v.created_at
				FROM %s.production_bom_versions v
				WHERE v.bom_id=pb.id
				  AND v.status='published'
				  AND EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)
				ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) latest ON true
			WHERE pb.output_product_id=p.id
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			ORDER BY CASE WHEN pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0) THEN 0 ELSE 1 END,
			         latest.published_at DESC NULLS LAST, latest.created_at DESC, latest.id DESC, pb.id DESC
			LIMIT 1
		) output_bom ON true
		LEFT JOIN %s.production_bom_versions pbv ON pbv.id=output_bom.bom_version_id
		JOIN LATERAL (
			SELECT pbi.id, pbi.material_id, pbi.ratio_pct, pbi.material_loss_rate, pbi.component_type, pbi.component_product_id, pbi.component_spec_g, pbi.consume_unit, pbi.qty_per_unit
			FROM %s.production_bom_version_items pbi
			WHERE COALESCE(output_bom.bom_version_id,0)>0
			  AND pbi.version_id=output_bom.bom_version_id
			UNION ALL
			SELECT lbi.id, lbi.material_id, lbi.ratio_pct, 0 AS material_loss_rate, lbi.component_type, lbi.component_product_id, lbi.component_spec_g, lbi.consume_unit, lbi.qty_per_unit
			FROM %s.product_bom_items lbi
			WHERE COALESCE(output_bom.bom_version_id,0)=0 AND lbi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
		) bi ON true
		LEFT JOIN %s.product_bom pb ON pb.product_id=CASE
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
			ELSE p.id
		END
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		LEFT JOIN %s.products cp ON cp.id=bi.component_product_id
		WHERE p.id=$1
		ORDER BY bi.id
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	rows, err := tx.Query(ctx, q, r.ProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bomRow struct {
		materialID           int64
		name                 string
		unit                 string
		ratio                float64
		materialLossRate     float64
		roastLevel           string
		yieldRate            float64
		componentType        string
		componentProductID   int64
		componentProductName string
		componentSpecG       int64
		consumeUnit          string
		qtyPerUnit           float64
		outputQty            float64
		outputUnit           string
		dripBoxBagCount      int64
	}
	bomRows := make([]bomRow, 0)
	for rows.Next() {
		var x bomRow
		if err := rows.Scan(
			&x.materialID, &x.name, &x.unit, &x.ratio, &x.materialLossRate, &x.roastLevel, &x.yieldRate,
			&x.componentType, &x.componentProductID, &x.componentProductName, &x.componentSpecG,
			&x.consumeUnit, &x.qtyPerUnit, &x.outputQty, &x.outputUnit, &x.dripBoxBagCount,
		); err != nil {
			return nil, err
		}
		x.componentType = normalizeBomComponentType(x.componentType)
		x.consumeUnit = normalizeBomConsumeUnit(x.consumeUnit)
		x.ratio = bomdomain.NormalizeRatioPct(x.ratio)
		x.materialLossRate = normalizeMaterialLossRate(x.materialLossRate)
		if x.componentType == "finished_product" {
			if x.componentProductID <= 0 || x.qtyPerUnit <= 0 {
				continue
			}
			x.materialID = x.componentProductID
			x.name = strings.TrimSpace(x.componentProductName)
			if x.name == "" {
				x.name = fmt.Sprintf("finished product %d", x.componentProductID)
			}
			x.unit = "g"
		} else if x.materialID <= 0 || strings.TrimSpace(x.name) == "" || (x.ratio <= 0 && x.qtyPerUnit <= 0) {
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
	totalPackedUnits := int64(0)
	totalOutputG := int64(0)
	for _, output := range outputs {
		totalPackedUnits += outputPackedUnits(output)
		totalOutputG += outputTotalG(output)
	}
	if totalOutputG <= 0 {
		totalOutputG = r.NeedG
	}

	needs := make([]materialConsumptionNeed, 0, len(bomRows)+len(outputs))
	for _, bi := range bomRows {
		unit := strings.TrimSpace(bi.unit)
		if unit == "" {
			unit = "g"
		}
		boxUnits := int64(0)
		if bi.consumeUnit == "unit_per_box" {
			boxUnits = ceilDiv64(totalPackedUnits, bi.dripBoxBagCount)
		}
		qty := componentConsumptionQtyWithMaterialLoss(bi.consumeUnit, bi.qtyPerUnit, bi.ratio, unit, rawG, totalOutputG, totalPackedUnits, boxUnits, bi.outputQty, bi.outputUnit, bi.materialLossRate)
		deductG, deductUnits := materialNeedToDeduct(unit, qty)
		source := "bom"
		componentType := bi.componentType
		if componentType == "finished_product" {
			source = "finished_product"
			deductG = qty
			deductUnits = 0
		}
		needs = append(needs, materialConsumptionNeed{
			MaterialID:         bi.materialID,
			MaterialName:       strings.TrimSpace(bi.name),
			Unit:               unit,
			Qty:                qty,
			DeductG:            deductG,
			DeductUnits:        deductUnits,
			RatioPct:           bi.ratio,
			MaterialLossRate:   bi.materialLossRate,
			Source:             source,
			ComponentType:      componentType,
			ComponentProductID: bi.componentProductID,
			ComponentSpecG:     bi.componentSpecG,
			ConsumeUnit:        bi.consumeUnit,
			QtyPerUnit:         bi.qtyPerUnit,
			OutputQty:          bi.outputQty,
			OutputUnit:         bi.outputUnit,
		})
	}

	for _, output := range outputs {
		packedUnits := outputPackedUnits(output)
		if output.SpecG <= 0 || packedUnits <= 0 {
			continue
		}
		var bagID int64
		var bagName, bagUnit string
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT m.material_id, COALESCE(mat.name,''), COALESCE(NULLIF(mat.unit,''),'个')
			FROM %s.packaging_spec_material_map m
			LEFT JOIN %s.materials mat ON mat.id=m.material_id
			WHERE m.spec_g=$1
		`, schema, schema), output.SpecG).Scan(&bagID, &bagName, &bagUnit)
		if err == nil && bagID > 0 && strings.TrimSpace(bagName) != "" {
			deductG, deductUnits := materialNeedToDeduct(bagUnit, packedUnits)
			needs = append(needs, materialConsumptionNeed{
				MaterialID:   bagID,
				MaterialName: strings.TrimSpace(bagName),
				Unit:         strings.TrimSpace(bagUnit),
				Qty:          packedUnits,
				DeductG:      deductG,
				DeductUnits:  deductUnits,
				Source:       "packaging",
			})
		} else if err != nil && err != pgx.ErrNoRows {
			return nil, err
		}
	}
	return aggregateMaterialConsumptionNeeds(needs), nil
}

func outputPackedUnits(output ProduceRunOutputRow) int64 {
	if output.FinishedUnits > 0 {
		return output.FinishedUnits
	}
	if output.PlanUnits > 0 {
		return output.PlanUnits
	}
	return 0
}

func outputTotalG(output ProduceRunOutputRow) int64 {
	if got := finishedTotalG(output.SpecG, output.FinishedUnits, output.FinishedLooseG); got > 0 {
		return got
	}
	if got := finishedTotalG(output.SpecG, output.PlanUnits, output.PlanLooseG); got > 0 {
		return got
	}
	return output.NeedG
}

func buildMaterialSnapshotForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow) ([]byte, error) {
	needs, err := currentMaterialNeedsTx(ctx, tx, schema, r, InvQty{Units: r.PlanUnits, LooseG: r.PlanLooseG})
	if err != nil {
		return nil, err
	}
	rows := make([]materialSnapshotRow, 0, len(needs))
	for _, need := range needs {
		source := strings.TrimSpace(need.Source)
		if source == "" {
			source = "bom"
		}
		rows = append(rows, materialSnapshotRow{
			MaterialID:         need.MaterialID,
			MaterialName:       need.MaterialName,
			Unit:               need.Unit,
			RatioPct:           need.RatioPct,
			MaterialLossRate:   need.MaterialLossRate,
			Source:             source,
			ComponentType:      need.ComponentType,
			ComponentProductID: need.ComponentProductID,
			ComponentSpecG:     need.ComponentSpecG,
			ConsumeUnit:        need.ConsumeUnit,
			QtyPerUnit:         need.QtyPerUnit,
			OutputQty:          need.OutputQty,
			OutputUnit:         need.OutputUnit,
		})
	}
	if len(rows) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(rows)
}

func buildMaterialSnapshotForBomVersionTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, bomVersionID int64, inputIncludesMaterialLoss bool) ([]byte, error) {
	if bomVersionID <= 0 {
		return nil, fmt.Errorf("production BOM version required: %s", r.Product)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT
			i.material_id,
			COALESCE(m.name,''),
			COALESCE(NULLIF(m.unit,''),'g'),
			COALESCE(i.ratio_pct,0)::float8,
			COALESCE(i.material_loss_rate,0)::float8,
			COALESCE(NULLIF(i.component_type,''),'material'),
			COALESCE(i.component_product_id,0),
			COALESCE(cp.name,''),
			COALESCE(i.component_spec_g,0),
			COALESCE(NULLIF(i.consume_unit,''),'ratio_pct'),
			COALESCE(i.qty_per_unit,0)::float8,
			COALESCE(NULLIF(v.output_qty,0),1)::float8,
			COALESCE(NULLIF(v.output_unit,''),'unit')
		FROM %s.production_bom_version_items i
		JOIN %s.production_bom_versions v ON v.id=i.version_id
		LEFT JOIN %s.materials m ON m.id=i.material_id
		LEFT JOIN %s.products cp ON cp.id=i.component_product_id
		WHERE i.version_id=$1
		ORDER BY i.id
	`, schema, schema, schema, schema), bomVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	snapshot := make([]materialSnapshotRow, 0)
	for rows.Next() {
		var row materialSnapshotRow
		var componentProductName string
		if err := rows.Scan(
			&row.MaterialID, &row.MaterialName, &row.Unit, &row.RatioPct, &row.MaterialLossRate,
			&row.ComponentType, &row.ComponentProductID, &componentProductName, &row.ComponentSpecG,
			&row.ConsumeUnit, &row.QtyPerUnit, &row.OutputQty, &row.OutputUnit,
		); err != nil {
			return nil, err
		}
		row.ComponentType = normalizeBomComponentType(row.ComponentType)
		row.ConsumeUnit = normalizeBomConsumeUnit(row.ConsumeUnit)
		row.RatioPct = bomdomain.NormalizeRatioPct(row.RatioPct)
		row.MaterialLossRate = normalizeMaterialLossRate(row.MaterialLossRate)
		row.InputIncludesMaterialLoss = inputIncludesMaterialLoss && row.MaterialLossRate > 0
		row.Source = "bom"
		if row.ComponentType == "finished_product" {
			if row.ComponentProductID <= 0 || row.QtyPerUnit <= 0 {
				return nil, fmt.Errorf("production BOM version has invalid finished-product component: %s", r.Product)
			}
			row.MaterialID = row.ComponentProductID
			row.MaterialName = firstNonEmpty(componentProductName, fmt.Sprintf("finished product %d", row.ComponentProductID))
			row.Unit = "g"
			row.Source = "finished_product"
		} else if row.MaterialID <= 0 || strings.TrimSpace(row.MaterialName) == "" || (row.RatioPct <= 0 && row.QtyPerUnit <= 0) {
			return nil, fmt.Errorf("production BOM version has invalid material line: %s", r.Product)
		}
		snapshot = append(snapshot, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(snapshot) == 0 {
		return nil, fmt.Errorf("production BOM version has no material lines: %s", r.Product)
	}
	return json.Marshal(snapshot)
}

func materialSnapshotNeedsTx(r ProduceRunRow, finished InvQty) ([]materialConsumptionNeed, bool, error) {
	raw := strings.TrimSpace(r.MaterialSnapshot)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil, false, nil
	}
	var rows []materialSnapshotRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, true, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	rawG := r.InputG
	if rawG <= 0 {
		rawG = r.NeedG
	}
	packedUnits := finished.Units
	if packedUnits < 0 {
		packedUnits = 0
	}
	outputG := finishedTotalG(r.SpecG, packedUnits, finished.LooseG)
	if outputG <= 0 {
		outputG = r.NeedG
	}
	needs := make([]materialConsumptionNeed, 0, len(rows))
	for _, row := range rows {
		if row.MaterialID <= 0 || strings.TrimSpace(row.MaterialName) == "" {
			continue
		}
		unit := strings.TrimSpace(row.Unit)
		if unit == "" {
			unit = "g"
		}
		source := strings.TrimSpace(row.Source)
		if source == "" {
			source = "bom"
		}
		qty := int64(0)
		qtyDecimal := float64(0)
		deductG, deductUnits := int64(0), int64(0)
		ratioPct := row.RatioPct
		if normalizeBomConsumeUnit(row.ConsumeUnit) == "ratio_pct" && !isWeightMaterialUnit(unit) && ratioPct <= 0 {
			ratioPct = 100
		}
		materialLossRate := normalizeMaterialLossRate(row.MaterialLossRate)
		if row.InputIncludesMaterialLoss {
			materialLossRate = 0
		}
		if source == "packaging" {
			qty = packedUnits
		} else if source == "finished_product" {
			qty = componentConsumptionQtyWithMaterialLoss(row.ConsumeUnit, row.QtyPerUnit, ratioPct, unit, rawG, outputG, packedUnits, 0, row.OutputQty, row.OutputUnit, 0)
		} else if isWeightMaterialUnit(unit) {
			deductG = componentConsumptionWeightGramsWithMaterialLoss(
				row.ConsumeUnit, row.QtyPerUnit, ratioPct, unit, rawG, outputG,
				packedUnits, 0, row.OutputQty, row.OutputUnit, materialLossRate,
			)
			if productionWeightUnitGrams(unit) > 1 {
				qtyDecimal = float64(deductG) / productionWeightUnitGrams(unit)
			} else {
				qtyDecimal = float64(deductG)
			}
			qty = int64(math.Ceil(qtyDecimal))
		} else {
			qty = componentConsumptionQtyWithMaterialLoss(row.ConsumeUnit, row.QtyPerUnit, ratioPct, unit, rawG, outputG, packedUnits, 0, row.OutputQty, row.OutputUnit, materialLossRate)
		}
		if qty <= 0 && deductG <= 0 {
			continue
		}
		if deductG <= 0 {
			deductG, deductUnits = materialNeedToDeduct(unit, qty)
		}
		if source == "finished_product" {
			deductG = qty
			deductUnits = 0
		}
		needs = append(needs, materialConsumptionNeed{
			MaterialID:         row.MaterialID,
			MaterialName:       strings.TrimSpace(row.MaterialName),
			Unit:               unit,
			Qty:                qty,
			QtyDecimal:         qtyDecimal,
			DeductG:            deductG,
			DeductUnits:        deductUnits,
			RatioPct:           row.RatioPct,
			MaterialLossRate:   materialLossRate,
			Source:             source,
			ComponentType:      row.ComponentType,
			ComponentProductID: row.ComponentProductID,
			ComponentSpecG:     row.ComponentSpecG,
			ConsumeUnit:        row.ConsumeUnit,
			QtyPerUnit:         row.QtyPerUnit,
			OutputQty:          row.OutputQty,
			OutputUnit:         row.OutputUnit,
		})
	}
	return needs, true, nil
}

func ensureWIPStockForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, materialSnapshot []byte) error {
	return ensureWIPStockForRunningItemWorkOrderTx(ctx, tx, schema, 0, r, materialSnapshot)
}

func runningItemWorkOrderMaterialNeedsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, r ProduceRunRow, materialSnapshot []byte) ([]materialConsumptionNeed, error) {
	r.MaterialSnapshot = strings.TrimSpace(string(materialSnapshot))
	needs, ok, err := materialSnapshotNeedsTx(r, InvQty{Units: r.PlanUnits, LooseG: r.PlanLooseG})
	if err != nil {
		return nil, err
	}
	if ok && len(needs) > 0 {
		return needs, nil
	}
	if workOrderID > 0 {
		needs, err = workOrderReservationNeedsTx(ctx, tx, schema, workOrderID)
		if err != nil {
			return nil, err
		}
		if len(needs) == 0 {
			return nil, fmt.Errorf("WIP资料待完善: work order %d has no frozen material snapshot or reservation requirements", workOrderID)
		}
		return needs, nil
	}
	return currentMaterialNeedsTx(ctx, tx, schema, r, InvQty{Units: r.PlanUnits, LooseG: r.PlanLooseG})
}

func ensureWIPStockForRunningItemWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, r ProduceRunRow, materialSnapshot []byte) error {
	needs, err := runningItemWorkOrderMaterialNeedsTx(ctx, tx, schema, workOrderID, r, materialSnapshot)
	if err != nil {
		return err
	}
	return ensureWIPStockForWorkOrderNeedsTx(ctx, tx, schema, workOrderID, needs)
}

type workOrderWIPNeedCoverage struct {
	Need                 materialConsumptionNeed
	WIPG                 int64
	WIPUnits             int64
	CurrentConsumedG     int64
	CurrentConsumedUnits int64
	OtherReservedG       int64
	OtherReservedUnits   int64
	AvailableG           int64
	AvailableUnits       int64
	ShortageG            int64
	ShortageUnits        int64
}

func workOrderWIPCoverageForNeedsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, needs []materialConsumptionNeed) ([]workOrderWIPNeedCoverage, error) {
	hasLocationUnits, err := schemaColumnExistsTx(ctx, tx, schema, "material_batch_locations", "qty_units")
	if err != nil {
		return nil, err
	}
	hasReservationUnits, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "reserved_units")
	if err != nil {
		return nil, err
	}
	hasReservationWorkOrder, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "work_order_id")
	if err != nil {
		return nil, err
	}
	hasBatchRemainingUnits, err := schemaColumnExistsTx(ctx, tx, schema, "material_batches", "remaining_units")
	if err != nil {
		return nil, err
	}
	hasWorkOrderStatus, err := schemaColumnExistsTx(ctx, tx, schema, "work_orders", "status")
	if err != nil {
		return nil, err
	}
	hasCustomerProcessingReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil {
		return nil, err
	}
	rows := make([]workOrderWIPNeedCoverage, 0)
	for _, need := range aggregateMaterialConsumptionNeeds(needs) {
		if need.Source == "finished_product" || need.ComponentType == "finished_product" {
			continue
		}
		if need.MaterialID <= 0 || (need.DeductG <= 0 && need.DeductUnits <= 0) {
			continue
		}
		var wipG, wipUnits int64
		if hasLocationUnits && hasBatchRemainingUnits {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(l.qty_g),0)::bigint,COALESCE(SUM(l.qty_units),0)::bigint
				FROM %s.material_batch_locations l
				JOIN %s.material_batches b ON b.id=l.material_batch_id
				WHERE l.material_id=$1
				  AND l.warehouse=$2
				  AND (l.qty_g > 0 OR l.qty_units > 0)
				  AND b.status='active'
				  AND (b.remaining_g > 0 OR b.remaining_units > 0)
				  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			`, schema, schema), need.MaterialID, stockdomain.WarehouseWIP).Scan(&wipG, &wipUnits)
		} else {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(l.qty_g),0)::bigint
				FROM %s.material_batch_locations l
				JOIN %s.material_batches b ON b.id=l.material_batch_id
				WHERE l.material_id=$1 AND l.warehouse=$2 AND l.qty_g>0
				  AND b.status='active' AND b.remaining_g>0
				  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			`, schema, schema), need.MaterialID, stockdomain.WarehouseWIP).Scan(&wipG)
			wipUnits = 0
		}
		if err != nil {
			return nil, err
		}
		var otherReservedG, otherReservedUnits int64
		if hasReservationUnits && hasReservationWorkOrder {
			if hasWorkOrderStatus {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
					       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
					FROM %s.work_order_material_reservations r
					JOIN %s.work_orders wo ON wo.id=r.work_order_id
					WHERE r.material_id=$1 AND r.status='reserved'
					  AND wo.status IN ('released','running','partially_completed','paused')
					  AND ($2::bigint=0 OR r.work_order_id<>$2)
				`, schema, schema), need.MaterialID, workOrderID).Scan(&otherReservedG, &otherReservedUnits)
			} else {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,
					       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint
					FROM %s.work_order_material_reservations
					WHERE material_id=$1 AND status='reserved' AND ($2::bigint=0 OR work_order_id<>$2)
				`, schema), need.MaterialID, workOrderID).Scan(&otherReservedG, &otherReservedUnits)
			}
		} else {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint
				FROM %s.work_order_material_reservations
				WHERE material_id=$1 AND status='reserved'
			`, schema), need.MaterialID).Scan(&otherReservedG)
			otherReservedUnits = 0
		}
		if err != nil {
			return nil, err
		}
		if hasCustomerProcessingReservations {
			var customerReservedG, customerReservedUnits int64
			if hasReservationWorkOrder {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
					       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
					FROM %s.customer_processing_material_reservations r
					WHERE r.material_id=$1 AND r.component_type<>'finished_product' AND r.status='reserved'
					  AND ($2::bigint=0 OR r.work_order_id<>$2)
					  AND (r.material_batch_id>0 OR r.source_warehouse_code=$3)
					  AND NOT EXISTS (
						SELECT 1 FROM %s.work_order_material_reservations wr
						WHERE wr.work_order_id=r.work_order_id AND wr.material_id=r.material_id AND wr.status='reserved'
					  )
				`, schema, schema), need.MaterialID, workOrderID, stockdomain.WarehouseWIP).Scan(&customerReservedG, &customerReservedUnits)
			} else {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,
					       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint
					FROM %s.customer_processing_material_reservations
					WHERE material_id=$1 AND component_type<>'finished_product' AND status='reserved'
					  AND ($2::bigint=0 OR work_order_id<>$2)
					  AND (material_batch_id>0 OR source_warehouse_code=$3)
				`, schema), need.MaterialID, workOrderID, stockdomain.WarehouseWIP).Scan(&customerReservedG, &customerReservedUnits)
			}
			if err != nil {
				return nil, err
			}
			otherReservedG += customerReservedG
			otherReservedUnits += customerReservedUnits
		}
		var currentConsumedG, currentConsumedUnits int64
		if workOrderID > 0 && hasReservationWorkOrder {
			if hasReservationUnits {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(consumed_g),0)::bigint,COALESCE(SUM(consumed_units),0)::bigint
					FROM %s.work_order_material_reservations
					WHERE work_order_id=$1 AND material_id=$2
				`, schema), workOrderID, need.MaterialID).Scan(&currentConsumedG, &currentConsumedUnits)
			} else {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(SUM(consumed_g),0)::bigint
					FROM %s.work_order_material_reservations
					WHERE work_order_id=$1 AND material_id=$2
				`, schema), workOrderID, need.MaterialID).Scan(&currentConsumedG)
			}
			if err != nil {
				return nil, err
			}
		}
		availableG := wipG - otherReservedG
		if availableG < 0 {
			availableG = 0
		}
		availableUnits := wipUnits - otherReservedUnits
		if availableUnits < 0 {
			availableUnits = 0
		}
		row := workOrderWIPNeedCoverage{
			Need: need, WIPG: wipG, WIPUnits: wipUnits,
			CurrentConsumedG: currentConsumedG, CurrentConsumedUnits: currentConsumedUnits,
			OtherReservedG: otherReservedG, OtherReservedUnits: otherReservedUnits,
			AvailableG: availableG, AvailableUnits: availableUnits,
		}
		row.ShortageG = workOrderRemainingWIPShortage(need.DeductG, currentConsumedG, availableG)
		row.ShortageUnits = workOrderRemainingWIPShortage(need.DeductUnits, currentConsumedUnits, availableUnits)
		rows = append(rows, row)
	}
	return rows, nil
}

func schemaColumnExistsTx(ctx context.Context, tx pgx.Tx, schema, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

func nonnegativeQuantity(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func workOrderRemainingWIPShortage(required, consumed, available int64) int64 {
	return nonnegativeQuantity(required - consumed - available)
}

func ensureWIPStockForNeedsTx(ctx context.Context, tx pgx.Tx, schema string, needs []materialConsumptionNeed) error {
	return ensureWIPStockForWorkOrderNeedsTx(ctx, tx, schema, 0, needs)
}

func ensureWIPStockForWorkOrderNeedsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, needs []materialConsumptionNeed) error {
	hasMaterials, err := schemaColumnExistsTx(ctx, tx, schema, "materials", "id")
	if err != nil {
		return err
	}
	if hasMaterials {
		materialIDs := make([]int64, 0)
		seen := map[int64]bool{}
		for _, need := range aggregateMaterialConsumptionNeeds(needs) {
			if need.MaterialID > 0 && !seen[need.MaterialID] {
				seen[need.MaterialID] = true
				materialIDs = append(materialIDs, need.MaterialID)
			}
		}
		if len(materialIDs) > 0 {
			rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE id=ANY($1::bigint[]) ORDER BY id FOR UPDATE`, schema), materialIDs)
			if err != nil {
				return err
			}
			for rows.Next() {
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()
		}
	}
	coverage, err := workOrderWIPCoverageForNeedsTx(ctx, tx, schema, workOrderID, needs)
	if err != nil {
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") || strings.Contains(err.Error(), "quality_status") {
			return nil
		}
		return err
	}
	shortages := make([]string, 0)
	for _, row := range coverage {
		if row.ShortageG > 0 || row.ShortageUnits > 0 {
			name := strings.TrimSpace(row.Need.MaterialName)
			if name == "" {
				name = fmt.Sprintf("material %d", row.Need.MaterialID)
			}
			if row.Need.DeductG > 0 {
				shortages = append(shortages, fmt.Sprintf("%s need %dg, available %dg, reserved %dg", name, row.Need.DeductG, row.WIPG, row.OtherReservedG))
			} else {
				shortages = append(shortages, fmt.Sprintf("%s need %d%s, available %d%s, reserved %d%s", name, row.Need.DeductUnits, row.Need.Unit, row.WIPUnits, row.Need.Unit, row.OtherReservedUnits, row.Need.Unit))
			}
		}
	}
	if len(shortages) > 0 {
		return fmt.Errorf("WIP stock insufficient: %s; transfer raw material to WIP before starting production", strings.Join(shortages, "; "))
	}
	return nil
}

func deductMaterialsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finished InvQty, operator string) error {
	needs, err := calcRunningItemMaterialNeedsTx(ctx, tx, schema, r, finished)
	if err != nil {
		return err
	}
	return deductMaterialNeedsForRunningItemTx(ctx, tx, schema, r, needs, operator)
}

func deductMaterialsForRunOutputsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, outputs []ProduceRunOutputRow, operator string) error {
	needs, err := materialNeedsForRunOutputsTx(ctx, tx, schema, r, outputs)
	if err != nil {
		return err
	}
	return deductMaterialNeedsForRunningItemTx(ctx, tx, schema, r, needs, operator)
}

func deductMaterialNeedsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, needs []materialConsumptionNeed, operator string) error {
	for _, need := range needs {
		if need.DeductG <= 0 && need.DeductUnits <= 0 {
			continue
		}
		if need.Source == "finished_product" || need.ComponentType == "finished_product" {
			if err := deductFinishedProductComponentForRunningItemTx(ctx, tx, schema, r, need, operator); err != nil {
				return err
			}
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
		allocations, err := materialBatchConsumptionsForRunningItemTx(ctx, tx, schema, r.ID, need.MaterialID, need.DeductG, need.DeductUnits)
		if err != nil {
			return err
		}
		if len(allocations) == 0 {
			allocations = []customerProcessingBatchAllocation{{QtyG: need.DeductG, QtyUnits: need.DeductUnits}}
		}
		for _, alloc := range allocations {
			logDeductG := alloc.QtyG
			logDeductUnits := alloc.QtyUnits
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
		if err := updateMaterialReservationConsumedTx(ctx, tx, schema, r.ID, need.MaterialID, need.DeductG, need.DeductUnits); err != nil {
			return err
		}
	}
	return nil
}

func deductFinishedProductComponentForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, need materialConsumptionNeed, operator string) error {
	productID := need.ComponentProductID
	if productID <= 0 {
		productID = need.MaterialID
	}
	need.DeductG, need.DeductUnits = normalizeCustomerProcessingFinishedQuantity(need.ComponentSpecG, need.DeductG, need.DeductUnits)
	if productID <= 0 || need.DeductG <= 0 {
		return nil
	}
	specG := need.ComponentSpecG
	batchAllocations, err := finishedBatchConsumptionsForRunningItemTx(ctx, tx, schema, r.ID, productID, specG, need.DeductG, need.DeductUnits)
	if err != nil {
		return err
	}
	warehouse, err := finishedComponentConsumptionWarehouse(batchAllocations)
	if err != nil {
		return err
	}
	var beforeUnits, beforeLooseG int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3
		FOR UPDATE
	`, schema), productID, specG, warehouse).Scan(&beforeUnits, &beforeLooseG)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	beforeG := finishedComponentTotalG(specG, beforeUnits, beforeLooseG)
	if beforeG < need.DeductG {
		return fmt.Errorf("finished product stock insufficient: %s", need.MaterialName)
	}
	if len(batchAllocations) == 0 {
		batchAllocations = []customerProcessingFinishedBatchAllocation{{QtyG: need.DeductG, QtyUnits: need.DeductUnits}}
	}
	afterUnits, afterLooseG, err := deductFinishedComponentQty(specG, beforeUnits, beforeLooseG, need.DeductG)
	if err != nil {
		return err
	}
	afterG := finishedComponentTotalG(specG, afterUnits, afterLooseG)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,
		    onhand_loose_g=excluded.onhand_loose_g,
		    updated_at=now()
	`, schema), productID, specG, warehouse, afterUnits, afterLooseG); err != nil {
		return err
	}
	for _, allocation := range batchAllocations {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_consumption_logs(
				running_item_id,batch_id,product_id,product_name,spec_g,
				material_id,material_name,unit,deduct_g,deduct_units,
				before_g,after_g,before_units,after_units,operator,
				material_batch_id,material_batch_code
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,0,$16)
		`, schema),
			r.ID, r.BatchID, r.ProductID, r.Product, r.SpecG,
			productID, need.MaterialName, "g", allocation.QtyG, allocation.QtyUnits,
			beforeG, afterG, beforeUnits, afterUnits, operator, allocation.BatchCode,
		); err != nil {
			return err
		}
	}
	if err := insertStockLedgerEntryTx(ctx, tx, schema,
		stockItemTypeFinishedProduct, productID, need.MaterialName, specG, warehouse,
		stockSourceProductionRun, r.ID, "", r.BatchID,
		stockLedgerQty{
			BeforeG:     beforeG,
			ChangeG:     -need.DeductG,
			AfterG:      afterG,
			BeforeUnits: beforeUnits,
			ChangeUnits: afterUnits - beforeUnits,
			AfterUnits:  afterUnits,
		}, operator,
	); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "produce_running", &r.ID, "consume_finished_product_component", postgresinfra.StrPtr("finished_product_component_consumption"), postgresinfra.StrPtr(fmt.Sprintf("%d", beforeG)), postgresinfra.StrPtr(fmt.Sprintf("%d", afterG)), postgresinfra.AuditMeta{
		"running_item_id":                           r.ID,
		"batch_id":                                  r.BatchID,
		"drip_demand":                               true,
		"drip_product_id":                           r.ProductID,
		"drip_product_name":                         r.Product,
		"drip_spec_g":                               r.SpecG,
		"drip_need_g":                               r.NeedG,
		"upstream_roast_demand_g":                   need.DeductG,
		"upstream_product_id":                       productID,
		"upstream_product_name":                     need.MaterialName,
		"finished_product_component_consumption":    true,
		"finished_product_component_before_g":       beforeG,
		"finished_product_component_after_g":        afterG,
		"finished_product_component_change_g":       -need.DeductG,
		"finished_product_component_component_spec": specG,
		"finished_product_component_warehouse":      warehouse,
	})
}

func finishedComponentConsumptionWarehouse(allocations []customerProcessingFinishedBatchAllocation) (string, error) {
	hasCustomerProcessing, hasUnreserved := false, false
	for _, allocation := range allocations {
		if allocation.ReservationID > 0 {
			hasCustomerProcessing = true
		} else {
			hasUnreserved = true
		}
	}
	if hasCustomerProcessing && hasUnreserved {
		return "", fmt.Errorf("finished-product consumption mixes customer processing WIP and unreserved stock")
	}
	if hasCustomerProcessing {
		return stockdomain.WarehouseWIP, nil
	}
	return stockdomain.WarehouseFinishedGoods, nil
}

func finishedComponentTotalG(specG, units, looseG int64) int64 {
	if specG <= 0 {
		return looseG
	}
	return units*specG + looseG
}

func deductFinishedComponentQty(specG, units, looseG, deductG int64) (int64, int64, error) {
	if deductG <= 0 {
		return units, looseG, nil
	}
	if specG <= 0 {
		if looseG < deductG {
			return 0, 0, fmt.Errorf("finished product stock insufficient")
		}
		return units, looseG - deductG, nil
	}
	after, deductedG, gapG, err := invDeduct(specG, InvQty{Units: units, LooseG: looseG}, deductG)
	if err != nil {
		return 0, 0, err
	}
	if gapG > 0 || deductedG != deductG {
		return 0, 0, fmt.Errorf("finished product stock insufficient")
	}
	return after.Units, after.LooseG, nil
}

func aggregateMaterialConsumptionNeeds(needs []materialConsumptionNeed) []materialConsumptionNeed {
	byMaterial := map[int64]materialConsumptionNeed{}
	order := make([]int64, 0, len(needs))
	for _, need := range needs {
		if need.MaterialID <= 0 {
			continue
		}
		row, ok := byMaterial[need.MaterialID]
		if !ok {
			row = materialConsumptionNeed{
				MaterialID:         need.MaterialID,
				MaterialName:       need.MaterialName,
				Unit:               need.Unit,
				RatioPct:           need.RatioPct,
				Source:             need.Source,
				ComponentType:      need.ComponentType,
				ComponentProductID: need.ComponentProductID,
				ComponentSpecG:     need.ComponentSpecG,
				ConsumeUnit:        need.ConsumeUnit,
				QtyPerUnit:         need.QtyPerUnit,
			}
			order = append(order, need.MaterialID)
		}
		if row.MaterialName == "" {
			row.MaterialName = need.MaterialName
		}
		if row.Unit == "" {
			row.Unit = need.Unit
		}
		row.Qty += need.Qty
		row.QtyDecimal += need.QtyDecimal
		row.DeductG += need.DeductG
		row.DeductUnits += need.DeductUnits
		byMaterial[need.MaterialID] = row
	}
	out := make([]materialConsumptionNeed, 0, len(order))
	for _, materialID := range order {
		out = append(out, byMaterial[materialID])
	}
	return out
}

func reservedWIPGForMaterialTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64) (int64, error) {
	var reservedG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,wr.reserved_g-wr.consumed_g-wr.returned_g)),0)::bigint
		FROM %s.work_order_material_reservations wr
		WHERE wr.material_id=$1 AND wr.status='reserved'
		  AND NOT EXISTS (
			SELECT 1 FROM %s.customer_processing_material_reservations cpr
			WHERE cpr.work_order_id=wr.work_order_id AND cpr.material_id=wr.material_id AND cpr.status='reserved'
		  )
	`, schema, schema), materialID).Scan(&reservedG)
	if err != nil {
		if strings.Contains(err.Error(), "work_order_material_reservations") {
			return 0, nil
		}
		return 0, err
	}
	var customerReservedG int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint
		FROM %s.customer_processing_material_reservations r
		LEFT JOIN %s.warehouses w ON w.code=r.source_warehouse_code AND w.active=true
		WHERE r.material_id=$1 AND r.component_type<>'finished_product' AND r.status='reserved'
		  AND (r.material_batch_id>0 OR w.kind='wip')
	`, schema, schema), materialID).Scan(&customerReservedG)
	if err != nil {
		if strings.Contains(err.Error(), "customer_processing_material_reservations") {
			return reservedG, nil
		}
		return 0, err
	}
	reservedG += customerReservedG
	return reservedG, nil
}

func createMaterialReservationsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID, runningItemID int64, needs []materialConsumptionNeed) error {
	for _, need := range aggregateMaterialConsumptionNeeds(needs) {
		if need.Source == "finished_product" || need.ComponentType == "finished_product" {
			continue
		}
		if need.MaterialID <= 0 || (need.DeductG <= 0 && need.DeductUnits <= 0) {
			continue
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_order_material_reservations
			SET running_item_id=$2,updated_at=now()
			WHERE work_order_id=$1
			  AND material_id=$3
			  AND status='reserved'
			  AND running_item_id IN (0,$2)
		`, schema), workOrderID, runningItemID, need.MaterialID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservations(
				work_order_id,running_item_id,material_id,material_name,unit,
				required_g,required_units,reserved_g,reserved_units,status,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$6,$7,'reserved',now(),now())
		`, schema), workOrderID, runningItemID, need.MaterialID, need.MaterialName, need.Unit, need.DeductG, need.DeductUnits); err != nil {
			return err
		}
	}
	return nil
}

func updateMaterialReservationConsumedTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, materialID, consumedG, consumedUnits int64) error {
	if runningItemID <= 0 || materialID <= 0 || (consumedG <= 0 && consumedUnits <= 0) {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET consumed_g=LEAST(reserved_g, consumed_g+$3),
		    consumed_units=LEAST(reserved_units, consumed_units+$4),
		    updated_at=now()
		WHERE running_item_id=$1 AND material_id=$2 AND status='reserved'
	`, schema), runningItemID, materialID, consumedG, consumedUnits)
	if err != nil && strings.Contains(err.Error(), "work_order_material_reservations") {
		return nil
	}
	return err
}

func releaseMaterialReservationsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET status='released',
		    returned_g=GREATEST(0,reserved_g-consumed_g),
		    returned_units=GREATEST(0,reserved_units-consumed_units),
		    updated_at=now()
		WHERE running_item_id=$1 AND status='reserved'
	`, schema), runningItemID)
	if err != nil && strings.Contains(err.Error(), "work_order_material_reservations") {
		return nil
	}
	return err
}

func completeMaterialReservationsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET status='consumed',
		    returned_g=GREATEST(0,reserved_g-consumed_g),
		    returned_units=GREATEST(0,reserved_units-consumed_units),
		    updated_at=now()
		WHERE running_item_id=$1 AND status='reserved'
	`, schema), runningItemID)
	if err != nil && strings.Contains(err.Error(), "work_order_material_reservations") {
		return nil
	}
	return err
}

func materialBatchAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, deductG int64) ([]stockdomain.BatchAllocation, error) {
	if deductG <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,l.qty_g,COALESCE(b.quality_status,'unchecked')
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
		if strings.Contains(err.Error(), "material_batches") || strings.Contains(err.Error(), "material_batch_locations") || strings.Contains(err.Error(), "quality_status") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	available := make([]stockdomain.BatchAvailability, 0)
	var frozenG int64
	for rows.Next() {
		var batch stockdomain.BatchAvailability
		var qualityStatus string
		if err := rows.Scan(&batch.BatchID, &batch.BatchCode, &batch.AvailableG, &qualityStatus); err != nil {
			return nil, err
		}
		if qualityStatus == "hold" || qualityStatus == "reject" {
			frozenG += batch.AvailableG
			continue
		}
		available = append(available, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(available) == 0 {
		if frozenG > 0 {
			return nil, fmt.Errorf("WIP stock blocked by quality status for material %d", materialID)
		}
		return nil, fmt.Errorf("WIP stock insufficient for material %d", materialID)
	}
	allocations, err := stockdomain.AllocateFIFO(available, deductG)
	if err != nil {
		if frozenG > 0 {
			return nil, fmt.Errorf("WIP stock blocked by quality status for material %d", materialID)
		}
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
