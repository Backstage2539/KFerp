# 包装部分完成后的备料显示与补料入口边界

日期：2026-10-26。范围：本地专用数据库 `kferp_release_20261026` 的隔离夹具；没有写业务数据库。

## 结论

四来源 `10 / 6 / 4 / 1` 袋合并包装，先完成 6 袋后，真实完成预览按剩余 15 袋计算，第二次完成、来源分摊、耗料和入库均正确；外层备料摘要仍按整单需求展示，出现已完成部分的虚假缺口。此问题在 `main 001c65b05485cca6ad5b9e241309bb1326039d04` 的相关实现中已存在，本次没有修改这部分业务逻辑。

这不是「部分完成后补料已验收通过」。对冻结 `planned_input_qty=0` 的旧包装卡，部分完成后从该入口进入领料预览，还会遇到「本批数量待核对，请先确认任务拆分」。应单独修正并验收这两个相邻问题。

## 观察与定位

首批完成 6 袋后，浏览器外层显示：需求 `4,767 g / 21 袋`、剩余可用 `3,405 g / 15 袋`、缺口 `1,362 g / 6 袋`。完成弹窗则显示剩余需 `3,405 g / 15 袋`，实际可用 `21,338 g / 94 袋`，提交 15 袋成功。最后四张工单全部完成，分别入库 `10 / 6 / 4 / 1` 袋，总耗熟豆 `4,767 g`、包装袋 `21` 个，共 5 个完成事件和 10 张已提交出入库单。

原因链：

- `internal/infrastructure/postgres/production/wip_reservation.go` 的 `GetWorkOrderWIPCoverage` 从完整冻结计划量生成需求，可用量取剩余预留，未从返回的整单需求扣本卡累计消耗。
- `internal/application/production/service.go` 的 `applyTaskBatchAndMaterialReadiness` 按计划批次分摊完整需求，跳过已完成卡，但未扣除仍未完成卡的部分消耗。
- `frontend-vue-shell/src/lib/production-workstation.js` 的 `workstationPackagingGroups` 直接累计材料 readiness；产出剩余量另行按计划减实际计算，两个展示口径因此不一致。

上述文件与该主分支基线对比无相关改动。

## 对补齐物料动作的影响

`WorkstationView.vue` 根据外层 shortage 启用「补齐物料」，再通过 `workstationCanOpenIssue` 仅选择 `stock_action=issue` 的任务。本场景实际选中卡 1。只读 API 预览复核结果是 `groups=[]`、卡 1 被阻止，提示「本批数量待核对，请先确认任务拆分」。

对应 `internal/infrastructure/postgres/production/batch_material_issue.go`：旧包装卡的零投入量回退，仅适用于尚无实际投入、产出和完成事件的未开始卡；部分完成后不再满足条件，`batchTaskReason` 返回上述原因。打开入口只请求领料预览，不会自动调拨或扣库存；本次没有通过浏览器提交这个部分完成后的补料动作。

诊断另行请求全部 4 张卡时，其余 3 张克隆卡因夹具未配置冻结来源批次绑定而显示来源资料待完善、可领 0。这不是该按钮实际筛选结果，也不能泛化成生产环境库存不足。

## 通过的独立安全与账务验证

- `TestReleaseFourSourcePackagingPartialRollbackAndReplay`：真实四来源 API `6 + 15`，分别重放同一请求；最终仍只完成 21 袋。最后一来源入库故障注入时，之前来源的事务写入全部回滚。材料成本合计 `402.36`，正式 `OperationLogMiddleware` 记录 4 次成功提交（含重放）和 1 次失败提交。
- `TestOperationCompletionAPIVariancesDraftAndRollback/shortage`：真实需 `22.7 kg`、WIP 仅 `10 kg` 时完成返回 400；无完成事件、耗料或入库写入。
- 既有来源领料测试另覆盖 `TestBatchMaterialIssueScarceStockAndReadOnlyPreview`、`TestBatchMaterialIssueGroupFailureDoesNotUndoOtherMaterial`、`TestBatchMaterialIssueGroupRollsBackEveryTaskOnPostingError` 及重试、不同来源和客户归属限制；见本批隔离 API 日志。

## 证据与限制

本次会话证据保存在工作树相邻 `evidence/` 目录：

- `packaging-browser-ledger.json`：浏览器完成的最终只读账务核对。
- `packaging-six-complete.png`、`packaging-fifteen-preview.png`、`packaging-all-complete.png`：浏览器观察。
- `packaging-after-six-api-previews.json`：部分完成后的真实完成、全部来源补料、按钮实际选中任务补料预览。
- `packaging-four-source-cost-audit.log`：非零成本、正式操作日志、回滚及幂等通过记录。
- `packaging-real-shortage-api.log`、`packaging-isolated-api.log`：真实缺料与来源补料通过记录。
- `packaging-partial-material-boundary.json`：结构化定位与边界。

初版浏览器包装夹具没有安装 HTTP 操作日志中间件，且批次成本为 0，因此该浏览器账务本身只证明 domain 审计与数量正确。正式操作日志及非零成本由上述后续独立 API 测试补证，不能混写成初版浏览器已观察的结果。
