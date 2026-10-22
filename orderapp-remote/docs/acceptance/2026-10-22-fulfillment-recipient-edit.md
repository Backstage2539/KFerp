# PR-713 代发收件信息编辑与客户规格报价

2026-10-22，Van 报告生产订单 SO-20261022-000117 仅修改地址却提示规格不在所选价格表。

## 复现与原因

生产只读 API 和现有编辑器确认：本单为库存代发，原明细为 prepaid_customer_stock、商品价零、未绑定销售价格表；普通销售编辑路径却会套用当前价格表并清空规格。客户表确实包含对应 BOM 规格，但报价快照制造版本早于当前制造版本，前端的严格变体匹配另行过滤掉了有效客户报价。没有改写实际订单、客户配置或价格表。

## 修复与验收

- 库存/无库存代发从原编辑入口显示专用收件信息编辑，原商品只读；PATCH 只传收件信息和原版本。
- 后端核对内部权限、实际客户、原单代发类型、并发版本和发货/作废状态，收件更新及审计同事务保存；不重建商品、费用、运费或库存占用。
- 客户报价以同一商品和稳定 BOM 规格匹配当前制造版本，保留报价快照；不混入其他商品、其他客户或表外规格。
- 失败保留输入，保存中阻止重复提交，成功刷新原单；慢速刷新期间保持专用编辑界面并暂时禁用修改，读取失败可重试，不显示普通录单商品控件。复制与普通新建保留原销售校验。

## 自动化证据

- Vue RED：原地址编辑把 product_id 从 7 清为 0；有效客户 BOM 规格报价在制造版本变化后消失。
- Vue GREEN：真实组件 setup/API 行为与客户报价隔离测试通过，日志位于本任务 outputs/order-address-spec-validation-20261022。
- 后端 RED：原实现新收件接口 404；受管确认订单初版错误接受修改，新增反例后封堵。数据库 GREEN 覆盖库存/无库存模式、完整订单非收件字段/明细/预留/请求/费用/运费快照、已发货/作废/并发提交、权限/客户隔离、未知商业字段、受管确认及旧门户待确认公司字段。
- 1482 项 Vue 测试及生产构建通过；完整 Go 测试（未设置数据库环境）通过；本地 PostgreSQL 中专项测试实际执行。手动档位平铺/分组快照回归通过，保留精确表、商品、规格及原发布行键。
- 本地真实 Vue 交互：仅修改地址并保存，PATCH 仅含收件四字段和旧版本；成功后出现“收件信息已保存”，两行原规格/数量保持。独立复审通过。
- 非本次范围的旧 CustomerWorkspaceContinuousOrdersAndRecipientIsolation 在实际 PostgreSQL 环境缺少夹具表，原实现对照同样失败，未通过修改生产逻辑掩盖；记录于 backend-existing-fixture-baseline.log。
- 实际 PostgreSQL API 进一步验证：旧发布快照报价 + 当前 BOM 变体的自动/手动档位均成功保存正确规格、单价及行键；显式过期变体仍拒绝且不增加订单。补全该专项旧测试缺失的隔离夹具关系/字段，未改变生产校验。日志 canonical-manual-api-green.log。

## 生产交付（2026-10-22）

- 特性/开发集成已推送：e33675e1；生产主线/运行版本：db427e4d0c6878fd2ceaff2b43410a9192fe8796，保留原 main-only BOM 发布保护。
- 命令：KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production。服务器 Vue1482、客户端425、类型/构建、主机与镜像完整 Go 测试及发布就绪检查通过。
- 回退：/opt/stacks/erp-production/orderapp.backup.deploy-20261022130428-db427e4d0c68；kferp-orderapp-rollback:production-20261022130428-db427e4d0c68。
- 运行：app running/restarts0、数据库 healthy；当前镜像 sha256:def55838c5848acb16176dd1329a1600bc339381fc63adb8dfb82b9df0270567 与运行标签一致。受保护收件 PATCH 匿名401；登录入口200。
- 静态资源 OrderEntryView-C4Xj4lpf.js 的源码、容器及公网 SHA256 均为 3ed92a2927363a17e955f40717be56ea755ba20dd5956798a825adf9c110a19b。
- 生产只读复核：订单117 edit_data 与发布前完全一致；原模板1/V2配置完全一致。新界面显示“修改收件信息/保存收件信息”，两行原227g规格各4袋，无价格表错误。没有替用户保存其未确认的新地址。
- 生产截图及验证摘要保存在本任务 outputs/order-address-spec-validation-20261022；实际保存事务已在隔离 PostgreSQL 回归验证。此记录为技术交付，Van实际地址保存及业务验收待完成。

- 手册：OP_MANUAL_ORDER_SALES.md、OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md；沿用 Vue 订单销售手册与客户履约手册入口。

Van 真实业务验收待完成。生产原订单只读核对，实际收件地址由 Van 保存。
