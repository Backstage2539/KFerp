# PR-761 工位逐批报工与 WIP 补料验收

状态：隔离验证通过，已合并 develop 和 main 并部署正式环境；用户验收待完成。

- 逐批：隔离 PostgreSQL/API 验证 59.962 kg 已扣投入拆为 15/15/15/14.962，数量、成本和库存守恒；历史记录、幂等、并发、权限、空产出和尾料校验通过。
- 补料：隔离 API 验证草稿、确认、恢复、取消、重试、跨仓失败回滚、共享 WIP、库存变化和实际投入变化；补料→开工→逐批完成全链通过。
- 商品共享 WIP：固定批次的 API、合格自由批次的数据库候选计算及货主/质检过滤通过；不得替换已固定的来源批次。
- 页面：前端全量 1935 项通过、Vue 构建通过；Chrome 桌面及 390px 组件检查使用模拟接口，不代表现场业务验收。
- 手册：`docs/OP_MANUAL_PRODUCTION.md` 和 `docs/OP_MANUAL_OPERATION_MODES.md`；生产手册页面读取前者。
- 当前工单 84 仅只读诊断；未代用户启用批次、补料或报工。正式环境仅只读验证。

- 正式运行版本 `9f7f58a4710943bd091f3a1fbff8b9e494627130`；应用运行且重启 0、数据库健康、登录 200、匿名工单 401、认证工单 200；工位资源公网、服务器与容器 SHA256 一致。
- 数据库备份 `/opt/stacks/erp-production/backups/pre-pr761-20261111203220.dump`；回滚源 `/opt/stacks/erp-production/orderapp.backup.deploy-20261111203302-9f7f58a47109`；回滚镜像 `kferp-orderapp-rollback:production-20261111203302-9f7f58a47109`。保留原小程序包。
