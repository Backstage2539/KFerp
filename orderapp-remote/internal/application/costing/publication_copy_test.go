package costing

import (
	"context"
	"reflect"
	"testing"

	domain "orderapp/internal/domain/costing"
)

type publicationCopyBatchRepo struct {
	fakeRepo
	commands []PublishBeanListCommand
	publish  bool
}

func (r *publicationCopyBatchRepo) SaveBeanListBatch(_ context.Context, commands []PublishBeanListCommand, publish bool) ([]BeanListPublication, error) {
	r.commands = commands
	r.publish = publish
	rows := make([]BeanListPublication, len(commands))
	for index, command := range commands {
		meta := BeanListBatchMetadata(command.Config)
		meta.ReleaseID = "copy-release"
		SetBeanListBatchMetadata(command.Config, meta)
		rows[index] = BeanListPublication{
			ID: int64(index + 1), Version: command.Version,
			Config: command.Config, Content: command.Content, Status: "draft",
			PublicationTableMetadata: meta,
		}
	}
	return rows, nil
}

func publicationCopyFixture() *publicationCopyBatchRepo {
	return &publicationCopyBatchRepo{
		fakeRepo: fakeRepo{
			customerInputs: []domain.ProductInput{{ProductID: 101, ParentProductID: 101, CustomerID: 42, Name: "公共名称", CustomerProductDisplayName: "客户商品A"}},
			beanListPublication: &BeanListPublication{
				ID: 88, PublicationPurpose: BeanListPublicationPurposeFactorySupply, ListType: "commercial",
				Status: "published", OwnerType: "official", Version: "V5.1",
				Config: map[string]any{"selectedProductIDs": []any{"101", "102"}, "layoutStyle": "table"},
				Content: map[string]any{
					"title": "来源 V5.1",
					"price_rows": []any{
						map[string]any{"product_id": float64(101), "parent_product_id": float64(101), "tier_label": "A", "final_unit_price": float64(90)},
						map[string]any{"product_id": float64(101), "parent_product_id": float64(101), "tier_label": "B", "final_unit_price": float64(80)},
						map[string]any{"product_id": float64(102), "parent_product_id": float64(102), "tier_label": "A", "final_unit_price": float64(70)},
					},
					"groups": []any{map[string]any{"name": "咖啡", "items": []any{
						map[string]any{"product_id": float64(101), "parent_product_id": float64(101), "name": "来源商品A"},
						map[string]any{"product_id": float64(102), "parent_product_id": float64(102), "name": "来源商品B"},
					}}},
				},
			},
		},
	}
}

func publicationCopyBatch() BeanListBatchCommand {
	return BeanListBatchCommand{
		PublishBeanListCommand: PublishBeanListCommand{
			ListType: "commercial", Version: "V2.1", OwnerType: "customer", OwnerKey: "42",
			PublicationPurpose: BeanListPublicationPurposeFactorySupply, Actor: "employee:7",
		},
		DefaultTableKey: "current",
		Tables: []BeanListBatchTable{
			{Key: "current", Name: "客户当前表", Config: map[string]any{"old": true}, Content: map[string]any{"price_rows": []any{map[string]any{"old": true}}}},
			{Key: "other", Name: "另一张表", Config: map[string]any{"keep": "config"}, Content: map[string]any{"keep": "content"}},
		},
	}
}

func TestCopyBeanListPublicationToDraftReplacesOnlyCurrentTableWithCustomerIntersection(t *testing.T) {
	repo := publicationCopyFixture()
	cmd := BeanListPublicationCopyCommand{
		SourceQuery:         BeanListPublicationQuery{ListType: "commercial", Scope: "official", OwnerType: "official", PublicationPurpose: BeanListPublicationPurposeFactorySupply},
		SourcePublicationID: 88, CustomerID: 42, TargetTableKey: "current", CopyRequestID: "copy-req-1", Batch: publicationCopyBatch(),
	}
	result, err := NewService(repo).CopyBeanListPublicationToDraft(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if repo.publish || len(repo.commands) != 2 {
		t.Fatalf("copy must save the complete batch as a draft, commands=%d publish=%v", len(repo.commands), repo.publish)
	}
	target := repo.commands[0]
	if target.Content["title"] != "客户当前表" || target.Config["old"] != nil || target.Config["layoutStyle"] != "table" || target.PriceSourcePublicationID != 0 {
		t.Fatalf("copy must replace target contents but retain target table identity and copied presentation config: %+v", target)
	}
	rows := target.Content["price_rows"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["tier_label"] != "A" || rows[1].(map[string]any)["tier_label"] != "B" || rows[0].(map[string]any)["frozen_final_price"] != true {
		t.Fatalf("target must keep every published tier and freeze copied final prices: %#v", rows)
	}
	if rows[0].(map[string]any)["name"] != "客户商品A" {
		t.Fatalf("customer product display name was not retained: %#v", rows[0])
	}
	if repo.commands[1].Config["keep"] != "config" || repo.commands[1].Content["keep"] != "content" || result.Stats.CopyProductCount != 1 || result.Stats.CopySpecCount != 1 {
		t.Fatalf("sibling table or intersection result changed: sibling=%+v stats=%+v", repo.commands[1], result.Stats)
	}
	if result.Draft == nil || result.Draft.Version != "V2.1" || len(result.Draft.Tables) != 2 {
		t.Fatalf("copy did not return the complete customer draft release: %+v", result.Draft)
	}
}

func TestCopyBeanListPublicationWithNoIntersectionLeavesTargetUnwritten(t *testing.T) {
	repo := publicationCopyFixture()
	repo.customerInputs = []domain.ProductInput{{ProductID: 999, ParentProductID: 999, CustomerID: 42, Name: "其他商品"}}
	cmd := BeanListPublicationCopyCommand{
		SourceQuery:         BeanListPublicationQuery{ListType: "commercial", Scope: "official", OwnerType: "official", PublicationPurpose: BeanListPublicationPurposeFactorySupply},
		SourcePublicationID: 88, CustomerID: 42, TargetTableKey: "current", CopyRequestID: "copy-req-2", Batch: publicationCopyBatch(),
	}
	if _, err := NewService(repo).CopyBeanListPublicationToDraft(context.Background(), cmd); err == nil || len(repo.commands) != 0 {
		t.Fatalf("no-overlap copy must not overwrite or save target: err=%v commands=%d", err, len(repo.commands))
	}
}

func TestFilterBeanListPublicationCopyKeepsOnlyExactCustomerProductSpecs(t *testing.T) {
	row := func(productID, specID, variantID int, price float64) map[string]any {
		return map[string]any{"product_id": float64(productID), "parent_product_id": float64(productID), "bom_spec_id": float64(specID), "bom_variant_id": float64(variantID), "final_unit_price": price}
	}
	priceASecondTier := row(101, 11, 111, 83)
	priceASecondTier["tier_label"] = "整箱"
	priceASecondTier["min_qty"] = float64(24)
	priceASecondTier["max_qty"] = nil
	priceASecondTier["manual_adjusted"] = true
	config := map[string]any{
		"selectedProductIDs": []any{"101", "102", "103"}, "layoutStyle": "card",
		"price_list_template_selection": map[string]any{"defaults": map[string]any{"pricing_mode": "tier_template"}},
		"product_spec_selections": []any{
			map[string]any{"product_id": float64(101), "parent_product_id": float64(101), "sku_id": float64(1001), "bom_spec_id": float64(11), "bom_variant_id": float64(111)},
			map[string]any{"product_id": float64(102), "parent_product_id": float64(102), "sku_id": float64(1002), "bom_spec_id": float64(22), "bom_variant_id": float64(222)},
			map[string]any{"product_id": float64(103), "parent_product_id": float64(103), "sku_id": float64(1003), "bom_spec_id": float64(33), "bom_variant_id": float64(333)},
		},
	}
	content := map[string]any{
		"title":      "来源版 · V5",
		"price_rows": []any{row(101, 11, 111, 88), priceASecondTier, row(102, 22, 222, 77), row(103, 33, 333, 66)},
		"groups": []any{
			map[string]any{"category": "咖啡", "items": []any{row(101, 11, 111, 88)}},
			map[string]any{"category": "挂耳", "items": []any{row(102, 22, 222, 77), row(103, 33, 333, 66)}},
		},
		"totalItems": 3,
	}
	allowed := []beanListCopyAllowedProduct{{identity: BeanListCopyProductIdentity{
		ProductID: 101, ParentProductID: 101, SKUID: 1001, BOMSpecID: 11, BOMVariantID: 111,
		CustomerID: 42, AliasID: 4201, DisplayName: "客户A商品", ItemCode: "A-01",
	}, keys: map[string]bool{"101:bom:11:111": true}}}

	config, content, stats := filterBeanListPublicationCopy(config, content, allowed, BeanListPublicationCopyStats{})
	rows := content["price_rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("copied price rows = %d, want both published tiers for only product A", len(rows))
	}
	if got := []float64{numberValue(rows[0].(map[string]any)["final_unit_price"]), numberValue(rows[1].(map[string]any)["final_unit_price"])}; !reflect.DeepEqual(got, []float64{88, 83}) {
		t.Fatalf("source final tier prices = %v, want preserved manual prices [88 83]", got)
	}
	if rows[1].(map[string]any)["manual_adjusted"] != true || rows[1].(map[string]any)["tier_label"] != "整箱" {
		t.Fatalf("source final price metadata not copied: %#v", rows[1])
	}
	if rows[0].(map[string]any)["frozen_final_price"] != true || rows[1].(map[string]any)["frozen_final_price"] != true {
		t.Fatalf("copied published prices must remain frozen when the draft is restored: %#v", rows)
	}
	item := content["groups"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
	if item["name"] != "客户A商品" || numberValue(item["customer_product_alias_id"]) != 4201 {
		t.Fatalf("customer item identity not applied: %#v", item)
	}
	if len(content["groups"].([]any)) != 1 || len(config["selectedProductIDs"].([]any)) != 1 || len(config["product_spec_selections"].([]any)) != 1 {
		t.Fatalf("filtered table still contains unavailable products: config=%#v content=%#v", config, content)
	}
	if stats.SourceProductCount != 3 || stats.SourceSpecCount != 3 || stats.CopyProductCount != 1 || stats.CopySpecCount != 1 || stats.SkipProductCount != 2 || stats.SkipSpecCount != 2 {
		t.Fatalf("copy preview counts = %+v", stats)
	}
	if config["layoutStyle"] != "card" || config["price_list_template_selection"] == nil {
		t.Fatalf("full display and pricing configuration must be preserved: %#v", config)
	}
}

func TestFilterBeanListPublicationCopyDoesNotInventNameOrCopyByName(t *testing.T) {
	config := map[string]any{}
	content := map[string]any{"price_rows": []any{map[string]any{"product_id": float64(101), "name": "同名商品", "final_unit_price": 88}}}
	_, filtered, stats := filterBeanListPublicationCopy(config, content, []beanListCopyAllowedProduct{{identity: BeanListCopyProductIdentity{ProductID: 999}, keys: map[string]bool{"999:product": true}}}, BeanListPublicationCopyStats{})
	if len(filtered["price_rows"].([]any)) != 0 || stats.CopySpecCount != 0 || stats.SkipSpecCount != 1 {
		t.Fatalf("source was copied without an exact archive identity: content=%#v stats=%+v", filtered, stats)
	}
}

func TestFilterBeanListPublicationCopyMatchesProductWithoutSKUOrBOMSpec(t *testing.T) {
	config := map[string]any{"selectedProductIDs": []any{"101"}}
	content := map[string]any{"price_rows": []any{map[string]any{
		"product_id": float64(101), "parent_product_id": float64(101), "final_unit_price": float64(88),
	}}}
	allowed := []beanListCopyAllowedProduct{{identity: BeanListCopyProductIdentity{ProductID: 101, ParentProductID: 101, CustomerID: 42, DisplayName: "普通商品"}, keys: map[string]bool{"101:product": true}}}
	config, content, stats := filterBeanListPublicationCopy(config, content, allowed, BeanListPublicationCopyStats{})
	if len(content["price_rows"].([]any)) != 1 || len(config["selectedProductIDs"].([]any)) != 1 || stats.CopyProductCount != 1 {
		t.Fatalf("ordinary product with no SKU/spec identity should copy by product archive identity: config=%#v content=%#v stats=%+v", config, content, stats)
	}
}

func TestPublicationCopyRetryReturnsExistingDraftForSameActorAndRequest(t *testing.T) {
	meta := PublicationTableMetadata{
		ReleaseID: "release-1", TableKey: "current", TableName: "客户价表",
		CopyRequestID: "request-1", CopyActor: "employee:7", CopySourcePublicationID: 88,
		CopySourceVersion: "V5.1", CopySourceProductCount: 3, CopySourceSpecCount: 4,
		CopyProductCount: 1, CopySpecCount: 2, CopySkipProductCount: 2, CopySkipSpecCount: 2,
	}
	repo := &fakeRepo{beanListPublications: []BeanListPublication{
		{PublicationTableMetadata: meta, ID: 121, PublicationPurpose: BeanListPublicationPurposeFactorySupply,
			ListType: "commercial", Version: "V2.3", Status: "draft", OwnerType: "customer", OwnerKey: "42"},
		{PublicationTableMetadata: PublicationTableMetadata{ReleaseID: "release-1", TableKey: "other", TableName: "不变的另一张表"},
			ID: 122, PublicationPurpose: BeanListPublicationPurposeFactorySupply, ListType: "commercial", Version: "V2.3", Status: "draft", OwnerType: "customer", OwnerKey: "42"},
	}}
	svc := NewService(repo)
	result, found, err := svc.existingBeanListPublicationCopy(context.Background(), BeanListPublicationCopyCommand{
		SourcePublicationID: 88, CustomerID: 42, TargetTableKey: "current", CopyRequestID: "request-1",
		Batch: BeanListBatchCommand{PublishBeanListCommand: PublishBeanListCommand{ListType: "commercial", Actor: "employee:7"}},
	})
	if err != nil || !found {
		t.Fatalf("retry lookup found=%v err=%v", found, err)
	}
	if result.Stats.CopyProductCount != 1 || result.Stats.CopySpecCount != 2 || result.Draft == nil || len(result.Draft.Tables) != 2 {
		t.Fatalf("retry result lost copy evidence or sibling tables: %+v", result)
	}
	if result.Draft.Tables[1].TableName != "不变的另一张表" {
		t.Fatalf("retry result omitted untouched named table: %+v", result.Draft.Tables)
	}
	_, found, err = svc.existingBeanListPublicationCopy(context.Background(), BeanListPublicationCopyCommand{
		SourcePublicationID: 88, CustomerID: 42, TargetTableKey: "current", CopyRequestID: "request-1",
		Batch: BeanListBatchCommand{PublishBeanListCommand: PublishBeanListCommand{ListType: "commercial", Actor: "employee:8"}},
	})
	if err != nil || found {
		t.Fatalf("another actor must not inherit retry result: found=%v err=%v", found, err)
	}
}
