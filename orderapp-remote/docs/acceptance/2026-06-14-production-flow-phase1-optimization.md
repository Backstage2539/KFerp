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

## 待补证据
- development 部署 commit、smoke 和 ERP 浏览器验收结果。
