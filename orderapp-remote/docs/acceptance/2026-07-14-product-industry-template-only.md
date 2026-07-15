# PR-536 商品行业字段仅来源于模板验收

- 日期：2026-07-14
- 状态：本地实现、目标测试和开发环境部署烟测已通过，等待 Van 验收；生产环境未部署。
- 需求：`PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY`

## 根因

- 前端曾在没有行业字段模板时继续展示已有商品生产配置字段，并允许按旧别名或显示名匹配模板字段；列表、抽屉、模板切换和保存 payload 的投影口径不一致，异步加载返回还可能覆盖当前商品或当前模板状态。
- 应用服务和 PostgreSQL 仓储曾允许 `industry_field_template_id=0` 时原样保存字段。`CopyProduct` 本来就不复制商品生产配置或行业字段模板，但曾有独立复制生产配置字段行的逻辑，因此会制造无模板孤立字段。
- 启动回填曾从 `products.roast_level` 和 `products.special_attrs_json` 通过 `jsonb_each_text` 自动生成商品行业字段；缺少幂等清理无配置孤儿、无模板字段和模板外字段的统一步骤。

## 目标行为

- 商品列表、商品档案配置抽屉、模板切换和保存只投影当前明确引用模板的精确字段键；无模板时统一为空。
- 编辑行业字段模板并删除或改名字段后，保存模板立即清除引用商品当前配置中的模板外字段；只修改当前 `product_production_config_fields`，不改历史订单、工单或价格表快照。
- 应用服务在模板 ID 为 0 时先把字段清为空切片，再执行字段校验并调用仓储；HTTP 层暴露应用服务返回结果。无模板保存响应必须包含 `fields`，值为 `[]`，不得为 `null` 或省略；Go 返回值必须是非 nil 空切片。
- PostgreSQL 仓储负责模板成员校验和字段元数据规范化，并为直接调用方重复执行无模板清空防御；HTTP 层不独立校验模板成员。
- `CopyProduct` 只复制商品主档基础资料、单位模板引用、单位覆盖和库存相关主数据；不复制商品生产配置、工艺路线、预期损耗率、商品配置模板、价格阶梯、生产 BOM、行业字段模板或行业字段值。复制后需重新配置行业字段、工艺路线、预期损耗率、生产 BOM 和价格/价格表。
- 启动清理幂等删除无配置孤儿、无模板字段和模板外字段，不再从旧商品列回填行业字段。

## RED 证据

- 前端：`node --test src/lib/product-settings.test.js` 的新增断言在实现前分别暴露无模板仍保留字段、仅显示名也被错误匹配，以及异步旧请求覆盖当前抽屉/模板投影的问题。
- 应用/API：`go test ./internal/application/catalog ./internal/interfaces/http/catalog -count=1` 的新增断言在实现前分别暴露应用服务未先清空无模板传入字段，以及 HTTP 保存响应没有稳定输出 `"fields":[]` 的问题。
- 仓储/迁移：`go test ./internal/infrastructure/postgres/catalog -count=1` 的新增断言在实现前暴露复制商品制造孤立字段、旧 `jsonb_each_text` 回填仍存在，以及缺少无模板/孤儿/模板外字段清理的问题。
- 文档合同：`go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1` 在本任务首次运行时退出码为 1，失败信息为 `docs/OP_MANUAL_INVENTORY_MATERIALS.md missing PR-536 marker "取消行业字段模板会清空商品行业字段"`。
- 文档合同 follow-up：强化六条 PR/DEV/REV 状态、当前复制边界、不歧义 `fields` 语义和双手册证据后，同一命令退出码为 1，失败信息为 `req_store.go must record both current PR-536 workflow manuals as DEV-536-DOCS-ACCEPTANCE evidence`。
- K20 冲突合同 follow-up：要求 K20 只保留为被 K78 / PR-536 覆盖的历史口径，并固定模板字段删除或改名后的即时清理边界；同一命令退出码为 1，失败信息为 ``docs/REQUIREMENTS.md missing PR-536 marker "删除或改名字段后，保存模板必须立即清除引用商品当前 `product_production_config_fields` 中已不属于模板的字段"``。
- 并发回归：模板保存和商品生产配置保存并发时，商品保存曾可能按旧模板定义通过校验，并在模板事务清理完成后成功回写已经删除的 `old-key`。

## GREEN 证据

- 前端：`node --test src/lib/product-settings.test.js` 通过 159/159，覆盖无模板清空、当前模板精确投影、拒绝旧别名/仅显示名匹配，以及抽屉异步结果防串写。
- 应用：`go test ./internal/application/catalog -count=1` 通过。
- HTTP API：`go test ./internal/interfaces/http/catalog -count=1` 通过。
- PostgreSQL 仓储：`go test ./internal/infrastructure/postgres/catalog -count=1` 通过；schema 回填不再包含 `jsonb_each_text`。真实清理 SQL 矩阵在一次性本地 PostgreSQL 中验证了首次执行、重复执行幂等、无模板、孤儿、模板外字段和首次启动缺少行业模板表的边界。该验证未连接开发或生产数据库。
- 并发与模板保存：`SaveIndustryTemplate` 在同一事务内按新模板定义立即清理引用商品当前配置中的模板外字段，并在模板保存操作日志元数据记录 `removed_product_field_count`；`SaveProductProductionConfig` 在读取模板字段定义前使用 `FOR SHARE`，与模板更新写锁串行化。一次性 LOCAL PostgreSQL 中 catalog 与 manufacturing 完整测试通过，真实验证商品保存等待模板锁，且模板提交后拒绝 `old-key`；development 和 production 均未触碰。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1` 和完整 support 包通过。PR-409 支持测试已删除对仓储内部 Go 返回字面量的绑定；无模板时的非 nil 空切片和 HTTP `"fields":[]` 语义由应用、HTTP API 与仓储行为测试负责。PR-536 支持合同另外固定六条 PR/DEV/REV 状态、历史 PR-439 口径、当前复制边界、双手册证据、K20 历史口径覆盖和模板字段删除/改名后的即时清理边界。
- Task 7 在 `ccfb36cf` 后重跑：`go test ./...` 通过；前端 `node --test src/lib/product-settings.test.js` 通过 159/159；Vue/Vite `npm run build` 通过，仅有既有 chunk-size warning；`scripts/verify_kferp.sh changed` 退出码为 0；`git diff --check` 通过。
- 开发部署：`./deploy_orderapp.sh development` 已部署 `origin/develop=207436b9d1dc1066f8afb9bb35e07d2b0ebe0c4a`；Docker 镜像内 `go test ./...`、Vue 构建、小程序 typecheck/微信构建通过。`erp_orderapp` 与 `erp_postgres` 正常运行，内部认证后的商品设置、需求和 Vue 商品档案入口均返回 200，需求 API 包含 PR-536。
- 开发数据烟测：部署前孤儿/无模板/模板外/总字段数为 `0/799/6/811`，部署后为 `0/0/0/6`，合法 6 条保留。商品设置 API 返回 498 个生产配置，`fields:null` 为 0，其中 492 个 `fields:[]`、6 个有合法字段。

## 数据边界

- 清理只删除 `product_production_config_fields` 中不满足当前模板约束的行。
- 保留行业字段模板和字段定义，不删除模板。
- 保留 `products.roast_level`、`products.special_attrs_json` 作为历史数据，但不再回填为商品行业字段。
- 保留已发布商品价格记录，以及历史订单、价格表、生产计划和工单快照；不回改历史业务快照。
- K20 的历史特殊属性回填口径已由 K78 / PR-536 覆盖；保留 `products.roast_level`、`products.special_attrs_json` 原列用于历史兼容，但不再据此生成当前商品行业字段，也不保留无模板或模板外的当前配置字段。
- 商品档案配置保存继续使用既有 `save_product_production_config` 操作日志动作；本需求没有新增绕过操作日志的用户触发写入口。

## 手册与人工验收

- 操作手册：`docs/OP_MANUAL_INVENTORY_MATERIALS.md`、`docs/OP_MANUAL_COSTING.md`。
- 验收清单：`docs/ACCEPTANCE_TESTS.md` 的 K78。
- Van 验收重点：无模板列表/抽屉为空且保存响应明确为 `"fields":[]`，模板切换只显示新模板字段，取消模板后字段清空；复制商品后行业模板和值、商品生产配置、工艺路线、损耗率、BOM 和价格配置均未带入，并按手册重新配置。

## 部署

- 开发环境已部署，合并提交为 `207436b9d1dc1066f8afb9bb35e07d2b0ebe0c4a`。
- 开发数据库部署前备份：`/opt/stacks/erp/backups/kferp-dev-pr536-industry-fields-predeploy-20260715121952.dump`；旧应用目录备份：`/opt/stacks/erp/orderapp.backup.deploy-20260715122139`。
- `dev.erp.qacoohee.com` 仍为 HTTP `000`，服务器当前只有 `erp_prod_caddy` 持有公网入口；本次没有切换 Caddy，也没有部署或修改 production stack。开发环境通过容器内部认证 API、源码标记、数据库清理结果和容器状态完成烟测。
