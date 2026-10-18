package customerfulfillment

import (
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"strings"
)

// ValidateDirectShipFixedPrices checks only the explicitly selected publication
// catalog. A direct-ship table must have one unbounded price per ordered spec.
func ValidateDirectShipFixedPrices(items []MiniDirectShipItemCommand, products []salesapp.ProductOption) error {
	for _, item := range items {
		count := 0
		for _, product := range products {
			if product.ID != item.ProductID && product.ParentProductID != item.ProductID {
				continue
			}
			for _, tier := range product.Tiers {
				specID, variantID := miniOrderTierBOMSpecIdentity(tier)
				if item.BomSpecID > 0 {
					if specID != item.BomSpecID || item.BomVariantID > 0 && variantID != item.BomVariantID {
						continue
					}
				} else if tier.SpecG != item.SpecG {
					continue
				}
				if item.SalesUnit != "" && tier.SalesUnit != "" && !strings.EqualFold(item.SalesUnit, tier.SalesUnit) {
					continue
				}
				if tier.MinQty > 1 || tier.MaxQty != nil || tier.UnitPrice <= 0 {
					return fmt.Errorf("商品「%s」的一件代发价格表须使用单一固定价，不设数量档位；请在商品价格表中调整", item.ProductName)
				}
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("商品「%s」的一件代发价格表须为当前规格设置唯一固定价", item.ProductName)
		}
	}
	return nil
}
