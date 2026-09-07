package customerportal

import (
	"fmt"
	salesapp "orderapp/internal/application/sales"
)

func miniEmployeePriceChoices(form salesapp.OrderFormData, cid int64) []salesapp.BeanListVersionOption {
	out := []salesapp.BeanListVersionOption{}
	seen := map[int64]bool{}
	for _, v := range form.BeanListVersionOptions {
		if v.ID > 0 && v.IsDefault && (v.CustomerID == cid || v.CustomerID == 0 && !v.IsCustomerOwned) && !seen[v.ID] {
			out = append(out, v)
			seen[v.ID] = true
		}
	}
	return out
}

// Explicit employee selection can use a public publication while retaining
// customer identity. It never grants access to another customer's publication.
func miniEmployeeSelectPublications(form salesapp.OrderFormData, cid int64, ids []int64) (salesapp.OrderFormData, error) {
	if len(ids) == 0 {
		return form, nil
	}
	allowed := map[int64]salesapp.BeanListVersionOption{}
	for _, v := range miniEmployeePriceChoices(form, cid) {
		allowed[v.ID] = v
	}
	options := []salesapp.BeanListVersionOption{}
	public := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		v, ok := allowed[id]
		if !ok {
			return form, fmt.Errorf("所选价格表不在该客户当前价格表范围，可能未发布、已过期或无权限，请重新选择")
		}
		v.CustomerID = cid
		v.IsDefault = true
		options = append(options, v)
		if !v.IsCustomerOwned {
			public[id] = true
		}
	}
	if len(options) == 0 {
		return form, nil
	}
	form.BeanListVersionOptions = options
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
	return form, nil
}
