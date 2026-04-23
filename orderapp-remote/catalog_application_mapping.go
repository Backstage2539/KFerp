package main

import catalogapp "orderapp/internal/application/catalog"

func productOptionFromCatalog(p catalogapp.Product) ProductOption {
	out := ProductOption{ID: p.ID, Name: p.Name, DefaultPrice: p.DefaultPrice}
	out.Tiers = make([]ProductTierOption, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, ProductTierOption{
			ID:      t.ID,
			MinLb:   t.MinLb,
			MaxLb:   t.MaxLb,
			PriceLb: t.PriceLb,
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
