package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func currentFinishedInventoryBomVariantIDTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	productID, bomSpecID int64,
) (int64, error) {
	var bomVariantID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT variant.id
		FROM %s.production_bom_output_bindings binding
		JOIN %s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		JOIN %s.production_bom_specs spec
		  ON spec.id=$2 AND spec.bom_id=binding.bom_id
		JOIN %s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product'
		  AND binding.output_id=$1
		  AND binding.is_default=true
		ORDER BY variant.id
		LIMIT 1
	`, schema, schema, schema, schema), productID, bomSpecID).Scan(&bomVariantID)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("current published BOM variant is unavailable for product %d specification %d", productID, bomSpecID)
	}
	if err != nil {
		return 0, err
	}
	return bomVariantID, nil
}

// finishedInventoryQtyIdentityTx locks one finished-goods identity. PR-600
// identities include the owning BOM specification; the legacy fallback keeps
// focused repository tests and pre-migration execution semantics intact.
func finishedInventoryQtyIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	productID, bomSpecID, specG int64,
	warehouse string,
) (int64, int64, error) {
	return finishedInventoryQtyIdentityOwnedTx(ctx, tx, schema, productID, bomSpecID, specG, warehouse, 0)
}

func finishedInventoryQtyIdentityOwnedTx(
	ctx context.Context, tx pgx.Tx, schema string,
	productID, bomSpecID, specG int64, warehouse string, ownerCustomerID int64,
) (int64, int64, error) {
	hasBomSpec, err := schemaColumnExistsTx(ctx, tx, schema, "finished_inventory", "bom_spec_id")
	if err != nil {
		return 0, 0, err
	}
	var units, looseG int64
	hasOwner, err := schemaColumnExistsTx(ctx, tx, schema, "finished_inventory", "owner_customer_id")
	if err != nil {
		return 0, 0, err
	}
	if hasBomSpec && hasOwner {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
			WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND warehouse=$4 AND owner_customer_id=$5 FOR UPDATE
		`, schema), productID, bomSpecID, specG, warehouse, ownerCustomerID).Scan(&units, &looseG)
	} else if hasBomSpec {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND warehouse=$4
			FOR UPDATE
		`, schema), productID, bomSpecID, specG, warehouse).Scan(&units, &looseG)
	} else if hasOwner {
		if bomSpecID > 0 {
			return 0, 0, fmt.Errorf("finished inventory BOM specification columns are not available")
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
			WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 AND owner_customer_id=$4 FOR UPDATE
		`, schema), productID, specG, warehouse, ownerCustomerID).Scan(&units, &looseG)
	} else {
		if bomSpecID > 0 {
			return 0, 0, fmt.Errorf("finished inventory BOM specification columns are not available")
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3
			FOR UPDATE
		`, schema), productID, specG, warehouse).Scan(&units, &looseG)
	}
	if err == pgx.ErrNoRows {
		return 0, 0, nil
	}
	return units, looseG, err
}

func upsertFinishedInventoryIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	productID, bomSpecID, bomVariantID, specG int64,
	warehouse string,
	units, looseG int64,
) error {
	return upsertFinishedInventoryIdentityOwnedTx(ctx, tx, schema, productID, bomSpecID, bomVariantID, specG, warehouse, units, looseG, 0)
}

func upsertFinishedInventoryIdentityOwnedTx(
	ctx context.Context, tx pgx.Tx, schema string,
	productID, bomSpecID, bomVariantID, specG int64, warehouse string, units, looseG, ownerCustomerID int64,
) error {
	hasBomSpec, err := schemaColumnExistsTx(ctx, tx, schema, "finished_inventory", "bom_spec_id")
	if err != nil {
		return err
	}
	hasOwner, err := schemaColumnExistsTx(ctx, tx, schema, "finished_inventory", "owner_customer_id")
	if err != nil {
		return err
	}
	if hasBomSpec && hasOwner {
		if bomSpecID > 0 {
			bomVariantID, err = currentFinishedInventoryBomVariantIDTx(ctx, tx, schema, productID, bomSpecID)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units,onhand_loose_g,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
			ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
			SET bom_variant_id=excluded.bom_variant_id,owner_customer_id=excluded.owner_customer_id,onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
		`, schema), productID, bomSpecID, bomVariantID, specG, warehouse, ownerCustomerID, units, looseG)
		return err
	}
	if hasBomSpec {
		if bomSpecID > 0 {
			bomVariantID, err = currentFinishedInventoryBomVariantIDTx(ctx, tx, schema, productID, bomSpecID)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(
				product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,
				onhand_units,onhand_loose_g,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,now())
			ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
			SET bom_variant_id=excluded.bom_variant_id,
			    onhand_units=excluded.onhand_units,
			    onhand_loose_g=excluded.onhand_loose_g,
			    updated_at=now()
		`, schema), productID, bomSpecID, bomVariantID, specG, warehouse, units, looseG)
		return err
	}
	if bomSpecID > 0 || bomVariantID > 0 {
		return fmt.Errorf("finished inventory BOM specification columns are not available")
	}
	if hasOwner {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,owner_customer_id,onhand_units,onhand_loose_g,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,now())
			ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
			SET owner_customer_id=excluded.owner_customer_id,onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
		`, schema), productID, specG, warehouse, ownerCustomerID, units, looseG)
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,
		    onhand_loose_g=excluded.onhand_loose_g,
		    updated_at=now()
	`, schema), productID, specG, warehouse, units, looseG)
	return err
}
