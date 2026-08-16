package customerportal

import (
	"fmt"
	"sort"
	"strings"

	salesapp "orderapp/internal/application/sales"
	catalogdomain "orderapp/internal/domain/catalog"
)

type miniEmployeeOrderCatalogResult struct {
	Products       []salesapp.ProductOption
	Families       []map[string]any
	BOMSpecOptions []salesapp.ProductBOMSpecOption
}

// miniEmployeeOrderCatalog keeps the legacy SKU catalog untouched until a
// product cuts over. A cutover product is then exposed only as parent product
// plus BOM-owned specification identity; bom_spec_id is never copied into a
// product/SKU id field.
func miniEmployeeOrderCatalog(
	form salesapp.OrderFormData,
	customerID int64,
	retailOrder bool,
) miniEmployeeOrderCatalogResult {
	legacyProducts := salesapp.FilterOrderProductsForDefaultPublications(
		form.Products,
		customerID,
		form.BeanListVersionOptions,
		form.CustomerPublicUsages,
		retailOrder,
	)
	cutoverParents := map[int64]bool{}
	cutoverOptions := make([]salesapp.ProductBOMSpecOption, 0)
	for _, option := range form.ProductBOMSpecOptions {
		if option.ParentProductID <= 0 || option.BomSpecID <= 0 || option.MigrationState != "cutover" {
			continue
		}
		cutoverParents[option.ParentProductID] = true
		if option.Published {
			cutoverOptions = append(cutoverOptions, option)
		}
	}
	if len(cutoverParents) == 0 {
		return miniEmployeeOrderCatalogResult{
			Products: legacyProducts,
			Families: salesapp.BuildOrderProductFamilies(legacyProducts),
		}
	}

	visibleLegacy := make([]salesapp.ProductOption, 0, len(legacyProducts))
	for _, product := range legacyProducts {
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		if !cutoverParents[parentID] {
			visibleLegacy = append(visibleLegacy, product)
		}
	}

	canonicalProducts, optionBySpec := miniEmployeeBOMSpecFilterProducts(form.Products, cutoverOptions)
	canonicalProducts = salesapp.FilterOrderProductsForDefaultPublications(
		canonicalProducts,
		customerID,
		form.BeanListVersionOptions,
		form.CustomerPublicUsages,
		retailOrder,
	)
	families, visibleOptions := miniEmployeeBOMSpecFamilies(canonicalProducts, optionBySpec)
	return miniEmployeeOrderCatalogResult{
		Products:       visibleLegacy,
		Families:       append(salesapp.BuildOrderProductFamilies(visibleLegacy), families...),
		BOMSpecOptions: visibleOptions,
	}
}

func miniEmployeeBOMSpecFilterProducts(
	products []salesapp.ProductOption,
	options []salesapp.ProductBOMSpecOption,
) ([]salesapp.ProductOption, map[int64]salesapp.ProductBOMSpecOption) {
	optionBySpec := make(map[int64]salesapp.ProductBOMSpecOption, len(options))
	optionsByParent := map[int64][]salesapp.ProductBOMSpecOption{}
	for _, option := range options {
		optionBySpec[option.BomSpecID] = option
		optionsByParent[option.ParentProductID] = append(optionsByParent[option.ParentProductID], option)
	}
	sourcesByParent := map[int64][]salesapp.ProductOption{}
	seenSource := map[string]bool{}
	for _, product := range products {
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		if len(optionsByParent[parentID]) == 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d:%d", parentID, product.CustomerID, product.CustomerProductAliasID)
		if seenSource[key] {
			continue
		}
		seenSource[key] = true
		sourcesByParent[parentID] = append(sourcesByParent[parentID], product)
	}

	out := make([]salesapp.ProductOption, 0)
	for parentID, parentOptions := range optionsByParent {
		sources := sourcesByParent[parentID]
		if len(sources) == 0 {
			sources = []salesapp.ProductOption{{ID: parentID, ParentProductID: parentID, Name: fmt.Sprintf("商品%d", parentID), Visibility: "public"}}
		}
		for _, source := range sources {
			for _, option := range parentOptions {
				row := source
				row.ID = parentID
				// SKUID is an internal correlation marker consumed before the
				// response is built. It is never returned as the specification
				// business identity.
				row.SKUID = option.BomSpecID
				row.ParentProductID = parentID
				row.ParentProductName = firstMiniOrderValue(source.ParentProductName, source.ProductRecordName, source.Name)
				row.Name = row.ParentProductName
				row.SKUName = option.SpecName
				row.SKUCode = option.SpecCode
				row.SpecLabel = option.SpecName
				row.InventoryUnit = option.InventoryUnit
				row.OrderUnit = option.InventoryUnit
				row.QuoteUnit = option.InventoryUnit
				row.Tiers = append([]salesapp.ProductTierOption(nil), option.Tiers...)
				for idx := range row.Tiers {
					row.Tiers[idx].ParentProductID = parentID
					row.Tiers[idx].BomSpecID = option.BomSpecID
					row.Tiers[idx].BomVariantID = option.BomVariantID
				}
				out = append(out, row)
			}
		}
	}
	return out, optionBySpec
}

type miniEmployeeBOMSpecFamilyState struct {
	row   map[string]any
	specs []map[string]any
}

func miniEmployeeBOMSpecFamilies(
	products []salesapp.ProductOption,
	optionBySpec map[int64]salesapp.ProductBOMSpecOption,
) ([]map[string]any, []salesapp.ProductBOMSpecOption) {
	states := map[string]*miniEmployeeBOMSpecFamilyState{}
	order := make([]string, 0)
	visibleBySpec := map[int64]salesapp.ProductBOMSpecOption{}
	for _, product := range products {
		option, ok := optionBySpec[product.SKUID]
		if !ok || option.ParentProductID <= 0 || option.BomSpecID <= 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d:%d", product.CustomerID, option.ParentProductID, product.CustomerProductAliasID)
		state := states[key]
		if state == nil {
			parentName := firstMiniOrderValue(product.ParentProductName, product.ProductRecordName, product.Name)
			aliasName := strings.TrimSpace(product.CustomerProductDisplayName)
			displayName := firstMiniOrderValue(aliasName, parentName)
			state = &miniEmployeeBOMSpecFamilyState{row: map[string]any{
				"parent_product_id":                    option.ParentProductID,
				"parent_product_name":                  parentName,
				"name":                                 displayName,
				"product_name_snapshot":                parentName,
				"product_code":                         firstMiniOrderValue(product.ProductCode, product.SKUCode),
				"customer_id":                          product.CustomerID,
				"customer_product_alias_id":            product.CustomerProductAliasID,
				"customer_product_display_name":        aliasName,
				"alias_name":                           aliasName,
				"customer_item_code":                   product.CustomerItemCode,
				"brand_name":                           product.BrandName,
				"customer_alias_display_category_id":   product.CustomerAliasDisplayCategoryID,
				"customer_alias_display_category_name": product.CustomerAliasDisplayCategoryName,
				"product_kind":                         product.ProductKind,
				"visibility":                           product.Visibility,
				"product_type_category_id":             product.ProductTypeCategoryID,
				"product_type_name":                    product.ProductTypeName,
				"migration_state":                      "cutover",
				"bom_spec_migration_state":             "cutover",
				"default_bom_spec_id":                  int64(0),
				"py":                                   catalogdomain.SearchPinyin(displayName + " " + parentName),
				"pyi":                                  catalogdomain.SearchInitials(displayName + " " + parentName),
			}}
			states[key] = state
			order = append(order, key)
		}
		if option.IsDefault {
			state.row["default_bom_spec_id"] = option.BomSpecID
		}
		search := strings.TrimSpace(option.SpecName + " " + option.SpecKey)
		tiers := append([]salesapp.ProductTierOption(nil), product.Tiers...)
		state.specs = append(state.specs, map[string]any{
			"product_id":        option.ParentProductID,
			"parent_product_id": option.ParentProductID,
			"migration_state":   "cutover",
			"bom_spec_id":       option.BomSpecID,
			"bom_variant_id":    option.BomVariantID,
			"spec_code":         option.SpecCode,
			"barcode":           option.Barcode,
			"spec_key":          option.SpecKey,
			"spec_name":         option.SpecName,
			"spec_label":        option.SpecName,
			"inventory_unit":    option.InventoryUnit,
			"sales_unit":        option.InventoryUnit,
			"order_unit":        option.InventoryUnit,
			"is_default_sku":    option.IsDefault,
			"is_default":        option.IsDefault,
			"sort_order":        option.SortOrder,
			"published":         option.Published,
			"bom_id":            option.BomID,
			"bom_version_id":    option.BomVersionID,
			"bom_version_no":    option.BomVersionNo,
			"tiers":             tiers,
			"py":                catalogdomain.SearchPinyin(search),
			"pyi":               catalogdomain.SearchInitials(search),
		})
		option.Tiers = tiers
		visibleBySpec[option.BomSpecID] = option
	}

	out := make([]map[string]any, 0, len(order))
	for _, key := range order {
		state := states[key]
		sort.SliceStable(state.specs, func(i, j int) bool {
			leftDefault, _ := state.specs[i]["is_default"].(bool)
			rightDefault, _ := state.specs[j]["is_default"].(bool)
			if leftDefault != rightDefault {
				return leftDefault
			}
			leftOrder, _ := state.specs[i]["sort_order"].(int)
			rightOrder, _ := state.specs[j]["sort_order"].(int)
			if leftOrder != rightOrder {
				return leftOrder < rightOrder
			}
			return fmt.Sprint(state.specs[i]["spec_key"]) < fmt.Sprint(state.specs[j]["spec_key"])
		})
		state.row["specs"] = state.specs
		out = append(out, state.row)
	}
	visibleOptions := make([]salesapp.ProductBOMSpecOption, 0, len(visibleBySpec))
	for _, option := range visibleBySpec {
		visibleOptions = append(visibleOptions, option)
	}
	sort.SliceStable(visibleOptions, func(i, j int) bool {
		if visibleOptions[i].ParentProductID != visibleOptions[j].ParentProductID {
			return visibleOptions[i].ParentProductID < visibleOptions[j].ParentProductID
		}
		if visibleOptions[i].IsDefault != visibleOptions[j].IsDefault {
			return visibleOptions[i].IsDefault
		}
		if visibleOptions[i].SortOrder != visibleOptions[j].SortOrder {
			return visibleOptions[i].SortOrder < visibleOptions[j].SortOrder
		}
		return visibleOptions[i].BomSpecID < visibleOptions[j].BomSpecID
	})
	return out, visibleOptions
}
