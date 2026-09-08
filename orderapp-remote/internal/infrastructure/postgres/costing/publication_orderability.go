package costing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/costing"
)

type priceTableOrderProduct struct {
	parentID           int64
	customerID         int64
	name               string
	active, configured bool
}

func (r Repository) ValidateBeanListOrderability(ctx context.Context, cmd app.PublishBeanListCommand) error {
	items, err := app.BeanListOrderabilityItems(cmd)
	if err != nil {
		return err
	}
	products := map[int64]priceTableOrderProduct{}
	canonicalParents := map[int64]bool{}
	for _, item := range items {
		if item.BomSpecID > 0 && item.BomVariantID > 0 {
			canonicalParents[item.ProductID] = true
		}
	}
	issues := []string{}
	seenIssues := map[string]bool{}
	addIssue := func(message string) {
		if !seenIssues[message] {
			seenIssues[message] = true
			issues = append(issues, message)
		}
	}
	for _, item := range items {
		product, ok := products[item.ProductID]
		if !ok {
			err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(p.parent_product_id,0),p.id),
			COALESCE(parent.name,p.name,''),p.active AND COALESCE(parent.active,true),COALESCE(authority.configured,false),COALESCE(parent.customer_id,p.customer_id,0)
			FROM %[1]s.products p
			LEFT JOIN %[1]s.products parent ON parent.id=NULLIF(p.parent_product_id,0)
			LEFT JOIN %[1]s.product_bom_spec_authorities authority ON authority.product_id=COALESCE(NULLIF(p.parent_product_id,0),p.id)
			WHERE p.id=$1`, r.schema), item.ProductID).Scan(&product.parentID, &product.name, &product.active, &product.configured, &product.customerID)
			if errors.Is(err, pgx.ErrNoRows) {
				addIssue(fmt.Sprintf("商品 #%d 不存在，无法用于录单", item.ProductID))
				continue
			}
			if err != nil {
				return err
			}
			products[item.ProductID] = product
		}
		if product.customerID > 0 && (cmd.OwnerType != "customer" || cmd.OwnerKey != fmt.Sprint(product.customerID)) {
			addIssue(fmt.Sprintf("商品 #%d 不属于当前价格表归属，请重新选择商品", item.ProductID))
			continue
		}
		label := fmt.Sprintf("商品「%s」（#%d）", product.name, product.parentID)
		switch {
		case !product.active:
			addIssue(label + "已停用，无法用于录单")
		case item.ParentProductID > 0 && item.ParentProductID != product.parentID:
			addIssue(label + "的价格表商品身份不一致，请重新选择商品规格")
		case !product.configured:
			addIssue(label + "未配置可用于录单的默认已发布 BOM 规格，请配置并发布默认 BOM 后重新选择规格")
		case item.BomSpecID <= 0 || item.BomVariantID <= 0:
			if item.ProductID != product.parentID || !canonicalParents[product.parentID] {
				addIssue(label + "仍使用旧销售规格，无法用于录单，请重新选择当前默认已发布 BOM 的规格")
			}
		default:
			identity, resolveErr := r.ResolveProductBOMSpecIdentity(ctx, product.parentID, item.BomSpecID, item.BomVariantID)
			if errors.Is(resolveErr, app.ErrProductBOMSpecIdentityNotFound) {
				addIssue(fmt.Sprintf("%s的规格 #%d 已不属于当前默认已发布 BOM，请重新选择规格", label, item.BomSpecID))
				continue
			}
			if resolveErr != nil {
				return resolveErr
			}
			if !identity.Active || !identity.Published || strings.TrimSpace(identity.SpecKey) == "" || strings.TrimSpace(identity.SpecName) == "" || strings.TrimSpace(identity.InventoryUnit) == "" {
				addIssue(fmt.Sprintf("%s的规格 #%d 缺少有效名称或库存单位，无法用于录单", label, item.BomSpecID))
			}
		}
	}
	if len(issues) > 0 {
		return fmt.Errorf("价格表无法用于录单：%s", strings.Join(issues, "；"))
	}
	return nil
}
