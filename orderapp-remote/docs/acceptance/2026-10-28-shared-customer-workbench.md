# PR-744 客户履约与留厂购货验收证据

范围：员工及客户 ERP 共用履约页面；库存代发按客户、仓库和规格显示可追溯的累计购买与入库；新留厂购货统一未付款。历史订单、收款和库存未批量改写。生产及微信正式发布不在本次范围。

## 自动化验证

- RED：累计数量接口缺失、同规格多行缺口、客户范围读取接口缺失、伪造收款和内部越权接口分别在对应新测试中失败；实现后 GREEN。
- PostgreSQL 隔离测试：部分及全部入库、仓库和客户隔离、历史来源缺失、已售完后累计数量、新留厂单未付款、伪造收款回滚、原留厂订单编辑及小程序留厂接口；`purchase_totals_test.go`、`retained_payment_test.go`、`retained_order_save_test.go`、`mini_retained_purchase_api_test.go`。
- HTTP 测试：客户绑定范围与账单能力检查、模拟两个客户切换、客户调用内部库存/出账接口被拒绝、小程序伪造付款字段被拒绝。
- Vue DOM 和运行时：员工及客户五个模块、隐藏未授权操作、地址簿、同规格跨版本缺口、新建与历史订单付款字段；`fulfillment-operations-dom.test.js`、`shared-workbench-runtime.test.js`、`order-entry-save-runtime.test.js`、`customer-direct-ship-runtime.test.js`。
- 全量 Go、Vue 测试与构建；小程序测试、类型检查与开发构建。具体验证结果以本次交付报告为准。
- 手册：`docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`docs/OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md`、`docs/OP_MANUAL_CUSTOMER_PORTAL.md`；既有 Vue 手册入口继续指向客户履约手册。

## 待业务验收

真实员工和两个真实客户账号的开发环境页面验收须单独记录；小程序开发包构建不代表真机验收、微信审核或发布。不得为验收直接修改生产订单、库存或收款。
