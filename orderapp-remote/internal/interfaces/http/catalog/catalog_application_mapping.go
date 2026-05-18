package catalog

import catalogapp "orderapp/internal/application/catalog"

func productOptionFromCatalog(p catalogapp.Product) ProductOption {
	out := ProductOption{
		ID:                      p.ID,
		Name:                    p.Name,
		ProductKind:             p.ProductKind,
		RoastLevel:              p.RoastLevel,
		DefaultPrice:            p.DefaultPrice,
		RetailPrice100G:         p.RetailPrice100G,
		RetailPrice200G:         p.RetailPrice200G,
		RetailPrice227G:         p.RetailPrice227G,
		RetailPrice250G:         p.RetailPrice250G,
		YieldRate:               p.YieldRate,
		ProductCategoryID:       p.ProductCategoryID,
		ProductCategoryPosition: p.ProductCategoryPosition,
		CustomerID:              p.CustomerID,
		BaseProductID:           p.BaseProductID,
		Visibility:              p.Visibility,
		CustomType:              p.CustomType,
		MarginRateOverride:      p.MarginRateOverride,
		BomItemCount:            p.BomItemCount,
		BomStatus:               p.BomStatus,
	}
	out.Tiers = make([]ProductTierOption, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, ProductTierOption{
			ID:        t.ID,
			SpecG:     t.SpecG,
			MinQty:    t.MinQty,
			MaxQty:    t.MaxQty,
			UnitPrice: t.UnitPrice,
		})
	}
	return out
}

func productOptionsFromCatalog(products []catalogapp.Product) []ProductOption {
	out := make([]ProductOption, 0, len(products))
	for _, p := range products {
		out = append(out, productOptionFromCatalog(p))
	}
	return out
}
