package productspecmigration

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type legacyChildMetadata struct {
	ID           int64
	SpecKey      string
	SpecName     string
	SalesUnit    string
	SpecG        int64
	SnapshotJSON string
}

// legacyChildCandidatePredicate is intentionally broader than the old derived
// SKU marker. Historical deployments also contain manually-created child rows
// identified only by parent_product_id, and older rows identified through
// base_product_id. Customer public aliases remain family-level catalog records
// and are not specification identities.
func legacyChildCandidatePredicate(alias string) string {
	return fmt.Sprintf(`(
		%[1]s.parent_product_id=$1
		OR (
			COALESCE(%[1]s.parent_product_id,0)=0
			AND COALESCE(%[1]s.base_product_id,0)=$1
			AND lower(COALESCE(%[1]s.custom_type,''))<>'public_sku_alias'
		)
	)`, alias)
}

func (r Repository) refreshMappingsTx(ctx context.Context, tx pgx.Tx, productID int64, actor string) error {
	bomID, defaultVersionID, err := r.defaultPublishedBomTx(ctx, tx, productID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT child.id,
		       COALESCE(NULLIF(child.derived_spec_key,''),NULLIF(child.spec_label,''),NULLIF(child.sku_code,''),child.id::text),
		       COALESCE(NULLIF(child.derived_spec_name,''),NULLIF(child.spec_label,''),NULLIF(child.sku_name,''),child.name),
		       COALESCE(NULLIF(child.derived_sales_unit,''),NULLIF(child.net_content_unit,''),''),
		       CASE lower(COALESCE(child.net_content_unit,''))
		         WHEN 'g' THEN ROUND(COALESCE(child.net_content_qty,0))::bigint
		         WHEN '克' THEN ROUND(COALESCE(child.net_content_qty,0))::bigint
		         WHEN 'kg' THEN ROUND(COALESCE(child.net_content_qty,0)*1000)::bigint
		         WHEN '公斤' THEN ROUND(COALESCE(child.net_content_qty,0)*1000)::bigint
		         ELSE 0
		       END,
		       jsonb_build_object(
		         'legacy_child_product_id',child.id,
		         'sku_code',COALESCE(child.sku_code,''),
		         'barcode',COALESCE(child.barcode,''),
		         'spec_label',COALESCE(child.spec_label,''),
		         'net_content_qty',COALESCE(child.net_content_qty,0),
		         'net_content_unit',COALESCE(child.net_content_unit,''),
		         'derived_unit_template_id',COALESCE(child.derived_unit_template_id,0),
		         'derived_spec_status',COALESCE(child.derived_spec_status,'')
		       )::text
		FROM %s.products child
		-- The whole-catalog PR-608 apply must preserve and map inactive
		-- historical child rows too; active-only filtering would leave physical
		-- legacy products behind after the dependency scan.
		WHERE %s
		ORDER BY child.id
	`, r.schema, legacyChildCandidatePredicate("child")), productID)
	if err != nil {
		return err
	}
	children := make([]legacyChildMetadata, 0)
	for rows.Next() {
		var child legacyChildMetadata
		if err := rows.Scan(&child.ID, &child.SpecKey, &child.SpecName, &child.SalesUnit, &child.SpecG, &child.SnapshotJSON); err != nil {
			rows.Close()
			return err
		}
		children = append(children, child)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	hasSpecs, err := relationExistsTx(ctx, tx, r.schema, "production_bom_specs")
	if err != nil {
		return err
	}
	hasVariants, err := relationExistsTx(ctx, tx, r.schema, "production_bom_version_variants")
	if err != nil {
		return err
	}
	for _, child := range children {
		var bomSpecID, bomVariantID int64
		if bomID > 0 && hasSpecs {
			err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT id FROM %s.production_bom_specs
				WHERE bom_id=$1 AND lower(spec_key)=lower($2)
				ORDER BY id LIMIT 1
			`, r.schema), bomID, child.SpecKey).Scan(&bomSpecID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if bomSpecID > 0 && defaultVersionID > 0 && hasVariants {
			err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT id FROM %s.production_bom_version_variants
				WHERE version_id=$1 AND bom_spec_id=$2
				ORDER BY is_default DESC,sort_order,id LIMIT 1
			`, r.schema), defaultVersionID, bomSpecID).Scan(&bomVariantID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.legacy_child_sku_bom_spec_mappings(
				parent_product_id,legacy_child_product_id,bom_id,bom_spec_id,bom_variant_id,
				legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,metadata_snapshot,
				created_by,updated_by
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$11)
			ON CONFLICT(legacy_child_product_id) DO UPDATE SET
				parent_product_id=EXCLUDED.parent_product_id,
				bom_id=EXCLUDED.bom_id,
				bom_spec_id=EXCLUDED.bom_spec_id,
				bom_variant_id=EXCLUDED.bom_variant_id,
				legacy_spec_key=EXCLUDED.legacy_spec_key,
				legacy_spec_name=EXCLUDED.legacy_spec_name,
				legacy_sales_unit=EXCLUDED.legacy_sales_unit,
				legacy_spec_g=EXCLUDED.legacy_spec_g,
				metadata_snapshot=EXCLUDED.metadata_snapshot,
				updated_at=now(),updated_by=EXCLUDED.updated_by
		`, r.schema), productID, child.ID, bomID, bomSpecID, bomVariantID, child.SpecKey, child.SpecName, child.SalesUnit, child.SpecG, child.SnapshotJSON, actor); err != nil {
			return err
		}
	}
	hasProductionConfigs, err := relationExistsTx(ctx, tx, r.schema, "product_production_configs")
	if err != nil {
		return err
	}
	if hasProductionConfigs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.legacy_child_sku_bom_spec_mappings mapping
			SET metadata_snapshot = mapping.metadata_snapshot || jsonb_build_object(
				'legacy_production_config', COALESCE((
					SELECT jsonb_build_object(
						'production_bom_id',COALESCE(config.production_bom_id,0),
						'production_bom_version_id',COALESCE(config.production_bom_version_id,0),
						'process_route_id',COALESCE(config.process_route_id,0),
						'expected_loss_rate',COALESCE(config.expected_loss_rate,0),
						'note',COALESCE(config.note,''),
						'industry_field_template_id',COALESCE(config.industry_field_template_id,0)
					)
					FROM %[1]s.product_production_configs config
					WHERE config.product_id=mapping.legacy_child_product_id
				), '{}'::jsonb),
				'parent_production_config', COALESCE((
					SELECT jsonb_build_object(
						'production_bom_id',COALESCE(config.production_bom_id,0),
						'production_bom_version_id',COALESCE(config.production_bom_version_id,0),
						'process_route_id',COALESCE(config.process_route_id,0),
						'expected_loss_rate',COALESCE(config.expected_loss_rate,0),
						'note',COALESCE(config.note,''),
						'industry_field_template_id',COALESCE(config.industry_field_template_id,0)
					)
					FROM %[1]s.product_production_configs config
					WHERE config.product_id=mapping.parent_product_id
				), '{}'::jsonb)
			)
			WHERE mapping.parent_product_id=$1
		`, r.schema), productID); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) defaultPublishedBomTx(ctx context.Context, tx pgx.Tx, productID int64) (int64, int64, error) {
	hasBindings, err := relationExistsTx(ctx, tx, r.schema, "production_bom_output_bindings")
	if err != nil {
		return 0, 0, err
	}
	if hasBindings {
		var bomID, versionID int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT binding.bom_id,binding.bom_version_id
			FROM %s.production_bom_output_bindings binding
			JOIN %s.production_bom_versions version ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id
			WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
			  AND version.status='published'
			ORDER BY binding.updated_at DESC LIMIT 1
		`, r.schema, r.schema), productID).Scan(&bomID, &versionID)
		if err == nil {
			return bomID, versionID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, err
		}
	}
	hasBoms, err := relationExistsTx(ctx, tx, r.schema, "production_boms")
	if err != nil || !hasBoms {
		return 0, 0, err
	}
	var bomID, versionID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom.id,version.id
		FROM %s.production_boms bom
		JOIN %s.production_bom_versions version ON version.bom_id=bom.id AND version.status='published'
		WHERE bom.output_type='product' AND bom.output_product_id=$1 AND bom.status='active'
		ORDER BY version.published_at DESC NULLS LAST,version.id DESC LIMIT 1
	`, r.schema, r.schema), productID).Scan(&bomID, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	return bomID, versionID, err
}

func relationExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists)
	return exists, err
}
