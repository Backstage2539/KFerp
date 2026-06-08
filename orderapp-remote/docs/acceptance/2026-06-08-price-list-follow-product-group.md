# PR-459 商品价格表跟随商品档案商品分组

## Scope
- 商品档案选择的 `商品分组` 作为商品价格表进入时的默认商品分组模板。
- 商品价格表选品分类复用 `business-grouping`，按 `product_catalog/product` 归类展示模板大类、小类和 `未分类`。
- 平铺价格行快照写入 `group_source=product_catalog`；价格表本次覆盖仍只写入价格表版本快照，不回写商品档案。

## Acceptance
- 在商品档案选择 `商品分组` 后进入商品价格表，选品分类看到该模板下的分类标题，例如 `咖啡熟豆`、`挂耳咖啡` 和 `未分类`。
- 商品只按照当前商品分组模板的 `business_group_assignments` 归类；其他模板、已删除分类或无归类的商品都进入 `未分类`。
- 生成平铺价格行时，分组快照来源为 `product_catalog`，不误用旧商品类型或其他分组模板。

## Evidence
- Targeted frontend: `node --test src/lib/product-bean-list-split.test.js` covers business group product categories and `business-group-unclassified`.
- Targeted Go: `go test ./internal/interfaces/http/costing -run TestCostingViewFollowsProductCatalogBusinessGroupTemplate -count=1` protects the Vue source contract.
- Support contract: `go test ./internal/interfaces/http/support -run TestDev459PriceListFollowProductGroupContracts -count=1` protects PR/DEV markers, docs and shared grouping bridge.
