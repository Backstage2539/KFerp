# PR-536 商品行业字段仅来源于模板验收

- 日期：2026-07-14
- 状态：本地实现和目标测试已通过，等待 Van 验收；本任务不部署。
- 需求：`PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY`

## 根因

- 前端曾在没有行业字段模板时继续展示已有商品生产配置字段，并允许按旧别名或显示名匹配模板字段；列表、抽屉、模板切换和保存 payload 的投影口径不一致，异步加载返回还可能覆盖当前商品或当前模板状态。
- 应用服务和 PostgreSQL 仓储曾允许 `industry_field_template_id=0` 时原样保存字段，`CopyProduct` 还会在不复制模板引用的同时复制字段值，形成无模板孤立字段。
- 启动回填曾从 `products.roast_level` 和 `products.special_attrs_json` 通过 `jsonb_each_text` 自动生成商品行业字段；缺少幂等清理无配置孤儿、无模板字段和模板外字段的统一步骤。

## 目标行为

- 商品列表、商品档案配置抽屉、模板切换和保存只投影当前明确引用模板的精确字段键；无模板时统一为空。
- 应用服务、HTTP API 和仓储直接调用都执行同一模板约束，并在无模板时返回非空 `fields: []`。
- `CopyProduct` 不复制行业字段模板引用，也不复制行业字段值。
- 启动清理幂等删除无配置孤儿、无模板字段和模板外字段，不再从旧商品列回填行业字段。

## RED 证据

- 前端：`node --test src/lib/product-settings.test.js` 的新增断言在实现前分别暴露无模板仍保留字段、仅显示名也被错误匹配，以及异步旧请求覆盖当前抽屉/模板投影的问题。
- 应用/API：`go test ./internal/application/catalog ./internal/interfaces/http/catalog -count=1` 的新增断言在实现前暴露 `industry_field_template_id=0` 仍接收传入字段、响应不是 `fields: []` 的问题。
- 仓储/迁移：`go test ./internal/infrastructure/postgres/catalog -count=1` 的新增断言在实现前暴露复制商品制造孤立字段、旧 `jsonb_each_text` 回填仍存在，以及缺少无模板/孤儿/模板外字段清理的问题。
- 文档合同：`go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1` 在本任务首次运行时退出码为 1，失败信息为 `docs/OP_MANUAL_INVENTORY_MATERIALS.md missing PR-536 marker "取消行业字段模板会清空商品行业字段"`。

## GREEN 证据

- 前端：`node --test src/lib/product-settings.test.js` 通过 159/159，覆盖无模板清空、当前模板精确投影、拒绝旧别名/仅显示名匹配，以及抽屉异步结果防串写。
- 应用：`go test ./internal/application/catalog -count=1` 通过。
- HTTP API：`go test ./internal/interfaces/http/catalog -count=1` 通过。
- PostgreSQL 仓储：`go test ./internal/infrastructure/postgres/catalog -count=1` 通过；schema 回填不再包含 `jsonb_each_text`。真实清理 SQL 矩阵在一次性本地 PostgreSQL 中验证了首次执行、重复执行幂等、无模板、孤儿、模板外字段和首次启动缺少行业模板表的边界。该验证未连接开发或生产数据库。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1` 通过；`go test ./internal/interfaces/http/support -count=1` 最初因 PR-409 旧源码合同仍要求 `return fields, nil`、PR-439 旧复制文案标记被改写而失败。保留 PR-439 历史标记并把 PR-409 源码合同更新为 PR-536 的非空空切片标记后，完整 support 包通过。
- Vue/Vite production build 与 `scripts/verify_kferp.sh changed` 本任务未运行，留给 Task 7 合并前总验证；本文件不把它们记录为已通过。

## 数据边界

- 清理只删除 `product_production_config_fields` 中不满足当前模板约束的行。
- 保留行业字段模板和字段定义，不删除模板。
- 保留 `products.roast_level`、`products.special_attrs_json` 作为历史数据，但不再回填为商品行业字段。
- 保留已发布商品价格记录，以及历史订单、价格表、生产计划和工单快照；不回改历史业务快照。
- 商品档案配置保存继续使用既有 `save_product_production_config` 操作日志动作；本需求没有新增绕过操作日志的用户触发写入口。

## 手册与人工验收

- 操作手册：`docs/OP_MANUAL_INVENTORY_MATERIALS.md`。
- 验收清单：`docs/ACCEPTANCE_TESTS.md` 的 K78。
- Van 验收重点：无模板列表/抽屉为空，模板切换只显示新模板字段，取消模板后字段清空，复制商品后模板和值均为空。

## 部署

- 未部署；本任务明确不部署。
- 后续部署前必须先备份目标环境数据库，并在合并前完成 Task 7 的 Vue/Vite build、`scripts/verify_kferp.sh changed` 和完整变更校验。
