package productspecmigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) Get(ctx context.Context, productID int64) (productspecmigrationapp.ProductMigration, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	defer tx.Rollback(ctx)
	row, err := r.loadMigrationTx(ctx, tx, productID, false)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	return row, nil
}

func (r Repository) Prepare(ctx context.Context, cmd productspecmigrationapp.PrepareCommand) (productspecmigrationapp.ProductMigration, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.requireParentProductTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := r.ensureMigrationRowTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	current, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, true)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if current.State == productspecmigrationapp.StateCutover {
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		return current, nil
	}
	if err := r.refreshMappingsTx(ctx, tx, cmd.ProductID, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_migrations
		SET state='preparing',prepared_at=COALESCE(prepared_at,now()),prepared_by=CASE WHEN prepared_by='' THEN $2 ELSE prepared_by END,updated_at=now()
		WHERE product_id=$1
	`, r.schema), cmd.ProductID, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_migration", &cmd.ProductID, "prepare", postgresinfra.StrPtr("state"), postgresinfra.StrPtr(string(current.State)), postgresinfra.StrPtr(string(productspecmigrationapp.StatePreparing)), postgresinfra.AuditMeta{
		"metadata_only":       true,
		"recipe_rows_created": 0,
	}); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	row, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, false)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	return row, nil
}

func (r Repository) Assess(ctx context.Context, cmd productspecmigrationapp.AssessCommand) (productspecmigrationapp.ProductMigration, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.requireParentProductTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := r.ensureMigrationRowTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	current, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, true)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if current.State == productspecmigrationapp.StateCutover {
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		return current, nil
	}
	if err := r.refreshMappingsTx(ctx, tx, cmd.ProductID, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	readiness, err := r.assessReadinessTx(ctx, tx, cmd.ProductID)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	state := productspecmigrationapp.StatePreparing
	if readiness.Ready {
		state = productspecmigrationapp.StateReady
	}
	if err := r.saveReadinessTx(ctx, tx, cmd.ProductID, state, readiness, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_migration", &cmd.ProductID, "assess_readiness", postgresinfra.StrPtr("state"), postgresinfra.StrPtr(string(current.State)), postgresinfra.StrPtr(string(state)), postgresinfra.AuditMeta{
		"ready":    readiness.Ready,
		"blockers": readiness.Blockers,
	}); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	row, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, false)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	return row, nil
}

func (r Repository) Cutover(ctx context.Context, cmd productspecmigrationapp.CutoverCommand) (productspecmigrationapp.ProductMigration, error) {
	// The migration row lock is the cutover serialization point. Business-write
	// triggers take a share lock on the same row, so READ COMMITTED lets the
	// readiness queries observe every write that committed before this lock was
	// acquired while later writes wait and re-evaluate the cutover state.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	defer tx.Rollback(ctx)
	if err := LockLegacySpecWriteRetirementTx(ctx, tx, r.schema); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := r.requireParentProductTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := r.ensureMigrationRowTx(ctx, tx, cmd.ProductID); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	current, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, true)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if current.State == productspecmigrationapp.StateCutover {
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		return current, nil
	}
	if err := r.refreshMappingsTx(ctx, tx, cmd.ProductID, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	readiness, err := r.assessReadinessTx(ctx, tx, cmd.ProductID)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if !readiness.Ready {
		if err := r.saveReadinessTx(ctx, tx, cmd.ProductID, productspecmigrationapp.StatePreparing, readiness, cmd.Actor); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_migration", &cmd.ProductID, "cutover_blocked", postgresinfra.StrPtr("state"), postgresinfra.StrPtr(string(current.State)), postgresinfra.StrPtr(string(productspecmigrationapp.StatePreparing)), postgresinfra.AuditMeta{"blockers": readiness.Blockers}); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return productspecmigrationapp.ProductMigration{}, err
		}
		return productspecmigrationapp.ProductMigration{}, &productspecmigrationapp.CutoverBlockedError{Readiness: readiness}
	}

	// Tombstone only. Product IDs, barcodes and legacy snapshots remain queryable.
	tombstoneResult, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products child
			SET active=false,derived_spec_status='bom_spec_cutover'
			WHERE child.active=true AND %s
		`, r.schema, legacyChildCandidatePredicate("child")), cmd.ProductID)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.legacy_child_sku_bom_spec_mappings
		SET tombstoned_at=COALESCE(tombstoned_at,now()),updated_at=now(),updated_by=$2
		WHERE parent_product_id=$1
	`, r.schema), cmd.ProductID, cmd.Actor); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_migrations
		SET state='cutover',readiness_json=$3::jsonb,ready_at=COALESCE(ready_at,now()),ready_by=CASE WHEN ready_by='' THEN $2 ELSE ready_by END,
			cutover_at=now(),cutover_by=$2,updated_at=now()
		WHERE product_id=$1
	`, r.schema), cmd.ProductID, cmd.Actor, mustJSON(readiness)); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_spec_migration", &cmd.ProductID, "cutover", postgresinfra.StrPtr("state"), postgresinfra.StrPtr(string(current.State)), postgresinfra.StrPtr(string(productspecmigrationapp.StateCutover)), postgresinfra.AuditMeta{
		"legacy_children_tombstoned":     tombstoneResult.RowsAffected(),
		"historical_snapshots_rewritten": false,
	}); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	row, err := r.loadMigrationTx(ctx, tx, cmd.ProductID, false)
	if err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productspecmigrationapp.ProductMigration{}, err
	}
	return row, nil
}

func mustJSON(value any) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func (r Repository) requireParentProductTx(ctx context.Context, tx pgx.Tx, productID int64) error {
	var parentID, baseProductID int64
	var customType string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(parent_product_id,0),COALESCE(base_product_id,0),COALESCE(custom_type,'')
		FROM %s.products WHERE id=$1
	`, r.schema), productID).Scan(&parentID, &baseProductID, &customType)
	if errors.Is(err, pgx.ErrNoRows) {
		return productspecmigrationapp.ErrMigrationNotFound
	}
	if err != nil {
		return err
	}
	if parentID > 0 {
		return fmt.Errorf("product %d is a legacy child SKU; migrate parent product %d", productID, parentID)
	}
	if baseProductID > 0 && !strings.EqualFold(strings.TrimSpace(customType), "public_sku_alias") {
		return fmt.Errorf("product %d is a legacy child SKU; migrate parent product %d", productID, baseProductID)
	}
	return nil
}

func (r Repository) ensureMigrationRowTx(ctx context.Context, tx pgx.Tx, productID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state)
		VALUES($1,'legacy') ON CONFLICT(product_id) DO NOTHING
	`, r.schema), productID)
	return err
}

func (r Repository) saveReadinessTx(ctx context.Context, tx pgx.Tx, productID int64, state productspecmigrationapp.MigrationState, readiness productspecmigrationapp.Readiness, actor string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_bom_spec_migrations
		SET state=$2,readiness_json=$3::jsonb,
			ready_at=CASE WHEN $2='ready' THEN COALESCE(ready_at,now()) ELSE NULL END,
			ready_by=CASE WHEN $2='ready' THEN CASE WHEN ready_by='' THEN $4 ELSE ready_by END ELSE '' END,
			prepared_at=COALESCE(prepared_at,now()),prepared_by=CASE WHEN prepared_by='' THEN $4 ELSE prepared_by END,
			updated_at=now()
		WHERE product_id=$1
	`, r.schema), productID, string(state), mustJSON(readiness), actor)
	return err
}

func (r Repository) loadMigrationTx(ctx context.Context, tx pgx.Tx, productID int64, lock bool) (productspecmigrationapp.ProductMigration, error) {
	lockSQL := ""
	if lock {
		lockSQL = " FOR UPDATE"
	}
	var row productspecmigrationapp.ProductMigration
	var readinessJSON string
	var state string
	var storedIdentityMode string
	legacyCatalogProduct := true
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,state,COALESCE((to_jsonb(product_bom_spec_migrations)->>'legacy_catalog_product')::boolean,true),COALESCE(to_jsonb(product_bom_spec_migrations)->>'spec_identity_mode',''),readiness_json::text,prepared_at,ready_at,cutover_at,updated_at
		FROM %s.product_bom_spec_migrations WHERE product_id=$1%s
	`, r.schema, lockSQL), productID).Scan(&row.ProductID, &state, &legacyCatalogProduct, &storedIdentityMode, &readinessJSON, &row.PreparedAt, &row.ReadyAt, &row.CutoverAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if scanErr := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.products WHERE id=$1)`, r.schema), productID).Scan(&exists); scanErr != nil {
			return row, scanErr
		}
		if !exists {
			return row, productspecmigrationapp.ErrMigrationNotFound
		}
		row.ProductID = productID
		row.State = productspecmigrationapp.StateLegacy
		row.MigrationState = row.State
		row.LegacyCatalogProduct = true
		row.SpecIdentityMode = productspecmigrationapp.SpecIdentityMode(row.State, row.LegacyCatalogProduct)
		row.BomSpecAuthoritative = false
		row.Mappings = []productspecmigrationapp.LegacyMapping{}
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.State = productspecmigrationapp.MigrationState(state)
	row.MigrationState = row.State
	row.LegacyCatalogProduct = legacyCatalogProduct
	row.SpecIdentityMode = productspecmigrationapp.ResolveSpecIdentityMode(storedIdentityMode, row.State, legacyCatalogProduct)
	row.BomSpecAuthoritative = row.SpecIdentityMode == productspecmigrationapp.SpecIdentityModeBOMSpec
	if strings.TrimSpace(readinessJSON) != "" {
		_ = json.Unmarshal([]byte(readinessJSON), &row.Readiness)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,parent_product_id,legacy_child_product_id,legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,bom_spec_id,bom_variant_id,metadata_snapshot::text,tombstoned_at
		FROM %s.legacy_child_sku_bom_spec_mappings
		WHERE parent_product_id=$1 ORDER BY legacy_spec_key,legacy_child_product_id
	`, r.schema), productID)
	if err != nil {
		return row, err
	}
	defer rows.Close()
	row.Mappings = make([]productspecmigrationapp.LegacyMapping, 0)
	for rows.Next() {
		var mapping productspecmigrationapp.LegacyMapping
		var variantID int64
		if err := rows.Scan(&mapping.ID, &mapping.ParentProductID, &mapping.LegacyChildProductID, &mapping.LegacySpecKey, &mapping.LegacySpecName, &mapping.LegacySalesUnit, &mapping.LegacySpecG, &mapping.BomSpecID, &variantID, &mapping.MetadataSnapshot, &mapping.TombstonedAt); err != nil {
			return row, err
		}
		if variantID > 0 {
			mapping.BomVariantID = &variantID
		}
		row.Mappings = append(row.Mappings, mapping)
	}
	return row, rows.Err()
}
