# PR-701 修正：订单复制收件信息和简化明细

- 用户反馈：生产小程序订单 SO-20261019-0004 复制收件人、电话、地址均为“未填写”；明细要求 `2. 曲奇｜227g袋装｜1 袋`。
- 分支：`codex/miniapp-order-copy-recipient-20261020`；起点 `origin/develop@861f54e0`（GitHub 连接器核对）。
- 原因：复制查询只加载订单收件字段，普通订单详情则在这些字段为空时使用客户联系人、电话和地址，导致两端结果不一致。未读取或修改该生产订单，本次采用同形数据复现逻辑差异。
- 修复：在同一只读事务中加载客户默认收件信息；普通订单先取订单保存值，再逐字段使用客户默认值。代发、商品下单、加工发货只使用已保存收件信息。仍缺失显示“未填写”。不新增业务写入或迁移，保留权限、客户隔离、确认中订单版本及冻结商品规格。
- 输出：所有复制入口统一为 `序号. 商品｜规格｜数量 单位`，去除“规格：”“数量：”。

## TDD 和验证

- RED：更新格式化测试后，旧格式包含“规格：/数量：”而失败；新增普通订单默认收件信息用例失败，电话/地址仍为“未填写”。新增小程序 HTTP 用例同样复现旧格式及空收件信息。日志 `/tmp/kferp-miniapp-copy-red.log`、`/tmp/kferp-miniapp-copy-red-api.log`。
- GREEN：`go -C orderapp-remote test ./internal/application/sales ./internal/interfaces/http/sales ./internal/interfaces/http/customerportal -run 'Test(OrderCopyDetails|MiniOrderCopyDetails)' -count=1` 通过（未配置数据库，PostgreSQL 用例跳过）。覆盖完整文字、数量/单位、默认/指定/部分收件信息、三类禁止默认回填订单、客户与员工小程序、权限与客户隔离。
- 网页定向复制测试 4/4；Vue 全部 1436/1436；小程序定向复制测试 4/4、全部 371/371；小程序类型检查、生产环境构建与 Vue 构建通过。
- `git diff --check` 通过。
- 数据库集成用例已补齐：`TestOrderCopyDetailsPostgresSavedAddressAndSnapshots`、`TestOrderCopyDetailsPostgresRecipientSources`。显式使用 `ORDERAPP_TEST_DATABASE_URL=postgres:///postgres?host=/tmp` 运行时，连接本地 PostgreSQL 被沙箱拒绝（`operation not permitted`），不记为通过。
- 全量 `go test ./...` 已执行；除 customerportal / officialaccount 中 `httptest.NewServer` 无法监听端口而失败外，其余包通过。未将环境限制下的全量结果记为 GREEN。

## 手册与交付

- 同步 `OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_CUSTOMER_ACCOUNT.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`、`OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`；保留 docs 单一来源，并同步 Vue 订单页提示。
- 验收：实现与本地定向验证通过；真实生产订单业务验收、PostgreSQL 集成复验待完成。
- 生产部署受限：本会话 SSH 连接 `root@1.12.242.58:22` 被沙箱拒绝；Git SSH DNS 也不可用。GitHub 连接器仍可访问，用于保存和合并已验证修复。
- 已构建生产 API 小程序：`miniapp/dist/build/mp-weixin`，API `https://erp.qacoohee.com/app`。它需要修复后的生产后端上线后才能验收收件信息；并不代表微信线上版本已更新。
- DevTools UI 控制连接已关闭，加载状态需另行确认；不上传、审核或发布微信版本。


## 2026-10-20 续办：合并与生产项目加载

- 功能提交 `13bd0d20` 已推送；复核并再次通过订单复制 Go 单元/HTTP 定向测试。
- `develop` 合并并推送为 `82543a081aa2ceaad3cae802644f10c799e8a58d`；完整最新 develop 合入 main，生产代码提交 `00a1ff25596946deac0b51645d0b2700329bac30` 已推送。main 保留自身已有生产修复，无冲突。
- 已执行 `KFERP_MINIAPP_EXPORT_DIR=/tmp/kferp-miniapp-production-20261020/KFerp-miniapp-mp-weixin ./deploy_orderapp.sh production`。脚本确认本地 HEAD 与 origin/main 一致后，在创建远端临时目录前被 `ssh: connect to host 1.12.242.58 port 22: Operation not permitted` 拒绝。未上传源码、构建镜像或重启生产服务；生产修复尚未上线。
- main 的生产小程序已本地重建，`RELEASE_INFO` commit 为 `00a1ff25`、environment 为 production、API 为 `https://erp.qacoohee.com/app`；24 个注册页面校验通过。目录 `/tmp/kferp-miniapp-production-20261020/KFerp-miniapp-mp-weixin`。
- DevTools CLI 因无法写入应用配置而受限，已通过原生 GUI 导入上述目录。项目名 `KFerp 生产环境 20261020`，AppID `wx6512f8684f584828`。已确认实际游客首页成功渲染，控制台没有错误，仅有原有页面配置/样式警告。未登录真实客户、未提交业务数据，也未上传、审核或发布微信版本。
- 本节后续提交仅记录交付状态；构建包对应的生产代码为 `00a1ff25`。待恢复本会话 SSH 访问或在授权的正常终端执行部署命令后，才能验收生产订单收件信息修复。
