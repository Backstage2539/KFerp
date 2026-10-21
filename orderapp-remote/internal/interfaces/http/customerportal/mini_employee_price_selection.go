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
