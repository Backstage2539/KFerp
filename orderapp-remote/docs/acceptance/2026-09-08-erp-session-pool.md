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
- 开发修复经 GitHub PR #85 合并；正式修复经 PR #84 合并。开发与正式环境均已按顺序部署并验证。


## 发布记录

- 用户授权顺序：先发布 development 并验证，再合并 main 并发布 production。
- 开发版本：`5d56503ecf9c9921ce03d342565c9dee279793e3`；命令 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`。
- 开发回滚源码：`/opt/stacks/erp/orderapp.backup.deploy-20260908233907-5d56503ecf9c`；镜像 `kferp-orderapp-rollback:development-20260908233907-5d56503ecf9c`。
- 开发发布检查：Vue 1115/1115、小程序 238/238、类型检查及构建、服务器及镜像内 Go 测试通过。应用 running、重启计数 0；公网登录页 200；入口 303；未认证账户接口 401、系统认证 200；PR-640 / DEV-640 可查。
- 开发现场回归：在已部署源码上连接开发 PostgreSQL，以独立测试 schema 执行 `TestERPChannelCustomerAuth*`，连接池 1 / 4、8 并发请求及资格检查期间安全变化全部通过，测试 schema 自动清理。
- 开发库无本次生产账号；未创建或修改真实客户账号。正式账号的页面与接口复验放在 production 完成。
- 正式目标版本：`f9e3c56acb32d4ace7991a337c3e8dcd9eac4cfb`；命令 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`。
- 正式回滚源码：`/opt/stacks/erp-production/orderapp.backup.deploy-20260908235113-f9e3c56acb32`；镜像 `kferp-orderapp-rollback:production-20260908235113-f9e3c56acb32`。
- 正式发布检查：Vue 1115/1115、小程序 238/238、类型检查及构建、服务器及镜像内 Go 测试通过；公网登录页 200、未认证入口及账户接口 401、系统认证 200；应用 running、重启计数 0；PR-640 / DEV-640 可查。
- 正式账号并发复验：正常密码登录后，同一会话连续 3 轮各 8 个并发请求，混合账户、工作台概览和商品选项接口，24/24 为 HTTP 200；每轮最慢请求分别为 2.622 / 3.843 / 4.625 秒。客户范围校验通过，能读取现有价格表、商品及订单；随后退出测试会话成功。
- 浏览器复验：通过正式登录页登录，进入绑定客户的履约工作台，菜单、已发布价格表、提交订单面板及现有履约订单正常显示，无持续空白或再次卡死；随后正常退出。未提交业务订单或变更业务数据。
- 源码核对：两个环境的 `employee_context.go` SHA-256 为 `4f30399951d6f585b9fe221ecb2ac9aded8d6b71963ea979b1a5bd0e9a0fe47e`；`customerfulfillment/repository.go` 为 `ca719f5077f86cb6cc621100c78287b5844fbd5b5cf88039b926075cbb40be24`，均与已验证代码一致。
- 现场数据库检查没有事务/行锁等待。产品业务验收仍由 Van 完成。
- 本发布记录在应用发布后补交；运行版本仍分别为上述 development / production 提交，记录更新不触发重复部署。
- 两次发布均不上传微信小程序，不覆盖本机小程序开发目录。
