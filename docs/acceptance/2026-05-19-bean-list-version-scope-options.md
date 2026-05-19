# 验收记录：产品豆单范围下拉收敛

## 需求
- 产品豆单页面顶部的“豆单范围”下拉列出“公共豆单”和每个履约客户。
- 页面顶部豆单范围是预览、生成和版本列表的全局上下文；生成豆单抽屉不再二次选择发布归属。
- 选择某个履约客户后，只展示该客户归属的客户豆单发布记录。

## 实现
- `CostingView.vue` 使用独立 `versionListScope`，页面顶部全局范围下拉渲染 `公共豆单` 和履约客户列表项（`customer:<id>`）。
- 版本列表选择客户时调用既有 `GET /api/costing/bean-list/publications?scope=customer&customer_id=<id>`，不再提供所有客户聚合 scope。
- 版本列表中的“生成新版”和“撤回”按行自身 `owner_type/owner_key` 设置或提交归属，避免误用其他客户归属。

## 验证
- 单元测试：`node --test src/lib/costing-bean-list-version-ui.test.js`
- API 测试：`go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPISupports(AllFulfillmentCustomerScope|CustomerScope)' -count=1`
- 仓储守卫：`go test ./internal/infrastructure/postgres/costing -run TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots -count=1`

## 验收结论
- [x] 页面顶部豆单范围下拉列出“公共豆单”和每个履约客户。
- [x] 客户豆单通过 API 按单个 `customer_id` 查询，不再汇总所有客户。
- [x] 生成豆单抽屉不再二次选择发布归属，按页面顶部豆单范围生成和发布。
