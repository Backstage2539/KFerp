package catalog

import (
	"encoding/json"
	"html/template"
	support "orderapp/internal/interfaces/http/support"
)

type jsTier struct {
	ID        int64    `json:"id"`
	SpecG     int64    `json:"spec_g"`
	Min       float64  `json:"min"`
	Max       *float64 `json:"max"`
	UnitPrice float64  `json:"price"`
}

type jsProduct struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Py              string   `json:"py"`
	Pyi             string   `json:"pyi"`
	ProductKind     string   `json:"product_kind"`
	SalesUnits      []string `json:"sales_units"`
	DripBagGrams    float64  `json:"drip_bag_grams"`
	DripBoxBagCount int      `json:"drip_box_bag_count"`
	RetailPrice100G float64  `json:"retail_price_100g"`
	RetailPrice200G float64  `json:"retail_price_200g"`
	RetailPrice227G float64  `json:"retail_price_227g"`
	RetailPrice250G float64  `json:"retail_price_250g"`
	RetailSpecs     []int64  `json:"retail_specs"`
	Tiers           []jsTier `json:"tiers"`
}

func buildProductsJSON(ps []ProductOption) template.JS {
	out := make([]jsProduct, 0, len(ps))
	for _, p := range ps {
		jp := jsProduct{ID: p.ID, Name: p.Name, Py: support.PinyinFull(p.Name), Pyi: support.PinyinInitials(p.Name), ProductKind: p.ProductKind, SalesUnits: p.SalesUnits, DripBagGrams: p.DripBagGrams, DripBoxBagCount: p.DripBoxBagCount, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G, RetailSpecs: p.RetailSpecs}
		for _, t := range p.Tiers {
			jp.Tiers = append(jp.Tiers, jsTier{ID: t.ID, SpecG: t.SpecG, Min: t.MinQty, Max: t.MaxQty, UnitPrice: t.UnitPrice})
		}
		out = append(out, jp)
	}
	b, _ := json.Marshal(out)
	return template.JS(b)
}
