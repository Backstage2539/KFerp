package productspecmigration

import (
	"context"
	"errors"
	"fmt"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ResolveIdentity(ctx context.Context, cmd productspecmigrationapp.ResolveIdentityCommand) (productspecmigrationapp.BusinessIdentity, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	defer tx.Rollback(ctx)
	identity, err := r.resolveIdentityTx(ctx, tx, cmd)
	if err != nil {
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	return identity, nil
}

func (r Repository) resolveIdentityTx(ctx context.Context, tx pgx.Tx, cmd productspecmigrationapp.ResolveIdentityCommand) (productspecmigrationapp.BusinessIdentity, error) {
	var parentID int64
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(parent_product_id,0),COALESCE(active,false) FROM %s.products WHERE id=$1 FOR SHARE`, r.schema), cmd.ProductID).Scan(&parentID, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrProductRequired
		}
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	if !active || parentID > 0 {
		return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrLegacyWriteRejected
	}
	var configured bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT configured FROM %s.product_bom_spec_authorities WHERE product_id=$1`, r.schema), cmd.ProductID).Scan(&configured); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrProductBOMSpecNotConfigured
		}
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	if !configured {
		return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrProductBOMSpecNotConfigured
	}
	if cmd.BomSpecID == nil || *cmd.BomSpecID <= 0 {
		return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrBomSpecRequired
	}
	identity := productspecmigrationapp.BusinessIdentity{
		ProductID:            cmd.ProductID,
		BomSpecID:            cmd.BomSpecID,
		BomVariantID:         cmd.BomVariantID,
		MigrationState:       productspecmigrationapp.StateCutover,
		SpecIdentityMode:     productspecmigrationapp.SpecIdentityModeBOMSpec,
		BomSpecAuthoritative: true,
		LegacyCompatible:     false,
	}
	variantID, err := r.publishedVariantForSpecTx(ctx, tx, cmd.ProductID, *cmd.BomSpecID)
	if err != nil {
		return productspecmigrationapp.BusinessIdentity{}, err
	}
	if variantID <= 0 {
		return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrBomSpecUnavailable
	}
	if cmd.BomVariantID != nil && *cmd.BomVariantID > 0 && *cmd.BomVariantID != variantID {
		return productspecmigrationapp.BusinessIdentity{}, productspecmigrationapp.ErrBomSpecUnavailable
	}
	identity.BomVariantID = &variantID
	return identity, nil
}

func (r Repository) publishedVariantForSpecTx(ctx context.Context, tx pgx.Tx, productID, bomSpecID int64) (int64, error) {
	hasSpecs, err := relationExistsTx(ctx, tx, r.schema, "production_bom_specs")
	if err != nil || !hasSpecs {
		return 0, err
	}
	hasVariants, err := relationExistsTx(ctx, tx, r.schema, "production_bom_version_variants")
	if err != nil || !hasVariants {
		return 0, err
	}
	var variantID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT variant.id
		FROM %s.production_bom_output_bindings binding
		JOIN %s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=binding.bom_id
		JOIN %s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
		ORDER BY variant.is_default DESC,variant.sort_order,variant.id LIMIT 1
	`, r.schema, r.schema, r.schema, r.schema), productID, bomSpecID).Scan(&variantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return variantID, err
}

func (r Repository) ListOptions(ctx context.Context, productID int64) ([]productspecmigrationapp.ProductSpecOption, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, relation := range []string{
		"production_bom_output_bindings",
		"production_bom_versions",
		"production_bom_specs",
		"production_bom_version_variants",
	} {
		exists, err := relationExistsTx(ctx, tx, r.schema, relation)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return []productspecmigrationapp.ProductSpecOption{}, nil
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT binding.output_id,
		       0::bigint,
		       binding.bom_id,
		       version.id,
		       COALESCE(version.version_no,''),
		       spec.id,
		       variant.id,
		       COALESCE(spec.code,''),
		       COALESCE(spec.barcode,''),
		       spec.spec_key,
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit),
		       variant.is_default,
		       variant.sort_order
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		JOIN %[1]s.production_bom_specs spec
		  ON spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id
		 AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product'
		  AND binding.output_id=$1
		  AND binding.is_default=true
		ORDER BY variant.sort_order,spec.spec_key,spec.id,variant.id
	`, r.schema), productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]productspecmigrationapp.ProductSpecOption, 0)
	for rows.Next() {
		var option productspecmigrationapp.ProductSpecOption
		if err := rows.Scan(
			&option.ParentProductID,
			&option.LegacyChildProductID,
			&option.BomID,
			&option.BomVersionID,
			&option.BomVersionNo,
			&option.BomSpecID,
			&option.BomVariantID,
			&option.SpecCode,
			&option.Barcode,
			&option.SpecKey,
			&option.SpecName,
			&option.InventoryUnit,
			&option.IsDefault,
			&option.SortOrder,
		); err != nil {
			return nil, err
		}
		option.Published = true
		option.MigrationState = productspecmigrationapp.StateCutover
		option.SpecIdentityMode = productspecmigrationapp.SpecIdentityModeBOMSpec
		option.BomSpecAuthoritative = true
		option.WriteProductID = productID
		option.WriteBomSpecID = option.BomSpecID
		option.WriteBomVariantID = option.BomVariantID
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, productspecmigrationapp.ErrProductBOMSpecNotConfigured
	}
	return options, nil
}
