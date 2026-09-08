# PR-640 履约客户登录连接池耗尽

## 现场与恢复

- 正式环境客户账号登录后并发加载，业务接口超时。数据库现场为 4 个应用连接：1 个在持有登录会话行锁的事务中等待客户端，3 个等待同一会话行锁；最早等待超过 10 分钟。
- 应用容器仍运行，CPU/内存无耗尽证据。只重启 `erp_prod_orderapp` 可恢复请求，但再次登录会复现，因此重启不是永久修复。
- 恢复后使用授权账户顺序验证登录、`/app/api/auth/me`、`/app/api/customer-processing/portal/overview`、`/app/api/customer-processing/portal/options` 均为 200，能读取绑定客户的订单、已发布价格表和商品。未变更客户绑定、权限、商品、价格、订单或库存。
- 本文不保存登录凭据、会话令牌或客户业务明细。

## 原因与实现

1. 会话校验持有事务与 `FOR UPDATE` 行锁时，再通过同一连接池调用客户工作台资格查询。并发请求占满连接后，锁持有者无法获得新连接，其他请求又无法获得行锁。
2. 客户绑定查询尚未关闭结果集时查询能力模板，也会额外占用连接。
3. 修复先完成工作台资格查询，再进入会话事务重新核验身份、有效期、停用及安全时间戳；资格检查期间退出或安全状态变化仍拒绝旧会话。客户绑定结果先完整读取并关闭，再查询模板。

## 验证

- RED：新增真实 PostgreSQL 测试，连接池上限 1 时工作台查询超时；上限 1 / 4 时同一会话 8 个并发请求均不能返回 200。
- GREEN：相同测试通过；两个池规模下的 8 个并发请求均为 200，无连接嵌套等待。
- 安全回归：新增资格检查期间退出、密码变更、账号类型变化拒绝旧会话；原会话永久撤销、ERP 工作台资格和密码登录测试通过。
- 定向命令：`ORDERAPP_TEST_DATABASE_URL=<local test database> go test ./internal/interfaces/http/support -run 'TestERP(ChannelCustomer|LoginSession|LoginToken|PasswordLogin|Bearer|SMS)' -count=1 -timeout=180s`。
- 完整后端门禁：`scripts/verify_kferp.sh backend` 通过；真实 PostgreSQL 定向安全回归通过（非跳过）；`git diff --check` 通过。
- 手册：`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`，沿用既有 Vue 手册入口。没有修改用户权限或业务操作流程。
- 合并和正式修复发布：尚未执行。正式环境目前仅做应用重启恢复。
