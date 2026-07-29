# PR-562 生产执行入口与工序卡职责收敛验收记录

## 需求

- PR：`PR-562-PRODUCTION-EXECUTION-JOB-CARD-CONSOLIDATION`
- DEV：
  - `DEV-562-WORKORDER-EXECUTION-HUB-COMMAND`
  - `DEV-562-JOB-CARD-READONLY-PROJECTION`
  - `DEV-562-WORKSTATION-STATE-ACTUALS`
  - `DEV-562-DOCS-DELIVERY`
- REV：`REV-562-PRODUCTION-EXECUTION-JOB-CARD-CONSOLIDATION`
- 环境边界：只部署 development，不部署 production；真实工单 `WO-PP-0000000083-0000000051` 只做只读冒烟，不自动执行开工、暂停、继续、完成或库存写入。

## 验收口径

- [x] 工单列表删除直接“开始生产”和“完工入库”，工单号和“执行枢纽”进入同一上下文；未执行 released 工单仍可编辑拆分，并保留打印。
- [x] 执行枢纽以 `action_type=command` 原地调用工单开工接口，遵守 readiness、阻断原因、全动作 busy、防重复提交、中文错误和成功刷新。
- [x] 工序卡只读展示冻结工序要求、状态、计划/实际分钟与成本、实际损耗及原因、异常原因、状态时间和操作人；没有状态按钮、输入框或“保存实际”。实际投入、实际产出和余料留在工位完成记录中，不作为工序卡列表展示字段。
- [x] 工序卡“进入工位”携带工单与工序卡上下文定位任务，“执行枢纽”打开所属工单。
- [x] 工位严格按服务端 `available_actions` 展示状态动作，并完成 `pending → running → paused → running → completed` 闭环；运行中仍可保留服务端返回的“报异常”“呼叫补料”等非状态业务动作。
- [x] 完成本工序一次提交实际分钟、投入产出、余料、损耗、异常/备注和适用仓库；投入、产出和余料使用冻结库存单位，成品件数按冻结每件库存数量换算，产出与件数互斥；非法、重复和并发动作不改变状态。
- [x] 工单开工及每次工序状态转换写操作日志；历史接口和历史业务数据保持兼容。
- [x] 生产执行读取要求 `production.read`，工单和工序状态写入要求 `production.run`；只读角色不能到达写处理器。

## 计划验证

### 前端定向

```bash
cd orderapp-remote/frontend-vue-shell
node --test \
  src/lib/production-execution-hub.test.js \
  src/lib/production-workstation.test.js \
  src/lib/work-orders.test.js \
  src/lib/manufacturing-execution.test.js
```

- 结果：通过，定向前端测试 `40/40`。

### 应用、API 与支持合同

```bash
cd orderapp-remote
go test ./internal/application/production \
  ./internal/interfaces/http/production \
  ./internal/interfaces/http/support -count=1
```

- 结果：通过，定向 Go 与旧支持合同全部为绿色。

### 临时 PostgreSQL 状态与审计闭环

```bash
cd orderapp-remote
ORDERAPP_TEST_DATABASE_URL=<temporary-postgresql> \
go test -vet=off ./internal/interfaces/http/production \
  -run 'TestPR562JobCardRequirementAndExecutionCommandAPIContracts|TestProductionPlanRepositoryCreatesSubmitsAndStartsFormalLifecycle|TestLegacyProduceStartAPIUsesTemporaryPlanAndStillStartsProduction' \
  -count=1 -v
```

- 结果：通过，PR-562 API 合同、正式生产计划生命周期和旧开工接口兼容三个临时 PostgreSQL 测试均为 `PASS`。

### 完整验证与构建

```bash
scripts/verify_kferp.sh backend
scripts/verify_kferp.sh frontend-tests
scripts/verify_kferp.sh frontend-build
```

- 结果：
  - 完整 `go test ./...`：通过。
  - Vue/Vite 构建：成功。
  - 完整前端：功能分支 `825/832`，干净 `origin/develop` 基线 `820/827`；两边 7 项失败名称完全一致，PR-562 新增测试全部通过。

### development 只读冒烟

- [ ] 工单列表入口和按钮符合新职责。
- [x] 指定工单详情 API 的执行枢纽返回 command 开工、正确 endpoint 及 readiness 阻断。
- [x] 指定工单工序卡返回冻结工序要求；当前工序卡已完成，不再进入可执行工位队列。
- [x] 当前工位任务均由服务端返回 `available_actions`，且不再出现旧 `partial_finish` 动作。
- [x] 容器运行、重启次数、需求记录、API、Vue shell 和 `index-DYYLfyWR.js` 均正常，容器日志无新增错误。
- [ ] 应用内浏览器因开发站点 `ERR_CERT_AUTHORITY_INVALID` 未进入页面，未绕过证书，浏览器控制台和页面布局待登录浏览器验收。
- [x] 未对指定真实工单或真实库存执行写操作。

## 手册

- `docs/OP_MANUAL_PRODUCTION.md`
- `docs/REQUIREMENTS.md`
- `docs/ACCEPTANCE_TESTS.md`

## 最终交付

- 功能分支：`codex/pr562-production-execution-entry`，提交 `835d38c8b080a883b1937acac292a2766e7eb911`。
- `develop` 合并提交：`ac21a5964f0cf24e098c19105d112b5b0c856e1a`。
- development 备份：`/opt/stacks/erp/orderapp.backup.deploy-20260730001320`。
- development 部署：成功；`erp_orderapp` 运行且重启次数为 0，部署镜像完整 Go 测试通过，Vue shell 与 `index-DYYLfyWR.js` 返回 200。
- 只读业务冒烟：PR-562 需求记录可见；指定工单详情、执行枢纽 command、冻结工序要求和当前工位动作合同通过；未调用任何状态或库存写接口。
- 浏览器：开发证书触发 `ERR_CERT_AUTHORITY_INVALID`，页面级验收未通过且未绕过。
- production：不部署。
