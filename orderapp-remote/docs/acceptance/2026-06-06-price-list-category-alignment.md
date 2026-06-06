# PR-435-PRICE-LIST-CATEGORY-ALIGNMENT 商品价格表分类与商品档案一致

Requirement: PR-435-PRICE-LIST-CATEGORY-ALIGNMENT

## Scope
- 商品价格表的商品类型下拉、已发布价格表分组和预览卡片使用商品档案当前分类。
- 旧 `product_type_category_id/product_type_name` 不再生成价格表分类，只作为历史兼容字段。
- 没有当前分类的商品合并为一个 `未分类商品`。

## Acceptance
- [ ] 商品档案 Tab 和商品价格表商品类型下拉分类名称一致。
- [ ] 旧字段数据只进入一个 `未分类商品`，不再出现多个 `其他`。
- [ ] `咖啡挂耳`、`咖啡熟豆`、`速溶咖啡` 等当前分类只显示各自商品。
- [ ] 混合未分类商品按单个商品渲染价格行，生豆仍显示生豆价格档。

## Evidence
- RED：`go test ./internal/interfaces/http/support -run TestDev435PriceListCategoryAlignment -count=1` 先失败于缺少 PR-435 种子和手册标记。
- GREEN：待本地测试、development 发布和浏览器验收补充。
