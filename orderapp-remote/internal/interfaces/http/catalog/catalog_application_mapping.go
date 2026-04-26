package catalog

import catalogapp "orderapp/internal/application/catalog"

func productOptionFromCatalog(p catalogapp.Product) ProductOption {
	out := ProductOption{ID: p.ID, Name: p.Name, RoastLevel: p.RoastLevel, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G}
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
