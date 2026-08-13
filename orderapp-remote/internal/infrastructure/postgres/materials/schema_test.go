package materials

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialsSchemaSeparatesBeanProfileTable(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "material_bean_profiles") {
		t.Fatalf("schema must create material_bean_profiles child table")
	}
	if !strings.Contains(src, "material_pack_profiles") {
		t.Fatalf("schema must create material_pack_profiles child table")
	}
	if !strings.Contains(src, "deprecated_at TIMESTAMPTZ") {
		t.Fatalf("materials schema must support deprecating old materials")
	}
	for _, want := range []string{
		"unit TEXT NOT NULL DEFAULT 'kg'",
		"cost_unit TEXT NOT NULL DEFAULT 'kg'",
		"ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS cost_unit",
		"ALTER TABLE %[1]s.materials ALTER COLUMN cost_unit SET DEFAULT 'kg'",
		"industry_field_template_id BIGINT NOT NULL DEFAULT 0",
		"material_industry_field_values",
		"material_classification_groups",
		"material_classification_group_categories",
		"material_classification_assignments",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("materials schema missing %q", want)
		}
	}
	if !strings.Contains(src, "SET unit='kg', cost_unit='kg'") {
		t.Fatal("materials schema must migrate legacy weight masters to kg without reinterpreting their price")
	}
	for _, want := range []string{
		"materials_unit_cost_unit_match",
		"CHECK (cost_unit = unit)",
		"历史物料存在无法安全自动合并的库存/成本单位",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("materials schema missing fail-closed invariant %q", want)
		}
	}
	materialsDDL := between(t, src, "CREATE TABLE IF NOT EXISTS %s.materials", ")`, schema)")
	for _, forbidden := range []string{
		"origin TEXT",
		"processing_station TEXT",
		"variety TEXT",
		"process_method TEXT",
		"grade TEXT",
		"altitude TEXT",
		"flavor TEXT",
		"bean_list_note TEXT",
		"size_spec TEXT",
		"dimensions TEXT",
		"material_texture TEXT",
		"capacity TEXT",
		"color TEXT",
	} {
		if strings.Contains(materialsDDL, forbidden) {
			t.Fatalf("materials DDL contains type-specific profile column %q", forbidden)
		}
	}
}

func TestEnsureSchemaMigratesLegacyWeightMasterWithoutRewritingSnapshots(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_unit_migration_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials (
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', unit TEXT NOT NULL DEFAULT 'g', cost_unit TEXT,
			batch_no TEXT NOT NULL DEFAULT '', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
			min_level_units BIGINT NOT NULL DEFAULT 0, deprecated_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.material_batches (
			id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, remaining_g BIGINT NOT NULL,
			unit_cost NUMERIC(12,2) NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_version_items (
			id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, consume_unit TEXT NOT NULL
		);
		CREATE TABLE %[1]s.work_orders (
			id BIGSERIAL PRIMARY KEY, output_material_id BIGINT NOT NULL, output_unit TEXT NOT NULL
		);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,batch_no,purchase_price,onhand_g,min_level_g)
		VALUES(1,'LEGACY-G-KG','历史重量物料','bean','g','kg','20260814',288,22700,1000);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,batch_no,purchase_price)
		VALUES
			(2,'LEGACY-LB-KG','历史磅物料','bean','lb','kg','20260814',288),
			(3,'LEGACY-OUNCE-KILOGRAM','历史盎司别名物料','bean','ounce','kilograms','20260814',288),
			(4,'LEGACY-CN-KG','历史公斤别名物料','bean','公斤','千克','20260814',288),
			(5,'LEGACY-G-NULL','历史空成本单位重量物料','bean','g',NULL,'20260814',288),
			(6,'LEGACY-UNIT-BLANK','历史空成本单位件数物料','other','个','','20260814',12);
		INSERT INTO %[1]s.material_batches(material_id,remaining_g,unit_cost) VALUES(1,22700,288);
		INSERT INTO %[1]s.production_bom_version_items(material_id,consume_unit) VALUES(1,'g');
		INSERT INTO %[1]s.work_orders(output_material_id,output_unit) VALUES(1,'g');
	`, schema)); err != nil {
		t.Fatal(err)
	}

	var constraintOID uint32
	for pass := 1; pass <= 2; pass++ {
		if err := EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatalf("EnsureSchema pass %d: %v", pass, err)
		}
		var oid uint32
		var validated bool
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT c.oid, c.convalidated
			FROM pg_constraint c
			WHERE c.conrelid='%s.materials'::regclass
			  AND c.conname='materials_unit_cost_unit_match'
		`, schema)).Scan(&oid, &validated); err != nil {
			t.Fatalf("constraint after pass %d: %v", pass, err)
		}
		if !validated {
			t.Fatalf("constraint after pass %d is not validated", pass)
		}
		if constraintOID != 0 && constraintOID != oid {
			t.Fatalf("constraint OID changed across idempotent EnsureSchema runs: %d -> %d", constraintOID, oid)
		}
		constraintOID = oid
	}

	var unit, costUnit string
	var purchasePrice float64
	var onhandG, minLevelG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit,purchase_price,onhand_g,min_level_g FROM %s.materials WHERE id=1`, schema)).Scan(&unit, &costUnit, &purchasePrice, &onhandG, &minLevelG); err != nil {
		t.Fatal(err)
	}
	if unit != "kg" || costUnit != "kg" || purchasePrice != 288 || onhandG != 22700 || minLevelG != 1000 {
		t.Fatalf("material after migration = unit %s cost %s price %.2f onhand_g %d min_level_g %d", unit, costUnit, purchasePrice, onhandG, minLevelG)
	}
	var canonicalCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials WHERE id BETWEEN 1 AND 4 AND unit='kg' AND cost_unit='kg'`, schema)).Scan(&canonicalCount); err != nil {
		t.Fatal(err)
	}
	if canonicalCount != 4 {
		t.Fatalf("canonical migrated weight masters = %d, want 4", canonicalCount)
	}
	var nullUnit, nullCostUnit, blankUnit, blankCostUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT unit FROM %s.materials WHERE id=5),
			(SELECT cost_unit FROM %s.materials WHERE id=5),
			(SELECT unit FROM %s.materials WHERE id=6),
			(SELECT cost_unit FROM %s.materials WHERE id=6)
	`, schema, schema, schema, schema)).Scan(&nullUnit, &nullCostUnit, &blankUnit, &blankCostUnit); err != nil {
		t.Fatal(err)
	}
	if nullUnit != "kg" || nullCostUnit != "kg" {
		t.Fatalf("NULL legacy cost unit was not backfilled before validation: %s/%s", nullUnit, nullCostUnit)
	}
	if blankUnit != "个" || blankCostUnit != "个" {
		t.Fatalf("blank legacy cost unit was not backfilled from inventory unit: %s/%s", blankUnit, blankCostUnit)
	}
	var remainingG int64
	var batchCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,unit_cost FROM %s.material_batches WHERE material_id=1`, schema)).Scan(&remainingG, &batchCost); err != nil {
		t.Fatal(err)
	}
	if remainingG != 22700 || batchCost != 288 {
		t.Fatalf("batch rewritten: remaining_g=%d unit_cost=%.2f", remainingG, batchCost)
	}
	var consumeUnit, outputUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT consume_unit FROM %s.production_bom_version_items WHERE material_id=1`, schema)).Scan(&consumeUnit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_unit FROM %s.work_orders WHERE output_material_id=1`, schema)).Scan(&outputUnit); err != nil {
		t.Fatal(err)
	}
	if consumeUnit != "g" || outputUnit != "g" {
		t.Fatalf("historical snapshots rewritten: BOM=%s work_order=%s", consumeUnit, outputUnit)
	}
}

func TestEnsureSchemaFailsClosedForUnknownUnitMismatch(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_unit_mismatch_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials (
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', unit TEXT NOT NULL DEFAULT 'kg', cost_unit TEXT,
			batch_no TEXT NOT NULL DEFAULT '', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
			min_level_units BIGINT NOT NULL DEFAULT 0, deprecated_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO %[1]s.materials(code,name,unit,cost_unit) VALUES('UNKNOWN-MISMATCH','待人工核对','箱','个');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	err = EnsureSchema(ctx, pool, schema)
	if err == nil || !strings.Contains(err.Error(), "无法安全自动合并") {
		t.Fatalf("EnsureSchema mismatch error = %v", err)
	}
	var unit, costUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit FROM %s.materials WHERE code='UNKNOWN-MISMATCH'`, schema)).Scan(&unit, &costUnit); err != nil {
		t.Fatal(err)
	}
	if unit != "箱" || costUnit != "个" {
		t.Fatalf("unknown mismatch was silently rewritten to %s/%s", unit, costUnit)
	}
}

func TestEnsureSchemaRollsBackWholeUnitMigrationWhenKnownAndUnknownLegacyRowsAreMixed(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_unit_atomic_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials (
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', unit TEXT NOT NULL DEFAULT 'g', cost_unit TEXT,
			batch_no TEXT NOT NULL DEFAULT '', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
			min_level_units BIGINT NOT NULL DEFAULT 0, deprecated_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO %[1]s.materials(code,name,unit,cost_unit,purchase_price)
		VALUES
			('KNOWN-G-KG','可自动迁移的历史重量物料','g','kg',288),
			('UNKNOWN-BOX-UNIT','必须人工核对的历史件数物料','箱','个',12);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	err = EnsureSchema(ctx, pool, schema)
	if err == nil || !strings.Contains(err.Error(), "无法安全自动合并") {
		t.Fatalf("EnsureSchema mixed mismatch error = %v", err)
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT code,unit,cost_unit FROM %s.materials ORDER BY code`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make(map[string][2]string)
	for rows.Next() {
		var code, unit, costUnit string
		if err := rows.Scan(&code, &unit, &costUnit); err != nil {
			t.Fatal(err)
		}
		got[code] = [2]string{unit, costUnit}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got["KNOWN-G-KG"] != [2]string{"g", "kg"} {
		t.Fatalf("known row was partially migrated: %v", got["KNOWN-G-KG"])
	}
	if got["UNKNOWN-BOX-UNIT"] != [2]string{"箱", "个"} {
		t.Fatalf("unknown row was rewritten: %v", got["UNKNOWN-BOX-UNIT"])
	}

	var unitDefault, costUnitDefault *string
	var costUnitNotNull bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			pg_get_expr(unit_attr.adbin, unit_attr.adrelid),
			pg_get_expr(cost_attr.adbin, cost_attr.adrelid),
			cost_col.attnotnull
		FROM pg_class tbl
		JOIN pg_namespace ns ON ns.oid=tbl.relnamespace
		JOIN pg_attribute unit_col ON unit_col.attrelid=tbl.oid AND unit_col.attname='unit'
		LEFT JOIN pg_attrdef unit_attr ON unit_attr.adrelid=tbl.oid AND unit_attr.adnum=unit_col.attnum
		JOIN pg_attribute cost_col ON cost_col.attrelid=tbl.oid AND cost_col.attname='cost_unit'
		LEFT JOIN pg_attrdef cost_attr ON cost_attr.adrelid=tbl.oid AND cost_attr.adnum=cost_col.attnum
		WHERE ns.nspname='%s' AND tbl.relname='materials'
	`, schema)).Scan(&unitDefault, &costUnitDefault, &costUnitNotNull); err != nil {
		t.Fatal(err)
	}
	if unitDefault == nil || !strings.Contains(*unitDefault, "'g'") {
		t.Fatalf("unit default was partially migrated: %v", unitDefault)
	}
	if costUnitDefault != nil {
		t.Fatalf("cost_unit default was partially added: %q", *costUnitDefault)
	}
	if costUnitNotNull {
		t.Fatal("cost_unit NOT NULL was partially added")
	}

	var constraintCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM pg_constraint
		WHERE conrelid='%s.materials'::regclass
		  AND conname='materials_unit_cost_unit_match'
	`, schema)).Scan(&constraintCount); err != nil {
		t.Fatal(err)
	}
	if constraintCount != 0 {
		t.Fatalf("unit/cost_unit constraint was partially added: %d", constraintCount)
	}
}

func TestEnsureSchemaRejectsCustomWeightDefinitionAndRollsBackUnitMigration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_custom_weight_migration_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials (
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', unit TEXT NOT NULL DEFAULT 'g', cost_unit TEXT,
			batch_no TEXT NOT NULL DEFAULT '', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
			min_level_units BIGINT NOT NULL DEFAULT 0, deprecated_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.product_unit_definitions(
			code TEXT PRIMARY KEY, unit_type TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT true
		);
		INSERT INTO %[1]s.product_unit_definitions(code,unit_type,active) VALUES
			('kg','weight',true), ('t','weight',true), ('吨','重量',true), ('袋','package',true);
		INSERT INTO %[1]s.materials(code,name,unit,cost_unit,purchase_price) VALUES
			('KNOWN-G-KG','可自动迁移的克物料','g','kg',288),
			('CUSTOM-T-T','字典定义的吨物料','t','t',288),
			('CUSTOM-CHINESE-TON','字典定义的中文吨物料','吨','吨',288),
			('CUSTOM-BAG-BAG','字典定义的袋物料','袋','袋',2);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	err = EnsureSchema(ctx, pool, schema)
	if err == nil || !strings.Contains(err.Error(), "重量库存单位") {
		t.Fatalf("EnsureSchema custom weight error = %v", err)
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT code,unit,cost_unit FROM %s.materials ORDER BY code`, schema))
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string][2]string)
	for rows.Next() {
		var code, unit, costUnit string
		if err := rows.Scan(&code, &unit, &costUnit); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		got[code] = [2]string{unit, costUnit}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if got["KNOWN-G-KG"] != [2]string{"g", "kg"} {
		t.Fatalf("known row was partially migrated: %v", got["KNOWN-G-KG"])
	}
	if got["CUSTOM-T-T"] != [2]string{"t", "t"} || got["CUSTOM-CHINESE-TON"] != [2]string{"吨", "吨"} || got["CUSTOM-BAG-BAG"] != [2]string{"袋", "袋"} {
		t.Fatalf("custom rows changed after rejected migration: ton=%v Chinese-ton=%v bag=%v", got["CUSTOM-T-T"], got["CUSTOM-CHINESE-TON"], got["CUSTOM-BAG-BAG"])
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.materials WHERE code IN ('CUSTOM-T-T','CUSTOM-CHINESE-TON')`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema with only canonical weight and package units: %v", err)
	}
	var gotUnit, gotCostUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit FROM %s.materials WHERE code='KNOWN-G-KG'`, schema)).Scan(&gotUnit, &gotCostUnit); err != nil {
		t.Fatal(err)
	}
	if gotUnit != "kg" || gotCostUnit != "kg" {
		t.Fatalf("known weight row after clean retry = %s/%s", gotUnit, gotCostUnit)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit FROM %s.materials WHERE code='CUSTOM-BAG-BAG'`, schema)).Scan(&gotUnit, &gotCostUnit); err != nil {
		t.Fatal(err)
	}
	if gotUnit != "袋" || gotCostUnit != "袋" {
		t.Fatalf("package row after clean retry = %s/%s", gotUnit, gotCostUnit)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema idempotent retry: %v", err)
	}
}

func TestEnsureSchemaAddsAndBackfillsMissingLegacyCostUnitColumn(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_unit_add_column_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials (
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', unit TEXT NOT NULL DEFAULT 'g',
			batch_no TEXT NOT NULL DEFAULT '', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
			min_level_units BIGINT NOT NULL DEFAULT 0, deprecated_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO %[1]s.materials(code,name,unit,purchase_price)
		VALUES ('NO-COST-COLUMN-G','旧重量物料','g',288), ('NO-COST-COLUMN-EACH','旧件数物料','个',12);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema with newly added cost_unit: %v", err)
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT code,unit,cost_unit FROM %s.materials ORDER BY code`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make(map[string][2]string)
	for rows.Next() {
		var code, unit, costUnit string
		if err := rows.Scan(&code, &unit, &costUnit); err != nil {
			t.Fatal(err)
		}
		got[code] = [2]string{unit, costUnit}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got["NO-COST-COLUMN-G"] != [2]string{"kg", "kg"} {
		t.Fatalf("new weight cost_unit column was not backfilled: %v", got["NO-COST-COLUMN-G"])
	}
	if got["NO-COST-COLUMN-EACH"] != [2]string{"个", "个"} {
		t.Fatalf("new discrete cost_unit column was not backfilled: %v", got["NO-COST-COLUMN-EACH"])
	}
	var costUnitNotNull bool
	var costUnitDefault *string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT col.attnotnull, pg_get_expr(def.adbin, def.adrelid)
		FROM pg_class tbl
		JOIN pg_namespace ns ON ns.oid=tbl.relnamespace
		JOIN pg_attribute col ON col.attrelid=tbl.oid AND col.attname='cost_unit'
		LEFT JOIN pg_attrdef def ON def.adrelid=tbl.oid AND def.adnum=col.attnum
		WHERE ns.nspname='%s' AND tbl.relname='materials'
	`, schema)).Scan(&costUnitNotNull, &costUnitDefault); err != nil {
		t.Fatal(err)
	}
	if !costUnitNotNull || costUnitDefault == nil || !strings.Contains(*costUnitDefault, "'kg'") {
		t.Fatalf("new cost_unit invariant incomplete: not_null=%t default=%v", costUnitNotNull, costUnitDefault)
	}
}

func between(t *testing.T, src, start, end string) string {
	t.Helper()
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatalf("missing %q", start)
	}
	j := strings.Index(src[i:], end)
	if j < 0 {
		t.Fatalf("missing %q after %q", end, start)
	}
	return src[i : i+j]
}
