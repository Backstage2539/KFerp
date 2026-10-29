# 订单列表批量修改付款与发货状态

## 验收范围

- 复用订单列表当前页的统一选择；可单独或同时指定付款和发货状态。
- 逐单核对和补充收款资料，不共享金额或凭证；各订单独立提交，展示成功、未变化及失败原因，并可修正失败项后重试。
- 检查权限、客户范围、订单版本、付款要求和物流要求；留厂库存、履约订单、免费样品、作废及待确认订单遵守原有业务限制。
- 验证只改付款或发货时对另一状态和资料的保护，以及审计记录和单据失效。
- 操作手册：`docs/OP_MANUAL_ORDER_SALES.md`；网页操作手册入口：订单销售手册。

## 验证状态

- 前端：`node --test src/lib/order-batch-status*.test.js src/lib/order-list-selection.test.js src/lib/operation-manuals.test.js` 通过 14/14；`npm run build` 通过（有既有 chunk 大小警告）。
- 接口与隔离 PostgreSQL：设置 `ORDERAPP_TEST_DATABASE_URL` 指向隔离库后执行 `go test ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales ./internal/interfaces/http/customerportal -run 'Test(BatchOrderStatusAPI|OrderStatusUpdatePreservesContentAndRecordsAudit|MiniEmployee.*Status)' -count=1 -v`，3 个包均通过，覆盖状态事务、批量接口与员工小程序回归。
- 全项目编译：`go test ./... -run '^$'` 通过（仅编译，不运行测试）。
- 浏览器验收与 Van 业务验收：待完成。
- 开发、生产部署不在本次范围。
