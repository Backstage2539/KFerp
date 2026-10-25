# PR-730 包装台四张小菠萝工单整改

## 目标与保护范围

将尾号 0017、0018、0019、0020 的兼容包装卡显示为一张工作卡：商品小菠萝、冻结规格 454g 袋装、合计 21 袋。按现有冻结资料汇总熟豆 9,534g（WIP 本组可用 9,534g）与包装袋 21 个（WIP 可用 0）。每张来源工单保留独立耗料、成本、成品入库单和操作日志；不改变销售客户、订单关系或批次总量。

本次不得在生产环境代现场填写并完工 21 袋。库存位置修复只有在生产最新预览与下述获批快照完全一致时才能提交：袋批次 ADJ-0000000004 变为包材仓 1,831 个、WIP 169 个，并且仅四张指定工单的 21 个预留从原料仓迁到包材仓；熟豆批次 MP-0000000002 只保留 WIP 11,627g 及其现有 WIP 预留。库存总量、成本与历史单据不改。

## 实现内容

- 用冻结商品、规格、单位、目标仓、物料身份、实际货主、来源仓及单位用量生成后端分组键；允许跨客户和 BOM 变体合组，阻止不同规格/货主/来源仓及同一工单多卡合组。
- 唯一旧包装卡、未执行、零计划投入的安全场景取工单冻结数量份额；多卡、已执行或冻结信息不完整时停止并显示异常。
- 包装组卡汇总需求、组内 WIP、缺口及只读来源仓可补数量；来源可补量来自原批量领料预览，提交仍走现有预留、质检、货主、批次与操作日志链路。
- 新增合并完成预览与提交接口。一次完成输入按工单编号分配，事务中分别调用工单完成记账链，任一失败回滚整组，版本、幂等键和请求摘要保护重试。
- 合组卡只显示“补齐物料”和“完成并入库”两个业务按钮，不显示重复领料菜单或“撤回并重新安排”；其他单卡工位流程保留。

## TDD 证据

RED 在 `origin/develop@4457e1e536c23a36596869594707428eab7b6c82` 的隔离归档上运行新测试：

- Go application RED：缺少 `AllocateOperationGroupOutput`、`OperationGroupMaxOutputQty`、分组字段与 `applyPackagingGroupKeys` 所需字段。
- Go API RED：缺少 `OperationCompletionGroupCommand/Preview/Result` 契约。
- Vue RED：新增分组完成测试因 `workstationGroupCompletionValidation` 未导出而无法加载。

GREEN（当前功能分支）：

- Go application：`go test ./internal/application/production -run 'Test(AllocateOperationGroup|OperationGroupMax|PackagingGroupKeys|PackagingGroup)' -count=1`：PASS。
- Go PostgreSQL production：`go test ./internal/infrastructure/postgres/production -run 'Test(DefaultMaterialUseScalesWithActualOverproduction|CompletionGroupIdentityKeepsProductionIdentityAcrossCustomerAndBOMVariants)' -count=1`：PASS。
- Go HTTP/PostgreSQL API：`DATABASE_URL=postgres:///postgres go test ./internal/interfaces/http/production -run 'Test(WorkstationCompletionGroupPreviewAndSubmitAPI|OperationCompletionAPIAtomicReceiptAndReplay|OperationCompletionAPIPartialOutputCanContinueAndFinish|OperationCompletionAPIIsolationAndFailureRetry)' -count=1`：PASS。
- Vue 全套：`node --test`：1,611/1,611 PASS。
- Vite production build：PASS；存在已有 PDFStampPreview 大 chunk 提示，不阻断构建。

完整 Go production 包曾联合运行，未全绿：既有数据库测试缺少 `product_bom_spec_authorities` 表、供料 fixture 数量不足及旧 PR-655 断言与当前文案不一致。相关组功能的 application、PostgreSQL 及 API 针对性测试通过；整包失败需在部署前以基线复核记录，不应标成全绿。

## 浏览器和环境验收

待补充：隔离开发数据的浏览器按钮验收（21 袋分为 10/6/4/1；分两次 6+15；补料批次来源；整组回滚和响应丢失重试）、开发部署及健康检查、干净 main 生产发布、生产页面只读检查、两批库位修复预览/提交及审计结果。

业务边界：生产包装实数仍由现场人员填写；生产只读页面验证不得点击任何提交、补料、完工、入库或库位修复按钮。Van 最终验收待确认。
