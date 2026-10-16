package customerportal

import (
	sales "orderapp/internal/application/sales"
	"testing"
)

func TestMiniSampleOrderDetailPreservesZeroAndServerMarker(t *testing.T) {
	for _, sample := range []bool{false, true} {
		d := miniEmployeeOrderDetail(sales.OrderRow{ID: 42}, sales.OrderFormData{EditData: &sales.OrderEditData{ID: 42, IsSampleOrder: sample, Items: []sales.OrderEditItem{{ItemID: 1, BomSpecID: 7, PriceOverride: true, UnitPrice: "0.00", Qty: "2"}}}})
		if d.IsSampleOrder != sample || len(d.Items) != 1 || !d.Items[0].PriceOverride || d.Items[0].UnitPrice != "0.00" {
			t.Fatalf("sample detail altered: %+v", d)
		}
	}
}

func sampleEditCatalogFixture() (sales.OrderFormData, sales.SaveOrderCommand) {
	parentID, price := int64(10), 0.0
	form := sales.OrderFormData{Customers: []sales.CustomerOption{{ID: 8}}, EditData: &sales.OrderEditData{ID: 42, CustomerID: 8, IsSampleOrder: true, Items: []sales.OrderEditItem{{ItemID: 101, ProductID: 10, BomSpecID: 7, BomVariantID: 70, PriceOverride: true, UnitPrice: "0.00", Qty: "2", Unit: "袋", SalesUnit: "袋", ProductKind: "roasted_bean", BeanListPublicationID: 0, BeanListVersionNo: "", PriceSourceJSON: `{"source":"manual","price_selection_mode":"manual"}`}}}}
	cmd := sales.SaveOrderCommand{EditID: 42, CustomerID: 8, Items: []sales.OrderItemCommand{{ItemID: 101, ProductID: &parentID, ParentProductID: 10, BomSpecID: 7, BomVariantID: 70, ManualPrice: &price, PriceSelectionMode: "manual", Units: 3, Unit: "袋", BeanListPublicationID: 999, BeanListVersionNo: "forged", PriceSourceJSON: `{"source":"forged"}`}}}
	return form, cmd
}

func TestMiniSampleExistingManualSpecKeepsSnapshotWithoutOrdinaryQuote(t *testing.T) {
	form, cmd := sampleEditCatalogFixture()
	message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd)
	if err != nil || message != "" {
		t.Fatalf("historical sample rejected: %q %v", message, err)
	}
	item, stored := cmd.Items[0], form.EditData.Items[0]
	if item.BeanListPublicationID != stored.BeanListPublicationID || item.BeanListVersionNo != stored.BeanListVersionNo || item.PriceSourceJSON != stored.PriceSourceJSON {
		t.Fatalf("sample repriced or source altered: %+v", item)
	}
	if item.Units != 3 || item.ManualPrice == nil || *item.ManualPrice != 0 || item.BomSpecID != 7 || item.BomVariantID != 70 {
		t.Fatalf("sample edit lost quantities or identity: %+v", item)
	}
}

func TestMiniSampleHistoricalCatalogExceptionRequiresExactTrustedEdit(t *testing.T) {
	cases := []struct {
		name   string
		change func(*sales.OrderFormData, *sales.SaveOrderCommand)
	}{
		{"new_or_copy", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.EditID = 0 }},
		{"ordinary_order", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { f.EditData.IsSampleOrder = false }},
		{"other_order", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.EditID = 43 }},
		{"other_customer", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { f.EditData.CustomerID = 9 }},
		{"no_edit_snapshot", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { f.EditData = nil }},
		{"new_item", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].ItemID = 0 }},
		{"other_item", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].ItemID = 102 }},
		{"changed_product", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { p := int64(11); c.Items[0].ProductID = &p }},
		{"changed_parent", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].ParentProductID = 11 }},
		{"changed_spec", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].BomSpecID = 8 }},
		{"changed_variant", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].BomVariantID = 71 }},
		{"changed_alias", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].CustomerProductAliasID = 2 }},
		{"changed_reference", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { c.Items[0].CustomerProductReferenceID = 2 }},
		{"stored_auto_price", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) { f.EditData.Items[0].PriceOverride = false }},
		{"requested_auto_price", func(f *sales.OrderFormData, c *sales.SaveOrderCommand) {
			c.Items[0].ManualPrice = nil
			c.Items[0].PriceSelectionMode = "auto"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, c := sampleEditCatalogFixture()
			tc.change(&f, &c)
			message, err := miniEmployeePrepareCurrentCatalogFromForm(f, &c)
			if err == nil && message == "" {
				t.Fatal("untrusted or changed line bypassed current catalog")
			}
		})
	}
}

func TestMiniSampleMixedEditStillValidatesNewItemsAgainstCurrentCatalog(t *testing.T) {
	form, cmd := sampleEditCatalogFixture()
	added := cmd.Items[0]
	added.ItemID = 0
	added.BomSpecID = 8
	cmd.Items = append(cmd.Items, added)
	message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd)
	if err == nil && message == "" {
		t.Fatal("new sample line without current quote accepted")
	}
}

func TestMiniSampleMixedEditNormalizesNewQuotedSpec(t *testing.T) {
	form, cmd := sampleEditCatalogFixture()
	parent := int64(20)
	form.Products = []sales.ProductOption{{ID: 20, ParentProductID: 20, CustomerID: 8, Name: "新增商品", ProductKind: "roasted_bean"}}
	form.BeanListVersionOptions = []sales.BeanListVersionOption{{ID: 901, CustomerID: 8, ListType: "commercial", VersionNo: "V9.1", IsDefault: true}}
	form.ProductBOMSpecOptions = []sales.ProductBOMSpecOption{{ParentProductID: 20, OwnerCustomerID: 8, BomID: 200, BomVersionID: 2000, BomSpecID: 201, BomVariantID: 2001, Published: true, IsDefault: true, MigrationState: "cutover", WriteProductID: 20, WriteBomSpecID: 201, WriteBomVariantID: 2001, InventoryUnit: "袋", SpecCode: "BOM-SPEC-201", Tiers: []sales.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"source":"price_list"}`}}}}
	cmd.Items = append(cmd.Items, sales.OrderItemCommand{ProductID: &parent, ParentProductID: 20, BomSpecID: 201, BomVariantID: 2001, Units: 2, BeanListPublicationID: 901, BeanListVersionNo: "V9.1"})
	message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd)
	if err != nil || message != "" {
		t.Fatalf("mixed valid edit rejected: %q %v", message, err)
	}
	if cmd.Items[0].BeanListPublicationID != 0 || cmd.Items[0].PriceSourceJSON != form.EditData.Items[0].PriceSourceJSON {
		t.Fatal("stored sample snapshot was replaced")
	}
	if cmd.Items[1].BeanListPublicationID != 901 || cmd.Items[1].Unit != "袋" || cmd.Items[1].ProductCodeSnapshot != "BOM-SPEC-201" || cmd.Items[1].PriceSourceJSON == "" {
		t.Fatalf("new item skipped normal catalog: %+v", cmd.Items[1])
	}
}

func TestMiniSampleStoredLineCannotBeReusedForAnotherUnquotedLine(t *testing.T) {
	form, cmd := sampleEditCatalogFixture()
	cmd.Items = append(cmd.Items, cmd.Items[0])
	message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd)
	if err == nil && message == "" {
		t.Fatal("duplicate saved item identity bypassed normal catalog")
	}
}
