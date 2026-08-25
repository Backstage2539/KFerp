package productspecmigration

// PR-608 whole-catalog upgrade.  This package deliberately keeps the
// compatibility columns/tables in place: the upgrade changes the active
// business identity, while the mapping archive is the source for historical
// display and a database-backup rollback.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const authorityUpgradeLockKey = "product-bom-spec-authority-upgrade"

type AuthorityUpgradeRepository struct {
	pool   *pgxpool.Pool
	schema string
}

// NewAuthorityUpgradeRepository is separate from the per-product repository
// so a command can be run without constructing the HTTP application graph.
func NewAuthorityUpgradeRepository(pool *pgxpool.Pool, schema string) AuthorityUpgradeRepository {
	return AuthorityUpgradeRepository{pool: pool, schema: schema}
}

func (r AuthorityUpgradeRepository) PreviewAuthorityUpgrade(ctx context.Context) (productspecmigrationapp.AuthorityUpgradeReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	defer tx.Rollback(ctx)
	report, err := r.buildAuthorityUpgradeReportTx(ctx, tx, productspecmigrationapp.AuthorityUpgradePreview)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	return report, nil
}

func (r AuthorityUpgradeRepository) PrepareAuthorityUpgrade(ctx context.Context, cmd productspecmigrationapp.AuthorityUpgradeCommand) (productspecmigrationapp.AuthorityUpgradeReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.lockAuthorityUpgradeTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	report, err := r.buildAuthorityUpgradeReportTx(ctx, tx, productspecmigrationapp.AuthorityUpgradePrepare)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := r.ensureUpgradeTableTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	var existingState string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_authority_upgrades WHERE manifest_id=$1`, r.schema), report.ManifestID).Scan(&existingState)
	if err == nil && (existingState == "prepared" || existingState == "applied") {
		report.State = existingState
		report.Message = "manifest already prepared; no new BOM draft was created"
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		return report, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	for _, product := range report.Products {
		if err := r.prepareAuthorityProductTx(ctx, tx, product, cmd.Actor); err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, fmt.Errorf("product %d: %w", product.ProductID, err)
		}
	}
	report, err = r.buildAuthorityUpgradeReportTx(ctx, tx, productspecmigrationapp.AuthorityUpgradePrepare)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	report.State = "prepared"
	manifestJSON, _ := json.Marshal(report)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_authority_upgrades(manifest_id,state,manifest_json,created_by,prepared_at,updated_at)
		VALUES($1,'prepared',$2::jsonb,$3,now(),now())
		ON CONFLICT(manifest_id) DO UPDATE SET state='prepared',manifest_json=EXCLUDED.manifest_json,prepared_at=COALESCE(product_bom_spec_authority_upgrades.prepared_at,now()),updated_at=now()
	`, r.schema), report.ManifestID, manifestJSON, cmd.Actor); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_authority_upgrade", nil, "prepare", nil, nil, nil, postgresinfra.AuditMeta{
		"manifest_id": report.ManifestID, "active_product_count": report.ActiveProductCount, "legacy_child_count": report.LegacyChildCount,
	}); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	return report, nil
}

func (r AuthorityUpgradeRepository) ApplyAuthorityUpgrade(ctx context.Context, cmd productspecmigrationapp.AuthorityUpgradeCommand) (productspecmigrationapp.AuthorityUpgradeReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.lockAuthorityUpgradeTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := r.ensureUpgradeTableTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	var state string
	var storedManifest string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT state,manifest_json::text FROM %s.product_bom_spec_authority_upgrades WHERE manifest_id=$1 FOR UPDATE`, r.schema), cmd.ManifestID).Scan(&state, &storedManifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return productspecmigrationapp.AuthorityUpgradeReport{}, fmt.Errorf("PR-608 manifest %s not prepared", cmd.ManifestID)
	}
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if state == "applied" {
		var report productspecmigrationapp.AuthorityUpgradeReport
		if json.Unmarshal([]byte(storedManifest), &report) == nil {
			report.Mode = productspecmigrationapp.AuthorityUpgradeApply
			report.State = "applied"
			report.Message = "manifest already applied; no changes written"
		}
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		return report, nil
	}
	current, err := r.buildAuthorityUpgradeReportTx(ctx, tx, productspecmigrationapp.AuthorityUpgradeApply)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if current.ManifestID != cmd.ManifestID {
		return productspecmigrationapp.AuthorityUpgradeReport{}, fmt.Errorf("PR-608 manifest is stale: expected %s, current %s", cmd.ManifestID, current.ManifestID)
	}
	if current.BlockedProductCount > 0 {
		return current, fmt.Errorf("PR-608 apply blocked: %d active products still need a default published BOM specification", current.BlockedProductCount)
	}
	legacyRepo := Repository{pool: r.pool, schema: r.schema}
	for _, product := range current.Products {
		if err := legacyRepo.refreshMappingsTx(ctx, tx, product.ProductID, cmd.Actor); err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
	}
	if err := r.captureAuthorityChildSnapshotsTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_authority_upgrades
		SET snapshot_json=jsonb_build_object(
			'captured_at',now(),
			'legacy_child_count',(SELECT COUNT(*) FROM %s.legacy_child_sku_bom_spec_mappings WHERE bom_spec_id>0 AND bom_variant_id>0),
			'active_product_count',$2,
			'backup_required',true
		),updated_at=now()
		WHERE manifest_id=$1
	`, r.schema, r.schema), cmd.ManifestID, current.ActiveProductCount); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	for _, target := range businessIdentityTables {
		changed, err := r.rewriteAuthorityReferencesTx(ctx, tx, target)
		if err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		current.RewrittenRefCount += changed
	}
	legacyChanged, err := r.rewriteLegacyCatalogAssociationsTx(ctx, tx)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	current.RewrittenRefCount += legacyChanged
	if err := r.clearLegacyCatalogAssociationsTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	// Preserve the mapping rows as the historical archive while marking every
	// child that is about to be physically removed.  The tombstone is also the
	// audit boundary used by the post-delete dependency scan.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.legacy_child_sku_bom_spec_mappings
		SET tombstoned_at=COALESCE(tombstoned_at,now()),updated_at=now(),updated_by=$1
		WHERE bom_spec_id>0 AND bom_variant_id>0
	`, r.schema), cmd.Actor); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	deleted, err := r.deleteMappedLegacyChildrenTx(ctx, tx)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	current.DeletedChildCount = deleted
	if err := r.assertNoLegacyChildReferencesTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_migrations migration
		SET state='cutover',legacy_catalog_product=false,cutover_at=COALESCE(cutover_at,now()),cutover_by=$1,updated_at=now()
		WHERE migration.product_id IN (SELECT product_id FROM %s.products WHERE active=true AND COALESCE(parent_product_id,0)=0)
	`, r.schema, r.schema), cmd.Actor); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	current.Mode = productspecmigrationapp.AuthorityUpgradeApply
	current.State = "applied"
	current.Message = "BOM 规格已成为商品规格唯一权威；旧规格子商品已归档并物理删除"
	manifestJSON, _ := json.Marshal(current)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_authority_upgrades
		SET state='applied',manifest_json=$2::jsonb,applied_at=now(),updated_at=now()
		WHERE manifest_id=$1
	`, r.schema), cmd.ManifestID, manifestJSON); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_authority_upgrade", nil, "apply", nil, nil, nil, postgresinfra.AuditMeta{
		"manifest_id": cmd.ManifestID, "deleted_child_count": deleted, "rewritten_reference_count": current.RewrittenRefCount,
	}); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	return current, nil
}

// rewriteLegacyCatalogAssociationsTx handles catalog tables that intentionally
// do not carry BOM identity columns.  Aliases and customer references remain
// parent-product records; child production/classification configs are already
// captured in the mapping snapshot and are removed to avoid FK/conflict rows.
func (r AuthorityUpgradeRepository) rewriteLegacyCatalogAssociationsTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	var changed int64
	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "customer_product_aliases", "product_id"); err != nil {
		return 0, err
	} else if has {
		result, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.customer_product_aliases alias
			SET product_id=mapping.parent_product_id
			FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
			WHERE alias.product_id=mapping.legacy_child_product_id
			  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
		`, r.schema))
		if err != nil {
			return 0, fmt.Errorf("rewrite customer product aliases: %w", err)
		}
		changed += result.RowsAffected()
	}
	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "product_customer_references", "product_id"); err != nil {
		return 0, err
	} else if has {
		// The unique product/customer key can collide with an existing parent
		// row; retain that parent row and remove only the legacy child reference.
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %[1]s.product_customer_references child_ref
			USING %[1]s.legacy_child_sku_bom_spec_mappings mapping
			WHERE child_ref.product_id=mapping.legacy_child_product_id
			  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
			  AND EXISTS (SELECT 1 FROM %[1]s.product_customer_references parent_ref WHERE parent_ref.product_id=mapping.parent_product_id AND parent_ref.customer_id=child_ref.customer_id)
		`, r.schema)); err != nil {
			return 0, err
		}
		result, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.product_customer_references ref
			SET product_id=mapping.parent_product_id
			FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
			WHERE ref.product_id=mapping.legacy_child_product_id
			  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
		`, r.schema))
		if err != nil {
			return 0, fmt.Errorf("rewrite product customer references: %w", err)
		}
		changed += result.RowsAffected()
	}
	for _, table := range []string{"product_classification_assignments", "product_production_configs"} {
		has, err := tableHasColumnsTx(ctx, tx, r.schema, table, "product_id")
		if err != nil {
			return 0, err
		}
		if !has {
			continue
		}
		result, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %[1]s.%[2]s legacy
			USING %[1]s.legacy_child_sku_bom_spec_mappings mapping
			WHERE legacy.product_id=mapping.legacy_child_product_id
			  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
		`, r.schema, table))
		if err != nil {
			return 0, fmt.Errorf("remove legacy %s rows: %w", table, err)
		}
		changed += result.RowsAffected()
	}
	return changed, nil
}

func (r AuthorityUpgradeRepository) RollbackAuthorityUpgrade(ctx context.Context, cmd productspecmigrationapp.AuthorityUpgradeCommand) (productspecmigrationapp.AuthorityUpgradeReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.lockAuthorityUpgradeTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := r.ensureUpgradeTableTx(ctx, tx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	var state string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_authority_upgrades WHERE manifest_id=$1 FOR UPDATE`, r.schema), cmd.ManifestID).Scan(&state); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if state == "rolled_back" {
		return productspecmigrationapp.AuthorityUpgradeReport{ManifestID: cmd.ManifestID, Mode: productspecmigrationapp.AuthorityUpgradeRollback, State: "rolled_back", Message: "manifest already rolled back"}, tx.Commit(ctx)
	}
	if state == "applied" {
		return productspecmigrationapp.AuthorityUpgradeReport{}, fmt.Errorf("PR-608 已执行 apply；存在新 BOM 规格写入时不能逐行回滚，请恢复升级前数据库备份")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_authority_upgrades SET state='rolled_back',rolled_back_at=now(),updated_at=now() WHERE manifest_id=$1`, r.schema), cmd.ManifestID); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_authority_upgrade", nil, "rollback", nil, nil, nil, postgresinfra.AuditMeta{"manifest_id": cmd.ManifestID, "backup_restore_required": false}); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	return productspecmigrationapp.AuthorityUpgradeReport{ManifestID: cmd.ManifestID, Mode: productspecmigrationapp.AuthorityUpgradeRollback, State: "rolled_back"}, nil
}

func (r AuthorityUpgradeRepository) ensureUpgradeTableTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom_spec_authority_upgrades (
		manifest_id TEXT PRIMARY KEY,
		state TEXT NOT NULL CHECK (state IN ('prepared','applied','rolled_back')),
		manifest_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),created_by TEXT NOT NULL DEFAULT '',
		prepared_at TIMESTAMPTZ,applied_at TIMESTAMPTZ,rolled_back_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, r.schema))
	return err
}

func (r AuthorityUpgradeRepository) lockAuthorityUpgradeTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+authorityUpgradeLockKey)
	return err
}

func (r AuthorityUpgradeRepository) buildAuthorityUpgradeReportTx(ctx context.Context, tx pgx.Tx, mode productspecmigrationapp.AuthorityUpgradeMode) (productspecmigrationapp.AuthorityUpgradeReport, error) {
	var hasBindings bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".production_bom_output_bindings").Scan(&hasBindings); err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	query := fmt.Sprintf(`
		SELECT p.id,COALESCE(p.name,''),
		       %s,
		       COALESCE(version.version_no,''),COALESCE(version.status,'')
		FROM %s.products p
		%s
		LEFT JOIN %s.production_bom_versions version ON version.id=binding.bom_version_id
		WHERE p.active=true AND COALESCE(p.parent_product_id,0)=0
		  AND COALESCE(p.base_product_id,0)=0
		  AND lower(COALESCE(p.custom_type,''))<>'public_sku_alias'
		ORDER BY p.id
	`, func() string {
		if hasBindings {
			return "COALESCE(binding.bom_id,0),COALESCE(binding.bom_version_id,0)"
		}
		return "0::bigint,0::bigint"
	}(), r.schema, func() string {
		if hasBindings {
			return fmt.Sprintf(`LEFT JOIN LATERAL (
				SELECT b.bom_id,b.bom_version_id
				FROM %s.production_bom_output_bindings b
				JOIN %s.production_bom_versions v ON v.id=b.bom_version_id AND v.bom_id=b.bom_id AND v.status='published'
				WHERE b.output_type='product' AND b.output_id=p.id AND b.is_default=true
				ORDER BY b.updated_at DESC NULLS LAST,b.bom_version_id DESC LIMIT 1
			) binding ON true`, r.schema, r.schema)
		}
		return "LEFT JOIN LATERAL (SELECT 0::bigint AS bom_id,0::bigint AS bom_version_id) binding ON true"
	}(), r.schema)
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	report := productspecmigrationapp.AuthorityUpgradeReport{Mode: mode, GeneratedAt: time.Now().UTC(), Products: make([]productspecmigrationapp.AuthorityUpgradeProduct, 0)}
	products := make([]struct {
		product      productspecmigrationapp.AuthorityUpgradeProduct
		versionState string
	}, 0)
	for rows.Next() {
		var product productspecmigrationapp.AuthorityUpgradeProduct
		var versionStatus string
		if err := rows.Scan(&product.ProductID, &product.ProductName, &product.BomID, &product.BomVersionID, &product.BomVersionNo, &versionStatus); err != nil {
			rows.Close()
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		products = append(products, struct {
			product      productspecmigrationapp.AuthorityUpgradeProduct
			versionState string
		}{product: product, versionState: versionStatus})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return productspecmigrationapp.AuthorityUpgradeReport{}, err
	}
	rows.Close()
	for _, entry := range products {
		product := entry.product
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products child WHERE (child.parent_product_id=$1 OR (COALESCE(child.parent_product_id,0)=0 AND child.base_product_id=$1 AND lower(COALESCE(child.custom_type,''))<>'public_sku_alias'))`, r.schema), product.ProductID).Scan(&product.LegacyChildCount); err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		product.ReferenceCounts, err = r.legacyReferenceCountsTx(ctx, tx, product.ProductID)
		if err != nil {
			return productspecmigrationapp.AuthorityUpgradeReport{}, err
		}
		if product.BomID <= 0 || product.BomVersionID <= 0 || entry.versionState != "published" {
			product.Blockers = append(product.Blockers, "缺少默认已发布 BOM 版本")
		} else {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),COUNT(*) FILTER (WHERE is_default),COUNT(*) FILTER (WHERE btrim(COALESCE(inventory_unit,''))='') FROM %s.production_bom_version_variants WHERE version_id=$1`, r.schema), product.BomVersionID).Scan(&product.PublishedVariantCount, &product.DefaultVariantCount, &product.InvalidUnitCount); err != nil {
				return productspecmigrationapp.AuthorityUpgradeReport{}, err
			}
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items WHERE version_id=$1`, r.schema), product.BomVersionID).Scan(&product.RecipeItemCount); err != nil {
				return productspecmigrationapp.AuthorityUpgradeReport{}, err
			}
			if product.PublishedVariantCount == 0 {
				product.Blockers = append(product.Blockers, "默认已发布 BOM 没有有效规格")
			}
			if product.DefaultVariantCount != 1 {
				product.Blockers = append(product.Blockers, "默认已发布 BOM 必须且只能有一个默认规格")
			}
			if product.InvalidUnitCount > 0 {
				product.Blockers = append(product.Blockers, "BOM 规格缺少库存单位")
			}
			if product.RecipeItemCount == 0 {
				product.Blockers = append(product.Blockers, "BOM 配方明细为空，需补齐后才能升级")
			}
		}
		product.Ready = len(product.Blockers) == 0
		report.Products = append(report.Products, product)
		report.LegacyChildCount += product.LegacyChildCount
	}
	report.ActiveProductCount = int64(len(report.Products))
	for _, product := range report.Products {
		if product.Ready {
			report.ReadyProductCount++
		} else {
			report.BlockedProductCount++
		}
	}
	manifestInput := make([]productspecmigrationapp.AuthorityUpgradeProduct, len(report.Products))
	copy(manifestInput, report.Products)
	manifestJSON, _ := json.Marshal(manifestInput)
	digest := sha256.Sum256(manifestJSON)
	report.ManifestID = "PR-608-" + hex.EncodeToString(digest[:8])
	if report.BlockedProductCount > 0 {
		report.State = "blocked"
		report.Message = "存在未就绪启用主商品；prepare 可生成待完善规格草稿，apply 会在全部商品就绪前拒绝"
	} else {
		report.State = "ready"
	}
	return report, nil
}

func (r AuthorityUpgradeRepository) legacyReferenceCountsTx(ctx context.Context, tx pgx.Tx, parentProductID int64) (map[string]int64, error) {
	counts := map[string]int64{}
	childPredicate := fmt.Sprintf(`(child.parent_product_id=$1 OR (COALESCE(child.parent_product_id,0)=0 AND child.base_product_id=$1 AND lower(COALESCE(child.custom_type,''))<>'public_sku_alias'))`)
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
		if err != nil {
			return nil, err
		}
		if !has {
			continue
		}
		typeClause := ""
		if target.TypeCol != "" {
			typeClause = fmt.Sprintf(" AND lower(COALESCE(ref.%s,'')) IN ('product','finished_product')", target.TypeCol)
		}
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.%[2]s ref
			JOIN %[1]s.products child ON ref.%[3]s=child.id
			WHERE %[4]s %[5]s
		`, r.schema, target.Table, target.ProductCol, childPredicate, typeClause), parentProductID).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s legacy references: %w", target.Table, err)
		}
		if count > 0 {
			counts[target.Table] = count
		}
	}
	return counts, nil
}

func (r AuthorityUpgradeRepository) prepareAuthorityProductTx(ctx context.Context, tx pgx.Tx, product productspecmigrationapp.AuthorityUpgradeProduct, actor string) error {
	bomID, versionID := product.BomID, product.BomVersionID
	sourceVersionID := int64(0)
	if bomID <= 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status,source_product_id,created_by,updated_by) VALUES($1,$2,'product',$3,'active',$3,$4,$4) RETURNING id`, r.schema), fmt.Sprintf("PR608-%d", product.ProductID), product.ProductName+" BOM（待完善）", product.ProductID, actor).Scan(&bomID); err != nil {
			return err
		}
	}
	if versionID > 0 {
		sourceVersionID = versionID
		var status string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_versions WHERE id=$1`, r.schema), versionID).Scan(&status); err != nil {
			return err
		}
		if status == "published" {
			versionID = 0
		}
	}
	if versionID <= 0 {
		// Reuse the draft created by an earlier prepare when possible.  This
		// makes prepare idempotent even when the preview manifest changed after
		// an operator edited another product.
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id FROM %s.production_bom_versions
			WHERE bom_id=$1 AND status='draft' AND version_no LIKE '%%-PR608'
			ORDER BY id DESC LIMIT 1
		`, r.schema), bomID).Scan(&versionID)
		if errors.Is(err, pgx.ErrNoRows) {
			if sourceVersionID > 0 {
				err = tx.QueryRow(ctx, fmt.Sprintf(`
					INSERT INTO %[1]s.production_bom_versions(
						bom_id,version_no,status,yield_rate,output_qty,output_unit,material_loss_rate,
						note,special_attrs_schema_json,special_attrs_json,process_route_id,
						source_spec_template_version_id,main_input_material_id,created_by
					)
					SELECT bom_id,
					       COALESCE(NULLIF(version_no,''),'V001') || '-PR608',
					       'draft',COALESCE(yield_rate,1),COALESCE(output_qty,1),COALESCE(NULLIF(output_unit,''),'unit'),
					       COALESCE(material_loss_rate,0),
					       COALESCE(note,'') || '；PR-608 迁移草稿，待核对主物料与配方',
					       COALESCE(special_attrs_schema_json,'[]'::jsonb),COALESCE(special_attrs_json,'{}'::jsonb),
					       COALESCE(process_route_id,0),COALESCE(source_spec_template_version_id,0),COALESCE(main_input_material_id,0),$2
					FROM %[1]s.production_bom_versions WHERE id=$1
					RETURNING id
				`, r.schema), sourceVersionID, actor).Scan(&versionID)
			} else {
				err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit,note,created_by) VALUES($1,'V001-PR608','draft',1,'unit','PR-608 待完善：未猜测主物料、包装或配方用量',$2) RETURNING id`, r.schema), bomID, actor).Scan(&versionID)
			}
		}
		if err != nil {
			return err
		}
	}
	if sourceVersionID > 0 && sourceVersionID != versionID {
		// An existing BOM is safe to carry forward as a migration draft: copy
		// its versioned variants and frozen recipe rows, while leaving the draft
		// unpublished so the operator can correct any unproven component.
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_bom_version_variants(
				version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
			)
			SELECT $1,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
			FROM %[1]s.production_bom_version_variants source
			WHERE source.version_id=$2
			  AND NOT EXISTS (SELECT 1 FROM %[1]s.production_bom_version_variants existing WHERE existing.version_id=$1)
		`, r.schema), versionID, sourceVersionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_bom_version_items(
				version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,
				consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot
			)
			SELECT $1,target_variant.id,source_item.material_id,source_item.component_type,source_item.component_product_id,
			       source_item.component_bom_spec_id,source_item.component_spec_g,source_item.consume_unit,source_item.qty_per_unit,
			       source_item.ratio_pct,source_item.material_loss_rate,source_item.unit_cost_snapshot
			FROM %[1]s.production_bom_version_items source_item
			JOIN %[1]s.production_bom_version_variants source_variant ON source_variant.id=source_item.variant_id
			JOIN %[1]s.production_bom_version_variants target_variant
			  ON target_variant.version_id=$1 AND target_variant.bom_spec_id=source_variant.bom_spec_id
			WHERE source_item.version_id=$2
			  AND NOT EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items existing WHERE existing.version_id=$1)
		`, r.schema), versionID, sourceVersionID); err != nil {
			return err
		}
		if exists, err := relationExistsTx(ctx, tx, r.schema, "production_bom_version_operation_costs"); err != nil {
			return err
		} else if exists {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %[1]s.production_bom_version_operation_costs(
					version_id,operation_id,operation_name,workstation_id,workstation_name,
					workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,
					batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,
					rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order
				)
				SELECT $1,operation_id,operation_name,workstation_id,workstation_name,
					workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,
					batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,
					rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order
				FROM %[1]s.production_bom_version_operation_costs source
				WHERE source.version_id=$2
				  AND NOT EXISTS (SELECT 1 FROM %[1]s.production_bom_version_operation_costs existing WHERE existing.version_id=$1)
			`, r.schema), versionID, sourceVersionID); err != nil {
				return err
			}
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(NULLIF(derived_spec_key,''),NULLIF(spec_label,''),NULLIF(sku_code,''),id::text),COALESCE(NULLIF(derived_spec_name,''),NULLIF(spec_label,''),NULLIF(sku_name,''),name),COALESCE(NULLIF(derived_sales_unit,''),NULLIF(net_content_unit,''),''),COALESCE(barcode,'') FROM %s.products WHERE (parent_product_id=$1 OR (COALESCE(parent_product_id,0)=0 AND base_product_id=$1 AND lower(COALESCE(custom_type,''))<>'public_sku_alias') ) ORDER BY id`, r.schema), product.ProductID)
	if err != nil {
		return err
	}
	children := make([]struct {
		id       int64
		specKey  string
		specName string
		unit     string
		barcode  string
	}, 0)
	for rows.Next() {
		var childID int64
		var specKey, specName, unit, barcode string
		if err := rows.Scan(&childID, &specKey, &specName, &unit, &barcode); err != nil {
			rows.Close()
			return err
		}
		children = append(children, struct {
			id       int64
			specKey  string
			specName string
			unit     string
			barcode  string
		}{id: childID, specKey: specKey, specName: specName, unit: unit, barcode: barcode})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, child := range children {
		childID, specKey, specName, unit, barcode := child.id, child.specKey, child.specName, child.unit, child.barcode
		var specID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_specs WHERE bom_id=$1 AND lower(spec_key)=lower($2) ORDER BY id LIMIT 1`, r.schema), bomID, specKey).Scan(&specID); errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id`, r.schema), bomID, fmt.Sprintf("PR608-%d-%d", product.ProductID, childID), barcode, specKey, specName, unit, actor).Scan(&specID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		var variantExists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_version_variants WHERE version_id=$1 AND bom_spec_id=$2)`, r.schema), versionID, specID).Scan(&variantExists); err != nil {
			return err
		}
		if !variantExists {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES($1,$2,$3,$4,false,100)`, r.schema), versionID, specID, specName, unit); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product,prepared_at,prepared_by,updated_at) VALUES($1,'preparing',true,now(),$2,now()) ON CONFLICT(product_id) DO UPDATE SET state=CASE WHEN product_bom_spec_migrations.state='cutover' THEN product_bom_spec_migrations.state ELSE 'preparing' END,prepared_at=COALESCE(product_bom_spec_migrations.prepared_at,now()),prepared_by=CASE WHEN product_bom_spec_migrations.prepared_by='' THEN EXCLUDED.prepared_by ELSE product_bom_spec_migrations.prepared_by END,updated_at=now()`, r.schema), product.ProductID, actor)
	return err
}

func (r AuthorityUpgradeRepository) captureAuthorityChildSnapshotsTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.legacy_child_sku_bom_spec_mappings mapping
		SET metadata_snapshot=jsonb_build_object('legacy_product_row',to_jsonb(child)) || COALESCE(mapping.metadata_snapshot,'{}'::jsonb),updated_at=now()
		FROM %[1]s.products child
		WHERE child.id=mapping.legacy_child_product_id
	`, r.schema))
	return err
}

func (r AuthorityUpgradeRepository) rewriteAuthorityReferencesTx(ctx context.Context, tx pgx.Tx, target businessIdentityTarget) (int64, error) {
	columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
	has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
	if err != nil || !has {
		return 0, err
	}
	typeClause := ""
	if target.TypeCol != "" {
		typeClause = fmt.Sprintf(" AND lower(COALESCE(%s,'')) IN ('product','finished_product')", target.TypeCol)
	}
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.%[2]s target
		SET %[3]s=mapping.parent_product_id,target.bom_spec_id=mapping.bom_spec_id,target.bom_variant_id=mapping.bom_variant_id
		FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
		WHERE target.%[3]s=mapping.legacy_child_product_id
		  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0 %[4]s
	`, r.schema, target.Table, target.ProductCol, typeClause))
	if err != nil {
		return 0, fmt.Errorf("rewrite %s: %w", target.Table, err)
	}
	return result.RowsAffected(), nil
}

func (r AuthorityUpgradeRepository) clearLegacyCatalogAssociationsTx(ctx context.Context, tx pgx.Tx) error {
	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "products", "unit_template_id", "default_sku_id"); err != nil {
		return err
	} else if has {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET unit_template_id=0,default_sku_id=0,derived_unit_template_id=0 WHERE active=true`, r.schema)); err != nil {
			return err
		}
	}
	if has, err := relationExistsTx(ctx, tx, r.schema, "product_unit_templates"); err != nil {
		return err
	} else if has {
		// The compatibility rows are retained only when an old immutable record
		// still points at them; active product behaviour no longer reads them.
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_unit_templates SET active=false,deleted_at=COALESCE(deleted_at,now()),updated_at=now()`, r.schema))
		return err
	}
	return nil
}

func (r AuthorityUpgradeRepository) deleteMappedLegacyChildrenTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %[1]s.products child
		USING %[1]s.legacy_child_sku_bom_spec_mappings mapping
		WHERE child.id=mapping.legacy_child_product_id
		  AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
		  AND (child.parent_product_id=mapping.parent_product_id OR (COALESCE(child.parent_product_id,0)=0 AND child.base_product_id=mapping.parent_product_id AND lower(COALESCE(child.custom_type,''))<>'public_sku_alias'))
	`, r.schema))
	if err != nil {
		return 0, fmt.Errorf("delete mapped legacy child products: %w", err)
	}
	return result.RowsAffected(), nil
}

func (r AuthorityUpgradeRepository) assertNoLegacyChildReferencesTx(ctx context.Context, tx pgx.Tx) error {
	var remaining int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products child JOIN %s.legacy_child_sku_bom_spec_mappings mapping ON mapping.legacy_child_product_id=child.id`, r.schema, r.schema)).Scan(&remaining); err != nil {
		return err
	}
	if remaining > 0 {
		return fmt.Errorf("PR-608 dependency scan found %d legacy child product rows still referenced", remaining)
	}
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		typeClause := ""
		if target.TypeCol != "" {
			typeClause = fmt.Sprintf(" AND lower(COALESCE(target.%s,'')) IN ('product','finished_product')", target.TypeCol)
		}
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.%[2]s target
			JOIN %[1]s.legacy_child_sku_bom_spec_mappings mapping
			  ON target.%[3]s=mapping.legacy_child_product_id
			WHERE mapping.tombstoned_at IS NOT NULL %[4]s
		`, r.schema, target.Table, target.ProductCol, typeClause)).Scan(&count); err != nil {
			return fmt.Errorf("scan %s legacy references: %w", target.Table, err)
		}
		if count > 0 {
			return fmt.Errorf("PR-608 dependency scan found %d legacy child references in %s", count, target.Table)
		}
	}
	for _, table := range []string{"customer_product_aliases", "product_customer_references", "product_classification_assignments", "product_production_configs"} {
		has, err := tableHasColumnsTx(ctx, tx, r.schema, table, "product_id")
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.%[2]s ref
			JOIN %[1]s.legacy_child_sku_bom_spec_mappings mapping ON ref.product_id=mapping.legacy_child_product_id
			WHERE mapping.tombstoned_at IS NOT NULL AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
		`, r.schema, table)).Scan(&count); err != nil {
			return fmt.Errorf("scan %s legacy references: %w", table, err)
		}
		if count > 0 {
			return fmt.Errorf("PR-608 dependency scan found %d legacy child references in %s", count, table)
		}
	}
	return nil
}

// Keep deterministic ordering in reports even if a future query gains another
// join.  This also makes the manifest safe to compare in CI and in backups.
func sortAuthorityProducts(rows []productspecmigrationapp.AuthorityUpgradeProduct) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].ProductID < rows[j].ProductID })
}
