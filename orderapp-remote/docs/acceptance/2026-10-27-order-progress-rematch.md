# PR-738 订单履约状态与寄件重新匹配

Van 业务验收待完成。分支 codex/order-progress-shipping-rematch。

目标：SO-20261022-0001/0002 显示生产完成、已入库；SO-20261022-000116/000117 显示库存充足、可以发货。ERP、客户小程序和员工小程序一致。生产仅重新匹配批次 1 的行 ID 2/4，不发货、不回填、不扣库存或记运费。

根因：原文件在入库之前导入，同文件重试返回持久化旧候选；页面直接显示原始发货字段，未统一交付语义。

## 验证

- RED：测试数据库复现状态变为可发货后同文件仍 Pending=0、unmatched；旧前端 helper 丢失生产完成信息。证据位于任务旁 evidence/pr738。
- GREEN：进度域单测、数据库库存自占用/版本/质量/余额/货主校验、重传仅匹配、审计失败回滚、并发重新匹配、权限/API；全部通过；Go 全量、Vue 1632/1632、Mini Program 436/436、两端构建及小程序类型检查通过。
- 接口：现有订单和代发包裹读取增加 fulfillment_progress；POST /api/orders/shipping-return-rematch 接收 batch_id 与可选 row_ids；仍由原权限边界保护。
- 仅 pending 行替换候选，已处理结果保留；无发货消息、费用或库存副作用。
- 手册：OP_MANUAL_ORDER_SALES.md、OP_MANUAL_CUSTOMER_FULFILLMENT.md、OP_MANUAL_SHIPPING_RETURN.md。

## 发布与验收

开发/生产提交、备份及线上验证待补齐。微信开发者工具已登录；微信管理后台被浏览器站点安全策略阻止，允许通过官方工具上传，但审核与正式发布需要 Van 在后台完成，不计为已发布。

只读生产核对：订单 116/117 的 4 笔实物分配与批次余额相等，按现行允许 unchecked、禁止冻结/拒收的质量规则，两单 physical_ready=true。已补同形回归及 RED/GREEN。
