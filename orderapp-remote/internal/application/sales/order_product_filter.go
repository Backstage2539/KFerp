package sales

import (
	"encoding/json"
	"strings"
)

// FilterOrderProductsForCustomer applies the same customer/public/alias
// visibility rules used by ERP order entry. Publication options constrain the
// product to versions available to that customer, but do not trim its tiers.
func FilterOrderProductsForCustomer(products []ProductOption, customerID int64, versionOptions []BeanListVersionOption, publicUsages ...[]CustomerPublicUsageOption) []ProductOption {
	publicChoiceIDsByType := map[string]map[int64]bool{}
	for _, v := range versionOptions {
		if v.CustomerID == 0 && !v.IsCustomerOwned && v.ID > 0 {
			key := orderNormalizeListType(v.ListType)
			if publicChoiceIDsByType[key] == nil {
				publicChoiceIDsByType[key] = map[int64]bool{}
			}
			publicChoiceIDsByType[key][v.ID] = true
		}
	}
	if customerID > 0 && len(publicChoiceIDsByType) > 0 {
		originals := products
		products = append([]ProductOption{}, products...)
		for i, p := range products {
			if p.CustomerID != customerID || (p.Visibility != "customer_reference" && p.Visibility != "customer_alias" && p.CustomerProductAliasID <= 0) {
				continue
			}
			tiers := append([]ProductTierOption{}, p.Tiers...)
			for _, base := range originals {
				if base.ID != p.ID || base.CustomerID != 0 {
					continue
				}
				for _, tier := range base.Tiers {
					pub, kind := orderTierPublicationIdentity(tier)
					if publicChoiceIDsByType[orderNormalizeListType(kind)][pub] {
						tiers = append(tiers, tier)
					}
				}
			}
			products[i].Tiers = tiers
		}
	}
	availablePublicationIDsByType := map[string]map[int64]bool{}
	ownedPublicationIDsByType := map[string]map[int64]bool{}
	if len(versionOptions) > 0 {
		availablePublicationIDsByType = orderAvailablePublicationIDsByListType(customerID, versionOptions)
		ownedPublicationIDsByType = orderCustomerOwnedPublicationIDsByListType(customerID, versionOptions)
	}
	allowsPublicProducts := true
	if len(publicUsages) > 0 {
		allowsPublicProducts = orderCustomerAllowsPublicProducts(customerID, publicUsages[0])
	}
	customerScopedProductIDs := map[int64]bool{}
	if customerID > 0 {
		for _, product := range products {
			visibility := orderProductVisibility(product.Visibility, product.CustomerID)
			if product.CustomerID == customerID && (product.CustomerProductAliasID > 0 || visibility == "customer_alias" || visibility == "customer_reference") {
				customerScopedProductIDs[product.ID] = true
			}
		}
	}
	out := make([]ProductOption, 0, len(products))
	for _, product := range products {
		visibility := orderProductVisibility(product.Visibility, product.CustomerID)
		if product.CustomerProductAliasID > 0 || visibility == "customer_alias" {
			if customerID > 0 && product.CustomerID == customerID {
				if !orderProductMatchesAvailablePublicationScope(product, availablePublicationIDsByType) {
					continue
				}
				product.Visibility = "customer_alias"
				out = append(out, product)
			}
			continue
		}
		if visibility == "public" || product.CustomerID == 0 {
			if customerID > 0 && customerScopedProductIDs[product.ID] {
				continue
			}
			if customerID > 0 && !allowsPublicProducts && !orderProductMatchesExplicitPublicationScope(product, ownedPublicationIDsByType) && !orderProductMatchesExplicitPublicationScope(product, publicChoiceIDsByType) {
				continue
			}
			if !orderProductMatchesAvailablePublicationScope(product, availablePublicationIDsByType) {
				continue
			}
			product.Visibility = "public"
			out = append(out, product)
			continue
		}
		if customerID > 0 && product.CustomerID == customerID {
			if !orderProductMatchesAvailablePublicationScope(product, availablePublicationIDsByType) {
				continue
			}
			product.Visibility = "customer_only"
			out = append(out, product)
		}
	}
	return out
}

// FilterOrderProductsForDefaultPublications trims the shared staff catalog to
// publications marked selected by the resolver. Customer/order types do not
// reroute it. The optional boolean remains only for old callers.
func FilterOrderProductsForDefaultPublications(products []ProductOption, customerID int64, versionOptions []BeanListVersionOption, publicUsages []CustomerPublicUsageOption, _ ...bool) []ProductOption {
	selected := map[int64]BeanListVersionOption{}
	for _, table := range versionOptions {
		if table.CustomerID == customerID && table.ID > 0 && table.IsDefault {
			selected[table.ID] = table
		}
	}
	visible := FilterOrderProductsForCustomer(products, customerID, nil)
	allowsPublic := orderCustomerAllowsPublicProducts(customerID, publicUsages)
	out := make([]ProductOption, 0, len(visible))
	for _, product := range visible {
		tiers := []ProductTierOption{}
		for _, tier := range product.Tiers {
			id, _ := orderTierPublicationIdentity(tier)
			table, ok := selected[id]
			if !ok {
				continue
			}
			if product.Visibility == "public" && !allowsPublic && !table.IsCustomerOwned {
				continue
			}
			tiers = append(tiers, orderCanonicalTierPublicationMetadata(tier, id, table.ListType, table))
		}
		if len(tiers) == 0 {
			continue
		}
		product.Tiers = tiers
		out = append(out, product)
	}
	return out
}

func orderCanonicalTierPublicationMetadata(tier ProductTierOption, publicationID int64, listType string, option BeanListVersionOption) ProductTierOption {
	tier.PublicationID = publicationID
	tier.ListType = orderNormalizeStrictListType(listType)
	if strings.TrimSpace(option.VersionNo) != "" {
		tier.PublicationVersionNo = strings.TrimSpace(option.VersionNo)
	}
	var source map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(tier.PriceSourceJSON)), &source) == nil && len(source) > 0 {
		source["publication_id"] = publicationID
		source["bean_list_publication_id"] = publicationID
		source["list_type"] = tier.ListType
		if tier.PublicationVersionNo != "" {
			source["version_no"] = tier.PublicationVersionNo
			source["bean_list_version_no"] = tier.PublicationVersionNo
		}
		if raw, err := json.Marshal(source); err == nil {
			tier.PriceSourceJSON = string(raw)
		}
	}
	return tier
}

func orderAvailablePublicationIDsByListType(customerID int64, options []BeanListVersionOption) map[string]map[int64]bool {
	out := map[string]map[int64]bool{}
	if customerID <= 0 {
		return out
	}
	for _, option := range options {
		if (option.CustomerID != customerID && !(option.CustomerID == 0 && !option.IsCustomerOwned)) || option.ID <= 0 {
			continue
		}
		listType := orderNormalizeListType(option.ListType)
		if out[listType] == nil {
			out[listType] = map[int64]bool{}
		}
		out[listType][option.ID] = true
	}
	return out
}

func orderCustomerOwnedPublicationIDsByListType(customerID int64, options []BeanListVersionOption) map[string]map[int64]bool {
	out := map[string]map[int64]bool{}
	if customerID <= 0 {
		return out
	}
	for _, option := range options {
		if option.CustomerID != customerID || !option.IsCustomerOwned || option.ID <= 0 {
			continue
		}
		listType := orderNormalizeListType(option.ListType)
		if out[listType] == nil {
			out[listType] = map[int64]bool{}
		}
		out[listType][option.ID] = true
	}
	return out
}

func orderStrictPublicationIDsByListType(customerID int64, options []BeanListVersionOption, customerOwnedOnly bool) map[string]map[int64]bool {
	out := map[string]map[int64]bool{}
	if customerID <= 0 {
		return out
	}
	for _, option := range options {
		if option.CustomerID != customerID || option.ID <= 0 || (customerOwnedOnly && !option.IsCustomerOwned) {
			continue
		}
		listType := orderNormalizeStrictListType(option.ListType)
		if out[listType] == nil {
			out[listType] = map[int64]bool{}
		}
		out[listType][option.ID] = true
	}
	return out
}

func orderCustomerAllowsPublicProducts(customerID int64, usages []CustomerPublicUsageOption) bool {
	if customerID <= 0 {
		return true
	}
	for _, usage := range usages {
		if usage.CustomerID == customerID {
			return usage.UsePublicSKU
		}
	}
	return true
}

func orderProductMatchesAvailablePublicationScope(product ProductOption, publicationIDsByType map[string]map[int64]bool) bool {
	listType := orderProductListType(product.ProductKind)
	if len(publicationIDsByType[listType]) == 0 {
		return true
	}
	return orderProductMatchesExplicitPublicationScope(product, publicationIDsByType)
}

func orderProductMatchesExplicitPublicationScope(product ProductOption, publicationIDsByType map[string]map[int64]bool) bool {
	listType := orderProductListType(product.ProductKind)
	publicationIDs := publicationIDsByType[listType]
	if len(publicationIDs) == 0 {
		return false
	}
	for _, tier := range product.Tiers {
		publicationID, tierListType := orderTierPublicationIdentity(tier)
		if publicationID <= 0 || !publicationIDs[publicationID] {
			continue
		}
		if tierListType != "" && orderNormalizeListType(tierListType) != listType {
			continue
		}
		return true
	}
	return false
}

func orderTierPublicationIdentity(tier ProductTierOption) (int64, string) {
	publicationID := tier.PublicationID
	listType := strings.TrimSpace(tier.ListType)
	if publicationID > 0 && listType != "" {
		return publicationID, listType
	}
	var source struct {
		PublicationID int64  `json:"publication_id"`
		ListType      string `json:"list_type"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(tier.PriceSourceJSON)), &source) == nil {
		if publicationID <= 0 {
			publicationID = source.PublicationID
		}
		if listType == "" {
			listType = source.ListType
		}
	}
	return publicationID, listType
}

func orderProductListType(productKind string) string {
	return orderProductListTypeForOrder(productKind, false)
}

func orderProductListTypeForOrder(productKind string, retailOrder bool) string {
	switch strings.TrimSpace(productKind) {
	case "green_bean":
		return "green"
	case "drip_bag":
		return "drip"
	default:
		if retailOrder {
			return "retail"
		}
		return "commercial"
	}
}

func orderNormalizeListType(listType string) string {
	switch strings.TrimSpace(listType) {
	case "green":
		return "green"
	case "drip":
		return "drip"
	default:
		return "commercial"
	}
}

func orderNormalizeStrictListType(listType string) string {
	if strings.TrimSpace(listType) == "retail" {
		return "retail"
	}
	return orderNormalizeListType(listType)
}

func orderProductVisibility(visibility string, customerID int64) string {
	visibility = strings.TrimSpace(visibility)
	if visibility != "" {
		return visibility
	}
	if customerID > 0 {
		return "customer_only"
	}
	return "public"
}
