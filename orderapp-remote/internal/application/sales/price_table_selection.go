package sales

import (
	"fmt"
	"sort"
	"strings"
)

func OrderPriceTableTypeKey(option BeanListVersionOption) string {
	id := option.ClassificationTemplateID
	if id <= 0 {
		id = option.ProductTypeCategoryID
	}
	if id > 0 {
		return fmt.Sprintf("classification:%d", id)
	}
	return "legacy:" + strings.TrimSpace(option.ListType)
}

func orderPriceTableCustomerKey(option BeanListVersionOption) string {
	return fmt.Sprintf("%d:%s", option.CustomerID, OrderPriceTableTypeKey(option))
}

// SQL determines the current release; its explicit default flag picks the table.
func ApplyNamedPriceTableDefaults(options []BeanListVersionOption) []BeanListVersionOption {
	rows := append([]BeanListVersionOption(nil), options...)
	current := map[string]string{}
	for _, row := range rows {
		if row.IsDefault && row.ReleaseID != "" {
			current[orderPriceTableCustomerKey(row)] = row.ReleaseID
		}
	}
	for i := range rows {
		if release := current[orderPriceTableCustomerKey(rows[i])]; release != "" {
			rows[i].IsDefault = rows[i].ReleaseID == release && rows[i].IsDefaultTable
		}
	}
	return rows
}

// Available tables include the customer's tables and public tables. Public
// rows are normalized to the selected customer only for order selection.
func orderCustomerAndPublicTables(options []BeanListVersionOption, customerID int64) []BeanListVersionOption {
	out := []BeanListVersionOption{}
	seen := map[int64]bool{}
	for _, row := range options {
		if row.ID <= 0 || (row.CustomerID != customerID && !(row.CustomerID == 0 && !row.IsCustomerOwned)) || seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		row.CustomerID = customerID
		out = append(out, row)
	}
	return out
}
func orderPriceTableOwnerTypeKey(row BeanListVersionOption) string {
	return fmt.Sprintf("%t:%s", row.IsCustomerOwned, OrderPriceTableTypeKey(row))
}
func ResolveOrderPriceTableSelection(options []BeanListVersionOption, customerID int64, ids []int64, currentOnly bool) ([]BeanListVersionOption, error) {
	byID := map[int64]BeanListVersionOption{}
	selected := map[string]BeanListVersionOption{}
	current := map[string]BeanListVersionOption{}
	for _, row := range orderCustomerAndPublicTables(options, customerID) {
		byID[row.ID] = row
		if row.IsDefault {
			key := OrderPriceTableTypeKey(row)
			current[orderPriceTableOwnerTypeKey(row)] = row
			previous, ok := selected[key]
			if !ok || row.IsCustomerOwned || !previous.IsCustomerOwned {
				selected[key] = row
			}
		}
	}
	explicit := map[string]bool{}
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("所选价格表不存在或不属于当前客户及公共价格表")
		}
		key := OrderPriceTableTypeKey(row)
		if explicit[key] {
			return nil, fmt.Errorf("同一商品类型只能选择一张价格表")
		}
		explicit[key] = true
		if currentOnly {
			latest, ok := current[orderPriceTableOwnerTypeKey(row)]
			if !ok || !(row.ID == latest.ID || row.ReleaseID != "" && row.ReleaseID == latest.ReleaseID) {
				return nil, fmt.Errorf("价格表已更新，请选择当前版本中的价格表")
			}
		}
		selected[key] = row
	}
	keys := []string{}
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := []BeanListVersionOption{}
	for _, key := range keys {
		row := selected[key]
		row.IsDefault = true
		result = append(result, row)
	}
	return result, nil
}
func CurrentOrderPriceTableOptions(options []BeanListVersionOption, customerID int64) []BeanListVersionOption {
	choices := orderCustomerAndPublicTables(options, customerID)
	current := map[string]BeanListVersionOption{}
	owned := map[string]bool{}
	for _, row := range choices {
		if row.IsDefault {
			current[orderPriceTableOwnerTypeKey(row)] = row
			if row.IsCustomerOwned {
				owned[OrderPriceTableTypeKey(row)] = true
			}
		}
	}
	result := []BeanListVersionOption{}
	for _, row := range choices {
		latest, ok := current[orderPriceTableOwnerTypeKey(row)]
		if ok && (row.ID == latest.ID || row.ReleaseID != "" && row.ReleaseID == latest.ReleaseID) {
			if !row.IsCustomerOwned && owned[OrderPriceTableTypeKey(row)] {
				row.IsDefault = false
			}
			result = append(result, row)
		}
	}
	return result
}

func FilterOrderProductsForSelectedPublications(products []ProductOption, customerID int64, options []BeanListVersionOption, usages []CustomerPublicUsageOption, ids []int64, retailOrder bool) ([]ProductOption, error) {
	selected, err := ResolveOrderPriceTableSelection(options, customerID, ids, true)
	if err != nil {
		return nil, err
	}
	return FilterOrderProductsForDefaultPublications(products, customerID, selected, usages, retailOrder), nil
}
