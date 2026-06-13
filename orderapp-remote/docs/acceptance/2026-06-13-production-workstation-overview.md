# PR-495-PRODUCTION-WORKSTATION-OVERVIEW 验收记录

## 范围
- 新增生产模块顶部切换条：生产视图、工位视图、生产计划、生产中、工单、工序卡、质检、日志、成本。
- 新增 `生产视图`，回答今日整体进度如何、哪里卡住、下一步谁处理。
- 新增 `工位视图`，回答当前工位现在要做什么、下一件做什么、为什么不能做。
- 新增生产工位 read model API，按工位、状态、阻塞原因、优先级聚合任务，并支持报异常、呼叫补料。

## 证据
- 单元测试：`go test ./internal/application/production -run TestProductionWorkstationOverviewAnswersProductionAndStationQuestions -count=1`
- API 测试：`go test ./internal/interfaces/http/production -run TestProductionWorkstationOverviewAPIAndStationActions -count=1`
- 前端测试：`node --test src/lib/production-workstation.test.js src/lib/menu-ia.test.js src/lib/view-routing.test.js`
- 后端相关包：`go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production`
- 前端构建：`npm run build`

## 浏览器验收项
- `生产视图`：顶部切换条可见；显示待处理、执行中、异常、工位负载；异常行显示阻塞原因和下一步处理人；可打开工单、库存作业、质检并保存工位/负责人/优先级。
- `工位视图`：按工位显示现在做、下一件和不能做原因；开始、暂停、继续、完成本工序、部分完成、报异常、呼叫补料按钮可见。
- `生产中`、`工单`、`工序卡`：页面保留原功能，并能通过顶部切换条进入生产视图和工位视图。

## 2026-06-13 development 部署验收
- `origin/develop`：`f5feaa03d125ff8913e74f16db32ff8f53be49d5`
- 部署：`./deploy_orderapp.sh development` 通过；镜像内 `go test ./...` 通过，`erp_orderapp` 重启成功。
- Smoke：`GET https://erp.qacoohee.com/app/vue-shell?view=productionOverview` 返回 200；`GET https://erp.qacoohee.com/api/production/workstation-overview?limit=2` 返回 200，包含 2 个今日工位任务、执行中状态汇总和工位负载。
- ERP 浏览器验收：临时短信登录后打开 `productionOverview`、`workstationView`、`produceRunning`、`workOrders`、`jobCards`。五个页面均未回登录页，顶部生产切换条完整，控制台错误 0。
- `生产视图` 实际页面信号：`今日生产总览`、`待处理`、`执行中`、`异常`、`工位负载`、`关键操作`、`打开工单`、`打开库存作业`、`打开质检`、`分配工位 / 调整优先级` 全部可见。
- `工位视图` 实际页面信号：`当前任务`、`下一件`、`阻塞原因`、`不能做原因`、`暂停`、`完成本工序`、`报异常`、`呼叫补料` 全部可见；当前线上任务均为执行中，`开始` 按钮由 API/UI 契约覆盖，不在当前数据中展示。
- 临时验收登录会话已通过页面退出和 `/api/auth/logout` 清理。
