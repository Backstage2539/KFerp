package orderbeans

import "encoding/json"

// PublishedFixedBOMSpecPricing applies the direct-ship requirement of one
// unbounded immutable price per stable specification. Current manufacturing
// variant authority is checked separately by the order writer.
func PublishedFixedBOMSpecPricing(raw []byte, productID, bomSpecID int64, listType string) (PublishedPricing, bool) {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return PublishedPricing{}, false
	}
	var flatRows []json.RawMessage
	_ = json.Unmarshal(root["price_rows"], &flatRows)
	tiers := []publishedPriceTier{}
	for _, row := range flatRows {
		var fields map[string]json.RawMessage
		if json.Unmarshal(row, &fields) != nil || publishedFlatPriceRowProductID(fields) != productID {
			continue
		}
		var tier publishedPriceTier
		if json.Unmarshal(row, &tier) == nil && fixedTierBOMSpecID(tier, fields) == bomSpecID {
			tiers = append(tiers, tier)
		}
	}
	if len(tiers) == 0 {
		var content publishedBeanListContent
		if json.Unmarshal(raw, &content) != nil {
			return PublishedPricing{}, false
		}
		for _, group := range content.Groups {
			for _, row := range group.Items {
				var fields map[string]json.RawMessage
				if json.Unmarshal(row, &fields) != nil || publishedBeanListItemProductID(fields) != productID {
					continue
				}
				for _, tier := range publishedItemTiers(row, listType) {
					if fixedTierBOMSpecID(tier, fields) == bomSpecID {
						tiers = append(tiers, tier)
					}
				}
			}
		}
	}
	if len(tiers) != 1 || tiers[0].MinQty > 1 || tiers[0].MaxQty != nil {
		return PublishedPricing{}, false
	}
	p := publishedTierPricing(tiers[0], 0)
	return p, p.UnitPrice > 0
}

// Published catalog generations froze specification identity on the tier,
// inside effective_sales_spec, or on the containing item. Explicit tier
// identity wins; container identity is only a fallback.
func fixedTierBOMSpecID(tier publishedPriceTier, container map[string]json.RawMessage) int64 {
	if tier.BomSpecID > 0 {
		return tier.BomSpecID
	}
	var frozen map[string]json.RawMessage
	if json.Unmarshal(tier.EffectiveSalesSpec, &frozen) == nil {
		if id := publishedJSONInt64Field(frozen, "bom_spec_id"); id > 0 {
			return id
		}
	}
	return publishedJSONInt64Field(container, "bom_spec_id")
}
