package costing

import (
	"context"
	"fmt"
	"strings"
)

// BeanListOrderabilityItem describes a selected or printed product specification.
// Generation must check content as well as selections so a client cannot omit
// the selection array to bypass the current order catalog rules.
type BeanListOrderabilityItem struct {
	ProductID, ParentProductID, BomSpecID, BomVariantID int64
}

func BeanListOrderabilityItems(cmd PublishBeanListCommand) ([]BeanListOrderabilityItem, error) {
	var rawItems []any
	rawItems = append(rawItems, beanListAnySlice(cmd.Config["product_spec_selections"])...)
	rawItems = append(rawItems, beanListAnySlice(cmd.Content["price_rows"])...)
	for _, raw := range beanListAnySlice(cmd.Content["groups"]) {
		group, _ := objectSnapshotMap(raw)
		rawItems = append(rawItems, beanListAnySlice(group["items"])...)
	}
	items := []BeanListOrderabilityItem{}
	seen := map[BeanListOrderabilityItem]bool{}
	for index, raw := range rawItems {
		row, ok := objectSnapshotMap(raw)
		if !ok {
			return nil, fmt.Errorf("第 %d 项商品规格无效，请重新选择", index+1)
		}
		item := BeanListOrderabilityItem{ParentProductID: int64(numberValue(row["parent_product_id"])), BomSpecID: int64(numberValue(row["bom_spec_id"])), BomVariantID: int64(numberValue(row["bom_variant_id"]))}
		for _, key := range []string{"product_id", "productId", "productID", "sku_id"} {
			if item.ProductID = int64(numberValue(row[key])); item.ProductID > 0 {
				break
			}
		}
		if item.BomSpecID > 0 || item.BomVariantID > 0 {
			if item.ParentProductID > 0 {
				item.ProductID = item.ParentProductID
			}
		}
		if item.ProductID <= 0 {
			item.ProductID = item.ParentProductID
		}
		if item.ProductID <= 0 {
			return nil, fmt.Errorf("第 %d 项没有有效商品身份，无法用于录单，请重新选择商品", index+1)
		}
		if !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("请选择可用于录单的商品和规格")
	}
	return items, nil
}

// ValidateBeanListOrderability is read-only. Draft editing stays possible;
// publishing and generating a new document require an orderable catalog.
func (s *Service) ValidateBeanListOrderability(ctx context.Context, cmd PublishBeanListCommand) error {
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.ValidateBeanListOrderability(ctx, cmd)
}

func (s *Service) validateDraftDocumentOrderability(ctx context.Context, row *BeanListPublication) error {
	if row == nil || strings.TrimSpace(row.Status) != "draft" {
		return nil
	}
	return s.ValidateBeanListOrderability(ctx, PublishBeanListCommand{ListType: row.ListType, OwnerType: row.OwnerType, OwnerKey: row.OwnerKey, Config: row.Config, Content: row.Content})
}
