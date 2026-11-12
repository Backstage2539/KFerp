package production

import (
	"context"
	"encoding/json"
	"fmt"
	pgproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestOrderOperationOptionsFollowOnlySelectedManufacturingGraph(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(901,'包装'),(902,'烘焙');
 UPDATE %[1]s.process_route_operations SET operation_id=CASE route_id WHEN 31 THEN 901 ELSE 902 END;
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(33,'无关路线','active');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation,operation_id) VALUES(33,1,'无关工序',901);
 INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_material_id,status) VALUES(201,'UNRELATED','无关物料','material',20,'active');
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit,process_route_id) VALUES(201,201,'V1','draft',1,'unit',33);
 INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('material',20,201,201,true);
 `, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	options, err := pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 {
		t.Fatalf("only root packaging and upstream roast expected: %+v", options)
	}
	seen := map[string]bool{}
	for _, o := range options {
		if o.StepID <= 0 || o.OperationID <= 0 || o.OutputName == "" {
			t.Fatalf("incomplete identity: %+v", o)
		}
		seen[fmt.Sprintf("%s:%d:%d", o.OutputType, o.OutputID, o.RouteID)] = true
	}
	if !seen["product:1:31"] || !seen["material:10:32"] {
		t.Fatalf("wrong graph: %+v", options)
	}
	if _, err = pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 999, 0, 0); err == nil {
		t.Fatal("unknown product accepted")
	}
}

func TestOrderOperationOptionsRejectManufacturingCycles(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_version_items(version_id,component_type,component_product_id,consume_unit,qty_per_unit) VALUES(200,'finished_product',1,'unit',1)`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 0, 0); err == nil {
		t.Fatal("cyclic graph accepted")
	}
}

func TestOrderOperationOptionsRespectSelectedVariantAndSharedDependencies(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(901,'包装'),(902,'烘焙'),(903,'另一个规格工序');
 UPDATE %[1]s.process_route_operations SET operation_id=CASE route_id WHEN 31 THEN 901 ELSE 902 END;
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(33,'另一规格路线','active');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation,operation_id) VALUES(33,1,'另一个规格工序',903);
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(1001,100,'OPTION-A','a','A袋','袋'),(1002,100,'OPTION-B','b','B袋','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,process_route_id) VALUES(1101,100,1001,'A袋','袋',31),(1102,100,1002,'B袋','袋',33);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,consume_unit,qty_per_unit) VALUES(100,1101,10,'fixed_qty',1),(100,1101,10,'fixed_qty',1),(100,1102,30,'fixed_qty',1);`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	a, err := pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 1001, 1101)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 {
		t.Fatalf("shared upstream dependency repeated or selected variant lost: %+v", a)
	}
	encoded, _ := json.Marshal(a)
	var identities []map[string]any
	if err := json.Unmarshal(encoded, &identities); err != nil {
		t.Fatal(err)
	}
	for _, o := range identities {
		if o["output_type"] == "product" && (o["bom_spec_id"] != float64(1001) || o["bom_variant_id"] != float64(1101)) {
			t.Fatalf("selected output spec identity missing: %s", encoded)
		}
	}
	for _, o := range a {
		if o.RouteID == 33 {
			t.Fatalf("unselected variant leaked: %+v", a)
		}
	}
	b, err := pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 1002, 1102)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || b[0].RouteID != 33 {
		t.Fatalf("wrong selected variant graph: %+v", b)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_version_variants SET process_route_id=31 WHERE id=1102; INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,component_type,component_product_id,component_bom_spec_id,consume_unit,qty_per_unit) VALUES(100,1101,'finished_product',1,1002,'fixed_qty',1)`, schema)); err != nil {
		t.Fatal(err)
	}
	shared, err := pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 1001, 1101)
	if err != nil {
		t.Fatal(err)
	}
	if len(shared) != 3 {
		t.Fatalf("same product and route with distinct specs must stay separate: %+v", shared)
	}
	seenSpecs := map[int64]int64{}
	for _, o := range shared {
		if o.OutputType == "product" {
			seenSpecs[o.BomSpecID] = o.BomVariantID
		}
	}
	if seenSpecs[1001] != 1101 || seenSpecs[1002] != 1102 {
		t.Fatalf("upstream product spec identity missing: %+v", shared)
	}
	if _, err = pgproduction.OrderOperationOptionsTx(ctx, tx, schema, 1, 1001, 1102); err == nil {
		t.Fatal("mismatched spec and variant accepted")
	}
}
