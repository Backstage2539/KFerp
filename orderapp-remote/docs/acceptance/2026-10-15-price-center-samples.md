# PR-692 价格表中心与样品申请闭环

代码、自动化测试及本地布局核对已完成；正在进行开发集成。Van 产品验收待进行，生产发布、微信上传／审核／发布未执行。

## 范围与规则

从 `origin/develop@25b377af` 建立 `codex/price-center-samples-20261015`。保留旧页面路径／入口键，统一价格表中心名称。销售与管理员可访问中心所选官方发布表；客户专属权限沿用。登记按后台微信身份恢复，收件电话独立于验证手机号。

管理员首次分配生成唯一工单，联系确认后原子保存客户、正式销售订单和工单关联。零价仅在内部样品事务的有效 BOM 规格订单、以及数据库核实的既有样品订单原明细编辑中允许；普通手工新建、复制和新增行仍拒绝零价。新客户不自动开放门户。回访与发货独立，首次分配起七天，确认寄样／有效回访重算；改派、阅读、发货不重算；成交／放弃停止。

## RED → GREEN

- 缓存清除恢复测试最初失败（未调用微信身份恢复），改为以服务端登记结果为准，网络错误不弹授权。
- 新增样品应用／HTTP／商品标识测试先因接口与模型未实现失败，随后通过。
- 员工缺失身份、样品视图权限、API 权限测试先失败，补齐销售／管理员限定后通过。
- 小程序与 Vue 样品辅助逻辑先因模块缺失失败；金额、规格隔离、确认条件、接口作用域随后通过。
- 真实数据库免费样品确认最初返回 `手动单价必须大于0`；增加内部样品事务的窄化例外后免费／收费均通过，普通订单零价仍拒绝。
- 消息已读 SQL 参数类型不一致，明确 bigint 后通过；他人收件消息不能被写入已读记录。
- 八并发提交暴露持锁事务再次租用连接导致连接池耗尽；发布表读取改为同一事务后并发防重通过。
- 完整申请号及收件人／电话搜索原先无结果，查询补齐后通过。
- 小程序标准 API `message` 错误体被吞的测试先失败，修复后能展示具体失败原因。

- 小程序非价格表文章／功能页原本被员工角色误拦；限制收窄后回归通过。
- 旧微信恢复响应原本能覆盖新的员工登录；响应绑定会话代次后通过。
- 样品订单再次编辑原本拒绝零价、要求普通价格表，或由客户端自动改价；保留可信样品原明细及价格快照后，新增／复制／伪造明细仍执行普通校验。
- 样品保存末尾不再自动附默认价格表；订单编辑 JSON 保留明细 ID、原 BOM 身份及零价，并由数据库验证历史样品明细；新行、其他订单明细、替换商品和重复明细身份均拒绝零价。
- 申请提交增加并发、离页、存储失败和未知网络结果重试测试；迟到响应不能删除新草稿。

## 验证矩阵

| 要求 | 自动化／本地核对证据 | 结果 |
|---|---|---|
| 角色权限、专属价格隔离 | `price_center_access_test.go`、样品 API、`TestSamplesPostgresAccessSubmissionAndReminders` | 通过 |
| 首次登记／取消／失效恢复／清缓存／停用／网络失败 | `registrationSession.test.ts`、`useBeanRegistration.test.ts`、原 `TestBeanCenterRegistrationPostgres` | 通过；真机跨设备仍待人工 |
| 商品来源／版本变动／停用／联系快照 | `TestSamplesPostgresConcurrentSubmissionAndPublicationChange` | 通过 |
| 并发申请、改派、唯一工单和订单 | 八并发提交、四并发确认与数据库唯一约束断言 | 通过 |
| BOM、已有客户、新建客户、金额与收件资料 | `TestSamplesPostgresAtomicConfirmation`：免费、收费 29.60 + 8.50 = 38.10；不授予门户；既有归属保留 | 通过 |
| 样品订单后续编辑 | 真实员工 GET/PUT 订单接口修改地址；免费／收费原明细保留规格、金额和来源；新行／他单／替换商品／重复 ID 不享受零价例外 | 通过 |
| 真正的事务回滚 | 最后关联工单处触发器主动失败，客户／订单／日志／消息与工单均回滚 | 通过 |
| 七天周期、重置、改派、关闭、恢复 | 定时边界、四并发扫描只发一次、40 天停机只补当前周期、下周期继续；发货不停止 | 通过 |
| 消息权限及完整未读数 | 收件人隔离、两端已读共享、超过 100 条总未读与历史分页、配置规则不扩展外部发送 | 通过 |
| ERP 布局与金额 | 同组件本地假数据 1280px／390px，部分商品寄样与 3 × 2.30 + 5 = 11.90；截图 `/tmp/kferp-sample-ui-evidence/` | 通过；无共享业务数据操作 |

## 测试命令与结果

- `scripts/verify_kferp.sh backend`：全量 Go 测试通过（未配置数据库的旧集成测试按原规则跳过）。
- `ORDERAPP_TEST_DATABASE_URL=postgres:///kferp_samples_20261015?sslmode=disable go test -race ./internal/appmain -run '^TestSamplesPostgres' -count=1 -timeout=2m`：四组真实 PostgreSQL 集成测试通过，race 无告警。每次使用独立 schema 并清理。
- 同一隔离数据库执行 `TestBeanCenterRegistrationPostgres|TestPageEntryPostgres|TestPageEntryAccessPostgres`：通过。
- `scripts/verify_kferp.sh frontend`：1362 项 Vue 测试与生产构建通过。
- `npm test --prefix miniapp`：52 个文件、323 项测试通过。
- `npm run typecheck --prefix miniapp` 与 `npm run build:mp-weixin:development --prefix miniapp`：通过。
- `git diff --check`：通过。

本机日志：`/tmp/kferp-samples-backend-final.log`、`/tmp/kferp-samples-integration.log`、`/tmp/kferp-sample-item-id-vue-gate.log`。不把本地假数据核对或构建通过记为真人业务验收。

## 手册和五表

- 独立手册：`docs/OP_MANUAL_SAMPLES.md`，Vue 入口 `sampleRequestsManual`。
- 更新 `OP_MANUAL_BEAN_LIST_CENTER.md`、`OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、门户、消息、页面入口与公众号手册及总索引。
- PR-692 为 review，DEV-765～770、UT-692、API-692 为 done；运行部署另记实际回执，REV-692 等待 Van 验收。

## 交付记录

开发环境部署提交、备份路径、健康检查和开发小程序产物将在实际完成后记录。生产和微信发布未纳入本次执行。
