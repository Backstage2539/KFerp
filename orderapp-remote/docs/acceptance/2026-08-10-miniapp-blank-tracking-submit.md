# PR-590 订单非空文本字段兼容与小程序草稿提交安全验收记录

## 范围

- 需求：`PR-590-MINIAPP-BLANK-TRACKING-SUBMIT`
- 分支：`codex/fix-miniapp-draft-submit-null-tracking-20260810`
- 问题：production 员工小程序恢复服务器录单草稿后正式提交，未填写物流方式、物流单号、备注、快递费或明细单位/规格时，销售订单仓储把空值转换为 SQL `NULL`，与 `orders` / `order_items` 非空文本列约束冲突并使事务失败。同类风险还存在于批量失效的空原因和行内更新清空备注。
- 目标：订单所有受影响的非空文本列把空值写为空字符串；订单未完整提交时保留员工草稿；操作日志沿用原合同；完成 development 和 production 双环境交付。
- 边界：不修改小程序请求字段或微信端代码，不新增操作日志类型，不迁移历史订单，不自动上传、提审或发布微信小程序。

## DEV 合同

### DEV-590-ORDER-NOT-NULL-TEXT-COMPAT

- `SaveOrder` 的订单新建与完整更新、`updateOrderHeader` 的订单头更新统一使用非空文本语义；`ship_method`、`ship_tracking_no`、`notes`、`express_fee` 为空或仅空白时写入空字符串，订单明细空 `unit` / `spec` 指针通过 `notNullTextPtr` 规范化为空字符串，不得写 SQL `NULL`。
- `VoidMany` 的空 `void_reason` 使用空字符串；订单行内更新清空备注时，以规范化空字符串更新 `notes`，不能把空指针绑定为 SQL `NULL`。
- 非空文本、订单物流明细以及既有网页录单、编辑、失效语义保持原行为；本修复不改变 API payload。

### DEV-590-DRAFT-TRANSACTION-AUDIT-COMPAT

- 员工小程序正式提交继续通过既有 `SaveOrder` 事务保存订单头、明细和物流，并在同一事务内清除当前员工草稿、写入订单与草稿操作日志。
- 草稿清理必须位于最终事务提交之前；任何订单写入、草稿清理、审计写入或事务提交失败都会整体回滚，因此失败后草稿仍可恢复，不留下半张订单。
- 成功提交继续产生既有订单创建/更新日志和 `employee_order_draft` 删除日志；不新增实体类型或动作，不把 SQL 错误、草稿内容或失败请求写成成功日志。

### DEV-590-DUAL-ENVIRONMENT-DELIVERY

- 同步根目录与 `orderapp-remote/docs` 的需求、验收、小程序员工 ERP 手册、PR/DEV 种子、支持合同和本验收记录。
- 完成销售仓储定向测试、支持合同和受影响范围验证后，先合入最新 `develop` 并部署 development；随后从最新 `main` 合入已验证 develop，完成 production 预检后部署 production。
- 双环境部署必须串行并分别补充提交、服务器源码备份、回滚镜像和健康检查证据。服务器部署与微信小程序上传、提审、发布是独立检查点。

## TDD / 回归证据

- production 只读复现：PostgreSQL 日志在 `2026-08-09 14:04:23/31/38+08` 和 `2026-08-10 00:15:43+08` 均记录 `orders.ship_tracking_no` 的 `NOT NULL` 约束失败；失败事务没有生成正式订单，当前员工服务器草稿仍在。排查全程未重提订单、未写业务数据。
- RED 原因：订单新建/更新和订单头更新原先对 `ship_method`、`ship_tracking_no`、`notes`、`express_fee` 使用可空文本绑定，订单明细直接绑定可空 `unit` / `spec` 指针，批量失效对 `void_reason` 使用相同绑定，行内清空备注绑定空指针；空值因此被转换为 SQL `NULL` 并违反 production `orders` / `order_items` 的非空文本列约束。
- RED 输出：最初的物流单号合同失败于 `SaveOrder must not convert a blank shipping tracking number to SQL NULL`；扩展 production 表合同后，`TestOrderWritesKeepBlankNotNullTextFieldsNonNull` 继续暴露物流方式、备注、快递费、失效原因、行内备注和明细单位/规格的同类可空绑定。
- GREEN 目标：`TestOrderWritesKeepBlankNotNullTextFieldsNonNull` 约束上述写路径统一使用 `notNullText`、`notNullTextPtr` 或规范化空字符串，不再出现对应可空绑定。
- GREEN 结果：销售 PostgreSQL 仓储、customerportal HTTP、sales application 三个相关包通过；`TestMiniEmployeeOrderCreateAllowsBlankTrackingNumberForUnshippedDraft` 验证未发货且未填物流单号的请求仍进入正式订单服务，并带当前员工草稿清理标识。
- 事务回归：`TestFormalOrderSaveClearsDraftInsideOrderTransaction` 约束正式订单在 `tx.Commit` 之前清理草稿；既有员工草稿审计测试继续约束创建、更新、删除动作与脱敏元数据。
- 定向命令：`go test ./internal/infrastructure/postgres/sales -run 'TestOrderWritesKeepBlankNotNullTextFieldsNonNull|TestFormalOrderSaveClearsDraftInsideOrderTransaction' -count=1`。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev590MiniappBlankTrackingSubmitContracts -count=1`。

## Van 验收

- [ ] development 员工小程序恢复有效订单草稿，不填写物流方式、物流单号、备注和快递费可以成功提交，订单号生成且草稿清除。
- [ ] production 员工小程序执行同一场景，不再返回数据库内部错误；订单空文本字段和详情可正常读取。
- [ ] ERP 批量失效订单留空原因、订单行内更新清空备注均可成功，不产生非空约束错误。
- [ ] 提交失败或结果不确定时，先确认草稿仍在并到“查看订单”排除已经生成的订单，再重试，避免重复订单。
- [ ] 操作日志仅出现既有订单与订单草稿记录，没有新的日志类型、SQL 内部信息或失败成功记录。

## 交付状态

- `DEV-590-ORDER-NOT-NULL-TEXT-COMPAT`：done，仓储实现和定向回归已落地。
- `DEV-590-DRAFT-TRANSACTION-AUDIT-COMPAT`：done，既有事务顺序和操作日志合同保持不变。
- `DEV-590-DUAL-ENVIRONMENT-DELIVERY`：done，development 与 production 已依次完成交付。
- `REV-590-MINIAPP-BLANK-TRACKING-SUBMIT`：todo，等待双环境交付后由 Van 验收。

## 部署证据

- Development：`develop@062c9be73bc3f1a5376316ab4dd402dce581c05a`；服务器源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260810011606-062c9be73bc3`；回滚镜像 `kferp-orderapp-rollback:development-20260810011606-062c9be73bc3`；内置门禁和 `https://dev.qacoohee.com/app/login` HTTP 200 通过。
- Production preflight：发布候选 `91cda66ad15141dfca3746f65380d7ce264557d6` 通过 Vue 925/925、小程序 195/195、类型检查/production 构建、完整 Go 和隔离镜像构建；预检没有提升源码或重启容器。
- Production：已验证 develop 合入 `main@91cda66ad15141dfca3746f65380d7ce264557d6`；服务器源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260810013030-91cda66ad151`；回滚镜像 `kferp-orderapp-rollback:production-20260810013030-91cda66ad151`；内置健康检查和 `https://erp.qacoohee.com/app/login` HTTP 200 通过。
- 两次部署均设置 `KFERP_SKIP_MINIAPP_EXPORT=1`：服务器仍构建并验证相应环境的小程序包，但未替换本机固定包，也未上传、提审或发布微信版本。未自动提交 production 真实草稿，保留给 Van 人工验收。
