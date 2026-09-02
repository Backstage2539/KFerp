package productspecmigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pr622CleanupLockKey = "pr622-product-bom-spec-authority-cleanup"

type PR622CleanupMode string

const (
	PR622CleanupPreview PR622CleanupMode = "preview"
	PR622CleanupApply   PR622CleanupMode = "apply"
	PR622CleanupVerify  PR622CleanupMode = "verify"
)

type PR622CleanupDependency struct {
	Table string `json:"table"`
	Count int64  `json:"count"`
}

type PR622BackupEvidence struct {
	Path   string
	SHA256 string
	Size   int64
}

type PR622CleanupApplyOptions struct {
	DiscardUnmappedTestData bool
}

type PR622CleanupReport struct {
	Mode                              PR622CleanupMode         `json:"mode"`
	State                             string                   `json:"state"`
	ManifestID                        string                   `json:"manifest_id"`
	GeneratedAt                       time.Time                `json:"generated_at"`
	LegacyChildCount                  int64                    `json:"legacy_child_count"`
	MappableChildCount                int64                    `json:"mappable_child_count"`
	ConvertibleSingleBOMCount         int64                    `json:"convertible_single_bom_count"`
	DetachedSingleBOMCount            int64                    `json:"detached_single_bom_count"`
	ConfiguredProductCount            int64                    `json:"configured_product_count"`
	UnconfiguredProductCount          int64                    `json:"unconfigured_product_count"`
	RewrittenReferenceCount           int64                    `json:"rewritten_reference_count"`
	DeletedChildCount                 int64                    `json:"deleted_child_count"`
	DeletedUnitTemplateCount          int64                    `json:"deleted_unit_template_count"`
	PublishedPriceVersionCount        int64                    `json:"published_price_version_count"`
	DiscardedUnmappedReferenceCount   int64                    `json:"discarded_unmapped_reference_count"`
	WithdrawnUnmappedPublicationCount int64                    `json:"withdrawn_unmapped_publication_count"`
	VoidedUnmappedOrderCount          int64                    `json:"voided_unmapped_order_count"`
	MigrationRelationsExist           bool                     `json:"migration_relations_exist"`
	LegacyUnitTemplateCount           int64                    `json:"legacy_unit_template_count"`
	ActiveProductSingleBOMCount       int64                    `json:"active_product_single_bom_count"`
	PublishedChildPriceReferenceCount int64                    `json:"published_child_price_reference_count"`
	BlockingDependencies              []PR622CleanupDependency `json:"blocking_dependencies"`
	DroppedRelations                  []string                 `json:"dropped_relations,omitempty"`
	Message                           string                   `json:"message"`
}

type pr622ManifestChild struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parent_id"`
	SpecKey  string `json:"spec_key"`
	SKUCode  string `json:"sku_code"`
	SpecName string `json:"spec_name"`
	Barcode  string `json:"barcode"`
	Active   bool   `json:"active"`
	Mappable bool   `json:"mappable"`
}

type PR622CleanupRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewPR622CleanupRepository(pool *pgxpool.Pool, schema string) PR622CleanupRepository {
	return PR622CleanupRepository{pool: pool, schema: schema}
}

func (r PR622CleanupRepository) Preview(ctx context.Context) (PR622CleanupReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR622CleanupReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockTx(ctx, tx); err != nil {
		return PR622CleanupReport{}, err
	}
	report, err := r.buildReportTx(ctx, tx, PR622CleanupPreview)
	if err != nil {
		return PR622CleanupReport{}, err
	}
	return report, tx.Commit(ctx)
}

func (r PR622CleanupRepository) Apply(ctx context.Context, manifestID, actor string, backup PR622BackupEvidence, options PR622CleanupApplyOptions) (PR622CleanupReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR622CleanupReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockTx(ctx, tx); err != nil {
		return PR622CleanupReport{}, err
	}
	report, err := r.buildReportTx(ctx, tx, PR622CleanupApply)
	if err != nil {
		return PR622CleanupReport{}, err
	}
	if !report.MigrationRelationsExist {
		report.State = "completed"
		report.Message = "PR-622 cleanup already completed; no changes written"
		return report, tx.Commit(ctx)
	}
	if strings.TrimSpace(manifestID) == "" || manifestID != report.ManifestID {
		return report, fmt.Errorf("PR-622 manifest is stale: expected %s, current %s", manifestID, report.ManifestID)
	}
	if len(report.BlockingDependencies) > 0 && !options.DiscardUnmappedTestData {
		return report, fmt.Errorf("PR-622 cleanup blocked by %d unresolved live dependency groups", len(report.BlockingDependencies))
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "operator-pr622"
	}
	if err := r.suspendPR622AuthorityGuardsTx(ctx, tx); err != nil {
		return report, err
	}
	converted, err := r.convertPublishedSingleBOMsTx(ctx, tx, actor)
	if err != nil {
		return report, err
	}
	detached, err := r.detachRemainingSingleProductBOMsTx(ctx, tx, actor)
	if err != nil {
		return report, err
	}
	report.DetachedSingleBOMCount = detached
	legacyRepo := Repository{pool: r.pool, schema: r.schema}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.products WHERE COALESCE(parent_product_id,0)=0 ORDER BY id`, r.schema))
	if err != nil {
		return report, err
	}
	productIDs := make([]int64, 0)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			rows.Close()
			return report, err
		}
		productIDs = append(productIDs, productID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	for _, productID := range productIDs {
		if err := legacyRepo.refreshMappingsTx(ctx, tx, productID, actor); err != nil {
			return report, fmt.Errorf("refresh product %d specification mapping: %w", productID, err)
		}
	}
	if options.DiscardUnmappedTestData {
		discarded, err := r.discardUnmappedTestDataTx(ctx, tx, actor)
		if err != nil {
			return report, err
		}
		report.DiscardedUnmappedReferenceCount = discarded.References
		report.WithdrawnUnmappedPublicationCount = discarded.Publications
		report.VoidedUnmappedOrderCount = discarded.Orders
		mappableChildIDs, err := r.mappableChildIDsTx(ctx, tx)
		if err != nil {
			return report, err
		}
		remaining, err := r.unmappedLiveDependenciesTx(ctx, tx, mappableChildIDs)
		if err != nil {
			return report, err
		}
		if len(remaining) > 0 {
			return report, fmt.Errorf("PR-622 authorized test-data discard left %d unresolved live dependency groups", len(remaining))
		}
		report.BlockingDependencies = []PR622CleanupDependency{}
	}
	priceVersions, err := r.clonePublishedPriceVersionsTx(ctx, tx, actor)
	if err != nil {
		return report, err
	}
	report.PublishedPriceVersionCount = priceVersions
	for _, target := range businessIdentityTables {
		changed, err := r.rewriteMappedReferencesTx(ctx, tx, target)
		if err != nil {
			return report, err
		}
		report.RewrittenReferenceCount += changed
	}
	historicalOrders, err := r.rewriteHistoricalOrderSnapshotsTx(ctx, tx)
	if err != nil {
		return report, err
	}
	report.RewrittenReferenceCount += historicalOrders
	changed, err := r.rewriteFamilyCatalogReferencesTx(ctx, tx)
	if err != nil {
		return report, err
	}
	report.RewrittenReferenceCount += changed
	if err := r.removeChildCatalogRowsTx(ctx, tx); err != nil {
		return report, err
	}
	deleted, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.products WHERE COALESCE(parent_product_id,0)>0`, r.schema))
	if err != nil {
		return report, fmt.Errorf("delete legacy child products: %w", err)
	}
	report.DeletedChildCount = deleted.RowsAffected()
	unitTemplates, err := r.clearLegacyUnitMetadataTx(ctx, tx)
	if err != nil {
		return report, err
	}
	report.DeletedUnitTemplateCount = unitTemplates
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_bom_spec_authorities WHERE configured=true`, r.schema)).Scan(&report.ConfiguredProductCount); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products product LEFT JOIN %s.product_bom_spec_authorities authority ON authority.product_id=product.id WHERE product.active=true AND COALESCE(product.parent_product_id,0)=0 AND COALESCE(authority.configured,false)=false`, r.schema, r.schema)).Scan(&report.UnconfiguredProductCount); err != nil {
		return report, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_bom_spec_authority_cleanup", nil, "apply", nil, nil, nil, postgresinfra.AuditMeta{
		"environment_schema":                   r.schema,
		"manifest_id":                          report.ManifestID,
		"legacy_child_count":                   report.LegacyChildCount,
		"mappable_child_count":                 report.MappableChildCount,
		"converted_single_bom_count":           converted,
		"detached_single_bom_count":            report.DetachedSingleBOMCount,
		"rewritten_reference_count":            report.RewrittenReferenceCount,
		"deleted_child_count":                  report.DeletedChildCount,
		"deleted_unit_template_count":          report.DeletedUnitTemplateCount,
		"published_price_version_count":        report.PublishedPriceVersionCount,
		"discarded_unmapped_reference_count":   report.DiscardedUnmappedReferenceCount,
		"withdrawn_unmapped_publication_count": report.WithdrawnUnmappedPublicationCount,
		"voided_unmapped_order_count":          report.VoidedUnmappedOrderCount,
		"unconfigured_product_count":           report.UnconfiguredProductCount,
		"backup_restore_required":              true,
		"backup_path":                          strings.TrimSpace(backup.Path),
		"backup_sha256":                        strings.ToLower(strings.TrimSpace(backup.SHA256)),
		"backup_size":                          backup.Size,
	}); err != nil {
		return report, err
	}
	if err := r.dropLegacyMigrationRelationsTx(ctx, tx); err != nil {
		return report, err
	}
	if err := r.restorePR622AuthorityGuardsTx(ctx, tx); err != nil {
		return report, err
	}
	report.Mode = PR622CleanupApply
	report.State = "completed"
	report.MigrationRelationsExist = false
	report.DroppedRelations = []string{"legacy_child_sku_bom_spec_mappings", "product_bom_spec_authority_upgrades", "product_bom_spec_migrations"}
	report.Message = "商品规格权威已收敛到默认已发布 BOM；旧规格子商品和迁移表已清理"
	if err := tx.Commit(ctx); err != nil {
		return PR622CleanupReport{}, err
	}
	return report, nil
}

func (r PR622CleanupRepository) Verify(ctx context.Context) (PR622CleanupReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR622CleanupReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockTx(ctx, tx); err != nil {
		return PR622CleanupReport{}, err
	}
	report, err := r.buildReportTx(ctx, tx, PR622CleanupVerify)
	if err != nil {
		return PR622CleanupReport{}, err
	}
	if report.LegacyChildCount != 0 || report.MigrationRelationsExist || report.LegacyUnitTemplateCount != 0 || report.ActiveProductSingleBOMCount != 0 || report.PublishedChildPriceReferenceCount != 0 {
		report.State = "failed"
		return report, fmt.Errorf("PR-622 verification failed: child products=%d migration_relations_exist=%t unit_templates=%d product_single_boms=%d published_child_prices=%d", report.LegacyChildCount, report.MigrationRelationsExist, report.LegacyUnitTemplateCount, report.ActiveProductSingleBOMCount, report.PublishedChildPriceReferenceCount)
	}
	report.State = "verified"
	report.Message = "legacy child products are zero and migration relations are absent"
	return report, tx.Commit(ctx)
}

func (r PR622CleanupRepository) lockTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+pr622CleanupLockKey)
	return err
}

func (r PR622CleanupRepository) buildReportTx(ctx context.Context, tx pgx.Tx, mode PR622CleanupMode) (PR622CleanupReport, error) {
	report := PR622CleanupReport{Mode: mode, GeneratedAt: time.Now().UTC(), BlockingDependencies: []PR622CleanupDependency{}}
	var migrationExists, mappingExists, upgradeExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL,to_regclass($3) IS NOT NULL`, r.schema+".product_bom_spec_migrations", r.schema+".legacy_child_sku_bom_spec_mappings", r.schema+".product_bom_spec_authority_upgrades").Scan(&migrationExists, &mappingExists, &upgradeExists); err != nil {
		return report, err
	}
	report.MigrationRelationsExist = migrationExists || mappingExists || upgradeExists
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT child.id,child.parent_product_id,
		       COALESCE(NULLIF(child.derived_spec_key,''),NULLIF(child.spec_label,''),NULLIF(child.sku_code,''),child.id::text),
		       COALESCE(child.sku_code,''),
		       COALESCE(NULLIF(child.derived_spec_name,''),NULLIF(child.spec_label,''),NULLIF(child.sku_name,''),child.name),
		       COALESCE(child.barcode,''),
		       child.active
		FROM %[1]s.products child
		WHERE COALESCE(child.parent_product_id,0)>0
		ORDER BY child.id
	`, r.schema))
	if err != nil {
		return report, err
	}
	manifest := make([]pr622ManifestChild, 0)
	for rows.Next() {
		var child pr622ManifestChild
		if err := rows.Scan(&child.ID, &child.ParentID, &child.SpecKey, &child.SKUCode, &child.SpecName, &child.Barcode, &child.Active); err != nil {
			rows.Close()
			return report, err
		}
		manifest = append(manifest, child)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	legacyRepo := Repository{pool: r.pool, schema: r.schema}
	for index := range manifest {
		child := &manifest[index]
		bomID, versionID, err := legacyRepo.defaultPublishedBomTx(ctx, tx, child.ParentID)
		if err != nil {
			return report, err
		}
		if bomID > 0 && versionID > 0 {
			specID, variantID, err := legacyRepo.resolveUniqueCurrentSpecTx(ctx, tx, bomID, versionID, legacyChildMetadata{
				ID: child.ID, SpecKey: child.SpecKey, SKUCode: child.SKUCode, SpecName: child.SpecName, Barcode: child.Barcode,
			})
			if err != nil {
				return report, err
			}
			child.Mappable = specID > 0 && variantID > 0
		}
		if child.Mappable {
			report.MappableChildCount++
		}
	}
	report.LegacyChildCount = int64(len(manifest))
	if exists, err := relationExistsTx(ctx, tx, r.schema, "product_unit_templates"); err != nil {
		return report, err
	} else if exists {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_unit_templates`, r.schema)).Scan(&report.LegacyUnitTemplateCount); err != nil {
			return report, err
		}
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE output_type='product' AND specification_mode='single' AND status='active'`, r.schema)).Scan(&report.ActiveProductSingleBOMCount); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM (
			SELECT row AS identity
			FROM %[1]s.bean_list_publications publication
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(publication.content_json->'price_rows','[]'::jsonb)) row
			WHERE publication.status='published'
			UNION ALL
			SELECT item AS identity
			FROM %[1]s.bean_list_publications publication
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(publication.content_json->'groups','[]'::jsonb)) group_row
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(group_row->'items','[]'::jsonb)) item
			WHERE publication.status='published'
		) price_identity
		WHERE EXISTS (
			SELECT 1 FROM %[1]s.products child
			WHERE child.parent_product_id>0
			  AND child.id IN (
				CASE WHEN COALESCE(price_identity.identity->>'product_id','')~'^[0-9]+$' THEN (price_identity.identity->>'product_id')::bigint ELSE 0 END,
				CASE WHEN COALESCE(price_identity.identity->>'sku_id','')~'^[0-9]+$' THEN (price_identity.identity->>'sku_id')::bigint ELSE 0 END
			  )
		)
	`, r.schema)).Scan(&report.PublishedChildPriceReferenceCount); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.output_type='product' AND bom.specification_mode='single' AND bom.status='active'
		JOIN %[1]s.production_bom_versions version ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
		WHERE binding.output_type='product' AND binding.is_default=true AND version.output_qty=1
		  AND EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items item WHERE item.version_id=version.id)
		  AND EXISTS (SELECT 1 FROM %[1]s.product_unit_definitions unit WHERE unit.active=true AND (lower(unit.code)=lower(version.output_unit) OR lower(unit.name)=lower(version.output_unit)))
	`, r.schema)).Scan(&report.ConvertibleSingleBOMCount); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_bom_spec_authorities WHERE configured=true`, r.schema)).Scan(&report.ConfiguredProductCount); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products product LEFT JOIN %s.product_bom_spec_authorities authority ON authority.product_id=product.id WHERE product.active=true AND COALESCE(product.parent_product_id,0)=0 AND COALESCE(authority.configured,false)=false`, r.schema, r.schema)).Scan(&report.UnconfiguredProductCount); err != nil {
		return report, err
	}
	if report.MigrationRelationsExist {
		mappableChildIDs := make([]int64, 0, report.MappableChildCount)
		for _, child := range manifest {
			if child.Mappable {
				mappableChildIDs = append(mappableChildIDs, child.ID)
			}
		}
		dependencies, err := r.unmappedLiveDependenciesTx(ctx, tx, mappableChildIDs)
		if err != nil {
			return report, err
		}
		report.BlockingDependencies = dependencies
	}
	sort.Slice(report.BlockingDependencies, func(i, j int) bool {
		return report.BlockingDependencies[i].Table < report.BlockingDependencies[j].Table
	})
	report.ManifestID = pr622ManifestID(manifest, report.BlockingDependencies, report.ConvertibleSingleBOMCount)
	if !report.MigrationRelationsExist && report.LegacyChildCount == 0 {
		report.State = "completed"
		report.Message = "cleanup already completed"
	} else if len(report.BlockingDependencies) > 0 {
		report.State = "blocked"
		report.Message = "unmapped legacy child products still have current price, order, inventory, production, reservation, or fulfillment dependencies"
	} else {
		report.State = "ready"
	}
	return report, nil
}

func pr622ManifestID(children []pr622ManifestChild, dependencies []PR622CleanupDependency, convertible int64) string {
	manifestJSON, _ := json.Marshal(struct {
		Children     []pr622ManifestChild     `json:"children"`
		Dependencies []PR622CleanupDependency `json:"dependencies"`
		Convertible  int64                    `json:"convertible"`
	}{children, dependencies, convertible})
	digest := sha256.Sum256(manifestJSON)
	return "PR-622-" + hex.EncodeToString(digest[:8])
}

func (r PR622CleanupRepository) unmappedLiveDependenciesTx(ctx context.Context, tx pgx.Tx, mappableChildIDs []int64) ([]PR622CleanupDependency, error) {
	checks := []struct {
		table string
		query string
	}{
		{"finished_inventory", `SELECT COUNT(*) FROM %[1]s.finished_inventory ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 WHERE (COALESCE(ref.onhand_units,0)<>0 OR COALESCE(ref.onhand_loose_g,0)<>0) AND NOT(child.id=ANY($1::bigint[]))`},
		{"stock_batches", `SELECT COUNT(*) FROM %[1]s.stock_batches ref JOIN %[1]s.products child ON child.id=ref.item_id AND child.parent_product_id>0 WHERE lower(COALESCE(ref.item_type,'')) IN ('product','finished_product') AND (COALESCE(ref.remaining_g,0)<>0 OR COALESCE(ref.remaining_units,0)<>0) AND NOT(child.id=ANY($1::bigint[]))`},
		{"production_plan_items", `SELECT COUNT(*) FROM %[1]s.production_plan_items ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 JOIN %[1]s.production_plans parent ON parent.id=ref.production_plan_id WHERE lower(COALESCE(parent.status,'')) NOT IN ('completed','cancelled','closed') AND NOT(child.id=ANY($1::bigint[]))`},
		{"work_orders", `SELECT COUNT(*) FROM %[1]s.work_orders ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 WHERE lower(COALESCE(ref.status,'')) NOT IN ('completed','cancelled','closed') AND NOT(child.id=ANY($1::bigint[]))`},
		{"produce_running_items", `SELECT COUNT(*) FROM %[1]s.produce_running_items ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 WHERE lower(COALESCE(ref.status,'')) IN ('running','paused','partially_completed') AND NOT(child.id=ANY($1::bigint[]))`},
	}
	out := make([]PR622CleanupDependency, 0)
	for _, check := range checks {
		exists, err := relationExistsTx(ctx, tx, r.schema, check.table)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(check.query, r.schema), mappableChildIDs).Scan(&count); err != nil {
			return nil, fmt.Errorf("check %s dependencies: %w", check.table, err)
		}
		if count > 0 {
			out = append(out, PR622CleanupDependency{Table: check.table, Count: count})
		}
	}
	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "order_items", "order_id", "product_id"); err != nil {
		return nil, err
	} else if has {
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %[1]s.order_items ref
			JOIN %[1]s.orders parent ON parent.id=ref.order_id AND COALESCE(parent.is_void,false)=false
			JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0
			WHERE NOT(child.id=ANY($1::bigint[]))
		`, r.schema), mappableChildIDs).Scan(&count); err != nil {
			return nil, fmt.Errorf("check unfinished order dependencies: %w", err)
		}
		if count > 0 {
			out = append(out, PR622CleanupDependency{Table: "order_items_unfinished", Count: count})
		}
	}
	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "bean_list_publications", "status", "content_json"); err != nil {
		return nil, err
	} else if has {
		var priceRows, groupItems int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.bean_list_publications publication
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(publication.content_json->'price_rows','[]'::jsonb)) row
			WHERE publication.status='published'
			  AND EXISTS (
				SELECT 1 FROM %[1]s.products child
				WHERE child.parent_product_id>0
				  AND NOT(child.id=ANY($1::bigint[]))
				  AND child.id IN (
					CASE WHEN COALESCE(row->>'product_id','')~'^[0-9]+$' THEN (row->>'product_id')::bigint ELSE 0 END,
					CASE WHEN COALESCE(row->>'sku_id','')~'^[0-9]+$' THEN (row->>'sku_id')::bigint ELSE 0 END
				  )
			  )
		`, r.schema), mappableChildIDs).Scan(&priceRows); err != nil {
			return nil, fmt.Errorf("check published flat price dependencies: %w", err)
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.bean_list_publications publication
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(publication.content_json->'groups','[]'::jsonb)) group_row
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(group_row->'items','[]'::jsonb)) item
			WHERE publication.status='published'
			  AND EXISTS (
				SELECT 1 FROM %[1]s.products child
				WHERE child.parent_product_id>0
				  AND NOT(child.id=ANY($1::bigint[]))
				  AND child.id IN (
					CASE WHEN COALESCE(item->>'product_id','')~'^[0-9]+$' THEN (item->>'product_id')::bigint ELSE 0 END,
					CASE WHEN COALESCE(item->>'sku_id','')~'^[0-9]+$' THEN (item->>'sku_id')::bigint ELSE 0 END
				  )
			  )
		`, r.schema), mappableChildIDs).Scan(&groupItems); err != nil {
			return nil, fmt.Errorf("check published grouped price dependencies: %w", err)
		}
		if priceRows > 0 {
			out = append(out, PR622CleanupDependency{Table: "bean_list_publications.price_rows", Count: priceRows})
		}
		if groupItems > 0 {
			out = append(out, PR622CleanupDependency{Table: "bean_list_publications.groups.items", Count: groupItems})
		}
	}
	currentTargets := map[string]bool{
		"product_price_tiers": true, "product_price_records": true,
		"order_stock_batch_allocations": true,
		"finished_inventory":            true, "stock_batches": true,
		"production_plan_items": true, "work_orders": true, "produce_running_items": true,
		"work_order_material_reservations": true, "work_order_material_reservation_batches": true,
		"customer_inventory_items": true, "customer_custody_items": true,
		"processing_job_request_items": true, "customer_processing_production_demands": true,
		"customer_processing_material_reservations": true, "customer_processing_work_orders": true,
		"customer_processing_packaging_jobs": true, "customer_direct_ship_import_order_items": true,
		"customer_direct_ship_request_items": true, "customer_direct_ship_request_allocations": true,
		"mall_products": true,
	}
	alreadyChecked := map[string]bool{"finished_inventory": true, "stock_batches": true, "production_plan_items": true, "work_orders": true, "produce_running_items": true}
	for _, target := range businessIdentityTables {
		if !currentTargets[target.Table] || alreadyChecked[target.Table] {
			continue
		}
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
			SELECT COUNT(*) FROM %[1]s.%[2]s ref
			JOIN %[1]s.products child ON child.id=ref.%[3]s AND child.parent_product_id>0
			WHERE NOT(child.id=ANY($1::bigint[])) %[4]s
		`, r.schema, target.Table, target.ProductCol, typeClause), mappableChildIDs).Scan(&count); err != nil {
			return nil, fmt.Errorf("check %s dependencies: %w", target.Table, err)
		}
		if count > 0 {
			out = append(out, PR622CleanupDependency{Table: target.Table, Count: count})
		}
	}
	return out, nil
}

type pr622DiscardReport struct {
	References   int64
	Publications int64
	Orders       int64
}

func (r PR622CleanupRepository) mappableChildIDsTx(ctx context.Context, tx pgx.Tx) ([]int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT legacy_child_product_id
		FROM %s.legacy_child_sku_bom_spec_mappings
		WHERE bom_spec_id>0 AND bom_variant_id>0
		ORDER BY legacy_child_product_id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r PR622CleanupRepository) suspendPR622AuthorityGuardsTx(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS product_child_sku_guard ON %s.products`, r.schema)); err != nil {
		return fmt.Errorf("suspend product child SKU guard: %w", err)
	}
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
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
		qualified := pgx.Identifier{r.schema, target.Table}.Sanitize()
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS bom_spec_identity_guard ON %s`, qualified)); err != nil {
			return fmt.Errorf("suspend %s BOM specification guard: %w", target.Table, err)
		}
	}
	return nil
}

func (r PR622CleanupRepository) restorePR622AuthorityGuardsTx(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DROP TRIGGER IF EXISTS product_child_sku_guard ON %[1]s.products;
		CREATE TRIGGER product_child_sku_guard
			BEFORE INSERT OR UPDATE OF parent_product_id,auto_derived_sku,active ON %[1]s.products
			FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_product_child_sku_write()
	`, r.schema)); err != nil {
		return fmt.Errorf("restore product child SKU guard: %w", err)
	}
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
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
		qualified := pgx.Identifier{r.schema, target.Table}.Sanitize()
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DROP TRIGGER IF EXISTS bom_spec_identity_guard ON %[1]s;
			CREATE TRIGGER bom_spec_identity_guard
				BEFORE INSERT OR UPDATE OF %[2]s ON %[1]s
				FOR EACH ROW EXECUTE FUNCTION %[3]s.validate_current_product_bom_spec_identity('%[4]s','bom_spec_id','bom_variant_id','%[5]s')
		`, qualified, strings.Join(columns, ","), r.schema, target.ProductCol, target.TypeCol)); err != nil {
			return fmt.Errorf("restore %s BOM specification guard: %w", target.Table, err)
		}
	}
	return nil
}

func (r PR622CleanupRepository) discardUnmappedTestDataTx(ctx context.Context, tx pgx.Tx, actor string) (pr622DiscardReport, error) {
	report := pr622DiscardReport{}
	voided, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.orders parent
		SET is_void=true,voided_at=COALESCE(voided_at,now()),
		    void_reason=CASE WHEN COALESCE(void_reason,'')='' THEN 'PR-622 开发测试规格清理' ELSE void_reason END
		WHERE COALESCE(parent.is_void,false)=false
		  AND EXISTS (
			SELECT 1
			FROM %[1]s.order_items item
			JOIN %[1]s.products child ON child.id=item.product_id AND child.parent_product_id>0
			WHERE item.order_id=parent.id
			  AND NOT EXISTS (
				SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
				WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
			  )
		  )
	`, r.schema))
	if err != nil {
		return report, fmt.Errorf("void unmapped test orders: %w", err)
	}
	report.Orders = voided.RowsAffected()

	if has, err := tableHasColumnsTx(ctx, tx, r.schema, "bean_list_publications", "status", "content_json", "withdrawn_at", "updated_at"); err != nil {
		return report, err
	} else if has {
		withdrawn, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s.bean_list_publications publication
			SET status='withdrawn',withdrawn_at=now(),updated_at=now()
			WHERE publication.status='published'
			  AND EXISTS (
				SELECT 1
				FROM (
					SELECT row AS identity
					FROM jsonb_array_elements(COALESCE(publication.content_json->'price_rows','[]'::jsonb)) row
					UNION ALL
					SELECT item AS identity
					FROM jsonb_array_elements(COALESCE(publication.content_json->'groups','[]'::jsonb)) group_row
					CROSS JOIN LATERAL jsonb_array_elements(COALESCE(group_row->'items','[]'::jsonb)) item
				) current_identity
				JOIN %[1]s.products child ON child.parent_product_id>0 AND child.id IN (
					CASE WHEN COALESCE(current_identity.identity->>'product_id','')~'^[0-9]+$' THEN (current_identity.identity->>'product_id')::bigint ELSE 0 END,
					CASE WHEN COALESCE(current_identity.identity->>'sku_id','')~'^[0-9]+$' THEN (current_identity.identity->>'sku_id')::bigint ELSE 0 END
				)
				WHERE NOT EXISTS (
					SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
					WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
				)
			  )
		`, r.schema))
		if err != nil {
			return report, fmt.Errorf("withdraw unmapped test price publications: %w", err)
		}
		report.Publications = withdrawn.RowsAffected()
	}

	maintenance := []struct {
		table string
		query string
		args  []any
	}{
		{"product_price_tiers", `UPDATE %[1]s.product_price_tiers ref SET active=false FROM %[1]s.products child WHERE child.id=ref.product_id AND child.parent_product_id>0 AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0)`, nil},
		{"finished_inventory", `UPDATE %[1]s.finished_inventory ref SET onhand_units=0,onhand_loose_g=0,updated_at=now() FROM %[1]s.products child WHERE child.id=ref.product_id AND child.parent_product_id>0 AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0)`, nil},
		{"stock_batches", `UPDATE %[1]s.stock_batches ref SET remaining_g=0,remaining_units=0 FROM %[1]s.products child WHERE child.id=ref.item_id AND child.parent_product_id>0 AND lower(COALESCE(ref.item_type,'')) IN ('product','finished_product') AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0)`, nil},
		{"processing_job_requests", `UPDATE %[1]s.processing_job_requests parent SET status='cancelled' WHERE lower(COALESCE(parent.status,'')) NOT IN ('completed','cancelled','closed') AND EXISTS (SELECT 1 FROM %[1]s.processing_job_request_items ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 WHERE ref.request_id=parent.id AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0))`, nil},
		{"production_plans", `UPDATE %[1]s.production_plans parent SET status='cancelled',cancelled_at=COALESCE(cancelled_at,now()) WHERE lower(COALESCE(parent.status,'')) NOT IN ('completed','cancelled','closed') AND EXISTS (SELECT 1 FROM %[1]s.production_plan_items ref JOIN %[1]s.products child ON child.id=ref.product_id AND child.parent_product_id>0 WHERE ref.production_plan_id=parent.id AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0))`, nil},
		{"work_orders", `UPDATE %[1]s.work_orders ref SET status='cancelled',completed_at=COALESCE(completed_at,now()) FROM %[1]s.products child WHERE child.id=ref.product_id AND child.parent_product_id>0 AND lower(COALESCE(ref.status,'')) NOT IN ('completed','cancelled','closed') AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0)`, nil},
		{"produce_running_items", `UPDATE %[1]s.produce_running_items ref SET status='cancelled',finished_by=$1,finished_at=COALESCE(finished_at,now()) FROM %[1]s.products child WHERE child.id=ref.product_id AND child.parent_product_id>0 AND lower(COALESCE(ref.status,'')) IN ('running','paused','partially_completed') AND NOT EXISTS (SELECT 1 FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0)`, []any{actor}},
	}
	for _, item := range maintenance {
		exists, err := relationExistsTx(ctx, tx, r.schema, item.table)
		if err != nil {
			return report, err
		}
		if !exists {
			continue
		}
		result, err := tx.Exec(ctx, fmt.Sprintf(item.query, r.schema), item.args...)
		if err != nil {
			return report, fmt.Errorf("retire unmapped %s test data: %w", item.table, err)
		}
		report.References += result.RowsAffected()
	}

	deleteTargets := []businessIdentityTarget{
		{Table: "customer_direct_ship_request_allocations", ProductCol: "product_id"},
		{Table: "customer_direct_ship_request_items", ProductCol: "product_id"},
		{Table: "customer_direct_ship_import_order_items", ProductCol: "product_id"},
		{Table: "customer_processing_material_reservations", ProductCol: "component_product_id", TypeCol: "component_type"},
		{Table: "customer_processing_production_demands", ProductCol: "product_id"},
		{Table: "processing_job_request_items", ProductCol: "product_id"},
		{Table: "customer_processing_packaging_jobs", ProductCol: "product_id"},
		{Table: "customer_processing_work_orders", ProductCol: "product_id"},
		{Table: "customer_inventory_items", ProductCol: "item_id", TypeCol: "item_type"},
		{Table: "customer_custody_items", ProductCol: "item_id", TypeCol: "item_type"},
		{Table: "order_stock_batch_allocations", ProductCol: "product_id"},
		{Table: "work_order_material_reservation_batches", ProductCol: "component_id", TypeCol: "component_type"},
		{Table: "work_order_material_reservations", ProductCol: "component_id", TypeCol: "component_type"},
		{Table: "mall_products", ProductCol: "product_id"},
		{Table: "product_price_records", ProductCol: "product_id"},
	}
	for _, target := range deleteTargets {
		columns := []string{target.ProductCol}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
		if err != nil {
			return report, err
		}
		if !has {
			continue
		}
		typeClause := ""
		if target.TypeCol != "" {
			typeClause = fmt.Sprintf(" AND lower(COALESCE(ref.%s,'')) IN ('product','finished_product')", target.TypeCol)
		}
		qualified := pgx.Identifier{r.schema, target.Table}.Sanitize()
		result, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %[1]s ref USING %[2]s.products child
			WHERE child.id=ref.%[3]s AND child.parent_product_id>0 %[4]s
			  AND NOT EXISTS (
				SELECT 1 FROM %[2]s.legacy_child_sku_bom_spec_mappings mapping
				WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
			  )
		`, qualified, r.schema, target.ProductCol, typeClause))
		if err != nil {
			return report, fmt.Errorf("delete unmapped %s test data: %w", target.Table, err)
		}
		report.References += result.RowsAffected()
	}

	parentified, err := r.parentifyUnmappedReferencesTx(ctx, tx)
	if err != nil {
		return report, err
	}
	report.References += parentified
	return report, nil
}

func (r PR622CleanupRepository) parentifyUnmappedReferencesTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	var changed int64
	for _, target := range businessIdentityTables {
		columns := []string{target.ProductCol}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
		if err != nil {
			return 0, err
		}
		if !has {
			continue
		}
		hasIdentity, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, "bom_spec_id", "bom_variant_id")
		if err != nil {
			return 0, err
		}
		setClause := fmt.Sprintf("%s=child.parent_product_id", target.ProductCol)
		if hasIdentity {
			setClause += ",bom_spec_id=0,bom_variant_id=0"
		}
		typeClause := ""
		if target.TypeCol != "" {
			typeClause = fmt.Sprintf(" AND lower(COALESCE(ref.%s,'')) IN ('product','finished_product')", target.TypeCol)
		}
		qualified := pgx.Identifier{r.schema, target.Table}.Sanitize()
		result, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %[1]s ref SET %[2]s
			FROM %[3]s.products child
			WHERE child.id=ref.%[4]s AND child.parent_product_id>0 %[5]s
			  AND NOT EXISTS (
				SELECT 1 FROM %[3]s.legacy_child_sku_bom_spec_mappings mapping
				WHERE mapping.legacy_child_product_id=child.id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
			  )
		`, qualified, setClause, r.schema, target.ProductCol, typeClause))
		if err != nil {
			return 0, fmt.Errorf("retire unmapped %s product identities: %w", target.Table, err)
		}
		changed += result.RowsAffected()
	}
	return changed, nil
}

func (r PR622CleanupRepository) convertPublishedSingleBOMsTx(ctx context.Context, tx pgx.Tx, actor string) (int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT binding.output_id,bom.id,version.id,version.output_unit
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.output_type='product' AND bom.specification_mode='single' AND bom.status='active'
		JOIN %[1]s.production_bom_versions version ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
		WHERE binding.output_type='product' AND binding.is_default=true AND version.output_qty=1
		  AND EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items item WHERE item.version_id=version.id)
		  AND EXISTS (SELECT 1 FROM %[1]s.product_unit_definitions unit WHERE unit.active=true AND (lower(unit.code)=lower(version.output_unit) OR lower(unit.name)=lower(version.output_unit)))
		ORDER BY binding.output_id
	`, r.schema))
	if err != nil {
		return 0, err
	}
	type candidate struct {
		productID, bomID, versionID int64
		unit                        string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var row candidate
		if err := rows.Scan(&row.productID, &row.bomID, &row.versionID, &row.unit); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, row := range candidates {
		var replacementBOMID, replacementVersionID, specID, variantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_boms(code,name,output_type,specification_mode,output_product_id,output_material_id,group_id,group_category_id,status,source_bom_id,source_bom_version_id,source_product_id,source_product_code_snapshot,source_product_name_snapshot,created_by,updated_by)
			SELECT 'PR622-' || $1::bigint::text || '-' || id::text,name,'product','spec_group',output_product_id,0,group_id,group_category_id,'active',id,$2,$1::bigint,source_product_code_snapshot,source_product_name_snapshot,$3,$3
			FROM %[1]s.production_boms WHERE id=$4 RETURNING id
		`, r.schema), row.productID, row.versionID, actor, row.bomID).Scan(&replacementBOMID); err != nil {
			return 0, err
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_bom_versions(bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,note,legacy_product_id,legacy_bom_version_id,special_attrs_schema_json,special_attrs_json,process_route_id,source_spec_template_version_id,main_input_material_id,created_at,published_at,created_by,published_by)
			SELECT $1,'V001','published',yield_rate,material_loss_rate,1,output_unit,COALESCE(note,'') || '；PR-622 单一产出替代 BOM',0,id,special_attrs_schema_json,special_attrs_json,process_route_id,0,main_input_material_id,now(),now(),$2,$2
			FROM %[1]s.production_bom_versions WHERE id=$3 RETURNING id
		`, r.schema), replacementBOMID, actor, row.versionID).Scan(&replacementVersionID); err != nil {
			return 0, err
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit,created_by,updated_by) VALUES($1,'PR622-SPEC-' || $2::bigint::text,'default','默认规格',$3,$4,$4) RETURNING id`, r.schema), replacementBOMID, row.productID, row.unit, actor).Scan(&specID); err != nil {
			return 0, err
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) SELECT $1,$2,'默认规格',$3,true,100,material_loss_rate,process_route_id FROM %s.production_bom_versions WHERE id=$1 RETURNING id`, r.schema, r.schema), replacementVersionID, specID, row.unit).Scan(&variantID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot)
			SELECT $1,$2,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot
			FROM %[1]s.production_bom_version_items WHERE version_id=$3 ORDER BY id
		`, r.schema), replacementVersionID, variantID, row.versionID); err != nil {
			return 0, err
		}
		if exists, err := relationExistsTx(ctx, tx, r.schema, "production_bom_version_operation_costs"); err != nil {
			return 0, err
		} else if exists {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %[1]s.production_bom_version_operation_costs(version_id,operation_id,operation_name,workstation_id,workstation_name,workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order)
				SELECT $1,operation_id,operation_name,workstation_id,workstation_name,workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order FROM %[1]s.production_bom_version_operation_costs WHERE version_id=$2
			`, r.schema), replacementVersionID, row.versionID); err != nil {
				return 0, err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), row.bomID, actor); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_output_bindings SET bom_id=$2,bom_version_id=$3,is_default=true,updated_at=now(),updated_by=$4 WHERE output_type='product' AND output_id=$1`, r.schema), row.productID, replacementBOMID, replacementVersionID, actor); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_at,bound_by) VALUES($1,$2,$3,now(),$4) ON CONFLICT(product_id) DO UPDATE SET bom_id=EXCLUDED.bom_id,bom_version_id=EXCLUDED.bom_version_id,bound_at=now(),bound_by=EXCLUDED.bound_by`, r.schema), row.productID, replacementBOMID, replacementVersionID, actor); err != nil {
			return 0, err
		}
	}
	return int64(len(candidates)), nil
}

func (r PR622CleanupRepository) detachRemainingSingleProductBOMsTx(ctx context.Context, tx pgx.Tx, actor string) (int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT output_product_id,id
		FROM %[1]s.production_boms
		WHERE output_type='product' AND specification_mode='single' AND status='active'
		ORDER BY output_product_id,id
	`, r.schema))
	if err != nil {
		return 0, err
	}
	type candidate struct{ productID, bomID int64 }
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.productID, &item.bomID); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, item := range candidates {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_output_bindings WHERE output_type='product' AND bom_id=$1`, r.schema), item.bomID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_production_bom_bindings WHERE bom_id=$1`, r.schema), item.bomID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), item.bomID, actor); err != nil {
			return 0, err
		}
	}
	return int64(len(candidates)), nil
}

func (r PR622CleanupRepository) rewriteMappedReferencesTx(ctx context.Context, tx pgx.Tx, target businessIdentityTarget) (int64, error) {
	columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
	if target.TypeCol != "" {
		columns = append(columns, target.TypeCol)
	}
	has, err := tableHasColumnsTx(ctx, tx, r.schema, target.Table, columns...)
	if err != nil || !has {
		return 0, err
	}
	typeClause := ""
	if target.TypeCol != "" {
		typeClause = fmt.Sprintf(" AND lower(COALESCE(target.%s,'')) IN ('product','finished_product')", target.TypeCol)
	}
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.%[2]s target
		SET %[3]s=mapping.parent_product_id,bom_spec_id=mapping.bom_spec_id,bom_variant_id=mapping.bom_variant_id
		FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
		WHERE target.%[3]s=mapping.legacy_child_product_id AND mapping.bom_spec_id>0 AND mapping.bom_variant_id>0 %[4]s
	`, r.schema, target.Table, target.ProductCol, typeClause))
	if err != nil {
		return 0, fmt.Errorf("rewrite %s product specification identities: %w", target.Table, err)
	}
	return result.RowsAffected(), nil
}

// Voided orders are immutable historical snapshots. They keep their frozen
// item/spec/unit/price text while the obsolete FK is moved to the parent
// product, so deleting a retired child SKU cannot break historical display.
func (r PR622CleanupRepository) rewriteHistoricalOrderSnapshotsTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	has, err := tableHasColumnsTx(ctx, tx, r.schema, "order_items", "order_id", "product_id", "bom_spec_id", "bom_variant_id")
	if err != nil || !has {
		return 0, err
	}
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.order_items item
		SET product_id=child.parent_product_id,bom_spec_id=0,bom_variant_id=0
		FROM %[1]s.products child,%[1]s.orders parent
		WHERE item.product_id=child.id AND child.parent_product_id>0
		  AND parent.id=item.order_id AND COALESCE(parent.is_void,false)=true
	`, r.schema))
	if err != nil {
		return 0, fmt.Errorf("rewrite voided historical order snapshot references: %w", err)
	}
	return result.RowsAffected(), nil
}

type pr622PublishedIdentity struct {
	ParentProductID int64
	BOMID           int64
	BOMVersionID    int64
	BOMSpecID       int64
	BOMVariantID    int64
}

func (r PR622CleanupRepository) clonePublishedPriceVersionsTx(ctx context.Context, tx pgx.Tx, actor string) (int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT mapping.legacy_child_product_id,mapping.parent_product_id,mapping.bom_id,
		       variant.version_id,mapping.bom_spec_id,mapping.bom_variant_id
		FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
		JOIN %[1]s.production_bom_version_variants variant ON variant.id=mapping.bom_variant_id
		WHERE mapping.bom_spec_id>0 AND mapping.bom_variant_id>0
	`, r.schema))
	if err != nil {
		return 0, err
	}
	identities := map[int64]pr622PublishedIdentity{}
	for rows.Next() {
		var childID int64
		var identity pr622PublishedIdentity
		if err := rows.Scan(&childID, &identity.ParentProductID, &identity.BOMID, &identity.BOMVersionID, &identity.BOMSpecID, &identity.BOMVariantID); err != nil {
			rows.Close()
			return 0, err
		}
		identities[childID] = identity
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	type publication struct {
		id, productTypeCategoryID, classificationTemplateID, classificationCategoryID int64
		priceSourcePublicationID, styleSourcePublicationID                            *int64
		purpose, listType, productTypeName, classificationTemplateName                string
		classificationCategoryName, versionNo, ownerType, ownerKey, sourceVersionNo   string
		configJSON, contentJSON, changelog                                            string
	}
	publicationRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,publication_purpose,list_type,product_type_category_id,product_type_name,
		       classification_template_id,classification_template_name,classification_category_id,classification_category_name,
		       version_no,owner_type,owner_key,price_source_publication_id,style_source_publication_id,source_version_no,
		       config_json::text,content_json::text,changelog
		FROM %s.bean_list_publications WHERE status='published' ORDER BY id
	`, r.schema))
	if err != nil {
		return 0, err
	}
	publications := make([]publication, 0)
	for publicationRows.Next() {
		var item publication
		if err := publicationRows.Scan(&item.id, &item.purpose, &item.listType, &item.productTypeCategoryID, &item.productTypeName,
			&item.classificationTemplateID, &item.classificationTemplateName, &item.classificationCategoryID, &item.classificationCategoryName,
			&item.versionNo, &item.ownerType, &item.ownerKey, &item.priceSourcePublicationID, &item.styleSourcePublicationID, &item.sourceVersionNo,
			&item.configJSON, &item.contentJSON, &item.changelog); err != nil {
			publicationRows.Close()
			return 0, err
		}
		publications = append(publications, item)
	}
	if err := publicationRows.Err(); err != nil {
		publicationRows.Close()
		return 0, err
	}
	publicationRows.Close()

	var cloned int64
	for _, item := range publications {
		var config, content any
		if err := json.Unmarshal([]byte(item.configJSON), &config); err != nil {
			return 0, fmt.Errorf("decode publication %d config: %w", item.id, err)
		}
		if err := json.Unmarshal([]byte(item.contentJSON), &content); err != nil {
			return 0, fmt.Errorf("decode publication %d content: %w", item.id, err)
		}
		config, configChanged := rewritePR622PublishedJSON(config, identities)
		content, contentChanged := rewritePR622PublishedJSON(content, identities)
		if !configChanged && !contentChanged {
			continue
		}
		configBytes, _ := json.Marshal(config)
		contentBytes, _ := json.Marshal(content)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bean_list_publications SET status='withdrawn',withdrawn_at=now(),updated_at=now() WHERE id=$1 AND status='published'`, r.schema), item.id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.bean_list_publications(
				publication_purpose,list_type,product_type_category_id,product_type_name,
				classification_template_id,classification_template_name,classification_category_id,classification_category_name,
				version_no,status,owner_type,owner_key,price_source_publication_id,style_source_publication_id,source_version_no,
				config_json,content_json,changelog,actor,published_at,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'published',$10,$11,$12,$13,$14,$15::jsonb,$16::jsonb,$17,$18,now(),now(),now())
		`, r.schema), item.purpose, item.listType, item.productTypeCategoryID, item.productTypeName,
			item.classificationTemplateID, item.classificationTemplateName, item.classificationCategoryID, item.classificationCategoryName,
			item.versionNo+"-PR622", item.ownerType, item.ownerKey, item.priceSourcePublicationID, item.styleSourcePublicationID, item.sourceVersionNo,
			string(configBytes), string(contentBytes), strings.TrimSpace(item.changelog)+"；PR-622 规格身份切换", actor); err != nil {
			return 0, fmt.Errorf("clone published price version %d: %w", item.id, err)
		}
		cloned++
	}
	return cloned, nil
}

func rewritePR622PublishedJSON(value any, identities map[int64]pr622PublishedIdentity) (any, bool) {
	switch typed := value.(type) {
	case []any:
		changed := false
		for index := range typed {
			var itemChanged bool
			typed[index], itemChanged = rewritePR622PublishedJSON(typed[index], identities)
			changed = changed || itemChanged
		}
		return typed, changed
	case map[string]any:
		changed := false
		for key, child := range typed {
			var itemChanged bool
			typed[key], itemChanged = rewritePR622PublishedJSON(child, identities)
			changed = changed || itemChanged
		}
		var identity pr622PublishedIdentity
		matched := false
		for _, key := range []string{"sku_id", "skuId", "skuID", "product_id", "productId", "productID"} {
			if id := pr622JSONInt64(typed[key]); id > 0 {
				if candidate, ok := identities[id]; ok {
					identity, matched = candidate, true
					break
				}
			}
		}
		if !matched {
			return typed, changed
		}
		for _, key := range []string{"sku_id", "skuId", "skuID", "product_id", "productId", "productID", "parent_product_id"} {
			if _, exists := typed[key]; exists {
				typed[key] = identity.ParentProductID
			}
		}
		typed["bom_id"] = identity.BOMID
		typed["bom_version_id"] = identity.BOMVersionID
		typed["bom_spec_id"] = identity.BOMSpecID
		typed["bom_variant_id"] = identity.BOMVariantID
		typed["spec_identity_mode"] = "bom_spec"
		typed["migration_state"] = "cutover"
		typed["bom_spec_authoritative"] = true
		return typed, true
	default:
		return value, false
	}
}

func pr622JSONInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		var parsed int64
		_, _ = fmt.Sscan(strings.TrimSpace(typed), &parsed)
		return parsed
	default:
		return 0
	}
}

func (r PR622CleanupRepository) rewriteFamilyCatalogReferencesTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	var changed int64
	for _, table := range []string{"customer_product_aliases", "product_customer_references"} {
		has, err := tableHasColumnsTx(ctx, tx, r.schema, table, "product_id")
		if err != nil || !has {
			if err != nil {
				return 0, err
			}
			continue
		}
		if table == "product_customer_references" {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %[1]s.product_customer_references child_ref USING %[1]s.products child WHERE child.id=child_ref.product_id AND child.parent_product_id>0 AND EXISTS (SELECT 1 FROM %[1]s.product_customer_references parent_ref WHERE parent_ref.product_id=child.parent_product_id AND parent_ref.customer_id=child_ref.customer_id)`, r.schema)); err != nil {
				return 0, err
			}
		}
		result, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.%[2]s ref SET product_id=child.parent_product_id FROM %[1]s.products child WHERE ref.product_id=child.id AND child.parent_product_id>0`, r.schema, table))
		if err != nil {
			return 0, err
		}
		changed += result.RowsAffected()
	}
	return changed, nil
}

func (r PR622CleanupRepository) removeChildCatalogRowsTx(ctx context.Context, tx pgx.Tx) error {
	for _, table := range []string{"product_classification_assignments", "product_production_configs"} {
		has, err := tableHasColumnsTx(ctx, tx, r.schema, table, "product_id")
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %[1]s.%[2]s ref USING %[1]s.products child WHERE ref.product_id=child.id AND child.parent_product_id>0`, r.schema, table)); err != nil {
			return err
		}
	}
	return nil
}

func (r PR622CleanupRepository) clearLegacyUnitMetadataTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET unit_template_id=0,default_sku_id=0,derived_unit_template_id=0,unit_rule_override_json='{}'::jsonb WHERE unit_template_id<>0 OR default_sku_id<>0 OR derived_unit_template_id<>0 OR unit_rule_override_json<>'{}'::jsonb`, r.schema)); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `
		SELECT col.table_name
		FROM information_schema.columns col
		JOIN information_schema.tables relation
		  ON relation.table_schema=col.table_schema AND relation.table_name=col.table_name AND relation.table_type='BASE TABLE'
		WHERE col.table_schema=$1 AND col.column_name='unit_template_id' AND col.table_name<>'product_unit_templates'
		ORDER BY col.table_name
	`, r.schema)
	if err != nil {
		return 0, err
	}
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return 0, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, table := range tables {
		qualified := pgx.Identifier{r.schema, table}.Sanitize()
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET unit_template_id=0 WHERE COALESCE(unit_template_id,0)<>0`, qualified)); err != nil {
			return 0, fmt.Errorf("clear %s unit template binding: %w", table, err)
		}
	}
	exists, err := relationExistsTx(ctx, tx, r.schema, "product_unit_templates")
	if err != nil || !exists {
		return 0, err
	}
	result, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_unit_templates`, r.schema))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r PR622CleanupRepository) dropLegacyMigrationRelationsTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		DROP FUNCTION IF EXISTS %[1]s.validate_legacy_child_product_write() CASCADE;
		DROP FUNCTION IF EXISTS %[1]s.validate_bom_spec_business_identity() CASCADE;
		DROP TABLE IF EXISTS %[1]s.legacy_child_sku_bom_spec_mappings CASCADE;
		DROP TABLE IF EXISTS %[1]s.product_bom_spec_authority_upgrades CASCADE;
		DROP TABLE IF EXISTS %[1]s.product_bom_spec_migrations CASCADE;
	`, r.schema))
	return err
}
