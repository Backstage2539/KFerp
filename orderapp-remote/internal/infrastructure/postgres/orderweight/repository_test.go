package orderweight_test

import (
	"context"
	"fmt"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	weights "orderapp/internal/infrastructure/postgres/orderweight"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func weightDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set disposable local ORDERAPP_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	s := fmt.Sprintf("weight_%d", time.Now().UnixNano())
	_, e = p.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %[1]s;
	CREATE TABLE %[1]s.products(id bigint primary key,name text,active boolean default true,customer_id bigint default 0,unit_rule_override_json jsonb default '{}');
	CREATE TABLE %[1]s.materials(id bigint primary key,unit text);
	CREATE TABLE %[1]s.production_bom_specs(id bigint primary key,inventory_unit text);
	CREATE TABLE %[1]s.production_bom_versions(id bigint primary key,bom_id bigint,status text,output_qty numeric,output_unit text);
	CREATE TABLE %[1]s.production_bom_output_bindings(output_type text,output_id bigint,bom_id bigint,bom_version_id bigint,is_default boolean);
	CREATE TABLE %[1]s.production_bom_version_variants(id bigint primary key,version_id bigint,bom_spec_id bigint,inventory_unit text,is_default boolean,logistics_weight_g numeric);
	CREATE TABLE %[1]s.production_bom_version_items(id bigint primary key,version_id bigint,variant_id bigint,material_id bigint,component_type text default 'material',component_product_id bigint,component_bom_spec_id bigint,consume_unit text,qty_per_unit numeric,ratio_pct numeric default 0,material_loss_rate numeric default .2);
	CREATE TABLE %[1]s.order_items(id bigint primary key,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,qty numeric,unit text,spec text default '',material_unit_weight_g numeric,material_item_weight_g numeric,logistics_unit_weight_g numeric,logistics_item_weight_g numeric,weight_snapshot_json jsonb default '{}');
	INSERT INTO %[1]s.products(id,name)VALUES(1,'测试袋装'); INSERT INTO %[1]s.materials VALUES(1,'kg'),(2,'个');
	INSERT INTO %[1]s.production_bom_specs VALUES(10,'袋');
	INSERT INTO %[1]s.production_bom_versions VALUES(20,30,'published',100,'kg');
	INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',1,30,20,true);
	INSERT INTO %[1]s.production_bom_version_variants VALUES(40,20,10,'袋',true,250);
	INSERT INTO %[1]s.production_bom_version_items(id,version_id,variant_id,material_id,consume_unit,qty_per_unit)VALUES(1,20,40,1,'kg',.227),(2,20,40,2,'个',1);
	INSERT INTO %[1]s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id,qty,unit)VALUES(1,1,1,10,40,4,'袋');`, s))
	if e != nil {
		p.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() { p.Exec(ctx, "DROP SCHEMA "+s+" CASCADE"); p.Close() })
	return p, s
}

func TestResolveAndFreezeManualLogisticsIndependentOfNetMaterials(t *testing.T) {
	p, s := weightDB(t)
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = weights.FreezeOrder(ctx, tx, s, 1); e != nil {
		t.Fatal(e)
	}
	var m, l float64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g,logistics_item_weight_g FROM %s.order_items WHERE id=1`, s)).Scan(&m, &l); e != nil || m != 908 || l != 1000 {
		t.Fatalf("material=%v logistics=%v err=%v", m, l, e)
	}
	// Changing recipe/configuration cannot rewrite a saved line on quantity edits.
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=999; UPDATE %[1]s.production_bom_version_items SET qty_per_unit=99; UPDATE %[1]s.order_items SET qty=2`, s)); e != nil {
		t.Fatal(e)
	}
	if e = weights.FreezeOrder(ctx, tx, s, 1); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g,logistics_item_weight_g FROM %s.order_items WHERE id=1`, s)).Scan(&m, &l); e != nil || m != 454 || l != 500 {
		t.Fatalf("frozen material=%v logistics=%v err=%v", m, l, e)
	}
}

func TestMissingManualWeightAndHistoricalRowsRemainUnknown(t *testing.T) {
	p, s := weightDB(t)
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = weights.MarkHistoricalUnknown(ctx, tx, s, 1); e != nil {
		t.Fatal(e)
	}
	if e = weights.FreezeOrder(ctx, tx, s, 1); e != nil {
		t.Fatal(e)
	}
	var m, l *float64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g,logistics_item_weight_g FROM %s.order_items`, s)).Scan(&m, &l); e != nil || m != nil || l != nil {
		t.Fatalf("historical weights changed %v %v %v", m, l, e)
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL`, s)); e != nil {
		t.Fatal(e)
	}
	v, e := weights.Resolve(ctx, tx, s, weights.Item{ProductID: 1, BomSpecID: 10, BomVariantID: 40, Qty: 4})
	if e != nil || v.LogisticsUnitWeightG != nil || v.MaterialUnitWeightG == nil || *v.MaterialUnitWeightG != 227 {
		t.Fatalf("missing manual must not use recipe weight: %+v %v", v, e)
	}
}

func TestFreightUsesManualWeightsAndRoundsOnlyWholeOrder(t *testing.T) {
	p, s := weightDB(t)
	ctx := context.Background()
	r := freightpg.NewRepository(p, s)
	g, basis, e := r.WeightTotals(ctx, p, 1, []freightpg.WeightItem{{ProductID: 1, BomSpecID: 10, BomVariantID: 40, Qty: 4}}, false)
	if e != nil || g != 1000 || len(basis) != 1 || basis[0].LogisticsUnitWeightG == nil || *basis[0].LogisticsUnitWeightG != 250 {
		t.Fatalf("logistics total=%v basis=%+v e=%v", g, basis, e)
	}
	if _, e = p.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=.333333`, s)); e != nil {
		t.Fatal(e)
	}
	g, _, e = r.WeightTotals(ctx, p, 1, []freightpg.WeightItem{{ProductID: 1, BomSpecID: 10, BomVariantID: 40, Qty: 3}}, false)
	if e != nil || g != 1 {
		t.Fatalf("fractional whole order=%d err=%v", g, e)
	}
	if _, e = p.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL`, s)); e != nil {
		t.Fatal(e)
	}
	if _, _, e = r.WeightTotals(ctx, p, 1, []freightpg.WeightItem{{ProductID: 1, BomSpecID: 10, BomVariantID: 40, Qty: 1}}, false); e == nil {
		t.Fatal("material reference used as freight fallback")
	}
}
