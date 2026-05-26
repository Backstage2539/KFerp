# PR-379 客户门户/工作台开关与渠道客户验收记录

## 需求范围
- PR-379-CHANNEL-PORTAL-WORKBENCH-SWITCH：客户档案新增“开通客户门户/工作台”开关；渠道客户作为客户类型，权限由能力模板决定。
- 开关开启时必须选择能力模板，并同步创建或启用客户门户配置；开关关闭只停用访问，不删除历史配置、订单、外部账号或操作日志。
- 门户配置默认列表只展示已开通客户；履约客户候选按 active、开通状态和能力模板判断，不再硬编码批发客户。
- 渠道客户下单时终端收件人进入订单收件字段，不新增客户档案；历史收件信息从该渠道客户历史订单聚合。
- 录单豆单/价格表选择按商品自定义产品类型分组，选择商品自动匹配同分类客户专属最新价格表，无专属则回退公共同分类最新版本。

## 验收用例
- [x] 客户档案可新增/编辑 `渠道客户`，开关关闭时不要求能力模板。
- [x] 打开“开通客户门户/工作台”后必须选择能力模板，保存后客户接口返回 `portal_enabled=true` 和 `capability_template_key`，操作日志可查。
- [x] 门户配置默认列表不显示未开通客户；开通后显示，关闭后从默认列表移除。
- [x] 履约客户候选只包含 active、已开通门户/工作台、模板暴露工作台或下单能力的客户，渠道客户满足条件后可选。
- [x] 渠道客户使用客户工作台/履约运营台下单时，可选择历史收件信息；保存订单后客户仍为渠道客户，未新增终端收件人客户档案。
- [x] 录单选择商品后自动匹配该商品产品类型分类的客户专属价格表；无专属时回退公共价格表。
- [x] 浏览器验收跑通：客户档案开通渠道客户门户/工作台 → 绑定模板 → 录单/客户工作台选择渠道客户 → 商品自动匹配价格表 → 保存订单 → 历史收件人可回查 → 操作日志可查。

## 验证证据
- 后端全量测试：`go test ./...`，通过。
- 后端目标包测试：`go test ./internal/application/customer ./internal/application/customerportal ./internal/infrastructure/postgres/customer ./internal/infrastructure/postgres/customerportal ./internal/infrastructure/postgres/customerfulfillment ./internal/infrastructure/postgres/sales ./internal/interfaces/http/customer ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/customerportal ./internal/interfaces/http/support`，通过。
- 前端工具函数测试：`node --test src/lib/*.test.js`，374 条通过。
- 前端构建：`npm run build`，通过；保留 Vite 既有大 chunk 警告。
- 开发栈部署：`origin/develop=79339e5d84e0925308f17b06867f574b389bafed` 已部署到 `/opt/stacks/erp`，`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` 均运行。
- 浏览器验收记录（2026-05-26）：
  - 在客户档案新增 `PR379CHANNEL05261854`，客户类型为渠道客户，打开“开通客户门户/工作台”，能力模板自动推荐并保存为“渠道代发/现货下单”。
  - 门户客户配置默认列表可搜索到 `PR379CHANNEL05261854`，显示渠道客户、门户启用和“渠道代发/现货下单”模板。
  - 客户履约运营台候选下拉可搜索到该渠道客户；创建外部用户 `PR379USER / 13900003791` 后可载入履约运营台。
  - 普通录单选择该渠道客户后显示客户类型“渠道客户”；选择 `Codex测试速溶盒装 10条/盒` 后自动匹配“速溶咖啡”分类的 `CODX-速溶盒装-20260525` 价格表，并带出 15/盒、13.80/盒梯度价。
  - 履约运营台提交渠道代发订单 `CDS-20260526-1186`，终端收件人为 `RECIPIENTPR379 / 13900003792 / SHANGHAI PR379 RECEIVER ROAD 2`，订单仍归属渠道客户 `PR379CHANNEL05261854`。
  - `GET /api/customer-fulfillment/160/options` 返回历史收件人 `RECIPIENTPR379`；`GET /api/customers?q=RECIPIENTPR379` 返回 `total=0`，确认未新增终端收件人客户档案。
  - `GET /api/audit?q=PR379` 返回客户档案新增日志：`13800138075 在客户档案新增了客户 PR379CHANNEL05261854`。

## 手册
- `docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `docs/OP_MANUAL_ORDER_SALES.md`
