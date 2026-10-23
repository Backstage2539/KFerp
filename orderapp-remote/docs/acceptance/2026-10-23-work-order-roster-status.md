# PR-721 已排班工单误报任务待分配

2026-10-23。基线 origin/develop `17c48f2a`，分支 `codex/work-order-roster-status-20261023`。原 ccd2 工作区的已有改动保留。生产修复沿用本会话生产目标，业务验收由 Van 完成。

## 复现与原因

生产只读核对计划 12 的两个工单：尾号 63 含烘焙、色选两项任务，尾号 62 含一项烘焙任务。当天排班已有有效负责人，工位任务接口正确返回 roster_status=assigned；工单列表却按工序摘要里旧 assigned_to 字段是否为空判断，显示“待分配 2 项任务 / 1 项任务”。排班没有丢失。

## 修复

- 工单列表加载时并行读取 GET /api/production-roster/today?scope=all，按当前工位 ID 匹配；无 ID 的旧记录才按工位名称兼容。
- 有效负责人显示“等待工位处理”、今日负责人和“进入工位”。本周未保存、无人值班、失效人工调整、待交接、读取失败分别展示，不再要求逐任务配人。
- 无人值班按工位去重计数，跳转“查看排班”；网络/权限错误保留工单列表并支持“重试”；过期异步请求不能覆盖较新结果。
- 已完成/取消的任务不会要求重新排班。执行中、暂停与本任务的交接状态保持独立；空工序不误报待入库。
- 跨页操作附带返回生产工单入口。没有新增业务写操作或修改接口、权限、数据库、历史执行人、冻结快照、库存逻辑。

## RED / GREEN 与 API 验证

- 先新增实际 Vue setup 运行测试，RED 复现：没有读取当天排班，当前待办仍为“待分配 2 项任务”；随后修复，8 项针对性测试 GREEN。
- 前端全量 1,545 项测试和 Vite 生产构建通过。构建仅有既有 chunk-size 提示。
- Go 全包测试通过；真实数据库测试在本机临时 schema 中运行。生产数据库无测试写入。
- TestPR657ProductionRosterPostgresLifecycle、TestPR658RosterReliefAndManualReplacementAPI 通过；补充 scope=all 返回合格替补负责人、版本，以及读取后 job_cards.assigned_employee_id 仍为 0 的断言，证明读取不会重新分配任务。
- git diff --check 和 changed 检查通过。

可重复命令：

```sh
(cd orderapp-remote/frontend-vue-shell && node --test src/lib/work-order-roster-runtime.test.js)
bash scripts/verify_kferp.sh frontend
(cd orderapp-remote && go test ./...)
(cd orderapp-remote && ORDERAPP_TEST_DATABASE_URL='postgresql:///postgres?host=/tmp' go test ./internal/interfaces/http/production -run '^Test(PR657ProductionRosterPostgresLifecycle|PR658RosterReliefAndManualReplacementAPI)$' -count=1)
```

## 浏览器与手册

浏览器使用真实 WorkOrdersView.vue 和本地隔离 API。已排班两张工单均显示“等待工位处理”；进入工位、查看排班均带返回入口；无人值班显示具体原因；排班接口 503 保留工单并显示“排班状态待核对 / 重试”。运行测试另覆盖旧姓名、未保存周、已结束、交接隔离、过期响应。

![隔离工单状态验证](assets/pr721/roster-status.png)

手册：`docs/OP_MANUAL_PRODUCTION.md` 的 PR-721 工单列表人员待办说明；Vue 手册页读取同一份文件。页面同时说明任务跟随当天工位排班。

## 发布记录

待补充生产版本、运行核对及回滚依据。生产工单 18/19（显示尾号 62/63）和排班只读验证，不进行开工、领料、重新分配等业务写入。
