package costing

import (
	"context"
	"fmt"
	"strings"
)

type SpecPricingGroupResolvedItem struct {
	PricingRuleFingerprint string

	BomSpecID     int64
	BomVariantID  int64
	PricingRuleID int64
	SpecKey       string
}
type SpecPricingGroupResolved struct {
	ID       int64
	Revision int64
	Active   bool
	Items    []SpecPricingGroupResolvedItem
}
type specPricingGroupResolver interface {
	ResolveSpecPricingGroupForProduct(context.Context, int64, int64) (SpecPricingGroupResolved, error)
}

func effectiveSpecPricingGroup(config map[string]any, parentID, groupID int64) (int64, error) {
	resolve := func(v map[string]any) (int64, bool, error) {
		mode := strings.TrimSpace(stringValue(v["pricing_mode"]))
		if mode == "" {
			return 0, false, nil
		}
		if mode != "spec_pricing_group" {
			return 0, true, nil
		}
		id := int64(numberValue(v["spec_pricing_group_id"]))
		if id <= 0 {
			return 0, true, fmt.Errorf("规格计价组未配置")
		}
		return id, true, nil
	}
	for _, raw := range beanListAnySlice(config["product_overrides"]) {
		v, _ := objectSnapshotMap(raw)
		if int64(numberValue(v["parent_product_id"])) == parentID && strings.TrimSpace(stringValue(v["scope"])) == "parent_product" {
			id, ok, err := resolve(v)
			if ok || err != nil {
				return id, err
			}
		}
	}
	groups := map[int64]map[string]any{}
	for _, raw := range beanListAnySlice(config["group_selections"]) {
		v, _ := objectSnapshotMap(raw)
		id := int64(numberValue(v["group_item_id"]))
		if id <= 0 {
			continue
		}
		previous := groups[id]
		if previous == nil {
			previous = map[string]any{}
			groups[id] = previous
		}
		if numberValue(previous["parent_group_item_id"]) <= 0 && numberValue(v["parent_group_item_id"]) > 0 {
			previous["parent_group_item_id"] = v["parent_group_item_id"]
		}
		if stringValue(previous["pricing_mode"]) == "" && stringValue(v["pricing_mode"]) != "" {
			previous["pricing_mode"] = v["pricing_mode"]
		}
		if numberValue(previous["spec_pricing_group_id"]) <= 0 && numberValue(v["spec_pricing_group_id"]) > 0 {
			previous["spec_pricing_group_id"] = v["spec_pricing_group_id"]
		}
	}
	seen := map[int64]bool{}
	for groupID > 0 {
		if seen[groupID] {
			return 0, fmt.Errorf("商品分类计价继承存在循环")
		}
		seen[groupID] = true
		v, ok := groups[groupID]
		if !ok {
			break
		}
		id, found, err := resolve(v)
		if found || err != nil {
			return id, err
		}
		groupID = int64(numberValue(v["parent_group_item_id"]))
	}
	defaults, _ := objectSnapshotMap(config["defaults"])
	id, _, err := resolve(defaults)
	return id, err
}
func (s *Service) ValidateSpecPricingGroups(ctx context.Context, cmd *PublishBeanListCommand) error {
	if cmd == nil || cmd.Config == nil {
		return nil
	}
	config, ok := objectSnapshotMap(cmd.Config["price_list_template_selection"])
	if !ok {
		return nil
	}
	if stringValue(config["product_pricing_scope"]) != "spec_pricing_group_aware" {
		defaults, _ := objectSnapshotMap(config["defaults"])
		if stringValue(defaults["pricing_mode"]) == "spec_pricing_group" || numberValue(defaults["spec_pricing_group_id"]) > 0 {
			return fmt.Errorf("规格计价组缺少发布配置标识")
		}
		for _, k := range []string{"group_selections", "product_overrides"} {
			for _, raw := range beanListAnySlice(config[k]) {
				v, _ := objectSnapshotMap(raw)
				if stringValue(v["pricing_mode"]) == "spec_pricing_group" || numberValue(v["spec_pricing_group_id"]) > 0 {
					return fmt.Errorf("规格计价组缺少发布配置标识")
				}
			}
		}
		for _, raw := range beanListAnySlice(cmd.Content["price_rows"]) {
			v, _ := objectSnapshotMap(raw)
			if numberValue(v["spec_pricing_group_id"]) > 0 {
				return fmt.Errorf("规格计价组缺少发布配置标识")
			}
		}
		return nil
	}
	resolver, ok := s.repo.(specPricingGroupResolver)
	if !ok {
		return fmt.Errorf("规格计价组发布校验不可用")
	}
	type parentRows struct {
		groupID int64
		rows    []map[string]any
	}
	parents := map[int64]*parentRows{}
	var ordinaryRows []any
	for _, raw := range beanListAnySlice(cmd.Content["price_rows"]) {
		row, ok := objectSnapshotMap(raw)
		if !ok {
			return fmt.Errorf("规格价格行无效")
		}
		parent := int64(numberValue(row["parent_product_id"]))
		if parent <= 0 {
			parent = int64(numberValue(row["product_id"]))
		}
		if parent <= 0 {
			return fmt.Errorf("规格价格行缺少商品")
		}
		snap, _ := objectSnapshotMap(row["group_snapshot"])
		category := int64(numberValue(snap["group_item_id"]))
		groupID, err := effectiveSpecPricingGroup(config, parent, category)
		if err != nil {
			return err
		}
		declared := int64(numberValue(row["spec_pricing_group_id"]))
		if declared != groupID {
			return fmt.Errorf("商品 %d 的规格计价组与继承配置不一致", parent)
		}
		if groupID <= 0 {
			ordinaryRows = append(ordinaryRows, row)
			continue
		}
		p := parents[parent]
		if p == nil {
			p = &parentRows{groupID: groupID}
			parents[parent] = p
		}
		if p.groupID != groupID {
			return fmt.Errorf("商品 %d 的规格计价组不一致", parent)
		}
		p.rows = append(p.rows, row)
	}
	// Ordinary pricing in a mixed table must retain the existing shared-parent rule.
	if len(ordinaryRows) > 0 {
		selectionCopy := map[string]any{}
		for k, v := range config {
			selectionCopy[k] = v
		}
		selectionCopy["product_pricing_scope"] = "parent_product_shared"
		configCopy := map[string]any{"price_list_template_selection": selectionCopy}
		contentCopy := map[string]any{"price_rows": ordinaryRows}
		if err := validateSharedParentProductPricing(&PublishBeanListCommand{Config: configCopy, Content: contentCopy}); err != nil {
			return err
		}
	}
	for parent, p := range parents {
		g, err := resolver.ResolveSpecPricingGroupForProduct(ctx, p.groupID, parent)
		if err != nil {
			return err
		}
		if !g.Active || len(g.Items) == 0 {
			return fmt.Errorf("商品 %d 的规格计价组已失效或无规格", parent)
		}
		if len(p.rows) != len(g.Items) {
			return fmt.Errorf("商品 %d 的规格计价组规格数量不完整", parent)
		}
		bySpec := map[int64]SpecPricingGroupResolvedItem{}
		for _, i := range g.Items {
			if i.BomSpecID <= 0 || i.BomVariantID <= 0 || i.PricingRuleID <= 0 {
				return fmt.Errorf("商品 %d 缺少规格 %s 或价格模板失效", parent, i.SpecKey)
			}
			bySpec[i.BomSpecID] = i
		}
		seen := map[int64]bool{}
		for _, row := range p.rows {
			spec := int64(numberValue(row["bom_spec_id"]))
			item, ok := bySpec[spec]
			if !ok || seen[spec] || int64(numberValue(row["bom_variant_id"])) != item.BomVariantID || int64(numberValue(row["pricing_rule_id"])) != item.PricingRuleID || item.PricingRuleFingerprint == "" || stringValue(row["pricing_rule_fingerprint"]) != item.PricingRuleFingerprint || int64(numberValue(row["spec_pricing_group_revision"])) != g.Revision || stringValue(row["pricing_mode"]) != "pricing_rule" || numberValue(row["min_qty"]) > 1 || row["max_qty"] != nil {
				return fmt.Errorf("商品 %d 规格计价组或价格行已变化，请刷新重算", parent)
			}
			seen[spec] = true
		}
	}
	return nil
}
