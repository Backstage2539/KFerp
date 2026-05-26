# KFerp 通用制造模型分阶段实施计划

日期：2026-05-26
需求：PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL

## 总体策略

一期不大改生产流程，只统一损耗/产出率口径并补齐工序卡实际损耗记录。保留 `product_bom.yield_rate` 作为兼容存储，BOM API 和 Vue 页面主展示“预期损耗率”，同步显示“预期产出率”。二期再引入工艺模板和工艺路线，三期做行业字段模板，四期做实际成本和异常损耗分析。

## PR / DEV 拆分

### PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL

目标：SKU、BOM、工艺、工序、工单、模板之间形成通用制造模型边界；一期落地 BOM 预期损耗率、工序卡实际损耗和工单快照展示。

验收口径：

- SKU/BOM/成本/生产页面不再只表达咖啡“出品率”。
- BOM 可维护预期损耗率，并显示换算的预期产出率。
- BOM API 保留 `yield_rate`，新增 `expected_loss_rate` 和 `expected_yield_rate`。
- 工序卡 API 和页面可记录实际投入、实际产出、实际损耗、实际损耗率和异常原因。
- 工单 API 和页面展示冻结的预期损耗、预期产出率、计划投料、预计产出和实际损耗汇总。
- 用户触发保存 BOM 损耗率、保存工序卡实际损耗必须写操作日志。
- 操作手册和验收证据完整。

### DEV-375-01-BOM-EXPECTED-LOSS

内容：

- 增加损耗率/产出率双向换算单元测试。
- BOM API 详情、列表、版本返回 `expected_loss_rate`、`expected_yield_rate`。
- `/api/bom/save` 接收 `expected_loss_rate` 并换算保存到 `yield_rate`。
- 保存 BOM 损耗率写 `product_bom` 操作日志。
- BOM Vue 页面把出品率文案改为预期损耗率/预期产出率。

验证：

- `go test ./internal/domain/production -run 'TestExpectedLossRate|TestYieldRateFromExpectedLossRate|TestActualLoss' -count=1`
- `go test ./internal/interfaces/http/bom -run 'TestBomDetailExposesExpectedLossAndYield|TestBomSaveAcceptsExpectedLossRate|TestBomSaveRejectsInvalidExpectedLossRate' -count=1`
- `go test ./internal/application/bom -count=1`

### DEV-375-02-JOB-CARD-ACTUAL-LOSS

内容：

- 扩展 `job_cards` 表实际损耗字段。
- 增加工序卡实际投入/产出/损耗计算单元测试。
- `GET /api/produce/job-cards` 返回实际损耗字段。
- 新增工序卡实际数据保存 API。
- 保存工序卡实际数据写操作日志。
- 生产完工时把实际投入、实际产出、实际损耗同步到默认工序卡。

验证：

- `go test ./internal/domain/production -run 'TestJobCardActuals|TestActualLoss' -count=1`
- `go test ./internal/interfaces/http/production -run 'TestJobCardAPI' -count=1`
- `go test ./internal/infrastructure/postgres/production -run 'Test.*JobCard|Test.*WorkOrder' -count=1`

### DEV-375-03-WORK-ORDER-SNAPSHOT-SUMMARY

内容：

- 工单 API 返回 `expected_loss_rate`、`expected_yield_rate`、`operation_summary_json`。
- 工单列表和打印工单展示 BOM 预期损耗、计划投料、预计产出和实际损耗汇总。
- 生产计划/生产日志/成本核算文案从“出品率/烘焙得率”泛化为“预期产出率/实际产出率”。

验证：

- `go test ./internal/interfaces/http/production -run TestWorkOrderAPIIncludesExpectedLossAndOperationSummary -count=1`
- `go test ./internal/domain/costing ./internal/interfaces/http/costing -count=1`
- `npm --prefix orderapp-remote/frontend-vue-shell run build`

### DEV-375-04-DOCS-REQ-ACCEPTANCE-BROWSER

内容：

- 更新 `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`。
- 更新 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`。
- 更新 `orderapp-remote/docs/OP_MANUAL_COSTING.md`。
- 更新 `REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`。
- 更新 `ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`。
- 增加 `docs/acceptance/2026-05-26-process-bom-workorder-sku-model.md` 和 `orderapp-remote/docs/acceptance/2026-05-26-process-bom-workorder-sku-model.md`。
- 更新 PR/DEV 种子。
- 使用浏览器跑通 BOM 维护、工序卡实际损耗、工单查看流程。

验证：

- `go test ./internal/interfaces/http/support -run TestDev375 -count=1`
- `scripts/verify_kferp.sh changed`
- 浏览器访问 Vue/Vite 页面并完成流程截图或记录。

## 分期路线

### 第一期：统一口径，不大改生产流程

交付：

- 预期损耗率优先。
- 兼容 `yield_rate`。
- 工序卡实际损耗字段和保存 API。
- 工单实际损耗汇总展示。
- 手册、验收证据、浏览器流程验证。

### 第二期：工艺模板 / 工艺路线

交付：

- 新增 `process_templates`。
- 新增 `process_template_operations`。
- 新增 Vue/Vite “工艺模板”页面。
- 工单开工时冻结工艺模板和工序路线。
- BOM 页展示关联工艺模板入口。

### 第三期：行业字段模板

交付：

- 新增行业字段定义表。
- 工艺模板和工序卡按行业模板渲染参数。
- 咖啡、服装、鲜果加工字段通过模板配置，不硬编码到通用模型。

### 第四期：实际成本与异常损耗分析

交付：

- 成本报表区分理论成本、计划成本、实际成本。
- 异常损耗原因统计。
- 按产品、工序、操作人、设备分析损耗。

## 一期测试计划

单元测试：

- `loss = 1 - yield` 双向换算。
- 非法损耗率拒绝。
- 工序卡实际损耗和损耗率计算。
- 工单多工序卡实际损耗汇总。
- 成本核算仍按兼容后的预期产出率计算。

API 测试：

- `GET /api/bom/detail/:id` 返回损耗率和产出率。
- `POST /api/bom/save` 保存损耗率。
- `GET /api/produce/job-cards` 返回实际损耗字段。
- 保存工序卡实际损耗。
- `GET /api/produce/work-orders` 返回预期损耗和工序汇总。

前端验证：

- `npm --prefix orderapp-remote/frontend-vue-shell run build`
- 浏览器打开 BOM、工序卡、工单页面，维护并查看损耗字段。

部署：

- 功能分支验证后推送。
- 更新到最新 `origin/develop` 后重跑验证。
- 合并到 `develop`。
- 部署 development stack。
- 做 post-deploy smoke。
