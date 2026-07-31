package sales

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	catalogdomain "orderapp/internal/domain/catalog"
)

type orderProductFamilySource struct {
	product ProductOption
}

type orderProductFamilySpecState struct {
	row      map[string]any
	tiers    []map[string]any
	tierKeys map[string]bool
	weightG  int64
}

type orderProductFamilyState struct {
	row      map[string]any
	specs    []*orderProductFamilySpecState
	bySKU    map[int64]*orderProductFamilySpecState
	sources  []orderProductFamilySource
	hasChild bool
	concrete bool
}

// BuildOrderProductFamilies converts flat order products into the shared
// product -> specification contract used by both the ERP and mini program.
// Product families are customer/parent/alias scoped so customer aliases never
// leak into another customer's choices.
func BuildOrderProductFamilies(products []ProductOption) []map[string]any {
	states := make([]*orderProductFamilyState, 0)
	byKey := map[string]*orderProductFamilyState{}
	productKeys := make([]string, len(products))
	for i, product := range products {
		parentID := orderFamilyParentProductID(product)
		if parentID <= 0 || orderFamilySKUID(product) <= 0 {
			continue
		}
		key := orderFamilyKey(product.CustomerID, parentID, product.CustomerProductAliasID)
		productKeys[i] = key
		state := byKey[key]
		if state == nil {
			parentName := orderFamilyParentProductName(product)
			aliasName := orderFamilyAliasName(product)
			displayName := orderFamilyFirstText(aliasName, parentName)
			searchName := strings.Join(nonEmptyOrderFamilyText([]string{displayName, parentName}), " ")
			state = &orderProductFamilyState{
				row: map[string]any{
					"parent_product_id":                    parentID,
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
					"visibility":                           orderFamilyVisibility(product.Visibility, product.CustomerID),
					"product_type_category_id":             product.ProductTypeCategoryID,
					"product_type_name":                    product.ProductTypeName,
					"default_sku_id":                       product.DefaultSKUID,
					"py":                                   catalogdomain.SearchPinyin(searchName),
					"pyi":                                  catalogdomain.SearchInitials(searchName),
				},
				bySKU: map[int64]*orderProductFamilySpecState{},
			}
			states = append(states, state)
			byKey[key] = state
		} else {
			mergeOrderFamilyMetadata(state.row, product)
		}
		if product.ParentProductID > 0 && product.ParentProductID != product.ID {
			state.hasChild = true
		}
		state.sources = append(state.sources, orderProductFamilySource{product: product})
	}

	for i, product := range products {
		state := byKey[productKeys[i]]
		if state == nil || (state.hasChild && orderFamilySKUID(product) == orderFamilyParentProductID(product)) {
			continue
		}
		skuID := orderFamilySKUID(product)
		if _, exists := state.bySKU[skuID]; exists {
			continue
		}
		spec := &orderProductFamilySpecState{
			row:      orderFamilySpec(product),
			tierKeys: map[string]bool{},
			weightG:  orderFamilyProductWeightG(product),
		}
		state.specs = append(state.specs, spec)
		state.bySKU[skuID] = spec
	}

	for _, state := range states {
		for _, source := range state.sources {
			for _, tier := range source.product.Tiers {
				spec := orderFamilyTierSpec(state.bySKU, state.specs, source.product, tier)
				if spec == nil {
					continue
				}
				key := orderFamilyTierKey(tier)
				if spec.tierKeys[key] {
					continue
				}
				spec.tierKeys[key] = true
				spec.tiers = append(spec.tiers, orderFamilyTier(tier))
				if orderFamilyTierIsConcrete(tier) {
					state.concrete = true
				}
			}
		}
	}

	out := make([]map[string]any, 0, len(states))
	for _, state := range states {
		if len(state.specs) == 0 {
			continue
		}
		aliasName := orderFamilyStateAliasName(state)
		parentName := strings.TrimSpace(fmt.Sprint(state.row["parent_product_name"]))
		displayName := orderFamilyFirstText(aliasName, parentName)
		state.row["name"] = displayName
		state.row["alias_name"] = aliasName
		state.row["customer_product_display_name"] = aliasName
		searchName := strings.Join(nonEmptyOrderFamilyText([]string{displayName, parentName}), " ")
		state.row["py"] = catalogdomain.SearchPinyin(searchName)
		state.row["pyi"] = catalogdomain.SearchInitials(searchName)
		sort.SliceStable(state.specs, func(i, j int) bool {
			leftDefault, _ := state.specs[i].row["is_default_sku"].(bool)
			rightDefault, _ := state.specs[j].row["is_default_sku"].(bool)
			if leftDefault != rightDefault {
				return leftDefault
			}
			if state.specs[i].weightG > 0 && state.specs[j].weightG > 0 && state.specs[i].weightG != state.specs[j].weightG {
				return state.specs[i].weightG < state.specs[j].weightG
			}
			return strings.TrimSpace(fmt.Sprint(state.specs[i].row["spec_label"])) < strings.TrimSpace(fmt.Sprint(state.specs[j].row["spec_label"]))
		})
		specs := make([]map[string]any, 0, len(state.specs))
		codes := make([]string, 0, len(state.specs)+2)
		codes = append(codes, strings.TrimSpace(fmt.Sprint(state.row["product_code"])), strings.TrimSpace(fmt.Sprint(state.row["customer_item_code"])))
		defaultSKUID := orderFamilyMapInt64(state.row, "default_sku_id")
		if state.bySKU[defaultSKUID] == nil {
			defaultSKUID = 0
		}
		for _, spec := range state.specs {
			spec.row["tiers"] = spec.tiers
			publicationIDs := make([]int64, 0)
			seenPublications := map[int64]bool{}
			for _, tier := range spec.tiers {
				publicationID := orderFamilyMapInt64(tier, "publication_id")
				if publicationID <= 0 || seenPublications[publicationID] {
					continue
				}
				seenPublications[publicationID] = true
				publicationIDs = append(publicationIDs, publicationID)
			}
			spec.row["publication_ids"] = publicationIDs
			if len(publicationIDs) > 0 {
				spec.row["default_publication_id"] = publicationIDs[0]
			} else {
				spec.row["default_publication_id"] = int64(0)
			}
			specs = append(specs, spec.row)
			codes = append(codes, strings.TrimSpace(fmt.Sprint(spec.row["sku_code"])))
			if defaultSKUID <= 0 {
				if isDefault, _ := spec.row["is_default_sku"].(bool); isDefault {
					defaultSKUID = orderFamilyMapInt64(spec.row, "sku_id")
				}
			}
		}
		if defaultSKUID <= 0 {
			defaultSKUID = orderFamilyMapInt64(specs[0], "sku_id")
		}
		state.row["default_sku_id"] = defaultSKUID
		state.row["specs"] = specs
		state.row["code"] = strings.TrimSpace(strings.Join(nonEmptyOrderFamilyText(codes), " "))
		state.row["__order_concrete_price_family"] = state.concrete
		out = append(out, state.row)
	}
	return out
}

func orderFamilyKey(customerID, parentID, aliasID int64) string {
	return strings.Join([]string{strconv.FormatInt(customerID, 10), strconv.FormatInt(parentID, 10), strconv.FormatInt(aliasID, 10)}, ":")
}

func orderFamilySKUID(product ProductOption) int64 {
	if product.SKUID > 0 {
		return product.SKUID
	}
	return product.ID
}

func orderFamilyParentProductID(product ProductOption) int64 {
	if product.ParentProductID > 0 {
		return product.ParentProductID
	}
	return product.ID
}

func orderFamilyParentProductName(product ProductOption) string {
	name := orderFamilyFirstText(product.ParentProductName, product.ProductRecordName, product.Name)
	specLabel := strings.TrimSpace(product.SpecLabel)
	if specLabel != "" && strings.HasSuffix(name, specLabel) {
		name = strings.TrimSpace(strings.TrimSuffix(name, specLabel))
	}
	return name
}

func orderFamilyAliasName(product ProductOption) string {
	name := strings.TrimSpace(product.CustomerProductDisplayName)
	if name == "" {
		return ""
	}
	specLabel := strings.TrimSpace(product.SpecLabel)
	if specLabel == "" {
		specLabel = orderFamilyNetContentLabel(product)
	}
	if specLabel != "" && strings.HasSuffix(name, specLabel) {
		name = strings.TrimSpace(strings.TrimSuffix(name, specLabel))
	}
	return name
}

func orderFamilyStateAliasName(state *orderProductFamilyState) string {
	if state == nil {
		return ""
	}
	aliases := make([]string, 0, len(state.sources))
	labels := make([]string, 0, len(state.sources)*2)
	for _, source := range state.sources {
		aliases = append(aliases, source.product.CustomerProductDisplayName)
		labels = append(labels, source.product.SpecLabel, orderFamilyNetContentLabel(source.product))
	}
	labels = nonEmptyOrderFamilyText(labels)
	sort.SliceStable(labels, func(i, j int) bool { return len(labels[i]) > len(labels[j]) })
	for _, alias := range nonEmptyOrderFamilyText(aliases) {
		if normalized := trimOrderFamilySpecSuffix(alias, labels); normalized != "" {
			return normalized
		}
	}
	return ""
}

func trimOrderFamilySpecSuffix(name string, labels []string) string {
	name = strings.TrimSpace(name)
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		for _, suffix := range []string{label, "（" + label + "）", "(" + label + ")"} {
			if len(name) < len(suffix) || !strings.EqualFold(name[len(name)-len(suffix):], suffix) {
				continue
			}
			trimmed := strings.TrimSpace(name[:len(name)-len(suffix)])
			trimmed = strings.TrimRight(trimmed, "-_/·|：:")
			if trimmed = strings.TrimSpace(trimmed); trimmed != "" {
				return trimmed
			}
		}
	}
	return name
}

func orderFamilySpec(product ProductOption) map[string]any {
	skuID := orderFamilySKUID(product)
	skuName := orderFamilyFirstText(product.SKUName, product.SpecLabel, "默认规格")
	specLabel := orderFamilyFirstText(product.SpecLabel, orderFamilyNetContentLabel(product), skuName)
	searchText := strings.TrimSpace(strings.Join(nonEmptyOrderFamilyText([]string{skuName, specLabel, product.SKUCode, product.ProductCode}), " "))
	return map[string]any{
		"product_id":           skuID,
		"sku_id":               skuID,
		"parent_product_id":    orderFamilyParentProductID(product),
		"sku_name":             skuName,
		"sku_code":             orderFamilyFirstText(product.SKUCode, product.ProductCode),
		"product_code":         orderFamilyFirstText(product.ProductCode, product.SKUCode),
		"spec_label":           specLabel,
		"sales_unit":           product.OrderUnit,
		"net_content_qty":      product.NetContentQty,
		"net_content_unit":     product.NetContentUnit,
		"is_default_sku":       product.IsDefaultSKU,
		"default_sku_id":       product.DefaultSKUID,
		"product_kind":         product.ProductKind,
		"unit_bean_g":          product.DripBagGrams,
		"unit_bag_count":       product.DripBoxBagCount,
		"inventory_unit":       product.InventoryUnit,
		"quote_unit":           product.QuoteUnit,
		"order_unit":           product.OrderUnit,
		"unit_conversion_json": product.UnitConversionJSON,
		"integer_unit":         product.IntegerUnit,
		"py":                   catalogdomain.SearchPinyin(searchText),
		"pyi":                  catalogdomain.SearchInitials(searchText),
	}
}

func orderFamilyTierSpec(bySKU map[int64]*orderProductFamilySpecState, specs []*orderProductFamilySpecState, product ProductOption, tier ProductTierOption) *orderProductFamilySpecState {
	if skuID := orderFamilyAnyInt64(tier.EffectiveSalesSpec["sku_id"]); skuID > 0 {
		if spec := bySKU[skuID]; spec != nil {
			return spec
		}
	}
	if tier.SpecG > 0 {
		var matched *orderProductFamilySpecState
		for _, spec := range specs {
			if spec.weightG != tier.SpecG {
				continue
			}
			if matched != nil {
				matched = nil
				break
			}
			matched = spec
		}
		if matched != nil {
			return matched
		}
	}
	if spec := bySKU[orderFamilySKUID(product)]; spec != nil {
		return spec
	}
	if len(specs) == 1 {
		return specs[0]
	}
	defaultSKUID := product.DefaultSKUID
	if defaultSKUID > 0 {
		return bySKU[defaultSKUID]
	}
	return nil
}

func orderFamilyTier(t ProductTierOption) map[string]any {
	tier := map[string]any{
		"id":                     t.ID,
		"spec_g":                 t.SpecG,
		"min":                    t.MinQty,
		"max":                    t.MaxQty,
		"min_qty":                t.MinQty,
		"max_qty":                t.MaxQty,
		"unit_price":             t.UnitPrice,
		"price":                  t.UnitPrice,
		"product_kind":           t.ProductKind,
		"sales_unit":             t.SalesUnit,
		"unit_bag_count":         t.UnitBagCount,
		"price_source_json":      t.PriceSourceJSON,
		"publication_id":         t.PublicationID,
		"publication_version_no": t.PublicationVersionNo,
		"version_no":             t.PublicationVersionNo,
		"list_type":              t.ListType,
	}
	if t.QuantityBasis != "" {
		tier["quantity_basis"] = t.QuantityBasis
	}
	if t.TierQuantityUnit != "" {
		tier["tier_quantity_unit"] = t.TierQuantityUnit
	}
	if len(t.EffectiveSalesSpec) > 0 {
		tier["effective_sales_spec"] = t.EffectiveSalesSpec
	}
	if t.DisplayUnit != "" {
		tier["display_unit"] = t.DisplayUnit
	}
	return tier
}

func orderFamilyTierKey(tier ProductTierOption) string {
	return fmt.Sprintf("%d:%d:%s:%s:%d:%g", tier.PublicationID, tier.ID, tier.ListType, tier.PublicationVersionNo, tier.SpecG, tier.MinQty)
}

func orderFamilyTierIsConcrete(tier ProductTierOption) bool {
	return strings.TrimSpace(tier.QuantityBasis) == "sales_spec_count" && tier.PublicationID > 0 && orderFamilyAnyInt64(tier.EffectiveSalesSpec["sku_id"]) > 0
}

func orderFamilyProductWeightG(product ProductOption) int64 {
	if product.NetContentQty <= 0 {
		return 0
	}
	factor := float64(0)
	switch strings.ToLower(strings.TrimSpace(product.NetContentUnit)) {
	case "g", "克":
		factor = 1
	case "kg", "千克", "公斤":
		factor = 1000
	case "lb", "lbs", "磅":
		factor = 453.59237
	}
	if factor <= 0 {
		return 0
	}
	return int64(math.Round(product.NetContentQty * factor))
}

func orderFamilyNetContentLabel(product ProductOption) string {
	if product.NetContentQty <= 0 || strings.TrimSpace(product.NetContentUnit) == "" {
		return ""
	}
	return fmt.Sprintf("%g%s", product.NetContentQty, strings.TrimSpace(product.NetContentUnit))
}

func orderFamilyMapInt64(values map[string]any, key string) int64 {
	return orderFamilyAnyInt64(values[key])
}

func orderFamilyAnyInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func orderFamilyFirstText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func nonEmptyOrderFamilyText(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func orderFamilyVisibility(visibility string, customerID int64) string {
	if visibility = strings.TrimSpace(visibility); visibility != "" {
		return visibility
	}
	if customerID > 0 {
		return "customer_only"
	}
	return "public"
}

func mergeOrderFamilyMetadata(row map[string]any, product ProductOption) {
	for key, value := range map[string]string{
		"product_code":                         orderFamilyFirstText(product.ProductCode, product.SKUCode),
		"customer_item_code":                   product.CustomerItemCode,
		"brand_name":                           product.BrandName,
		"customer_alias_display_category_name": product.CustomerAliasDisplayCategoryName,
		"product_kind":                         product.ProductKind,
		"product_type_name":                    product.ProductTypeName,
	} {
		current, _ := row[key].(string)
		if strings.TrimSpace(current) == "" && strings.TrimSpace(value) != "" {
			row[key] = value
		}
	}
	for key, value := range map[string]int64{
		"customer_alias_display_category_id": product.CustomerAliasDisplayCategoryID,
		"product_type_category_id":           product.ProductTypeCategoryID,
		"default_sku_id":                     product.DefaultSKUID,
	} {
		if orderFamilyMapInt64(row, key) == 0 && value > 0 {
			row[key] = value
		}
	}
}
