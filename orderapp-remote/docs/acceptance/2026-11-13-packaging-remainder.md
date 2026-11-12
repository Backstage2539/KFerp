# 包装逐单报工与计划级余料验收

需求：`PR-PACKAGING-REMAINDER-20261113`。状态：隔离验证通过并已发布正式 ERP，待用户验收。

- 单张包装工单按冻结配方和实际数量结转已投入材料；完整和分次报工、旧逐单余料方式在隔离 PostgreSQL 接口回归通过，未用组件及成本保留。接口 RED 首次因测试库未配置而跳过；独立用量单测取得 RED/GREEN。
- 页面移除计划内工单逐单余料输入；冻结工时可修改，显式零和部分报工按数量分配测试通过。无有效计划或产品身份的历史独立工单继续使用旧协议。
- 隔离 PostgreSQL HTTP 回归：留 WIP、异仓转拨、包材及成本守恒、零余料、同源并发和幂等、操作日志失败回滚、部分处理、跨计划阻断、历史兼容及新工单重新领取同批均通过；库存净零变动或连续净流出后的重核、差额保留及人工核对通过。
- 前端全量 2040 项、后端无业务数据库 `go test ./...` 和 Vue 构建通过；桌面与窄屏模拟 API 检查通过。命令：`ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go test ./internal/interfaces/http/production -run '^(TestPlanPackaging|TestPieceCompletionDefersRemainder|TestPlanTransferRejectsUnscopedWorkOrdersBeforeAnyWrite)' -count=1 -v`。
- 范围限制：其他历史计划使用过同批库存但缺少明确结算记录时，来源会保守阻断并提示核对，不自动推断。人工核对差额仅记录说明，不修改库存或成本。
- 正式 ERP 运行 `c1e0d8282a9ae25a2c9a8225403d7a000966d62f`，应用运行且重启次数 0、数据库健康、登录 200、未登录接口 401，工位资源服务器与容器哈希一致；既有小程序包保留。数据库备份 `/opt/stacks/erp-production/backups/pre-packaging-remainder-20261113013343.dump` 已校验；旧源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261113013415-c1e0d8282a9a`、回滚镜像 `kferp-orderapp-rollback:production-20261113013415-c1e0d8282a9a`。
- 手册：`docs/OP_MANUAL_PRODUCTION.md`。正式环境只读核验，不操作现有工单或余料。
