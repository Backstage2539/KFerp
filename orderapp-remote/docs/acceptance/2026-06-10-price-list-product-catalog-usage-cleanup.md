# PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP

## Scope
- 商品价格表顶部商品类型优先按商品档案 `product_catalog` 商品分组顶层大类生成。
- 归在子类下的商品跟随父类入口展示；`咖啡熟豆 / 意式拼配豆` 下的 `熟豆-红岩拼配` 不能因旧分类 `熟豆 / 默认熟豆` 从商品价格表消失。
- ERP 商品价格表版本列表删除用途筛选和用途列，固定查询 `factory_supply` 供货版本。
- `customer_resale` 继续保留为客户小程序转售分享用途，不进入 ERP 录单默认价格版本、订单结算、费用中心或工厂履约计价。

## Evidence
- RED frontend: `node --test src/lib/product-price-list-types.test.js` 先失败，平铺 `business_group_items` 下子类归组商品没有生成父类 `咖啡熟豆` 价格表类型。
- GREEN frontend targeted:
  - `node --test src/lib/product-price-list-types.test.js`
  - `node --test src/lib/costing-bean-list-version-ui.test.js`
- Support contract: `go test ./internal/interfaces/http/support -run 'TestDev46[13]|TestCustomerResaleBeanList' -count=1`

## Manual Updates
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/acceptance/2026-06-06-customer-resale-bean-list.md`

## Browser Acceptance
- 打开 商品价格表，选择 `咖啡熟豆`。
- 在选品树 `意式拼配豆` 下确认能看到 `熟豆-红岩拼配`。
- 在已发布价格表列表确认不显示用途筛选、用途列、`工厂供货价格表` 或 `客户转售价格表` 标签。
