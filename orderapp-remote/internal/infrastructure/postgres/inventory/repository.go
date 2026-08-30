package inventory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	inventoryapp "orderapp/internal/application/inventory"
	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	inventorydomain "orderapp/internal/domain/inventory"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stockItemTypeFinishedProduct = "finished_product"
	stockSourceManualAdjustment  = "manual_adjustment"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListFinished(ctx context.Context, query inventoryapp.FinishedInventoryQuery) (inventoryapp.FinishedInventoryResult, error) {
	hasSpecCatalog, err := r.hasFinishedSpecCatalog(ctx)
	if err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	identitySelect := `fi.bom_spec_id,fi.bom_variant_id,''::text,''::text,''::text,''::text,'legacy_sku'::text,false`
	identityJoins := ""
	if hasSpecCatalog {
		identitySelect = `fi.bom_spec_id,fi.bom_variant_id,
		       COALESCE(spec.spec_key,''),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
		       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),NULLIF(to_jsonb(p)->'unit_rule_override_json'->>'inventory_unit',''),''),
		       COALESCE(NULLIF(migration.state,''),'legacy'),
		       COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END),
		       COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END)='bom_spec'`
		identityJoins = fmt.Sprintf(`
		LEFT JOIN %s.production_bom_specs spec ON spec.id=fi.bom_spec_id
		LEFT JOIN %s.production_bom_version_variants variant
		  ON variant.id=fi.bom_variant_id AND variant.bom_spec_id=fi.bom_spec_id
		LEFT JOIN %s.product_bom_spec_migrations migration ON migration.product_id=fi.product_id
	`, r.schema, r.schema, r.schema)
	}
	where := ""
	args := []any{}
	argn := 1
	if s := strings.TrimSpace(query.Q); s != "" {
		where = fmt.Sprintf("WHERE p.name ILIKE $%d", argn)
		args = append(args, "%"+s+"%")
		argn++
	}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.finished_inventory fi
		LEFT JOIN %s.products p ON p.id = fi.product_id
		%s
	`, r.schema, r.schema, where), args...).Scan(&total); err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg := argn
	offsetArg := argn + 1

	sql := fmt.Sprintf(`
		SELECT fi.product_id, COALESCE(p.name,''), fi.spec_g,%s,
		       fi.warehouse, fi.onhand_units, fi.onhand_loose_g,
		       to_char(fi.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finished_inventory fi
		LEFT JOIN %s.products p ON p.id = fi.product_id
		%s
		%s
		ORDER BY COALESCE(p.name,''), fi.bom_spec_id, fi.spec_g
		LIMIT $%d OFFSET $%d
	`, identitySelect, r.schema, r.schema, identityJoins, where, limitArg, offsetArg)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	defer rows.Close()

	out := make([]inventoryapp.FinishedInventoryRow, 0)
	for rows.Next() {
		var row inventoryapp.FinishedInventoryRow
		if err := rows.Scan(
			&row.ProductID, &row.Product, &row.SpecG,
			&row.BomSpecID, &row.BomVariantID, &row.SpecKey, &row.SpecName, &row.InventoryUnit, &row.MigrationState, &row.SpecIdentityMode, &row.BomSpecAuthoritative,
			&row.Warehouse, &row.Units, &row.LooseG, &row.UpdatedAt,
		); err != nil {
			return inventoryapp.FinishedInventoryResult{}, err
		}
		if row.BomSpecID == 0 {
			if total, err := inventorydomain.TotalGrams(row.SpecG, inventorydomain.Quantity{Units: row.Units, LooseG: row.LooseG}); err == nil {
				row.TotalG = total
			}
		} else {
			row.SpecG = 0
			row.LooseG = 0
			row.TotalG = 0
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}

	products, err := r.listProducts(ctx)
	if err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return inventoryapp.FinishedInventoryResult{Rows: out, Products: products, Total: total, HasNext: hasNext}, nil
}

func (r Repository) AdjustFinished(ctx context.Context, cmd inventoryapp.AdjustFinishedInventoryCommand) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), cmd.ProductID).Scan(&productName); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("product not found")
		}
		return err
	}
	identity, err := r.resolveFinishedBomSpecIdentityTx(ctx, tx, cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, cmd.UnitCode)
	if err != nil {
		return err
	}
	if cmd.BomSpecID > 0 || (cmd.BomSpecID == 0 && cmd.SpecG == 0 && strings.TrimSpace(identity.InventoryUnit) != "") {
		cmd.BomVariantID = identity.BomVariantID
		cmd.UnitCode = identity.InventoryUnit
	}
	if cmd.BomSpecID == 0 && cmd.SpecG == 0 && strings.TrimSpace(identity.InventoryUnit) == "" {
		return fmt.Errorf("direct product inventory identity is not enabled for product %d", cmd.ProductID)
	}

	before := inventorydomain.Quantity{}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND warehouse=$4
		FOR UPDATE
	`, r.schema), cmd.ProductID, cmd.BomSpecID, cmd.SpecG, cmd.Warehouse).Scan(&before.Units, &before.LooseG)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	after := inventorydomain.Quantity{Units: cmd.Units, LooseG: cmd.LooseG}
	beforeG, afterG := int64(0), int64(0)
	if cmd.BomSpecID == 0 && cmd.SpecG > 0 {
		beforeG, err = inventorydomain.TotalGrams(cmd.SpecG, before)
		if err != nil {
			return err
		}
		afterG, err = inventorydomain.TotalGrams(cmd.SpecG, after)
		if err != nil {
			return err
		}
	}
	changeG := afterG - beforeG
	changeUnits := after.Units - before.Units

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
		SET bom_variant_id=excluded.bom_variant_id,onhand_units=excluded.onhand_units,
		    onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, r.schema), cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, cmd.SpecG, cmd.Warehouse, after.Units, after.LooseG); err != nil {
		return err
	}

	batchCode := manualAdjustmentBatchCode()
	if err := r.insertFinishedAdjustmentBatchTx(ctx, tx, batchCode, productName, cmd, changeG, changeUnits); err != nil {
		return err
	}

	if err := r.insertFinishedAdjustmentLedgerTx(ctx, tx, batchCode, productName, cmd, before, after, beforeG, changeG, afterG); err != nil {
		return err
	}

	oldValue := fmt.Sprintf("%d+%dg", before.Units, before.LooseG)
	newValue := fmt.Sprintf("%d+%dg", after.Units, after.LooseG)
	if cmd.BomSpecID > 0 || cmd.SpecG == 0 {
		oldValue = fmt.Sprintf("%d %s", before.Units, cmd.UnitCode)
		newValue = fmt.Sprintf("%d %s", after.Units, cmd.UnitCode)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "finished_inventory", nil, "adjust", postgresinfra.StrPtr("quantity"), postgresinfra.StrPtr(oldValue), postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{
		"product_id":     cmd.ProductID,
		"bom_spec_id":    cmd.BomSpecID,
		"bom_variant_id": cmd.BomVariantID,
		"inventory_unit": cmd.UnitCode,
		"spec_g":         cmd.SpecG,
		"warehouse":      cmd.Warehouse,
		"change_g":       changeG,
		"change_units":   changeUnits,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r Repository) ListAllocations(ctx context.Context, query inventoryapp.AllocationLogQuery) (inventoryapp.AllocationLogResult, error) {
	batches, total, hasNext, err := r.listAllocationBatches(ctx, query.Limit, query.Offset)
	if err != nil {
		return inventoryapp.AllocationLogResult{}, err
	}
	batchID := strings.TrimSpace(query.BatchID)
	if batchID == "" && len(batches) > 0 {
		batchID = strings.TrimSpace(batches[0].BatchID)
	}
	rows := []inventoryapp.AllocationLogRow{}
	if batchID != "" {
		rows, err = r.fetchAllocationLogsByBatch(ctx, batchID)
		if err != nil {
			return inventoryapp.AllocationLogResult{}, err
		}
	}
	return inventoryapp.AllocationLogResult{
		BatchID: batchID,
		Batches: batches,
		Rows:    rows,
		HasNext: hasNext,
		Total:   total,
	}, nil
}

func (r Repository) listAllocationBatches(ctx context.Context, limit, offset int) ([]inventoryapp.AllocationBatchRow, int, bool, error) {
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(DISTINCT batch_id)::int
		FROM %s.finished_allocation_logs
	`, r.schema)).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	q := fmt.Sprintf(`
		SELECT batch_id,
		       count(*)::bigint as items,
		       COALESCE(max(operator), '') as operator,
		       to_char(max(created_at),'YYYY-MM-DD HH24:MI') as created_at
		FROM %s.finished_allocation_logs
		GROUP BY batch_id
		ORDER BY max(created_at) DESC
		LIMIT $1 OFFSET $2
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, limit+1, offset)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	out := make([]inventoryapp.AllocationBatchRow, 0)
	for rows.Next() {
		var row inventoryapp.AllocationBatchRow
		if err := rows.Scan(&row.BatchID, &row.Items, &row.Operator, &row.CreatedAt); err != nil {
			return nil, 0, false, err
		}
		row.OperatorName = strings.TrimSpace(row.Operator)
		if row.OperatorName == "" {
			row.OperatorName = "未知"
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	if len(out) > limit {
		return out[:limit], total, true, nil
	}
	return out, total, false, nil
}

func (r Repository) fetchAllocationLogsByBatch(ctx context.Context, batchID string) ([]inventoryapp.AllocationLogRow, error) {
	q := fmt.Sprintf(`
		SELECT l.batch_id, COALESCE(p.name,''), l.spec_g, l.need_g, l.deducted_g, l.gap_g,
		       COALESCE(l.operator,''), to_char(l.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finished_allocation_logs l
		LEFT JOIN %s.products p ON p.id = l.product_id
		WHERE l.batch_id = $1
		ORDER BY l.gap_g DESC, COALESCE(p.name,''), l.spec_g
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]inventoryapp.AllocationLogRow, 0)
	for rows.Next() {
		var row inventoryapp.AllocationLogRow
		if err := rows.Scan(&row.BatchID, &row.Product, &row.SpecG, &row.NeedG, &row.DeductedG, &row.GapG, &row.Operator, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.OperatorName = strings.TrimSpace(row.Operator)
		if row.OperatorName == "" {
			row.OperatorName = "未知"
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProducts(ctx context.Context) ([]inventoryapp.ProductOption, error) {
	hasCatalog, err := r.hasFinishedSpecCatalog(ctx)
	if err != nil {
		return nil, err
	}
	for _, table := range []string{"production_bom_output_bindings", "production_bom_versions"} {
		exists, existsErr := r.relationExists(ctx, table)
		if existsErr != nil {
			return nil, existsErr
		}
		hasCatalog = hasCatalog && exists
	}
	if hasCatalog {
		return r.listProductsWithBOMSpecs(ctx)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name
		FROM %s.products
		WHERE active=true
		  AND (NOT COALESCE(auto_derived_sku,false) OR COALESCE(NULLIF(derived_spec_status,''),'active')<>'template_removed')
		ORDER BY name`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]inventoryapp.ProductOption, 0)
	for rows.Next() {
		var p inventoryapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r Repository) listProductsWithBOMSpecs(ctx context.Context) ([]inventoryapp.ProductOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT p.id,p.name,COALESCE(NULLIF(migration.state,''),'legacy'),
		       COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END),
		       COALESCE(current_spec.bom_spec_id,0),COALESCE(current_spec.bom_variant_id,0),
		       COALESCE(current_spec.spec_key,''),COALESCE(current_spec.spec_name,''),
		       COALESCE(current_spec.inventory_unit,''),COALESCE(current_spec.is_default,false),
		       COALESCE(current_spec.sort_order,0)
		FROM %s.products p
		LEFT JOIN %s.product_bom_spec_migrations migration ON migration.product_id=p.id
		LEFT JOIN LATERAL (
			SELECT spec.id AS bom_spec_id,variant.id AS bom_variant_id,spec.spec_key,
			       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name) AS spec_name,
			       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit) AS inventory_unit,
			       variant.is_default,variant.sort_order
			FROM %s.production_bom_output_bindings binding
			JOIN %s.production_bom_versions version
			  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
			JOIN %s.production_bom_version_variants variant ON variant.version_id=version.id
			JOIN %s.production_bom_specs spec
			  ON spec.id=variant.bom_spec_id AND spec.bom_id=binding.bom_id
			WHERE binding.output_type='product' AND binding.output_id=p.id AND binding.is_default=true
			ORDER BY variant.sort_order,variant.id
		) current_spec ON true
		WHERE p.active=true
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
		ORDER BY p.name,current_spec.sort_order,current_spec.bom_spec_id
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]*inventoryapp.ProductOption{}
	order := make([]int64, 0)
	for rows.Next() {
		var productID, bomSpecID, bomVariantID int64
		var productName, migrationState, identityMode, specKey, specName, unit string
		var isDefault bool
		var sortOrder int
		if err := rows.Scan(
			&productID, &productName, &migrationState, &identityMode,
			&bomSpecID, &bomVariantID, &specKey, &specName, &unit, &isDefault, &sortOrder,
		); err != nil {
			return nil, err
		}
		product := byID[productID]
		if product == nil {
			product = &inventoryapp.ProductOption{ID: productID, Name: productName, MigrationState: migrationState, SpecIdentityMode: identityMode, BomSpecAuthoritative: identityMode == productspecmigrationapp.SpecIdentityModeBOMSpec}
			byID[productID] = product
			order = append(order, productID)
		}
		if bomSpecID > 0 {
			product.BOMSpecs = append(product.BOMSpecs, inventoryapp.ProductSpecOption{
				BomSpecID: bomSpecID, BomVariantID: bomVariantID, SpecKey: specKey,
				Name: specName, Unit: unit, IsDefault: isDefault, SortOrder: sortOrder,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]inventoryapp.ProductOption, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func (r Repository) relationExists(ctx context.Context, table string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", r.schema, table)).Scan(&exists)
	return exists, err
}

func (r Repository) hasFinishedSpecCatalog(ctx context.Context) (bool, error) {
	for _, table := range []string{"production_bom_specs", "production_bom_version_variants", "product_bom_spec_migrations"} {
		exists, err := r.relationExists(ctx, table)
		if err != nil || !exists {
			return false, err
		}
	}
	return true, nil
}

func inventoryColumnExistsTx(ctx context.Context, tx pgx.Tx, schema, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

type finishedBomSpecIdentity struct {
	BomVariantID  int64
	InventoryUnit string
}

func (r Repository) resolveFinishedBomSpecIdentityTx(ctx context.Context, tx pgx.Tx, productID, bomSpecID, explicitVariantID int64, explicitUnit string) (finishedBomSpecIdentity, error) {
	hasMigrations, err := inventoryRelationExistsTx(ctx, tx, r.schema, "product_bom_spec_migrations")
	if err != nil {
		return finishedBomSpecIdentity{}, err
	}
	if hasMigrations {
		var state string
		var legacyCatalogProduct bool
		var storedIdentityMode string
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT state,COALESCE((to_jsonb(product_bom_spec_migrations)->>'legacy_catalog_product')::boolean,true),COALESCE(to_jsonb(product_bom_spec_migrations)->>'spec_identity_mode','') FROM %s.product_bom_spec_migrations WHERE product_id=$1 FOR SHARE`, r.schema), productID).Scan(&state, &legacyCatalogProduct, &storedIdentityMode)
		if err == pgx.ErrNoRows {
			if bomSpecID > 0 || explicitVariantID > 0 {
				return finishedBomSpecIdentity{}, fmt.Errorf("BOM spec identity is not enabled for product %d", productID)
			}
			return finishedBomSpecIdentity{}, nil
		}
		if err != nil {
			return finishedBomSpecIdentity{}, err
		}
		identityMode := productspecmigrationapp.ResolveSpecIdentityMode(storedIdentityMode, productspecmigrationapp.MigrationState(state), legacyCatalogProduct)
		if identityMode != productspecmigrationapp.SpecIdentityModeBOMSpec {
			if bomSpecID > 0 || explicitVariantID > 0 {
				return finishedBomSpecIdentity{}, fmt.Errorf("BOM spec identity is not ready for product %d", productID)
			}
			if identityMode == productspecmigrationapp.SpecIdentityModeProduct {
				var inventoryUnit string
				if err := tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COALESCE(NULLIF(to_jsonb(products)->'unit_rule_override_json'->>'inventory_unit',''),'')
					FROM %s.products WHERE id=$1
				`, r.schema), productID).Scan(&inventoryUnit); err != nil {
					return finishedBomSpecIdentity{}, err
				}
				inventoryUnit = strings.TrimSpace(inventoryUnit)
				if inventoryUnit == "" {
					return finishedBomSpecIdentity{}, fmt.Errorf("direct product inventory unit is empty for product %d", productID)
				}
				if strings.TrimSpace(explicitUnit) != "" && !strings.EqualFold(strings.TrimSpace(explicitUnit), inventoryUnit) {
					return finishedBomSpecIdentity{}, fmt.Errorf("direct product inventory unit mismatch for product %d", productID)
				}
				return finishedBomSpecIdentity{InventoryUnit: inventoryUnit}, nil
			}
			return finishedBomSpecIdentity{}, nil
		}
		if bomSpecID <= 0 {
			return finishedBomSpecIdentity{}, fmt.Errorf("bom_spec_id required after BOM spec cutover")
		}
	}
	if bomSpecID <= 0 {
		return finishedBomSpecIdentity{}, nil
	}
	for _, table := range []string{"production_bom_specs", "production_bom_version_variants", "production_bom_versions", "production_bom_output_bindings"} {
		exists, existsErr := inventoryRelationExistsTx(ctx, tx, r.schema, table)
		if existsErr != nil {
			return finishedBomSpecIdentity{}, existsErr
		}
		if !exists {
			return finishedBomSpecIdentity{}, fmt.Errorf("BOM specification catalog is not available")
		}
	}
	var bomID, versionID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT binding.bom_id,binding.bom_version_id
		FROM %s.production_bom_output_bindings binding
		WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
		FOR SHARE OF binding
	`, r.schema), productID).Scan(&bomID, &versionID)
	if err == pgx.ErrNoRows {
		return finishedBomSpecIdentity{}, fmt.Errorf("BOM specification does not belong to the current default published BOM")
	}
	if err != nil {
		return finishedBomSpecIdentity{}, err
	}
	var identity finishedBomSpecIdentity
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT variant.id,COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),'')
		FROM %s.production_bom_versions version
		JOIN %s.production_bom_specs spec ON spec.id=$3 AND spec.bom_id=version.bom_id
		JOIN %s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE version.id=$2 AND version.bom_id=$1 AND version.status='published'
		ORDER BY variant.id
		LIMIT 1
	`, r.schema, r.schema, r.schema), bomID, versionID, bomSpecID).Scan(&identity.BomVariantID, &identity.InventoryUnit)
	if err == pgx.ErrNoRows {
		return finishedBomSpecIdentity{}, fmt.Errorf("BOM specification does not belong to the current default published BOM")
	}
	if err != nil {
		return finishedBomSpecIdentity{}, err
	}
	identity.InventoryUnit = strings.TrimSpace(identity.InventoryUnit)
	if explicitVariantID > 0 && explicitVariantID != identity.BomVariantID {
		return finishedBomSpecIdentity{}, fmt.Errorf("BOM variant is stale; use current default published BOM specification")
	}
	if unit := strings.TrimSpace(explicitUnit); unit != "" && !strings.EqualFold(unit, identity.InventoryUnit) {
		return finishedBomSpecIdentity{}, fmt.Errorf("inventory unit does not match current BOM specification: %s", identity.InventoryUnit)
	}
	if identity.InventoryUnit == "" {
		return finishedBomSpecIdentity{}, fmt.Errorf("current default published BOM specification inventory unit is empty")
	}
	return identity, nil
}

func inventoryRelationExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", schema, table)).Scan(&exists)
	return exists, err
}

func (r Repository) insertFinishedAdjustmentBatchTx(ctx context.Context, tx pgx.Tx, batchCode, productName string, cmd inventoryapp.AdjustFinishedInventoryCommand, changeG, changeUnits int64) error {
	hasBomSpec, err := inventoryColumnExistsTx(ctx, tx, r.schema, "stock_batches", "bom_spec_id")
	if err != nil {
		return err
	}
	if hasBomSpec {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_batches(
				batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
				source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,'',0,'',$8,$9,$10,now())
		`, r.schema), batchCode, stockItemTypeFinishedProduct, cmd.ProductID, productName,
			cmd.BomSpecID, cmd.BomVariantID, cmd.SpecG, changeG, changeUnits, cmd.Operator)
		return err
	}
	if cmd.BomSpecID > 0 {
		return fmt.Errorf("stock batch BOM specification columns are not available")
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,
			source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,operator,created_at
		) VALUES($1,$2,$3,$4,$5,'',0,'',$6,$7,$8,now())
	`, r.schema), batchCode, stockItemTypeFinishedProduct, cmd.ProductID, productName, cmd.SpecG, changeG, changeUnits, cmd.Operator)
	return err
}

func (r Repository) insertFinishedAdjustmentLedgerTx(ctx context.Context, tx pgx.Tx, batchCode, productName string, cmd inventoryapp.AdjustFinishedInventoryCommand, before, after inventorydomain.Quantity, beforeG, changeG, afterG int64) error {
	hasBomSpec, err := inventoryColumnExistsTx(ctx, tx, r.schema, "stock_ledger_entries", "bom_spec_id")
	if err != nil {
		return err
	}
	if hasBomSpec {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_ledger_entries(
				item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,
				source_doc_type,source_doc_id,source_batch_code,source_batch_id,
				qty_before_g,qty_change_g,qty_after_g,
				qty_before_units,qty_change_units,qty_after_units,operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0,$9,'',$10,$11,$12,$13,$14,$15,$16,now())
		`, r.schema), stockItemTypeFinishedProduct, cmd.ProductID, productName, cmd.BomSpecID, cmd.BomVariantID,
			cmd.SpecG, cmd.Warehouse, stockSourceManualAdjustment, batchCode,
			beforeG, changeG, afterG, before.Units, after.Units-before.Units, after.Units, cmd.Operator)
		return err
	}
	if cmd.BomSpecID > 0 {
		return fmt.Errorf("stock ledger BOM specification columns are not available")
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,0,$7,'',$8,$9,$10,$11,$12,$13,$14,now())
	`, r.schema), stockItemTypeFinishedProduct, cmd.ProductID, productName, cmd.SpecG, cmd.Warehouse,
		stockSourceManualAdjustment, batchCode, beforeG, changeG, afterG,
		before.Units, after.Units-before.Units, after.Units, cmd.Operator)
	return err
}

func manualAdjustmentBatchCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("ADJ-%s-%s", time.Now().Format("20060102150405"), hex.EncodeToString(b[:]))
}
