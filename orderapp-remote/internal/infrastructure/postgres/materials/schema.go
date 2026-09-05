package materials

import (
	"context"
	"fmt"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.materials (
		id BIGSERIAL PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT 'other',
		is_semi_finished BOOLEAN NOT NULL DEFAULT false,
		unit TEXT NOT NULL DEFAULT 'kg',
		cost_unit TEXT NOT NULL DEFAULT 'kg',
		batch_no TEXT NOT NULL DEFAULT '',
		purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		onhand_g BIGINT NOT NULL DEFAULT 0,
		onhand_units BIGINT NOT NULL DEFAULT 0,
		min_level_g BIGINT NOT NULL DEFAULT 0,
		min_level_units BIGINT NOT NULL DEFAULT 0,
		deprecated_at TIMESTAMPTZ NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS sale_price NUMERIC(12,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS batch_no TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS deprecated_at TIMESTAMPTZ NULL`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS industry_field_template_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS is_semi_finished BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS cost_unit TEXT`,
		`UPDATE %[1]s.materials SET batch_no=to_char(now(),'YYYYMMDD') WHERE batch_no=''`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	if err := ensureMaterialUnitCostUnitConstraint(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureSemiFinishedPurchasePriceConstraint(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureBeanProfileSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensurePackProfileSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialClassificationSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialIndustryFieldSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialCustomerReferenceSchema(ctx, pool, schema); err != nil {
		return err
	}
	logQ := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_consumption_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL,
		batch_id TEXT NOT NULL DEFAULT '',
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		material_id BIGINT NOT NULL,
		material_name TEXT NOT NULL DEFAULT '',
		unit TEXT NOT NULL DEFAULT '',
		deduct_g BIGINT NOT NULL DEFAULT 0,
		deduct_units BIGINT NOT NULL DEFAULT 0,
		before_g BIGINT NOT NULL DEFAULT 0,
		after_g BIGINT NOT NULL DEFAULT 0,
		before_units BIGINT NOT NULL DEFAULT 0,
		after_units BIGINT NOT NULL DEFAULT 0,
		operator TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_running_idx ON %s.material_consumption_logs(running_item_id, id);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_material_idx ON %s.material_consumption_logs(material_id, created_at DESC);`, schema, schema, schema)
	if _, err := pool.Exec(ctx, logQ); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_consumption_logs ADD COLUMN IF NOT EXISTS material_batch_id BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_consumption_logs ADD COLUMN IF NOT EXISTS material_batch_code TEXT NOT NULL DEFAULT ''`, schema))
	return nil
}

func ensureMaterialCustomerReferenceSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.material_customer_references (
	id BIGSERIAL PRIMARY KEY,
	material_id BIGINT NOT NULL,
	customer_id BIGINT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(material_id, customer_id)
);
CREATE INDEX IF NOT EXISTS material_customer_references_customer_active_idx
	ON %[1]s.material_customer_references(customer_id, active, material_id);
CREATE INDEX IF NOT EXISTS material_customer_references_material_active_idx
	ON %[1]s.material_customer_references(material_id, active, customer_id);
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureSemiFinishedPurchasePriceConstraint(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s.materials IN SHARE ROW EXCLUSIVE MODE`, schema)); err != nil {
		return err
	}

	type legacyPrice struct {
		id    int64
		price float64
	}
	legacy := make([]legacyPrice, 0)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,purchase_price::float8
		FROM %s.materials
		WHERE COALESCE(is_semi_finished,false)=true AND purchase_price<>0
		ORDER BY id
	`, schema))
	if err != nil {
		return err
	}
	for rows.Next() {
		var row legacyPrice
		if err := rows.Scan(&row.id, &row.price); err != nil {
			rows.Close()
			return err
		}
		legacy = append(legacy, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(legacy) > 0 {
		materialIDs := make([]int64, 0, len(legacy))
		for _, row := range legacy {
			materialIDs = append(materialIDs, row.id)
		}
		if err := lockSemiFinishedInboundTablesTx(ctx, tx, schema); err != nil {
			return err
		}
		if err := assertNoUnfinishedSemiFinishedInboundTx(ctx, tx, schema, materialIDs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.materials
			SET purchase_price=0,updated_at=now()
			WHERE COALESCE(is_semi_finished,false)=true AND purchase_price<>0
		`, schema)); err != nil {
			return err
		}
	}

	var auditTableExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".audit_logs").Scan(&auditTableExists); err != nil {
		return err
	}
	if auditTableExists {
		for _, row := range legacy {
			if err := postgresinfra.AuditInsertTx(
				ctx, tx, schema, "system:migration", "material", &row.id, "normalize",
				postgresinfra.StrPtr("purchase_price"), postgresinfra.StrPtr(fmt.Sprintf("%.2f", row.price)), postgresinfra.StrPtr("0.00"),
				postgresinfra.AuditMeta{"reason": "semi_finished_manufacture_only"},
			); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
DO $materials_semi_finished_purchase_price_constraint$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname='%[1]s'
		  AND t.relname='materials'
		  AND c.conname='materials_semi_finished_purchase_price_zero'
	) THEN
		ALTER TABLE %[1]s.materials
			ADD CONSTRAINT materials_semi_finished_purchase_price_zero
			CHECK (NOT COALESCE(is_semi_finished,false) OR purchase_price=0) NOT VALID;
	END IF;
END
$materials_semi_finished_purchase_price_constraint$;
ALTER TABLE %[1]s.materials VALIDATE CONSTRAINT materials_semi_finished_purchase_price_zero;
`, schema)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func materialManufactureOnlyLockKey(schema string, materialID int64) string {
	return fmt.Sprintf("%s:material-manufacture-only:%d", strings.TrimSpace(schema), materialID)
}

func lockSemiFinishedInboundTablesTx(ctx context.Context, tx pgx.Tx, schema string) error {
	for _, table := range []string{"purchase_orders", "purchase_receipts", "stock_entries", "stock_entry_items"} {
		exists, err := materialTableExistsTx(ctx, tx, schema, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s.%s IN SHARE ROW EXCLUSIVE MODE`, schema, table)); err != nil {
			return err
		}
	}
	return nil
}

func assertNoUnfinishedSemiFinishedInboundTx(ctx context.Context, tx pgx.Tx, schema string, materialIDs []int64) error {
	if len(materialIDs) == 0 {
		return nil
	}

	if exists, err := materialTableExistsTx(ctx, tx, schema, "purchase_orders"); err != nil {
		return err
	} else if exists {
		if err := requireMaterialColumnsTx(ctx, tx, schema, "purchase_orders", "material_id", "status"); err != nil {
			return err
		}
		var blocked bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1 FROM %s.purchase_orders
				WHERE material_id=ANY($1::bigint[])
				  AND lower(btrim(COALESCE(status,''))) NOT IN ('received','completed','cancelled','canceled','closed')
			)
		`, schema), materialIDs).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("存在未完成采购单，拒绝将物料切换为半成品或清零历史采购价")
		}
	}

	if exists, err := materialTableExistsTx(ctx, tx, schema, "purchase_receipts"); err != nil {
		return err
	} else if exists {
		if err := requireMaterialColumnsTx(ctx, tx, schema, "purchase_receipts", "material_id"); err != nil {
			return err
		}
		hasStatus, err := materialColumnExistsTx(ctx, tx, schema, "purchase_receipts", "status")
		if err != nil {
			return err
		}
		hasStockReceiptID, err := materialColumnExistsTx(ctx, tx, schema, "purchase_receipts", "stock_receipt_id")
		if err != nil {
			return err
		}
		condition := "true"
		switch {
		case hasStatus:
			condition = "lower(btrim(COALESCE(status,''))) NOT IN ('received','submitted','completed','cancelled','canceled','closed')"
		case hasStockReceiptID:
			condition = "COALESCE(stock_receipt_id,0)<=0"
		}
		var blocked bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1 FROM %s.purchase_receipts
				WHERE material_id=ANY($1::bigint[]) AND (%s)
			)
		`, schema, condition), materialIDs).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("存在未完成采购收货，拒绝将物料切换为半成品或清零历史采购价")
		}
	}

	stockEntriesExist, err := materialTableExistsTx(ctx, tx, schema, "stock_entries")
	if err != nil {
		return err
	}
	stockItemsExist, err := materialTableExistsTx(ctx, tx, schema, "stock_entry_items")
	if err != nil {
		return err
	}
	if stockEntriesExist != stockItemsExist {
		return fmt.Errorf("普通物料入库表结构不完整，无法确认是否存在未完成入库，拒绝继续")
	}
	if stockEntriesExist {
		if err := requireMaterialColumnsTx(ctx, tx, schema, "stock_entries", "id", "purpose", "status"); err != nil {
			return err
		}
		if err := requireMaterialColumnsTx(ctx, tx, schema, "stock_entry_items", "stock_entry_id", "material_id", "item_type"); err != nil {
			return err
		}
		var blocked bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1
				FROM %[1]s.stock_entries se
				JOIN %[1]s.stock_entry_items si ON si.stock_entry_id=se.id
				WHERE si.material_id=ANY($1::bigint[])
				  AND lower(btrim(COALESCE(si.item_type,'')))='material'
				  AND lower(btrim(COALESCE(se.purpose,'')))='material_receipt'
				  AND lower(btrim(COALESCE(se.status,''))) NOT IN ('submitted','cancelled','canceled')
			)
		`, schema), materialIDs).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("存在未完成普通物料入库，拒绝将物料切换为半成品或清零历史采购价")
		}
	}
	return nil
}

func requireMaterialColumnsTx(ctx context.Context, tx pgx.Tx, schema, table string, columns ...string) error {
	for _, column := range columns {
		exists, err := materialColumnExistsTx(ctx, tx, schema, table, column)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%s.%s 缺少 %s 列，无法确认未完成业务数据，拒绝继续", schema, table, column)
		}
	}
	return nil
}

func materialTableExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists)
	return exists, err
}

func materialColumnExistsTx(ctx context.Context, tx pgx.Tx, schema, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

func ensureMaterialUnitCostUnitConstraint(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s.materials IN SHARE ROW EXCLUSIVE MODE`, schema)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.materials
		SET cost_unit=CASE
			WHEN lower(btrim(unit)) IN ('g','gram','grams','kg','kgs','kilogram','kilograms','lb','lbs','pound','pounds','oz','ounce','ounces','克','千克','公斤','磅','盎司') THEN 'kg'
			ELSE COALESCE(NULLIF(unit,''),'unit')
		END
		WHERE COALESCE(NULLIF(btrim(cost_unit),''),'')='';

		UPDATE %[1]s.materials
		SET unit='kg', cost_unit='kg'
		WHERE lower(btrim(unit)) IN ('g','gram','grams','kg','kgs','kilogram','kilograms','lb','lbs','pound','pounds','oz','ounce','ounces','克','千克','公斤','磅','盎司')
		  AND lower(btrim(cost_unit)) IN ('kg','kgs','kilogram','kilograms','千克','公斤');
	`, schema)); err != nil {
		return err
	}
	var mismatched int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials WHERE cost_unit IS DISTINCT FROM unit`, schema)).Scan(&mismatched); err != nil {
		return err
	}
	if mismatched > 0 {
		return fmt.Errorf("历史物料存在无法安全自动合并的库存/成本单位: %d 条；已停止迁移，请先核对单价口径", mismatched)
	}
	var nonCanonicalWeight int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.materials
		WHERE lower(btrim(unit)) IN ('g','gram','grams','lb','lbs','pound','pounds','oz','ounce','ounces','克','磅','盎司')
	`, schema)).Scan(&nonCanonicalWeight); err != nil {
		return err
	}
	if nonCanonicalWeight > 0 {
		return fmt.Errorf("历史物料存在无法安全自动归一的重量库存单位: %d 条；已停止迁移，请先核对单价口径", nonCanonicalWeight)
	}
	var unitDictionaryExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".product_unit_definitions").Scan(&unitDictionaryExists); err != nil {
		return err
	}
	if unitDictionaryExists {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`LOCK TABLE %s.product_unit_definitions IN SHARE MODE`, schema)); err != nil {
			return err
		}
		var customNonCanonicalWeight int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s.materials m
			JOIN %[1]s.product_unit_definitions u ON lower(btrim(u.code))=lower(btrim(m.unit))
			WHERE lower(btrim(u.unit_type)) IN ('weight','重量')
			  AND lower(btrim(m.unit)) <> 'kg'
		`, schema)).Scan(&customNonCanonicalWeight); err != nil {
			return err
		}
		if customNonCanonicalWeight > 0 {
			return fmt.Errorf("历史物料存在全局单位字典定义的非 kg 重量库存单位: %d 条；已停止迁移，请先核对单价口径", customNonCanonicalWeight)
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %[1]s.materials ALTER COLUMN unit SET DEFAULT 'kg';
		ALTER TABLE %[1]s.materials ALTER COLUMN cost_unit SET DEFAULT 'kg';
		ALTER TABLE %[1]s.materials ALTER COLUMN cost_unit SET NOT NULL;
	`, schema)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
DO $materials_unit_cost_unit_constraint$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname='%[1]s'
		  AND t.relname='materials'
		  AND c.conname='materials_unit_cost_unit_match'
	) THEN
		ALTER TABLE %[1]s.materials
			ADD CONSTRAINT materials_unit_cost_unit_match CHECK (cost_unit = unit) NOT VALID;
	END IF;
END
$materials_unit_cost_unit_constraint$;
ALTER TABLE %[1]s.materials VALIDATE CONSTRAINT materials_unit_cost_unit_match;
`, schema)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ensureMaterialClassificationSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_classification_groups (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 100,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_classification_groups_sort_idx ON %s.material_classification_groups(sort_order, id);

	CREATE TABLE IF NOT EXISTS %s.material_classification_group_categories (
		id BIGSERIAL PRIMARY KEY,
		group_id BIGINT NOT NULL REFERENCES %s.material_classification_groups(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 100,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_classification_group_categories_group_idx ON %s.material_classification_group_categories(group_id, sort_order, id);

	CREATE TABLE IF NOT EXISTS %s.material_classification_assignments (
		material_id BIGINT PRIMARY KEY REFERENCES %s.materials(id) ON DELETE CASCADE,
		group_id BIGINT NOT NULL DEFAULT 0,
		category_id BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_by TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS material_classification_assignments_group_idx ON %s.material_classification_assignments(group_id, category_id);`,
		schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureMaterialIndustryFieldSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_industry_field_values (
		material_id BIGINT NOT NULL REFERENCES %s.materials(id) ON DELETE CASCADE,
		field_key TEXT NOT NULL,
		value_text TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_by TEXT NOT NULL DEFAULT '',
		PRIMARY KEY(material_id, field_key)
	);
	CREATE INDEX IF NOT EXISTS material_industry_field_values_material_idx ON %s.material_industry_field_values(material_id);`, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	backfill := fmt.Sprintf(`
		INSERT INTO %s.material_industry_field_values(material_id, field_key, value_text, updated_at)
		SELECT material_id, field_key, value_text, now()
		FROM (
			SELECT material_id, '产地' AS field_key, origin AS value_text FROM %s.material_bean_profiles WHERE COALESCE(origin,'') <> ''
			UNION ALL SELECT material_id, '处理站', processing_station FROM %s.material_bean_profiles WHERE COALESCE(processing_station,'') <> ''
			UNION ALL SELECT material_id, '品种', variety FROM %s.material_bean_profiles WHERE COALESCE(variety,'') <> ''
			UNION ALL SELECT material_id, '处理法', process_method FROM %s.material_bean_profiles WHERE COALESCE(process_method,'') <> ''
			UNION ALL SELECT material_id, '等级', grade FROM %s.material_bean_profiles WHERE COALESCE(grade,'') <> ''
			UNION ALL SELECT material_id, '海拔', altitude FROM %s.material_bean_profiles WHERE COALESCE(altitude,'') <> ''
			UNION ALL SELECT material_id, '风味', flavor FROM %s.material_bean_profiles WHERE COALESCE(flavor,'') <> ''
			UNION ALL SELECT material_id, '豆单备注', bean_list_note FROM %s.material_bean_profiles WHERE COALESCE(bean_list_note,'') <> ''
		) legacy
		ON CONFLICT(material_id, field_key) DO NOTHING`, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, backfill)
	return err
}

func ensurePackProfileSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_pack_profiles (
		material_id BIGINT PRIMARY KEY REFERENCES %s.materials(id) ON DELETE CASCADE,
		size_spec TEXT NOT NULL DEFAULT '',
		dimensions TEXT NOT NULL DEFAULT '',
		material_texture TEXT NOT NULL DEFAULT '',
		capacity TEXT NOT NULL DEFAULT '',
		color TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_pack_profiles_size_spec_idx ON %s.material_pack_profiles(size_spec);`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureBeanProfileSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_bean_profiles (
		material_id BIGINT PRIMARY KEY REFERENCES %s.materials(id) ON DELETE CASCADE,
		origin TEXT NOT NULL DEFAULT '',
		processing_station TEXT NOT NULL DEFAULT '',
		variety TEXT NOT NULL DEFAULT '',
		process_method TEXT NOT NULL DEFAULT '',
		grade TEXT NOT NULL DEFAULT '',
		altitude TEXT NOT NULL DEFAULT '',
		flavor TEXT NOT NULL DEFAULT '',
		bean_list_note TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_bean_profiles_flavor_idx ON %s.material_bean_profiles(flavor);`, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	schemaLiteral := strings.ReplaceAll(schema, "'", "''")
	migrate := fmt.Sprintf(`
DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema='%[2]s' AND table_name='materials' AND column_name='origin'
	) THEN
		INSERT INTO %[1]s.material_bean_profiles(
			material_id, origin, processing_station, variety, process_method,
			grade, altitude, flavor, bean_list_note, updated_at
		)
		SELECT id,
		       COALESCE(origin, ''),
		       COALESCE(processing_station, ''),
		       COALESCE(variety, ''),
		       COALESCE(process_method, ''),
		       COALESCE(grade, ''),
		       COALESCE(altitude, ''),
		       COALESCE(flavor, ''),
		       COALESCE(bean_list_note, ''),
		       now()
		FROM %[1]s.materials
		WHERE kind='bean'
		  AND (
			COALESCE(origin, '') <> ''
			OR COALESCE(processing_station, '') <> ''
			OR COALESCE(variety, '') <> ''
			OR COALESCE(process_method, '') <> ''
			OR COALESCE(grade, '') <> ''
			OR COALESCE(altitude, '') <> ''
			OR COALESCE(flavor, '') <> ''
			OR COALESCE(bean_list_note, '') <> ''
		  )
		ON CONFLICT (material_id) DO UPDATE SET
			origin=excluded.origin,
			processing_station=excluded.processing_station,
			variety=excluded.variety,
			process_method=excluded.process_method,
			grade=excluded.grade,
			altitude=excluded.altitude,
			flavor=excluded.flavor,
			bean_list_note=excluded.bean_list_note,
			updated_at=now();
	END IF;
END $$;`, schema, schemaLiteral)
	if _, err := pool.Exec(ctx, migrate); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS origin`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS processing_station`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS variety`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS process_method`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS grade`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS altitude`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS flavor`,
		`ALTER TABLE %[1]s.materials DROP COLUMN IF EXISTS bean_list_note`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}
