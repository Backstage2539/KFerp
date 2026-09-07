package customerportal

import salesapp "orderapp/internal/application/sales"

func miniEmployeeSelectPublications(form salesapp.OrderFormData, cid int64, ids []int64) (salesapp.OrderFormData, error) {
	selected, err := salesapp.ResolveOrderPriceTableSelection(form.BeanListVersionOptions, cid, ids, true)
	if err != nil {
		return form, err
	}
	form.BeanListVersionOptions = selected
	return miniEmployeeCatalogWithSelectedPublicPrices(form, cid), nil
}

// Public quotation selection preserves customer reference names and product
// identity while making only the selected public tiers available.
func miniEmployeeCatalogWithSelectedPublicPrices(form salesapp.OrderFormData, cid int64) salesapp.OrderFormData {
	public := map[int64]bool{}
	for _, v := range form.BeanListVersionOptions {
		if v.IsDefault && !v.IsCustomerOwned {
			public[v.ID] = true
		}
	}
	if len(public) > 0 {
		products := append([]salesapp.ProductOption{}, form.Products...)
		for i, p := range products {
			if p.CustomerID != cid || (p.Visibility != "customer_reference" && p.Visibility != "customer_alias" && p.CustomerProductAliasID <= 0) {
				continue
			}
			tiers := append([]salesapp.ProductTierOption{}, p.Tiers...)
			for _, base := range form.Products {
				if base.CustomerID != 0 || base.ID != p.ID {
					continue
				}
				for _, t := range base.Tiers {
					if public[t.PublicationID] {
						tiers = append(tiers, t)
					}
				}
			}
			products[i].Tiers = tiers
		}
		form.Products = products
		form.CustomerPublicUsages = []salesapp.CustomerPublicUsageOption{{CustomerID: cid, UsePublicSKU: true}}
	}
	return form
}
