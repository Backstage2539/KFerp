# PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER 验收记录

## 范围
- 本次只处理生产计划页面单据流：`选择缺口商品 -> 创建生产计划 -> 在生产计划列表勾选计划 -> 提交生成工单`。
- 不处理通用制造抽象，不改生产 BOM、工艺路线、工序、工位模型，不改旧 `/api/produce/start` 兼容入口。

## 验收场景
1. 在生产计划页面选择一个库存不足商品，点击 `创建生产计划`。
2. 生产计划列表显示新建计划，状态为 `草稿`，创建动作不自动生成生产工单。
3. 用状态和时间过滤能找到该计划。
4. 勾选该草稿计划，点击列表顶部 `提交生成工单`。
5. 列表刷新后该计划状态显示为 `已提交工单`，复选框置灰。
6. 进入生产工单页可看到对应 `released` 工单，工序卡为 `pending`。

## 自动化覆盖
- 前端：`produce-plan.test.js` 覆盖旧 `生成计划` 删除、行内 `提交` 删除、草稿计划选择三态、批量提交按钮 payload、状态中文和颜色 key、过滤 URL 参数。
- 服务/API：`service_flow_test.go` 和 `work_order_api_test.go` 覆盖生产计划过滤、默认最近 50 条、批量提交部分成功、非草稿失败、重复 ID 不重复生成工单。
- 仓储：`production_plan_static_test.go` 覆盖 `created_at/submitted_at/completed_at` 时间过滤和 `completed_at` 返回字段。
- 支撑契约：`dev_473_production_plan_bulk_submit_filter_test.go` 覆盖 PR/DEV/UT/API/REV 种子、接口、Vue、手册和验收文档同步。

## 手工验收要点
- `创建生产计划` 保持创建草稿，不自动提交。
- 生产计划列表顶部 `提交生成工单` 未选择草稿计划时置灰。
- 非草稿计划不可勾选，且批量接口返回失败明细时不重复生成工单。
