package sales

import (
	"math"
	"testing"

	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func TestPublishedPricingLineTotalUsesSalesSpecCountWithoutWeightRecalculation(t *testing.T) {
	countPricing := orderbeans.PublishedPricing{
		UnitPrice: 68, UnitG: 1000, QuantityBasis: "sales_spec_count",
	}
	if got := publishedPricingLineTotal(countPricing, 454, 2); got != 136 {
		t.Fatalf("sales-spec-count line total = %v, want 68*2=136", got)
	}

	legacyPricing := orderbeans.PublishedPricing{UnitPrice: 68, UnitG: 1000}
	wantLegacy := 68 * 908.0 / 1000.0
	if got := publishedPricingLineTotal(legacyPricing, 454, 2); math.Abs(got-wantLegacy) > 1e-9 {
		t.Fatalf("legacy weight line total = %v, want %v", got, wantLegacy)
	}
}

func TestOrderItemBeanListPublicationIDPrefersEachLineSelection(t *testing.T) {
	cmd := salesapp.SaveOrderCommand{
		BeanListPublicationID:           900,
		CommercialBeanListPublicationID: 901,
		GreenBeanListPublicationID:      902,
	}
	tests := []struct {
		name              string
		itemPublicationID int64
		listType          string
		want              int64
	}{
		{name: "first retail category line", itemPublicationID: 1101, listType: orderbeans.ListTypeRetail, want: 1101},
		{name: "second retail category line", itemPublicationID: 1201, listType: orderbeans.ListTypeRetail, want: 1201},
		{name: "commercial line overrides commercial header", itemPublicationID: 2101, listType: orderbeans.ListTypeCommercial, want: 2101},
		{name: "green line overrides green header", itemPublicationID: 3101, listType: orderbeans.ListTypeGreen, want: 3101},
		{name: "retail falls back to header", listType: orderbeans.ListTypeRetail, want: 900},
		{name: "commercial falls back to header", listType: orderbeans.ListTypeCommercial, want: 901},
		{name: "green falls back to header", listType: orderbeans.ListTypeGreen, want: 902},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderItemBeanListPublicationID(cmd, tc.itemPublicationID, tc.listType); got != tc.want {
				t.Fatalf("orderItemBeanListPublicationID() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestResolvedOrderHeaderPublicationIDUsesResolvedCommercialLines(t *testing.T) {
	tests := []struct {
		name      string
		requested int64
		refs      []orderHeaderPublicationRef
		wantID    int64
		ambiguous bool
	}{
		{
			name:      "one resolved commercial line overrides a stale submitted header",
			requested: 9950,
			refs: []orderHeaderPublicationRef{
				{publicationID: 9951, listType: orderbeans.ListTypeCommercial},
				{publicationID: 9961, listType: orderbeans.ListTypeGreen},
			},
			wantID: 9951,
		},
		{
			name:      "multiple commercial classifications leave the legacy header empty",
			requested: 9950,
			refs: []orderHeaderPublicationRef{
				{publicationID: 9951, listType: orderbeans.ListTypeCommercial},
				{publicationID: 9952, listType: orderbeans.ListTypeCommercial},
			},
			wantID:    0,
			ambiguous: true,
		},
		{
			name:      "no resolved commercial line preserves the legacy fallback request",
			requested: 9950,
			refs:      []orderHeaderPublicationRef{{publicationID: 9961, listType: orderbeans.ListTypeGreen}},
			wantID:    9950,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotAmbiguous := resolvedOrderHeaderPublicationID(tc.requested, tc.refs, orderbeans.ListTypeCommercial)
			if gotID != tc.wantID || gotAmbiguous != tc.ambiguous {
				t.Fatalf("resolvedOrderHeaderPublicationID() = %d/%t, want %d/%t", gotID, gotAmbiguous, tc.wantID, tc.ambiguous)
			}
		})
	}
}
