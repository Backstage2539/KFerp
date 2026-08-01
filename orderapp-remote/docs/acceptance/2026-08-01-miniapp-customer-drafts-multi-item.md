# PR-569-MINIAPP-CUSTOMER-DRAFTS-MULTI-ITEM 验收记录

## 需求与开发项

- PR：`PR-569-MINIAPP-CUSTOMER-DRAFTS-MULTI-ITEM`
- `DEV-569-CUSTOMER-PERMISSION`：销售只能新增为本人负责的客户并修改本人负责客户；管理员可维护全部客户和负责人，后端拒绝越权并记录合法写操作。
- `DEV-569-ORDER-CUSTOMER-QUICK-EDIT`：录单选择客户后可原地维护，有权保存后刷新当前客户和默认收货资料。
- `DEV-569-ORDER-DRAFT`：每名登录员工只有一份自己的服务器录单草稿，完整保存全部订单字段和商品行，正式提交成功后由服务端清理。
- `DEV-569-MULTI-ITEM`：一张小程序新订单可新增、编辑、删除并一次提交多条独立商品规格明细。
- `DEV-569-AUDIT-DOCS-DELIVERY`：客户及草稿业务写入操作日志，需求、验收、手册和交付证据同步。

## 验收口径

- [ ] 销售新建客户时负责人固定为当前员工；销售修改本人负责客户成功，修改他人负责客户或改派负责人返回 403 且不产生客户或日志越权写入。
- [ ] 管理员可新增、修改任意客户并调整负责人；改派后销售的客户修改范围立即随负责人变化。
- [ ] 录单页选中有权客户后可直接维护，保存后仍停留在当前订单并刷新客户和默认值；本单收货资料未手改时同步最新资料，已手改时可选择同步或保留本单；无权客户只读且 API 同样拒绝。
- [ ] 同一订单可维护至少三条商品明细，每行商品、具体规格、数量、销售单位和单价独立；增删改任意行不会串行，合计正确。
- [ ] 切换客户后逐行校验商品范围；正式提交只生成一张订单，全部有效明细及顺序、数量、单价和合计完整落库。
- [ ] 每名销售或管理员只保存和恢复自己的一份服务器草稿；重复保存覆盖本人草稿，任何员工都不能读取或覆盖另一员工草稿。
- [ ] 草稿恢复保留订单日期、客户、收货快照、系统带入的收款/发货状态默认值、备注、全部商品行和行顺序；保存草稿不生成正式订单或下游业务影响。
- [ ] 单次正式提交只生成一张正式订单，提交处理中前端禁用重复操作，创建成功后服务端清理当前员工草稿。
- [ ] 客户新增/修改、负责人变化、草稿保存/清理等合法写操作可在操作日志按操作人和业务对象查到。
- [ ] `orderapp-remote/docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md` 和 `OPERATION_MANUALS.md` 已同步权限、多商品和草稿操作说明。

## TDD 证据

- RED（开始实施时暂定编号为 PR-568 / TestDev568，随后因共享 develop 先占用 PR-568 而顺延）：`go test ./internal/interfaces/http/support -run TestDev568MiniappCustomerDraftsMultiItemDocumentationContracts -count=1`
  - 结果：失败；实现前 `docs/ACCEPTANCE_TESTS.md` 缺少当时暂定的需求标识，证明需求、验收、PR/DEV 种子和手册合同尚未接入。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev569MiniappCustomerDraftsMultiItemDocumentationContracts -count=1`
  - 结果：通过，`ok orderapp/internal/interfaces/http/support`。合同确认根目录与线上需求/验收、PR/DEV 种子、验收记录、三份相关手册和总索引均包含 PR-569 权限、多商品和本人服务器草稿口径。

## 自动化与独立复核

- 后端：`go test ./... -count=1` 全部通过；覆盖客户读写双权限、负责人事务锁和越权回滚、来源/订单类型引用校验、未知仓储错误不泄露、草稿按员工隔离及正式订单事务内清理。
- 小程序：合并环境隔离后共 19 个测试文件、113 项测试全部通过；`vue-tsc --noEmit` 与 development/production 环境门禁下的 `mp-weixin` 构建通过。覆盖多行独立增删改、草稿完整恢复、加载完成前表单锁定、失效客户/商品保留并阻止误提交、手工单价、同 SKU 多客户别名身份贯通、快捷客户维护请求失效保护及全页面环境标识；草稿仓储异常统一为不泄露内部细节的 500。
- ERP Vue：853 项 Node 测试与 Vite production 构建通过；订单草稿可在操作日志按业务对象筛选和检索。
- 质量门：`scripts/verify_kferp.sh changed` 通过；独立后端、前端和文档复核提出的问题均已修正，当前无未关闭 P0-P2。

## 交付与人工验收

- 功能分支 `05828bc3b448dd97fdd8006c20f704b43b040936` 分别通过 development/production 远程预检；两次预检均未提升服务器源码、重启容器或替换固定小程序目录。
- `origin/develop` 应用提交 `fb792ead8db7fb6d985ea9d0dc3655a240e3836f` 已部署 development。服务器源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260801172738-fb792ead8db7`，回滚镜像为 `kferp-orderapp-rollback:development-20260801172738-fb792ead8db7`；`https://dev.qacoohee.com/app/login` 严格外部冒烟返回 HTTP 200。
- development 小程序包已同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，上一包备份为 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260801173306-fb792ead8db7`。
- `origin/main` 应用提交 `77477e3e8a9fc3d2116b1cf33e328c523538e61a` 已部署 production。服务器源码备份为 `/opt/stacks/erp-production/orderapp.backup.deploy-20260801173547-77477e3e8a9f`，回滚镜像为 `kferp-orderapp-rollback:production-20260801173547-77477e3e8a9f`；`https://erp.qacoohee.com/app/login` 严格外部冒烟返回 HTTP 200。
- production 正式小程序包已同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，上一包备份为 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260801174114-77477e3e8a9f`。上传、微信审核和发布仍须在微信开发者工具中人工完成。
- 两次发布均只重启目标应用容器；数据库、网关和另一环境未重启。本记录中的 HTTP 200 是部署脚本冒烟证据，不代替业务验收。
- PR 保持待 Van 验收；本记录不代替销售/管理员权限、多商品草稿恢复和正式提交的人工验证。
