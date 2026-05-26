# PR-379 客户门户/工作台开关与渠道客户验收记录

## 需求范围
- PR-379-CHANNEL-PORTAL-WORKBENCH-SWITCH：客户档案新增“开通客户门户/工作台”开关；渠道客户作为客户类型，权限由能力模板决定。
- 开关开启时必须选择能力模板，并同步创建或启用客户门户配置；开关关闭只停用访问，不删除历史配置、订单、外部账号或操作日志。
- 门户配置默认列表只展示已开通客户；履约客户候选按 active、开通状态和能力模板判断，不再硬编码批发客户。
- 渠道客户下单时终端收件人进入订单收件字段，不新增客户档案；历史收件信息从该渠道客户历史订单聚合。
- 录单豆单/价格表选择按商品自定义产品类型分组，选择商品自动匹配同分类客户专属最新价格表，无专属则回退公共同分类最新版本。

## 验收用例
- [ ] 客户档案可新增/编辑 `渠道客户`，开关关闭时不要求能力模板。
- [ ] 打开“开通客户门户/工作台”后必须选择能力模板，保存后客户接口返回 `portal_enabled=true` 和 `capability_template_key`，操作日志可查。
- [ ] 门户配置默认列表不显示未开通客户；开通后显示，关闭后从默认列表移除。
- [ ] 履约客户候选只包含 active、已开通门户/工作台、模板暴露工作台或下单能力的客户，渠道客户满足条件后可选。
- [ ] 渠道客户使用客户工作台/履约运营台下单时，可选择历史收件信息；保存订单后客户仍为渠道客户，未新增终端收件人客户档案。
- [ ] 录单选择商品后自动匹配该商品产品类型分类的客户专属价格表；无专属时回退公共价格表。
- [ ] 浏览器验收跑通：客户档案开通渠道客户门户/工作台 → 绑定模板 → 录单/客户工作台选择渠道客户 → 商品自动匹配价格表 → 选择历史收件人 → 保存订单 → 操作日志可查。

## 验证证据
- 后端全量测试：`go test ./...`，通过。
- 后端目标包测试：`go test ./internal/application/customer ./internal/application/customerportal ./internal/infrastructure/postgres/customer ./internal/infrastructure/postgres/customerportal ./internal/infrastructure/postgres/customerfulfillment ./internal/infrastructure/postgres/sales ./internal/interfaces/http/customer ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/customerportal ./internal/interfaces/http/support`，通过。
- 前端工具函数测试：`node --test src/lib/*.test.js`，365 条通过。
- 前端构建：`npm run build`，通过；保留 Vite 既有大 chunk 警告。
- 待补：浏览器验收截图或操作记录。

## 手册
- `docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `docs/OP_MANUAL_ORDER_SALES.md`
