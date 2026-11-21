package customerportal

import salesapp "orderapp/internal/application/sales"

func miniEmployeeSelectPublications(form salesapp.OrderFormData, cid int64, ids []int64) (salesapp.OrderFormData, error) {
	selected, err := salesapp.ResolveOrderPriceTableSelection(form.BeanListVersionOptions, cid, ids, true)
	if err != nil {
		return form, err
	}
	form.BeanListVersionOptions = selected
	return salesapp.OrderFormWithSelectedPublicPrices(form, cid), nil
}
func miniEmployeeCatalogWithSelectedPublicPrices(form salesapp.OrderFormData, cid int64) salesapp.OrderFormData {
	return salesapp.OrderFormWithSelectedPublicPrices(form, cid)
}

func preferredCurrentOrderTables(options []salesapp.BeanListVersionOption, preferred []int64) []int64 {
	byID := make(map[int64]string, len(options))
	for _, opt := range options {
		byID[opt.ID] = salesapp.OrderPriceTableTypeKey(opt)
	}
	var selected []int64
	seen := map[string]bool{}
	for _, id := range preferred {
		if key, ok := byID[id]; ok && !seen[key] {
			selected = append(selected, id)
			seen[key] = true
		}
	}
	return selected
}
