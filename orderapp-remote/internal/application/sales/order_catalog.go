package sales

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	catalogdomain "orderapp/internal/domain/catalog"
)

type OrderCatalog struct {
	Products       []ProductOption
	Families       []map[string]any
	BOMSpecOptions []ProductBOMSpecOption
	BOMProducts    []ProductOption
}

// buildOrderCatalog keeps the legacy SKU catalog untouched until a
// product cuts over. A cutover product is then exposed only as parent product
// plus BOM-owned specification identity; bom_spec_id is never copied into a
// product/SKU id field.
func buildOrderCatalog(
	form OrderFormData,
	customerID int64,
) OrderCatalog {
	legacyProducts := FilterOrderProductsForDefaultPublications(
		form.Products,
		customerID,
		form.BeanListVersionOptions,
		form.CustomerPublicUsages,
	)
	cutoverParents := map[int64]bool{}
	cutoverOptions := make([]ProductBOMSpecOption, 0)
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
		return OrderCatalog{
			Products: legacyProducts,
			Families: BuildOrderProductFamilies(legacyProducts),
		}
	}

	visibleLegacy := make([]ProductOption, 0, len(legacyProducts))
	for _, product := range legacyProducts {
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		if !cutoverParents[parentID] {
			visibleLegacy = append(visibleLegacy, product)
		}
	}

	canonicalProducts, optionBySpec := OrderBOMSpecProducts(form.Products, cutoverOptions)
	canonicalProducts = FilterOrderProductsForDefaultPublications(
		canonicalProducts,
		customerID,
		form.BeanListVersionOptions,
		form.CustomerPublicUsages,
	)
	families, visibleOptions := OrderBOMSpecFamilies(canonicalProducts, optionBySpec)
	return OrderCatalog{
		Products:       visibleLegacy,
		Families:       append(BuildOrderProductFamilies(visibleLegacy), families...),
		BOMSpecOptions: visibleOptions,
		BOMProducts:    canonicalProducts,
	}
}

func OrderBOMSpecProducts(
	products []ProductOption,
	options []ProductBOMSpecOption,
) ([]ProductOption, map[int64]ProductBOMSpecOption) {
	optionBySpec := make(map[int64]ProductBOMSpecOption, len(options))
	optionsByParent := map[int64][]ProductBOMSpecOption{}
	for _, option := range options {
		optionBySpec[option.BomSpecID] = option
		optionsByParent[option.ParentProductID] = append(optionsByParent[option.ParentProductID], option)
	}
	sourcesByParent := map[int64][]ProductOption{}
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

	out := make([]ProductOption, 0)
	for parentID, parentOptions := range optionsByParent {
		sources := sourcesByParent[parentID]
		if len(sources) == 0 {
			sources = []ProductOption{{ID: parentID, ParentProductID: parentID, Name: fmt.Sprintf("商品%d", parentID), Visibility: "public"}}
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
				row.ParentProductName = orderFamilyFirstText(source.ParentProductName, source.ProductRecordName, source.Name)
				row.Name = row.ParentProductName
				row.SKUName = option.SpecName
				row.SKUCode = option.SpecCode
				row.SpecLabel = option.SpecName
				row.InventoryUnit = option.InventoryUnit
				row.OrderUnit = option.InventoryUnit
				row.QuoteUnit = option.InventoryUnit
				row.Tiers = append([]ProductTierOption(nil), option.Tiers...)
				if source.CustomerID != option.OwnerCustomerID {
					row.Tiers = nil
					for _, tier := range source.Tiers {
						if OrderTierMatchesBOMSpec(tier, option) {
							row.Tiers = append(row.Tiers, tier)
						}
					}
				}
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

func OrderTierMatchesBOMSpec(tier ProductTierOption, option ProductBOMSpecOption) bool {
	spec := tier.EffectiveSalesSpec
	if len(spec) == 0 && tier.PriceSourceJSON != "" {
		var source struct {
			EffectiveSalesSpec map[string]any `json:"effective_sales_spec"`
		}
		if json.Unmarshal([]byte(tier.PriceSourceJSON), &source) == nil {
			spec = source.EffectiveSalesSpec
		}
	}
	specID, variantID := tier.BomSpecID, tier.BomVariantID
	if specID <= 0 {
		specID = orderFamilyAnyInt64(spec["bom_spec_id"])
	}
	if variantID <= 0 {
		variantID = orderFamilyAnyInt64(spec["bom_variant_id"])
	}
	return specID == option.BomSpecID && (variantID <= 0 || option.BomVariantID <= 0 || variantID == option.BomVariantID)
}

type orderBOMSpecFamilyState struct {
	row   map[string]any
	specs []map[string]any
}

func OrderBOMSpecFamilies(
	products []ProductOption,
	optionBySpec map[int64]ProductBOMSpecOption,
) ([]map[string]any, []ProductBOMSpecOption) {
	states := map[string]*orderBOMSpecFamilyState{}
	order := make([]string, 0)
	visibleBySpec := map[int64]ProductBOMSpecOption{}
	for _, product := range products {
		option, ok := optionBySpec[product.SKUID]
		if !ok || option.ParentProductID <= 0 || option.BomSpecID <= 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d:%d", product.CustomerID, option.ParentProductID, product.CustomerProductAliasID)
		state := states[key]
		if state == nil {
			parentName := orderFamilyFirstText(product.ParentProductName, product.ProductRecordName, product.Name)
			aliasName := strings.TrimSpace(product.CustomerProductDisplayName)
			displayName := orderFamilyFirstText(aliasName, parentName)
			state = &orderBOMSpecFamilyState{row: map[string]any{
				"parent_product_id":                    option.ParentProductID,
				"parent_product_name":                  parentName,
				"name":                                 displayName,
				"product_name_snapshot":                parentName,
				"product_code":                         orderFamilyFirstText(product.ProductCode, product.SKUCode),
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
		tiers := append([]ProductTierOption(nil), product.Tiers...)
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
	visibleOptions := make([]ProductBOMSpecOption, 0, len(visibleBySpec))
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

// BuildOrderCatalog is the shared product/specification projection for staff
// entry points and their submit validation. Publication selection is resolved
// before this function; HTTP adapters must not reconstruct the catalog.
func BuildOrderCatalog(form OrderFormData, customerID int64) OrderCatalog {
	catalog := buildOrderCatalog(form, customerID)
	byID := map[int64]BeanListVersionOption{}
	for _, table := range form.BeanListVersionOptions {
		if table.IsDefault {
			byID[table.ID] = table
		}
	}
	for _, family := range catalog.Families {
		ids := map[int64]bool{}
		if specs, ok := family["specs"].([]map[string]any); ok {
			for _, spec := range specs {
				switch tiers := spec["tiers"].(type) {
				case []map[string]any:
					for _, tier := range tiers {
						ids[orderFamilyAnyInt64(tier["publication_id"])] = true
					}
				case []ProductTierOption:
					for _, tier := range tiers {
						id, _ := orderTierPublicationIdentity(tier)
						ids[id] = true
					}
				}
			}
		}
		categories := []map[string]any{}
		ordered := []int64{}
		for id := range ids {
			ordered = append(ordered, id)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		for _, id := range ordered {
			if table, ok := byID[id]; ok {
				categories = append(categories, map[string]any{"key": OrderPriceTableTypeKey(table), "label": orderFamilyFirstText(table.ClassificationTemplateName, table.ProductTypeName, table.TableName, "商品"), "publication_id": id})
			}
		}
		family["price_table_categories"] = categories
	}
	return catalog
}

type ResolvedOrderCatalog struct {
	Catalog               OrderCatalog
	PriceTableOptions     []BeanListVersionOption
	SelectedTables        []BeanListVersionOption
	SelectedPriceTableIDs []int64
	Updated               bool
}

func ResolveOrderCatalog(form OrderFormData, customerID int64, ids []int64, currentOnly bool) (ResolvedOrderCatalog, error) {
	selected, err := ResolveOrderPriceTableSelection(form.BeanListVersionOptions, customerID, ids, currentOnly)
	updated := false
	if currentOnly && errors.Is(err, ErrOrderPriceTableUpdated) {
		// Read refreshes may follow the same named table into its new release.
		// Writes still use the strict resolver and must reject stale prices.
		choices := CurrentOrderPriceTableOptions(form.BeanListVersionOptions, customerID)
		byID := map[int64]BeanListVersionOption{}
		for _, row := range orderCustomerAndPublicTables(form.BeanListVersionOptions, customerID) {
			byID[row.ID] = row
		}
		rebased := []int64{}
		for _, id := range ids {
			old, ok := byID[id]
			if !ok {
				return ResolvedOrderCatalog{}, err
			}
			var match *BeanListVersionOption
			for i := range choices {
				row := &choices[i]
				if OrderPriceTableTypeKey(*row) != OrderPriceTableTypeKey(old) || row.IsCustomerOwned != old.IsCustomerOwned || row.ListType != old.ListType {
					continue
				}
				if old.TableKey != "" && row.TableKey == old.TableKey {
					match = row
					break
				}
				if old.TableKey == "" && row.IsDefault {
					match = row
				}
			}
			if match == nil {
				return ResolvedOrderCatalog{}, ErrOrderPriceTableUpdated
			}
			rebased = append(rebased, match.ID)
		}
		selected, err = ResolveOrderPriceTableSelection(form.BeanListVersionOptions, customerID, rebased, true)
		updated = err == nil
	}
	if err != nil {
		return ResolvedOrderCatalog{}, err
	}
	options := CurrentOrderPriceTableOptions(form.BeanListVersionOptions, customerID)
	form.BeanListVersionOptions = selected
	form = OrderFormWithSelectedPublicPrices(form, customerID)
	selectedIDs := []int64{}
	for _, table := range selected {
		selectedIDs = append(selectedIDs, table.ID)
	}
	return ResolvedOrderCatalog{Catalog: BuildOrderCatalog(form, customerID), PriceTableOptions: options, SelectedTables: selected, SelectedPriceTableIDs: selectedIDs, Updated: updated}, nil
}
