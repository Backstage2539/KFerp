package customerportal

import salesapp "orderapp/internal/application/sales"

type miniEmployeeOrderCatalogResult = salesapp.OrderCatalog

func miniEmployeeOrderCatalog(form salesapp.OrderFormData, customerID int64, _ bool) miniEmployeeOrderCatalogResult {
	return salesapp.BuildOrderCatalog(form, customerID)
}
func miniEmployeeBOMSpecFilterProducts(products []salesapp.ProductOption, options []salesapp.ProductBOMSpecOption) ([]salesapp.ProductOption, map[int64]salesapp.ProductBOMSpecOption) {
	return salesapp.OrderBOMSpecProducts(products, options)
}
func miniEmployeeTierMatchesBOMSpec(tier salesapp.ProductTierOption, option salesapp.ProductBOMSpecOption) bool {
	return salesapp.OrderTierMatchesBOMSpec(tier, option)
}

func miniEmployeeBOMSpecFamilies(products []salesapp.ProductOption, options map[int64]salesapp.ProductBOMSpecOption) ([]map[string]any, []salesapp.ProductBOMSpecOption) {
	return salesapp.OrderBOMSpecFamilies(products, options)
}
