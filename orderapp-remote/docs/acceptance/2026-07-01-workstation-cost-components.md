# PR-514 工位成本组件与产能批量折算工序成本验收记录

## 范围
- 工位/设备维护 `机器成本/小时`、`人工成本/小时`、`其他成本/小时`，系统合计为小时成本。
- 工位产能只维护标准批量、单位、标准分钟/批、状态和备注；旧 `hourly_rate` 只做兼容兜底。PR-517 后适用工序改由工位/设备统一维护。
- 工艺路线只作为模板维护工序顺序、损耗记录要求和质检项，不选择工位产能，也不保存 `计划工序成本`。
- 生产计划草稿/未开工工单拆分选择工位产能后，按 `工位小时成本 × 标准分钟/60 × 批次数` 自动折算 `计划工序成本`。
- 商品价格试算可显示当前工艺路线作为生产路径模板说明，但不从路线模板读取计划工序成本；生产计划拆分和工单继续冻结本次拆分的总工序成本。

## 公式验收
- 工位成本组成：`机器 2 + 人工 2 + 其他 1 = 5 元/小时`。
- 工位产能：`标准批量 5 kg`，`标准分钟 60`。
- 工艺路线单位工序成本：`5 × 60/60 ÷ 5 = 1 元/kg`。
- 生产成本数量单位使用库存/产能单位，不使用销售单位。

## RED 证据
- `cd orderapp-remote && go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing -count=1`
  - 预期失败：缺少 `MachineHourlyCost/LaborHourlyCost/OverheadHourlyCost` 字段，schema 缺少 `machine_hourly_cost NUMERIC(14,4) NOT NULL DEFAULT 0`。
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/process-routes.test.js`
  - 预期失败：工位页仍显示旧 `默认小时费率`，工位产能仍暴露可编辑 `hourly_rate`。
- 2026-07-01 follow-up RED：Van 指出工位产能只能在生产计划之后选择，工艺路线只是模板。
  - `go test ./internal/application/manufacturing ./internal/interfaces/http/support -count=1`
  - `node --test src/lib/process-routes.test.js`
  - 预期失败：工艺路线页面仍加载/显示 `/api/manufacturing-workstation-capacities` 和 `工位产能`；后端保存路线仍把 `workstation_capacity_id` 折算成 `planned_operation_cost`；价格试算仓储仍读取 `process_route_operations.planned_operation_cost`。

## GREEN 证据
- `cd orderapp-remote && go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing -count=1`
  - 通过。
- `cd orderapp-remote && go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1`
  - 通过。
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/product-settings.test.js src/lib/produce-plan.test.js src/lib/process-routes.test.js`
  - 通过，192 个 frontend helper/source tests。
- `npm run build`
  - 通过；仅有 Vite chunk size warning。
- `scripts/verify_kferp.sh changed`
  - 通过，无额外输出。
- `git diff --check`
  - 通过。

## 待补充
- development 部署后 smoke
