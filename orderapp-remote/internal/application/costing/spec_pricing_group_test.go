package costing

import (
	"context"
	"testing"
)

func TestResolveEffectiveSpecPricingGroup(t *testing.T) {
	config := map[string]any{"defaults": map[string]any{"pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(4)}, "group_selections": []any{map[string]any{"group_item_id": float64(12), "parent_group_item_id": float64(11), "pricing_mode": ""}, map[string]any{"group_item_id": float64(11), "pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(7)}}, "product_overrides": []any{map[string]any{"scope": "parent_product", "parent_product_id": float64(3), "pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(8)}}}
	if got, _ := effectiveSpecPricingGroup(config, 3, 12); got != 8 {
		t.Fatalf("product override got %d", got)
	}
	if got, _ := effectiveSpecPricingGroup(config, 9, 12); got != 7 {
		t.Fatalf("category inheritance got %d", got)
	}
	if got, _ := effectiveSpecPricingGroup(config, 9, 99); got != 4 {
		t.Fatalf("default got %d", got)
	}
}

type specGroupValidationRepo struct {
	*fakeRepo
	group SpecPricingGroupResolved
}

func (r *specGroupValidationRepo) ResolveSpecPricingGroupForProduct(_ context.Context, _, _ int64) (SpecPricingGroupResolved, error) {
	return r.group, nil
}
func TestSpecPricingGroupPublicationValidation(t *testing.T) {
	repo := &specGroupValidationRepo{fakeRepo: &fakeRepo{}, group: SpecPricingGroupResolved{ID: 4, Revision: 3, Active: true, Items: []SpecPricingGroupResolvedItem{{BomSpecID: 10, BomVariantID: 20, PricingRuleID: 30, SpecKey: "227g", PricingRuleFingerprint: "hash"}, {BomSpecID: 11, BomVariantID: 21, PricingRuleID: 31, SpecKey: "100g", PricingRuleFingerprint: "hash"}}}}
	svc := NewService(repo)
	mkRow := func(spec, variant, rule float64) map[string]any {
		return map[string]any{"parent_product_id": float64(1), "bom_spec_id": spec, "bom_variant_id": variant, "pricing_rule_id": rule, "pricing_rule_fingerprint": "hash", "spec_pricing_group_id": float64(4), "spec_pricing_group_revision": float64(3), "pricing_mode": "pricing_rule", "min_qty": float64(1)}
	}
	cmd := PublishBeanListCommand{Config: map[string]any{"price_list_template_selection": map[string]any{"product_pricing_scope": "spec_pricing_group_aware", "defaults": map[string]any{"pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(4)}}}, Content: map[string]any{"price_rows": []any{mkRow(10, 20, 30), mkRow(11, 21, 31)}}}
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err != nil {
		t.Fatal(err)
	}
	rows := cmd.Content["price_rows"].([]any)
	cmd.Content["price_rows"] = rows[:1]
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("missing group spec accepted")
	}
	cmd.Content["price_rows"] = rows
	row := rows[0].(map[string]any)
	row["spec_pricing_group_revision"] = float64(2)
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("stale revision accepted")
	}
	row["spec_pricing_group_revision"] = float64(3)
	row["spec_pricing_group_id"] = float64(0)
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("omitted group accepted")
	}
	row["spec_pricing_group_id"] = float64(4)
	row["pricing_rule_id"] = float64(99)
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("wrong pricing template accepted")
	}
}

func TestSpecPricingGroupRejectsMissingScopeAndPreservesOrdinarySharedPricing(t *testing.T) {
	svc := NewService(&specGroupValidationRepo{fakeRepo: &fakeRepo{}})
	cmd := PublishBeanListCommand{Config: map[string]any{"price_list_template_selection": map[string]any{"defaults": map[string]any{"pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(1)}}}, Content: map[string]any{}}
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("group without scope marker accepted")
	}
	config := cmd.Config["price_list_template_selection"].(map[string]any)
	config["product_pricing_scope"] = "spec_pricing_group_aware"
	config["defaults"] = map[string]any{"pricing_mode": "pricing_rule"}
	cmd.Content["price_rows"] = []any{map[string]any{"parent_product_id": float64(2), "pricing_mode": "pricing_rule", "pricing_rule_id": float64(4)}, map[string]any{"parent_product_id": float64(2), "pricing_mode": "pricing_rule", "pricing_rule_id": float64(5)}}
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("ordinary parent product price sharing was bypassed")
	}
}

func TestSpecPricingGroupRejectsChangedPricingRuleFingerprint(t *testing.T) {
	repo := &specGroupValidationRepo{fakeRepo: &fakeRepo{}, group: SpecPricingGroupResolved{ID: 4, Revision: 3, Active: true, Items: []SpecPricingGroupResolvedItem{{BomSpecID: 10, BomVariantID: 20, PricingRuleID: 30, PricingRuleFingerprint: "current"}}}}
	svc := NewService(repo)
	row := map[string]any{"parent_product_id": float64(1), "bom_spec_id": float64(10), "bom_variant_id": float64(20), "pricing_rule_id": float64(30), "pricing_rule_fingerprint": "old", "spec_pricing_group_id": float64(4), "spec_pricing_group_revision": float64(3), "pricing_mode": "pricing_rule"}
	cmd := PublishBeanListCommand{Config: map[string]any{"price_list_template_selection": map[string]any{"product_pricing_scope": "spec_pricing_group_aware", "defaults": map[string]any{"pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(4)}}}, Content: map[string]any{"price_rows": []any{row}}}
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err == nil {
		t.Fatal("changed formula was accepted")
	}
	row["pricing_rule_fingerprint"] = "current"
	if err := svc.ValidateSpecPricingGroups(context.Background(), &cmd); err != nil {
		t.Fatal(err)
	}
}
