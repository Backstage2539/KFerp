package sales

import "testing"

func TestFilterOrderProductsForDefaultPublicationsIsStrictAndTrimsHistoricalTiers(t *testing.T) {
	products := []ProductOption{
		{ID: 551, CustomerID: 0, Visibility: "public", ProductKind: "roasted_bean", Tiers: []ProductTierOption{
			{ID: 11, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`},
			{ID: 10, PublicationID: 900, PublicationVersionNo: "V9.0", ListType: "commercial", PriceSourceJSON: `{"publication_id":900,"list_type":"commercial"}`},
		}},
		{ID: 888, CustomerID: 0, Visibility: "public", ProductKind: "roasted_bean", Tiers: []ProductTierOption{{ID: 20, PublicationID: 900, ListType: "commercial", PriceSourceJSON: `{"publication_id":900,"list_type":"commercial"}`}}},
		{ID: 777, CustomerID: 0, Visibility: "public", ProductKind: "green_bean", Tiers: []ProductTierOption{{ID: 30, PublicationID: 300, ListType: "green", PriceSourceJSON: `{"publication_id":300,"list_type":"green"}`}}},
	}
	options := []BeanListVersionOption{
		{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true},
		{CustomerID: 8, ListType: "commercial", ID: 900, IsDefault: false},
	}
	got := FilterOrderProductsForDefaultPublications(products, 8, options, []CustomerPublicUsageOption{{CustomerID: 8, UsePublicSKU: true}})
	if len(got) != 1 || got[0].ID != 551 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 901 || got[0].Tiers[0].PublicationVersionNo != "V9.1" || got[0].Tiers[0].ListType != "commercial" {
		t.Fatalf("filtered products=%+v", got)
	}
	if len(products[0].Tiers) != 2 {
		t.Fatalf("input products mutated: %+v", products[0].Tiers)
	}
}

func TestFilterOrderProductsForDefaultPublicationsUsesRetailPriceTableForRetailOrders(t *testing.T) {
	products := []ProductOption{{
		ID: 551, Visibility: "public", ProductKind: "roasted_bean",
		Tiers: []ProductTierOption{
			{ID: 11, PublicationID: 901, ListType: "commercial", PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`},
			{ID: 12, PublicationID: 902, ListType: "retail", PriceSourceJSON: `{"publication_id":902,"list_type":"retail"}`},
		},
	}}
	options := []BeanListVersionOption{
		{CustomerID: 8, ListType: "commercial", ID: 901, IsDefault: true},
		{CustomerID: 8, ListType: "retail", ID: 902, IsDefault: true},
	}
	got := FilterOrderProductsForDefaultPublications(products, 8, options, nil, true)
	if len(got) != 1 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 902 {
		t.Fatalf("retail products=%+v", got)
	}
}

func TestFilterOrderProductsForDefaultPublicationsPrefersGeneralDripSKUPriceTables(t *testing.T) {
	products := []ProductOption{{
		ID: 711, Visibility: "public", ProductKind: "drip_bag",
		Tiers: []ProductTierOption{
			{ID: 11, PublicationID: 901, ListType: "commercial", UnitPrice: 3.2, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`},
			{ID: 12, PublicationID: 902, ListType: "retail", UnitPrice: 3.8, PriceSourceJSON: `{"publication_id":902,"list_type":"retail"}`},
			{ID: 13, PublicationID: 903, ListType: "drip", UnitPrice: 2.9, PriceSourceJSON: `{"publication_id":903,"list_type":"drip"}`},
		},
	}}
	options := []BeanListVersionOption{
		{CustomerID: 8, ListType: "commercial", ID: 901, IsDefault: true},
		{CustomerID: 8, ListType: "retail", ID: 902, IsDefault: true},
		{CustomerID: 8, ListType: "drip", ID: 903, IsDefault: true},
	}
	commercial := FilterOrderProductsForDefaultPublications(products, 8, options, nil)
	if len(commercial) != 1 || len(commercial[0].Tiers) != 1 || commercial[0].Tiers[0].PublicationID != 901 || commercial[0].Tiers[0].ListType != "commercial" {
		t.Fatalf("commercial drip SKU catalog=%+v", commercial)
	}
	retail := FilterOrderProductsForDefaultPublications(products, 8, options, nil, true)
	if len(retail) != 1 || len(retail[0].Tiers) != 1 || retail[0].Tiers[0].PublicationID != 902 || retail[0].Tiers[0].ListType != "retail" {
		t.Fatalf("retail drip SKU catalog=%+v", retail)
	}
}

func TestFilterOrderProductsForDefaultPublicationsFallsBackToHistoricalDripOnlyWithoutGeneralTier(t *testing.T) {
	products := []ProductOption{{
		ID: 711, Visibility: "public", ProductKind: "drip_bag",
		Tiers: []ProductTierOption{{ID: 13, PublicationID: 903, ListType: "drip", UnitPrice: 2.9, PriceSourceJSON: `{"publication_id":903,"list_type":"drip"}`}},
	}}
	options := []BeanListVersionOption{
		{CustomerID: 8, ListType: "commercial", ID: 901, IsDefault: true},
		{CustomerID: 8, ListType: "drip", ID: 903, IsDefault: true},
	}
	got := FilterOrderProductsForDefaultPublications(products, 8, options, nil)
	if len(got) != 1 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 903 || got[0].Tiers[0].ListType != "drip" {
		t.Fatalf("historical drip fallback=%+v", got)
	}
}

func TestOrderEditabilityBlocksEveryDownstreamState(t *testing.T) {
	tests := []struct {
		name  string
		state OrderEditState
		want  string
	}{
		{name: "void", state: OrderEditState{IsVoid: true}, want: "订单已作废，不能再编辑"},
		{name: "shipped status", state: OrderEditState{ShipStatus: "已发货"}, want: "订单已发货，不能再编辑"},
		{name: "shipment", state: OrderEditState{HasShipment: true}, want: "订单已进入发货流程，不能再编辑"},
		{name: "stock deduction", state: OrderEditState{HasStockDeduction: true}, want: "订单已扣减库存，不能再编辑"},
		{name: "work order", state: OrderEditState{HasWorkOrder: true}, want: "订单已进入生产流程，不能再编辑"},
		{name: "legacy produce batch", state: OrderEditState{HasProduceBatch: true}, want: "订单已进入旧版生产批次，不能再编辑"},
		{name: "plan", state: OrderEditState{HasProductionPlan: true}, want: "订单已进入生产计划，不能再编辑"},
		{name: "process status", state: OrderEditState{ProcessStatus: "生产中"}, want: "订单已进入生产流程，不能再编辑"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateOrderEditability(tc.state)
			if got.CanEdit || got.BlockReason != tc.want {
				t.Fatalf("editability=%+v want=%q", got, tc.want)
			}
		})
	}
	for _, status := range []string{"", "待处理", "待生产"} {
		got := EvaluateOrderEditability(OrderEditState{ProcessStatus: status})
		if !got.CanEdit || got.BlockReason != "" {
			t.Fatalf("status=%q editability=%+v", status, got)
		}
	}
}

func TestOrderEditConflictCarriesSafeMessage(t *testing.T) {
	err := NewOrderEditConflictError("订单已进入生产计划，不能再编辑")
	message, ok := OrderEditConflictMessage(err)
	if !ok || message != "订单已进入生产计划，不能再编辑" {
		t.Fatalf("message=%q ok=%v err=%v", message, ok, err)
	}
}
