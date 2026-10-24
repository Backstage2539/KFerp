package costing

import (
	"context"
	"testing"
)

func TestDefaultSpecHistoryIsNotASelectionRequirement(t *testing.T) {
	repo := &fakeRepo{productSpecIdentities: map[int64]ProductSpecIdentity{
		550: {ProductID: 550, EffectiveParentProductID: 550, Active: true, SpecValid: true},
		551: {ProductID: 551, EffectiveParentProductID: 550, Active: true, SpecValid: true},
		561: {ProductID: 561, EffectiveParentProductID: 560, Active: true, SpecValid: true},
	}}
	for _, historicalDefault := range []int64{0, 551, 561, 999} {
		cmd := PublishBeanListCommand{
			Config: map[string]any{"product_spec_selections": []any{map[string]any{
				"parent_product_id": float64(550), "sku_id": float64(551), "selection_source": "explicit", "default_sku_id_at_selection": float64(historicalDefault),
			}}},
			Content: map[string]any{"price_rows": []any{map[string]any{"parent_product_id": float64(550), "sku_id": float64(551), "final_unit_price": float64(82)}}},
		}
		if err := NewService(repo).validateProductSpecSelections(context.Background(), &cmd); err != nil {
			t.Fatalf("valid selected spec rejected for historical default %d: %v", historicalDefault, err)
		}
	}
}
