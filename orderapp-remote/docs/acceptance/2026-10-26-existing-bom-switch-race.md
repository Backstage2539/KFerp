# 既有 BOM 切换并发风险核查（2026-10-26）

## 基线与范围

- 基线：`main 001c65b05485cca6ad5b9e241309bb1326039d04`。
- 在生产补充发布候选的隔离数据库中核查；相关 `productspecmigration` 应用/仓库代码、BOM 与 catalog 默认切换调用均与该基线一致。本次功能整合未引入以下行为。
- 探索仅临时补齐 `repository_postgres_integration_test.go` 的 fixture 初始化，未修改业务代码、schema 或严格断言。探索修改已撤回，该测试文件保持基线原样。
- 本记录区分退役迁移路径与现行默认 BOM 实际规格切换，不将既有问题称为本次回归，也不据此宣称并发场景已通过。

## 观察与契约

1. 原测试开启 `KF_RUN_POSTGRES_INTEGRATION=1` 后，两项均因 fixture 缺少 `product_bom_spec_authorities` 而失败（`SQLSTATE 42P01`）。平常整包运行未开启此开关会跳过这两项。
2. 补齐 authority 初始化后，`TestPerProductMigrationPostgresFlow` 在旧业务写事务尚未提交时观察到 cutover 提前完成。该断言针对已退役的逐商品迁移行锁流程：现行 `appmain/schema_setup.go` 安装 authority view/projection，运行时身份不再依赖旧迁移状态表；`TestMigrationRoutesAreRemoved` 明确要求旧 prepare/readiness/cutover 路由返回 404。这不是当前在线迁移入口的失败证明。
3. 同样补齐 fixture 后，`TestDefaultBOMSwitchSerializesConcurrentRemovedSpecWritePostgres` 观察到默认 BOM 切换事务提交前，旧规格业务 INSERT 已成功（错误值为 `nil`），触发原有严格断言：`removed-spec business write completed before the default switch committed: <nil>`。
4. 第二项从 BOM 10 / spec 100 切到 BOM 20 / spec 200，涉及实际规格移除，不只是默认标签改变。PR-727 取消默认变更提醒，但保留实际规格身份、有效性和移除检查；因此这项并发场景仍有意义。

相关实现：`default_bom_switch_guard.go` 的 `GuardDefaultProductBOMSwitchTx` 获取商品 advisory lock 和默认绑定行锁；`schema.go` 的 `validate_bom_spec_business_identity` 查询当前 authority/绑定，但没有取得相同的锁。BOM 发布、设置默认及 catalog 配置仍调用该切换 guard。上述事实说明：切换检查与新业务身份写入之间可能存在并发窗口，让即将移除的规格在切换提交前新增业务记录。尚未在真实生产数据上复现，也未评估全部上层事务是否另有序列化保护。

## 隔离复现

只使用可丢弃的本地测试数据库，不对生产数据库运行。两个旧测试使用 `pgxpool.New(ctx, "")`，实际连接由 `PGHOST` / `PGDATABASE` 指定，而不是只靠 `ORDERAPP_TEST_DATABASE_URL`。

在 `orderapp-remote` 执行：

```sh
ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_release_20261026?host=/tmp' \
PGHOST=/tmp PGDATABASE=kferp_release_20261026 \
KF_RUN_POSTGRES_INTEGRATION=1 GOMAXPROCS=2 \
go test -p 1 -count=1 -v ./internal/infrastructure/postgres/productspecmigration \
  -run 'Test(PerProductMigrationPostgresFlow|DefaultBOMSwitchSerializesConcurrentRemovedSpecWritePostgres)$'
```

基线原 fixture 的结果是缺 authority。若在独立临时副本验证后续并发行为，最小 fixture 补充为以下 11 行，不修改断言或业务函数：

- 两个测试各自在 `EnsureSchema` 成功后调用 `EnsureAuthorityProjection(ctx, pool, schema)`，错误仍 `t.Fatal(err)`。
- 第二个测试的建表列表增加 `products(id BIGINT PRIMARY KEY, active BOOLEAN NOT NULL DEFAULT true, parent_product_id BIGINT NOT NULL DEFAULT 0, auto_derived_sku BOOLEAN NOT NULL DEFAULT false)`。
- 第二个测试的种子 SQL 增加 `INSERT INTO <fixture schema>.products(id) VALUES(42)`。

之后执行同一命令，可观察上述两个并发断言失败。当前仓库没有保留这段探索补丁。

## 已通过验证与未修边界

以下现行契约定向运行共 5 项，全部通过、无跳过：清理后无旧表依赖、当前规格及精确冻结来源写入、resolver 使用发布 authority、旧迁移路由移除、规格选项提供规范身份。

```sh
ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_release_20261026?host=/tmp' GOMAXPROCS=2 \
go test -p 1 -count=1 -v \
  ./internal/infrastructure/postgres/productspecmigration \
  ./internal/interfaces/http/productspecmigration \
  -run 'Test(BusinessIdentityGuard|GenericBOMSpecResolver|MigrationRoutesAreRemoved|ProductBOMSpecOptionsExposeCanonicalWriteIdentity)'
```

当前规格 guard 覆盖顺序写入、历史身份原样更新以及订单/工单/工序来源精确匹配；不包含默认 BOM 切换与新业务写入的并发验证。其通过不能替代该并发场景。

本次未增加数据库锁、未修改业务 schema、未重写旧测试断言。默认 BOM 实际规格移除的并发窗口保留为独立待评估问题；修复需另外验证锁顺序、并发事务和完整业务调用链。

本机证据：

- `/tmp/kferp-followup-production-repository-tests.log`：production 与 productspecmigration 默认整包验证通过，旧开关测试不在执行范围。
- `/tmp/kferp-followup-spec-migration-explicit-integration.log`：开启旧集成开关后的缺 authority 失败。
- `/tmp/kferp-followup-spec-migration-fixture-integration.log`：最小 fixture 补齐后的严格并发断言失败。
- `/tmp/kferp-followup-current-identity-contract.log`：当前身份契约 5 项通过、无跳过。
