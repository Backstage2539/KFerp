# 验收记录：豆单版本列表范围下拉收敛

## 需求
- 产品豆单的“豆单版本列表”范围下拉只保留“公共豆单”和“所有履约客户豆单”。
- 版本列表查看范围和生成豆单抽屉的发布归属分开；生成豆单仍可发布公共豆单或指定客户豆单。
- “所有履约客户豆单”只汇总有效履约客户归属的客户豆单发布记录。

## 实现
- `CostingView.vue` 新增独立 `versionListScope`，版本列表下拉只渲染 `公共豆单` 和 `所有履约客户豆单`。
- `GET /api/costing/bean-list/publications?scope=fulfillment_customers` 新增所有履约客户豆单查询语义。
- 版本列表中的“生成新版”和“撤回”按行自身 `owner_type/owner_key` 设置或提交归属，避免汇总列表误用当前发布归属。

## 验证
- 单元测试：`node --test src/lib/costing-bean-list-version-ui.test.js`
- API 测试：`go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPISupports(AllFulfillmentCustomerScope|CustomerScope)' -count=1`
- 仓储守卫：`go test ./internal/infrastructure/postgres/costing -run TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots -count=1`

## 验收结论
- [x] 版本列表范围下拉只有“公共豆单”和“所有履约客户豆单”。
- [x] 所有履约客户豆单通过 API 汇总 customer owner 发布记录。
- [x] 生成豆单抽屉的发布归属未被删除，仍支持指定客户豆单。
