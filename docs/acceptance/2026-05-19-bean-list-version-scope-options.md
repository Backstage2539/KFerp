# 验收记录：豆单版本列表范围下拉收敛

## 需求
- 产品豆单的“豆单版本列表”范围下拉列出“公共豆单”和每个履约客户。
- 版本列表查看范围和生成豆单抽屉的发布归属分开；生成豆单仍可发布公共豆单或指定客户豆单。
- 选择某个履约客户后，只展示该客户归属的客户豆单发布记录。

## 实现
- `CostingView.vue` 使用独立 `versionListScope`，版本列表下拉渲染 `公共豆单` 和履约客户列表项（`customer:<id>`）。
- 版本列表选择客户时调用既有 `GET /api/costing/bean-list/publications?scope=customer&customer_id=<id>`，不再提供所有客户聚合 scope。
- 版本列表中的“生成新版”和“撤回”按行自身 `owner_type/owner_key` 设置或提交归属，避免汇总列表误用当前发布归属。

## 验证
- 单元测试：`node --test src/lib/costing-bean-list-version-ui.test.js`
- API 测试：`go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPISupports(AllFulfillmentCustomerScope|CustomerScope)' -count=1`
- 仓储守卫：`go test ./internal/infrastructure/postgres/costing -run TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots -count=1`

## 验收结论
- [x] 版本列表范围下拉列出“公共豆单”和每个履约客户。
- [x] 客户豆单通过 API 按单个 `customer_id` 查询，不再汇总所有客户。
- [x] 生成豆单抽屉的发布归属未被删除，仍支持指定客户豆单。
