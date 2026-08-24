package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	catalogdomain "orderapp/internal/domain/catalog"

	"github.com/jackc/pgx/v5"
)

type portalProductOrderIdentity struct {
	Canonical     bool
	ProductID     int64
	BomSpecID     int64
	BomVariantID  int64
	ProductName   string
	ProductKind   string
	SpecName      string
	InventoryUnit string
}

func (r Repository) productOrderBOMSpecSchemaAvailable(ctx context.Context, q portalQueryRower) bool {
	for _, relation := range []string{
		"product_bom_spec_migrations",
		"production_bom_output_bindings",
		"production_boms",
		"production_bom_versions",
		"production_bom_specs",
		"production_bom_version_variants",
	} {
		if !portalRelationExists(ctx, q, fmt.Sprintf("%s.%s", r.schema, relation)) {
			return false
		}
	}
	return true
}

func (r Repository) listProductOrderBOMSpecOptions(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProductSummary, error) {
	if limit <= 0 || !r.productOrderBOMSpecSchemaAvailable(ctx, r.pool) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT p.id,spec.id,variant.id,COALESCE(p.name,''),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit),variant.is_default,variant.sort_order,
		       COALESCE(p.roast_level,''),COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
		       COALESCE(p.drip_bag_grams,10)::float8,COALESCE(p.drip_box_bag_count,10),
		       to_char(COALESCE(p.default_price,0), 'FM999999990.00'),
		       to_char(COALESCE(p.retail_price_100g,0), 'FM999999990.00'),
		       to_char(COALESCE(p.retail_price_200g,0), 'FM999999990.00'),
		       to_char(COALESCE(p.retail_price_227g,0), 'FM999999990.00'),
		       to_char(COALESCE(p.retail_price_250g,0), 'FM999999990.00')
		FROM %[1]s.product_bom_spec_migrations migration
		JOIN %[1]s.products p ON p.id=migration.product_id AND p.active=true
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=p.id AND binding.is_default=true
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
		JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id
		JOIN %[1]s.production_bom_specs spec ON spec.id=variant.bom_spec_id AND spec.bom_id=bom.id
		WHERE migration.state='cutover' AND %[2]s
		ORDER BY p.name,p.id,variant.sort_order,variant.id
		LIMIT $2
	`, r.schema, portalProductVisibleToCustomerAliasSQL(r.schema+".products", "p", "$1")), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProductSummary, 0)
	for rows.Next() {
		var row customerportalapp.ProductSummary
		if err := rows.Scan(
			&row.ID, &row.BomSpecID, &row.BomVariantID, &row.Name, &row.SpecName, &row.InventoryUnit, &row.IsDefault, &row.SortOrder,
			&row.RoastLevel, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount,
			&row.DefaultPrice, &row.RetailPrice100, &row.RetailPrice200, &row.RetailPrice227, &row.RetailPrice250,
		); err != nil {
			return nil, err
		}
		row.ProductKind = catalogdomain.NormalizeProductKind(row.ProductKind)
		row.MigrationState = "cutover"
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) resolveProductOrderIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	customerID int64,
	cmd customerportalapp.CreateFulfillmentOrderCommand,
) (portalProductOrderIdentity, error) {
	identity := portalProductOrderIdentity{ProductID: cmd.ProductID}
	if cmd.ProductID <= 0 {
		return identity, fmt.Errorf("product unavailable")
	}
	if !r.productOrderBOMSpecSchemaAvailable(ctx, tx) {
		if cmd.BomSpecID > 0 || cmd.BomVariantID > 0 {
			return identity, fmt.Errorf("BOM spec migration unavailable")
		}
		return identity, nil
	}

	if portalRelationExists(ctx, tx, fmt.Sprintf("%s.legacy_child_sku_bom_spec_mappings", r.schema)) {
		var mappedState string
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(migration.state,'legacy')
			FROM %s.legacy_child_sku_bom_spec_mappings mapping
			LEFT JOIN %s.product_bom_spec_migrations migration ON migration.product_id=mapping.parent_product_id
			WHERE mapping.legacy_child_product_id=$1
		`, r.schema, r.schema), cmd.ProductID).Scan(&mappedState)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return identity, err
		}
		if err == nil && mappedState == "cutover" {
			return identity, fmt.Errorf("legacy child SKU write rejected after BOM spec cutover")
		}
	}

	state := "legacy"
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_migrations WHERE product_id=$1 FOR SHARE`, r.schema), cmd.ProductID).Scan(&state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return identity, err
	}
	if state != "cutover" {
		if cmd.BomSpecID > 0 || cmd.BomVariantID > 0 {
			return identity, fmt.Errorf("BOM spec identity requires cutover product")
		}
		return identity, nil
	}
	if cmd.BomSpecID <= 0 {
		return identity, fmt.Errorf("BOM spec identity required after cutover")
	}

	identity.Canonical = true
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT p.id,spec.id,variant.id,COALESCE(p.name,''),
		       COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit)
		FROM %[1]s.products p
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=p.id AND binding.is_default=true
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=bom.id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE p.id=$1 AND p.active=true AND %[2]s AND ($3::bigint=0 OR variant.id=$3)
		ORDER BY variant.id
		LIMIT 1
	`, r.schema, portalProductVisibleToCustomerAliasSQL(r.schema+".products", "p", "$4")),
		cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, customerID,
	).Scan(
		&identity.ProductID, &identity.BomSpecID, &identity.BomVariantID, &identity.ProductName,
		&identity.ProductKind, &identity.SpecName, &identity.InventoryUnit,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return portalProductOrderIdentity{}, fmt.Errorf("BOM spec is not in the current default published BOM")
	}
	if err != nil {
		return portalProductOrderIdentity{}, err
	}
	identity.SpecName = strings.TrimSpace(identity.SpecName)
	identity.InventoryUnit = strings.TrimSpace(identity.InventoryUnit)
	if identity.SpecName == "" || identity.InventoryUnit == "" {
		return portalProductOrderIdentity{}, fmt.Errorf("BOM spec snapshot is incomplete")
	}
	if !strings.EqualFold(strings.TrimSpace(cmd.InventoryUnit), identity.InventoryUnit) {
		return portalProductOrderIdentity{}, fmt.Errorf("inventory_unit does not match current BOM spec")
	}
	return identity, nil
}
