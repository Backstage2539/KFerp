package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
)

var errMallBOMSpecIdentity = errors.New("mall BOM spec identity invalid")

type portalMallOutputIdentity struct {
	Canonical     bool
	ProductID     int64
	BomSpecID     int64
	BomVariantID  int64
	ProductName   string
	ProductKind   string
	SpecName      string
	InventoryUnit string
	DefaultPrice  float64
}

func mallBOMSpecIdentityError(message string) error {
	return fmt.Errorf("%w: %s", errMallBOMSpecIdentity, message)
}

func validateMallOrderItemIdentity(line mallOrderLine, item customerportalapp.MallOrderItemCommand) error {
	if line.BomSpecID > 0 {
		if item.ProductID <= 0 {
			return mallBOMSpecIdentityError("product_id required after BOM spec cutover")
		}
		if item.BomSpecID <= 0 {
			return mallBOMSpecIdentityError("bom_spec_id required after BOM spec cutover")
		}
		if item.ProductID != line.ProductID || item.BomSpecID != line.BomSpecID || (item.BomVariantID > 0 && item.BomVariantID != line.BomVariantID) {
			return mallBOMSpecIdentityError("mall BOM spec selection is no longer current")
		}
		return nil
	}
	if item.BomSpecID > 0 || item.BomVariantID > 0 {
		return mallBOMSpecIdentityError("BOM spec identity requires cutover product")
	}
	if item.ProductID > 0 && item.ProductID != line.ProductID {
		return mallBOMSpecIdentityError("mall product identity mismatch")
	}
	return nil
}

func (r Repository) resolveMallOutputIdentity(
	ctx context.Context,
	q portalQueryRower,
	productID, bomSpecID, bomVariantID int64,
) (portalMallOutputIdentity, error) {
	identity := portalMallOutputIdentity{ProductID: productID}
	if productID <= 0 || q == nil {
		return identity, mallBOMSpecIdentityError("mall product unavailable")
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT p.id,COALESCE(p.name,''),COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),COALESCE(p.default_price,0)::float8
		FROM %s.products p
		WHERE p.id=$1 AND p.active=true AND %s
	`, r.schema, mallProductPublicCatalogSQL("p")), productID).Scan(
		&identity.ProductID, &identity.ProductName, &identity.ProductKind, &identity.DefaultPrice,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return portalMallOutputIdentity{}, mallBOMSpecIdentityError("mall product unavailable")
	}
	if err != nil {
		return portalMallOutputIdentity{}, err
	}

	if bomSpecID <= 0 {
		return portalMallOutputIdentity{}, mallBOMSpecIdentityError("product_bom_spec_not_configured")
	}
	identity.Canonical = true
	err = q.QueryRow(ctx, fmt.Sprintf(`
		SELECT spec.id,variant.id,
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit)
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
		  AND ($3::bigint=0 OR variant.id=$3)
		ORDER BY variant.id
		LIMIT 1
	`, r.schema), productID, bomSpecID, bomVariantID).Scan(
		&identity.BomSpecID, &identity.BomVariantID, &identity.SpecName, &identity.InventoryUnit,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return portalMallOutputIdentity{}, mallBOMSpecIdentityError("BOM spec is not in the current default published BOM")
	}
	if err != nil {
		return portalMallOutputIdentity{}, err
	}
	identity.SpecName = strings.TrimSpace(identity.SpecName)
	identity.InventoryUnit = strings.TrimSpace(identity.InventoryUnit)
	if identity.SpecName == "" || identity.InventoryUnit == "" {
		return portalMallOutputIdentity{}, mallBOMSpecIdentityError("BOM spec snapshot is incomplete")
	}
	return identity, nil
}

func (r Repository) listMallProductOptions(ctx context.Context) ([]customerportalapp.MallProductOption, error) {
	out := make([]customerportalapp.MallProductOption, 0)
	canonicalRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT p.id,COALESCE(p.name,''),COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),COALESCE(p.default_price,0)::float8,
		       spec.id,variant.id,COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit)
		FROM %[1]s.product_bom_spec_authorities authority
		JOIN %[1]s.products p ON p.id=authority.product_id AND p.active=true
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=p.id AND binding.is_default=true
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE authority.configured=true AND %s
		ORDER BY p.name,variant.is_default DESC,variant.sort_order,spec.spec_key,spec.id
	`, r.schema, mallProductPublicCatalogSQL("p")))
	if err != nil {
		return nil, err
	}
	for canonicalRows.Next() {
		var row customerportalapp.MallProductOption
		if err := canonicalRows.Scan(
			&row.ProductID, &row.Name, &row.ProductKind, &row.DefaultPrice,
			&row.BomSpecID, &row.BomVariantID, &row.SpecName, &row.InventoryUnit,
		); err != nil {
			canonicalRows.Close()
			return nil, err
		}
		row.ID = row.BomSpecID
		row.Name = strings.TrimSpace(row.Name + " · " + row.SpecName)
		out = append(out, row)
	}
	if err := canonicalRows.Err(); err != nil {
		canonicalRows.Close()
		return nil, err
	}
	canonicalRows.Close()
	return out, nil
}
