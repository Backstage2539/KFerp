package customerfulfillment

import (
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestMiniDirectShipCatalogUsesPublishedBOMSpecNamesDefaultsAndOrder(t *testing.T) {
	products := []salesapp.ProductOption{{
		ID: 91, ParentProductID: 91, Name: "小菠萝", ParentProductName: "小菠萝",
		Tiers: []salesapp.ProductTierOption{
			{BomSpecID: 802, BomVariantID: 902, SalesUnit: "袋", MinQty: 1, UnitPrice: 27},
			{BomSpecID: 801, BomVariantID: 901, SalesUnit: "袋", MinQty: 1, UnitPrice: 31},
		},
	}}
	specs := []salesapp.ProductBOMSpecOption{
		{ParentProductID: 91, BomSpecID: 802, BomVariantID: 902, SpecName: "454g 袋装", InventoryUnit: "袋", Published: true, SortOrder: 10},
		{ParentProductID: 91, BomSpecID: 801, BomVariantID: 901, SpecName: "227g 袋装", InventoryUnit: "袋", Published: true, IsDefault: true, SortOrder: 20},
	}

	catalog := miniDirectShipCatalogFromSales(
		MiniDirectShipCatalogQuery{CustomerID: 7, UsageCode: "direct_ship"},
		[]MiniDirectShipPriceTable{{ID: 31, TableName: "客户代发表", VersionNo: "V3.0.43"}},
		products,
		specs,
	)
	if len(catalog.ProductFamilies) != 1 {
		t.Fatalf("families=%#v", catalog.ProductFamilies)
	}
	family := catalog.ProductFamilies[0]
	if family["default_bom_spec_id"] != int64(801) {
		t.Fatalf("default_bom_spec_id=%#v", family["default_bom_spec_id"])
	}
	rows, ok := family["specs"].([]map[string]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("specs=%#v", family["specs"])
	}
	if rows[0]["spec_label"] != "227g 袋装" || rows[0]["is_default"] != true || rows[1]["spec_label"] != "454g 袋装" {
		t.Fatalf("ordered named specs=%#v", rows)
	}
}

func TestMiniOrderPriceTablePreviewReturnsReadOnlyPublishedTierRows(t *testing.T) {
	maxSeven := float64(7)
	products := []salesapp.ProductOption{{
		ID: 91, ParentProductID: 91, Name: "小菠萝", ParentProductName: "小菠萝",
		Tiers: []salesapp.ProductTierOption{
			{PublicationID: 31, BomSpecID: 801, BomVariantID: 901, SalesUnit: "袋", MinQty: 2, MaxQty: &maxSeven, UnitPrice: 31},
			{PublicationID: 31, BomSpecID: 801, BomVariantID: 901, SalesUnit: "袋", MinQty: 8, UnitPrice: 27},
		},
	}}
	specs := []salesapp.ProductBOMSpecOption{{ParentProductID: 91, BomSpecID: 801, BomVariantID: 901, SpecName: "227g 袋装", InventoryUnit: "袋", Published: true, IsDefault: true}}
	preview := miniOrderPriceTablePreviewFromSales(
		MiniOrderPriceTablePreviewQuery{CustomerID: 7, UsageCode: "direct_ship"},
		[]MiniDirectShipPriceTable{{ID: 31, TableName: "客户代发表", VersionNo: "V3.0.43"}},
		products,
		specs,
	)
	if preview.CurrentCustomerID != 7 || preview.UsageCode != "direct_ship" || len(preview.Rows) != 2 {
		t.Fatalf("preview=%#v", preview)
	}
	if preview.Rows[0].ProductName != "小菠萝" || preview.Rows[0].SpecName != "227g 袋装" || preview.Rows[0].MinQty != 2 || preview.Rows[0].MaxQty == nil || *preview.Rows[0].MaxQty != 7 || preview.Rows[1].UnitPrice != 27 {
		t.Fatalf("rows=%#v", preview.Rows)
	}
}

func TestNormalizeMiniDirectShipCommandValidatesAndKeepsOrderDate(t *testing.T) {
	base := MiniDirectShipCommand{
		CustomerID: 7, OrderDate: " 2026-09-15 ", RecipientName: "张三", RecipientPhone: "13800138000",
		DetailAddress: "咖啡路 88 号", Items: []MiniDirectShipItemCommand{{ProductID: 91, BomSpecID: 801, Qty: 1}},
	}
	got, err := normalizeMiniDirectShipCommand(base, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderDate != "2026-09-15" {
		t.Fatalf("order_date=%q", got.OrderDate)
	}
	base.OrderDate = "2026-02-30"
	if _, err := normalizeMiniDirectShipCommand(base, false); err == nil {
		t.Fatal("invalid order_date was accepted")
	}
}
