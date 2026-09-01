package sales

import (
	"encoding/json"
	"strings"
)

// FilterOrderProductsForCustomer applies the same customer/public/alias
// visibility rules used by ERP order entry. Publication options constrain the
// product to versions available to that customer, but do not trim its tiers.
func FilterOrderProductsForCustomer(products []ProductOption, customerID int64, versionOptions []BeanListVersionOption, publicUsages ...[]CustomerPublicUsageOption) []ProductOption {
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
			if customerID > 0 && !allowsPublicProducts && !orderProductMatchesExplicitPublicationScope(product, ownedPublicationIDsByType) {
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

// FilterOrderProductsForDefaultPublications is the authoritative mini-program
// catalog. It keeps only current default, published price-list versions,
// removes historical tiers, and excludes products/specifications with no
// current sellable price.
func FilterOrderProductsForDefaultPublications(products []ProductOption, customerID int64, versionOptions []BeanListVersionOption, publicUsages []CustomerPublicUsageOption, retailOrders ...bool) []ProductOption {
	retailOrder := len(retailOrders) > 0 && retailOrders[0]
	defaults := make([]BeanListVersionOption, 0, len(versionOptions))
	defaultByID := make(map[int64]BeanListVersionOption, len(versionOptions))
	for _, option := range versionOptions {
		if option.CustomerID == customerID && option.ID > 0 && option.IsDefault {
			defaults = append(defaults, option)
			defaultByID[option.ID] = option
		}
	}
	allowedByType := orderStrictPublicationIDsByListType(customerID, defaults, false)
	ownedByType := orderStrictPublicationIDsByListType(customerID, defaults, true)
	allowsPublicProducts := orderCustomerAllowsPublicProducts(customerID, publicUsages)
	// Customer visibility is shared with ERP, while the strict publication
	// trimming below is order-type aware (retail orders use retail prices for
	// ordinary products).
	visible := FilterOrderProductsForCustomer(products, customerID, nil)
	out := make([]ProductOption, 0, len(visible))
	for _, product := range visible {
		listType := ""
		tiers := []ProductTierOption(nil)
		for _, candidateListType := range orderStrictProductListTypesForOrder(product.ProductKind, retailOrder) {
			allowed := allowedByType[candidateListType]
			if len(allowed) == 0 {
				continue
			}
			candidateTiers := make([]ProductTierOption, 0, len(product.Tiers))
			for _, tier := range product.Tiers {
				publicationID, tierListType := orderTierPublicationIdentity(tier)
				if publicationID <= 0 || !allowed[publicationID] {
					continue
				}
				if tierListType != "" && orderNormalizeStrictListType(tierListType) != candidateListType {
					continue
				}
				candidateTiers = append(candidateTiers, orderCanonicalTierPublicationMetadata(tier, publicationID, candidateListType, defaultByID[publicationID]))
			}
			if len(candidateTiers) > 0 {
				listType = candidateListType
				tiers = candidateTiers
				break
			}
		}
		if len(tiers) == 0 {
			continue
		}
		if product.Visibility == "public" && customerID > 0 && !allowsPublicProducts {
			owned := ownedByType[listType]
			hasOwnedTier := false
			for _, tier := range tiers {
				publicationID, _ := orderTierPublicationIdentity(tier)
				if owned[publicationID] {
					hasOwnedTier = true
					break
				}
			}
			if !hasOwnedTier {
				continue
			}
		}
		product.Tiers = tiers
		out = append(out, product)
	}
	return out
}

func orderStrictProductListTypesForOrder(productKind string, retailOrder bool) []string {
	switch strings.TrimSpace(productKind) {
	case "green_bean":
		return []string{"green"}
	case "drip_bag":
		primary := "commercial"
		if retailOrder {
			primary = "retail"
		}
		return []string{primary, "drip"}
	default:
		if retailOrder {
			return []string{"retail"}
		}
		return []string{"commercial"}
	}
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
		if option.CustomerID != customerID || option.ID <= 0 {
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
