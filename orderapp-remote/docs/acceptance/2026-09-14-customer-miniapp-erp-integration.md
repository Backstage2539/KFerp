# PR-663 客户小程序与 ERP 业务适配验收记录

## 目标与边界

本需求把客户小程序的一件代发、代加工工单、客户库存、客户商品物流和客户账单接入 ERP 现有价格、订单、生产、库存、应收、费用与结算链路。新增客户写操作按登录身份锁定客户范围并记录操作日志；历史订单、工单、价格、预留和账单快照保留，不批量回算。

本次交付目标为 `develop` 与 development 环境。正式环境部署以及微信小程序上传、审核和发布不在范围内。

## 最终行为

| 范围 | 验收结果 |
| --- | --- |
| 一件代发 | ERP 价格表可标记“适用于一件代发”，门户为客户绑定稳定表标识；小程序只能读取指定表。服务端重新核价并校验报价签名，保存实际发布版本和价格快照；同一请求幂等。提交生成共享 ERP 订单，缺货复用订单现有生产链路。 |
| 代加工工单 | 商品档案维护“是否代加工商品”；目录只含当前客户专属、已标记且生产配置完整的商品。预览分开返回配置有效性和物料齐套结果；缺料允许提交，申请直接进入待排产且不预留物料。同一提交标识重试返回原申请，内容变化返回冲突。回传真实计划、工单、部分完成和实际入库数量。 |
| 客户库存 | 按客户货权展示成品、生豆、包材和半成品；分别保留单位。明细显示仓库、规格、批次、可用、占用、生产中、质量状态和流水；生产中扣除已入库数量且不计入可发库存。 |
| 物流 | 发货中心关联 ERP 订单、商品、生产及发货状态，支持部分发货、一单多包裹、运单和已有物流轨迹；没有轨迹时明确提示。 |
| 客户账单 | 商品货款、加工费、代发服务费、运费、退款和调整按 ERP 来源归集去重；订单已包含费用不重复计费。支持 PDF/Excel、确认对账、针对费用项提出异议及 ERP 回复。确认不改变付款状态，账单内容改变后要求重新确认。 |
| 页面 | 首页增加待处理、生产、库存和待对账摘要；生产页默认列表并由“新建工单”进入表单；库存按类型查询；个人中心显示当前客户、联系人和常用收件人；空列表隐藏分页并给出下一步。 |

## TDD 证据

- RED：发布批次测试最初缺少代发标记；商品接口与前端表单最初缺少代加工标记；生产申请测试最初仍因缺料拒绝并写预留；客户库存和统一账单契约测试最初缺少新接口；小程序页面契约最初仍读取旧客户成品仓和旧费用入口。
- RED：`TestMiniDirectShipPriceQuoteRequiresExactPublishedPriceSnapshot` 首次运行因报价签名和价格变化错误不存在而编译失败；实现后转为 GREEN。
- RED：`TestMiniDirectShipCancellationOnlyAllowsLegacyUnshippedReservations` 首次运行因新 ERP 订单取消边界不存在而编译失败；实现后转为 GREEN。
- RED：客户库存生产中数量契约首次缺少“目标数量减实际入库数量”；实现后转为 GREEN。
- RED：代加工申请最初没有提交标识，网络重试会生成重复待排产需求；新增客户级防重键、内容摘要和并发锁后转为 GREEN。
- GREEN：任务验证脚本通过相关 Go 应用、PostgreSQL、API、PDF 与 Excel 包；ERP Vue 1214 项测试全部通过且构建成功；小程序 39 个测试文件、249 项测试全部通过，类型检查及 development 微信包构建成功。合入最新 `develop` 后会再次执行。

真实 PostgreSQL 定向验证使用一次性本机数据库完成：代加工请求覆盖首次写入、同内容重试复用原 ID、变更数量冲突、只有一条待排产需求和一条提交日志；统一账单覆盖订单、独立费用、已付金额、应付金额、客户隔离和分页。旧的无 BOM 规格测试夹具不属于本次新流程，未用它们放宽当前 ERP 的 BOM 规格权威校验。

## 接口与权限证据

- 一件代发：`GET /api/mini/direct-ship/catalog`、`POST /api/mini/direct-ship/preview`、`POST /api/mini/direct-ship/requests`；客户 ID 由 mini 登录上下文注入，客户端价格和选表不作为权威。
- 生产：`GET /api/mini/processing/catalog`、`POST /api/mini/processing-requests/preview`、`POST /api/mini/processing-requests`；目标商品归属和标记由服务端再次校验。
- 库存：`GET /api/mini/customer-inventory/assets` 与流水接口；客户 ID 由登录上下文注入。
- 账单：`GET /api/mini/customer-account`、PDF/Excel 下载、确认和异议接口；ERP 异议列表/回复继续使用 `customers.read` / `customers.write` 权限。
- 操作日志：价格表发布、商品代加工标记、门户能力及指定价格表、一件代发提交、生产申请、对账确认、异议和 ERP 回复均写审计记录。

## 文档与文件输出

- 客户履约手册：`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- 客户账单手册：`orderapp-remote/docs/OP_MANUAL_CUSTOMER_ACCOUNT.md`
- 前端手册入口：ERP “客户履约 → 客户履约手册”，由 Vue/Vite 通用手册页读取上述单一来源。
- PDF 视觉检查：三页 A4 中文可读，订单摘要、独立费用、正式结算单和异议回复分页正常；本地证据 `/private/tmp/kferp-pr663-acceptance/customer-account.pdf` 及对应 PNG。
- Excel 内容检查：四个工作表包含订单账单、费用明细、正式结算单和账单异议，金额、来源、付款和对账字段可读取。

## 自动验证与 development 验收

待最终全量验证、合入和 development 部署后补充：

- 功能分支、合并提交和运行版本。
- Go 全包、Vue、miniapp、类型检查和构建结果。
- development 服务、数据库、页面/API smoke 与回滚信息。
- 使用 development 完整业务数据逐页核对的结果及仍需 Van 人工确认的业务口径。
