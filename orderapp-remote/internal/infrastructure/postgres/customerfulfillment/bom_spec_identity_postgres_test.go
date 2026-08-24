package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
)

func TestCustomerDirectShipCutoverBOMSpecIdentityPersistsAndRejectsLegacyWrites(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("productspecmigration.EnsureSchema: %v", err)
	}

	var customerID, parentProductID, childProductID, bomID, versionID, bomSpecID, bomVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,customer_type) VALUES('PR600代发客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,product_kind,active,customer_id,visibility,custom_type,unit_rule_override_json)
		VALUES('PR600父商品','PR600父商品','PARENT-PR600','roasted_bean',true,$1,'customer_only','customer_product','{"inventory_unit":"g"}'::jsonb) RETURNING id
	`, schema), customerID).Scan(&parentProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,parent_product_id,base_product_id,product_kind,active,customer_id,visibility,custom_type,spec_label,net_content_qty,net_content_unit,auto_derived_sku,derived_spec_key,derived_spec_name,derived_sales_unit)
		VALUES('PR600旧227g子SKU','PR600旧227g子SKU','LEGACY-227-BARCODE',$1,$1,'roasted_bean',true,$2,'customer_only','derived_sku','227g',227,'g',true,'227g','227g','bag') RETURNING id
	`, schema), parentProductID, customerID).Scan(&childProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status)
		VALUES('BOM-PR600-DS','PR600代发商品BOM','product',$1,'active') RETURNING id
	`, schema), parentProductID).Scan(&bomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v1','published',1,'袋') RETURNING id
	`, schema), bomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit)
		VALUES($1,'BSP-PR600-227','NEW-PR600-227','bag-227','227g袋','袋') RETURNING id
	`, schema), bomID).Scan(&bomSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'227g袋','袋',true,1) RETURNING id
	`, schema), versionID, bomSpecID).Scan(&bomVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES('product',$1,$2,$3,true,'test')
	`, schema), parentProductID, bomID, versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,cutover_by,cutover_at)
		VALUES($1,'cutover','test',now())
	`, schema), parentProductID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.legacy_child_sku_bom_spec_mappings(
			parent_product_id,legacy_child_product_id,bom_id,bom_spec_id,bom_variant_id,
			legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,created_by
		) VALUES($1,$2,$3,$4,$5,'227g','227g','bag',227,'test')
	`, schema), parentProductID, childProductID, bomID, bomSpecID, bomVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	priceContent := fmt.Sprintf(`{
		"price_rows":[{
			"product_id":%d,"parent_product_id":%d,"bom_spec_id":%d,"bom_variant_id":%d,
			"source_price_record_id":600227,"min_qty":1,"final_unit_price":42,
			"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"
		}]
	}`, parentProductID, parentProductID, bomSpecID, bomVariantID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,config_json,content_json,changelog,actor)
		VALUES('commercial','PR600-DS-PRICE','published','customer',$1,'factory_supply','{}'::jsonb,$2::jsonb,'','test')
	`, schema), fmt.Sprint(customerID), priceContent); err != nil {
		t.Fatal(err)
	}
	archivedVariantID := bomVariantID
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{fmt.Sprintf(`DELETE FROM %s.legacy_child_sku_bom_spec_mappings WHERE parent_product_id=$1`, schema), []any{parentProductID}},
		{fmt.Sprintf(`UPDATE %s.products SET active=false WHERE id=$1`, schema), []any{childProductID}},
		{fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='archived' WHERE id=$1`, schema), []any{versionID}},
	} {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	var currentVersionID, currentVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v2','published',1,'袋') RETURNING id
	`, schema), bomID).Scan(&currentVersionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'227g袋','袋',true,1) RETURNING id
	`, schema), currentVersionID, bomSpecID).Scan(&currentVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_output_bindings SET bom_version_id=$1
		WHERE output_type='product' AND output_id=$2 AND is_default=true
	`, schema), currentVersionID, parentProductID); err != nil {
		t.Fatal(err)
	}
	bomVariantID = currentVariantID

	repo := NewRepository(pool, schema)
	options, err := repo.CustomerFulfillmentOptions(ctx, customerID)
	if err != nil {
		t.Fatalf("CustomerFulfillmentOptions: %v", err)
	}
	var canonical *app.CustomerSKUOption
	for idx := range options.CustomerSKUs {
		option := &options.CustomerSKUs[idx]
		if option.ProductID == childProductID || option.SKUCode == "LEGACY-227-BARCODE" {
			t.Fatalf("cutover candidate exposed legacy child SKU: %#v", *option)
		}
		if option.ProductID == parentProductID && option.BomSpecID == bomSpecID {
			canonical = option
		}
	}
	if canonical == nil || canonical.BomVariantID != bomVariantID || canonical.BomSpecKey != "bag-227" || canonical.InventoryUnit != "袋" || canonical.SKUCode != "BSP-PR600-227" || len(canonical.Tiers) != 1 || canonical.Tiers[0].UnitPrice != 42 || canonical.Tiers[0].BomSpecID != bomSpecID || canonical.Tiers[0].BomVariantID != bomVariantID {
		t.Fatalf("canonical candidate=%#v options=%#v", canonical, options.CustomerSKUs)
	}

	created, err := repo.SubmitCustomerDirectShipOrder(ctx, app.SubmitCustomerDirectShipOrderCommand{
		CustomerID: customerID, ReceiverName: "张三", ReceiverPhone: "13800138000", ReceiverAddress: "咖啡路8号", Actor: "test:operator",
		Items: []app.SubmitCustomerDirectShipOrderItem{{
			ProductID: parentProductID, BomSpecID: bomSpecID,
			Spec: "客户端伪造名称", SalesUnit: "bag", QuantityUnits: 2,
		}},
	})
	if err != nil {
		t.Fatalf("SubmitCustomerDirectShipOrder canonical: %v", err)
	}
	var importProductID, importBomSpecID, importBomVariantID int64
	var orderProductID, orderBomSpecID, orderBomVariantID int64
	var productTitle, importSpec, orderSpec, orderUnit, priceSource string
	var qty, unitBeanG, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,product_title,spec
		FROM %s.customer_direct_ship_import_order_items
		WHERE import_order_id=(SELECT id FROM %s.customer_direct_ship_import_orders WHERE order_id=$1)
	`, schema, schema), created.OrderID).Scan(&importProductID, &importBomSpecID, &importBomVariantID, &productTitle, &importSpec); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,spec,unit,qty::float8,unit_bean_g::float8,line_total::float8,price_source_json::text
		FROM %s.order_items WHERE order_id=$1
	`, schema), created.OrderID).Scan(&orderProductID, &orderBomSpecID, &orderBomVariantID, &orderSpec, &orderUnit, &qty, &unitBeanG, &lineTotal, &priceSource); err != nil {
		t.Fatal(err)
	}
	if importProductID != parentProductID || importBomSpecID != bomSpecID || importBomVariantID != bomVariantID || productTitle != "PR600父商品 227g袋" || importSpec != "227g袋" {
		t.Fatalf("import identity=%d/%d/%d %q %q", importProductID, importBomSpecID, importBomVariantID, productTitle, importSpec)
	}
	if orderProductID != parentProductID || orderBomSpecID != bomSpecID || orderBomVariantID != bomVariantID || orderSpec != "227g袋" || orderUnit != "袋" || qty != 2 || unitBeanG != 0 || lineTotal != 84 {
		t.Fatalf("order identity=%d/%d/%d spec=%q unit=%q qty=%.2f bean=%.2f total=%.2f source=%s", orderProductID, orderBomSpecID, orderBomVariantID, orderSpec, orderUnit, qty, unitBeanG, lineTotal, priceSource)
	}
	for _, want := range []string{fmt.Sprintf(`"bom_spec_id": %d`, bomSpecID), fmt.Sprintf(`"bom_variant_id": %d`, bomVariantID), `"quantity_basis": "sales_spec_count"`} {
		if !strings.Contains(priceSource, want) {
			t.Fatalf("price source missing %s: %s", want, priceSource)
		}
	}
	assertCustomerFulfillmentCount(t, pool, schema, "audit_logs", "entity_type='customer_fulfillment_order' AND entity_id=$1 AND action='submit'", created.OrderID, 1)

	for _, item := range []app.SubmitCustomerDirectShipOrderItem{
		{ProductID: parentProductID, QuantityUnits: 1},
		{ProductID: childProductID, SpecG: 227, QuantityUnits: 1},
		{ProductID: parentProductID, BomSpecID: bomSpecID, BomVariantID: archivedVariantID, QuantityUnits: 1},
	} {
		_, err := repo.SubmitCustomerDirectShipOrder(ctx, app.SubmitCustomerDirectShipOrderCommand{
			CustomerID: customerID, ReceiverName: "拒绝", ReceiverPhone: "13800138000", ReceiverAddress: "咖啡路8号", Actor: "test:operator", Items: []app.SubmitCustomerDirectShipOrderItem{item},
		})
		if err == nil {
			t.Fatalf("invalid cutover identity unexpectedly accepted: %#v", item)
		}
	}
}

func TestMiniDirectShipCutoverBOMSpecIdentityReservesOneToOneUnitsAndEchoesIdentity(t *testing.T) {
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("productspecmigration.EnsureSchema: %v", err)
	}

	var parentProductID, childProductID, bomID, versionID, bomSpecID, bomVariantID int64
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customers(id,name,active) VALUES(601,'PR600小程序代发客户',true)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,product_kind,active,customer_id,visibility,custom_type,unit_rule_override_json)
		VALUES('PR600小程序父商品','PR600小程序父商品','PARENT-MINI-PR600','roasted_bean',true,601,'customer_only','customer_product','{"inventory_unit":"袋"}'::jsonb) RETURNING id
	`, schema)).Scan(&parentProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,parent_product_id,base_product_id,product_kind,active,customer_id,visibility,custom_type,spec_label,net_content_qty,net_content_unit,auto_derived_sku,derived_spec_key,derived_spec_name,derived_sales_unit)
		VALUES('PR600小程序旧子SKU','PR600小程序旧子SKU','LEGACY-MINI-CODE',$1,$1,'roasted_bean',true,601,'customer_only','derived_sku','227g',227,'g',true,'227g','227g','bag') RETURNING id
	`, schema), parentProductID).Scan(&childProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status) VALUES('BOM-PR600-MINI','PR600小程序BOM','product',$1,'active') RETURNING id`, schema), parentProductID).Scan(&bomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit) VALUES($1,'v1','published',1,'袋') RETURNING id`, schema), bomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit) VALUES($1,'BSP-MINI-227','NEW-MINI-BARCODE','bag-227','227g袋','袋') RETURNING id`, schema), bomID).Scan(&bomSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES($1,$2,'227g袋','袋',true,1) RETURNING id`, schema), versionID, bomSpecID).Scan(&bomVariantID); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by) VALUES('product',$1,$2,$3,true,'test')`, schema), []any{parentProductID, bomID, versionID}},
		{fmt.Sprintf(`INSERT INTO %s.product_bom_spec_migrations(product_id,state,cutover_by,cutover_at) VALUES($1,'cutover','test',now())`, schema), []any{parentProductID}},
		{fmt.Sprintf(`INSERT INTO %s.legacy_child_sku_bom_spec_mappings(parent_product_id,legacy_child_product_id,bom_id,bom_spec_id,bom_variant_id,legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,created_by) VALUES($1,$2,$3,$4,$5,'227g','227g','bag',227,'test')`, schema), []any{parentProductID, childProductID, bomID, bomSpecID, bomVariantID}},
		{fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,sort_order,active,customer_id) VALUES('PR600-MINI-WH','PR600客户成品仓','finished',1,true,601)`, schema), nil},
		{fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES($1,$2,$3,0,'PR600-MINI-WH',4,0)`, schema), []any{parentProductID, bomSpecID, bomVariantID}},
		{fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status) VALUES('PR600-MINI-BATCH','finished_product',$1,'PR600小程序父商品 227g袋',$2,$3,0,'production_work_order',6001,0,4,0,4,'passed')`, schema), []any{parentProductID, bomSpecID, bomVariantID}},
		{fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units) VALUES('finished_product',$1,'PR600小程序父商品 227g袋',$2,$3,0,'PR600-MINI-WH','production_work_order',6001,'PR600-MINI-BATCH',0,0,4,4)`, schema), []any{parentProductID, bomSpecID, bomVariantID}},
	} {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed canonical mini direct ship: %v\n%s", err, statement.sql)
		}
	}
	archivedVariantID := bomVariantID
	repo := NewRepository(pool, schema)
	historicalCmd := app.MiniDirectShipCommand{
		CustomerID: 601, EmployeeID: 701, MiniUserID: 801, IdempotencyKey: "PR600-MINI-HISTORICAL",
		RecipientName: "历史单据", RecipientPhone: "13800138000", DetailAddress: "咖啡路8号", Actor: "mini_user:801",
		Items: []app.MiniDirectShipItemCommand{{ProductID: parentProductID, BomSpecID: bomSpecID, InventoryUnit: "袋", Qty: 1}},
	}
	historical, err := repo.SubmitMiniDirectShip(ctx, historicalCmd)
	if err != nil || len(historical.Items) != 1 || historical.Items[0].BomVariantID != archivedVariantID {
		t.Fatalf("historical V1 request=%+v err=%v", historical, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='archived' WHERE id=$1`, schema), versionID); err != nil {
		t.Fatal(err)
	}
	var currentVersionID, currentVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v2','published',1,'袋') RETURNING id
	`, schema), bomID).Scan(&currentVersionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'227g袋','袋',true,1) RETURNING id
	`, schema), currentVersionID, bomSpecID).Scan(&currentVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_output_bindings SET bom_version_id=$1
		WHERE output_type='product' AND output_id=$2 AND is_default=true
	`, schema), currentVersionID, parentProductID); err != nil {
		t.Fatal(err)
	}
	bomVariantID = currentVariantID
	historicalRetry, err := repo.SubmitMiniDirectShip(ctx, historicalCmd)
	if err != nil || historicalRetry.ID != historical.ID || len(historicalRetry.Items) != 1 || historicalRetry.Items[0].BomVariantID != archivedVariantID {
		t.Fatalf("historical retry after V2=%+v err=%v", historicalRetry, err)
	}

	catalog, err := repo.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 601})
	if err != nil {
		t.Fatalf("MiniDirectShipCatalog: %v", err)
	}
	foundCanonical := false
	for _, family := range catalog.ProductFamilies {
		specs, _ := family["specs"].([]map[string]any)
		for _, spec := range specs {
			if spec["sku_code"] == "LEGACY-MINI-CODE" {
				t.Fatalf("catalog exposed legacy child code: %#v", spec)
			}
			if spec["product_id"] == parentProductID && spec["bom_spec_id"] == bomSpecID {
				foundCanonical = spec["bom_variant_id"] == bomVariantID && spec["inventory_unit"] == "袋" && spec["sku_code"] == "BSP-MINI-227" && spec["available_qty"] == int64(3)
			}
		}
	}
	if !foundCanonical {
		t.Fatalf("canonical catalog spec missing: %#v", catalog.ProductFamilies)
	}

	created, err := repo.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 601, EmployeeID: 701, MiniUserID: 801, IdempotencyKey: "PR600-MINI-CANONICAL",
		RecipientName: "张三", RecipientPhone: "13800138000", DetailAddress: "咖啡路8号", Actor: "mini_user:801",
		Items: []app.MiniDirectShipItemCommand{{ProductID: parentProductID, BomSpecID: bomSpecID, InventoryUnit: "袋", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("SubmitMiniDirectShip canonical: %v", err)
	}
	if len(created.Items) != 1 || created.Items[0].ProductID != parentProductID || created.Items[0].BomSpecID != bomSpecID || created.Items[0].BomVariantID != bomVariantID || created.Items[0].SpecG != 0 || created.Items[0].InventoryUnit != "袋" || created.Items[0].SKUCode != "BSP-MINI-227" {
		t.Fatalf("created canonical echo=%#v", created)
	}
	var requestProductID, requestBomSpecID, requestBomVariantID, requestSpecG int64
	var allocationBomSpecID, allocationBomVariantID, allocationSpecG, allocatedUnits, allocatedG int64
	var orderProductID, orderBomSpecID, orderBomVariantID int64
	var orderUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id,bom_variant_id,spec_g FROM %s.customer_direct_ship_request_items WHERE request_id=$1`, schema), created.ID).Scan(&requestProductID, &requestBomSpecID, &requestBomVariantID, &requestSpecG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id,bom_variant_id,spec_g,allocated_units,allocated_g FROM %s.customer_direct_ship_request_allocations WHERE request_id=$1`, schema), created.ID).Scan(&allocationBomSpecID, &allocationBomVariantID, &allocationSpecG, &allocatedUnits, &allocatedG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id,bom_variant_id,unit FROM %s.order_items WHERE order_id IN (SELECT order_id FROM %s.customer_direct_ship_request_orders WHERE request_id=$1)`, schema, schema), created.ID).Scan(&orderProductID, &orderBomSpecID, &orderBomVariantID, &orderUnit); err != nil {
		t.Fatal(err)
	}
	if requestProductID != parentProductID || requestBomSpecID != bomSpecID || requestBomVariantID != bomVariantID || requestSpecG != 0 || allocationBomSpecID != bomSpecID || allocationBomVariantID != bomVariantID || allocationSpecG != 0 || allocatedUnits != 2 || allocatedG != 0 || orderProductID != parentProductID || orderBomSpecID != bomSpecID || orderBomVariantID != bomVariantID || orderUnit != "袋" {
		t.Fatalf("canonical persistence request=%d/%d/%d/%d allocation=%d/%d/%d/%d/%d order=%d/%d/%d/%q", requestProductID, requestBomSpecID, requestBomVariantID, requestSpecG, allocationBomSpecID, allocationBomVariantID, allocationSpecG, allocatedUnits, allocatedG, orderProductID, orderBomSpecID, orderBomVariantID, orderUnit)
	}
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d AND action='submit'", created.ID), 1)

	for name, item := range map[string]app.MiniDirectShipItemCommand{
		"missing spec":  {ProductID: parentProductID, SpecG: 227, Qty: 1},
		"legacy child":  {ProductID: childProductID, SpecG: 227, Qty: 1},
		"stale variant": {ProductID: parentProductID, BomSpecID: bomSpecID, BomVariantID: archivedVariantID, Qty: 1},
	} {
		_, err := repo.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{
			CustomerID: 601, EmployeeID: 701, MiniUserID: 801, IdempotencyKey: "PR600-MINI-REJECT-" + name,
			RecipientName: "拒绝", RecipientPhone: "13800138000", DetailAddress: "咖啡路8号", Actor: "mini_user:801",
			Items: []app.MiniDirectShipItemCommand{item},
		})
		if err == nil {
			t.Fatalf("%s unexpectedly accepted", name)
		}
	}
}

func TestCustomerInventoryBatchDetailUsesParentAndBOMSpecIdentityAcrossVariants(t *testing.T) {
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("productspecmigration.EnsureSchema: %v", err)
	}

	var parentProductID, bomID, version1ID, version2ID int64
	var bagSpecID, boxSpecID, oldBagVariantID, currentBagVariantID, boxVariantID int64
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customers(id,name,active) VALUES(604,'PR600库存详情客户',true)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,product_kind,active)
		VALUES('PR600库存详情父商品','PR600库存详情父商品','PARENT-INVENTORY-PR600','roasted_bean',true) RETURNING id
	`, schema)).Scan(&parentProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status)
		VALUES('BOM-PR600-INVENTORY-DETAIL','PR600库存详情BOM','product',$1,'active') RETURNING id
	`, schema), parentProductID).Scan(&bomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit) VALUES($1,'v1','archived',1,'袋') RETURNING id`, schema), bomID).Scan(&version1ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit) VALUES($1,'v2','published',1,'袋') RETURNING id`, schema), bomID).Scan(&version2ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit) VALUES($1,'BSP-INVENTORY-BAG','','bag','袋装','袋') RETURNING id`, schema), bomID).Scan(&bagSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit) VALUES($1,'BSP-INVENTORY-BOX','','box','盒装','盒') RETURNING id`, schema), bomID).Scan(&boxSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES($1,$2,'袋装旧版','袋',true,1) RETURNING id`, schema), version1ID, bagSpecID).Scan(&oldBagVariantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES($1,$2,'袋装当前版','袋',true,1) RETURNING id`, schema), version2ID, bagSpecID).Scan(&currentBagVariantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES($1,$2,'盒装','盒',false,2) RETURNING id`, schema), version2ID, boxSpecID).Scan(&boxVariantID); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,sort_order,active,customer_id) VALUES('PR600-INVENTORY-DETAIL-WH','PR600库存详情成品仓','finished',1,true,604)`, schema), nil},
		{fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES ($1,$2,$3,0,'PR600-INVENTORY-DETAIL-WH',2,0),($1,$4,$5,0,'PR600-INVENTORY-DETAIL-WH',1,0)`, schema), []any{parentProductID, bagSpecID, currentBagVariantID, boxSpecID, boxVariantID}},
		{fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status) VALUES ('PR600-INVENTORY-BAG-OLD','finished_product',$1,'PR600库存详情父商品 袋装旧版',$2,$6,0,'production_work_order',6101,0,1,0,1,'passed'),('PR600-INVENTORY-BAG-CURRENT','finished_product',$1,'PR600库存详情父商品 袋装当前版',$2,$3,0,'production_work_order',6102,0,1,0,1,'passed'),('PR600-INVENTORY-BOX','finished_product',$1,'PR600库存详情父商品 盒装',$4,$5,0,'production_work_order',6103,0,1,0,1,'passed')`, schema), []any{parentProductID, bagSpecID, currentBagVariantID, boxSpecID, boxVariantID, oldBagVariantID}},
		{fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units) VALUES ('finished_product',$1,'PR600库存详情父商品 袋装旧版',$2,$6,0,'PR600-INVENTORY-DETAIL-WH','production_work_order',6101,'PR600-INVENTORY-BAG-OLD',0,0,1,1),('finished_product',$1,'PR600库存详情父商品 袋装当前版',$2,$3,0,'PR600-INVENTORY-DETAIL-WH','production_work_order',6102,'PR600-INVENTORY-BAG-CURRENT',0,0,1,2),('finished_product',$1,'PR600库存详情父商品 盒装',$4,$5,0,'PR600-INVENTORY-DETAIL-WH','production_work_order',6103,'PR600-INVENTORY-BOX',0,0,1,1)`, schema), []any{parentProductID, bagSpecID, currentBagVariantID, boxSpecID, boxVariantID, oldBagVariantID}},
	} {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewRepository(pool, schema)
	batches, err := repo.ListCustomerCentralInventoryBatches(ctx, app.CustomerInventoryBatchQuery{
		CustomerID: 604, ProductID: parentProductID, BomSpecID: bagSpecID, BomVariantID: currentBagVariantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("bag detail batches=%#v; canonical spec detail must not include the sibling box spec", batches)
	}
	seenVariants := map[int64]bool{}
	for _, batch := range batches {
		if batch.ProductID != parentProductID || batch.BomSpecID != bagSpecID {
			t.Fatalf("unexpected canonical inventory identity: %#v", batch)
		}
		seenVariants[batch.BomVariantID] = true
	}
	if !seenVariants[oldBagVariantID] || !seenVariants[currentBagVariantID] {
		t.Fatalf("variant is trace metadata and both historical/current variants must remain visible: %#v", batches)
	}
}
