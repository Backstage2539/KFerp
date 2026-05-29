# KFerp 通用制造模型实施计划

## 一期
- 统一口径：BOM 预期损耗/产出，工序卡实际投入/产出/损耗，工单展示快照。
- 测试：损耗换算、BOM 保存、工序卡损耗、工单汇总、成本兼容。
- 手册：生产、库存/BOM、成本手册同步。

## 二期
- 新增 `process_templates`、`process_template_operations`。
- Vue/Vite 新增“工艺模板”页面。
- 开工时冻结已发布工艺模板到工单，并按工艺路线生成工序卡。
- API/审计：模板保存、发布、停用必须写操作日志。

## 三期
- 新增 `industry_field_templates`、`industry_field_definitions`。
- Vue/Vite 新增“行业字段模板”页面。
- 工艺模板和工序卡使用 JSON 保存行业参数，通用字段保持结构化。

## 四期
- 实际成本、异常损耗分析和报表。
- 按理论成本、计划成本、实际成本分层展示。
- 维度包含 SKU、BOM 版本、工艺模板、工序、设备、人员和异常原因。

## 当前交付证据
- 二、三期代码：`internal/application/manufacturing`、`internal/infrastructure/postgres/manufacturing`、`internal/interfaces/http/manufacturing`。
- 前端：`ProcessTemplatesView.vue`、`IndustryFieldTemplatesView.vue`、`WorkOrdersView.vue`、`JobCardsView.vue`、`BomView.vue`。
- 手册：`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_INVENTORY_MATERIALS.md`。
- 验收：`docs/acceptance/2026-05-26-process-bom-workorder-sku-model-phase-2-3.md`。
