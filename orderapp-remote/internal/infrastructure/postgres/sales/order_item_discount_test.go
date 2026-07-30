package sales

import (
	"testing"

	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func TestApplyOrderItemDiscountSupportsUnitAmount(t *testing.T) {
	discount, lineTotal := applyOrderItemDiscount(176, "unit_amount", 10, 2)

	if discount != 20 || lineTotal != 156 {
		t.Fatalf("unit amount discount = %.2f line total = %.2f, want 20.00 and 156.00", discount, lineTotal)
	}
}

func TestOrderItemUnitDiscountUnitsUsesCurrentPriceUnit(t *testing.T) {
	tests := []struct {
		name        string
		item        orderDiscountItem
		retailOrder bool
		want        float64
	}{
		{
			name:        "wholesale 454g uses lb basis",
			item:        orderDiscountItem{productKind: "roasted_bean", specG: 454, units: 2},
			retailOrder: false,
			want:        2,
		},
		{
			name:        "wholesale 1000g uses kg basis",
			item:        orderDiscountItem{productKind: "roasted_bean", specG: 1000, units: 30},
			retailOrder: false,
			want:        30,
		},
		{
			name:        "retail uses package count basis",
			item:        orderDiscountItem{productKind: "roasted_bean", specG: 227, units: 3},
			retailOrder: true,
			want:        3,
		},
		{
			name: "published sales spec count ignores wholesale weight basis",
			item: orderDiscountItem{
				productKind: "roasted_bean", quantityBasis: "sales_spec_count", specG: 227, units: 2,
			},
			retailOrder: false,
			want:        2,
		},
		{
			name:        "drip uses sales unit quantity",
			item:        orderDiscountItem{productKind: "drip_bag", salesUnit: "box", specG: 100, units: 4},
			retailOrder: false,
			want:        4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderItemUnitDiscountUnits(tc.item, tc.retailOrder); got != tc.want {
				t.Fatalf("orderItemUnitDiscountUnits() = %.3f, want %.3f", got, tc.want)
			}
		})
	}
}

func TestPublishedSalesSpecCountPriceAndUnitDiscountUseSameQuantityBasis(t *testing.T) {
	pricing := orderbeans.PublishedPricing{UnitPrice: 68, UnitG: 1000, QuantityBasis: "sales_spec_count"}
	base := publishedPricingLineTotal(pricing, 227, 2)
	discountUnits := orderItemUnitDiscountUnits(orderDiscountItem{
		productKind: "roasted_bean", quantityBasis: pricing.QuantityBasis, specG: 227, units: 2,
	}, false)
	discount, lineTotal := applyOrderItemDiscount(base, "unit_amount", 10, discountUnits)
	if base != 136 || discount != 20 || lineTotal != 116 {
		t.Fatalf("sales-spec-count amount chain = base %.2f discount %.2f total %.2f, want 136/20/116", base, discount, lineTotal)
	}
}

func TestManualSalesSpecCountPriceKeepsPackageQuantityBasis(t *testing.T) {
	item := orderDiscountItem{
		productKind: "roasted_bean", quantityBasis: "sales_spec_count", specG: 227, units: 2,
	}
	if got := orderManualPriceLineTotal(68, item, false); got != 136 {
		t.Fatalf("manual sales-spec-count line total = %.2f, want 136", got)
	}
	item.quantityBasis = ""
	if got := orderManualPriceLineTotal(68, item, false); got != 68 {
		t.Fatalf("legacy manual weight line total = %.2f, want 68", got)
	}
}
