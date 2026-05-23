# PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW

## 需求
- 发布新版豆单后旧版仍保持已发布，不能自动撤回。
- 撤回只能通过版本列表手动撤回。
- 录单只能选择已发布豆单，已撤回豆单不能作为新订单豆单版本。

## 验收口径
- 连续发布同一归属、同一豆单类型的两个版本后，两条记录都保持 `published`，版本列表按发布时间展示最新在前。
- 手动撤回某个版本后，该版本状态变为 `withdrawn`，历史快照和 PDF 下载仍可追溯。
- 录单豆单选择器只返回 `published` 版本；已撤回豆单不出现在录单豆单选择器。
- 保存订单时显式提交已撤回豆单 ID，接口返回 `invalid bean list publication`，不会把撤回版本写入订单。

## 证据
- 单元测试：`go test ./internal/infrastructure/postgres/costing -run 'TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots|TestPublishBeanListDoesNotWithdrawExistingPublishedSnapshots' -count=1`
- 单元测试：`go test ./internal/infrastructure/postgres/orderbeans -run TestExplicitPublicationSelectionRequiresPublishedSnapshots -count=1`
- API/仓储测试：`go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormBeanListVersionOptionsUseOnlyPublishedSnapshots|TestOrderSaveExplicitBeanListPublicationRequiresPublishedSnapshot' -count=1`
- 数据库 API 测试：`go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormHidesWithdrawnPublicBeanListVersionsForFallbackCustomer|TestOrderAPIRejectsWithdrawnPublicBeanListPublicationVersion' -count=1` 在有测试数据库时覆盖表单过滤和保存拒绝。
- 手册：`OP_MANUAL_COSTING.md`、`OP_MANUAL_ORDER_SALES.md`
