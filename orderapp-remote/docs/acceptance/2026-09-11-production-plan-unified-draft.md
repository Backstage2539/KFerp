# PR-652 生产计划统一草稿入口验收

实施：2026-09-11 至 2026-09-12。功能分支：`codex/production-plan-unified-draft-20260911`，起点 `1ef28cdba86fd354f0e4bc3093ca2c3205f13e25`。

## 最终行为

- 创建前只有选需求、核对缺口；创建按钮明确为“创建草稿并编辑”，没有保存、提交、撤销草稿按钮。
- 创建成功、点击计划号和带 `production_plan_id` 的链接均调用同一详情加载入口，挂载 `ProductionPlanDetailWorkspace.vue`。创建后的旧第三步及重复按钮组移除。
- 持续显示“正在编辑草稿”。来源、工位批次、阻断定位、保存和提交都由详情工作区承接；刷新供应和撤销在“更多”。改订单需求须撤销重建。
- 创建前 URL 保存选择并重新校验；创建后 URL 保存单据身份并清理选择和旧预览任务。详情失败保留已创建计划号，只重试读取。
- 双击创建、请求重试、晚到的预览/详情、未保存离开、保存失败和版本冲突有回归保护。
- 提交后“查看生产工单”按计划关联工单 ID 读取，避免全局列表数量限制漏单；直接链接刷新保留计划范围。
- 无数据库迁移，无新增业务写接口；创建、保存、供应刷新、提交、撤销沿用原接口和操作日志。

## 自动验证

| 验证 | 结果与证据 |
| --- | --- |
| 首轮 TDD | 实际 Vue setup/handler 的初始回归产生 5 个失败，再实现统一入口；后续增加选择恢复和晚到详情用例，均先 RED 后 GREEN |
| 选择恢复 RED | 未计划摘要返回空 selected 覆盖 URL 选择；`/tmp/pr652-selection-red.log`，修复后保留选择并校验 |
| 历史竞态 RED | 晚到详情可能覆盖浏览器返回后的选择页；`/tmp/pr652-history-red.log`，用请求序号废弃旧请求 |
| 工单范围 RED | 新模块缺失时失败；`/tmp/pr652-workorders-red.log`；修复后覆盖超过全局列表上限、去重、状态筛选 |
| 定向行为回归 | 13 个实际 Vue handler 场景通过，`/tmp/pr652-behavior-final.log` |
| 完整标准门禁 | `scripts/verify_kferp.sh all` 通过，Go 全包、Vue 1190/1190、构建及 changed 检查；`/tmp/pr652-final-gates2.log` |
| 最终前端复验 | 1190/1190、0 失败，Vite 构建通过；`/tmp/pr652-final-frontend4.log` |
| 真 PostgreSQL/API | 7 个顶层定向测试及其子用例全部通过，0 skip；`/tmp/pr652-api2.log` |

数据库定向命令：

```sh
ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp&sslmode=disable' go test ./internal/interfaces/http/production -run 'TestProductionPlanRepositoryCreatesSubmitsAndStartsFormalLifecycle|TestProductionPlanDraftCancelAPI|TestProductionPlanDraftCancelRollsBack|Test.*ProductionPlan.*(Idempot|Refresh|Version)' -count=1 -v
```

原撤销 API 测试夹具缺少当前提交规则要求的来源仓，首次验证因此失败；仅补齐隔离夹具的 WIP 库存和草稿来源设置，没有放宽产品提交校验。

## 隔离业务与浏览器验证

使用真实 Vue 页面、现有生产 API handler 和本机 PostgreSQL 独立测试 schema。辅助登录/导航接口使用本地测试壳，不使用正式账号凭据。写操作未触碰开发环境业务数据。

| 场景 | 实际观察 |
| --- | --- |
| 浏览器选需求、核对缺口、创建 | 验收拼配咖啡 227g，10 袋与 8 袋两张订单；点击创建生成 PP-0000000001，直接进入统一详情且出现创建提示 |
| 刷新草稿 | 仍为 PP-0000000001；创建提示消失，持续显示正在编辑草稿与保存状态；没有旧第三步按钮 |
| 供应刷新与撤销 | 隔离 API 均返回 200；计划 1 保留 cancelled 状态，需求重新可选 |
| 重建与防重复 | 相同 request_id 重复创建都返回计划 2，仅一张新草稿 |
| 来源与工序安排 | 保存 WIP 来源仓、5 kg/批烘焙与 20 袋/批包装安排；readiness.can_submit=true |
| 提交与工单 | 隔离 API 提交计划 2 成功，生成 1 张工单、2 张工序卡；浏览器工单页显示来源计划及该 1 张工单，刷新后保持范围 |
| 操作日志 | 已查询 create、refresh_supply、cancel、save_draft、submit 对应审计记录 |
| 窄屏 | 390×844；整页宽度等于视口，更多不折字，编辑工位批次独立一行，底部保存/提交可见 |

接口请求状态记录：`/tmp/pr652-api-browser-evidence.json`；提交响应：`/tmp/pr652-submit-result.json`；就绪草稿：`/tmp/pr652-ready-plan.json`。

浏览器原生离开确认框能被触发，但电脑控制工具在该原生对话框上超时，无法稳定自动点击确认/取消。因此未把原生对话框点击、保存和提交全部声称为浏览器端到端通过；离开取消/确认、刷新保护由实际 Vue handler 回归验证，保存、撤销和提交由真实隔离 API 与数据库验证。

开发环境 PP-0000000113 只读对照，进入本次工作前已是“已取消”；本次没有改变它的状态或来源订单。

## 截图与设计核对

截图为实际运行页面，没有用渲染示意图替代验收：

1. `/private/tmp/kferp-pr652-acceptance/01-select.png`：选需求，18 袋、2 张订单。
2. `/private/tmp/kferp-pr652-acceptance/02-gap.png`：核对缺口，创建前核对及明确创建按钮。
3. `/private/tmp/kferp-pr652-acceptance/03-created-draft.png`：浏览器创建成功，计划 1。
4. `/private/tmp/kferp-pr652-acceptance/04-refreshed-draft.png`：刷新后的同一计划 1。
5. `/private/tmp/kferp-pr652-acceptance/05-narrow-draft.png`：390px 窄屏，隔离草稿计划 3。
6. `/private/tmp/kferp-pr652-acceptance/06-plan-work-orders.png`：计划 2 的关联工单。

设计比较见同目录 `2026-09-11-production-plan-unified-draft-design-qa.md`。手册单一来源 `docs/OP_MANUAL_PRODUCTION.md` 已同步，两步创建和撤销重建取代旧说明；Vue 手册使用现有文档服务。

## 交付边界

目标为 develop 集成和 development 部署，production 不在本次范围。PR-652 保持待 Van 产品验收；自动测试、隔离业务验证和实际截图由 Codex 执行。发布提交、回滚和上线只读检查在部署完成后补录。
