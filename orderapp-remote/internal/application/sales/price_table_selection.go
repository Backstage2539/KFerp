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

func ResolveOrderPriceTableSelection(options []BeanListVersionOption, customerID int64, ids []int64, currentOnly bool) ([]BeanListVersionOption, error) {
	byID := map[int64]BeanListVersionOption{}
	selected := map[string]BeanListVersionOption{}
	current := map[string]BeanListVersionOption{}
	for _, row := range options {
		if row.CustomerID != customerID || row.ID <= 0 {
			continue
		}
		byID[row.ID] = row
		if row.IsDefault {
			key := OrderPriceTableTypeKey(row)
			selected[key] = row
			current[key] = row
		}
	}
	explicit := map[string]bool{}
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("所选价格表不存在或不属于当前客户")
		}
		key := OrderPriceTableTypeKey(row)
		if explicit[key] {
			return nil, fmt.Errorf("同一商品类型只能选择一张价格表")
		}
		explicit[key] = true
		if currentOnly {
			latest, ok := current[key]
			if !ok || !(row.ID == latest.ID || (row.ReleaseID != "" && row.ReleaseID == latest.ReleaseID)) {
				return nil, fmt.Errorf("价格表已更新，请选择当前版本中的价格表")
			}
		}
		selected[key] = row
	}
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]BeanListVersionOption, 0, len(keys))
	for _, key := range keys {
		row := selected[key]
		row.IsDefault = true
		result = append(result, row)
	}
	return result, nil
}

func CurrentOrderPriceTableOptions(options []BeanListVersionOption, customerID int64) []BeanListVersionOption {
	defaults := map[string]BeanListVersionOption{}
	for _, row := range options {
		if row.CustomerID == customerID && row.IsDefault {
			defaults[OrderPriceTableTypeKey(row)] = row
		}
	}
	result := []BeanListVersionOption{}
	for _, row := range options {
		latest, ok := defaults[OrderPriceTableTypeKey(row)]
		if row.CustomerID == customerID && ok && (row.ID == latest.ID || (row.ReleaseID != "" && row.ReleaseID == latest.ReleaseID)) {
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
