# PR-486 工艺路线工时费率归属验收记录

## 范围
- 工艺路线工序行是计划批量、标准分钟/批、小时费率和计划工序成本的权威位置。
- 工位产能用于给路线行带出默认值，保存后成为路线行快照。
- 生产计划提交生成工单时，工序卡冻结路线行计划值；实际分钟和实际工序成本记录在工序卡。

## 验收场景
1. 在 `生产管理 -> 工位/设备` 创建 `布勒烘焙机`，设置默认小时费率。
2. 为该工位创建工位产能 `布勒 18kg`、`布勒 15kg`、`布勒 6kg`。
3. 在 `生产管理 -> 工序` 创建 `烘焙`，确认页面不显示默认分钟。
4. 在 `生产管理 -> 工艺路线` 选择 `烘焙 + 布勒烘焙机 + 布勒 18kg`，确认带出标准批量、标准分钟/批和小时费率。
5. 创建生产计划并提交生成工单。
6. 在生产工单页确认工序摘要显示工位产能、计划分钟和计划工序成本。
7. 在工序卡页录入实际分钟并保存实际，确认实际工序成本显示。

## 验证命令
- `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/production ./internal/application/production ./internal/interfaces/http/support -run 'TestSaveWorkstationCapacityNormalizesReusablePreset|TestSaveProcessRouteSnapshotsWorkstationCapacityValues|TestSaveProcessRouteRejectsCapacityFromDifferentWorkstation|TestWorkstationCapacityAPIListSaveAndDeactivate|TestManufacturingSchemaAddsWorkstationCapacitiesAndRouteCostSnapshots|TestJobCardsSchemaFreezesRouteOperationTimeAndCost|TestWorkOrderFreezesRouteCapacityTimeAndCostIntoJobCards|TestDev486WorkstationCapacityRouteCostContracts' -count=1`
- `node --test src/lib/process-routes.test.js src/lib/work-orders.test.js src/lib/production-costs.test.js`
