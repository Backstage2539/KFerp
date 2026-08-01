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

## 待完成

- [ ] 支持合同、受影响/完整测试、前端构建和双环境远程预检。
- [ ] 分支推送并合入 `develop`、`main`。
- [ ] development、production 部署和认证后的已有订单详情只读冒烟。
- [ ] 用户业务验收。
