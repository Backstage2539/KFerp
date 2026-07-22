package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func TestValidateConcreteOrderPriceSourceIdentityRejectsPublicationSKUAndParentMismatch(t *testing.T) {
	usage := orderbeans.Usage{PublicationID: 901, VersionNo: "V1"}
	spec := orderbeans.PublishedProductSpec{SKUID: 551, ParentProductID: 550}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "publication", raw: `{"publication_id":902}`, want: "价格表版本"},
		{name: "sku", raw: `{"publication_id":901,"sku_id":552}`, want: "SKU"},
		{name: "parent", raw: `{"publication_id":901,"parent_product_id":600}`, want: "商品"},
		{name: "frozen sku", raw: `{"publication_id":901,"effective_sales_spec":{"sku_id":552}}`, want: "有效销售规格 SKU"},
		{name: "frozen spec", raw: `{"publication_id":901,"effective_sales_spec":{"sku_id":551,"spec_label":"454g"}}`, want: "spec_label"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConcreteOrderPriceSourceIdentity(tt.raw, usage, spec)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validate source err = %v, want %q", err, tt.want)
			}
		})
	}
	if err := validateConcreteOrderPriceSourceIdentity(`{"publication_id":901,"sku_id":551,"parent_product_id":550,"effective_sales_spec":{"sku_id":551}}`, usage, spec); err != nil {
		t.Fatalf("matching source identity: %v", err)
	}
}

func TestManualConcreteOrderPriceSourceKeepsFrozenPublicationAndSKU(t *testing.T) {
	selection := concreteOrderPublicationSelection{
		Strict: true, ListType: "commercial",
		Usage: orderbeans.Usage{PublicationID: 901, VersionNo: "V1"},
		Spec: orderbeans.PublishedProductSpec{
			SKUID: 551, ParentProductID: 550, QuantityBasis: "sales_spec_count",
			EffectiveSalesSpecJSON: `{"sku_id":551,"spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}`,
		},
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(manualConcreteOrderPriceSourceJSON(selection, 551)), &got); err != nil {
		t.Fatal(err)
	}
	if got["source"] != "manual" || got["publication_id"] != float64(901) || got["sku_id"] != float64(551) || got["parent_product_id"] != float64(550) || got["quantity_basis"] != "sales_spec_count" {
		t.Fatalf("manual source = %#v", got)
	}
	frozen, _ := got["effective_sales_spec"].(map[string]any)
	if frozen["spec_label"] != "227g" || frozen["sales_unit"] != "袋" {
		t.Fatalf("manual frozen spec = %#v", frozen)
	}
}

func TestConcreteOrderSpecWeightUsesFrozenNetContent(t *testing.T) {
	for _, tt := range []struct {
		qty  float64
		unit string
		want int64
	}{
		{qty: 227, unit: "g", want: 227},
		{qty: 1, unit: "kg", want: 1000},
		{qty: 1, unit: "lb", want: 454},
	} {
		if got := concreteOrderSpecWeightG(orderbeans.PublishedProductSpec{NetContentQty: tt.qty, NetContentUnit: tt.unit}); got != tt.want {
			t.Fatalf("weight %v%s = %d, want %d", tt.qty, tt.unit, got, tt.want)
		}
	}
}

func TestOrderRepositoryValidatesConcretePublicationBeforeManualPriceBypass(t *testing.T) {
	raw, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	guard := strings.Index(source, "selection, err = resolveConcreteOrderPublicationSelectionTx")
	if guard < 0 {
		t.Fatal("concrete publication guard call not found")
	}
	manual := strings.Index(source[guard:], "if items[idx].manualPrice != nil {")
	if manual < 0 {
		t.Fatal("manual-price branch not found after concrete publication guard")
	}
	for _, want := range []string{
		"submittedParentProductID != product.ParentProductID",
		"价格表 SKU 不属于订单商品",
		"COALESCE(p.active,true)=true",
		"derived_spec_status",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("concrete order guard missing %q", want)
		}
	}
}

func TestOrderAliasSnapshotResolvesAtEffectiveParent(t *testing.T) {
	raw, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func resolveOrderItemCustomerAliasSnapshotTx")
	end := strings.Index(source[start:], "func (r Repository) UpdateHeader")
	if start < 0 || end <= 0 {
		t.Fatal("alias snapshot resolver not found")
	}
	resolver := source[start : start+end]
	for _, want := range []string{"effectiveParentID", "alias_product.parent_product_id", "effective_parent.name"} {
		if !strings.Contains(resolver, want) {
			t.Fatalf("effective-parent alias resolver missing %q", want)
		}
	}
}

func TestResolveConcreteOrderPublicationSelectionAndParentAliasPostgres(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	ddl := fmt.Sprintf(`
		CREATE TABLE %[1]s.products (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_product_id BIGINT NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			sku_name TEXT NOT NULL DEFAULT '',
			spec_label TEXT NOT NULL DEFAULT '',
			sku_code TEXT NOT NULL DEFAULT '',
			product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
			auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
			derived_spec_status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %[1]s.bean_list_publications (
			id BIGINT PRIMARY KEY,
			version_no TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'published',
			list_type TEXT NOT NULL DEFAULT 'commercial',
			publication_purpose TEXT NOT NULL DEFAULT 'factory_supply',
			owner_type TEXT NOT NULL DEFAULT 'official',
			owner_key TEXT NOT NULL DEFAULT '',
			content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			published_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.customer_product_aliases (
			id BIGINT PRIMARY KEY,
			customer_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			customer_item_code TEXT NOT NULL DEFAULT '',
			brand_name TEXT NOT NULL DEFAULT '',
			display_category_id BIGINT NOT NULL DEFAULT 0,
			include_in_price_list BOOLEAN NOT NULL DEFAULT true,
			sort_order INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true
		);
		INSERT INTO %[1]s.products(id,name,active) VALUES (550,'乌拉嘎',true),(600,'其他商品',true);
		INSERT INTO %[1]s.products(id,name,parent_product_id,active,sku_name,spec_label,sku_code,product_kind,auto_derived_sku)
		VALUES
			(551,'乌拉嘎 227g',550,true,'227g袋装','227g','SKU-WLG-227','roasted_bean',true),
			(552,'乌拉嘎 454g',550,true,'454g袋装','454g','SKU-WLG-454','roasted_bean',true);
		INSERT INTO %[1]s.bean_list_publications(id,version_no,content_json) VALUES
			(901,'V1','{"price_rows":[{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,"effective_sales_spec":{"sku_id":551,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]}'::jsonb),
			(902,'LEGACY','{"groups":[{"items":[{"productId":551,"commercial_wholesale_tiers":[{"spec_g":227,"min_qty":1,"price_per_unit":68}]}]}]}'::jsonb),
			(903,'LEGACY-OTHER','{"groups":[{"items":[{"productId":552,"commercial_wholesale_tiers":[{"spec_g":454,"min_qty":1,"price_per_unit":118}]}]}]}'::jsonb);
		INSERT INTO %[1]s.customer_product_aliases(id,customer_id,product_id,display_name,customer_item_code,active)
		VALUES
			(81,3,550,'客户乌拉嘎','C-WLG',true),
			(82,4,550,'其他客户乌拉嘎','OTHER-CUSTOMER',true),
			(83,3,600,'其他父商品别名','OTHER-PARENT',true),
			(84,3,550,'已停用乌拉嘎','INACTIVE',false);
	`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatalf("prepare concrete order guard schema: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cmd := salesapp.SaveOrderCommand{CustomerID: 3, CommercialBeanListPublicationID: 901}
	selection, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, cmd, 551, 550, "roasted_bean", false, 901, "commercial", `{"publication_id":901,"sku_id":551,"parent_product_id":550}`)
	if err != nil {
		t.Fatalf("resolve concrete selection: %v", err)
	}
	if !selection.Strict || selection.Usage.PublicationID != 901 || selection.Spec.SKUID != 551 || selection.Spec.ParentProductID != 550 || selection.Product.ParentProductName != "乌拉嘎" {
		t.Fatalf("concrete selection = %+v", selection)
	}
	alias, err := resolveOrderItemCustomerAliasSnapshotTx(ctx, tx, schema, 3, 551, 81)
	if err != nil {
		t.Fatalf("resolve parent alias for child SKU: %v", err)
	}
	if alias.AliasID != 81 || alias.DisplayName != "客户乌拉嘎" || alias.ProductName != "乌拉嘎" || alias.ProductCode != "SKU-WLG-227" {
		t.Fatalf("parent alias snapshot = %+v", alias)
	}
	for _, tt := range []struct {
		name    string
		aliasID int64
	}{
		{name: "cross customer", aliasID: 82},
		{name: "cross parent", aliasID: 83},
		{name: "inactive", aliasID: 84},
	} {
		t.Run(tt.name+" alias", func(t *testing.T) {
			if _, err := resolveOrderItemCustomerAliasSnapshotTx(ctx, tx, schema, 3, 551, tt.aliasID); err == nil || !strings.Contains(err.Error(), "客户商品别名") {
				t.Fatalf("alias %d err = %v", tt.aliasID, err)
			}
		})
	}
	if _, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, cmd, 551, 600, "roasted_bean", false, 901, "commercial", ""); err == nil || !strings.Contains(err.Error(), "归属") {
		t.Fatalf("cross-parent submitted SKU err = %v", err)
	}
	if _, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, cmd, 552, 550, "roasted_bean", false, 901, "commercial", ""); err == nil || !strings.Contains(err.Error(), "不包含") {
		t.Fatalf("unpublished SKU err = %v", err)
	}
	invalidPublicationCmd := salesapp.SaveOrderCommand{CustomerID: 3, CommercialBeanListPublicationID: 9999}
	if _, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, invalidPublicationCmd, 551, 0, "roasted_bean", false, 9999, "commercial", ""); err == nil || !strings.Contains(err.Error(), "价格表版本无效") {
		t.Fatalf("explicit invalid publication without parent err = %v", err)
	}
	legacy, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, salesapp.SaveOrderCommand{CustomerID: 3, CommercialBeanListPublicationID: 902}, 551, 550, "roasted_bean", false, 902, "commercial", "")
	if err != nil || legacy.Strict {
		t.Fatalf("legacy publication compatibility = %+v err=%v", legacy, err)
	}
	legacyOtherCmd := salesapp.SaveOrderCommand{CustomerID: 3, CommercialBeanListPublicationID: 903}
	if _, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, legacyOtherCmd, 551, 0, "roasted_bean", false, 903, "commercial", ""); err == nil || !strings.Contains(err.Error(), "不包含") {
		t.Fatalf("explicit legacy publication without SKU err = %v", err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET active=false WHERE id=551`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveConcreteOrderPublicationSelectionTx(ctx, tx, schema, cmd, 551, 550, "roasted_bean", false, 901, "commercial", ""); err == nil || !strings.Contains(err.Error(), "停用") {
		t.Fatalf("inactive SKU err = %v", err)
	}
}

func TestFetchStandardOrderPublicationTiersKeepsOldAndNewConcreteSKUVersionsPostgres(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	ddl := fmt.Sprintf(`
		CREATE TABLE %[1]s.bean_list_publications (
			id BIGINT PRIMARY KEY, version_no TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'published',
			list_type TEXT NOT NULL DEFAULT 'commercial', publication_purpose TEXT NOT NULL DEFAULT 'factory_supply',
			owner_type TEXT NOT NULL DEFAULT 'official', owner_key TEXT NOT NULL DEFAULT '',
			content_json JSONB NOT NULL DEFAULT '{}'::jsonb, published_at TIMESTAMPTZ NOT NULL
		);
		INSERT INTO %[1]s.bean_list_publications(id,version_no,content_json,published_at) VALUES
		(901,'V1','{"price_rows":[{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","min_qty":1,"final_unit_price":68,"effective_sales_spec":{"sku_id":551,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]}'::jsonb,'2026-07-20 09:00:00+08'),
		(902,'V2','{"price_rows":[
			{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","min_qty":1,"final_unit_price":70,"effective_sales_spec":{"sku_id":551,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}},
			{"product_id":550,"sku_id":552,"parent_product_id":550,"quantity_basis":"sales_spec_count","min_qty":1,"final_unit_price":118,"effective_sales_spec":{"sku_id":552,"spec_name":"454g袋装","spec_label":"454g","sales_unit":"袋","net_content_qty":454,"net_content_unit":"g"}}
		]}'::jsonb,'2026-07-21 09:00:00+08');
	`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatalf("prepare publication query schema: %v", err)
	}
	repo := NewRepository(pool, schema)
	products := []salesapp.ProductOption{
		{ID: 551, ProductKind: "roasted_bean"},
		{ID: 552, ProductKind: "roasted_bean"},
	}
	got, err := repo.fetchStandardOrderPublicationTiers(ctx, products, "commercial")
	if err != nil {
		t.Fatalf("fetch concrete publication tiers: %v", err)
	}
	firstSKU := got[orderPublicationProductKey{ProductID: 551}]
	if len(firstSKU) != 2 || firstSKU[0].PublicationID != 902 || firstSKU[1].PublicationID != 901 {
		t.Fatalf("SKU 551 versions = %+v, want V2 and V1", firstSKU)
	}
	secondSKU := got[orderPublicationProductKey{ProductID: 552}]
	if len(secondSKU) != 1 || secondSKU[0].PublicationID != 902 {
		t.Fatalf("SKU 552 versions = %+v, want V2 only", secondSKU)
	}
}
