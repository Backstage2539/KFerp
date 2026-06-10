# PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX

## Scope
- 商品价格表顶部选择商品类型后，默认全选该类型下的商品，预览立即按当前选择生成。
- `选择分类和产品` 只展示当前商品类型相关父类、子类和商品，不展示其他商品类型的空分类。
- 勾选父类包含子类商品；取消勾选分类或商品后，下方价格表预览动态更新。
- 点击父类 `收起` 时，子类分类行一起隐藏；展开后恢复显示且不清空已选商品。

## Verification
- RED: `node --test src/lib/product-price-list-types.test.js` failed before `priceListSelectionStateKey` existed.
- RED: `node --test src/lib/product-price-list-selection.test.js` failed before picker selection helper existed.
- RED: `node --test src/lib/costing-bean-list-version-ui.test.js` failed before `CostingView.vue` used product-catalog selection and cascade helpers.
- GREEN: `node --test src/lib/product-price-list-types.test.js src/lib/product-price-list-selection.test.js src/lib/costing-bean-list-version-ui.test.js`.

## Manual Acceptance
- 打开商品价格表，顶部选择 `咖啡熟豆`。
- 确认 `选择分类和产品` 默认全选当前类型商品，下方预览有价格表内容。
- 确认 `选择分类和产品` 中不出现 `挂耳咖啡` 等其他类型空分类。
- 取消勾选一个商品或分类，确认预览同步减少；重新勾选后预览恢复。
- 点击父类 `收起`，确认其子类分类行一起隐藏；点击 `展开` 后恢复。
