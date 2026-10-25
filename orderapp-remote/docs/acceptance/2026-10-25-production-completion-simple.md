# PR-731 工序完成与旧工单入库简化

状态：实现、本地全量验证与构建通过；计划备料、开发部署和隔离浏览器按钮验收待完成。生产未改动，生产验证保持只读。

## 交付范围

- 工单列表“确认数量并入库”改为使用已记录的累计完成数量。前端请求只携带版本和固定请求标识，服务端锁定工单后补齐缺少的耗料、成本和入库；保留原工序卡的完成数量与时间。
- 智烘及同类简化工序卡只显示“工序完成”和“查看工单”。表单只显示本批产出数量，不要求重新输入投料、用料、时间或差异原因。
- 烘焙消耗按本批实际投料乘冻结 BOM 配比计算，绝不按产出量反推投料。配方不完整时返回“BOM 配料比例合计 70%，请在 BOM 中补齐后继续”并提供 BOM 入口。
- 计划备料显示旧单的只读 WIP 覆盖和缺口；补料入口带入工单并转到库存作业。库存不足不会跨仓补货或占用他单预留。

## 已完成验证

- TDD RED/GREEN：旧入库请求和 UI 合约测试在实现前失败；自动投料/BOM 比例测试在函数缺失时失败；实现后均转绿。
- Vue 定向运行：`node --test src/lib/operation-completion-dom.test.js src/lib/production-workstation.test.js src/lib/work-order-roster-runtime.test.js src/lib/roasting-workstation-runtime.test.js src/lib/recorded-output-receipt-runtime.test.js`，41/41 通过。
- Vue 全量运行：`node --test`，1612/1612 通过。
- Go 全量运行：`go test ./...`，全部通过；包含旧计划备料可用量/缺口只读投影测试。
- `npm run build` 通过；Vite 仅报告已有大 chunk 提示。`git diff --check` 通过。
- PostgreSQL/API 定向运行：`go test ./internal/interfaces/http/production -run 'TestRecordedOutputReceiptUsesSavedQuantityAndReplaysAfterItLeavesPendingList|TestLegacyReview' -count=1` 通过。隔离数据库断言记录数量保持 14、成品入库 14、耗料只过账一次、同请求重试返回原结果。
- Go 生产应用与数据库包定向测试通过：`go test ./internal/application/production ./internal/infrastructure/postgres/production -run 'TestRoasting|TestOperationCompletion|TestLegacyReview'`。
- Vue/Vite 生产构建通过。Vite 仅报告现有大 chunk 提示。

## 待完成

- 开发部署检查、分支推送/合并到 develop。
- 在开发环境用隔离数据通过浏览器按钮走通补料、自动烘焙扣料、烘焙产出入库、包装合组及发货；核对库存、耗料、成本、单据和日志。
- 浏览器验收通过后才集成 main 并发布生产。生产版本、资源和运行状态只读确认，不对真实工单执行补料或入库。
