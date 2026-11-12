package customerportal

import (
	"context"
	"fmt"
	manufacturingpg "orderapp/internal/infrastructure/postgres/manufacturing"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestMallOperationOptionsRequirePublishedVisibleProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	if err := manufacturingpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := productionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.products(id,name,active,customer_id,visibility) VALUES(77001,'商城公开商品',true,0,'public');
 INSERT INTO %[1]s.materials(id,code,name,unit,cost_unit) VALUES(77002,'OPTION-MATERIAL','熟豆','kg','kg');
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(77003,'包装路线','active');
 INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(77004,'包装');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation,operation_id) VALUES(77003,1,'包装',77004);
 INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status) VALUES(77005,'OPTION-BOM','包装BOM','product',77001,'active');
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit,process_route_id) VALUES(77006,77005,'V1','published',1,'袋',77003);
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(77007,77005,'OPTION-SPEC','bag','227g袋','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,process_route_id) VALUES(77008,77006,77007,'227g袋','袋',77003);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,consume_unit,qty_per_unit) VALUES(77006,77008,77002,'fixed_qty',0.227);
 INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',77001,77005,77006,true);
 INSERT INTO %[1]s.mall_products(id,product_id,bom_spec_id,bom_variant_id,spec_name,inventory_unit,title,status) VALUES(77009,77001,77007,77008,'227g袋','袋','工序候选测试','published');
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	options, err := r.LoadMallOperationOptions(ctx, 1, 77009)
	if err != nil || len(options) != 1 || options[0].OperationName != "包装" {
		t.Fatalf("visible published: %+v %v", options, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.mall_products SET status='draft' WHERE id=77009`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.LoadMallOperationOptions(ctx, 1, 77009); err == nil {
		t.Fatal("draft product exposed")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.mall_products SET status='published' WHERE id=77009; UPDATE %[1]s.products SET customer_id=123456,visibility='customer_only' WHERE id=77001`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.LoadMallOperationOptions(ctx, 1, 77009); err == nil {
		t.Fatal("private product exposed")
	}
}
