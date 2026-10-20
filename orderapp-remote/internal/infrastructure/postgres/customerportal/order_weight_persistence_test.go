package customerportal

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerportal"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	"testing"
)

func TestPR706PortalOrderWritersFreezeBothWeights(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalCostingSchema(t, ctx, pool, schema)
	f := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.products SET customer_id=0,visibility='public' WHERE id=$1`, schema), f.ProductID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=250,material_loss_rate=20 WHERE id=$1`, schema), f.BomVariantID); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityView(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	mall, err := repo.SaveMallProduct(ctx, app.SaveMallProductCommand{ProductID: f.ProductID, BomSpecID: f.BomSpecID, Title: "PR706商品", UnitPrice: 68, Status: app.MallProductStatusPublished, Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	order, err := repo.CreateMallOrder(ctx, app.CreateMallOrderCommand{CustomerID: f.CustomerID, CreatedByMiniUserID: 706, RecipientName: "张三", RecipientPhone: "13800138000", RecipientAddress: "上海测试路", Items: []app.MallOrderItemCommand{{MallProductID: mall.ID, ProductID: f.ProductID, BomSpecID: f.BomSpecID, Qty: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	assertWeights := func(id int64) {
		t.Helper()
		var material, logistics float64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8 FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&material, &logistics); err != nil {
			t.Fatal(err)
		}
		if material != 908 || logistics != 1000 {
			t.Fatalf("portal weights %v/%v want908/1000", material, logistics)
		}
	}
	assertWeights(order.OrderID)
	content := fmt.Sprintf(`{"price_rows":[{"product_id":%d,"bom_spec_id":%d,"bom_variant_id":%d,"min_qty":1,"final_unit_price":68,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]}`, f.ProductID, f.BomSpecID, f.BomVariantID)
	var publicationID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,config_json,content_json,changelog,actor)VALUES('commercial','PR706','published','official','','{}',$1::jsonb,'','test') RETURNING id`, schema), content).Scan(&publicationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES($1,'product_order','commercial',$2)`, schema), f.CustomerID, publicationID); err != nil {
		t.Fatal(err)
	}
	fulfilled, err := repo.CreateFulfillmentOrder(ctx, app.CreateFulfillmentOrderCommand{CustomerID: f.CustomerID, CreatedByMiniUserID: 706, PortalServiceCode: app.PortalServiceProductOrder, RecipientName: "张三", RecipientPhone: "13800138000", RecipientAddress: "上海测试路", ProductID: f.ProductID, BomSpecID: f.BomSpecID, BomVariantID: f.BomVariantID, BeanListPublicationID: publicationID, InventoryUnit: "袋", Qty: 4})
	if err != nil {
		t.Fatal(err)
	}
	assertWeights(fulfilled.OrderID)
	summaries, err := repo.listCustomerOrderItems(ctx, []int64{fulfilled.OrderID})
	if err != nil {
		t.Fatal(err)
	}
	row := summaries[fulfilled.OrderID][0]
	if row.MaterialItemWeightG == nil || row.LogisticsItemWeightG == nil || *row.MaterialItemWeightG != "908.000000" || *row.LogisticsItemWeightG != "1000.000000" {
		t.Fatalf("customer summary lost frozen weights: %+v", row)
	}
}
