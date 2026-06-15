# PR-496 生产管理高频视图优化一期验收记录

## 范围
- 顶部生产切换条保持 `生产视图 / 工位视图 / 生产计划 / 生产中 / 工单 / 工序卡 / 质检 / 日志 / 成本` 顺序，并在生产视图、工位视图、生产中展示待处理/阻塞/执行中 badge。
- 生产 overview/read model 返回 `today_summary`、`nav_badges`、任务 readiness、阻塞原因和下一处理人，生产视图和工位视图不只靠状态字符串推断卡点。
- 生产计划页增加五步步骤条、sticky 下一步按钮和提交工单后的下一步面板。
- 生产中页将完成/部分完成收敛到统一完成面板，展示 WIP 不足或质检冻结的原因、影响对象和动作入口。
- 从生产视图或生产中打开库存作业时带入 WIP 上下文参数，预填工单、工序卡、running item、物料和缺口数量。

## RED
- `node --test src/lib/production-workstation.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js`：实现前失败，因为 `navItemsWithProductionBadges`、`stockOperationContextParams`、`productionPlanSteps`、`currentProductionPlanStep`、`buildProductionPlanNextActions`、`buildFinishPanelModel` 和 `productionFinishErrorDetail` 未导出或未实现。
- `go test ./internal/application/production -run TestProductionWorkstationOverviewAnswersProductionAndStationQuestions -count=1`：实现前失败，因为 overview read model 未提供 `today_summary`、`nav_badges`、任务 readiness。
- `go test ./internal/interfaces/http/production -run TestProductionWorkstationOverviewAPIAndStationActions -count=1`：实现前失败，因为 API 响应缺少 `today_summary`、`nav_badges`、`readiness`。

## GREEN
- `node --test src/lib/production-workstation.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js`：43/43 passed。
- `go test ./internal/application/production -run TestProductionWorkstationOverviewAnswersProductionAndStationQuestions -count=1`：passed。
- `go test ./internal/interfaces/http/production -run TestProductionWorkstationOverviewAPIAndStationActions -count=1`：passed。
- `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`：passed。
- `node --test src/lib/production-workstation.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js src/lib/menu-ia.test.js src/lib/view-routing.test.js`：68/68 passed。
- `npm run build`：passed，保留既有 chunk size warning。
- `scripts/verify_kferp.sh changed`：passed。
- `git diff --check`：passed。

## Development 部署
- Feature branch：`codex/production-flow-phase1-20260614` 已推送。
- develop 集成：PR-496 代码已进入 `origin/develop`；并发工作流推进 develop 后，已在 `origin/develop=a2f2d9b4c2213b2e73b6c4df2b895dd3b4b6cfdc` 基线上复验通过。
- 初次 development 部署：`root@1.12.242.58:/opt/stacks/erp`，备份目录 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260614231948`。
- Docker build：镜像构建阶段执行 `go test ./...` passed，`erp_orderapp` 已重新创建并启动。最终 evidence 提交合入 develop 后，再按最新 `origin/develop` 执行 development 部署复核。
- Server smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` 运行；`erp_orderapp` 日志包含 `orderapp listening on :8080`。
- Authenticated smoke：生产视图、工位视图、生产计划、生产中、工单、工序卡页面均返回 200；`/app/api/production/workstation-overview?limit=5` 返回 `nav_badges`、`today_summary`、`readiness`、`workstation_load`；需求 API 可查到 `PR-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION`。

## ERP 浏览器验收
- 工具：优先尝试 Codex in-app Browser，因 webview attach timeout，改用 headless Chrome CDP 执行同等浏览器验收。
- 页面：生产视图、工位视图、生产计划、生产中、工单、工序卡。
- 结果：6 个页面均验证顶部生产切换条顺序为 `生产视图 / 工位视图 / 生产计划 / 生产中 / 工单 / 工序卡 / 质检 / 日志 / 成本`，切换条为 sticky，当前页面 active 高亮存在，生产视图/工位视图/生产中 badge 存在。
- 关键页面文案：生产视图包含 `今日生产总览`、`待处理`、`执行中`、`异常`、`工位负载`、`阻塞原因`；工位视图包含 `工位视图`；生产计划包含 `选需求`、`生成草稿`、`拆分产能`、`提交工单`、`开始生产`、`下一步`；生产中包含 `生产中`、`开始时间`、`批次`、`计划摘要`、`主动作`；工单包含 `生产工单`；工序卡包含 `工序卡`。
- Overview API 浏览器内校验：`today_summary`、`nav_badges`、`status_summary`、`blocked_summary`、`priority_summary`、`workstation_load`、任务 `readiness`、`next_handler`、`blocking_reason` 均存在。
- Console/runtime：0 errors。

## 业务口径
- 未改变生产计划创建、拆分、提交、完工入库、Stock Entry、WIP 或质检冻结核心业务规则。
- 本次实现未发现需要私自调整的完工入库、Stock Entry、WIP、质检冻结口径冲突；如后续需要改变冻结/入库判定，应另列为“需要业务确认”。
