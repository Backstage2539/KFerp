package productspecmigration

import (
	"context"
	"fmt"
	"strings"

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
CREATE OR REPLACE FUNCTION %[1]s.validate_current_product_bom_spec_identity()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
	business_product_id BIGINT;
	business_bom_spec_id BIGINT;
	business_bom_variant_id BIGINT;
	business_item_type TEXT := '';
BEGIN
	business_product_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[0], '')::bigint, 0);
	business_bom_spec_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[1], '')::bigint, 0);
	business_bom_variant_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[2], '')::bigint, 0);
	IF business_product_id <= 0 THEN RETURN NEW; END IF;
	IF COALESCE(TG_ARGV[3], '') <> '' THEN
		business_item_type := lower(COALESCE(to_jsonb(NEW)->>TG_ARGV[3], ''));
		IF business_item_type NOT IN ('product','finished_product') THEN RETURN NEW; END IF;
	END IF;
	IF business_bom_spec_id <= 0 OR business_bom_variant_id <= 0 THEN
		RAISE EXCEPTION 'product_bom_spec_not_configured: product %%',business_product_id USING ERRCODE='check_violation';
	END IF;
	IF NOT EXISTS (
		SELECT 1
		FROM %[1]s.products product
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product' AND binding.output_id=product.id AND binding.is_default=true
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.output_type='product' AND bom.output_product_id=product.id AND bom.status='active'
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=business_bom_spec_id AND spec.bom_id=bom.id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.id=business_bom_variant_id AND variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE product.id=business_product_id AND product.active=true AND COALESCE(product.parent_product_id,0)=0
	) THEN
		RAISE EXCEPTION 'bom_spec_not_current: product %% spec %% variant %%',business_product_id,business_bom_spec_id,business_bom_variant_id USING ERRCODE='check_violation';
	END IF;
	RETURN NEW;
END $$;

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
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		ok, err := tableHasColumns(ctx, pool, schema, target.Table, columns...)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
DROP TRIGGER IF EXISTS bom_spec_identity_guard ON %s.%s;
CREATE TRIGGER bom_spec_identity_guard
	BEFORE INSERT OR UPDATE OF %s ON %s.%s
	FOR EACH ROW EXECUTE FUNCTION %s.validate_current_product_bom_spec_identity('%s','bom_spec_id','bom_variant_id','%s');
`, schema, target.Table, strings.Join(columns, ","), schema, target.Table, schema, target.ProductCol, target.TypeCol)); err != nil {
			return fmt.Errorf("install %s BOM specification authority guard: %w", target.Table, err)
		}
	}
	return nil
}
