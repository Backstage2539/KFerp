# PR-570-ORDER-ENTRY-PRICE-SPECS-ORDER-DETAIL 验收记录

## 范围

- `DEV-570-PRICE-TABLE-SPEC-SCOPE`：ERP 新建/复制订单及主动切换价格表时，仅提供当前已选已发布价格表中有价的具体 SKU，并同步默认规格和可售规格计数。
- `DEV-570-ORDER-DETAIL-SCHEMA-COMPAT`：订单详情读取权威数据库列 `price_overridden`，保持 API `price_override` 兼容，不做数据库迁移或数据修复。
- `DEV-570-HISTORY-DOCS-DELIVERY`：历史订单继续读取冻结规格；复制订单执行当前价格表规则；同步需求、验收、手册并交付 development/production。

## TDD 证据

- RED（Vue）：publication 91 只给 100g 定价时，旧实现仍返回 100g/227g/454g 并选择无价默认 227g；定向测试 124 项中 1 项失败。
- RED（Go）：订单详情查询合同调用尚不存在的权威查询 helper，复现旧查询未受列名合同保护；定向包编译失败。
- 二次 RED（Vue 视图）：商品候选仍统计商品档案总规格，页面手册仍描述无价规格可见；124 项中 2 项失败。
- GREEN（focused）：`node --test src/lib/order-entry.test.js` 125/125；`go test ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales ./internal/interfaces/http/support -count=1` 通过。
- 独立审查发现并修复：首次选择默认 SKU 未应用发布冻结规格快照、未定价规格名/编码仍可命中商品搜索、PostgreSQL 集成测试列数断言错误。修复后前端与后端/文档复核均无剩余 P0-P2。

## 实现边界

- 可售规格按规格至少存在一条当前 publication 的价格阶梯判断，不按当前录入数量过滤；数量档位继续由既有缺价校验负责。
- 规格候选名称使用当前商品档案名称，选中后订单继续冻结价格表销售规格快照。
- 历史编辑保留只读历史规格；复制和主动切换价格表不保留当前表无价规格。
- SQL 仅把不存在的 `oi.price_override` 更正为 `oi.price_overridden`；Go 字段和 JSON 名称不变。
- 本需求没有新增用户写操作，不改变操作日志范围；验收不创建测试订单、不修改生产业务数据。

## 自动验收与交付证据

- [x] 支持合同、受影响测试及完整门禁通过：Vue 854 项、miniapp 113 项、miniapp 类型检查与 development/production 构建、Go 全量测试、隔离 Docker 构建均为 GREEN。
- [x] development 远程预检与部署通过，实际应用提交为 `34513324c909748a611c1c0915812486f38c0d85`；源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260801185634-34513324c909`，回滚镜像为 `kferp-orderapp-rollback:development-20260801185634-34513324c909`。
- [x] development 认证只读冒烟通过：既有订单列表、`/api/orders/:id/detail` 和 `/api/order/form?edit_id=:id` 均返回 200，明细包含 `price_override`；新容器日志无旧列错误、`SQLSTATE 42703`、panic 或 fatal。
- [x] `develop` 已合入；production 候选从最新 `main` 合入已验证 `develop`，生产配置远程预检通过后合入 `main`。
- [x] production 部署通过，实际应用提交为 `5adb44b858f36b8d8b73435643fde8d4d636290e`；源码备份为 `/opt/stacks/erp-production/orderapp.backup.deploy-20260801192708-5adb44b858f3`，回滚镜像为 `kferp-orderapp-rollback:production-20260801192708-5adb44b858f3`。
- [x] production 当前真实库订单与明细均为 0，无法做已有订单点击冒烟；为保持零写入，已在真实生产表验证仅存在 `price_overridden`、不存在 `price_override`，修正后的只读 SQL 可编译执行，空白录单 API 与需求合同返回 200，新容器日志无旧列错误、`SQLSTATE 42703`、panic 或 fatal。
- [x] 两环境数据库均未重启；无数据库迁移、数据修复或测试订单写入。服务器发布不上传或发布微信小程序版本。

## 待用户验收

- [ ] 用户在 development/production 自行检查：录单仅显示当前价格表有价规格；有历史订单的环境点击订单名称可正常打开。
