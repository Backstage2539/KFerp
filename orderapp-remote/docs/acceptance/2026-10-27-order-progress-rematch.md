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

首轮开发发布 4c494c94d373、生产发布 ad478d0ca95e 完成。ERP 实际列表、详情及两个候选弹窗已核对；员工小程序真实接口及开发者工具列表显示一致。客户小程序正式版本验收仍待完成。

- 开发备份：`/opt/stacks/erp/orderapp.backup.deploy-20261027112948-4c494c94d373`。
- 生产备份：`/opt/stacks/erp-production/orderapp.backup.deploy-20261027114237-ad478d0ca95e`。
- 正式重新匹配仅提交 batch_id=1、row_ids=[2,4]，Excel 第 3/5 行分别唯一匹配订单 116/117。操作日志 8674（2026-10-27 11:54:33 +08:00）记录 actor 刘祎泊、action rematch、matched_only=true。
- 重匹配前后订单、运费、费用、运单、批次、汇总库存和分配的快照哈希全部相同。两行仍 pending、selected_order_id 为空；候选弹窗查看未执行“使用此单”。
- 小程序官方开发者工具上传版本 `20261027.1156` 成功，1,434,498 字节。微信管理后台被浏览器站点安全策略阻止，Van 回复“稍后处理”；审核及正式发布待 Van 处理，不计为已发布。
- 扩展的数据库全套验证发现 86 个旧夹具失败；以原生产源码 4db5ed982 对照，失败集合完全相同、无新增。相关定向 PostgreSQL/API 测试及无外部数据库的完整 Go 测试通过；不宣称全部数据库集成测试通过。

只读生产核对：订单 116/117 的 4 笔实物分配与批次余额相等，按现行允许 unchecked、禁止冻结/拒收的质量规则，两单 physical_ready=true。已补同形回归及 RED/GREEN。

## “使用此单”历史规格报错修复

Van 随后报告 `bom_variant_not_current: product 7 spec 19 variant 558`。只读快照确认该次失败整体回滚，未留下运单、费用、发货或库存变化。原因：历史生产入库和分配可以保留冻结版本，但发货新建出库流水与出库记录仍要求当前版本。

- RED：真实 HTTP `shipping-return-selection`、真实数据库身份触发器下，V1 批次分配给同规格 V2 订单，复现 HTTP 409 `bom_variant_not_current`。
- 修复：仅允许与已有订单实物分配完全对应的历史批次出库流水，以及有该流水支撑的出库记录；核对商品、稳定规格、冻结版本、批次、货主、仓库、订单明细及分配数量。不更改生产历史，不允许借发货来源新增旧版本库存。
- GREEN：真实确认接口成功并保留 V1；重复及并发确认只记一次；错误订单/批次/仓库/货主、正向入库、超额数量、错误余额、无出库流水的扣减记录全部被拒绝。模拟最后更新寄件行失败，订单、运费、库存、流水及日志快照完全回滚。原历史生产入库与身份限制测试保持通过。
- 开发环境更新至 `6626c6a47b792e2b47c656f7b1190a6cc56444af`，备份 `/opt/stacks/erp/orderapp.backup.deploy-20261027132546-6626c6a47b79`；生产更新至 `bbee1003a682505c67735c7013e7cf70c597f836`，备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261027133802-bbee1003a682`。两次均使用 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh <environment>`；部署脚本全量后端检查、Vue 1632 项、小程序 436 项及镜像构建通过。应用健康、登录页外部检查通过。
- 生产身份触发器已装载 shipment 追溯分支，stock ledger 和 order stock deduction 两个出库表的触发器均启用。订单 116/117 的四笔 allocation 均有匹配的历史批次、同货主、同仓库、0g规格和数量；变体 558 等留在历史批次，不回写当前订单版本。
- ERP 生产列表/详情、员工小程序生产列表/四张详情接口均返回原定状态。小程序开发工具的员工列表和四张详情也已核验，留厂单显示“生产完成 · 已入库”、库存代发单显示“库存充足 · 可以发货”。
- 第二次发布前后订单、批次、分配、汇总库存、运单、实际运费和费用快照完全一致；错误确认未留业务写入，batch 1 的行 2/4 仍为 pending，selected_order_id 为空，候选仍唯一对应 116/117。发货状态为待发货，运单为空。
- 最终只读核验结果：`../evidence/pr738/frozen-shipment-final-verification.json`。没有使用生产接口执行“使用此单”。
- 微信小程序版本 `20261027.1156` 已上传；Van 选择稍后处理审核及正式发布。客户小程序正式版本验收和 Van 业务验收仍待完成。
