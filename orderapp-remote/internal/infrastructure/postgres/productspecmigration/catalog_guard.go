package productspecmigration

import (
	"context"
	"fmt"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5"
)

const legacySpecWriteRetirementLock = "legacy-product-spec-write-retirement"

// LockLegacySpecWriteRetirementTx serializes the final per-product cutover
// with every legacy SKU/template write. Callers must take this lock before a
// parent-product migration lock so the lock order is consistent.
func LockLegacySpecWriteRetirementTx(ctx context.Context, tx pgx.Tx, schema string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, schema, legacySpecWriteRetirementLock)
	return err
}

// LegacySpecWritesRetiredTx reports whether every active top-level product has
// completed BOM-spec cutover. An empty catalog or a pre-PR-600 schema keeps the
// legacy setup interfaces available.
func LegacySpecWritesRetiredTx(ctx context.Context, tx pgx.Tx, schema string) (bool, error) {
	if err := LockLegacySpecWriteRetirementTx(ctx, tx, schema); err != nil {
		return false, err
	}
	var productsExist bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".products").Scan(&productsExist); err != nil {
		return false, err
	}
	if !productsExist {
		return false, nil
	}
	var activeParents int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.products product
		WHERE product.active=true
		  AND COALESCE(product.parent_product_id,0)=0
		  AND COALESCE(NULLIF(to_jsonb(product)->>'base_product_id','')::bigint,0)=0
		  AND COALESCE(NULLIF(to_jsonb(product)->>'public_sku_alias','')::boolean,false)=false
	`, schema)).Scan(&activeParents); err != nil {
		return false, err
	}
	return activeParents > 0, nil
}

// PrepareNewProductTx only takes the catalog retirement lock. Product BOM-spec
// authority is derived after a default published BOM is configured.
func PrepareNewProductTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, actor string) error {
	if productID <= 0 {
		return fmt.Errorf("product_id required")
	}
	if err := LockLegacySpecWriteRetirementTx(ctx, tx, schema); err != nil {
		return err
	}
	return nil
}

// LockParentMigrationStateTx serializes legacy child-SKU catalog writes. New
// child-SKU writes are retired for every product.
func LockParentMigrationStateTx(ctx context.Context, tx pgx.Tx, schema string, parentProductID int64) (productspecmigrationapp.MigrationState, error) {
	if parentProductID <= 0 {
		return productspecmigrationapp.StateLegacy, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, parentProductID); err != nil {
		return productspecmigrationapp.StateLegacy, err
	}
	return productspecmigrationapp.StateCutover, nil
}
