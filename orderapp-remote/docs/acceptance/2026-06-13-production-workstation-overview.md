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
