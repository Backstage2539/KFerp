package productspecmigration

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureAuthorityProjection installs table-free runtime projections. They are
// derived from the current default published product BOM and remain valid after
// the legacy migration and child-SKU mapping tables are removed.
func EnsureAuthorityView(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE VIEW %[1]s.product_bom_spec_authorities AS
SELECT product.id AS product_id,
       'cutover'::text AS state,
       false::boolean AS legacy_catalog_product,
       'bom_spec'::text AS spec_identity_mode,
       EXISTS (
         SELECT 1
         FROM %[1]s.production_bom_output_bindings binding
         JOIN %[1]s.production_bom_versions version
           ON version.id=binding.bom_version_id
          AND version.bom_id=binding.bom_id
          AND version.status='published'
         JOIN %[1]s.production_bom_version_variants variant
           ON variant.version_id=version.id
         WHERE binding.output_type='product'
           AND binding.output_id=product.id
           AND binding.is_default=true
       ) AS configured
FROM %[1]s.products product
WHERE COALESCE(product.parent_product_id,0)=0;
	`, schema))
	return err
}

func EnsureAuthorityProjection(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := EnsureAuthorityView(ctx, pool, schema); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %[1]s.reject_product_child_sku_write()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF COALESCE(NEW.active,true) AND (COALESCE(NEW.parent_product_id,0)>0 OR COALESCE(NEW.auto_derived_sku,false)) THEN
		RAISE EXCEPTION 'product_child_sku_retired: maintain specifications in the default published BOM' USING ERRCODE='check_violation';
	END IF;
	RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS legacy_child_sku_cutover_guard ON %[1]s.products;
DROP TRIGGER IF EXISTS product_child_sku_guard ON %[1]s.products;
CREATE TRIGGER product_child_sku_guard
	BEFORE INSERT OR UPDATE OF parent_product_id,auto_derived_sku,active ON %[1]s.products
	FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_product_child_sku_write();
`, schema)); err != nil {
		return err
	}
	// Reinstall the complete business-identity guard only after every module has
	// created its tables. This keeps frozen order and production identities
	// traceable while still rejecting arbitrary archived BOM variants.
	return ensureBusinessIdentityWriteGuards(ctx, pool, schema)
}
