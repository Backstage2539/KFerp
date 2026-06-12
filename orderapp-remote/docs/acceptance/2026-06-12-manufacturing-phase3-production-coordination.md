# 制造三期生产协调层验收记录

范围：PR-480 到 PR-484。

## PR 列表
- PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY：排程字段、工作中心产能日历和冲突提示。
- PR-481-MANUFACTURING-PHASE3-SCHEDULING-WORKBENCH：生产排程工作台，含列表、日历、甘特和工位负载视图。
- PR-482-MANUFACTURING-PHASE3-MRP-SUGGESTIONS：MRP 采购建议和调拨建议，只读建议，不自动生成采购单或调拨单。
- PR-483-MANUFACTURING-PHASE3-INDUSTRY-CALCULATORS：行业字段模板页的通用制造计算预览，支持咖啡烘焙、包装盒和童装。
- PR-484-MANUFACTURING-PHASE3-TRACEABILITY-ANALYTICS：生产成本页的追溯链路、成本差异和异常损耗看板。

## 验收场景
1. 在生产排程工作台按日期和工作中心查询工单，切换列表、日历、甘特和工位负载视图。
2. 给一个 released 工单保存计划开始/结束、班次、负责人、优先级和工作中心；保存工作中心当天产能后刷新，页面展示冲突提示。
3. 在同一排程工作台查看 MRP 区块，确认可看到采购建议、调拨建议、需求量、WIP 可用量、原料仓库存和来源工单。
4. 进入设置 / 行业设置，选择咖啡烘焙、包装盒、童装任一预设，输入需求产出、损耗率、原料单价和工时，点击计算预览，确认返回计划投入、预计损耗和预计成本。
5. 进入生产成本页，按工单 ID 或生产批次过滤，确认可看到成本差异、异常损耗和追溯链路；追溯链路包含工单、工序卡、Stock Entry、批次和数量。

## RED Evidence
- PR-480/481 后端 RED：排程字段、产能日历、排程 API 类型和 schema marker 缺失。
- PR-481 前端 RED：`productionSchedule` 菜单、`ProductionScheduleView.vue`、排程 helper 和视图 marker 缺失。
- PR-482 RED：`MRPSuggestionQuery`、`MRPSuggestions`、`GET /api/mrp/suggestions` 和 MRP 前端入口缺失。
- PR-483 RED：`PreviewIndustryCalculator`、`POST /api/industry-calculators/preview` 和行业设置计算预览入口缺失。
- PR-484 RED：`ProductionTraceAnalytics`、`GET /api/production-trace/analytics` 和生产成本页追溯/差异/异常损耗入口缺失。

## GREEN Evidence
- `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -run 'TestServiceOwnsManufacturingPhase3ScheduleCapacity|TestServiceRejectsInvalidManufacturingPhase3ScheduleCommands|TestManufacturingPhase3ScheduleCapacityAPIs|TestManufacturingPhase3ScheduleSchemaCreatesCapacityAndScheduleFields' -count=1`
- `node --test src/lib/production-schedule.test.js src/lib/menu-ia.test.js`
- `go test ./internal/application/production ./internal/interfaces/http/production -run 'TestServiceOwnsManufacturingPhase3MRPSuggestions|TestServiceRejectsInvalidManufacturingPhase3ScheduleCommands|TestManufacturingPhase3MRPSuggestionAPI' -count=1`
- `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing -run 'TestIndustryCalculatorPreviewUsesGenericManufacturingFormula|TestIndustryCalculatorPreviewRejectsInvalidInput|TestIndustryCalculatorPreviewAPI' -count=1`
- `go test ./internal/application/production ./internal/interfaces/http/production -run 'TestServiceOwnsManufacturingPhase3TraceAnalytics|TestServiceRejectsInvalidManufacturingPhase3ScheduleCommands|TestManufacturingPhase3ProductionTraceAnalyticsAPI' -count=1`
- `node --test src/lib/industry-field-templates.test.js src/lib/production-costs.test.js`

## Notes
- 三期不做自动产能优化、不自动生成采购单、不回改已发布价格表快照。
- 用户触发的排程保存和产能保存写操作日志；MRP 和追溯分析是读取型建议/看板。
