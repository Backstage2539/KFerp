package productspecmigration

import (
	"context"
	"errors"
	"fmt"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	postgresinfra "orderapp/internal/infrastructure/postgres"

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
	var migrationsExist, productsExist bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL`, schema+".product_bom_spec_migrations", schema+".products").Scan(&migrationsExist, &productsExist); err != nil {
		return false, err
	}
	if !migrationsExist || !productsExist {
		return false, nil
	}
	var activeParents, legacyParents, cutoverParents int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*),
		       COUNT(*) FILTER (
			   WHERE COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=true
		       ),
		       COUNT(*) FILTER (
			   WHERE COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=true
			     AND migration.state='cutover'
		       )
		FROM %s.products product
		LEFT JOIN %s.product_bom_spec_migrations migration ON migration.product_id=product.id
		WHERE product.active=true
		  AND COALESCE(product.parent_product_id,0)=0
		  AND COALESCE(NULLIF(to_jsonb(product)->>'base_product_id','')::bigint,0)=0
		  AND COALESCE(NULLIF(to_jsonb(product)->>'public_sku_alias','')::boolean,false)=false
	`, schema, schema)).Scan(&activeParents, &legacyParents, &cutoverParents); err != nil {
		return false, err
	}
	return activeParents > 0 && legacyParents == cutoverParents, nil
}

// PrepareNewProductTx registers a newly created parent product directly in the
// BOM-spec workflow. It is not a legacy catalog migration candidate, so adding
// it after final cutover must not reopen retired child-SKU/template writes.
func PrepareNewProductTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, actor string) error {
	if productID <= 0 {
		return fmt.Errorf("product_id required")
	}
	if err := LockLegacySpecWriteRetirementTx(ctx, tx, schema); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(
			product_id,state,legacy_catalog_product,spec_identity_mode,prepared_at,prepared_by,updated_at
		) VALUES($1,'preparing',false,'bom_spec',now(),$2,now())
	`, schema), productID, actor); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "product_bom_spec_migration", &productID, "prepare_new_product", postgresinfra.StrPtr("state"), nil, postgresinfra.StrPtr(string(productspecmigrationapp.StatePreparing)), postgresinfra.AuditMeta{
		"legacy_catalog_product": false,
		"recipe_rows_created":    0,
	})
}

// LockParentMigrationStateTx serializes legacy child-SKU catalog writes with
// per-product BOM-spec cutover. A missing migration table or row preserves the
// legacy catalog behavior used before PR-600 is enabled for a product.
func LockParentMigrationStateTx(ctx context.Context, tx pgx.Tx, schema string, parentProductID int64) (productspecmigrationapp.MigrationState, error) {
	if parentProductID <= 0 {
		return productspecmigrationapp.StateLegacy, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, parentProductID); err != nil {
		return productspecmigrationapp.StateLegacy, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".product_bom_spec_migrations").Scan(&exists); err != nil {
		return productspecmigrationapp.StateLegacy, err
	}
	if !exists {
		return productspecmigrationapp.StateLegacy, nil
	}
	var state string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT state FROM %s.product_bom_spec_migrations
		WHERE product_id=$1
		FOR SHARE
	`, schema), parentProductID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return productspecmigrationapp.StateLegacy, nil
	}
	if err != nil {
		return productspecmigrationapp.StateLegacy, err
	}
	return productspecmigrationapp.MigrationState(state), nil
}
