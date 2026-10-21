package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	costingapp "orderapp/internal/application/costing"
	salesapp "orderapp/internal/application/sales"
	costingpg "orderapp/internal/infrastructure/postgres/costing"
)

// Exercise the actual publication writer, order-form queries, shared catalog,
// and write guards in an isolated schema, rather than maintaining another DTO fixture.
func TestPublishedOrderCatalogLifecyclePostgres(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	source, err := os.ReadFile("../costing/schema.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "CREATE TABLE IF NOT EXISTS %[1]s.bean_list_publications (")
	end := start + strings.Index(string(source)[start:], "\n);") + 4
	ddl := strings.ReplaceAll(string(source)[start:end], "%[1]s", schema)
	ddl += fmt.Sprintf(`;
	CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY, active BOOLEAN NOT NULL DEFAULT true);
	CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY, customer_id BIGINT NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT true);
	CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
	INSERT INTO %[1]s.customers(id) VALUES(487);
	INSERT INTO %[1]s.products(id) VALUES(100),(200);`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	writer, reader := costingpg.NewRepository(pool, schema), NewRepository(pool, schema)
	command := func(key, list string, product, spec, category int64, price float64, def bool) costingapp.PublishBeanListCommand {
		config := map[string]any{}
		costingapp.SetBeanListBatchMetadata(config, costingapp.PublicationTableMetadata{TableKey: key, TableName: key, IsDefaultTable: def})
		return costingapp.PublishBeanListCommand{ListType: list, PublicationPurpose: "factory_supply", OwnerType: "official", Version: "V3.0.1", ClassificationTemplateID: category, ClassificationTemplateName: "分类" + fmt.Sprint(category), Config: config, Actor: "catalog-lifecycle-test", Content: map[string]any{"price_rows": []any{map[string]any{"product_id": product, "parent_product_id": product, "bom_spec_id": spec, "bom_variant_id": spec + 1000, "quantity_basis": "sales_spec_count", "min_qty": 1, "final_unit_price": price, "effective_sales_spec": map[string]any{"bom_spec_id": spec, "bom_variant_id": spec + 1000, "spec_name": key, "inventory_unit": "袋"}}}}}
	}
	first, err := writer.SaveBeanListBatch(ctx, []costingapp.PublishBeanListCommand{command("small", "commercial", 100, 901, 56, 38, true), command("large", "commercial", 100, 902, 56, 68, false)}, true)
	if err != nil {
		t.Fatal(err)
	}
	retail, err := writer.SaveBeanListBatch(ctx, []costingapp.PublishBeanListCommand{command("retail", "retail", 100, 901, 56, 48, true)}, true)
	if err != nil {
		t.Fatal(err)
	}
	custom, err := writer.SaveBeanListBatch(ctx, []costingapp.PublishBeanListCommand{command("custom", "commercial", 200, 903, 88, 18, true)}, true)
	if err != nil {
		t.Fatal(err)
	}
	load := func() salesapp.OrderFormData {
		t.Helper()
		options, err := reader.fetchOrderBeanListVersionOptions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		products := []salesapp.ProductOption{{ID: 100, Name: "熟豆", ProductKind: "roasted_bean", Visibility: "public"}, {ID: 200, Name: "自定义商品", ProductKind: "generic", Visibility: "public"}}
		for _, list := range []string{"commercial", "retail"} {
			tiers, err := reader.fetchStandardOrderPublicationTiers(ctx, products, list)
			if err != nil {
				t.Fatal(err)
			}
			for i := range products {
				products[i].Tiers = append(products[i].Tiers, tiers[orderPublicationProductKeyForProduct(products[i])]...)
			}
		}
		form := salesapp.OrderFormData{Products: products, BeanListVersionOptions: options, Customers: []salesapp.CustomerOption{{ID: 487, DefaultOrderTypeID: 2}}}
		for _, identity := range [][2]int64{{100, 901}, {100, 902}, {200, 903}} {
			option := salesapp.ProductBOMSpecOption{ParentProductID: identity[0], BomSpecID: identity[1], BomVariantID: identity[1] + 1000, SpecName: fmt.Sprint(identity[1]), MigrationState: "cutover", Published: true, InventoryUnit: "袋"}
			for _, p := range products {
				if p.ID == identity[0] {
					option.Tiers = orderBOMSpecPublicationTiers(option, p.Tiers)
				}
			}
			form.ProductBOMSpecOptions = append(form.ProductBOMSpecOptions, option)
		}
		return form
	}
	assertCatalog := func(form salesapp.OrderFormData, ids []int64, spec int64, price float64, updated bool) {
		t.Helper()
		got, err := salesapp.ResolveOrderCatalog(form, 487, ids, true)
		if err != nil {
			t.Fatal(err)
		}
		if got.Updated != updated || len(got.Catalog.Families) != 2 || len(got.Catalog.BOMSpecOptions) != 2 {
			t.Fatalf("catalog=%+v", got)
		}
		found := false
		for _, option := range got.Catalog.BOMSpecOptions {
			if option.ParentProductID == 100 {
				found = true
				if option.BomSpecID != spec || len(option.Tiers) != 1 || option.Tiers[0].UnitPrice != price {
					t.Fatalf("selected table/spec price leaked: %+v", option)
				}
			}
		}
		if !found {
			t.Fatal("published roasted family disappeared for retail customer")
		}
	}
	form := load()
	assertCatalog(form, nil, 901, 38, false)
	assertCatalog(form, []int64{first[1].ID}, 902, 68, false)
	assertCatalog(form, []int64{retail[0].ID}, 901, 48, false)
	if _, err := writer.SaveBeanListBatch(ctx, []costingapp.PublishBeanListCommand{command("small", "commercial", 100, 901, 56, 999, true)}, false); err != nil {
		t.Fatal(err)
	}
	assertCatalog(load(), nil, 901, 38, false)
	second, err := writer.SaveBeanListBatch(ctx, []costingapp.PublishBeanListCommand{command("small", "commercial", 100, 901, 56, 40, true), command("large", "commercial", 100, 902, 56, 70, false)}, true)
	if err != nil {
		t.Fatal(err)
	}
	form = load()
	assertCatalog(form, []int64{first[1].ID}, 902, 70, true)
	if _, err := salesapp.ResolveOrderPriceTableSelection(form.BeanListVersionOptions, 487, []int64{first[1].ID}, true); err == nil {
		t.Fatal("stale write accepted")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cmd := salesapp.SaveOrderCommand{CustomerID: 487, SelectedPriceTableIDs: []int64{second[1].ID, custom[0].ID}}
	meta, err := validateSelectedPriceTablesTx(ctx, tx, schema, cmd, []int64{second[1].ID, custom[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(second[1].Content)
	if err := validateNamedOrderItemSnapshot(raw, meta[second[1].ID], 100, 902, 1902, 0, 1, "袋", 0); err != nil {
		t.Fatal(err)
	}
	if err := validateNamedOrderItemSnapshot(raw, meta[second[1].ID], 100, 901, 1901, 0, 1, "袋", 0); err == nil {
		t.Fatal("unpublished sibling specification accepted")
	}
	if _, err := validateSelectedPriceTablesTx(ctx, tx, schema, cmd, []int64{retail[0].ID}); err == nil {
		t.Fatal("cross-table price accepted")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := writer.WithdrawBeanList(ctx, costingapp.WithdrawBeanListCommand{ID: second[1].ID, PublicationPurpose: "factory_supply", OwnerType: "official", Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := salesapp.ResolveOrderCatalog(load(), 487, []int64{second[1].ID}, true); err == nil {
		t.Fatal("withdrawn selection accepted")
	}
	var audits int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".audit_logs").Scan(&audits); err != nil || audits < 7 {
		t.Fatalf("publication audit count=%d err=%v", audits, err)
	}
}
