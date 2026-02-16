package main

import (
	"encoding/json"
	"html/template"
)

type jsTier struct {
	ID    int64    `json:"id"`
	Min   float64  `json:"min"`
	Max   *float64 `json:"max"`
	Price float64  `json:"price"`
}

type jsProduct struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Py    string   `json:"py"`
	Pyi   string   `json:"pyi"`
	Tiers []jsTier `json:"tiers"`
}

func buildProductsJSON(ps []ProductOption) template.JS {
	out := make([]jsProduct, 0, len(ps))
	for _, p := range ps {
		jp := jsProduct{ID: p.ID, Name: p.Name, Py: pinyinFull(p.Name), Pyi: pinyinInitials(p.Name)}
		for _, t := range p.Tiers {
			jp.Tiers = append(jp.Tiers, jsTier{ID: t.ID, Min: t.MinLb, Max: t.MaxLb, Price: t.PriceLb})
		}
		out = append(out, jp)
	}
	b, _ := json.Marshal(out)
	return template.JS(b)
}
