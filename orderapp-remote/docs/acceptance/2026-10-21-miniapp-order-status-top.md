# PR-708 员工小程序订单详情顶部状态

日期：2026-10-21。产品验收人：Van，待验收。交付范围：develop 与 development，小程序正式上传/审核/发布另行处理。

## 问题与实现

原“编辑订单”在订单进入生产或发货后关闭，未付款订单无法便捷补录后续收款。详情页将“订单状态”移为第一张卡，提供“编辑付款与发货状态”，使用独立权限与状态接口，不重交商品、不重算冻结价格。原整单编辑门禁继续适用。

- DEV-813-STATUS-API：PUT `/api/mini/employee/orders/:id/status` 仅接受付款状态、方式、预付款、实收金额/凭证、发货状态及版本；事务行锁下重新检查版本、作废/履约限制及收款规则。只改付款时 SQL 不包含 `ship_status_id`，避免触发发货资料检查/进度重算；业务修改与双操作日志同事务，旧单据保留历史并失效当前版本。
- DEV-814-STATUS-TOP：顶部状态、独立编辑、预付款/收款方式、实收金额与收款截图上传、保存中禁用、失败保留草稿、成功刷新及刷新失败提示。POST `/api/mini/employee/orders/:id/payment-voucher` 复用网页上传与资产保存，员工身份、写权限和订单范围受限，8MB 上限，元数据失败清理文件。
- DEV-815-STATUS-DELIVERY：自动化、真实隔离数据库、375px 真实 Vue 页面本地验证、手册及开发环境交付。

## 自动化证据

| 范围 | 结果与证据 |
| --- | --- |
| TDD RED | 状态 API/事务先因缺失新接口失败：`status-api-red.log`、`status-transaction-red.log`；凭证上传 `status-upload-red.log`；前端先验证缺少状态生命周期和请求方法。 |
| API/真实 PostgreSQL | `ORDERAPP_TEST_DATABASE_URL=<isolated local PostgreSQL> go test ./internal/interfaces/http/customerportal ./internal/infrastructure/postgres/sales -run 'TestMiniEmployeeOrderStatus|TestOrderStatusUpdate' -count=1` 通过，`status-green.log`。 |
| 事务边界 | 普通/已发货补款、预付款、发货、已收款实收凭证、字段省略保留、他人凭证拒绝、免费样品/仅收运费、版本冲突、作废及履约门禁、商品价格不变、日志与单据失效；真实 UPDATE OF 触发器验证付款不触发发货列。 |
| 上传 | 定向 RED/GREEN，sales/support/customerportal 三包回归通过，`status-upload-green.log` / `status-upload-regression.log`。 |
| 小程序 | 65 文件 / 389 测试通过；typecheck 与受控 development 构建通过。 |
| Go | `go test ./...` 通过（无数据库环境时可选旧集成套件跳过；本次数据库定向套件单独明确通过）。 |
| Vue | 1,459 测试通过，Vite build 通过；现有员工小程序/销售手册入口读取更新后的 Markdown。 |
| 本地页面 | 真实 employee-order-detail.vue + 合成数据/UniApp 适配，375px 无横向溢出；生产中单未付款→已付款/微信支付，保存后状态与已付摘要刷新，已收款缺实收金额时保留表单并提示；local-ui-check.json 与截图留存。未写入线上业务数据。 |

测试文件：`miniapp/src/composables/useEmployeeOrderStatus.test.ts`、API client/customerPortal 测试、详情页结构测试、Go `order_status_test.go`、`mini_employee_status_test.go`、`mini_employee_payment_voucher_test.go`、共享上传测试。

## 操作手册与现有边界

- `docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`：顶部状态入口、付款/截图、保存反馈及限制。
- `docs/OP_MANUAL_ORDER_SALES.md`：同一操作流程，网页 `miniappEmployeeManual` / `orderSalesManual` 入口保持同步读取。
- 生产或已发货的有效单可补录付款。已收款仍须原有实收货款/运费/凭证，已付款仍须收款方式。免费样品无需收款沿用既有规则。
- 普通快递订单改“已发货”仍需有效物流公司和产品；本编辑器不新增物流配置入口，缺资料需先在 ERP 补齐。履约订单发货状态只读，由实际履约进度维护；待确认/拒绝状态需先完成现有确认流程。
- 客户不能使用员工接口，销售限本人订单，作废拒绝；新凭证须由当前员工上传，原单已有凭证可保留。

## 开发交付

集成、部署、回滚锚点及只读运行核对待部署后记录。未进行 development 业务验收或生产业务写入，未上传/审核/发布微信版本。
