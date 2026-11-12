# PR-765 客户历史生产批次代发提交修复

- 日期：2026-11-12（Asia/Shanghai）
- 分支：`codex/dropship-submit-fix-20261112`
- 产品需求：`PR-765-DROPSHIP-PRODUCED-BATCH`
- 开发任务：`DEV-PR765-01` / `DEV-PR765-02`
- 当前状态：修复、50 个负例及定向 RED/GREEN、独立审查、历史转仓和发货 HTTP 回归通过；两项同基线数据库缺表夹具失败仍记录。已合入 main 并发布生产，服务和数据库规则只读核验通过，用户真实提交验收待完成。

## 问题与现场证据

客户在生产小程序选择多项自有库存商品，点击提交代发订单，界面显示 `internal error`。开发者工具中的商品、数量与用户截图一致；真实客户、商品和订单数据不纳入此公开验收记录。

生产数据库日志记录：写入 `customer_direct_ship_request_allocations` 被 `validate_bom_spec_business_identity` 拒绝，错误类别为 `bom_variant_not_current`。

只读查询确认所选批次归客户所有，仍有可用实物，来源是已过账的最终生产产出事件，已直接入该客户仓库。系统已有历史转仓来源兼容，尚未覆盖这一生产入库来源的后续新代发分配。自动回归使用合成客户、商品、工单和批次。

检查时未发现此次失败对应的新订单；排查只读，无真实订单、库存、费用或发货写入。

## 修复边界

- 只允许能够核对真实生产入库来源、客户货权、相同父商品与稳定规格、批次、仓库及剩余可用量的历史批次参加代发分配。
- 新订单按现行下单规则保留当前规格身份，实物分配保留历史批次身份；不把历史批次改写为当前配方版本。
- 原有跨客户、错误来源、错误规格、冻结/不可用库存、超量分配等拒绝规则继续生效。
- 提交过程仍在事务中完成，并复用原有幂等与操作日志路径。
- 不调整历史业务数据、不代用户重提生产订单、不重建或发布小程序。

## 验证记录

- [x] 生产只读日志与开发者工具现场核对。
- [x] 最小真实数据库 HTTP 回归先复现相同错误（RED）。
- [x] 修复后历史生产自有批次正常分配（GREEN）。
- [x] 50 个负例通过，覆盖错误来源、客户/仓库/规格、单位和数量边界、重复及镜像分配保护。
- [x] 幂等重试、失败全事务回滚和操作日志覆盖。
- [x] 相关历史转仓批次回归通过。
- [x] 历史批次发货回归通过；最终三个定向 HTTP 用例合计 23.122s。
- [x] 独立静态代码审查通过。
- [x] 不连接业务数据库的后端全包测试通过。
- [ ] 真实数据库身份保护全包通过（存在已确认的同基线夹具失败，见下文）。
- [ ] 已预留生产入库回归通过（存在已确认的同基线夹具失败，见下文）。
- [x] 同步最新 `develop` 后重跑相关验证。
- [ ] 用户业务验收。

在 `orderapp-remote` 目录运行；隔离数据库连接由 `ORDERAPP_TEST_DATABASE_URL` 提供，不使用真实业务库。回归样本仅使用合成数据。

1. RED：`TestMiniDirectShipProducedHistoricalBatchWithCurrentOrderVariantHTTP` 通过真实小程序提交路由返回 HTTP 500 `internal error`；对应隔离 PostgreSQL 日志为 `bom_variant_not_current`，失败点为申请分配记录写入。完整证据在本地 `/tmp/dropship-produced-red.log`、`/tmp/dropship-produced-red-postgres-only.log`，不纳入公开仓库。
2. GREEN：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestMiniDirectShipProducedHistoricalBatchWithCurrentOrderVariantHTTP|TestDirectShipHistoricalOwnedBatchesWithCurrentOrderVariantHTTP' -count=1 -v` 通过（21.219s）。覆盖混合制造/转仓两类历史库存、新订单使用当前规格身份、分配保留历史身份、原库存数量不变、幂等重试、操作日志及后续分配失败全部回滚；新用例包含 50 个负例。本地证据 `/tmp/dropship-produced-green.log`。
3. 后端回归：`env -u DATABASE_URL -u ORDERAPP_TEST_DATABASE_URL go test ./...` 通过；未配置数据库的集成测试按各测试原有规则跳过，因此此结果不能代替真实数据库全包通过。本地证据 `/tmp/dropship-backend-regression.log`。
4. 已知基线失败：真实数据库执行 `productspecmigration` 全包时，`TestBusinessIdentityGuardRequiresCurrentVariantForNewIdentityPostgres` 因夹具缺少 `order_resolution_stock_returns` 表而失败。相同测试在原 `origin/develop` 基线独立检出后也以同一原因失败；本轮不扩展修复范围。证据 `/tmp/dropship-productspec-regression.log` 与 `/tmp/dropship-productspec-baseline.log`。该套件没有全绿。
5. 最终相关回归：新增历史制造批次提交、原历史转仓批次提交及历史批次发货三个 HTTP 用例通过（23.122s），证据 `/tmp/dropship-final-related.log`。
6. 另一已知基线失败：`TestFrozenProductionReceiptConvertsRetainedSupplyAfterBOMRepublish` 的两个子用例也因缺少 `order_resolution_stock_returns` 表失败；原 `origin/develop` 基线同样失败。证据 `/tmp/dropship-final-related.log` 与 `/tmp/dropship-production-baseline.log`。本次不声称已预留生产入库回归通过。

修复提交 `08d75e78f`，同步 `origin/develop` 的 `1e541e90e` 后，在 `cb3c96a6a` 再跑三项代发/历史发货数据库 HTTP 测试通过（19.969s）；support 与 productspecmigration 包的无数据库测试、`scripts/verify_kferp.sh changed` 通过。证据为 `/tmp/dropship-merged-targeted.log`、`/tmp/dropship-merged-backend.log`、`/tmp/dropship-changed.log`。整合记录见 [GitHub PR #337](https://github.com/Backstage2539/KFerp/pull/337)。

## 手册影响

本次修复恢复既有库存代发流程，不改变入口、字段、权限、价格、计费、异常处理操作或提交步骤，故不修改操作手册。既有操作说明见 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md`，前端手册入口不变。

## 交付状态

PR/DEV 已通过现有需求 API 登记。生产发布后，以认证只读 API 读回产品为 `review`、两个开发任务为 `done`，保留技术验证和用户验收的区别。开发环境本次未部署，仍保留原登记状态。不新增旧 UT/API/REV 表条目。

用户明确授权“合并 main 并部署生产环境”后，已将验证过的 `origin/develop` fast-forward 合入 main 并推送。真实订单重提及用户验收未执行。

## 2026-11-12 生产发布

- 运行提交：`48b2dacec0244a32c965f9976606aead70bbe04f`；命令 `KFERP_ERP_ONLY=1 ./deploy_orderapp.sh production`。
- 发布范围还包含已完成技术验证、开发部署及分页现场检查的通知接收人管理更新（`5c64d6fbe`、`1e541e90e`）；该工作流的开发回执为 `c68924f3a`，不代表用户业务验收通过。
- 服务器 1991 项前端测试、Vue 构建、服务器 Go 全包和镜像内 Go 全包通过。服务器 Go 测试未连接业务数据库，不替代上文真实数据库结果。
- 数据库备份 `/opt/stacks/erp-production/backups/pre-pr765-20261112210418.dump`，`pg_restore --list` 验证通过。
- 回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261112210458-48b2dacec024`；回滚镜像 `kferp-orderapp-rollback:production-20261112210458-48b2dacec024`。
- 小程序沿用原产物，保留脚本的文件 SHA-256 比对通过；未重新构建、导出或微信发布。
- 生产应用 running、重启计数 0，PostgreSQL healthy；登录页面 HTTP 200、未认证接口 HTTP 401、认证 PR/DEV 接口 HTTP 200。
- 生产需求的发布证据文字已同步，并写入 `req_product/deploy` 操作日志；只更新发布跟踪元数据，未修改订单、库存、费用或发货。后续种子证据文字同步，support 包测试通过。
- 2026-11-12 21:16（上海）只读核验确认：实际数据库函数已安装制造批次分支和单位校验，两个分配表触发器启用；所报两类批次的完整来源、货权、规格、仓库和可用量检查均通过。原库存及历史变体保持不变，未产生新订单或代发申请。该检查没有真实提交订单。
- 部署源文件与已验证本地 `productspecmigration/schema.go` SHA-256 一致：`c22fca1da4a620d1346fe6c424ccfe6558f55404e028182ce5243043fd4cda4c`。
- 本地完整日志 `/tmp/dropship-production-deploy.log`、私有只读现场结果 `/tmp/pr765-production-readonly-result.log`。后续发布回执只更新文档及需求证据文字，不改变上述运行版本。
