# 客户履约账户与 Excel 导入闭环设计

日期：2026-05-07

## 背景

Van 的代加工客户会把自己的生豆、包材和成品托管在棵凡烘焙厂。客户通过共享文档提交烘焙工单和一件代发订单，棵凡负责烘焙、包装、库存维护、发货和月度结算。

本轮样本文件：

- `/Users/yiiiple-work/Downloads/誉观山生产工单&物料库存.xlsx`
- `/Users/yiiiple-work/Downloads/誉观山&口加-代发.xlsx`
- `/Users/yiiiple-work/Documents/cofe/订单/誉观山咖啡/YGS-DJG-20260304.xlsx`

第一版确认采用“共享文档导出 Excel 后在 ERP 导入处理”，不接腾讯文档自动同步。

## 总体目标

建立一套可复用的客户履约账户模型，覆盖代加工客户的托管资源、生产履约、一件代发和月度结算。誉观山只是首个模板样本，系统不能把业务写死为单一客户。

用户在 ERP 中选择客户并导入 Excel。系统解析后把数据落入客户专属 SKU、客户托管库存、生产工单/加工申请、代发订单、费用明细和结算单。小程序按客户能力展示库存、工单、订单、物流和结算。

## 业务抽象

### 客户履约账户

客户履约账户绑定现有 `customers.id`，由客户门户配置和服务能力控制：

- `processing`：代加工工单和成品入客户仓。
- `direct_ship`：导入下游收件人订单并由棵凡发货。
- `inventory_custody`：客户生豆、包材、成品托管库存。
- `settlement`：费用明细和结算单。

客户类型只是标签。真正驱动功能的是能力、库存资源、履约记录和计费规则。

### 客户资源

客户资源分三类：

- 生豆：客户购买或自带，托管在棵凡原料仓，生产时按客户工单消耗。
- 成品：客户专属 SKU 生产完成后进入客户专属成品仓。
- 包材：有些向棵凡采购，有些客户自带，库存和费用独立计算。

资源必须支持流水。不能只保留当前库存，否则无法追溯导入、工单消耗、发货扣减和盘点调整。

### 客户产品

复用现有客户专属 SKU 模型：

- 公共产品：`products.customer_id=0`，`visibility='public'`。
- 客户产品：`products.customer_id=<customer_id>`，`visibility='customer_only'`。

代加工客户的产品进入 ERP 产品表，但不会进入公共产品列表，也不会被其他客户豆单或订单选择。生产、库存、BOM、成本继续以 `product_id` 为核心。

## 样本 Excel 字段映射

### 代加工工单与物料库存

`生豆入库表`

- 字段：原料编号、生豆名称、入库时间、入库数量、入库后剩余库存、备注。
- 落点：客户资源入库流水；必要时创建或匹配客户生豆资源。

`生豆出库表`

- 字段：工单编号、出库时间、生豆名称、出库数量、备注。
- 落点：客户资源出库流水；如果工单编号存在，关联加工工单；否则作为历史消耗导入。

`生豆库存表`

- 字段：原料编号、生豆名称、当前库存、最后更新日期、备注。
- 落点：客户资源当前快照与导入核对，不直接覆盖流水；差异生成盘点调整流水。

`生产工单`

- 字段：工单编号、下单日期、生豆名称、烘焙总量、状态、烘焙完成日期。
- 落点：客户加工工单主记录；状态同步到客户门户加工申请和生产计划。

`生产子工单-包装`

- 字段：日期、状态、成品名称、生产日期、预期包装数量、实际包装数量、烘焙工单编号、备注。
- 落点：加工工单包装明细；完工数量进入客户专属成品库存。

`库存转换工单`

- 字段：工单编号、原 SKU、原批次号、预期/实际出库、新 SKU、预期/实际入库。
- 落点：客户成品库存转换流水，例如挂耳盲盒、磨粉、重新包装。

`SKU`

- 字段：产品编号、产品名称、规格、单位。
- 落点：客户专属 SKU；匹配已有 SKU 时更新外部编号，不重复创建。

`耗材库存（预估）`

- 字段：品类、库存预估、更新时间、备注。
- 落点：客户包材资源快照与调整流水。

`生豆报价表`

- 字段：编号、品类、kg 价、麻袋价、更新日期、备注。
- 落点：客户资源采购/报价参考，第一版作为资源价格备注和后续结算规则输入，不直接驱动成本核算。

### 代发清单

`代发信息`

- 字段：时间、序号、订单编号、收货地址、商品标题、属性、商品规格、数量、磨粉服务、备注、运单号、发货日期、状态。
- 同一订单编号可能多行商品；空订单编号/地址沿用上一行订单头。
- 落点：导入批次、代发订单、订单明细、收件人快照、物流字段。

导入后创建普通 `orders`，但带：

- `customer_id=<客户>`
- `portal_service_code='direct_ship'` 或 `processing_ship`
- `receiver_*` 收件人快照
- `source_warehouse=<客户成品仓>`

下游收件人不进入 `customers`。

### 月度结算单

结算单包含：

- 烘焙费：按锅或按 kg 阶梯计费。
- 磨粉/装袋费：按袋计费。
- SC 挂靠费：按产品最小售卖单元计费。
- 挂耳加工费：按包数量阶梯计费。
- 挂耳装盒费：按盒计费。
- 代发服务费：按件计费。
- 生豆仓储费：按 kg/月计费。
- 物流费用：通常转私账或单独统计。
- 其他费用：人工调整项。

落点为 `customer_fee_items`，再按期间生成 `customer_settlement_batches`。

## 数据模型

### 复用现有表

- `customers`：客户主体。
- `customer_portal_profiles`：客户门户配置、客户专属仓、默认寄件人。
- `customer_service_capabilities`：客户能力。
- `products`：客户专属 SKU。
- `finished_inventory`：客户成品仓库存。
- `orders` / `order_items`：代发订单与发货订单。
- `customer_processing_production_demands`：加工申请进入生产计划。
- `customer_fee_items`：费用明细。
- `customer_settlement_batches`：客户结算单。

### 新增客户履约导入表

`customer_fulfillment_import_batches`

- `id`
- `customer_id`
- `import_type`：`processing_workbook`、`direct_ship_workbook`、`settlement_workbook`
- `source_filename`
- `source_sha256`
- `status`：`parsed`、`applied`、`failed`
- `total_rows`
- `valid_rows`
- `invalid_rows`
- `summary_json`
- `error_json`
- `created_by`
- `created_at`
- `applied_at`

用途：每次 Excel 导入都有审计记录和解析摘要，支持幂等和排错。

`customer_fulfillment_import_rows`

- `id`
- `batch_id`
- `sheet_name`
- `row_no`
- `row_type`
- `external_key`
- `status`
- `payload_json`
- `error`
- `target_type`
- `target_id`

用途：保存每行解析结果和落库目标，方便 UI 展示导入错误。

### 新增客户资源表

`customer_custody_items`

- `id`
- `customer_id`
- `item_type`：`raw_bean`、`packaging`、`finished_product`
- `external_code`
- `item_id`：可指向 `materials.id` 或 `products.id`，没有主数据时为 0。
- `item_name`
- `spec_text`
- `unit`
- `unit_cost`
- `active`
- `created_at`
- `updated_at`

`customer_custody_ledger_entries`

- `id`
- `customer_id`
- `custody_item_id`
- `item_type`
- `movement_type`：`receipt`、`issue`、`adjustment`、`production_output`、`conversion_out`、`conversion_in`、`shipment`
- `source_type`
- `source_id`
- `external_doc_no`
- `occurred_at`
- `qty_g_change`
- `qty_units_change`
- `qty_g_after`
- `qty_units_after`
- `unit_cost`
- `note`
- `import_batch_id`
- `created_at`

`customer_custody_balances`

- `customer_id`
- `custody_item_id`
- `warehouse`
- `qty_g`
- `qty_units`
- `updated_at`

这些表服务客户托管资源。它们不替代公司通用库存表，而是在客户维度记录资源权属和客户可见余额。客户成品仍可同步到 `finished_inventory`，用于现有发货扣减。

### 新增加工履约表

`customer_processing_work_orders`

- `id`
- `customer_id`
- `external_work_order_no`
- `order_date`
- `status`
- `roast_completed_at`
- `note`
- `import_batch_id`
- `created_at`
- `updated_at`

`customer_processing_work_order_inputs`

- `id`
- `work_order_id`
- `custody_item_id`
- `raw_bean_name`
- `expected_qty_g`
- `actual_qty_g`

`customer_processing_packaging_jobs`

- `id`
- `work_order_id`
- `external_row_no`
- `product_id`
- `product_name`
- `spec_text`
- `expected_qty_units`
- `actual_qty_units`
- `status`
- `production_date`
- `note`

`customer_inventory_conversion_jobs`

- `id`
- `customer_id`
- `external_job_no`
- `source_product_id`
- `source_sku_name`
- `source_batch_code`
- `target_product_id`
- `target_sku_name`
- `expected_out_units`
- `expected_in_units`
- `actual_out_units`
- `actual_in_units`
- `status`
- `import_batch_id`

这些表保留客户工单语义，再同步到现有生产计划、成品库存和库存流水。

### 新增代发导入明细

`customer_direct_ship_import_orders`

- `id`
- `batch_id`
- `customer_id`
- `external_order_no`
- `external_seq`
- `order_date`
- `recipient_raw`
- `receiver_name`
- `receiver_phone`
- `receiver_address`
- `remark`
- `tracking_no`
- `shipped_at`
- `status`
- `order_id`

`customer_direct_ship_import_order_items`

- `id`
- `import_order_id`
- `product_id`
- `product_name`
- `attribute_text`
- `spec_text`
- `qty`
- `grind_service`
- `note`

这些表保存原始导入数据。系统创建的真实履约订单仍落在 `orders` / `order_items`。

### 计费规则

`customer_billing_rules`

- `id`
- `customer_id`
- `rule_type`：`roasting`、`grinding`、`bagging`、`drip_bag`、`boxing`、`sc_license`、`direct_ship_service`、`storage`、`packaging_material`
- `config_json`
- `active`
- `updated_at`

第一版提供内置规则配置，不做复杂公式编辑器。规则配置例：

- 烘焙：按 kg 分段单价，低于 2kg 时按锅计费。
- 代发：每单固定服务费。
- 仓储：按 kg/月。
- 磨粉/装袋/SC：按袋或按产品件数。

## ERP 页面

新增 Vue/Vite 页面 `CustomerFulfillmentView.vue`，菜单名称“客户履约账户”。

页面结构采用工作台式布局：

- 左侧：客户搜索和导入批次列表。
- 中间：当前客户的工单、代发订单、库存和费用 tabs。
- 右侧：导入面板、解析摘要、错误行和操作按钮。

核心操作：

- 选择客户。
- 上传或选择本地 Excel 文件。
- 选择导入类型：代加工工单/代发清单/月结结算单。
- 解析预览：显示有效行、错误行、将创建/更新的对象数量。
- 应用导入：写入库存、工单、订单、费用。
- 查看导入历史。
- 生成指定月份结算单。

页面必须使用 Vue + Vite，不新增 HTML template。

## 小程序

小程序第一版不承担复杂 Excel 录入，只展示和轻量提交：

- 代加工：查看工单进度、生豆/成品/包材库存、提交轻量加工申请。
- 我的订单：查看代发订单、物流单号、发货状态。
- 结算中心：查看费用明细和结算单。

后续可在小程序增加手工补单和确认结算。

## API

新增内部 ERP API：

- `GET /api/customer-fulfillment/customers?q=`
- `GET /api/customer-fulfillment/:customer_id/overview`
- `POST /api/customer-fulfillment/:customer_id/imports/parse`
- `POST /api/customer-fulfillment/imports/:batch_id/apply`
- `GET /api/customer-fulfillment/:customer_id/imports`
- `GET /api/customer-fulfillment/:customer_id/custody`
- `GET /api/customer-fulfillment/:customer_id/work-orders`
- `GET /api/customer-fulfillment/:customer_id/direct-ship-orders`
- `GET /api/customer-fulfillment/:customer_id/fees`
- `POST /api/customer-fulfillment/:customer_id/settlements`

Excel 上传第一版使用 multipart form-data。解析阶段只写导入批次和行，不改业务库存。应用阶段才生成业务数据。

客户小程序 API 继续复用 `/api/mini/services/:key`，必要时扩展返回字段。

## 导入幂等规则

- 同一客户、同一文件 SHA、同一导入类型重复解析时，返回已存在批次。
- 应用导入以 `external_key` 做幂等。
- SKU 以 `customer_id + external_code` 或 `customer_id + product_name + spec` 匹配。
- 工单以 `customer_id + external_work_order_no` 匹配。
- 代发订单以 `customer_id + external_order_no + external_seq` 匹配；订单编号为空时使用导入批次、日期和行号生成外部键。
- 费用以 `customer_id + settlement_period + fee_type + source_type + source_id` 匹配。

重复导入不能重复扣库存、重复创建订单或重复计费。

## 错误处理

解析错误不阻断整个文件：

- 必填字段缺失。
- 数量无法解析。
- 日期无法解析。
- SKU 无法匹配且不能自动创建。
- 收件人信息无法拆分。

UI 展示错误行，允许用户修 Excel 后重新导入。第一版不在系统内逐行编辑错误数据。

## 安全与权限

- ERP 导入接口使用现有后台鉴权。
- 小程序只看当前绑定客户数据。
- 下游收件人信息只在订单快照和导入表中保存，不进入客户档案。
- 导入文件内容不长期保存为公开下载资源；数据库保留结构化解析结果和源文件摘要。

## 测试策略

每个需求必须维护 5 张需求表：

- 产品需求表：客户履约账户与 Excel 导入闭环。
- 开发需求表：导入批次、客户资源库存、加工工单、代发订单、结算生成、ERP 页面、小程序查询。
- 单元测试表：解析器、数量/日期/SKU 匹配、计费规则、幂等规则、前端 helper。
- API 测试表：multipart 导入解析、应用导入、订单创建、库存流水、结算生成、客户隔离。
- 需求审核表：按样本 Excel 验收。

TDD 顺序：

1. 先写 Excel 解析单测并确认 RED。
2. 再写 API handler 测试并确认 RED。
3. 实现解析器、仓储和 API。
4. 前端 helper 和页面源码守卫先 RED 再实现。
5. 最后跑单元/API/build，并补 REV 证据。

## 一期验收

- 能在 ERP 选择誉观山客户并导入三类 Excel。
- 代加工工单导入后能看到生豆库存、包材库存、工单、包装明细和客户 SKU。
- 代发清单导入后能创建客户订单，订单保留收件人快照和商品明细。
- 结算单导入或生成后，费用明细包含烘焙费、磨粉费、代发费、仓储费、物流费和调整项。
- 重复导入同一个文件不会重复扣库存、重复创建订单或重复计费。
- 小程序该客户只能看到自己的库存、订单、物流和结算。
- 公共产品列表不出现该客户专属 SKU；其他客户也不能使用该客户 SKU。
- 所有实现都有 UT/API/REV 证据。

## 一期不做

- 不接腾讯文档自动同步。
- 不做复杂公式编辑器。
- 不让客户在小程序里批量录入代发订单。
- 不把下游收件人写入客户档案。
- 不把客户托管资源混成棵凡自有库存权属；只在需要履约扣减时同步到现有库存能力。

## 后续扩展

- 腾讯文档自动同步。
- 客户在小程序确认结算单。
- 客户自助上传代发订单。
- 更细的客户库存成本和仓储费自动按日计提。
- 客户包材采购与采购应付联动。
