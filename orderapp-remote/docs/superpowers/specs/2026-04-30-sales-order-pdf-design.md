# 销售单 PDF 生成与设置设计

日期：2026-04-30
分支：`codex/sales-order-pdf-20260430`
状态：待用户最终审核后进入实现计划

## 背景

订单需要支持生成正式销售单。销售单用于发给客户、财务留档或打印盖章，因此必须支持可配置公司信息、个性化说明、收款方式、多个收款码和公章。

用户已确认：

- 销售单版式采用 `A. 正式销售单`。
- 交付方式采用系统直接生成 PDF 文件。
- PDF 生成时固化订单数据和销售单设置快照。
- 订单后续修改时，允许重新生成新版本，旧版本保留。

## 产品范围

### 订单侧

- 订单列表每行增加 `销售单` 操作。
- 点击后进入该订单的销售单页面。
- 页面展示该订单已有销售单版本列表：
  - 版本号：`V1`、`V2`、`V3`...
  - 生成时间
  - 生成人
  - PDF 下载入口
  - 是否最新版
- 如果没有销售单，显示 `生成销售单 PDF`。
- 如果已有销售单，显示 `重新生成销售单 PDF`，生成新版本，不覆盖旧版本。
- 默认下载最新版 PDF。

### 设置侧

设置菜单新增 `销售单设置`，维护以下字段：

- 销售单公司名称
- 销售单个性化说明
- 销售单收款方式说明
- 多个收款码：
  - 图片
  - 说明
  - 排序
  - 启用/停用
- 公章图片

设置保存后记录操作日志。

### PDF 内容

PDF 使用正式销售单版式，包含：

- 公司名称
- 标题：`销售单 SALES ORDER`
- 单号、订单日期、客户名称
- 收货/客户信息
- 订单信息：订单类型、收款状态、发货状态、发货方式等现有字段
- 商品明细：
  - 商品
  - 规格
  - 数量
  - 单价
  - 金额
- 商品合计、运费、优惠、取整、应收金额
- 个性化说明
- 收款方式说明
- 启用的收款码图片和说明
- 公章图片

## 非目标范围

本次不做：

- 电子签章法律认证。
- PDF 在线编辑。
- 客户在线确认/签收。
- 自动发送微信、邮件或短信。
- 多语言销售单。
- 批量生成多个订单销售单。

这些功能后续可以基于销售单版本和 PDF 存储结构扩展。

## 技术设计

### 分层结构

按现有 DDD 分层继续拆分：

- `internal/application/sales`
  - 定义销售单设置、收款码、销售单版本、生成命令和查询结果。
  - 编排生成流程。
- `internal/domain/sales`
  - 放置销售单快照、金额格式化、版本号规则等纯业务逻辑。
- `internal/infrastructure/postgres/sales`
  - 持久化销售单设置、收款码、销售单版本元数据和快照 JSON。
- `internal/infrastructure/pdf`
  - 封装 PDF 渲染能力，避免 HTTP 层直接依赖 PDF 细节。
- `internal/interfaces/http/sales`
  - 暴露销售单设置 API、订单销售单版本 API、生成/下载 API。
- `frontend-vue-shell`
  - 新增 `SalesOrderSettingsView.vue`
  - 新增或扩展订单列表操作入口
  - 新增 `SalesOrderView.vue`

### 数据表

新增表：

`sales_order_settings`

- `id`
- `company_name`
- `note`
- `payment_text`
- `seal_asset_id`
- `updated_at`
- `updated_by`

默认只保留一行配置。可用 `id=1` 或唯一约束控制。

`sales_order_payment_codes`

- `id`
- `label`
- `description`
- `asset_id`
- `sort`
- `active`
- `created_at`
- `updated_at`

`sales_order_assets`

- `id`
- `kind`：`payment_code` / `seal`
- `filename`
- `content_type`
- `bytes`
- `sha256`
- `object_key`
- `created_at`
- `created_by`

图片文件存储在本地 `assetDir` 下，数据库记录元数据。实现方式参考客户资产上传模块，避免把图片二进制直接放数据库。

`sales_order_documents`

- `id`
- `order_id`
- `order_no`
- `version_no`
- `snapshot_json`
- `pdf_asset_id`
- `created_at`
- `created_by`
- `is_latest`

约束：

- `(order_id, version_no)` 唯一。
- 同一订单只有一个 `is_latest=true`。
- 重新生成时在事务中把旧版本 `is_latest=false`，插入新版本 `is_latest=true`。

### 快照规则

生成 PDF 时创建 `snapshot_json`，包含：

- 订单头字段
- 客户字段
- 商品明细
- 金额字段
- 当前销售单设置
- 当前启用收款码的图片引用和说明
- 当前公章图片引用

后续订单或设置修改不会影响已有 PDF。重新生成才会基于当时最新数据创建新快照和新 PDF。

### PDF 渲染

推荐使用 Go 原生 PDF 库生成 PDF，封装在 `internal/infrastructure/pdf`：

- 需要支持中文字体。
- 字体文件随应用打包，避免服务器缺字体导致乱码。
- 图片支持 PNG/JPEG。
- 页面尺寸默认 A4。
- 多商品行超过一页时自动分页，页尾保留合计和收款信息。

实现时优先选一个依赖简单、可在 Alpine Docker 镜像稳定运行的库。如果 PDF 库对中文支持不稳定，则在实现计划里先做最小可行 Spike，并用中文快照测试锁定。

### HTTP API

设置 API：

- `GET /api/settings/sales-order`
  - 返回公司名称、说明、收款方式、收款码列表、公章信息。
- `POST /api/settings/sales-order`
  - 保存文本设置。
- `POST /api/settings/sales-order/payment-codes`
  - 新增收款码图片和说明。
- `PUT /api/settings/sales-order/payment-codes/:id`
  - 更新说明、排序、启用状态。
- `DELETE /api/settings/sales-order/payment-codes/:id`
  - 停用或删除收款码。
- `POST /api/settings/sales-order/seal`
  - 上传/替换公章图片。

订单销售单 API：

- `GET /api/orders/:id/sales-orders`
  - 返回该订单销售单版本列表。
- `POST /api/orders/:id/sales-orders`
  - 基于当前订单和设置生成新版本 PDF。
- `GET /orders/:id/sales-orders/:doc_id.pdf`
  - 下载指定版本 PDF。
- `GET /orders/:id/sales-order-latest.pdf`
  - 下载最新版 PDF。

页面路由：

- `/orders/:id/sales-order` 重定向到 `/vue-shell?view=salesOrder&order_id=:id`。
- `/settings/sales-order` 重定向到 `/vue-shell?view=salesOrderSettings`。

### 前端设计

`SalesOrderSettingsView.vue`

- 文本设置表单：
  - 公司名称
  - 个性化说明
  - 收款方式说明
- 收款码列表：
  - 上传图片
  - 编辑说明
  - 启用/停用
  - 排序
- 公章上传/替换。

`SalesOrderView.vue`

- 订单摘要。
- 销售单版本列表。
- `生成销售单 PDF` / `重新生成销售单 PDF` 按钮。
- 最新版下载按钮。
- 历史版本下载按钮。

`OrdersView.vue`

- 操作列增加 `销售单` 链接。

### 操作日志

以下操作写入统一操作日志：

- 保存销售单设置。
- 上传/替换公章。
- 新增/修改/停用收款码。
- 生成销售单 PDF。

日志元数据至少包含：

- `order_id`
- `order_no`
- `document_id`
- `version_no`
- 设置字段名或资产类型。

## 测试策略

### 单元测试

- 销售单版本号递增规则：
  - 无历史版本时生成 `V1`
  - 已有 `V1` 时生成 `V2`
- 快照构造：
  - 包含订单头、商品明细、金额、设置、收款码、公章引用。
  - 设置变更后旧快照不变。
- 金额格式化：
  - 商品合计、运费、优惠、应收金额按订单字段输出。
- PDF 渲染输入校验：
  - 中文公司名、商品名、说明不丢失。
  - 无公章/无收款码时也能生成 PDF。

### API 测试

- `GET /api/settings/sales-order` 初始返回默认结构。
- `POST /api/settings/sales-order` 保存文本设置并可查询。
- 上传多个收款码后列表按排序返回。
- 上传公章后设置返回公章资产。
- `POST /api/orders/:id/sales-orders` 生成 `V1`。
- 再次生成同一订单得到 `V2`，且 `V2` 为最新版，`V1` 仍可下载。
- 下载 PDF 返回：
  - `200`
  - `Content-Type: application/pdf`
  - 文件名包含订单号和版本号。

### 验收

- 在订单列表点击 `销售单` 可以进入销售单版本页面。
- 无销售单时能生成 PDF。
- PDF 样式为正式销售单版式。
- PDF 中包含公司名称、说明、收款方式、多个收款码、公章。
- 修改销售单设置后，已生成 PDF 不变化。
- 重新生成后出现新版本，旧版本仍可下载。

## 开发顺序

1. 新增需求表记录：PR/DEV/UT/API/REV。
2. 新增领域模型和单元测试。
3. 新增数据库 schema 和 repository。
4. 新增销售单设置 API 和 API 测试。
5. 新增销售单生成/下载 API 和 API 测试。
6. 新增 PDF 渲染封装和最小 PDF 测试。
7. 新增 Vue 设置页和销售单版本页。
8. 订单列表增加销售单入口。
9. 完整验证：`go test ./...`、前端单测、`npm run build`。
10. 合入 `develop` 后部署测试服务器。

## 风险和约束

- PDF 中文字体是最大技术风险，必须在 Docker 构建环境中验证。
- 图片尺寸和格式需要限制，避免超大二维码或公章导致 PDF 过大。
- 生成 PDF 属于可审计业务动作，失败时不能写入半成品版本。
- 由于销售单设置含图片上传，本次必须复用现有资产存储模式，避免引入新的文件存储方案。
