package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	app "orderapp/internal/application/customerfulfillment"

	"github.com/jackc/pgx/v5"
)

type customerFulfillmentBOMSpecIdentity struct {
	ProductID              int64
	BomSpecID              int64
	BomVariantID           int64
	BomSpecKey             string
	BomSpecName            string
	InventoryUnit          string
	SpecCode               string
	ProductName            string
	ProductCode            string
	ProductKind            string
	LegacyPricingProductID int64
	LegacySpecG            int64
	LegacySalesUnit        string
}

func resolveCustomerFulfillmentBOMSpecIdentityTx(ctx context.Context, q miniDirectShipQuerier, schema string, productID, bomSpecID, bomVariantID int64) (customerFulfillmentBOMSpecIdentity, error) {
	if productID <= 0 {
		return customerFulfillmentBOMSpecIdentity{}, nil
	}
	var configured bool
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT authority.configured
		FROM %[1]s.product_bom_spec_authorities authority
		JOIN %[1]s.products product ON product.id=authority.product_id AND product.active=true
		WHERE authority.product_id=$1
	`, schema), productID).Scan(&configured)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerFulfillmentBOMSpecIdentity{}, fmt.Errorf("product_bom_spec_not_configured")
	}
	if err != nil {
		return customerFulfillmentBOMSpecIdentity{}, err
	}
	if !configured {
		return customerFulfillmentBOMSpecIdentity{}, fmt.Errorf("product_bom_spec_not_configured")
	}
	if bomSpecID <= 0 {
		return customerFulfillmentBOMSpecIdentity{}, fmt.Errorf("bom_spec_id required for product output")
	}

	var identity customerFulfillmentBOMSpecIdentity
	err = q.QueryRow(ctx, fmt.Sprintf(`
		SELECT parent.id,spec.id,variant.id,spec.spec_key,
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),''),
		       spec.code,
		       parent.name,COALESCE(NULLIF(parent.sku_code,''),'SKU-' || parent.id::text),
		       COALESCE(NULLIF(parent.product_kind,''),'roasted_bean'),
		       0::bigint,0::bigint,''::text
		FROM %[1]s.product_bom_spec_authorities authority
		JOIN %[1]s.products parent ON parent.id=authority.product_id AND parent.active=true
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=parent.id AND binding.is_default=true
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE authority.product_id=$1 AND authority.configured=true
		  AND ($3::bigint=0 OR variant.id=$3)
		ORDER BY variant.id
		LIMIT 1
	`, schema), productID, bomSpecID, bomVariantID).Scan(
		&identity.ProductID, &identity.BomSpecID, &identity.BomVariantID,
		&identity.BomSpecKey, &identity.BomSpecName, &identity.InventoryUnit,
		&identity.SpecCode, &identity.ProductName, &identity.ProductCode, &identity.ProductKind,
		&identity.LegacyPricingProductID, &identity.LegacySpecG, &identity.LegacySalesUnit,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerFulfillmentBOMSpecIdentity{}, fmt.Errorf("BOM spec is not published for product")
	}
	return identity, err
}

func customerFulfillmentBOMSpecPriceSource(raw string, identity customerFulfillmentBOMSpecIdentity) string {
	source := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &source)
	}
	if source == nil {
		source = map[string]any{}
	}
	source["product_id"] = identity.ProductID
	source["parent_product_id"] = identity.ProductID
	source["bom_spec_id"] = identity.BomSpecID
	source["bom_variant_id"] = identity.BomVariantID
	source["bom_spec_key"] = identity.BomSpecKey
	source["bom_spec_name"] = identity.BomSpecName
	source["inventory_unit"] = identity.InventoryUnit
	source["quantity_basis"] = "sales_spec_count"
	delete(source, "sku_id")
	delete(source, "spec_g")
	if identity.LegacyPricingProductID > 0 {
		source["legacy_pricing_product_id"] = identity.LegacyPricingProductID
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

type customerFulfillmentBOMSpecOptionRow struct {
	ParentProductID        int64
	BomSpecID              int64
	BomVariantID           int64
	BomSpecKey             string
	BomSpecName            string
	InventoryUnit          string
	SpecCode               string
	ParentName             string
	ParentCode             string
	ProductKind            string
	LegacyChildProductID   int64
	LegacyPricingSpecG     int64
	LegacyPricingSalesUnit string
}

func (r *Repository) replaceCutoverCustomerSKUOptions(ctx context.Context, customerID int64, options []app.CustomerSKUOption) ([]app.CustomerSKUOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT parent.id,spec.id,variant.id,spec.spec_key,
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),''),
		       spec.code,parent.name,COALESCE(NULLIF(parent.sku_code,''),'SKU-' || parent.id::text),
		       COALESCE(NULLIF(parent.product_kind,''),'roasted_bean'),
		       0::bigint,0::bigint,''::text
		FROM %[1]s.product_bom_spec_authorities authority
		JOIN %[1]s.products parent ON parent.id=authority.product_id AND parent.active=true
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=parent.id AND binding.is_default=true
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id
		JOIN %[1]s.production_bom_specs spec ON spec.id=variant.bom_spec_id AND spec.bom_id=binding.bom_id
		WHERE authority.configured=true
		ORDER BY parent.id,variant.sort_order,variant.id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	canonical := make([]customerFulfillmentBOMSpecOptionRow, 0)
	cutoverProducts := map[int64]struct{}{}
	for rows.Next() {
		var row customerFulfillmentBOMSpecOptionRow
		if err := rows.Scan(
			&row.ParentProductID, &row.BomSpecID, &row.BomVariantID, &row.BomSpecKey,
			&row.BomSpecName, &row.InventoryUnit, &row.SpecCode, &row.ParentName,
			&row.ParentCode, &row.ProductKind, &row.LegacyChildProductID,
			&row.LegacyPricingSpecG, &row.LegacyPricingSalesUnit,
		); err != nil {
			return nil, err
		}
		canonical = append(canonical, row)
		cutoverProducts[row.ParentProductID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(canonical) == 0 {
		return options, nil
	}

	byProduct := map[int64]app.CustomerSKUOption{}
	for _, option := range options {
		if _, found := byProduct[option.ProductID]; !found || option.CustomerProductAliasID > 0 {
			byProduct[option.ProductID] = option
		}
	}
	out := make([]app.CustomerSKUOption, 0, len(options)+len(canonical))
	for _, option := range options {
		if _, cutover := cutoverProducts[option.ProductID]; cutover {
			continue
		}
		out = append(out, option)
	}
	for _, identity := range canonical {
		source, eligible := byProduct[identity.ParentProductID]
		if eligible {
			matching := make([]app.CustomerSKUPriceTier, 0, len(source.Tiers))
			for _, tier := range source.Tiers {
				if tier.BomSpecID == identity.BomSpecID {
					matching = append(matching, tier)
				}
			}
			source.Tiers = matching
		}
		if (!eligible || len(source.Tiers) == 0) && identity.LegacyChildProductID > 0 {
			source, eligible = byProduct[identity.LegacyChildProductID]
		}
		if !eligible {
			continue
		}
		if source.ProductID != identity.ParentProductID {
			source.CustomerProductAliasID = 0
		}
		source.ProductID = identity.ParentProductID
		source.BaseProductID = 0
		source.BomSpecID = identity.BomSpecID
		source.BomVariantID = identity.BomVariantID
		source.BomSpecKey = identity.BomSpecKey
		source.BomSpecName = identity.BomSpecName
		source.InventoryUnit = identity.InventoryUnit
		source.MigrationState = "cutover"
		source.SKUCode = identity.SpecCode
		source.ProductCode = identity.ParentCode
		source.ProductRecordName = identity.ParentName
		source.ProductName = strings.TrimSpace(identity.ParentName + " " + identity.BomSpecName)
		source.ProductKind = identity.ProductKind
		source.Spec = identity.BomSpecName
		source.SalesUnits = []string{identity.InventoryUnit}
		for idx := range source.Tiers {
			source.Tiers[idx].BomSpecID = identity.BomSpecID
			source.Tiers[idx].BomVariantID = identity.BomVariantID
			source.Tiers[idx].SpecG = 0
			source.Tiers[idx].SalesUnit = identity.InventoryUnit
		}
		out = append(out, source)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ProductName != out[j].ProductName {
			return out[i].ProductName < out[j].ProductName
		}
		return out[i].BomSpecID < out[j].BomSpecID
	})
	return out, nil
}
