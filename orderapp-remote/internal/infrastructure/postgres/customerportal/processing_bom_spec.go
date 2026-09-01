package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"

	"github.com/jackc/pgx/v5"
)

type portalProcessingOutputIdentity struct {
	Canonical     bool
	ProductID     int64
	BomID         int64
	BomVersionID  int64
	BomVersionNo  string
	BomSpecID     int64
	BomVariantID  int64
	ProductName   string
	SpecName      string
	InventoryUnit string
	IsDefault     bool
	SortOrder     int
}

type portalProcessingMaterialSnapshotRow struct {
	MaterialID                int64   `json:"material_id"`
	MaterialName              string  `json:"material_name"`
	Unit                      string  `json:"unit"`
	RatioPct                  float64 `json:"ratio_pct,omitempty"`
	MaterialLossRate          float64 `json:"material_loss_rate,omitempty"`
	LossCalculationMode       string  `json:"loss_calculation_mode,omitempty"`
	InputIncludesMaterialLoss bool    `json:"input_includes_material_loss,omitempty"`
	Source                    string  `json:"source"`
	ComponentType             string  `json:"component_type,omitempty"`
	ComponentProductID        int64   `json:"component_product_id,omitempty"`
	ComponentBomSpecID        int64   `json:"component_bom_spec_id,omitempty"`
	ComponentSpecG            int64   `json:"component_spec_g,omitempty"`
	ConsumeUnit               string  `json:"consume_unit,omitempty"`
	QtyPerUnit                float64 `json:"qty_per_unit,omitempty"`
	OutputQty                 float64 `json:"output_qty,omitempty"`
	OutputUnit                string  `json:"output_unit,omitempty"`
}

func (r Repository) resolveProcessingOutputIdentityTx(ctx context.Context, tx pgx.Tx, customerID int64, item customerportalapp.ProcessingRequestItemCommand) (portalProcessingOutputIdentity, error) {
	if err := r.ensureProcessingTargetProductTx(ctx, tx, customerID, item.ProductID); err != nil {
		return portalProcessingOutputIdentity{}, err
	}
	if item.BomSpecID <= 0 {
		return portalProcessingOutputIdentity{}, fmt.Errorf("product_bom_spec_not_configured")
	}
	identity := portalProcessingOutputIdentity{Canonical: true}
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT binding.output_id,binding.bom_id,version.id,COALESCE(version.version_no,''),
		       spec.id,variant.id,COALESCE(p.name,''),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit),
		       variant.is_default,variant.sort_order
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		JOIN %[1]s.products p ON p.id=binding.output_id AND p.active=true
		WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
		  AND ($3::bigint=0 OR variant.id=$3)
		ORDER BY variant.id
		LIMIT 1
	`, r.schema), item.ProductID, item.BomSpecID, item.BomVariantID).Scan(
		&identity.ProductID, &identity.BomID, &identity.BomVersionID, &identity.BomVersionNo,
		&identity.BomSpecID, &identity.BomVariantID, &identity.ProductName,
		&identity.SpecName, &identity.InventoryUnit, &identity.IsDefault, &identity.SortOrder,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return portalProcessingOutputIdentity{}, fmt.Errorf("BOM spec is not published for product")
	}
	return identity, err
}

func (r Repository) resolveProcessingBOMSpecTargetTx(ctx context.Context, tx pgx.Tx, identity portalProcessingOutputIdentity, qty int64) (postgresproduction.CustomerProcessingResolvedItem, error) {
	if !identity.Canonical || qty <= 0 {
		return postgresproduction.CustomerProcessingResolvedItem{}, fmt.Errorf("invalid BOM spec processing target")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT item.material_id,COALESCE(material.name,''),COALESCE(NULLIF(material.unit,''),'g'),
		       COALESCE(NULLIF(item.component_type,''),'material'),COALESCE(item.component_product_id,0),
		       COALESCE(item.component_bom_spec_id,0),COALESCE(item.component_spec_g,0),
		       COALESCE(NULLIF(item.consume_unit,''),'ratio_pct'),COALESCE(item.qty_per_unit,0)::float8,
		       COALESCE(item.ratio_pct,0)::float8,COALESCE(item.material_loss_rate,0)::float8
		FROM %s.production_bom_version_items item
		LEFT JOIN %s.materials material ON material.id=item.material_id
		WHERE item.version_id=$1 AND item.variant_id=$2
		ORDER BY item.id
	`, r.schema, r.schema), identity.BomVersionID, identity.BomVariantID)
	if err != nil {
		return postgresproduction.CustomerProcessingResolvedItem{}, err
	}
	defer rows.Close()

	needs := make([]postgresproduction.CustomerProcessingNeed, 0)
	snapshot := make([]portalProcessingMaterialSnapshotRow, 0)
	for rows.Next() {
		var row portalProcessingMaterialSnapshotRow
		if err := rows.Scan(
			&row.MaterialID, &row.MaterialName, &row.Unit, &row.ComponentType, &row.ComponentProductID,
			&row.ComponentBomSpecID, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit,
			&row.RatioPct, &row.MaterialLossRate,
		); err != nil {
			return postgresproduction.CustomerProcessingResolvedItem{}, err
		}
		row.ComponentType = normalizePortalProcessingComponentType(row.ComponentType)
		row.ConsumeUnit = strings.TrimSpace(row.ConsumeUnit)
		row.Source = "bom"
		row.OutputQty = 1
		row.OutputUnit = identity.InventoryUnit
		row.LossCalculationMode = "yield_denominator"
		row.InputIncludesMaterialLoss = row.MaterialLossRate > 0 && row.ConsumeUnit == "ratio_pct"
		if row.ComponentType != "material" {
			return postgresproduction.CustomerProcessingResolvedItem{}, fmt.Errorf("BOM spec customer processing product component awaits canonical component identity")
		}
		if row.MaterialID <= 0 || strings.TrimSpace(row.MaterialName) == "" {
			return postgresproduction.CustomerProcessingResolvedItem{}, fmt.Errorf("production BOM version has invalid material line: %s", identity.ProductName)
		}
		requiredG, requiredUnits, err := portalProcessingFixedMaterialNeed(row, qty)
		if err != nil {
			return postgresproduction.CustomerProcessingResolvedItem{}, err
		}
		if requiredG <= 0 && requiredUnits <= 0 {
			return postgresproduction.CustomerProcessingResolvedItem{}, fmt.Errorf("production BOM version has invalid material line: %s", identity.ProductName)
		}
		needs = append(needs, postgresproduction.CustomerProcessingNeed{
			MaterialID: row.MaterialID, MaterialName: strings.TrimSpace(row.MaterialName), Unit: strings.TrimSpace(row.Unit),
			RequiredG: requiredG, RequiredUnits: requiredUnits, ComponentType: "material",
		})
		snapshot = append(snapshot, row)
	}
	if err := rows.Err(); err != nil {
		return postgresproduction.CustomerProcessingResolvedItem{}, err
	}
	if len(needs) == 0 {
		return postgresproduction.CustomerProcessingResolvedItem{}, fmt.Errorf("production BOM version has no material lines: %s", identity.ProductName)
	}
	sort.SliceStable(needs, func(i, j int) bool { return needs[i].MaterialID < needs[j].MaterialID })
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return postgresproduction.CustomerProcessingResolvedItem{}, err
	}
	return postgresproduction.CustomerProcessingResolvedItem{
		ProductID: identity.ProductID, ParentProductID: identity.ProductID, ProductName: identity.ProductName,
		Qty: qty, BomVersionID: identity.BomVersionID, BomVersionNo: identity.BomVersionNo,
		BomSourceProductID: identity.ProductID, MaterialSnapshot: string(raw), Materials: needs,
	}, nil
}

func portalProcessingFixedMaterialNeed(row portalProcessingMaterialSnapshotRow, outputQty int64) (int64, int64, error) {
	consumeUnit := strings.ToLower(strings.TrimSpace(row.ConsumeUnit))
	if consumeUnit == "ratio_pct" {
		return 0, 0, fmt.Errorf("ratio BOM spec processing requires production quantity basis")
	}
	if row.QtyPerUnit <= 0 || outputQty <= 0 {
		return 0, 0, nil
	}
	materialFactor := portalProcessingWeightUnitGrams(row.Unit)
	if materialFactor > 0 {
		consumeFactor := portalProcessingWeightUnitGrams(consumeUnit)
		if consumeFactor <= 0 {
			consumeFactor = materialFactor
		}
		grams := row.QtyPerUnit * float64(outputQty) * consumeFactor
		return int64(math.Ceil(grams)), 0, nil
	}
	return 0, int64(math.Ceil(row.QtyPerUnit * float64(outputQty))), nil
}

func portalProcessingWeightUnitGrams(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "gram", "grams", "克":
		return 1
	case "kg", "kilogram", "kilograms", "千克", "公斤":
		return 1000
	case "lb", "lbs", "pound", "pounds", "磅":
		return 453.59237
	case "oz", "ounce", "ounces", "盎司":
		return 28.349523125
	default:
		return 0
	}
}

func normalizePortalProcessingComponentType(value string) string {
	switch strings.TrimSpace(value) {
	case "product", "finished_product":
		return "finished_product"
	default:
		return "material"
	}
}

func (r Repository) ListProcessingCatalogTargets(ctx context.Context, customerID int64, productIDs []int64) ([]customerportalapp.ProcessingCatalogTarget, error) {
	if customerID <= 0 || len(productIDs) == 0 {
		return []customerportalapp.ProcessingCatalogTarget{}, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	out := make([]customerportalapp.ProcessingCatalogTarget, 0)
	for _, productID := range productIDs {
		if productID <= 0 {
			continue
		}
		{
			rows, err := tx.Query(ctx, fmt.Sprintf(`
				SELECT binding.output_id,spec.id,variant.id,
				       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
				       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit),variant.is_default,variant.sort_order
				FROM %[1]s.production_bom_output_bindings binding
				JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
				JOIN %[1]s.production_bom_versions version
				  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
				JOIN %[1]s.production_bom_specs spec ON spec.bom_id=binding.bom_id
				JOIN %[1]s.production_bom_version_variants variant
				  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
				WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
				  AND EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items item WHERE item.version_id=version.id AND item.variant_id=variant.id)
				ORDER BY variant.is_default DESC,variant.sort_order,spec.spec_key,spec.id
			`, r.schema), productID)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var row customerportalapp.ProcessingCatalogTarget
				if err := rows.Scan(&row.ProductID, &row.BomSpecID, &row.BomVariantID, &row.SpecName, &row.InventoryUnit, &row.IsDefault, &row.SortOrder); err != nil {
					rows.Close()
					return nil, err
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
			continue
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
