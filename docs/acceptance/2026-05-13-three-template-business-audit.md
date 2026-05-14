# 三种能力模板业务链路审计（2026-05-13）

## 目标拆解
- 三种能力模板：`processing_fulfillment`（客户代加工履约）、`public_sku_direct_ship`（公共 SKU 小批量代发）、`retail_mall`（零售商城客户）。
- 每种模板至少要覆盖：客户档案/类型、客户账号绑定或小程序身份、模板能力、订单生成、订单可查询、生产或履约影响、财务/结算归集、客户小程序、ERP 工作台。
- 交付进度必须录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程落到自动化测试、验收文档和操作手册。

## 当前证据
| 模板 | 已有能力证据 | 本轮补强 |
| --- | --- | --- |
| 客户代加工履约 | `DefaultCapabilityTemplates` 启用代发、代加工、托管库存、结算；客户履约账户可处理代加工工单、代发订单、费用和结算；生产计划读取代加工需求。 | 已补 `TestMiniAPITemplateBusinessContract` 和 `TestThreeTemplateBusinessWalkthroughAcrossModules`，覆盖小程序、订单、履约、生产、财务链路。 |
| 公共 SKU 小批量代发 | 模板启用现货下单、代发和结算；小批量规则 `<14lb` 使用 `15-28lb` 档；客户履约账户可提交代发订单并进入订单列表。 | 已补小程序 API 矩阵和跨模块走查，覆盖公共 SKU 现货单、代发单、生产计划、财务收入/费用归集。 |
| 零售商城客户 | 模板启用商城首页和商城下单；零售/电商客户默认创建 `retail_mall` 门户；商城订单写入现有订单表，后端订单服务允许 `mall` 能力读取订单历史。 | 修复小程序商城首页缺“我的订单”入口；新增 ERP 工作台绑定禁用规则和前端禁用提示，避免零售客户误进批发履约工作台。 |

## 本轮修复证据
- 单元测试 RED：`npm test -- src/utils/capabilities.test.ts src/utils/mall.test.ts` 先失败，缺 `mall` 订单入口和商城页 `openOrders`。
- 实现：
  - `miniapp/src/utils/capabilities.ts`：`orders` 入口能力增加 `mall`。
  - `miniapp/src/pages/mall/mall.vue`：商城页顶部增加“我的订单”跳转到 `/pages/service/service?key=orders`。
  - `orderapp-remote/internal/application/customerportal/service_test.go`：订单服务能力矩阵包含 `CapabilityMall`。
  - `orderapp-remote/internal/interfaces/http/support/req_store.go`：新增 `PR/DEV/UT/API/REV-175-RETAIL-MALL-ORDER-HISTORY`。
  - `OP_MANUAL_CUSTOMER_PORTAL.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`：补充商城首页查看订单记录说明。

## 三模板业务契约矩阵
- 新增 `TestDefaultCapabilityTemplatesRuntimeBusinessContract`，用三种默认模板生成小程序当前客户上下文，并在应用/API 服务层逐项验证允许和拒绝行为。
- 新增 `TestMiniAPITemplateBusinessContract`，通过 `/api/mini/services/:key`、`/api/mini/mall`、`/api/mini/mall/orders`、`/api/mini/direct-ship/batches`、`/api/mini/processing-requests`、`/api/mini/fulfillment-orders` 验证同一矩阵在小程序 HTTP API 层返回正确的 `200/403`。
- 覆盖入口模式、主题、ERP 工作台权限、订单历史、现货下单、一件代发、代加工、托管库存、结算中心、商城页、商城下单、代发批次、代加工申请、履约订单创建。
- 需求种子新增 `PR/DEV/UT/API/REV-176-THREE-TEMPLATE-BUSINESS-CONTRACT`，把该矩阵作为三模板能力边界的长期验收证据。

| 操作/入口 | `processing_fulfillment` | `public_sku_direct_ship` | `retail_mall` |
| --- | --- | --- | --- |
| 小程序入口 | services | services | mall |
| ERP 工作台权限 | `customer_processing.read/submit` + `customerProcessingPortal` | `customer_processing.read/submit` + `customerProcessingPortal` | 无客户履约 ERP 工作台 |
| 我的订单 `orders` | 允许 | 允许 | 允许 |
| 现货下单 `productOrder` | 拒绝 | 允许 | 拒绝 |
| 一件代发 `directShip` / 代发批次 / 代发履约单 | 允许 | 允许 | 拒绝 |
| 代加工 `processing` / 加工申请 / 加工发货单 | 允许 | 拒绝 | 拒绝 |
| 托管库存 `inventory` | 允许 | 拒绝 | 拒绝 |
| 结算中心 `settlement` | 允许 | 允许 | 拒绝 |
| 商城页 / 商城下单 | 拒绝 | 拒绝 | 允许 |

## 跨模块业务走查矩阵
- 新增 `TestThreeTemplateBusinessWalkthroughAcrossModules`，在不依赖本地数据库的条件下，用同一个内存业务场景串联真实应用服务：`customerportalapp.NewService`、`customerfulfillmentapp.NewService`、`productionapp.NewService`、`financeapp.NewService`。
- 覆盖账号模板应用、ERP 账号绑定、小程序下单/代发/代加工、客户履约工作台、客户结算、生产计划启动、财务费用维度、经营报表和来源钻取。
- 需求种子新增 `PR-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH`、`DEV-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH`、`UT-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH`、`API-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH`、`REV-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH`，把订单、生产、财务、客户履约工作台的跨模块闭环作为独立验收证据。

| 模板 | 订单 | 生产 | 财务 | 客户履约工作台 |
| --- | --- | --- | --- | --- |
| `processing_fulfillment` | 小程序代加工申请、代加工发货单、一件代发单均生成业务记录 | 代加工需求进入生产计划并可启动生产批次 | 订单收入、生产成本、客户费用和结算进入经营视角 | ERP 账号可查看托管库存、加工单、代发单、费用和结算 |
| `public_sku_direct_ship` | 小程序现货单、一件代发单和工作台代发单均生成订单 | 公共 SKU 订单进入生产计划并可启动生产批次 | 订单收入、生产成本、代发耗材费用进入经营报表和来源明细 | ERP 账号可查看代发单、费用和结算，不暴露代加工/库存能力 |
| `retail_mall` | 商城页可下商城订单并进入订单历史 | 商城订单作为普通成品需求进入生产计划 | 商城订单收入和生产成本进入经营报表 | 无客户履约 ERP 工作台绑定，避免零售客户看到批发履约台 |

## 零售商城客户不开放 ERP 工作台绑定
- 新增 `PR-178-RETAIL-MALL-ERP-WORKBENCH-GUARD`、`DEV-178-RETAIL-MALL-ERP-WORKBENCH-GUARD`、`UT-178-RETAIL-MALL-ERP-WORKBENCH-GUARD`、`API-178-RETAIL-MALL-ERP-WORKBENCH-GUARD`、`REV-178-RETAIL-MALL-ERP-WORKBENCH-GUARD`。
- 业务规则：只有暴露 ERP 权限或 ERP 视图的能力模板才能激活客户 ERP 工作台绑定。`processing_fulfillment` 和 `public_sku_direct_ship` 暴露 `customerProcessingPortal`，可以绑定渠道客户账号；`retail_mall` 不暴露 ERP 工作台，只保留小程序商城、商城下单和我的订单闭环。
- 单元证据：`TestUpsertPortalERPBindingRejectsRetailMallTemplate` 验证 `retail_mall` 激活 ERP 工作台绑定被服务层拒绝，且不会委托仓储层写入绑定。
- API 证据：`TestPortalAdminERPBindingRejectsTemplatesWithoutWorkbench` 验证 `/api/customer-portal/admin/customers/147/erp-binding` 对无工作台模板返回 `400` 和可理解错误，不把产品规则拒绝误报成 `500`。
- 前端证据：`CustomerPortalSettingsView.vue` 使用 `templateSupportsERPWorkbench` 在不支持 ERP 工作台的模板下禁用 ERP 账号选择和绑定按钮，并显示“该模板不开放 ERP 工作台”；`customer-portal-theme.test.js` 已覆盖该源码守卫。
- PostgreSQL 集成测试源码已同步为 `TestUpsertCustomerERPBindingDoesNotGrantHiddenTemplateRoles`，避免有数据库时继续期望旧的隐藏客户角色授权。
- 手册证据：`OP_MANUAL_CUSTOMER_PORTAL.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md` 已补充“零售商城客户不绑定 ERP 工作台账号”和常见错误处理。

## 零售商城模板 ERP 工作台字段不变式
- 新增 `PR-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT`、`DEV-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT`、`UT-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT`、`API-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT`、`REV-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT`，把零售商城模板不能通过自定义模板保存 ERP 工作台字段作为独立安全验收项。
- 问题：默认 `retail_mall` 模板不暴露 ERP 工作台，但 `SaveCapabilityTemplate` 原来会接受前端或手工 API 提交的 `erp_permissions`、`erp_view_keys`；保存后的零售商城模板会让 `ExposesERPWorkbench()` 变成 true，从而绕过 ERP 工作台绑定禁用规则。
- 修复：保存能力模板前，如果模板默认不暴露 ERP 工作台且请求带 ERP 权限或 ERP 视图，返回 `ERP workbench unavailable for capability template`；模板归一化也会对不暴露 ERP 工作台的模板清空 ERP 字段，避免历史异常保存行继续生效。
- RED/GREEN 证据：`TestSaveCapabilityTemplateRejectsRetailMallERPWorkbenchFields` 修复前可保存零售商城 ERP 工作台字段，修复后拒绝且不委托仓储；`TestUpsertPortalERPBindingRejectsSavedRetailMallTemplateWithERPWorkbench` 修复前坏保存行会允许绑定 ERP 工作台，修复后仍拒绝绑定。
- API 证据：`TestPortalAdminCapabilityTemplateERPWorkbenchUnavailableMapsToBadRequest` 固定能力模板保存接口对零售商城 ERP 工作台字段返回 `400` 和可理解错误。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充零售商城模板不能保存 ERP 工作台字段，误填 ERP 权限或 ERP 视图会被拒绝。

## 客户专属豆单 PDF 下载边界
- 新增 `PR-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION`、`DEV-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION`、`UT-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION`、`API-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION`、`REV-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION`，把客户专属豆单 PDF 下载范围作为独立数据安全验收项。
- 业务规则：客户专属豆单 PDF 只能由归属客户访问，不能下载其他客户专属豆单；官方已发布豆单可作为公共兜底访问，客户没有专属豆单时仍可查看官方已发布豆单。
- 仓储边界：`LoadBeanListPublication` 读取 `bean_list_publications` 时要求 `status='published'`，并只允许 `(owner_type='customer' AND owner_key=当前客户ID)` 或 `owner_type='official'`。
- 真实 PostgreSQL 证据：`TestLoadBeanListPublicationRejectsAnotherCustomerPublication` 创建客户 A/B 和客户 B 专属发布，客户 A 加载客户 B 发布返回 `ErrBeanListPublicationNotFound`；`TestLoadBeanListPublicationAllowsOfficialPublication` 验证客户 A 可加载官方已发布豆单。
- API 证据：`TestMiniBeanListPDFPublicationNotFoundMapsToNotFound` 固定小程序豆单 PDF API 对非归属客户专属发布返回 `404 {"error":"bean list publication not found"}`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户专属豆单 PDF 只能由归属客户访问、官方已发布豆单可作为公共兜底访问、不能下载其他客户专属豆单。

## 商城商品图片上传类型边界
- 新增 `PR-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD`、`DEV-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD`、`UT-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD`、`API-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD`、`REV-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD`，把商城商品图片上传只接受图片文件作为独立公开资产安全验收项。
- 问题：商城商品图片上传原来只拒绝空文件，`promo.html` 这类 HTML/脚本内容会被写入 `/assets/mall_products/...` 公开路径，并更新为商品图片 URL。
- 修复：`saveUploadedMallProductAsset` 写文件前用 `http.DetectContentType` 嗅探文件内容，只允许 PNG、JPEG、GIF 和 WebP；非图片内容返回 `image file required`，不写入公开资产目录，也不调用商品图片 URL 更新。
- RED/GREEN 证据：`TestMallAdminImageUploadRejectsNonImageAsset` 修复前返回 200 并把 `promo.html` 保存为公开商品图片，修复后返回 400 且 `UpdateMallProductImage` 未被调用；`TestMallAdminImageUploadStoresAndServesPublicAsset` 使用真实 PNG 字节固定合法图片仍可上传并以 `image/png` 服务。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充商城商品图片上传只接受图片文件、不能上传 HTML 或脚本文件、图片文件格式不支持时接口返回无效请求。

## 商城商品图片上传大小边界
- 新增 `PR-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD`、`DEV-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD`、`UT-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD`、`API-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD`、`REV-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD`，把商城商品图片上传超过 8MB 时必须拒绝作为独立公开资产质量验收项。
- 问题：商城商品图片上传原来用 `io.LimitReader(src, 8<<20)`，超过 8MB 的图片会被截断后继续保存和更新为公开商品图片 URL，可能产生损坏图片，也会误导运营以为原图完整上传。
- 修复：上传读取改为 `maxMallProductImageUploadBytes+1`，读到超过 8MB 时立即返回 `image file too large`，不能截断后保存为公开商品图片，也不会调用商品图片 URL 更新。
- RED/GREEN 证据：`TestMallAdminImageUploadRejectsOversizedAsset` 修复前返回 200 并保存 `huge.png`，修复后返回 400 且 `UpdateMallProductImage` 未被调用。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充商城商品图片上传超过 8MB 时必须拒绝、不能截断后保存为公开商品图片、图片文件过大时接口返回无效请求。

## 商城商品图片缺失商品边界
- 新增 `PR-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD`、`DEV-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD`、`UT-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD`、`API-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD`、`REV-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD`，把商城商品图片上传必须先确认商品存在作为公开资产安全验收项。
- 问题：商城商品图片上传原来先读取并写入 `/assets/mall_products/...`，再调用 `UpdateMallProductImage` 更新商品图片 URL；缺失商品或更新失败时会返回错误，但有效图片已留下公开孤儿资产。
- 修复：上传流程拆成 `readUploadedMallProductImage` 和 `saveMallProductImageData`；先完成文件大小/类型校验，再通过 `ensureMallProductImageUploadTarget` 确认商城商品存在，之后才写入公开资产。若后续 `UpdateMallProductImage` 仍失败，`cleanupMallProductImageAsset` 会删除刚写入的文件和空目录。
- RED/GREEN 证据：`TestMallAdminImageUploadRejectsMissingMallProductWithoutWritingAsset` 修复前看到 `mall_products/` 孤儿目录，修复后资产目录保持空；`TestMallAdminImageUploadCleansAssetWhenImageUpdateFails` 修复前在图片更新失败后留下公开资产，修复后清理干净。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充商城商品图片上传必须先确认商品存在、缺失商品或图片更新失败时不能留下公开孤儿资产、商品不存在时接口返回无效请求。

## 销售单收款码上传边界
- 新增 `PR-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD`、`DEV-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD`、`UT-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD`、`API-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD`、`REV-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD`，把销售单收款码上传只接受图片文件且超过 8MB 时拒绝作为公开销售资料安全验收项。
- 问题：销售单设置收款码上传原来信任 multipart `Content-Type` 且用 `io.LimitReader(src, 8<<20)`；`pay.html` 会被保存成公开收款码资源，超过 8MB 的 `huge.jpg` 会被截断后保存。
- 修复：`saveUploadedSalesOrderAsset` 对收款码上传按文件内容嗅探，只允许 PNG/JPEG/GIF/WebP；读取改为 `maxSalesOrderSettingsAssetUploadBytes+1`，超过 8MB 返回 `image file too large`，非图片返回 `image file required`，均不写入 `sales_order_assets`。
- RED/GREEN 证据：`TestSalesOrderPaymentCodeUploadRejectsNonImageAsset` 修复前返回 200 并保存 `pay.html`，修复后返回 400 且不入库；`TestSalesOrderPaymentCodeUploadRejectsOversizedAsset` 修复前返回 200 并保存截断后的 `huge.jpg`，修复后返回 400 且不入库；`TestSalesOrderPaymentCodeUploadStoresImageAsset` 固定有效 JPEG 收款码仍可上传。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充销售单收款码上传只接受图片文件、不能上传 HTML 或脚本文件作为收款码、收款码图片超过 8MB 时必须拒绝。

## 销售单收款码标签必填边界
- 新增 `PR-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD`、`DEV-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD`、`UT-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD`、`API-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD`、`REV-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD`，把销售单收款码上传必须先填写标签作为公开收款资料安全验收项。
- 问题：销售单设置收款码上传原来先保存图片文件和 `sales_order_assets` 元数据，再由应用服务校验收款码标签；标签为空时 API 返回 `label required`，但已留下公开收款码资产。
- 修复：`uploadPaymentCode` 在读取 multipart 文件和调用 `saveUploadedSalesOrderAsset` 前先校验 `label`，标签为空立即返回 `label required`，不会写入文件或资产元数据。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestSalesOrderPaymentCodeUploadRequiresLabelBeforeWritingAsset` 修复前看到 `sales_order_assets kind "payment_code" rows=1`，修复后返回 `400 label required` 且资产表和资产目录均为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充销售单收款码上传必须先填写标签、标签为空时不能写入收款码资产、补齐标签后再上传。

## 销售单公章资产清理边界
- 新增 `PR-202-SALES-ORDER-SEAL-ASSET-CLEANUP`、`DEV-202-SALES-ORDER-SEAL-ASSET-CLEANUP`、`UT-202-SALES-ORDER-SEAL-ASSET-CLEANUP`、`API-202-SALES-ORDER-SEAL-ASSET-CLEANUP`、`REV-202-SALES-ORDER-SEAL-ASSET-CLEANUP`，把销售单公章上传或去除背景失败时不能留下公开孤儿公章资产作为销售资料安全验收项。
- 问题：销售单公章上传和去除背景流程先写入 `/assets/sales_order_assets/seal/...`，再写入资产元数据或更新 `sales_order_settings.seal_asset_id`；如果资产元数据保存失败或设置更新失败，公开公章文件和/或 `sales_order_assets` 行会残留。
- 修复：`saveUploadedSalesOrderAsset` 在 `SaveSalesOrderAsset` 失败时调用 `cleanupSalesOrderAssetFile` 清理文件；`uploadSeal`、`removeSealBackground` 和收款码保存失败路径通过 `cleanupSavedSalesOrderAsset` 删除未引用资产行和文件；新增 `DeleteSalesOrderAsset` 仓储方法用于回滚刚写入的资产元数据。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestSalesOrderSealUploadCleansFileWhenAssetMetadataFails` 修复前看到 `sales_order_assets/` 孤儿目录，修复后资产目录为空；`TestSalesOrderSealUploadCleansAssetWhenSettingsUpdateFails` 修复前看到 `sales_order_assets kind "seal" rows=1`，修复后资产表和资产目录均为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充销售单公章上传或去除背景失败时必须清理刚写入的公章资产、不会留下公开孤儿公章文件、重新上传公章。

## 物流单号 Excel 上传大小边界
- 新增 `PR-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD`、`DEV-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD`、`UT-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD`、`API-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD`、`REV-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD`，把物流单号 Excel 上传超过 20MB 时必须拒绝作为订单发货回传安全验收项。
- 问题：新版 `/api/orders/shipping-tracking-excel` 和旧版 `/ship/tracking_fill` 物流回传入口原来直接把上传流交给 `excelize.OpenReader`，超过 20MB 的非 Excel 或异常大文件会先进入 Excel 解析，反馈不清晰且增加内存/解析压力。
- 修复：两个入口共用 `readShipmentTrackingExcelUpload`，先读取 `maxShippingTrackingExcelUploadBytes+1`，物流单号 Excel 上传超过 20MB 时必须拒绝并返回 `file too large`；不能解析超大物流回传 Excel。
- RED/GREEN 证据：`TestOrdersShippingTrackingExcelAPIRejectsOversizedUpload` 修复前返回 `Excel 文件格式无法解析`，修复后返回 `file too large`；`TestLegacyShippingTrackingFillRejectsOversizedUpload` 修复前返回 `excel解析失败`，修复后返回 `file too large`。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充物流单号 Excel 上传超过 20MB 时必须拒绝、不能解析超大物流回传 Excel、接口返回 `file too large`。

## 客户档案资产图片上传大小边界
- 新增 `PR-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD`、`DEV-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD`、`UT-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD`、`API-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD`、`REV-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD`，把客户档案资产图片上传超过 8MB 时必须拒绝作为客户资料资产边界验收项。
- 问题：客户档案附件上传 UI 只接受图片，但后端 `/customers/:id/assets/upload` 给客户资产保存服务传入 `100 * 1024 * 1024`，客户 Logo、标签图等资料图片允许到 100MB，和商城图片、销售单收款码等图片 8MB 边界不一致。
- 修复：客户档案资产上传 API 使用 `maxCustomerAssetUploadBytes = 8 << 20`，把 8MB 上限传给客户资产保存服务；原有内容嗅探仍用于识别 `image/png` 等真实图片类型。
- RED/GREEN 证据：`TestCustomerAssetUploadUsesImageSizeLimit` 修复前记录到 `MaxBytes=104857600` 并失败，修复后 `MaxBytes=8<<20` 且 `ContentType=image/png`。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户档案资产图片上传超过 8MB 时必须拒绝，PNG/JPEG/WebP 图片压缩到 8MB 内后再上传。

## 客户档案资产元数据失败清理边界
- 新增 `PR-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP`、`DEV-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP`、`UT-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP`、`API-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP`、`REV-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP`，把客户档案资产元数据保存失败时必须清理刚写入的文件作为客户资料资产安全验收项。
- 问题：`customer.Repository.SaveAsset` 原来先执行 `saveAssetFile` 写入 `customer_assets` 文件，再插入客户资产元数据；若客户不存在或 DB 元数据插入失败，会返回错误但文件系统留下公开孤儿客户资产。
- 修复：`SaveAsset` 在 `insertCustomerAsset` 失败时调用 `cleanupCustomerAssetFile`，删除刚写入的文件并清理空父目录；成功路径不受影响。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestSaveAssetCleansFileWhenMetadataInsertFails` 修复前因为缺失客户 FK 失败后看到 `customers/` 孤儿目录，修复后资产目录保持空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户档案资产元数据保存失败时必须清理刚写入的文件、不会留下公开孤儿客户资产、上传后重新打开客户档案确认附件列表。

## 发票文件上传缺失订单边界
- 新增 `PR-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD`、`DEV-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD`、`UT-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD`、`API-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD`、`REV-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD`，把发票文件上传必须先确认订单存在作为订单附件资产安全验收项。
- 问题：`/api/orders/:id/invoice-file` 原来先读取并写入上传文件，再调用销售服务保存发票记录；缺失订单会返回失败，但有效 PDF 上传已经在资产目录下留下孤儿发票文件。缺失订单且没有有效文件时还会先返回 `file required`，没有明确提示订单不存在。
- 修复：发票文件上传 API 在读取和写入文件前先调用 `LoadOrderInvoice` 验证订单存在；订单不存在时返回 `order not found`，不会写入发票资产文件。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestOrderInvoiceAPIRejectsMissingOrderWithoutWritingAsset` 修复前看到 `sales_order_assets/` 孤儿目录，修复后资产目录保持空；`TestOrderInvoiceAPIRejectsMissingOrderBeforeReadingFile` 修复前返回 `file required`，修复后返回 `order not found`。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充发票文件上传必须先确认订单存在、订单不存在时返回 order not found、不会写入发票资产文件。

## 发票文件保存失败清理边界
- 新增 `PR-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP`、`DEV-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP`、`UT-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP`、`API-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP`、`REV-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP`，把发票文件保存失败时必须清理刚写入的发票资产文件作为订单附件资产安全验收项。
- 问题：订单存在时 `/api/orders/:id/invoice-file` 会先写入 `/assets/sales_order_assets/order_invoices/...`，再调用 `SaveOrderInvoiceFile` 落库；若发票记录保存或事务提交失败，数据库会回滚但公开发票文件会残留。
- 修复：发票上传 API 在 `SaveOrderInvoiceFile` 返回错误时调用 `cleanupUploadedOrderInvoiceFile`，删除刚写入的文件和空目录，再返回原始保存错误。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestOrderInvoiceAPIUploadCleansFileWhenInvoiceSaveFails` 修复前看到 `sales_order_assets/` 孤儿目录，修复后资产目录为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充发票文件保存失败时必须清理刚写入的发票资产文件、不会留下公开孤儿发票文件、重新上传发票。

## 销售单生成文件失败清理边界
- 新增 `PR-204-SALES-ORDER-GENERATED-FILE-CLEANUP`、`DEV-204-SALES-ORDER-GENERATED-FILE-CLEANUP`、`UT-204-SALES-ORDER-GENERATED-FILE-CLEANUP`、`API-204-SALES-ORDER-GENERATED-FILE-CLEANUP`、`REV-204-SALES-ORDER-GENERATED-FILE-CLEANUP`，把销售单 PDF/图片生成失败时必须清理刚写入的文件作为销售单公开资产安全验收项。
- 问题：`GenerateSalesOrderDocument` 和 `GenerateSalesOrderImage` 先写入 `/assets/sales_order_documents/...` 或 `/assets/sales_order_images/...`，再插入资产行、销售单版本行、审计日志并提交事务；若记录插入或事务提交失败，数据库回滚但公开 PDF/PNG 文件会残留。
- 修复：销售单 PDF/图片生成在写入文件后记录 `fileWritten`，只有事务成功提交后才标记 `committed`；所有未提交返回路径都会调用 `cleanupGeneratedSalesOrderAssetFile` 删除刚写入的文件和空目录。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestGenerateSalesOrderDocumentCleansFileWhenDocumentInsertFails` 修复前看到 `sales_order_documents/` 孤儿目录、`TestGenerateSalesOrderImageCleansFileWhenImageInsertFails` 修复前看到 `sales_order_images/` 孤儿目录，修复后两个失败路径资产目录均为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充销售单 PDF/图片生成失败时必须清理刚写入的文件、不会留下公开孤儿销售单文件、重新生成销售单 PDF 或图片。

## 出库单生成文件失败清理边界
- 新增 `PR-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP`、`DEV-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP`、`UT-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP`、`API-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP`、`REV-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP`，把出库单 PDF 生成失败时必须清理刚写入的文件作为出库单公开资产安全验收项。
- 问题：`GenerateDeliveryNoteDocument` 先写入 `/assets/delivery_note_documents/...`，再插入出库单资产行、版本行、审计日志并提交事务；若记录插入或事务提交失败，数据库回滚但公开出库单 PDF 文件会残留。
- 修复：出库单 PDF 生成在写入文件后记录 `fileWritten`，只有事务成功提交后才标记 `committed`；所有未提交返回路径都会调用 `cleanupGeneratedDeliveryNoteAssetFile` 删除刚写入的文件和空目录。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestGenerateDeliveryNoteDocumentCleansFileWhenDocumentInsertFails` 修复前看到 `delivery_note_documents/` 孤儿目录，修复后失败路径资产目录为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充出库单 PDF 生成失败时必须清理刚写入的文件、不会留下公开孤儿出库单文件、重新生成出库单 PDF。

## 快递录单 Excel 生成文件失败清理边界
- 新增 `PR-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP`、`DEV-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP`、`UT-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP`、`API-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP`、`REV-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP`，把快递录单 Excel 生成失败时必须清理刚写入的文件作为发货公开导出资产安全验收项。
- 问题：`generateOrdersShippingExcel` 先 `SaveAs(path)` 写入 `/ship/order_exports/...xlsx`，再调用 `CreateOrderShipment` 保存发货批次；若发货批次保存失败，数据库无记录但公开 Excel 文件会残留。
- 修复：`CreateOrderShipment` 返回错误时调用 `cleanupOrderShippingExportFile` 删除刚写入的快递录单 Excel，再返回原始保存错误。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestOrdersShippingExcelAPICleansFileWhenShipmentSaveFails` 修复前看到导出目录残留 `ship_20260427_测试客户_SO-SHIP-SAVE-FAIL.xlsx`，修复后导出目录为空。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充快递录单 Excel 生成失败时必须清理刚写入的文件、不会留下公开孤儿快递录单文件、重新生成快递录单 Excel。

## 财务历史订单收入回退边界
- 新增 `PR-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK`、`DEV-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK`、`UT-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK`、`API-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK`、`REV-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK`，把历史订单收入不漏计作为财务报表数据安全验收项。
- 问题：财务月度收入和来源明细原来使用 `COALESCE(grand_total,total_amount,0)`；但历史/最小订单表中 `grand_total` 是非空默认 0，只有 `total_amount` 的旧订单会被计为 0，导致财务首页和经营报告收入漏计。
- 修复：新增 `financeOrderRevenueSQL`，月度收入和来源明细共用同一金额表达式：`grand_total` 非 0、存在折扣或运费时使用 `grand_total`；否则回退 `total_amount`，从而兼容历史订单，同时保留全额折扣等明确 0 `grand_total` 订单为 0；作废订单继续排除。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestMonthlySourceTotalsUsesLegacyTotalAmountWhenGrandTotalWasDefaultZero` 修复前收入为 `110.00`、期望 `230.00`，修复后通过，并验证 `SO-LEGACY-TOTAL` 来源明细为 120、`SO-FULL-DISCOUNT` 为 0、`SO-VOID` 不进入收入明细。
- 文档证据：`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充历史订单只有 `total_amount` 且 `grand_total` 为默认 0 时收入回退、全额折扣仍为 0、作废订单不计入收入。

## 多品项订单部分完工状态边界
- 新增 `PR-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD`、`DEV-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD`、`UT-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD`、`API-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD`、`REV-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD`，把多品项订单部分完工不能提前进入可发货范围作为生产状态安全验收项。
- 问题：生产完工原来只检查同一订单是否还有 `status='running'` 的生产中记录；同一订单有 A/B 两个已绑定 SKU 明细时，如果只启动并完工 A，系统会因为没有 running 工单引用该订单而把整单改成“生产完成”，让 B 未生产也进入可发货范围。
- 修复：`completeOrderIfAllRunningDone` 在无 running 工单后继续调用 `orderHasRemainingProductionGapTx`，按订单明细的商品/规格、生产日志、成品库存、库存预留和强制生产决策计算剩余生产缺口；只要还有缺口就保持“生产中”，全部已生产或已有可用成品库存覆盖后才推进到“生产完成”。
- RED/GREEN 证据：真实 PostgreSQL API 测试 `TestProduceFinishAPIKeepsOrderInProductionWhenOtherItemsRemainUnproduced` 修复前返回订单状态 `"生产完成"`、期望 `"生产中"`；修复后通过。全量 `go test ./internal/interfaces/http/production -count=1` 通过，保留多规格全部完工后整单完成的既有闭环。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18155/vue-shell?view=produceRunning`，对订单 `SO-PARTIAL-ORDER-UI` 的第一项 `半产状态A-UI` 点击“完成”；页面显示 `生产已完成` 且生产中列表变为 `暂无生产中项目`，真实 PostgreSQL 反查该 running item 为 `done`、`production_logs=1`，订单状态仍为 `生产中`，没有提前推进到 `生产完成`。证据标记：`PRODUCTION_PARTIAL_ORDER_STATUS_UI_CLICK_OK app=http://127.0.0.1:18155 pg=55625 evidence=finish_first_item_only db=order_status_production_running_logs_1_running_done`。
- 文档证据：`OP_MANUAL_PRODUCTION.md`、`orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充多品项订单只完成部分生产品项时仍保持生产中，不能提前进入可发货范围。

## 财务重复结账已调整状态边界
- 新增 `PR-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS`、`DEV-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS`、`UT-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS`、`API-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS`、`REV-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS`，把结账后调整状态不能被重复结账隐藏作为财务月结审计验收项。
- 问题：已结账月份新增结账后金额调整后，`finance_monthly_reports.status` 会变为 `adjusted`；但再次执行 `CloseMonth` 会先重算快照，再无条件把状态设为 `closed`，导致已有调整痕迹从经营报告状态上被隐藏。
- 修复：`CloseMonth` 重算月度快照时只在当前状态不是 `MonthStatusAdjusted` 时写回 `MonthStatusClosed`；已有结账后调整的月份重复结账仍保留 `adjusted`，让财务复盘和会计交接能持续看到“已调整”状态。
- RED/GREEN 证据：应用层单元测试 `TestCloseMonthPreservesAdjustedStatus` 修复前返回并保存 `closed`，期望 `adjusted`；修复后通过。真实 PostgreSQL 服务/仓储测试 `TestCloseMonthKeepsAdjustedStatusAfterAdjustment` 覆盖首次结账、创建 `补记费用` 调整、重复结账和 persisted status 仍为 `adjusted`。
- 文档证据：`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充已调整月份重复结账仍保持已调整，不会降回已结账。

## 真实 PostgreSQL 持久化审计与迁移硬化
- 新增 `PR-179-REAL-POSTGRES-PERSISTENCE-AUDIT`、`DEV-179-REAL-POSTGRES-PERSISTENCE-AUDIT`、`UT-179-REAL-POSTGRES-PERSISTENCE-AUDIT`、`API-179-REAL-POSTGRES-PERSISTENCE-AUDIT`、`REV-179-REAL-POSTGRES-PERSISTENCE-AUDIT`。
- 本机无预置 `DATABASE_URL` 且 Docker daemon 不可用，但系统有 `initdb/pg_ctl/postgres`，已用 `/tmp/kferp-pg.*` 一次性测试库执行真实 PostgreSQL 集成验证，命令形态为 `ORDERAPP_TEST_DATABASE_URL=ephemeral go test ./internal/infrastructure/postgres/... -count=1`。
- 首轮真实库验证发现 `customerfulfillment` PostgreSQL 测试 helper 未先初始化 company schema，导致 `authz.EnsureSchema` 找不到 `company_employees`；已补 `postgrescompany.EnsureSchema`，并把测试员工数据改为真实约束所需的 `account_type='channel_customer'`、手机号和部门。
- 更宽的 postgres 包集合发现 `sales` schema 会在旧 `orders` 表缺少 `ship_tracking_no` 时先读旧字段再建 tracking 表；已在 `ensureOrderShippingTrackingTables` 开头补 `ALTER TABLE orders ADD COLUMN IF NOT EXISTS ship_tracking_no`，并新增 `TestEnsureOrderShippingTrackingTablesAddsLegacyTrackingColumnBeforeBackfill` 源码守卫。
- 真实 PostgreSQL 结果：`customerportal`、`customerfulfillment` 定向集成测试通过；全量 `go test ./internal/infrastructure/postgres/... -count=1` 通过，覆盖 `authz/catalog/costing/customer/customerportal/customerfulfillment/finance/materials/production/sales/stock`。

## 本地真实服务与浏览器 Smoke
- 本地用一次性 PostgreSQL schema `kferp_e2e` 启动真实后端：`DATABASE_URL=ephemeral DB_SCHEMA=kferp_e2e APP_USER=order APP_PASS=secret LISTEN=127.0.0.1:18080 CUSTOMER_PORTAL_DEV_LOGIN=true go run .`。
- 种子数据包含三类模板客户、三个渠道客户账号、一个小程序用户绑定、一个商城商品；通过真实管理 API 应用 `processing_fulfillment`、`public_sku_direct_ship`、`retail_mall` 模板，并验证 retail mall 绑定 ERP 工作台账号返回 `400 {"error":"ERP workbench unavailable for capability template"}`。
- 真实小程序 HTTP smoke：
  - `processing_fulfillment`：切换客户、订单页、代发批次、代加工申请、代加工发货单均 `200`，商城页 `403`。
  - `public_sku_direct_ship`：切换客户、现货下单页、现货订单均 `200`，代加工申请 `403`。
  - `retail_mall`：切换客户、商城页、商城下单、我的订单均 `200`，代发批次 `403`。
- 追加真实小程序 HTTP 矩阵（一次性 PostgreSQL `kferp_wechat` + 本地后端 `127.0.0.1:18094`，stamp `20260513173535`）：36 项全部通过。processing 客户登录/`me`/订单/代发/代加工/库存/结算均 `200`，`productOrder` 和商城 `403`；成功创建 `DS-20260514-0003`、`PJ-20260514-0001`、`SO-20260514-0001` 和 `SO-20260514-0002:CUST-PROC-101`。public SKU 客户订单/代发/现货/结算均 `200`，代加工/库存/商城 `403`；成功创建 `DS-20260514-0004`、`SO-20260514-0003` 和 `SO-20260514-0004`。retail mall 客户订单和商城均 `200`，代发/现货/代加工/库存/结算均 `403`；成功创建商城单 `SO-20260514-0005`。
- 追加数据库落库核对：`orders` 中 `notes='three-template-api-smoke'` 汇总为 `101/direct_ship/finished_goods=1`、`101/processing_ship/CUST-PROC-101=1`、`102/direct_ship/finished_goods=1`、`102/product_order/finished_goods=1`、`103/mall/finished_goods=1`；`processing_job_requests` 中 `101/submitted=1`；`direct_ship_import_batches` 中 `101/submitted=1`、`102/submitted=1`。
- 真实 ERP API smoke：`/api/produce/unproduced?plan=1`、`/api/finance/dashboard?month=2026-05`、`/api/finance/reports/2026-05`、`/api/customer-portal/admin/customers?q=E2E` 均返回 `200`。
- Chrome headless 浏览器证据：加载真实 `/vue-shell?view=customerPortalSettings` 后 DOM 包含 `客户门户配置`、`E2E代加工客户`、`E2E公共SKU代发客户`、`E2E零售商城客户` 和 `该模板不开放 ERP 工作台`；截图已生成到临时文件 `/tmp/kferp-vue-shell-customer-portal.png`。
- 追加多页面 DOM smoke：用一次性 PostgreSQL schema `kferp_dom` 启动真实后端到 `127.0.0.1:18081`，经真实管理/小程序 API 生成代加工发货单、公共 SKU 现货单、零售商城单和 `SO-20260513-PAGE` 历史单；Chrome headless DOM 验证通过 `customerPortalSettings`、`orders`、`producePlan`、`financeDashboard`、`customerProcessingPortal(processing)`、`customerProcessingPortal(public SKU)`，证据输出 `EXTENDED_DOM_SMOKE_OK app=http://127.0.0.1:18081 pg=55435`。
- 该 smoke 暴露并修复一个真实数据鲁棒性问题：同日已有 `SO-YYYYMMDD-PAGE` 这类非数字后缀订单号时，小程序再创建客户门户订单会因 `right(order_no,4)::int` 转换失败返回 `500`。已新增 RED/GREEN 测试 `TestNextCustomerPortalOrderNoIgnoresNonNumericSameDaySuffix`，并让 `nextCustomerPortalOrderNo` 只统计四位数字后缀。

## 浏览器点击级真实服务 Smoke
- 新增 `PR-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE`、`DEV-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE`、`UT-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE`、`API-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE`、`REV-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE`，把真实后端、真实 PostgreSQL、真实 Chrome CDP 点击级验证纳入本轮验收证据。
- 用一次性 PostgreSQL schema `kferp_click` 启动真实后端到 `127.0.0.1:18085`，种三类模板客户、管理员账号、两个渠道客户账号、商城商品、生产订单、客户托管生豆、财务费用和客户 ERP 绑定；Chrome headless 通过 CDP 使用真实 DOM 点击和输入。
- 点击级订单证据：进入 `/vue-shell?view=orders`，点击 `SO-20260513-PAGE` 订单号，等待 `aria-label="订单详情"` 抽屉显示该订单。
- 点击级财务证据：从 `/vue-shell?view=financeDashboard` 点击“费用管理”，在费用页输入 `点击级财务费用`、金额、付款方式和备注，点击“保存”，页面显示“已保存”和新费用行。
- 点击级生产证据：进入 `/vue-shell?view=producePlan`，点击“全选库存不足商品”和“生成计划”，生产计划区、物料汇总和烘焙建议显示 `E2E代加工成品`。
- 点击级客户履约工作台证据：用 processing 渠道账号进入 `customerProcessingPortal`，选择客户 SKU 和托管生豆，提交加工工单并出现 `已提交工单 CP-`；再填写收件信息、选择商品并提交代发，出现 `已提交代发 CDS-`。
- 点击级 public SKU 边界证据：切换 public SKU 渠道账号进入 `customerProcessingPortal`，页面显示 `E2E公共SKU代发客户` 和“提交代发信息”，且不显示“提交加工工单”。
- 最终输出：`CLICK_LEVEL_SMOKE_OK app=http://127.0.0.1:18085`。该 smoke 覆盖 ERP 主干点击流，但不替代微信开发者工具里的小程序真实点击。
- 追加生产/财务闭环点击证据：用一次性 PostgreSQL schema `kferp_click_ext` 启动真实后端到 `127.0.0.1:18090`，种管理员、生产订单、真实 BOM 明细、WIP 生豆批次 `E2E完工点击生豆` 和财务费用；Chrome CDP 点击“全选库存不足商品”→“生成计划”→“开始生产”，再在生产中页点击“完成”，页面出现 `生产已完成`，数据库确认 `running_done=1`。
- 追加财务结账点击证据：同一真实服务里从财务首页点击“经营报告”，页面包含 `月度经营报告`、`来源明细`、`SO-20260513-FINISHCLICK` 和 `点击级财务费用`；再点击“月度结账”→“结账”→录入 `点击级月结调整`→“新增调整”，页面出现 `已结账`、`已新增调整`，数据库确认 `adjustments=1`、`monthly_status=adjusted`。
- 追加输出：`EXTENDED_CLICK_LEVEL_SMOKE_OK app=http://127.0.0.1:18090 pg=55448`。

## 客户账号隔离与服务端能力兜底
- 新增 `PR-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD`、`DEV-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD`、`UT-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD`、`API-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD`、`REV-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD`，把渠道客户 ERP 工作台的服务端能力校验和多账号隔离作为独立验收项。
- 本轮发现 `customerProcessingPortal` 前端会按 `overview.capabilities` 隐藏代加工/代发表单，但后端 `/api/customer-processing/portal/work-orders` 与 `/api/customer-processing/portal/direct-ship-orders` 共用 `customer_processing.submit` 权限，仓储层只按 `EmployeeID` 解析绑定客户，未再次校验绑定客户是否具备 `processing` 或 `direct_ship` 能力。公共 SKU 渠道账号理论上可绕过 UI 直接 POST 代加工工单。
- 修复：`SubmitCustomerProcessingWorkOrder` 和 `SubmitCustomerDirectShipOrder` 在 portal 路径（`CustomerID <= 0`、由绑定员工解析客户）调用 `requireCustomerCapability(ctx, customerID, "processing")` 或 `requireCustomerCapability(ctx, customerID, "direct_ship")`；内部/管理侧显式 `CustomerID` 写入保持原业务能力，避免破坏已有后台批量导入或管理操作。
- 单元/源码证据：`TestCustomerPortalSubmitRequiresBoundCustomerCapability` 验证 direct-ship-only 客户账号提交加工返回 `customer capability processing unavailable`，processing-only 客户账号提交代发返回 `customer capability direct_ship unavailable`，且错误路径不写入错客户业务表；`TestCustomerPortalDirectShipSubmitRepositoryWiresERPOrderCreation` 固定两个 capability gate 的源码守卫。
- 真实 PostgreSQL 证据：一次性测试库执行 customerfulfillment 定向测试通过，输出 `REAL_PG_CAPABILITY_GATE_OK pg=55450`。
- 真实服务/Chrome CDP 隔离证据：用一次性 PostgreSQL schema `kferp_isolation` 启动真实后端到 `127.0.0.1:18091`，种 `E2E隔离代加工客户` 与 `E2E隔离公共SKU客户` 两个渠道账号。processing 账号页面显示本客户、`提交加工工单` 和 `A隔离生豆`，不显示 public SKU 客户与 `B隔离地址`，并可通过真实 API 提交本客户代发单；public SKU 账号页面显示本客户、`提交代发信息` 和 `B隔离地址`，不显示 `提交加工工单` 或 `A隔离生豆`，直接 POST 加工接口返回 `400 customer capability processing unavailable`。
- DB 隔离确认：`a_direct_ship_rows=1`、`b_processing_rows=0`；最终输出 `CUSTOMER_ACCOUNT_ISOLATION_SMOKE_OK app=http://127.0.0.1:18091 pg=55451`。
- 操作手册证据：`OP_MANUAL_CUSTOMER_PORTAL.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md` 已补“客户 ERP 工作台账号隔离”，说明代加工模板账号才会显示和提交加工工单、公共 SKU 代发模板账号只处理代发信息，以及 `customer capability ... unavailable` 的处理方式。

## 订单列表范围 fail closed
- 新增 `PR-182-ORDER-LIST-SCOPE-FAIL-CLOSED`、`DEV-182-ORDER-LIST-SCOPE-FAIL-CLOSED`、`UT-182-ORDER-LIST-SCOPE-FAIL-CLOSED`、`API-182-ORDER-LIST-SCOPE-FAIL-CLOSED`、`REV-182-ORDER-LIST-SCOPE-FAIL-CLOSED`，把订单列表范围参数从“未知值默认全量”改为 fail closed。
- 问题：订单列表仓储层只处理 `mine` 和 `fulfillment`，API 层未校验 scope；如果外部通知链接或手工 URL 带 `scope=fulfillment_typo`，会绕过履约范围过滤并展示全部订单。虽然渠道客户账号没有 `orders.read`，但对后台用户来说这是不安全的范围放宽，也会让通知链接错误难以发现。
- 修复：`ordersQueryFromContext` 调用 `validOrderListScope`，只接受空值、`all`、`mine`、`fulfillment`；非法值在进入服务/仓储前返回 `400 {"error":"invalid scope"}`。
- RED/GREEN 证据：`TestOrderAPIListRejectsInvalidScope` 先复现 `/api/orders?scope=fulfillment_typo` 返回 `200` 且调用仓储的问题；修复后同一测试确认返回 `400`、响应包含 `invalid scope`、仓储未被调用。
- 前端证据：`OrdersView.vue` 改用 `orderListScopeForRequest`，保留 URL 或通知参数中的非法 scope 传给 API，而不是在浏览器端归一化为 `all`；`order-scope.test.js` 覆盖 `fulfillment_typo` 会被保留。
- 浏览器证据：服务 built Vue shell 到 `http://127.0.0.1:18121`，Chrome CDP 加载 `/vue-shell?view=orders&scope=fulfillment_typo`，mock API 记录 `API_ORDERS_SCOPE fulfillment_typo`，页面显示 `invalid scope`；输出 `ORDER_SCOPE_BROWSER_FAIL_CLOSED_OK app=http://127.0.0.1:18121 scope=fulfillment_typo text=invalid_scope apiScopes=fulfillment_typo`。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充非法订单范围不能退化为全部订单。

## 小程序入口模式持久化
- 新增 `PR-183-MINIAPP-ENTRY-MODE-PERSISTENCE`、`DEV-183-MINIAPP-ENTRY-MODE-PERSISTENCE`、`UT-183-MINIAPP-ENTRY-MODE-PERSISTENCE`、`API-183-MINIAPP-ENTRY-MODE-PERSISTENCE`、`REV-183-MINIAPP-ENTRY-MODE-PERSISTENCE`，确保小程序登录、刷新 `/api/mini/me`、打开服务页和切换当前客户后都返回或保留当前客户真实 `miniapp_entry_mode`。
- 问题：微信开发者工具 GUI 导入当前分支小程序并准备真实运行前，本地真实 API readiness 发现 retail mall 客户具备 `mall` 能力，但 `/api/mini/me` 返回 `"miniapp_entry_mode":"services"`。根因是 PostgreSQL customerportal 仓储只读取 `theme_key`，没有读取 `customer_portal_profiles.miniapp_entry_mode`，应用层把空值归一化为 `services`。
- 修复：`CreateLoginSession` 和 `CurrentContextByToken` 均调用 `miniappEntryModeForCustomerTx`，从当前客户 profile 读取入口模式；无客户或无 profile 时仍回退 `services`。`GetServicePage` 也把当前上下文的 `miniapp_entry_mode` 序列化给小程序，服务页前端用 `page.value.miniapp_entry_mode || session.entryMode` 保留商城首页模式，避免零售商城客户从商城点“我的订单”后在内存会话中退回 `services`。
- RED/GREEN 证据：新增 `TestCurrentContextByTokenReturnsCurrentCustomerMiniappEntryMode` 和 `TestCreateLoginSessionReturnsCurrentCustomerMiniappEntryMode`，修复前真实 PostgreSQL 测试返回空入口模式失败，修复后通过。
- API 证据：`TestMiniLoginAndMeAPI` 固定 `/api/mini/login` 和 `/api/mini/me` 必须序列化 `miniapp_entry_mode`；`TestMiniServicePageAPIRequiresTokenAndReturnsScopedData` 固定 `/api/mini/services/:key` 也返回 `miniapp_entry_mode`；miniapp `mall.test.ts` 固定服务页保留 `page.value.miniapp_entry_mode || session.entryMode`。一次性 PostgreSQL + 本地真实后端 `127.0.0.1:18094` readiness 输出 `WECHAT_GUI_LOCAL_API_READY app=http://127.0.0.1:18094 pg=55462 token=aef28131... processing=ok public=ok retail=ok`，覆盖 processing/public SKU 为 `services`、retail mall 为 `mall`，以及 public SKU/retail mall 越权服务 `403`。
- 微信开发者工具状态：已通过 GUI/CLI 导入当前分支 `miniapp/dist/build/mp-weixin`，并用 `VITE_KFERP_API_BASE=http://127.0.0.1:18094` 重新构建；Service Port 已开启。Van 已允许 Trust & Run/Service Port 后，GUI 可打开当前项目并完成登录，页面进入 `三模板-代加工履约客户` 首页。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充零售商城客户入口模式刷新/排障要求。

## 公共 SKU 小批量小程序订单计价
- 新增 `PR-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING`、`DEV-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING`、`UT-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING`、`API-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING`、`REV-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING`，确保公共 SKU 小批量客户从小程序服务页提交现货/代发订单时复用模板内置的小批量价格规则。
- 问题：普通 ERP 录单已经支持 `<14lb` 使用 `15-28lb` 豆单重量档，但小程序服务页 `CreateFulfillmentOrder` 只按当前规格找价格档，非 454g 规格没有同规格档时会回退产品默认价。真实 PostgreSQL RED：`TestCreateFulfillmentOrderUsesSmallBatchWeightTierForNon454Spec` 保存 1000g × 1 公共 SKU 订单时得到 `unit_price/line_total=999/999`，而不是从 15-28lb 档折算出的 `198/198`。
- 修复：`portalFulfillmentUnitPriceTx` 保留同规格价格档优先，同时增加豆单重量档 fallback；`portalPackageUnitPriceFromLb` 将 `price_per_lb` 按当前规格折算成订单明细的包价。1000g 等非 454g 规格低于 14lb 时按模板规则使用 15-28lb 档，缺少任何可用价格档时才回退产品默认价。
- RED/GREEN 证据：`ORDERAPP_TEST_DATABASE_URL=host=127.0.0.1 port=55463 ... go test ./internal/infrastructure/postgres/customerportal -run TestCreateFulfillmentOrderUsesSmallBatchWeightTierForNon454Spec -count=1`，修复前失败为 `999/999`，修复后通过。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充公共 SKU 小批量小程序订单按重量档折算计价和异常排查。

## 结算服务页客户隔离
- 新增 `PR-185-SETTLEMENT-SERVICE-PAGE-ISOLATION`、`DEV-185-SETTLEMENT-SERVICE-PAGE-ISOLATION`、`UT-185-SETTLEMENT-SERVICE-PAGE-ISOLATION`、`API-185-SETTLEMENT-SERVICE-PAGE-ISOLATION`、`REV-185-SETTLEMENT-SERVICE-PAGE-ISOLATION`，把小程序结算服务页的费用明细和结算单客户隔离作为独立数据安全验收项。
- 业务规则：`/api/mini/services/settlement` 只能返回当前 token 绑定客户自己的 `customer_fee_items` 和 `customer_settlement_batches`，不能显示其他客户的费用明细、结算单号或金额。
- 真实 PostgreSQL 证据：`TestLoadSettlementServicePageFiltersFinanceRowsByCustomer` 在同一个测试库创建客户 A/B 的费用明细和结算单，调用 `LoadServicePage(ServiceKeySettlement)` 只返回 `客户A费用` 和 `客户A结算单`，并验证 `客户B不应泄露` 不会出现在结果中。
- API 证据：`TestMiniSettlementServicePageAPIReturnsFinancePayload` 固定小程序 settlement 服务 API 会序列化 `fee_items` 和 `settlement_batches`，该 API 继续复用 `GetServicePage` 的当前客户查询边界。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充结算服务页只能看到当前客户财务数据，以及发现串客户时的排查动作。

## 小程序当前客户订单隔离
- 新增 `PR-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION`、`DEV-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION`、`UT-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION`、`API-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION`、`REV-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION`，把同一小程序用户绑定多个客户时的当前客户订单隔离作为独立会话安全验收项。
- 业务规则：`/api/mini/current-customer` 只能切换到 approved 绑定客户；切换后“我的订单”等服务页必须使用新的 `current_customer_id` 查询，不能显示切换前客户的订单号、商品或金额。
- 真实 PostgreSQL 证据：`TestMiniappCurrentCustomerSwitchScopesOrderServicePage` 创建同一个 mini user、客户 A/B、两个 approved 绑定和两边订单，先验证 `GetServicePage(ServiceKeyOrders)` 只返回 `SO-CURRENT-A`，再调用 `SwitchCurrentCustomer` 切到客户 B，并验证订单服务页只返回 `SO-CURRENT-B`，且 `customer A order leaked after switch` 不会发生。`TestMiniappCurrentCustomerSwitchRejectsUnapprovedCustomerWithoutChangingSession` 进一步验证 pending 绑定不能切换，且 rejected switch 不会改写 session 当前客户。
- API/服务证据：该测试使用真实 `customerportalapp.NewService(repo, nil)` 串联 `SwitchCurrentCustomer` 与 `GetServicePage`，覆盖小程序 API 所依赖的服务边界，而不是只检查 SQL 子查询。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充切换当前客户后“我的订单”不能显示切换前客户数据，以及发现串客户时的排查动作。

## 小程序停用客户绑定边界
- 新增 `PR-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD`、`DEV-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD`、`UT-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD`、`API-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD`、`REV-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD`，把客户档案停用后的旧小程序绑定失效作为客户账号安全验收项。
- 问题：`customer_portal_user_bindings.status='approved'` 仍会被 `CurrentContextByToken` 和 `SwitchCurrentCustomer` 接受，即使绑定指向的 `customers.active=false`。停用客户可能继续作为小程序当前客户，或通过旧 approved binding 被切换进入。
- 修复：`listBindingsTx` 只返回 `b.status='approved' AND c.active=true` 的绑定；`SwitchCurrentCustomer` 的绑定校验也 join `customers c ON c.id=b.customer_id AND c.active=true`。原当前客户停用后会自动切到第一个可用客户或清空，停用客户不能切换进入。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestCurrentContextByTokenSwitchesInactiveCurrentCustomerToFirstActiveBinding` 修复前返回 `已停用客户`，期望 `可用客户`；`TestSwitchCurrentCustomerRejectsInactiveApprovedCustomer` 修复前错误为 nil，修复后返回 `ErrCustomerBindingNotFound` 且不改写当前可用客户。
- API 证据：`TestMiniCurrentCustomerInactiveBindingMapsToForbidden` 固定小程序 `/api/mini/current-customer` 对停用客户旧绑定返回 `403 {"error":"customer binding not found"}`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充停用客户不能继续作为小程序当前客户，不能切换进入停用客户。

## 客户履约结算能力边界
- 新增 `PR-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD`、`DEV-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD`、`UT-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD`、`API-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD`、`REV-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD`，把客户履约工作台生成月结前必须具备结算能力作为财务/工作台安全验收项。
- 问题：`CreateSettlement` 原来只按 `customer_id` 和期间创建 `customer_settlement_batches`，没有确认客户是否开通 `settlement` 能力；零售商城或未开通结算能力的客户如果被直接请求 API，可能留下空结算批次并混淆月结口径。
- 修复：`CreateSettlement` 在写入结算批次前复用 `requireCustomerCapability(ctx, cmd.CustomerID, "settlement")`；未开通结算能力的客户直接返回 `customer capability settlement unavailable`，不创建结算批次，也不改写未结算费用。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestCreateSettlementRejectsCustomerWithoutSettlementCapability` 修复前 `err=nil`，修复后返回 settlement capability unavailable，且 `customer_settlement_batches` 仍为 0、费用保持 `status='unsettled'`。
- API 证据：`TestCreateSettlementAPICapabilityUnavailableMapsToBadRequest` 固定 `/api/customer-fulfillment/:customer_id/settlements` 对未开通结算能力错误返回 400 和可理解错误文本。
- 真实后端证据：`CUSTOMER_SETTLEMENT_CAPABILITY_REAL_API_OK app=http://127.0.0.1:18157 pg=55627 evidence=POST_/api/customer-fulfillment/21101/settlements_400 db=customer_settlement_batches_0_unsettled_fees_1 error=customer_capability_settlement_unavailable`；一次性 PostgreSQL `kferp_pr211214_api` + 本地后端真实 POST 生成月结，未开通 `settlement` 能力客户返回 400，数据库核对未生成结算批次且原未结费用仍保留。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充未开通结算能力的客户不能生成结算批次。

## 客户履约导入应用能力边界
- 新增 `PR-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD`、`DEV-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD`、`UT-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD`、`API-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD`、`REV-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD`，把客户履约 Excel 应用导入批次前必须校验客户能力作为工作台/订单/财务安全验收项。
- 问题：`ApplyImport` 原来按导入批次的 `import_type` 直接加载有效行并写加工工单、代发订单或费用数据，没有确认目标客户是否开通 `processing`、`direct_ship` 或 `settlement` 能力；直接调用 API 可能给零售商城或未开能力客户写入错误业务数据。
- 修复：新增 `capabilityForImportType`，把 `processing_workbook`、`direct_ship_workbook`、`settlement_workbook` 映射到 `processing`、`direct_ship`、`settlement`；`ApplyImport` 在加载和应用有效行前复用 `requireCustomerCapability(ctx, customerID, capability)`，未开通对应能力时直接返回 `customer capability ... unavailable`。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestApplyDirectShipImportRejectsCustomerWithoutDirectShipCapability` 修复前 `ApplyImport err=<nil>` 且可继续写代发导入业务，修复后返回 `customer capability direct_ship unavailable`，`customer_direct_ship_import_orders` 和 `orders.portal_service_code='direct_ship'` 均不写入。
- API 证据：`TestApplyImportAPICapabilityUnavailableMapsToBadRequest` 固定 `/api/customer-fulfillment/imports/:id/apply` 对客户能力不可用错误返回 400 和可理解错误文本。
- 真实后端证据：`CUSTOMER_IMPORT_APPLY_CAPABILITY_REAL_API_OK app=http://127.0.0.1:18157 pg=55627 evidence=POST_/api/customer-fulfillment/imports/212101/apply_400 db=direct_ship_import_orders_0_erp_direct_ship_orders_0 error=customer_capability_direct_ship_unavailable`；一次性 PostgreSQL `kferp_pr211214_api` + 本地后端真实 POST 应用代发导入批次，未开通 `direct_ship` 能力客户返回 400，数据库核对未写代发导入订单或 ERP 代发订单。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充应用导入批次前必须确认导入类型对应客户能力，未开通对应能力时不写加工工单、代发订单、费用或结算数据。

## 客户履约手工提交能力边界
- 新增 `PR-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD`、`DEV-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD`、`UT-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD`、`API-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD`、`REV-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD`，把客户履约账户手工提交代加工工单/代发订单前必须校验客户能力作为工作台/订单安全验收项。
- 问题：`SubmitCustomerProcessingWorkOrder` 和 `SubmitCustomerDirectShipOrder` 之前只在 `customerID <= 0` 的客户门户账号路径校验能力；ERP 内部 API 显式带 `customer_id` 调用时会跳过 `processing`/`direct_ship` 能力判断，可能给零售商城或未开能力客户写工单、代发导入单和 ERP 代发订单。
- 修复：两个仓储方法在解析出最终 `customerID` 后统一调用 `requireCustomerCapability(ctx, customerID, "processing")` 或 `requireCustomerCapability(ctx, customerID, "direct_ship")`，不再区分客户门户路径和内部路径。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestInternalCustomerFulfillmentSubmitRequiresCustomerCapability` 修复前内部代加工提交 `err=<nil>`，修复后返回 `customer capability processing unavailable`/`customer capability direct_ship unavailable`，且 `customer_processing_work_orders`、`customer_direct_ship_import_orders` 和 `orders.portal_service_code='direct_ship'` 都不写入。
- API 证据：`TestInternalSubmitAPICapabilityUnavailableMapsToBadRequest` 固定 `/api/customer-fulfillment/:customer_id/work-orders` 和 `/api/customer-fulfillment/:customer_id/direct-ship-orders` 对能力不可用错误返回 400 和可理解错误文本。
- 真实后端证据：`CUSTOMER_INTERNAL_SUBMIT_CAPABILITY_REAL_API_OK app=http://127.0.0.1:18157 pg=55627 evidence=POST_work_orders_400_POST_direct_ship_orders_400 db=processing_orders_0_direct_ship_import_orders_0_erp_direct_ship_orders_0 errors=customer_capability_processing_unavailable/customer_capability_direct_ship_unavailable`；一次性 PostgreSQL `kferp_pr211214_api` + 本地后端真实 POST 内部代加工工单和内部代发订单，未开通对应能力客户均返回 400，数据库核对未写工单、代发导入单或 ERP 代发订单。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充手工提交工单或代发订单前必须确认客户开通对应能力，未开通时不写客户履约或 ERP 订单数据。

## 客户履约托管库存调整能力边界
- 新增 `PR-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD`、`DEV-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD`、`UT-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD`、`API-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD`、`REV-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD`，把客户履约账户手工调整托管库存前必须校验托管库存能力作为工作台/库存安全验收项。
- 问题：`AdjustCustodyInventory` 原来直接按显式 `customer_id` upsert 托管库存物料并写 `customer_custody_ledger_entries` 和 `customer_custody_balances`，没有确认目标客户是否开通 `inventory_custody` 能力；直接调用 API 可能给零售商城或未开托管能力客户写库存台账。
- 修复：`AdjustCustodyInventory` 在开启事务和写库存物料前统一调用 `requireCustomerCapability(ctx, cmd.CustomerID, "inventory_custody")`；未开通托管库存能力时返回 `customer capability inventory_custody unavailable`，不写库存物料、台账或余额。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestAdjustCustodyInventoryRequiresCustomerInventoryCapability` 修复前 `AdjustCustodyInventory err=<nil>`，修复后返回 `customer capability inventory_custody unavailable`，且 `customer_custody_items`、`customer_custody_ledger_entries`、`customer_custody_balances` 都不写入。
- API 证据：`TestCustodyAdjustmentAPICapabilityUnavailableMapsToBadRequest` 固定 `/api/customer-fulfillment/:customer_id/custody-adjustments` 对托管库存能力不可用错误返回 400 和可理解错误文本。
- 真实后端证据：`CUSTOMER_CUSTODY_ADJUSTMENT_CAPABILITY_REAL_API_OK app=http://127.0.0.1:18157 pg=55627 evidence=POST_/api/customer-fulfillment/21401/custody-adjustments_400 db=custody_items_0_ledger_0_balances_0 error=customer_capability_inventory_custody_unavailable`；一次性 PostgreSQL `kferp_pr211214_api` + 本地后端真实 POST 托管库存调整，未开通 `inventory_custody` 能力客户返回 400，数据库核对未写库存物料、台账或余额。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充手工调整托管库存前必须确认客户开通 `inventory_custody`，未开通时不写库存数据。

## 客户履约内部 ERP 绑定工作台边界
- 新增 `PR-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD`、`DEV-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD`、`UT-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD`、`API-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD`、`REV-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD`，把客户履约内部 ERP 绑定入口也纳入能力模板 ERP 工作台边界。
- 问题：客户门户配置服务已经拒绝 `retail_mall` 等不暴露 ERP 工作台的模板绑定 ERP 工作台账号，但客户履约模块自己的 `/api/customer-fulfillment/:customer_id/erp-bindings` 会直接写 `customer_erp_user_bindings`；直接调用该内部 API 可绕过门户配置页，把零售商城客户绑定到批发履约工作台。
- 修复：`UpsertCustomerERPBinding` 在 active 绑定写入前调用 `requireCustomerERPWorkbenchTemplateTx`，读取目标客户 `customer_portal_profiles.capability_template_key` 并用 `CustomerCapabilityTemplateByKey` 判断模板是否 `ExposesERPWorkbench()`；不暴露 ERP 工作台时返回 `ERP workbench unavailable for capability template`，不写 active binding 或隐藏角色。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestUpsertCustomerERPBindingRejectsTemplateWithoutERPWorkbench` 修复前 `UpsertCustomerERPBinding err=<nil>`，修复后返回 ERP workbench template rejection，且 `customer_erp_user_bindings` active 绑定和 `employee_roles` 都为 0。
- API 证据：`TestInternalERPBindingAPIWorkbenchUnavailableMapsToBadRequest` 固定 `/api/customer-fulfillment/:customer_id/erp-bindings` 对模板不支持 ERP 工作台错误返回 400 和可理解错误文本。
- 真实后端证据：`CUSTOMER_ERP_BINDING_WORKBENCH_REAL_API_OK app=http://127.0.0.1:18158 pg=55628 evidence=POST_/api/customer-fulfillment/21501/erp-bindings_400 db=active_bindings_0_employee_roles_0 template=retail_mall error=ERP_workbench_unavailable_for_capability_template`；一次性 PostgreSQL `kferp_pr215_api` + 本地后端真实 POST 内部 ERP 绑定，`retail_mall` 客户返回 400，数据库核对未写 active 绑定或隐藏角色。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户履约内部 API 不能绕过 ERP 工作台绑定限制。

## 客户履约历史 ERP 绑定上下文边界
- 新增 `PR-216-CUSTOMER-FULFILLMENT-LEGACY-ERP-BINDING-WORKBENCH-CONTEXT-GUARD`、`DEV-216-CUSTOMER-FULFILLMENT-LEGACY-ERP-BINDING-WORKBENCH-CONTEXT-GUARD`，把历史遗留 active ERP 工作台绑定也纳入能力模板 ERP 工作台边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：PR-215 已阻止新建错误绑定，但历史遗留 `customer_erp_user_bindings.status='active'` 如果指向 `retail_mall` 或其他不暴露 ERP 工作台的模板，`CustomerPortalContext` 原来只校验 active 绑定和 active 客户，仍会让渠道账号进入客户履约工作台。
- 修复：`CustomerPortalContext` 读取 active 绑定时同时读取 `customer_portal_profiles.capability_template_key`，使用 `CustomerCapabilityTemplateByKey` 和 `ExposesERPWorkbench()` 跳过不暴露 ERP 工作台的模板；如果没有有效绑定，返回 `ErrCustomerERPBindingNotFound`，客户工作台 overview API 映射为 403。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestCustomerPortalContextRejectsLegacyBindingWithoutERPWorkbench` 修复前 `CustomerPortalContext err=<nil>`，修复后 `CustomerPortalContext` 和 `CustomerPortalOverview` 都返回 `ErrCustomerERPBindingNotFound`。
- API 证据：`TestCustomerPortalOverviewAPILegacyWorkbenchBindingMapsToForbidden` 固定 `/api/customer-processing/portal/overview` 在无有效 ERP 工作台绑定时返回 403。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充历史 active 绑定指向非工作台模板时按无有效绑定拒绝进入。

## 订单履约客户范围工作台模板边界
- 新增 `PR-217-ORDER-FULFILLMENT-SCOPE-WORKBENCH-TEMPLATE-GUARD`、`DEV-217-ORDER-FULFILLMENT-SCOPE-WORKBENCH-TEMPLATE-GUARD`，把订单列表“履约客户订单”范围也纳入能力模板 ERP 工作台边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：`scope=fulfillment` 原来只要求客户为批发客户、订单来源是客户门户履约服务、且存在 active `customer_erp_user_bindings`。如果历史 active 绑定指向 `retail_mall` 或其他不暴露 ERP 工作台的模板，客户履约工作台 overview 已会拒绝，但订单列表仍可能显示该客户的履约订单。
- 修复：`orderListWhere` 在 fulfillment scope 的 active 绑定子查询中同时关联 `customer_portal_profiles`，只允许 `processing_fulfillment`、`public_sku_direct_ship` 或无历史 profile 的老批发绑定进入；`retail_mall` 等非工作台模板绑定不再让订单进入“履约客户订单”范围。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestOrderAPIListFulfillmentScopeSkipsLegacyNonWorkbenchBinding` 修复前同时返回 `SO-VALID-PROCESSING-WORKBENCH` 和 `SO-LEGACY-RETAIL-WORKBENCH`，修复后只返回有效代加工模板订单。
- API/查询证据：`TestOrderListWhereSupportsMineAndFulfillmentScopes` 固定 fulfillment scope 查询包含 `customer_portal_profiles`、`capability_template_key`、`processing_fulfillment` 和 `public_sku_direct_ship`，防止范围过滤退回只看 active binding。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充“履约客户订单”只包含已绑定 ERP 履约账号且能力模板暴露 ERP 工作台的批发客户订单。

## 客户履约客户选择器工作台模板边界
- 新增 `PR-218-CUSTOMER-FULFILLMENT-PICKER-WORKBENCH-TEMPLATE-GUARD`、`DEV-218-CUSTOMER-FULFILLMENT-PICKER-WORKBENCH-TEMPLATE-GUARD`，把客户履约账户客户选择器也纳入能力模板 ERP 工作台边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：`/api/customer-fulfillment/customers` 原来只要求客户为启用批发客户且存在 active `customer_erp_user_bindings`。如果历史 active 绑定指向 `retail_mall` 或其他不暴露 ERP 工作台的模板，客户工作台 overview 会拒绝、订单范围会隐藏，但客户选择器仍会展示该客户，造成运营误选和操作路径不一致。
- 修复：新增 `CustomerERPWorkbenchAvailable`，读取 `customer_portal_profiles.capability_template_key` 并复用能力模板 `ExposesERPWorkbench()` 判断；客户选择器在发现 active ERP 绑定后继续校验该客户是否真的暴露 ERP 工作台，非工作台模板不返回给选择器，同时保留原始绑定列表给运营排查历史绑定。
- RED/GREEN 证据：`TestCustomerOptionsAPISkipsLegacyNonWorkbenchBinding` 修复前选择器响应同时包含“零售商城历史绑定客户”和“代加工工作台客户”，修复后只保留代加工工作台客户；真实 PostgreSQL 下 `TestCustomerERPWorkbenchAvailableRejectsTemplateWithoutWorkbench` 固定 `retail_mall` 模板返回不可用。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户选择器只展示已绑定 ERP 履约账号且能力模板暴露 ERP 工作台的批发客户。

## 客户履约未知模板工作台 Fail-Closed
- 新增 `PR-219-CUSTOMER-FULFILLMENT-UNKNOWN-TEMPLATE-WORKBENCH-FAIL-CLOSED`、`DEV-219-CUSTOMER-FULFILLMENT-UNKNOWN-TEMPLATE-WORKBENCH-FAIL-CLOSED`，把非空但无法识别的 `capability_template_key` 作为工作台边界的脏数据处理。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：PR-216/218 已过滤 `retail_mall` 等已知非工作台模板，但 helper 对非空未知模板返回可用；如果历史数据或手工 SQL 写入未知模板键，新建内部 ERP 绑定、历史客户工作台上下文和客户选择器仍可能把它当作工作台模板放行。
- 修复：`customerERPWorkbenchAvailableForTemplateKey` 对空模板保留老批发客户兼容，对非空模板要求 `CustomerCapabilityTemplateByKey` 能识别且 `ExposesERPWorkbench()` 为真；内部绑定、历史上下文和客户选择器统一复用该 fail-closed 判定。
- RED/GREEN 证据：真实 PostgreSQL 下 `TestUpsertCustomerERPBindingRejectsUnknownTemplateKey` 修复前 `err=<nil>`，修复后返回 `ERP workbench unavailable for capability template` 且不写 active binding；`TestCustomerPortalContextRejectsLegacyBindingWithUnknownTemplateKey` 修复前 `err=<nil>`，修复后返回 `ErrCustomerERPBindingNotFound`；`TestCustomerERPWorkbenchAvailableRejectsUnknownTemplateKey` 修复前返回 true，修复后返回 false。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充无法识别模板不能绑定或进入 ERP 工作台，也不会出现在客户选择器中。

## 客户门户未知模板工作台绑定 Fail-Closed
- 新增 `PR-220-CUSTOMER-PORTAL-UNKNOWN-TEMPLATE-WORKBENCH-FAIL-CLOSED`、`DEV-220-CUSTOMER-PORTAL-UNKNOWN-TEMPLATE-WORKBENCH-FAIL-CLOSED`，把客户门户配置页 ERP 工作台绑定入口也纳入未知模板 fail-closed。
- 问题：客户门户配置服务的 `UpsertPortalERPBinding` 已拒绝 `retail_mall` 等已知不暴露 ERP 工作台模板，但对非空未知模板键会因 `ok=false` 放行；历史脏 profile 仍可能通过门户配置入口写 active ERP 绑定。
- 修复：active 绑定前读取 `PortalAdminDetail.Customer.CapabilityTemplateKey`，只要原始模板键非空但 `capabilityTemplateByKeyStrict` 无法识别，就返回 `ErrCapabilityTemplateERPWorkbenchUnavailable`，不委托仓储写绑定。
- RED/GREEN 证据：`TestUpsertPortalERPBindingRejectsUnknownTemplateKey` 修复前 `err=<nil>`，修复后返回 `ErrCapabilityTemplateERPWorkbenchUnavailable`，且 `repo.erpBindingCommand` 保持空值，证明不会写 active binding。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充无法识别模板在客户门户配置页也不能绑定 ERP 工作台账号。

## 客户门户配置未知模板保存 Fail-Closed
- 新增 `PR-221-CUSTOMER-PORTAL-VISIBILITY-UNKNOWN-TEMPLATE-INVALID`、`DEV-221-CUSTOMER-PORTAL-VISIBILITY-UNKNOWN-TEMPLATE-INVALID`，把客户门户配置保存能力模板也纳入未知模板 fail-closed。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：`UpdatePortalVisibility` 先调用 `NormalizeCapabilityTemplateKey`，非空未知模板 key 会被归一为空，随后按手工能力配置保存；这会让模板 typo 或历史脏 key 静默清掉模板关系。
- 修复：归一化前保留原始 `capability_template_key`，只有空 key 代表手工配置；非空但无法识别的模板 key 返回 `capability template invalid`，不调用仓储保存。
- RED/GREEN 证据：`TestUpdatePortalVisibilityRejectsUnknownTemplateKey` 修复前 `err=<nil>`，修复后返回 `capability template invalid` 且 `repo.visibilityCommand` 保持空值；`TestPortalAdminVisibilityTemplateInvalidMapsToBadRequest` 固定门户配置 API 对该错误返回 400。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充非空未知模板 key 不能静默保存为手工配置。

## 客户门户配置页未知模板显式提示
- 新增 `PR-222-CUSTOMER-PORTAL-FRONTEND-UNKNOWN-TEMPLATE-PRESERVE`、`DEV-222-CUSTOMER-PORTAL-FRONTEND-UNKNOWN-TEMPLATE-PRESERVE`，把未知模板 fail-closed 延伸到 Vue/Vite 客户门户配置页。
- 问题：后端 PR-221 已拒绝未知 key，但 `CustomerPortalSettingsView.vue` 加载和保存时仍会 `normalizeTemplateKey`，把非空未知 key 转为空；历史脏模板在页面上只显示“请选择模板”，并存在前端清空 key 的风险。
- 修复：页面保留原始 `capability_template_key`，未知 key 在下拉中显示“未知模板：key”，摘要区显示“未知能力模板”；保存按钮在未知 key 未处理前不可用，ERP 账号绑定也提示先重新选择系统模板；保存 payload 使用 `trimTemplateKey`，不再把未知 key 归一为空。
- RED/GREEN 证据：`customer portal settings preserves unknown template keys for correction` 修复前缺少 `unknownTemplateKey(row)` 且 payload 使用 `normalizeTemplateKey(row.form.capability_template_key)`，修复后该前端守卫通过。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充配置页显示未知能力模板时必须重新选择有效模板，不能静默清空。

## 客户门户配置 API 未知模板读路径保留
- 新增 `PR-223-CUSTOMER-PORTAL-ADMIN-UNKNOWN-TEMPLATE-READ-PRESERVE`、`DEV-223-CUSTOMER-PORTAL-ADMIN-UNKNOWN-TEMPLATE-READ-PRESERVE`，补齐 PR-222 的后端数据来源。
- 问题：`PortalAdminDetail` 和 `ListPortalAdminCustomers` 的 `normalizePortalAdminCustomer` 会把非空未知 `capability_template_key` 归一化为空；即使前端已保留未知 key，配置页也拿不到历史脏 key，只能显示“请选择模板”。
- 修复：客户门户配置读路径只归一化已知系统模板；非空未知模板 key 去首尾空格后原样返回，让配置页显示“未知能力模板”和原始 key，并阻止静默保存。
- RED/GREEN 证据：`TestPortalAdminCustomerResponsesPreserveUnknownTemplateKeyForCorrection` 修复前失败为列表 `capability_template_key=""`；`TestPortalAdminAPIPreservesUnknownTemplateKeyForCorrection` 修复前接口响应也为空，修复后应用层和 API 层均返回 `legacy_unknown_template`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户列表/详情 API 必须保留未知模板 key 供页面提示和排查。

## 客户类型切换停用历史 ERP 工作台绑定
- 新增 `PR-224-CUSTOMER-TYPE-RETAIL-DEACTIVATES-ERP-BINDING`、`DEV-224-CUSTOMER-TYPE-RETAIL-DEACTIVATES-ERP-BINDING`，把客户档案从批发切为零售/电商时的历史 ERP 工作台绑定纳入自动清理。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：客户从批发客户切换为零售或电商客户时，持久化层会自动应用 `retail_mall` 和商城能力，但历史 `customer_erp_user_bindings.status='active'` 仍可能残留；现有工作台入口已有模板兜底，但数据状态会误导运营，也增加未来绕过风险。
- 修复：`ensureDefaultRetailPortalTx` 在确认目标客户类型为 retail/ecommerce 且 active 后，同一事务先检查并停用该客户 active `customer_erp_user_bindings`，再继续写入 `retail_mall` profile 和商城能力；未部署客户履约表的旧环境中该步骤为 no-op。
- RED/GREEN 证据：真实 PostgreSQL `TestUpsertRetailCustomerDeactivatesLegacyERPWorkbenchBinding` 修复前失败为 `active ERP workbench bindings=1`；修复后客户 profile 为 `retail_mall` 且 active ERP 工作台绑定数为 0。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户从批发改为零售/电商后旧 ERP 工作台绑定自动停用。

## 客户 ERP 工作台绑定启用账号边界
- 新增 `PR-225-CUSTOMER-ERP-BINDING-LOGIN-ENABLED-GUARD`、`DEV-225-CUSTOMER-ERP-BINDING-LOGIN-ENABLED-GUARD`，把客户 ERP 工作台绑定纳入账号登录启用边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：用户权限页已经支持停用渠道客户账号登录，但客户门户配置和客户履约内部 ERP 绑定入口只检查员工启用和 `account_type='channel_customer'`；禁用登录的渠道账号仍可能被选中或通过 API 绑定，历史 active 绑定也会继续出现在客户工作台上下文和客户履约选择器中。
- 修复：公司账号基础 schema 统一初始化 `employee_login_passwords`；客户门户配置列表/详情、`UpsertPortalERPBinding`、客户履约 `UpsertCustomerERPBinding`、`CustomerPortalContext` 和 `ListCustomerERPBindings` 均要求 active 绑定对应账号为启用的渠道客户账号且 `login_disabled=false`；前端 `CustomerPortalSettingsView` 绑定下拉框隐藏 `login_disabled=true` 的渠道客户账号。
- RED/GREEN 证据：真实 PostgreSQL `TestUpsertPortalERPBindingRejectsDisabledLoginAccount`、`TestPortalAdminDetailHidesDisabledLoginERPBinding`、`TestUpsertCustomerERPBindingRejectsDisabledLoginAccount`、`TestCustomerPortalContextRejectsDisabledLoginBinding` 在修复前分别允许绑定或返回历史绑定；修复后返回 `login-enabled channel customer account required` 或 `ErrCustomerERPBindingNotFound`。前端 `customer portal settings excludes disabled channel accounts from ERP binding selector` 覆盖禁用账号不进入绑定下拉。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充禁用登录渠道账号不能绑定或继续作为有效 ERP 工作台绑定。

## 订单履约范围启用账号边界
- 新增 `PR-226-ORDER-FULFILLMENT-SCOPE-LOGIN-ENABLED-GUARD`、`DEV-226-ORDER-FULFILLMENT-SCOPE-LOGIN-ENABLED-GUARD`，把订单列表“履约客户订单”范围也纳入账号登录启用边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：PR-225 已覆盖客户工作台绑定入口、上下文和客户履约选择器，但 `scope=fulfillment` 的订单列表查询仍只要求 active ERP 绑定和工作台能力模板；如果历史 active 绑定对应渠道账号已停用登录，该客户的代发/代加工订单仍会出现在履约订单范围。
- 修复：`orderListWhere` 的 fulfillment scope 在 `customer_erp_user_bindings` 上联查 `company_employees` 与 `employee_login_passwords`，同时要求员工启用、`account_type='channel_customer'` 且 `COALESCE(login_disabled,false)=false`，再继续校验工作台模板和客户门户履约服务类型。
- RED/GREEN 证据：`TestOrderListWhereSupportsMineAndFulfillmentScopes` 修复前缺少 `company_employees` 和 `employee_login_passwords` 条件；真实 PostgreSQL `TestOrderAPIListFulfillmentScopeSkipsDisabledLoginBinding` 修复前响应包含 `SO-DISABLED-LOGIN-BINDING`，修复后只返回启用登录账号绑定的订单。
- 文档证据：`OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充“履约客户订单”范围必须过滤登录已停用的 ERP 绑定账号。

## 客户履约内部概览有效绑定边界
- 新增 `PR-227-CUSTOMER-FULFILLMENT-OVERVIEW-ACTIVE-BINDING-GUARD`、`DEV-227-CUSTOMER-FULFILLMENT-OVERVIEW-ACTIVE-BINDING-GUARD`，把客户履约账户内部概览载入也纳入有效 ERP 工作台绑定边界。按 Van 最新要求，进度录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留。
- 问题：客户履约客户选择器已经只展示有启用登录 ERP 工作台绑定且模板暴露工作台的批发客户，但页面仍支持 URL `customer_id` 回显；手动修改 URL 或直接请求 `/api/customer-fulfillment/:customer_id/overview` 时，`Overview` 原来只按 `customer_id` 聚合数据，没有确认该客户仍有有效 ERP 工作台绑定。
- 修复：新增 `requireActiveCustomerERPWorkbenchBinding`，内部概览载入前联查 `customer_erp_user_bindings`、`customers`、`company_employees`、`employee_login_passwords` 和 `customer_portal_profiles`，要求 active 绑定、客户启用、渠道账号启用登录、且能力模板暴露 ERP 工作台；不满足时返回 `ErrCustomerERPBindingNotFound`。
- RED/GREEN 证据：真实 PostgreSQL `TestInternalCustomerFulfillmentOverviewRequiresActiveERPBinding` 修复前 `Overview err=<nil>`，修复后未绑定但具备履约能力和工作台模板的客户按 `ErrCustomerERPBindingNotFound` 拒绝载入。
- 文档证据：`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充手动 URL/API 指向未绑定客户时不能载入客户履约概览。

## 现货下单商品可见范围
- 新增 `PR-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION`、`DEV-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION`、`UT-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION`、`API-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION`、`REV-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION`，把小程序现货下单商品列表和提交下单的客户专属商品边界作为独立数据安全验收项。
- 问题：`LoadServicePage(productOrder)` 原来调用 `listProducts(ctx, limit)`，SQL 只按 `active=true` 列出所有商品；`CreateFulfillmentOrder` 也只按 `id AND active=true` 读取商品。ERP 录单页已有客户专属商品过滤，但客户门户小程序会把其他客户 `visibility='customer_only'` 的商品显示给公共 SKU 小批量客户，并可通过手工商品 ID 创建订单。
- 修复：`portalProductVisibleToCustomerSQL` 统一定义当前客户商品可见范围：公共商品对现货下单客户可见，`customer_only` 或 `customer_id>0` 且空可见范围的专属商品只对归属客户可见。`listProducts` 使用当前 `customer_id` 过滤，`CreateFulfillmentOrder` 在插入 ERP 订单前再次校验同一范围；不可见商品返回 `product unavailable`，小程序 fulfillment order API 映射为 `400 invalid request`。
- RED/GREEN 证据：`TestLoadProductOrderServicePageFiltersCustomerOnlyProducts` 在真实 PostgreSQL 中创建公共商品、客户 A 专属商品和 `客户B不应显示专属深烘`，修复前服务页泄露客户 B 商品，修复后只返回公共商品和客户 A 专属商品。`TestCreateFulfillmentOrderRejectsAnotherCustomerOnlyProduct` 修复前能用客户 B 专属商品给客户 A 创建订单，修复后返回 `product unavailable` 且 `another customer product created order` 不会发生。
- API 证据：`TestMiniFulfillmentOrderProductUnavailableMapsToBadRequest` 固定小程序 fulfillment order API 对不可用商品返回 `400 {"error":"invalid request"}`，避免把业务拒绝误报成 500。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充现货下单商品可见范围只能包含公共商品和当前客户专属商品，不能显示其他客户专属商品。

## 商城商品公共目录范围
- 新增 `PR-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION`、`DEV-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION`、`UT-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION`、`API-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION`、`REV-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION`，把零售商城公共目录与客户专属产品隔离作为独立验收项。
- 问题：`LoadMallPage` 原来只按 `p.active=true` 和 `m.status='published'` 展示商城商品，`CreateMallOrder` 也只校验商城商品已发布和产品 active；商城管理的产品选项同样列出所有 active 产品，`SaveMallProduct` 不拦截客户专属商品。因此历史错误配置或手工保存都可能把批发客户专属商品暴露到零售商城并允许下单。
- 修复：`mallProductPublicCatalogSQL` 统一定义商城公共目录范围：只允许 `customer_id=0` 且 `visibility='public'` 的公共商品进入商城。`LoadMallPage` 隐藏不符合公共目录的历史商城商品，`CreateMallOrder` 在下单前再次校验并返回 `mall product unavailable`，`ListMallProducts` 的商城管理选品只返回公共商品，`SaveMallProduct` 拒绝保存客户专属商品。
- RED/GREEN 证据：`TestLoadMallPageFiltersCustomerOnlyMallProducts` 修复前小程序商城泄露 `商城客户专属不应展示`，修复后只显示公共商品；`TestCreateMallOrderRejectsCustomerOnlyMallProduct` 修复前能为客户专属商城商品创建订单，修复后返回 `mall product unavailable` 且 `customer-only mall product created order` 不会发生；`TestListMallProductsExcludesCustomerOnlyOptions` 和 `TestSaveMallProductRejectsCustomerOnlyProduct` 固定管理端选品和保存边界。
- API 证据：`TestMiniMallOrderUnavailableMapsToBadRequest` 和 `TestPortalAdminMallProductUnavailableMapsToBadRequest` 固定小程序商城下单与商城管理保存 API 对不可用商城商品返回 `400 {"error":"invalid request"}`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充商城商品公共目录范围只允许公共商品，不能上架客户专属商品。

## 代加工申请库存与目标产品范围
- 新增 `PR-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY`、`DEV-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY`、`UT-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY`、`API-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY`、`REV-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY`，把小程序代加工申请的投入库存和目标产品边界作为独立数据安全验收项。
- 问题：`CreateProcessingRequest` 原来在写入 `processing_job_requests` 前只做必填字段校验，不验证 `input_material_id` 是否属于当前客户托管库存、不验证可用克重是否足够，也不验证 `target_product_id` 是否是公共商品或当前客户专属商品。客户理论上可通过手工 ID 使用其他客户库存或其他客户专属目标产品生成代加工申请和生产需求。
- 修复：创建申请前先调用 `ensureProcessingInputInventoryTx` 聚合当前客户 `customer_inventory_items` 中可用 raw/material 库存，要求可用克重覆盖本次投入；再调用 `ensureProcessingTargetProductTx` 复用商品可见范围校验目标产品。生产需求插入也再次用目标产品可见范围兜底，若无有效目标产品则回滚并返回 `target product unavailable`。
- RED/GREEN 证据：`TestCreateProcessingRequestRejectsAnotherCustomerInventory` 修复前能用 `客户B托管生豆不应使用` 创建客户 A 申请，修复后返回 `input material unavailable` 且 `another customer processing request created` 不会发生；`TestCreateProcessingRequestRejectsInsufficientCustomerInventory` 覆盖当前客户库存不足；`TestCreateProcessingRequestRejectsAnotherCustomerTargetProduct` 覆盖不能使用其他客户专属目标产品。
- API 证据：`TestMiniProcessingRequestUnavailableInputsMapToBadRequest` 固定小程序 processing request API 对 `input material unavailable` 和 `target product unavailable` 返回 `400 {"error":"invalid request"}`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充代加工申请库存与目标产品范围只能使用当前客户托管库存和当前客户可见产品。

## 代发批次不能为空
- 新增 `PR-190-DIRECT-SHIP-BATCH-NONEMPTY`、`DEV-190-DIRECT-SHIP-BATCH-NONEMPTY`、`UT-190-DIRECT-SHIP-BATCH-NONEMPTY`、`API-190-DIRECT-SHIP-BATCH-NONEMPTY`、`REV-190-DIRECT-SHIP-BATCH-NONEMPTY`，把小程序代发批次必须包含实际订单行数作为独立操作合理性验收项。
- 问题：`CreateDirectShipBatch` 原来只拒绝负数 `total_rows`，允许 `total_rows=0` 写入 `direct_ship_import_batches` 并标记为 `submitted`，会让 ERP 工作台出现没有业务内容的待处理代发批次。
- 修复：应用服务和 PostgreSQL 仓储都改为 `cmd.TotalRows <= 0` 时返回 `total_rows invalid`，小程序服务页在提交前同步提示“订单行数必须大于 0”，避免空代发批次进入后端。
- RED/GREEN 证据：`TestCreateDirectShipBatchRejectsEmptyRows` 在应用服务层先复现空批次会进入仓储，修复后拒绝且不委托仓储；同名真实 PostgreSQL 测试先复现空批次会插入，修复后返回 `total_rows invalid` 且 `empty direct ship batch inserted` 不发生；`miniapp/src/utils/mall.test.ts` 固定小程序前端提交前校验。
- API 证据：`TestMiniDirectShipBatchEmptyRowsMapsToBadRequest` 固定小程序 direct-ship batch API 对 `total_rows invalid` 返回 `400 {"error":"invalid request"}`。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充代发批次不能为空、订单行数必须大于 0、不能提交空代发批次。

## 小程序服务表单选择器
- 新增 `PR-228-MINIAPP-SERVICE-FORMS-PICKER-UX`、`DEV-228-MINIAPP-SERVICE-FORMS-PICKER-UX`，按 Van 最新流程只录入 PR/DEV，测试与验收证据保留在 Superpower/TDD 输出和本文档中。
- 问题：客户在小程序“一件代发”“代加工”“现货下单”服务页提交订单或加工申请时，页面要求手填 `产品ID`、`生豆物料ID`、`目标产品ID`。这不符合客户侧操作习惯，也容易把安全兜底变成日常操作负担。
- 修复：后端 `LoadServicePage` 在 `directShip` 和 `processing` 服务页返回当前客户可见商品列表，沿用已有商品可见范围隔离；前端服务页把投入物料、目标产品和发货产品改为 picker 选择器，提交时仍传 ID，后端继续做能力、库存和商品归属校验。
- RED/GREEN 证据：真实 PostgreSQL `TestLoadDirectShipAndProcessingServicePagesReturnSelectableProducts` 先复现 directShip/processing 服务页缺可选商品，修复后返回公共商品和当前客户专属商品且不泄露其他客户专属商品；miniapp `uses customer-facing pickers instead of raw system ID fields for service order forms` 先复现页面缺 picker 且含 ID 输入框，修复后固定 `processingInputOptions`、`processingTargetProductOptions`、`fulfillmentProductOptions` 和三个 setter，并确保不再出现 `生豆物料ID`、`目标产品ID`、`产品ID` 输入框。
- 真实 HTTP/API 证据：重启本地后端到新代码后，`127.0.0.1:18094` 下 processing 客户的 `directShip` 和 `processing` 服务页均返回 `Nenka嫩咖 454g 公共SKU` 与 `客户专属加工成品`；切换到 public SKU 客户后 `productOrder` 只返回公共 SKU，不泄露 101 的专属商品。
- 构建证据：`VITE_KFERP_API_BASE=http://127.0.0.1:18094 npm run build:mp-weixin` 通过。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充客户服务表单使用选择器，不要求客户手填系统 ID。

## 小程序履约订单后端定价
- 新增 `PR-229-MINIAPP-FULFILLMENT-SERVER-PRICE-AUTHORITY`、`DEV-229-MINIAPP-FULFILLMENT-SERVER-PRICE-AUTHORITY`，按 Van 最新流程只录入 PR/DEV，测试与验收证据保留在 Superpower/TDD 输出和本文档中。
- 问题：客户小程序“一件代发”“代加工发货”“现货下单”提交履约订单时，API 请求体允许携带 `unit_price`，PostgreSQL 仓储只在单价为 0 时才使用后端商品默认价/小批量阶梯价。手工请求可把单价改为 `0.01`，导致 ERP 订单金额、后续财务收入和客户结算被客户端篡改。
- 修复：`/api/mini/fulfillment-orders` 不再向应用服务传递客户端 `unit_price`；应用服务在进入仓储前清空单价；PostgreSQL `CreateFulfillmentOrder` 无条件按当前客户、商品、规格、数量、默认价和小批量阶梯价计算单价。小程序服务页移除可编辑单价输入，payload 类型也不再包含 `unit_price`。
- RED/GREEN 证据：真实 PostgreSQL `TestCreateFulfillmentOrderIgnoresClientSuppliedUnitPrice` 先复现 `unit_price=0.01` 会写出 `0.01/0.02/0.02`，修复后写出后端价格 `88.00/176.00/176.00`；API 测试 `TestMiniFulfillmentOrderAPIIgnoresClientUnitPrice` 先复现路由把 `0.01` 交给服务层，修复后服务命令 `UnitPrice=0`；miniapp `keeps fulfillment order prices server-authoritative` 固定服务页无单价输入、无 `unit_price` payload。
- 文档证据：`OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md` 均补充小程序履约订单后端定价，客户侧不能手填或篡改单价。

## 微信开发者工具零售商城闭环
- 证据只补验收记录，不新增业务代码。测试使用本地真实 PostgreSQL `127.0.0.1:55552/kferp_wechat`、本地后端 `127.0.0.1:18094` 和当前分支微信构建产物 `miniapp/dist/build/mp-weixin`。
- 操作证据：通过 `/api/mini/current-customer` 把当前测试会话切到客户 `103`（`三模板-零售商城客户`，`miniapp_entry_mode=mall`），在微信开发者工具 GUI 中退回首页后自动进入 `pages/mall/mall`；页面显示客户服务台、`三模板-零售商城客户`、公开 SKU、购物车、收货信息和提交订单按钮。
- 点击闭环：GUI 点击“商城下单/商城商品”，点击“加入”，填写收件人 `Retail Test`、手机号 `13800138103`、地址 `Hangzhou Reail Road 103`，点击“提交订单”，开发者工具弹出成功 toast 并清空购物车。
- 数据库核对：`orders` 最新零售商城订单为 `SO-20260514-0006`，`customer_id=103`，`portal_service_code=mall`，`receiver_name=Retail Test`，`total_amount=127`，`grand_total=127`；`order_items` 为 `商城公开 SKU：Nenka嫩咖`，数量 `1`，单价 `127`，行金额 `127`。
- 证据标记：`WECHAT_GUI_RETAIL_MALL_CLICK_OK app=http://127.0.0.1:18094 pg=55552 project=miniapp/dist/build/mp-weixin path=pages/mall/mall template=retail_mall evidence=current_customer_103->mall_page->add_public_sku->submit_order SO-20260514-0006 total=127`。

## 微信开发者工具公共 SKU 闭环
- 证据只补验收记录，不新增业务代码。测试继续使用本地真实 PostgreSQL `127.0.0.1:55552/kferp_wechat`、本地后端 `127.0.0.1:18094` 和当前分支微信构建产物 `miniapp/dist/build/mp-weixin`。
- 操作证据：通过 `/api/mini/current-customer` 把当前测试会话切到客户 `102`（`三模板-公共SKU代发客户`），微信开发者工具 GUI 首页只显示“我的订单 / 现货下单 / 一件代发 / 结算中心”，没有“代加工”或“我的库存”入口。
- 点击闭环：GUI 点击“现货下单”，页面显示 `1` 个可见商品，仅有 `Nenka嫩咖 454g 公共SKU / 默认 ¥127.00`，表单字段为收件人、手机号、地址、公司、商品 picker、规格、件数、运费和备注，没有“单价”输入；选择公共 SKU 后提交订单，toast 显示“订单已提交”。
- 数据库核对：新订单 `SO-20260514-0007`，`customer_id=102`，`portal_service_code=product_order`，`receiver_name=Puic SKU GUI`，`receiver_phone=13800138102`，`receiver_address=Hangzhou Public SKU Road 102`，`total_amount=127`，`shipping_amount=0`，`grand_total=127`；`order_items` 为 `Nenka嫩咖 454g 公共SKU`，规格 `454g`，数量 `1`，单价 `127`，行金额 `127`。
- 证据标记：`WECHAT_GUI_PUBLIC_SKU_CLICK_OK app=http://127.0.0.1:18094 pg=55552 project=miniapp/dist/build/mp-weixin path=pages/home/home->pages/service/service template=public_sku_direct_ship evidence=current_customer_102->home_entries_orders_productOrder_directShip_settlement->product_order_public_sku_only->submit_order SO-20260514-0007 total=127 no_unit_price_input`。

## Prompt-to-artifact checklist
| 目标原文要求 | 必须看到的工件/证据 | 当前状态 |
| --- | --- | --- |
| “三种能力模板的客户” | `processing_fulfillment`、`public_sku_direct_ship`、`retail_mall` 在 `DefaultCapabilityTemplates`、`TestDefaultCapabilityTemplatesRuntimeBusinessContract`、`TestMiniAPITemplateBusinessContract`、`TestThreeTemplateBusinessWalkthroughAcrossModules` 中同时出现 | 已有应用层/API 层/跨模块证据。 |
| “账号、客户” | 客户档案/能力模板应用、ERP 账号绑定、零售商城不绑定工作台、零售商城模板不能保存 ERP 工作台字段、渠道账号隔离、小程序当前客户切换、停用客户旧绑定失效、批发切零售/电商时停用旧 ERP 工作台绑定、历史 ERP 工作台绑定不能绕过模板工作台边界、禁用登录渠道账号不能作为有效 ERP 工作台绑定；证据为 PR-177、PR-178、PR-181、PR-186、PR-191、PR-210、PR-216、PR-217、PR-218、PR-219、PR-220、PR-221、PR-222、PR-223、PR-224、PR-225、真实 Chrome 门户配置页和 `CUSTOMER_ACCOUNT_ISOLATION_SMOKE_OK` | 已有自动化、真实服务、小程序当前客户切换隔离、停用客户绑定边界、客户类型切换停用历史 ERP 绑定、禁用登录渠道账号绑定边界、历史 ERP 绑定上下文/订单范围/客户选择器边界、未知模板 fail-closed、配置保存未知模板 invalid、配置 API 未知模板读路径保留、前端未知模板显式提示和零售模板工作台不变式证据；微信 GUI 已登录 processing 模板真实会话。 |
| “订单” | 三模板下商城单、现货单、代发单、代加工单创建和查询；证据为小程序 API 矩阵、真实 HTTP smoke、Chrome 订单详情点击、微信开发者工具一件代发服务页点击和零售商城下单点击、PR-182 scope fail-closed、PR-184 小批量计价、PR-186 当前客户订单隔离、PR-187 现货下单商品可见范围隔离、PR-188 商城公共目录商品隔离、PR-190 代发批次非空、PR-195 销售单收款码公开资产上传防护、PR-201 收款码标签必填、PR-202 公章资产失败清理、PR-203 发票文件保存失败清理、PR-204 销售单生成文件失败清理、PR-205 出库单生成文件失败清理、PR-206 快递录单 Excel 生成失败清理、PR-196 物流回传 Excel 大小边界、PR-197 客户档案图片资产大小边界、PR-198 发票文件缺失订单边界、PR-199 商城图片缺失商品边界、PR-200 客户资产元数据失败清理、PR-217 履约客户订单范围工作台模板边界、PR-229 小程序履约订单后端定价 | 主干、安全、计价、当前客户切换订单隔离、现货商品可见范围、商城公共目录范围、代发空批次拒绝、履约客户订单范围模板边界、小程序履约订单不能篡改客户端单价、微信 GUI 服务页无单价输入且商品选择器只展示默认价，零售商城 GUI 已用公开 SKU 提交 `SO-20260514-0006` 并核对金额 127、销售单收款码上传防护、收款码标签必填先验、公章孤儿资产失败清理、发票文件保存失败清理、销售单生成文件失败清理、出库单生成文件失败清理、快递录单 Excel 生成失败清理、物流回传 Excel 超限拒绝、客户档案图片超限拒绝、发票文件孤儿资产拒绝、商城商品图片孤儿资产拒绝和客户档案资产孤儿文件清理已补。 |
| “订单履约范围登录启用边界” | PR-226、`TestOrderListWhereSupportsMineAndFulfillmentScopes`、真实 PostgreSQL `TestOrderAPIListFulfillmentScopeSkipsDisabledLoginBinding` | “履约客户订单”范围现在同时要求 active ERP 绑定指向启用登录的渠道客户账号；禁用登录账号绑定的代发/代加工订单不会进入该范围。 |
| “生产” | 三模板订单进入生产计划、生成计划、开始生产、完工；证据为 PR-177、真实 `/api/produce/unproduced`、Chrome CDP 生成计划/开始生产/完成、`running_done=1`、PR-189 代加工申请库存与目标产品范围、PR-208 多品项订单部分完工状态边界、PR-230 开始生产空选择/缺投料 fail-closed、PR-232 取消生产释放 WIP、PR-233 完工遇冻结 WIP 批次 fail-closed、PR-234 合并多规格生产单禁止部分完工、PR-236 完工产出不能大于消耗投料、PR-237 WIP 占用调整返回可用量排除冻结/拒收批次、PR-238 按 running item 释放孤立 WIP 占用实际落库、PR-239 WIP 列表排除停用/已耗尽批次、PR-243 陈旧计划/重复点击开始生产不能重复开工 | 主干通过；代加工申请进入生产需求前的库存/目标产品边界、部分完工不能提前整单生产完成、开始生产空选择/缺投料已补 Chrome 页面点击/强制请求 fail-closed 证据，陈旧计划重复开工 fail-closed、取消生产释放 WIP、完工遇质检冻结 WIP 批次和成品产出大于投料 fail-closed 均已追加 Chrome CDP 生产中页面点击证据和真实 PostgreSQL 核对，合并多规格生产单部分完工 fail-closed、WIP 调整返回可用量质量口径、running item 释放孤立 WIP 占用、WIP 列表 active/remaining 可用量口径均已补 Chrome CDP 仓库 WIP 抽屉点击证据；生产全量边缘矩阵未穷尽。 |
| “财务” | 收入、成本、费用、结算、经营报告、来源明细、月结、调整、销售单收款码；证据为 PR-177、真实 finance API、Chrome CDP 费用保存/经营报告/月结/调整、`monthly_status=adjusted`、PR-185 结算服务页客户隔离、PR-195 收款码上传只接受图片且超过 8MB 拒绝、PR-201 收款码标签必填、PR-207 历史订单收入回退、PR-209 重复结账保留已调整状态、PR-211 客户履约月结必须具备结算能力、PR-212 客户履约导入应用按类型校验客户能力、PR-231 票税台账强锁账边界、PR-235 未结账月份调整 API/UI 拒绝、PR-244 停用员工不能作为新费用经办人、PR-245 费用订单/客户/商品维度引用必须存在、PR-246 费用订单和客户维度必须一致、PR-247 费用订单和商品维度必须一致、PR-240 客户履约重复周期月结不清零已结费用、PR-241 客户履约空周期月结不生成 0 元批次、PR-242 客户履约非草稿月结批次不可重复改动 | 主干通过；结算服务页隔离、客户履约结算能力 gate、客户履约导入应用能力 gate、客户履约重复周期月结金额保留、空周期月结不写 0 元批次、非草稿月结批次不可被重复生成改动、收款码公开资产边界、标签必填先验、历史订单收入不漏计、已调整月份重复结账不降回已结账、票税台账强锁账边界已补 Chrome CDP 点击闭环和真实 PostgreSQL/API 证据，未结账月份调整已补月结页 Chrome CDP 点击和 API 拒绝证据，停用员工费用经办人、费用维度引用缺失、订单客户不一致、订单商品不一致均已补真实 PostgreSQL/API 和 Chrome CDP 费用管理页点击证据；财务全量边缘矩阵未穷尽。 |
| “财务费用员工状态边界” | PR-244、`TestCreateExpenseRejectsInactiveEmployee`、真实 PostgreSQL `TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense`、Chrome CDP `FINANCE_EXPENSE_INACTIVE_EMPLOYEE_UI_CLICK_OK`、`OP_MANUAL_FINANCE.md` | 新增费用带 `employee_id` 时服务端会校验员工仍为 active；费用管理页只展示在职员工，篡改页面提交停用员工会返回 `employee inactive` 且不写 `finance_expenses`，在职员工仍可正常写入。 |
| “财务费用维度引用边界” | PR-245、真实 PostgreSQL `TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense`、Chrome CDP `FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK`、`OP_MANUAL_FINANCE.md` | 新增费用填写 `order_id/customer_id/product_id` 时必须引用存在的订单、客户和商品；缺失维度返回 `finance dimension ... not found` 且不写 `finance_expenses`，费用管理页真实点击保存会显示同一错误，真实维度仍可写入。 |
| “财务费用订单客户一致性边界” | PR-246、真实 PostgreSQL `TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense`、Chrome CDP `FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK`、`OP_MANUAL_FINANCE.md` | 新增费用同时填写订单和客户时，客户必须与订单归属客户一致；不一致返回 `finance dimension customer does not match order` 且不写 `finance_expenses`，费用管理页真实点击保存会显示同一错误。 |
| “财务费用订单商品一致性边界” | PR-247、真实 PostgreSQL `TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense`、Chrome CDP `FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK`、`OP_MANUAL_FINANCE.md` | 新增费用同时填写订单和商品时，商品必须属于该订单明细；不一致返回 `finance dimension product does not match order` 且不写 `finance_expenses`，费用管理页真实点击保存会显示同一错误。 |
| “财务票税台账重复发票号边界” | PR-251、真实 PostgreSQL `TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger`、Chrome CDP `FINANCE_TAX_LEDGER_DUPLICATE_UI_CLICK_OK`、`OP_MANUAL_FINANCE.md` | 同类型非空发票号或凭证号跨月份重复提交返回 `tax ledger invoice already exists`，页面清楚呈现错误，且不写第二条 `finance_tax_ledger`。 |
| “小程序端” | miniapp 单测、类型检查、`build:mp-weixin`、真实小程序 HTTP smoke、微信开发者工具 GUI 登录/首页/一件代发服务页点击/公共 SKU 现货下单点击/零售商城下单点击、入口模式 PR-183、服务页保留 `miniapp_entry_mode`、PR-186 当前客户切换订单隔离、PR-210 停用客户不能作为当前客户、PR-190 代发批次行数前端校验、PR-192 客户专属豆单 PDF 下载边界、PR-193/PR-194/PR-199 商城商品图片上传类型、大小和缺失商品边界、PR-228 服务表单选择器、PR-229 履约订单后端定价、当前分支导入微信开发者工具 | API/构建/服务级会话隔离、停用客户绑定边界、空批次前端提示、豆单 PDF 租户边界、商城图片公开资产类型/大小/缺失商品边界、小程序表单不手填系统 ID 和小程序履约订单不暴露单价输入通过；WeChat DevTools Service Port 已开启，GUI 登录进入 `三模板-代加工履约客户` 首页并点击进入一件代发服务页，切换到 `三模板-公共SKU代发客户` 后完成现货下单 `SO-20260514-0007`，切换到 `三模板-零售商城客户` 后完成商城公开 SKU 下单 `SO-20260514-0006`。 |
| “工作台” | processing/public SKU 客户履约工作台差异、retail mall 无工作台绑定、自定义零售模板不能加入 ERP 工作台字段、服务端 capability 兜底；证据为 PR-178、PR-181、PR-189、PR-191、PR-211、PR-212、PR-213、PR-214、PR-215、PR-216、PR-217、PR-218、PR-219、PR-220、PR-221、PR-222、PR-223、PR-224、PR-225、Chrome CDP 工作台提交/隔离 | 主干通过；代加工申请输入边界、客户履约结算能力边界、导入应用能力边界、手工提交能力边界、托管库存调整能力边界、内部 ERP 绑定工作台边界、历史 ERP 绑定上下文/订单范围/客户选择器边界、客户类型切换停用历史 ERP 绑定、禁用登录渠道账号绑定边界、客户门户配置绑定 fail-closed、客户门户配置保存未知模板 invalid、配置 API 未知模板读路径保留、前端未知模板显式提示、未知模板 fail-closed 和零售模板工作台不变式已补；微信 GUI 已补 processing 模板服务页、public SKU 现货下单和 retail mall 商城下单真实点击。 |
| “操作逻辑更合理清晰、更简单直观” | 商城首页“我的订单”、retail mall 禁用工作台绑定提示、订单 scope fail-closed 错误提示、公共 SKU 计价排障手册、代发批次订单行数必须大于 0 的前端提示和手册、PR-228 小程序服务表单商品/库存选择器、PR-229 移除客户侧履约订单单价输入、销售单收款码上传格式/大小/标签必填失败说明、公章上传失败后重新上传公章说明、发票保存失败后重新上传发票说明、销售单 PDF/图片生成失败后重新生成销售单 PDF 或图片说明、出库单 PDF 生成失败后重新生成出库单 PDF 说明、快递录单 Excel 生成失败后重新生成快递录单 Excel 说明、物流回传 Excel 超限 `file too large` 说明、客户档案图片压缩到 8MB 内后再上传说明、客户档案附件失败后重新打开确认附件列表说明、发票上传缺失订单返回 `order not found` 说明、已调整月份重复结账仍显示已调整的手册说明、同一周期客户履约月结重复生成不会清零已结费用的手册说明、客户履约空周期月结返回 `no fees for settlement period` 的手册说明、非草稿月结返回 `settlement batch is not draft` 的手册说明、重复开始生产返回 `production already started` 的手册说明 | 已有产品操作优化和手册；客户侧小程序不再要求手填系统 ID，也不再暴露可误填/被篡改的单价字段。 |
| “可扩展性、可测试性、可维护性” | 模板契约测试、跨模块走查、支持守卫 PR-175 至 PR-271、PR/DEV 进度录入至 PR/DEV-271、真实 PostgreSQL 集成测试、acceptance 文档 | 已补，但不能替代未覆盖的 ERP 边缘矩阵。 |
| “数据更安全” | 模板越权 403、渠道账号隔离、服务端 capability gate、订单 scope fail-closed、履约客户订单范围模板 gate、履约客户订单范围登录启用 gate、履约客户选择器模板 gate、客户履约概览有效绑定 gate、未知模板工作台 fail-closed、客户门户配置绑定 fail-closed、ERP 绑定账号登录启用 gate、客户门户配置保存未知模板 invalid、客户门户配置 API 未知模板读路径保留、前端未知模板不静默清空、客户类型切换停用历史 ERP 绑定、零售工作台绑定禁用、零售模板 ERP 工作台字段保存拒绝、客户履约内部 ERP 绑定不能绕过模板工作台边界、历史 ERP 工作台绑定不能绕过模板工作台上下文边界、真实 PostgreSQL 迁移硬化、财务历史订单收入不漏计且作废订单不计入收入、财务结账后调整状态不被重复结账隐藏、客户履约月结必须具备结算能力、客户履约应用导入批次按类型校验客户能力、客户履约手工提交工单/代发按客户能力校验、客户履约手工调整托管库存按 `inventory_custody` 能力校验、结算服务页费用/结算单按客户隔离、小程序当前客户订单隔离、停用客户不能继续作为小程序当前客户或被旧绑定切换进入、现货下单商品客户专属可见范围隔离、小程序履约订单不信任客户端单价、客户专属豆单 PDF 只能由归属客户访问、商城商品图片上传只接受图片文件且超过 8MB 必须拒绝、商城商品图片缺失商品或更新失败不能留下公开孤儿资产、销售单收款码上传只接受图片文件且超过 8MB 必须拒绝且标签为空时不能写入收款码资产、销售单公章上传或去除背景失败时不能留下公开孤儿公章资产、销售单 PDF/PNG 生成失败时不能留下公开孤儿销售单文件、出库单 PDF 生成失败时不能留下公开孤儿出库单文件、快递录单 Excel 生成失败时不能留下公开孤儿快递录单文件、客户档案资产图片上传超过 8MB 时必须拒绝、客户档案资产元数据失败不能留下公开孤儿文件、发票文件上传必须先确认订单存在且不写入孤儿资产、发票文件保存失败时不能留下公开孤儿发票文件、物流单号 Excel 上传超过 20MB 时必须拒绝、零售商城公共目录公共商品边界、代加工申请当前客户库存/目标产品边界 | 已补关键边界、结算财务隔离、客户履约结算能力 gate、客户履约导入应用能力 gate、客户履约手工提交能力 gate、客户履约托管库存调整能力 gate、客户履约内部 ERP 绑定工作台 gate、客户履约内部概览有效绑定 gate、历史 ERP 绑定上下文/订单范围/客户选择器 gate、禁用登录渠道账号绑定 gate、履约客户订单范围登录启用 gate、未知模板工作台 fail-closed、客户门户配置绑定 fail-closed、客户门户配置保存未知模板 invalid、客户门户配置 API 未知模板读路径保留、前端未知模板不静默清空、客户类型切换停用历史 ERP 绑定、财务历史订单收入回退、结账后调整状态保留、当前客户订单隔离、停用客户绑定边界、现货商品可见范围隔离、小程序履约订单后端定价、客户专属豆单 PDF 租户边界、商城图片/销售单收款码/公章/销售单生成文件/出库单生成文件/快递录单 Excel 生成文件/发票/客户档案图片/发票文件资产边界、物流回传 Excel 超限拒绝、商城公共目录隔离、零售模板工作台不变式和代加工申请输入边界；真实小程序 GUI 已补 processing 服务页和 retail mall 商城下单主链路。 |
| “浏览器或者任何方法跑一遍” | Chrome DOM/CDP smoke、真实 HTTP/API smoke、真实 PostgreSQL、miniapp build、WeChat DevTools GUI 导入、一件代发服务页点击、公共 SKU 现货下单点击和零售商城下单点击 | 非微信 GUI 证据继续增强；订单抽屉已补库存待发货回填快递单号扣库存且重复点击幂等的 Chrome CDP 点击证据；微信开发者工具已能访问 `127.0.0.1:18094`，已完成登录、processing 服务页点击、public SKU 现货下单和 retail mall 公开 SKU 下单；ERP 边缘点击矩阵仍未完成。 |

## ERP 订单库存发货扣减边界补证
- 老 `PR-150` / `DEV-150-01` 已录入系统：库存待发货订单在回填快递单号时才正式扣减已预留的成品库存，重复回填必须幂等，且已发货历史订单不能误入生产计划。本次补强为既有 PR/DEV-150 的真实页面证据和 evidence guard，不新增需求行。
- API/真实 PostgreSQL 证据：`TestOrdersShippingTrackingAPIDeductsReservedLegacyFinishedInventoryOnce` 覆盖 LEGACY-FP 库存余额发货扣减一次、重复 POST 不二次扣减、`order_stock_deductions` 和 `stock_ledger_entries` 各 1 条；`TestOrdersSingleShippingTrackingAPIDeductsReservedFinishedBatch` 覆盖 FP 批次扣减；`TestOrdersShippingTrackingAPIDeductsOrderSourceWarehouseWithoutAllocation` 覆盖客户源仓库存扣减；`TestProducePlanExcludesShippedOrdersWithBlankProcessStatus` 覆盖已发货旧单不进入生产计划。
- UI 点击证据：一次性 PostgreSQL `kferp_pr150_ui` + 本地后端 `127.0.0.1:18142` + Chrome CDP 打开 `/vue-shell?view=orders&q=PR150`，点击 `SO-PR150-STOCK-UI` 进入订单抽屉，在“快递单号（可多个）”输入 `SF-PR150-UI-001` 并两次点击“回填”；页面显示 `已回填 SF-PR150-UI-001` 和 `已发货`。真实 PostgreSQL 核对 `finished_inventory=2_units_0_loose_g`、`order_stock_deductions=1`、`stock_ledger_entries=1`、`order_shipping_trackings=1`，即 2 件订单只扣一次、重复点击不重复扣库存或物流子表。证据标记：`ORDER_STOCK_SHIPMENT_DEDUCTION_UI_CLICK_OK app=http://127.0.0.1:18142 pg=55612 evidence=drawer_tracking_click_twice db=inventory_2_units_deductions_1_ledger_1_tracking_1 order=SF-PR150-UI-001|已发货`。
- 操作手册：`OP_MANUAL_ORDER_SALES.md` 与 `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md` 已说明库存待发货订单生成 Excel 时不扣库存，回填快递单号并标记已发货时才扣减已预留 `FP-...` 批次或 `LEGACY-FP-...` 库存余额并写库存流水。

## ERP 生产开始边界补证
- `PR-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD` / `DEV-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD` 已录入系统：生产计划开始生产必须拒绝空选择或无正数投料，失败时不能打开生产中记录、生产工单或 WIP 占用。
- 服务层证据：`go test ./internal/application/production -run 'Test(StartRejects|ServiceOwnsRunningProductionUseCases)' -count=1 -v` 通过，覆盖 `TestStartRejectsEmptySelectionWithoutOpeningWork`、`TestStartRejectsSelectedNeedWithoutPositiveInput`。
- API/真实 PostgreSQL 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55554/kferp_prod_edge?sslmode=disable' go test ./internal/interfaces/http/production -run 'TestProduceStartAPIRejects(EmptySelection|MissingInput)|TestProduceStartAPIUsesSubmittedInputG' -count=1 -v` 通过，覆盖空选择 400、缺投料 400，并断言 `produce_running_items`、`work_orders`、`work_order_material_reservations` 均未新增。
- UI 点击证据：`PRODUCTION_START_EMPTY_INPUT_UI_CLICK_OK app=http://127.0.0.1:18141 pg=55611 evidence=empty_selection_alert_then_forced_api_and_missing_input_tamper empty_response=400 missing_input_response=400 db=running_0_work_orders_0_reservations_0_order_status_pending`；Chrome 页面打开 `/vue-shell?view=producePlan` 后“开始生产”在未生成计划时为 disabled，点击“生成计划”弹出 `请先选择产品后再生成计划`；从浏览器上下文强制空 selection 请求返回 400 `请先生成计划并选择项目`；随后勾选 `SO-START-EMPTY-MISSING` 生成计划，点击可见“开始生产”时把请求投料篡改为 0，页面显示 `投料数必须大于0`，真实 PostgreSQL 核对 `produce_running_items`、`work_orders`、`work_order_material_reservations` 均为 0，订单仍为 `待处理`。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充开始生产失败时不生成生产中记录、生产工单或 WIP 占用的操作说明和校验点。

## ERP 财务锁账边界补证
- `PR-231-FINANCE-TAX-LEDGER-CLOSE-LOCK-GUARD` / `DEV-231-FINANCE-TAX-LEDGER-CLOSE-LOCK-GUARD` 已录入系统：强锁账月份不能继续新增同月票税台账，票税台账作为财务来源单据必须和费用来源单据遵循同一结账锁定规则。
- RED 证据：`go test ./internal/application/finance -run TestCreateTaxLedgerRespectsStrongLockForClosedMonth -count=1 -v` 曾失败，失败点为 closed strong-lock 月份仍允许新增票税台账。
- GREEN 证据：`go test ./internal/application/finance -run 'TestCreate(TaxLedger|Expense)RespectsStrongLockForClosedMonth|TestFinanceClosingReviewDrilldownTaxLedgerAndHandoff' -count=1 -v` 通过，覆盖 strong-lock closed 拒绝票税台账，light-confirmation 允许票税台账，且原票税台账/会计交接链路仍可用。
- API 证据：`go test ./internal/interfaces/http/finance -run 'TestFinanceTaxLedgerAPIReturnsBadRequestWhenServiceRejectsClosedMonth|TestFinanceImprovementAPIs' -count=1 -v` 通过，覆盖服务拒绝 closed month 票税台账时 `/api/finance/tax-ledger` 返回 400。
- UI 点击证据：一次性 PostgreSQL `kferp_pr231_ui` + 本地后端 `127.0.0.1:18132` + Chrome CDP 打开 `/vue-shell?view=financeClosing` 点击 `结账`，页面显示 `已结账`；随后打开 `/vue-shell?view=financeTaxLedger` 录入 `LOCK-UI-001` 并点击 `保存`，页面显示 `month is closed by strong lock`。数据库核对 `finance_monthly_reports.status='closed'`，且 `finance_tax_ledger` 中 `LOCK-UI-001` 为 0 条。输出 `FINANCE_TAX_LEDGER_CLOSE_LOCK_UI_CLICK_OK app=http://127.0.0.1:18132 pg=55602 evidence=close_2026_05_then_tax_ledger_save error=month is closed by strong lock`。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已说明强锁账月份同月费用和票税台账都不能继续新增。

## ERP 财务票税台账重复发票号边界补证
- `PR-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD` / `DEV-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD` 已录入系统：票税台账同类型非空发票号或凭证号不能重复写入，避免同一发票跨月份或重复点击被双计入票税来源。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55574/kferp_finance_tax_dup?sslmode=disable' go test ./internal/interfaces/http/finance -run TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger -count=1 -v` 曾失败，失败点为第二次提交 `PINV-DUP-001` 返回 200 并写入第 2 条 `finance_tax_ledger`。
- GREEN 证据：同一命令通过，覆盖 `/api/finance/tax-ledger` 对同类型非空重复发票号返回 400 `tax ledger invoice already exists`，且 `finance_tax_ledger` 只保留 1 条。
- UI 点击证据：一次性 PostgreSQL `kferp_pr251_ui` + 本地后端 `127.0.0.1:18131` + Chrome CDP 打开 `/vue-shell?view=financeTaxLedger`，先保存 `2026-05/PINV-UI-DUP-001`，再切到 `2026-06` 点击保存同类型同发票号；页面显示 `tax ledger invoice already exists`，数据库核对 `finance_tax_ledger` 中该发票号仍只有 1 条。输出 `FINANCE_TAX_LEDGER_DUPLICATE_UI_CLICK_OK app=http://127.0.0.1:18131 pg=55601 evidence=save_2026_05_then_duplicate_2026_06 error=tax ledger invoice already exists`。
- 实现证据：`FinanceRepository.CreateTaxLedgerEntry` 在事务内对 `kind + invoice_no` 加 `pg_advisory_xact_lock`，再按大小写不敏感发票号查重；空发票号仍用于税款缴纳、其他或未取票说明，不触发重复校验。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已说明重复发票号拒绝写入和排障方式。

## 客户履约代发跨批次重传幂等补证
- `PR-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY` / `DEV-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY` 已录入系统：同一外部代发订单即使通过不同 Excel 批次重传，也不能重复写代发明细或 ERP 订单明细。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems -count=1 -v` 通过，覆盖第二个 Excel 批次重传同一外部订单后仍只有 1 张代发订单、2 条代发明细和 2 条 ERP `order_items`。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_REIMPORT_IDEMPOTENCY_UI_CLICK_OK app=http://127.0.0.1:18152 pg=55622 evidence=apply_latest_direct_ship_batch_pr248 db=import_orders_1_import_items_2_erp_orders_1_erp_items_2_batch248102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=24801`，切到“代发清单”并点击应用第二批 `pr248-direct-ship-reupload.xlsx`，刷新后批次表显示 248102 已应用；页面代发订单显示 `YGS248-UI-001 / 2026-03-04 / 李四 13900000000 浙江宁波鄞州区 / 待发货 / 2`，真实 PostgreSQL 核对同一外部订单仍只有 1 条导入订单、2 条代发明细、1 张 ERP 订单、2 条 ERP `order_items`。
- 实现证据：`ApplyImport` 处理 `direct_ship_item` 时按同一导入订单和 `line_no` 更新 `customer_direct_ship_import_order_items` 与 ERP `order_items`，并清理同 `line_no` 历史重复行，避免跨批次重传追加重复明细。

## 客户履约托管流水跨批次重传幂等补证
- `PR-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY` / `DEV-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY` 已录入系统：客户履约代加工物料出入库流水跨 Excel 批次重传时，托管库存余额不能因同一外部流水重复加减。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance -count=1 -v` 通过，覆盖第二个 Excel 批次重传同一外部生豆入库流水后库存台账仍只有 1 条、托管余额仍为 1500g。
- UI 点击证据：`CUSTOMER_PROCESSING_CUSTODY_REIMPORT_IDEMPOTENCY_UI_CLICK_OK app=http://127.0.0.1:18153 pg=55623 evidence=apply_latest_processing_workbook_pr249 db=ledger_1_sum_1500_balance_1500_batch249102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=24901`，点击应用第二批 `pr249-custody-reupload.xlsx`，刷新后批次表显示 249101/249102 均已应用；页面托管库存显示 `生豆 / 埃塞花魁 / 1500g`，真实 PostgreSQL 核对同一 `raw_bean_receipt:IN-REIMPORT-UI:埃塞花魁` 台账仍 1 条、delta 汇总 1500g、余额 1500g。
- 实现证据：`upsertCustodyMovementLedgerTx` 按同客户、来源类型和外部流水 key 复用已应用台账，只有新增台账时才调用 `addCustodyBalanceTx` 调整托管余额。

## 客户履约结算费用跨批次重传幂等补证
- `PR-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY` / `DEV-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY` 已录入系统：客户履约结算单费用跨 Excel 批次重传时，同一外部费用行不能重复生成费用明细或抬高结算金额。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplySettlementImportReimportSameFeeDoesNotDuplicateFeeItems -count=1 -v` 通过，覆盖第二个 Excel 批次重传同一外部结算费用行后费用明细仍只有 1 条、总额仍为 8000 分。
- UI 点击证据：`CUSTOMER_SETTLEMENT_FEE_REIMPORT_IDEMPOTENCY_UI_CLICK_OK app=http://127.0.0.1:18153 pg=55623 evidence=apply_latest_settlement_workbook_pr250 db=fee_items_1_sum_8000_batch250102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25001`，切到“结算单”并点击应用第二批 `pr250-settlement-reupload.xlsx`，刷新后批次表显示 250101/250102 均已应用；页面费用明细显示 `processing / 烘焙费 / 80.00 / customer_fulfillment_import`，真实 PostgreSQL 核对同一费用 note 仍 1 条、金额汇总 8000 分。
- 实现证据：`appliedSettlementFeeItemIDByExternalKeyTx` 按同客户相同 `fee_item` external key 查找已应用导入行 target fee item，新批次行复用原 `customer_fee_items` 记录。

## 客户履约代发修正版少行重传边界补证
- `PR-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM` / `DEV-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM` 已录入系统：同一外部代发订单用少行修正版 Excel 重传时，已删除旧商品行不能继续留在代发明细或 ERP 订单明细。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55575/kferp_customer_direct_ship_trim?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems -count=1 -v` 曾失败，失败点为修正版只剩 1 行后 `customer_direct_ship_import_order_items count = 2, want 1`。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyDirectShipImportReimport(ShorterOrderRemovesStaleItems|SameExternalOrderDoesNotDuplicateItems)' -count=1 -v` 通过，覆盖少行重传删除旧行、同长度重传继续不重复追加。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_REIMPORT_STALE_LINE_TRIM_UI_CLICK_OK app=http://127.0.0.1:18152 pg=55622 evidence=apply_latest_direct_ship_batch_pr252 db=import_items_1_erp_items_1_qty_3_batch252102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25201`，切到“代发清单”并点击应用当前少行修正版 `pr252-direct-ship-shorter.xlsx`，刷新后批次表显示 252102 已应用；页面代发订单显示 `YGS252-UI-001 / 2026-03-04 / 王五 13700000000 浙江杭州滨江区 / 待发货 / 1`，真实 PostgreSQL 核对代发导入明细 1 条、ERP `order_items` 1 条、保留行数量 3。
- 实现证据：`ApplyImport` 在 `direct_ship_workbook` 应用完本批次有效行后调用 `trimDirectShipStaleLinesTx`，只对本批次出现过 `direct_ship_item` 的外部订单按最新 `line_no` 数裁剪 `customer_direct_ship_import_order_items` 和 ERP `order_items` 多余旧行。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明少行修正版以最新 Excel 行数为准。

## 客户履约代发订单头重传更正边界补证
- `PR-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION` / `DEV-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION` 已录入系统：同一外部代发订单跨 Excel 批次重传并更正订单日期、收件信息或备注时，ERP 代发订单头必须显示最新 Excel。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55582/kferp_direct_ship_header_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot -count=1 -v` 曾失败，失败点为重传后 ERP 代发订单日期仍是 `2026-03-04`，期望 `2026-03-06`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot|TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems|TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems' -count=1 -v` 通过，覆盖同一外部订单重传后 ERP `orders.order_date`、收件人、电话、地址和备注都刷新为最新 Excel，且明细幂等和少行裁剪回归通过。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_HEADER_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18147 pg=55617 evidence=apply_latest_direct_ship_batch_pr259 db=date_2026_03_06_receiver_zhou2_phone_13300000000_note_corrected_batch259102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25901`，切到“代发清单”并点击应用当前修正版 `pr259-header-corrected.xlsx`，刷新后该批次显示 `已应用`；页面代发订单显示 `YGS-HEADER-UI-001 / 2026-03-06 / 周二 13300000000 浙江杭州滨江区 / 待发货`，真实 PostgreSQL 核对收件人、电话、地址、订单日期和备注均为最新 Excel。
- 实现证据：`applyDirectShipOrderTx` 对已关联 ERP 订单的 `direct_ship_order` 更新路径新增 `order_date` 和 `notes` 刷新；原有 receiver 快照、运单、仓库和 `portal_service_code='direct_ship'` 更新保持不变。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明代发订单头修正要保持外部订单号和序号不变，系统会更新原 ERP 订单头快照。

## 客户履约代发发货状态重传更正边界补证
- `PR-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION` / `DEV-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION` 已录入系统：同一外部代发订单跨 Excel 批次重传并更正发货状态时，ERP 代发订单发货状态必须显示最新 Excel。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55583/kferp_direct_ship_status_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus -count=1 -v` 曾失败，失败点为重传后 ERP 代发订单发货状态为空，期望 `已发货`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus -count=1 -v` 通过，覆盖同一外部订单发货状态从 `待发货` 修正为 `已发货` 后，ERP `orders.ship_status_id` 指向最新状态。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_STATUS_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18147 pg=55617 evidence=apply_latest_direct_ship_batch_pr260 db=ship_status_delivered_batch260102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26001`，切到“代发清单”并点击应用当前修正版 `pr260-status-corrected.xlsx`，刷新后该批次显示 `已应用`；页面代发订单显示 `YGS-STATUS-UI-001 / 已发货`，真实 PostgreSQL 核对 ERP 订单发货状态为 `已发货`。
- 实现证据：`applyDirectShipOrderTx` 新增 `directShipImportShipStatusID` 映射，创建 ERP 代发订单时写入 `ship_status_id`，已关联订单重传时用已识别状态刷新 `ship_status_id`；空/未发货走默认未发货，待发货优先匹配待发货，已发货匹配已发货，未知状态不覆盖已有 ERP 状态。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明代发发货状态修正要保持外部订单号和序号不变，系统会更新原 ERP 订单发货状态。

## 客户履约代加工生产工单拼配投料重传边界补证
- `PR-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION` / `DEV-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION` 已录入系统：同一外部代加工生产工单多行生豆投料或重传少投料时，工单投料明细必须以最新 Excel 投料集合为准。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55584/kferp_processing_work_order_inputs?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet -count=1 -v` 曾失败，失败点为同一工单两行投料导入后只剩 1 行、总投料 `400g`，期望 2 行、总投料 `1000g`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet -count=1 -v` 通过，覆盖同一工单两种生豆拼配投料都保留，工单头投豆量汇总为 `1000g`，重传只剩一种生豆时旧投料被裁剪且工单头投豆量更新为 `700g`。
- UI 点击证据：`CUSTOMER_PROCESSING_WORK_ORDER_INPUT_SET_UI_CLICK_OK app=http://127.0.0.1:18148 pg=55618 evidence=apply_latest_processing_workbook_pr261 db=inputs_1_total_700_header_700_bean_埃塞花魁_batch261102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26101`，页面显示 `pr261-work-order-inputs-corrected.xlsx / 代加工工单` 当前可应用批次，点击“应用当前类型最新批次”后刷新为 `已应用`；页面加工工单显示 `WO-BLEND-UI-001 / 誉观山拼配227g / 投豆 700 / 产量 4`，真实 PostgreSQL 核对旧 `哥伦比亚慧兰` 投料被裁剪，只剩 `埃塞花魁:700`，工单头投豆量为 700g。
- 实现证据：`ApplyImport` 为 processing workbook 维护 `processingApplyState`；`applyProcessingWorkOrderTx` 按 `work_order_id + raw_bean_name` 更新或插入投料行，不再逐行删除全部投料；批次结束后 `trimProcessingWorkOrderStaleInputsTx` 只删除本次最新 Excel 未出现的旧投料，并用 `refreshProcessingWorkOrderInputTotalTx` 把工单头投豆量刷新为投料合计。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明代加工生产工单可以有多行不同生豆投料，修正版 Excel 以最新投料集合为准。

## 客户履约库存转换工单单号重传更正边界补证
- `PR-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION` / `DEV-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION` 已录入系统：同一库存转换单号跨 Excel 批次重传并更正转换前产品、转换后产品或数量时，必须更新原转换工单。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55585/kferp_conversion_job_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob -count=1 -v` 曾失败，失败点为同一转换单号重传后出现 2 条转换工单。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob -count=1 -v` 通过，覆盖 `CV-CORRECT-001` 从旧转换前后产品和 2 件数量修正为新转换前后产品和 3 件后，只保留 1 条最新转换工单。
- UI 点击证据：`CUSTOMER_CONVERSION_JOB_REIMPORT_JOB_NO_UI_CLICK_OK app=http://127.0.0.1:18148 pg=55618 evidence=apply_latest_processing_workbook_pr262 db=jobs_1_from_誉观山花魁454g_to_誉观山挂耳新品_qty_3_batch262102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26201`，页面显示 `pr262-conversion-corrected.xlsx / 代加工工单` 批次，点击“应用当前类型最新批次”后导入批次表刷新为 `已应用`；真实 PostgreSQL 核对同一 `CV-UI-CORRECT-001` 只保留 1 条转换工单，转换前产品为 `誉观山花魁454g`、转换后产品为 `誉观山挂耳新品`、数量为 3。
- 实现证据：`applyConversionJobTx` 在 `job_no` 非空时优先调用 `updateConversionJobByJobNoTx`，按 `customer_id + job_no` 锁定原转换工单并刷新 `external_key`、转换前后产品、数量和 payload，同时清理同单号重复行；无 `job_no` 时保留原 external_key upsert 兜底。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明转换单号是库存转换工单修正重传的稳定身份。

## 客户履约包装子工单工单编号重传更正边界补证
- `PR-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION` / `DEV-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION` 已录入系统：同一包装子工单编号跨 Excel 批次重传并更正产品、包装耗材或数量时，必须更新原包装子工单。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55586/kferp_packaging_job_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob -count=1 -v` 曾失败，失败点为同一包装子工单编号重传后出现 2 条包装子工单。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob -count=1 -v` 通过，覆盖 `PK-CORRECT-001` 从旧产品、旧挂耳袋和 20 件修正为新产品、新挂耳袋和 30 件后，只保留 1 条最新包装子工单。
- UI 点击证据：`CUSTOMER_PACKAGING_JOB_REIMPORT_WORK_ORDER_UI_CLICK_OK app=http://127.0.0.1:18149 pg=55619 evidence=apply_latest_processing_workbook_pr263 db=jobs_1_product_誉观山挂耳新品_packaging_新挂耳袋_qty_30_batch263102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26301`，页面显示 `pr263-packaging-corrected.xlsx / 代加工工单` 当前可应用批次，点击“应用当前类型最新批次”后导入批次表刷新为 `已应用`；真实 PostgreSQL 核对同一 `PK-UI-CORRECT-001` 只保留 1 条包装子工单，产品为 `誉观山挂耳新品`、耗材为 `新挂耳袋`、数量为 30。
- 实现证据：`applyPackagingJobTx` 在 `work_order_no` 非空时优先调用 `updatePackagingJobByWorkOrderNoTx`，按 `customer_id + work_order_no` 锁定原包装子工单并刷新 `external_key`、产品、包装耗材、数量和 payload，同时清理同工单编号重复行；无 `work_order_no` 时保留原 external_key upsert 兜底。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明包装子工单编号是包装子工单修正重传的稳定身份。

## 客户履约结算费用名称重传更正边界补证
- `PR-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION` / `DEV-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION` 已录入系统：同一结算费用行跨 Excel 批次重传并更正费用名称时，必须更新原未结费用明细。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55587/kferp_settlement_fee_name_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem -count=1 -v` 曾失败，失败点为同一费用行更名后出现 2 条费用、总额从最新 9500 分变成 17500 分。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem -count=1 -v` 通过，覆盖同一 `storage` 费用行第 3 行从“仓储费旧名称/8000 分”修正为“仓储费新名称/9500 分”后，只保留 1 条最新费用。
- UI 点击证据：`CUSTOMER_SETTLEMENT_FEE_NAME_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18149 pg=55619 evidence=apply_latest_settlement_workbook_pr264 db=fee_1_total_9500_note_仓储费新名称_batch264102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26401`，切到“结算单”并点击应用当前修正版 `pr264-fee-name-corrected.xlsx`，刷新后批次显示 `已应用`；页面费用明细显示 `storage / 仓储费新名称 / 95.00 / customer_fulfillment_import`，真实 PostgreSQL 核对导入来源费用仍只有 1 条、金额 9500 分、note 为最新费用名称。
- 实现证据：`applySettlementImportRow` 先按完整 `external_key` 复用费用；完整键因费用名称变化未命中时，调用 `appliedSettlementFeeItemIDByFeeLineTx` 按同一结算表 `sheet_name + row_no` 查找原 applied 行，再刷新未结且未绑定月结批次的 `customer_fee_items`。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明费用名称更正导致完整外部键变化时，系统按同一结算表和 Excel 行号复用原费用明细。

## 客户履约结算费用类型重传更正边界补证
- `PR-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION` / `DEV-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION` 已录入系统：同一结算费用行跨 Excel 批次重传并更正费用类型时，必须更新原未结费用明细。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55588/kferp_settlement_fee_type_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplySettlementImportReimportCorrectedFeeTypeUpdatesExistingFeeItem -count=1 -v` 曾失败，失败点为同一费用行类型从仓储费改物流费后出现 2 条费用、总额从最新 9500 分变成 17500 分。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplySettlementImportReimportCorrectedFee(Name|Type)UpdatesExistingFeeItem' -count=1 -v` 通过，覆盖同一结算表第 3 行更正费用名称或费用类型时都只保留 1 条最新未结费用。
- UI 点击证据：`CUSTOMER_SETTLEMENT_FEE_TYPE_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18150 pg=55620 evidence=apply_latest_settlement_workbook_pr265 db=fee_1_total_9500_type_shipping_note_物流费_batch265102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26501`，切到“结算单”并点击应用当前修正版 `pr265-fee-type-corrected.xlsx`，刷新后批次显示 `已应用`；页面费用明细显示 `shipping / 物流费 / 95.00 / customer_fulfillment_import`，真实 PostgreSQL 核对导入来源费用仍只有 1 条、金额 9500 分、`fee_type=shipping`、note 为最新费用名称。
- 实现证据：`loadValidImportRowsTx` 加载导入行 `sheet_name` 和 `row_no`；`applySettlementImportRow` 完整 external_key 未命中时调用 `appliedSettlementFeeItemIDByFeeLineTx`，按 `customer_id + sheet_name + row_no` 复用原 applied fee item，再刷新 `fee_type`、金额、日期和名称。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明费用类型或费用名称更正导致完整外部键变化时，系统按同一结算表和 Excel 行号复用原费用明细。

## 客户履约代加工托管流水生豆名称重传更正边界补证
- `PR-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION` / `DEV-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION` 已录入系统：同一生豆入库或出库流水跨 Excel 批次重传并更正生豆名称时，旧生豆余额必须回退，新生豆余额和台账必须以最新 Excel 为准。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55589/kferp_custody_name_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance -count=1 -v` 曾失败，失败点为同一入库/出库行从 `埃塞花魁` 更正为 `肯尼亚AA` 后，旧生豆余额仍为 `1500g`、新生豆余额 `1200g`，库存台账变成 4 条、汇总 `2700g`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyProcessingImportReimportCorrectedCustodyMovement(RawBeanNameMovesLedgerAndBalance|AdjustsBalanceDelta)|TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance' -count=1 -v` 通过，覆盖生豆名称更正、数量更正和同外部流水幂等三个重传边界。
- UI 点击证据：`CUSTOMER_PROCESSING_CUSTODY_RAW_BEAN_NAME_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18150 pg=55620 evidence=apply_latest_processing_workbook_pr266 db=old_0_new_1200_ledger_2_sum_1200_names_肯尼亚AA_batch266102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26601`，点击应用代加工工单修正版 `pr266-custody-bean-corrected.xlsx`，刷新后批次显示 `已应用`；页面托管库存显示旧生豆 `埃塞花魁` 余额 0、新生豆 `肯尼亚AA` 余额 1200g，真实 PostgreSQL 核对旧余额 0、新余额 1200、相关库存台账 2 条汇总 1200 且都指向 `肯尼亚AA`。
- 实现证据：`upsertCustodyMovementLedgerTx` 完整 external_key 未命中时，按 `customer_id + row_type + sheet_name + row_no` 找到最近 applied 导入行并 `FOR UPDATE` 锁定原库存台账，把 ledger 移到最新生豆 item；若 item 改变，则对旧 item 写负向余额调整，对新 item 写最新净增减量。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明同一 Excel 行号更正生豆名称时，旧生豆余额回退，新生豆余额和台账汇总以最新 Excel 为准。

## 客户履约库存余额物料名称重传更正边界补证
- `PR-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION` / `DEV-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION` 已录入系统：同一生豆库存余额或耗材库存余额跨 Excel 批次重传并更正物料名称时，旧物料余额必须回退，新物料余额和盘点调整台账必须以最新 Excel 为准。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55590/kferp_balance_item_name_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportCorrectedCustodyBalanceItemNameMovesLedgerAndBalance -count=1 -v` 曾失败，失败点为生豆余额从 `埃塞花魁` 更正为 `肯尼亚AA` 后旧余额仍为 `1000g`，新余额 `1200g`，台账变成 2 条、汇总 `2200g`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyProcessingImportReimportCorrectedCustodyBalance(ItemNameMovesLedgerAndBalance|UpdatesLedgerDelta)|TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance' -count=1 -v` 通过，覆盖余额物料名称更正、余额数量更正和出入库生豆名称更正回归。
- UI 点击证据：`CUSTOMER_CUSTODY_BALANCE_ITEM_NAME_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18151 pg=55621 evidence=apply_latest_processing_workbook_pr267 db=raw_old_0_new_1200_ledger_1_sum_1200_names_肯尼亚AA_pack_old_0_new_80_ledger_1_sum_80_names_挂耳袋_batch267102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26701`，点击应用当前“代加工工单”修正版 `pr267-balance-name-corrected.xlsx`，刷新后该批次显示 `已应用`；页面托管库存显示旧物料 `埃塞花魁 / 227g袋` 余额均为 0，新物料 `肯尼亚AA 1200g`、`挂耳袋 80件`，真实 PostgreSQL 核对生豆余额/台账 `0/1200/1/1200/肯尼亚AA`，耗材余额/台账 `0/80/1/80/挂耳袋`。
- 实现证据：`upsertCustodyBalanceAdjustmentLedgerTx` 完整 external_key 未命中时，按 `customer_id + row_type + sheet_name + row_no` 找到最近 applied 的 `customer_custody_balance` 导入行并 `FOR UPDATE` 锁定原盘点调整台账，把 ledger 移到最新物料 item；若 item 改变，则对旧物料余额写负向回退，并把新物料余额设置为最新盘点数。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明同一 Excel 行号更正生豆库存或耗材库存物料名称时，旧物料余额回退，新物料余额和台账汇总以最新 Excel 为准。

## 客户履约代发订单序号重传更正边界补证
- `PR-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION` / `DEV-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION` 已录入系统：同一外部代发订单跨 Excel 批次重传并更正序号时，必须更新原代发订单和 ERP 订单，不能因序号变化生成重复订单。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55591/kferp_direct_ship_sequence_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportCorrectedSequenceNoUpdatesExistingOrder -count=1 -v` 曾失败，失败点为同一外部订单 `YGS-SEQ-001` 序号从 `1` 修正为 `2` 后出现 2 条代发导入订单、2 张 ERP 订单和 2 条订单明细。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyDirectShipImportReimportCorrected(SequenceNoUpdatesExistingOrder|OrderHeaderUpdatesERPOrderSnapshot|StatusUpdatesERPShipStatus)|TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems' -count=1 -v` 通过，覆盖序号更正、订单头更正、发货状态更正和少行裁剪回归。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_SEQUENCE_REIMPORT_UI_CLICK_OK app=http://127.0.0.1:18151 pg=55621 evidence=apply_latest_direct_ship_batch_pr268 db=import_orders_1_seq_2_receiver_吴二_13100000001_浙江杭州滨江区_erp_orders_1_items_1_batch268102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=26801`，切到“代发清单”并点击应用当前修正版 `pr268-sequence-corrected.xlsx`，刷新后该批次显示 `已应用`；页面代发订单显示 `YGS-SEQ-UI-001 / 2026-03-04 / 吴二 13100000001 浙江杭州滨江区 / 待发货 / 1`，真实 PostgreSQL 核对同一外部订单只有 1 条导入订单且 `external_seq=2`，ERP 订单 1 张、明细 1 条，收件人/电话/地址为最新 Excel。
- 实现证据：`applyDirectShipOrderTx` 先按 `customer_id + external_order_no` 锁定原 `customer_direct_ship_import_orders`，刷新 `external_seq`、订单日期、收件快照、状态和 payload；仅没有原外部订单时才插入新导入订单，ERP `orders` 继续复用原 `order_id` 更新快照。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明同一外部订单号更正序号时不会生成重复代发订单或 ERP 订单。

## PR/DEV-269 生产管理菜单点击矩阵补证
- `PR-269-PRODUCTION-MENU-CLICK-MATRIX` / `DEV-269-PRODUCTION-MENU-CLICK-MATRIX` 已录入系统：生产管理剩余入口不能只停留在菜单可渲染，至少要证明 `workOrders`、`jobCards`、`qualityInspections`、`produceLogs`、`productionCosts` 有真实数据态和关键点击入口。
- UI 点击证据：`PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18160/vue-shell mock=auth_api_static chrome_cdp=9239 views=5 actions=status_filter,print,select_work_order,save_quality,batch_operator_filter,refresh quality_rows=2 cleanup=port_18160_free,port_9239_free covered=workOrders,jobCards,qualityInspections,produceLogs,productionCosts`；使用已构建 Vue shell、临时 mock auth/API 服务和 headless Chrome，逐页执行：生产工单 `status=completed` 筛选并点击打印；工序卡 `status=completed` 筛选；生产质检选择 `WO-PROD-MATRIX-001` 工单并保存“首锅通过”；生产日志按 `PB-MATRIX-001` 和完成人“小张”筛选；生产成本点击刷新。服务端请求日志确认触发 `GET /api/produce/work-orders?status=completed`、`GET /api/produce/job-cards?status=completed`、`POST /api/produce/quality-inspections`、`GET /api/produce/logs?batch_id=PB-MATRIX-001&operator=小张`、`GET /api/produce/costs`。
- 边界说明：该证据补齐生产管理五个剩余入口的真实浏览器点击和数据态，不替代后续接入真实后端业务数据后的更深失败态/权限态矩阵。

## PR/DEV-270 库存管理菜单点击矩阵补证
- `PR-270-INVENTORY-MENU-CLICK-MATRIX` / `DEV-270-INVENTORY-MENU-CLICK-MATRIX` 已录入系统：库存管理剩余入口不能只停留在菜单可渲染，至少要证明 `stockOperations`、`stockOutboundLogs`、`purchase`、`materials` 有真实数据态和关键点击入口。
- UI 点击证据：`INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18161/vue-shell mock=auth_api_static chrome_cdp=9240 views=4 actions=tab_switch,filter,open_delivery_note,save_supplier,create_order,receive_order,material_search,stock_backfill receipts=1 materials=1 cleanup=port_18161_free,port_9240_free covered=stockOperations,stockOutboundLogs,purchase,materials`；使用已构建 Vue shell、临时 mock auth/API 服务和 headless Chrome，逐页执行：库存作业切换 `原料入库/WIP领退/转仓/成品转仓/库存调整` tab；出库日志搜索 `SO-INV-MATRIX-001` 并打开出库单抽屉；采购入库保存供应商、创建采购单并点击收货入库；物料档案搜索 `矩阵库存水洗生豆`、选择物料并提交库存补录。服务端请求日志确认触发 `GET /api/stock/material-batch-locations?warehouse=wip&active_only=1`、`GET /api/stock/outbound-logs?limit=50&offset=0&q=SO-INV-MATRIX-001`、`GET /api/orders/701/delivery-note-preview`、`POST /api/purchase/suppliers`、`POST /api/purchase/orders`、`POST /api/purchase/receipts`、`GET /api/materials?limit=500&q=矩阵库存水洗生豆`、`POST /api/stock/adjustments`。
- 边界说明：该证据补齐库存管理四个剩余入口的真实浏览器点击和数据态，不替代后续真实后端业务数据下的采购失败态、盘点权限态、出库单生成失败态和追溯隐藏页矩阵。

## PR/DEV-271 商品与配方菜单点击矩阵补证
- `PR-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX` / `DEV-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX` 已录入系统：商品与配方剩余入口不能只停留在菜单可渲染，至少要证明 `productSettings`、`mallSettings`、`costing`、`bom` 有真实数据态和关键点击入口。
- UI 点击证据：`PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18162/vue-shell mock=auth_api_static chrome_cdp=9241 views=4 actions=create_public_product,save_category,select_customer_sku,save_product_basics,create_mall_product,open_costing_settings,save_costing_run,publish_costing_run,open_bean_list,select_bom_product,sync_bom,save_bom_item,save_bom_version,save_bag_mapping cleanup=port_18162_free,port_9241_free covered=productSettings,mallSettings,costing,bom`；使用已构建 Vue shell、临时 mock auth/API 服务和 headless Chrome，逐页执行：产品设置创建公共产品、新增分类、切换客户 SKU 并保存客户 SKU 基础信息；商城管理新增并保存商城商品；价格与豆单打开参数设置、保存试算、发布价格并打开豆单抽屉；BOM 配方选择产品、同步出品率、保存物料、保存版本和保存袋材映射。服务端请求日志确认触发 `POST /api/product-settings/products`、`POST /api/product-settings/categories`、`PUT /api/products/502`、`POST /api/customer-portal/admin/mall-products`、`GET /api/costing/settings`、`POST /api/costing/runs`、`POST /api/costing/runs/271/publish`、`GET /api/bom/detail/503`、`POST /api/bom/save`、`POST /api/bom/item/save`、`POST /api/bom/versions`、`POST /api/bom/bag-spec-mappings/save`。
- 边界说明：该证据补齐商品与配方四个剩余入口的真实浏览器点击和数据态，不替代后续真实后端业务数据下的图片上传失败态、BOM 删除/版本启用确认态、成本 PDF 打印态和客户专属豆单权限态矩阵。

## 客户履约代发重传运单号更正边界补证
- `PR-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC` / `DEV-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC` 已录入系统：同一外部代发订单用更正运单号的 Excel 重传时，订单不能继续显示已更正的旧导入运单号。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55576/kferp_customer_direct_ship_waybill?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportCorrectedWaybillReplacesImportedTrackings -count=1 -v` 曾失败，失败点为订单物流汇总为 `"SF-OLD-001\nSF-NEW-001"`，旧导入运单仍残留。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyDirectShipImportReimport(CorrectedWaybillReplacesImportedTrackings|ShorterOrderRemovesStaleItems|SameExternalOrderDoesNotDuplicateItems)' -count=1 -v` 通过，覆盖同一外部订单更正运单后只显示 `SF-NEW-001`，少行裁剪和同长度幂等仍通过。
- 实现证据：`directShipApplyState` 记录本批次提供的当前运单号；`trimDirectShipStaleTrackingsTx` 只裁剪 `customer_fulfillment_direct_ship` / `customer_fulfillment_direct_ship_item` 导入来源的旧运单号，并用 `refreshCustomerFulfillmentOrderTrackingSummaryTx` 刷新订单物流汇总。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_WAYBILL_REPLACE_UI_CLICK_OK app=http://127.0.0.1:18144 pg=55614 evidence=apply_latest_direct_ship_batch_pr253 response=applied db=tracking_SF_NEW_UI_001_old_0_new_1 batch2_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25301`，在客户履约运营台切到“代发清单”，页面显示 `pr253-new-waybill.xlsx / 代发清单` 当前可应用批次，点击“应用当前类型最新批次”后刷新显示该批次已应用；真实 PostgreSQL 核对 `orders.ship_tracking_no='SF-NEW-UI-001'`，`SF-OLD-UI-001` 运单行数为 0，`SF-NEW-UI-001` 运单行数为 1。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明更正运单号时以最新 Excel 导入值为准。

## 客户履约代发重传空运单号清空边界补证
- `PR-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR` / `DEV-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR` 已录入系统：同一外部代发订单用空运单号修正版 Excel 重传时，旧导入运单号必须被清空。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55577/kferp_customer_direct_ship_clear_waybill?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImportReimportBlankWaybillClearsImportedTrackings -count=1 -v` 曾失败，失败点为订单物流汇总仍是 `"SF-REMOVE-001"`，期望为空。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyDirectShipImportReimport(BlankWaybillClearsImportedTrackings|CorrectedWaybillReplacesImportedTrackings|ShorterOrderRemovesStaleItems|SameExternalOrderDoesNotDuplicateItems)' -count=1 -v` 通过，覆盖空运单号清空、非空运单更正、少行裁剪和同长度幂等。
- 实现证据：`directShipApplyState.recordTrackings` 即使当前运单号为空也记录该订单的当前导入状态；`trimDirectShipStaleTrackingsTx` 在当前集合为空时删除全部客户履约导入来源运单，并刷新 `orders.ship_tracking_no`。手工或其他来源运单不在删除范围内。
- UI 点击证据：`CUSTOMER_DIRECT_SHIP_WAYBILL_CLEAR_UI_CLICK_OK app=http://127.0.0.1:18144 pg=55614 evidence=apply_latest_direct_ship_batch_pr254 response=applied db=tracking_empty_old_0_batch4_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25401`，在客户履约运营台切到“代发清单”，页面显示 `pr254-blank-waybill.xlsx / 代发清单` 当前可应用批次，点击“应用当前类型最新批次”后刷新显示该批次已应用；真实 PostgreSQL 核对 `orders.ship_tracking_no=''`，旧 `SF-REMOVE-UI-001` 运单行数为 0。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明最新 Excel 运单号为空时会清空旧导入运单。

## 客户履约代加工托管流水更正数量边界补证
- `PR-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION` / `DEV-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION` 已录入系统：同一外部生豆入库或出库流水跨 Excel 批次重传并更正数量时，托管库存余额必须按差额修正。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55578/kferp_customer_custody_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta -count=1 -v` 曾失败，失败点为更正入库 2000g→1500g、出库 500g→300g 后余额仍为 `1500`，期望 `1200`。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyProcessingImportReimport(CorrectedCustodyMovementAdjustsBalanceDelta|SameCustodyMovementDoesNotDoubleBalance)' -count=1 -v` 通过，覆盖同一外部托管流水重复重传不重复加减，以及数量更正按差额修正。
- UI 点击证据：`CUSTOMER_PROCESSING_CUSTODY_REIMPORT_DELTA_UI_CLICK_OK app=http://127.0.0.1:18145 pg=55615 evidence=apply_latest_processing_workbook_pr255 db=custody_1200_receipt_1500_issue_-300_ledger_2_batch255102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25501`，在客户履约运营台保持“代加工工单”，页面显示 `pr255-custody-corrected.xlsx / 代加工工单` 当前可应用批次，点击“应用当前类型最新批次”后刷新显示该批次 `已应用`；真实 PostgreSQL 核对 `customer_custody_balances.quantity_g=1200`，入库台账 delta 更新为 `1500`、出库台账 delta 更新为 `-300`，托管库存台账仍只有 2 条。
- 实现证据：`applyRawBeanMovementTx` 改为通过 `upsertCustodyMovementLedgerTx` 锁定相同外部流水的旧台账，更新 `qty_g_delta` 后只把 `新台账增减量 - 旧台账增减量` 写入 `customer_custody_balances`。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明托管生豆出入库数量修正要保持外部流水号不变，系统按差额调余额。

## 客户履约结算单费用重传更正边界补证
- `PR-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION` / `DEV-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION` 已录入系统：同一外部结算费用行跨 Excel 批次重传并更正未结费用金额时，费用明细必须以最新 Excel 为准。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55579/kferp_customer_settlement_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplySettlementImportReimportCorrectedFeeUpdatesUnsettledFeeItem -count=1 -v` 曾失败，失败点为同一外部费用行金额从 8000 分修正为 9500 分后，`customer_fee_items` 总额仍为 `8000`。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplySettlementImportReimport(CorrectedFeeUpdatesUnsettledFeeItem|SameFeeDoesNotDuplicateFeeItems)' -count=1 -v` 通过，覆盖同一外部费用行不重复生成费用，且未结未绑定月结批次的费用金额按最新 Excel 更新。
- UI 点击证据：`CUSTOMER_SETTLEMENT_FEE_REIMPORT_AMOUNT_UI_CLICK_OK app=http://127.0.0.1:18145 pg=55615 evidence=apply_latest_settlement_workbook_pr256 db=fee_9500_count_1_batch256102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25601`，在客户履约运营台切到“结算单”，页面显示 `pr256-settlement-corrected.xlsx / 结算单` 当前可应用批次，点击“应用当前类型最新批次”后刷新显示该批次 `已应用`；页面费用明细显示 `processing / 烘焙费 / 95.00 / customer_fulfillment_import`，真实 PostgreSQL 核对 `customer_fee_items` 仍只有 1 条且金额汇总为 9500 分。
- 实现证据：`applySettlementImportRow` 命中相同外部 `fee_item` key 或同一导入行 source 时调用 `refreshImportedSettlementFeeItemTx`，只刷新 `status='unsettled' AND settlement_batch_id=0` 的导入费用行的 `fee_type`、`amount`、`occurred_at`、`note` 和 `source_id`。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明未结费用修正要保持外部费用行 key 不变，已确认或已结算批次走撤回草稿或财务调整。

## 客户履约代加工 SKU 外部键重传更正边界补证
- `PR-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION` / `DEV-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION` 已录入系统：同一外部 SKU 编码跨 Excel 批次重传并更正 SKU 名称或烘焙度时，不能生成重复客户专属商品或让工作台选择器继续显示旧名称。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55580/kferp_customer_sku_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct -count=1 -v` 曾失败，失败点为相同 `customer_sku:YGS-HK-227` 用旧名/浅烘和新名/中烘跨批次应用后，`products count = 2, want 1`。
- GREEN 证据：`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct|TestApplyProcessingImportCreatesCustodyAndWorkOrdersIdempotently' -count=1 -v` 通过，覆盖相同外部 SKU key 重传后只保留 1 条 active customer-only 商品，商品名称和烘焙度更新为最新 Excel 值，客户履约工作台 SKU 选项只显示 1 个最新选项。
- UI 点击证据：`CUSTOMER_SKU_REIMPORT_EXTERNAL_KEY_UI_CLICK_OK app=http://127.0.0.1:18146 pg=55616 evidence=apply_latest_processing_workbook_pr257 db=product_new_name_roast_products_1_batch257102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25701`，在客户履约运营台点击应用当前“代加工工单”修正版 `pr257-sku-corrected.xlsx`，刷新后该批次显示 `已应用`；再展开“成品名称”客户 SKU 候选项，页面只显示 `誉观山花魁新名227g / YGS-HK-227 / 227g / 中烘`，真实 PostgreSQL 核对 customer-only 商品仍只有 1 条，名称/烘焙度为最新值。
- 实现证据：`upsertCustomerProductTx` 对 `customer_sku` 先按同客户相同 `external_key` 查找已应用导入行的目标 `products.id`，命中后更新原商品 `name/roast_level/visibility/custom_type/active`；`listCustomerSKUOptions` 按 `product_id` 优先去重，避免同一商品因名称修正被显示成新旧两个选项。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明 SKU 名称或烘焙度修正要保持 SKU 编码不变，系统会更新原客户商品和工作台选择器。

## 客户履约代加工库存余额重传台账更正边界补证
- `PR-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION` / `DEV-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION` 已录入系统：同一外部生豆余额或耗材余额跨 Excel 批次重传并更正盘点数时，库存余额和库存台账 delta 汇总必须同时以最新 Excel 为准。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55581/kferp_customer_balance_correction?sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta -count=1 -v` 曾失败，失败点为生豆最新余额已显示 `1200`，但库存台账 delta 汇总仍为 `1000`。
- GREEN 证据：同库同跑 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta|TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta|TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance' -count=1 -v` 通过，覆盖生豆余额 1000→1200、耗材余额 100→80 修正后，余额表和台账 delta 汇总都等于最新值，同时回归增减流水幂等和数量更正。
- UI 点击证据：`CUSTOMER_CUSTODY_BALANCE_REIMPORT_LEDGER_UI_CLICK_OK app=http://127.0.0.1:18146 pg=55616 evidence=apply_latest_processing_workbook_pr258 db=raw_1200_ledger_1200_packaging_80_ledger_80_batch258102_applied`；Chrome CDP 打开 `/vue-shell?view=customerFulfillment&customer_id=25801`，在客户履约运营台点击应用当前“代加工工单”修正版 `pr258-balance-corrected.xlsx`，刷新后该批次显示 `已应用`；页面托管库存显示 `埃塞花魁 1200g` 和 `227g袋 80件`，真实 PostgreSQL 核对生豆余额/台账汇总 `1200/1200`，耗材余额/台账汇总 `80/80`。
- 实现证据：`raw_bean_balance` / `packaging_balance` 改为调用 `upsertCustodyBalanceAdjustmentLedgerTx`；命中相同外部余额 key 时先读取旧盘点调整 delta，用 `当前余额 - 旧 delta` 反推基础余额，再更新原 `balance_adjustment` 台账 delta，并设置 `customer_custody_balances` 最新余额。
- 操作手册与需求：`REQUIREMENTS.md`、`orderapp-remote/docs/REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`orderapp-remote/docs/ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已说明库存余额盘点数修正要保持外部余额 key 不变，系统会同步更新原盘点调整台账和余额。

## ERP 生产取消边界补证
- `PR-232-PRODUCTION-CANCEL-WIP-RELEASE-EVIDENCE` / `DEV-232-PRODUCTION-CANCEL-WIP-RELEASE-EVIDENCE` 已录入系统：取消生产必须同时取消生产工单和工序卡，并释放未消耗的 WIP 占用，避免废弃工单继续占用原料。
- 真实 PostgreSQL API 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_cancel?host=/tmp&port=55556&sslmode=disable' go test ./internal/interfaces/http/production -run 'TestProduceCancelAPIReleasesWIPReservationAndCancelsWorkOrder' -count=1 -v` 通过，覆盖 `/api/produce/running/cancel` 后 `produce_running_items.status='cancelled'`、`work_orders.status='cancelled'`、`job_cards.status='cancelled'`、`work_order_material_reservations.status='released'` 且 `returned_g=reserved_g-consumed_g`。
- 回归同跑：同一临时 PostgreSQL 下 `TestProduceStartAPIUsesSubmittedInputG`、`TestProduceStartAPIRejectsEmptySelectionWithoutOpeningWork`、`TestProduceStartAPIRejectsMissingInputWithoutOpeningWork` 仍通过。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18136/vue-shell?view=produceRunning`，在生产中页面对 `BATCH-CANCEL-001` 点击“取消生产”，页面显示 `生产已取消` 且列表变为“暂无生产中项目”；真实 PostgreSQL 反查为 `cancelled|cancelled|cancelled|released:400`，即 running item、work order、job card 均取消，WIP reservation 释放 400g。证据标记：`PRODUCTION_CANCEL_WIP_RELEASE_UI_CLICK_OK app=http://127.0.0.1:18136 pg=55606 evidence=cancel_running_item_77`。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充取消生产会取消工单/工序卡并释放未消耗 WIP 占用。

## ERP 生产完工冻结 WIP 批次边界补证
- `PR-233-PRODUCTION-FINISH-WIP-QUALITY-BLOCK-EVIDENCE` / `DEV-233-PRODUCTION-FINISH-WIP-QUALITY-BLOCK-EVIDENCE` 已录入系统：完成生产遇到质检待处理或不通过的 WIP 批次时必须拒绝扣料，并且不能写完成日志、成品库存或 FP 批次。
- 真实 PostgreSQL API 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55557/kferp_prod_quality?sslmode=disable' go test ./internal/interfaces/http/production -run 'TestProduce(FinishAPIRejectsHeldWIPBatchWithoutWritingFinishArtifacts|FinishAPIUsesEditedInputForFullCompletion|CancelAPIReleasesWIPReservationAndCancelsWorkOrder|StartAPIRejects(EmptySelection|MissingInput)|StartAPIUsesSubmittedInputG)' -count=1 -v` 通过；其中 `TestProduceFinishAPIRejectsHeldWIPBatchWithoutWritingFinishArtifacts` 覆盖 `/api/produce/running/finish` 遇 `material_batches.quality_status='hold'` 的 WIP 批次返回 400 quality block，且 `produce_running_items.status` 保持 `running`。
- 事务回滚证据：同一测试断言 `production_logs`、`finished_inventory`、`stock_batches` 和 `material_consumption_logs` 均未写入，原料总量、原料批次余量和 WIP 仓位置数量均保持不变。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18135/vue-shell?view=produceRunning`，在生产中页面对 `BATCH-HELD-WIP` 点击“完成”，页面显示 `WIP stock blocked by quality status for material 10` 且生产中行仍保留；真实 PostgreSQL 反查为 `running|0|0|0|0`，即 `produce_running_items.status=running`，`production_logs`、`finished_inventory`、`stock_batches`、`material_consumption_logs` 均未写入。证据标记：`PRODUCTION_FINISH_WIP_QUALITY_UI_CLICK_OK app=http://127.0.0.1:18135 pg=55605 evidence=finish_held_wip_batch`。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充完工遇冻结 WIP 批次时先复核质检、解除冻结或补领合格 WIP 批次后再提交完工。

## ERP 合并多规格生产部分完工边界补证
- `PR-234-PRODUCTION-MULTISPEC-PARTIAL-FINISH-GUARD` / `DEV-234-PRODUCTION-MULTISPEC-PARTIAL-FINISH-GUARD` 已录入系统：合并多规格生产单不能被部分完工，避免只录入其中一个规格导致同一烘焙工单库存、成本和订单状态不一致。
- 真实 PostgreSQL API 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55558/kferp_prod_multispec?sslmode=disable' go test ./internal/interfaces/http/production -run 'TestProduce(FinishAPIRejectsPartialForMultiSpecRunWithoutWritingArtifacts|FinishAPIMultiSpecRunCompletesAllLinkedOrders|FinishAPIRejectsHeldWIPBatchWithoutWritingFinishArtifacts|CancelAPIReleasesWIPReservationAndCancelsWorkOrder|StartAPIRejects(EmptySelection|MissingInput)|StartAPIUsesSubmittedInputG)' -count=1 -v` 通过；其中 `TestProduceFinishAPIRejectsPartialForMultiSpecRunWithoutWritingArtifacts` 覆盖多规格 `produce_running_outputs` 存在时提交 `partial=true`，`/api/produce/running/finish` 返回 400 `合并多规格生产暂不支持部分完工`，且 `produce_running_items.status` 保持 `running`。
- 事务回滚证据：同一测试断言 `produce_running_outputs` 未更新实际产出，`production_logs`、`finished_inventory`、`stock_batches` 和 `material_consumption_logs` 均未写入。
- UI 点击证据：`PRODUCTION_MULTISPEC_PARTIAL_UI_CLICK_OK app=http://127.0.0.1:18139 pg=55609 evidence=multi_spec_finish_click_with_partial_tamper response=400 error=multispec_partial_block ui_partial_checkbox_count=0 db=running_outputs_2_finished_0_logs_0_inventory_0_stock_batches_0_consumption_0`；Chrome CDP 打开 `/vue-shell?view=produceRunning`，多规格生产中行显示“多规格”且不暴露“部分完工”复选框；对可见“完成”按钮点击时把请求改为 `partial=true`，页面显示 `合并多规格生产暂不支持部分完工`，真实 PostgreSQL 核对 running 仍为 `running`、2 条输出完成数仍为 0，生产日志、成品库存、FP 批次和原料消耗日志均未写入。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已说明合并多规格生产单必须一次填写全部规格产出；如果需要分批完成，应按单规格分别生成生产计划。

## ERP 生产完工产出投料比例边界补证
- `PR-236-PRODUCTION-FINISH-OUTPUT-INPUT-RATIO-GUARD` / `DEV-236-PRODUCTION-FINISH-OUTPUT-INPUT-RATIO-GUARD` 已录入系统：生产完工的成品克重不能大于本次消耗投料，避免凭空增加成品库存、FP 批次和错误成本。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_output?host=/tmp&port=55560&sslmode=disable' go test ./internal/interfaces/http/production -run TestProduceFinishAPIRejectsOutputGreaterThanConsumedInputWithoutWritingArtifacts -count=1 -v` 曾失败，失败点为 `/api/produce/running/finish` 对 681g 成品 / 600g 投料返回 `200 {"ok":true}`。
- GREEN 证据：同一真实 PostgreSQL API 测试修复后通过；`TestProduceFinishAPIRejectsOutputGreaterThanConsumedInputWithoutWritingArtifacts` 覆盖接口返回 400 `finished output cannot exceed consumed input`，`produce_running_items.status` 保持 `running`。
- 事务回滚证据：同一测试断言 `production_logs`、`finished_inventory`、`stock_batches` 和 `material_consumption_logs` 均未写入，原料和包装物库存保持不变。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18134/vue-shell?view=produceRunning`，在生产中页面把 `BATCH-OUTPUT-GT-INPUT` 的完成件数改为 3 件、投料保持 600g 后点击“完成”，页面显示 `finished output cannot exceed consumed input` 且生产中行仍保留；真实 PostgreSQL 反查为 `running|0|0|0|0`，即 `produce_running_items.status=running`，`production_logs`、`finished_inventory`、`stock_batches`、`material_consumption_logs` 均未写入。证据标记：`PRODUCTION_FINISH_OUTPUT_INPUT_UI_CLICK_OK app=http://127.0.0.1:18134 pg=55604 evidence=finish_3x227g_with_600g_input`。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充成品总克重不能大于本次消耗投料，以及出现提示时核对实际产出、投料和补投料。

## ERP 生产 WIP 占用调整可用量质量口径补证
- `PR-237-PRODUCTION-WIP-ADJUST-QUALITY-AVAILABILITY` / `DEV-237-PRODUCTION-WIP-ADJUST-QUALITY-AVAILABILITY` 已录入系统：WIP 占用调整成功后返回的 WIP 总量和可用量必须排除质检冻结或拒收批次，避免操作员按错误可用量继续调整占用。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_wip_adjust?host=/tmp&port=55561&sslmode=disable' go test ./internal/interfaces/http/production -run TestProduceWIPReservationAdjustAPIExcludesHeldWIPFromReturnedAvailability -count=1 -v` 曾失败，失败点为 `/api/produce/wip-reservations/adjust` 调整成功后返回 `wip_g=1500, available_g=600`，把 `quality_status='hold'` 的 WIP 批次计入了返回可用量。
- GREEN 证据：同一真实 PostgreSQL API 测试修复后通过；`TestProduceWIPReservationAdjustAPIExcludesHeldWIPFromReturnedAvailability` 覆盖调整响应返回 `wip_g=1000, available_g=100`，只统计 active 且非 `hold/reject` 的 WIP 批次。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18154/vue-shell?view=warehouseInventory`，打开仓库库存页 WIP 占用抽屉，把 `WO-WIP-ADJUST-UI-001` 的占用从 300g 调整为 400g 后点击“调整”；页面返回行显示 `WIP 1000g / 可用 100g`，真实 PostgreSQL 反查 `reservation_237901_reserved_400_remaining_400_status_reserved`。证据标记：`PRODUCTION_WIP_ADJUST_QUALITY_AVAILABILITY_UI_CLICK_OK app=http://127.0.0.1:18154 pg=55624 evidence=warehouse_wip_drawer_adjust_reserved_400 db=reservation_237901_reserved_400_available_100`。
- 口径一致证据：`getWIPReservationRowTx` 调整后返回行与 WIP 列表、调整校验使用同一质量过滤条件，避免 API 成功响应和列表页可用量不一致。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充 WIP 占用抽屉的 WIP 总量和可用量只统计 active 且非待处理/拒收冻结批次，以及调整后可用量低于预期时先处理质检或更换合格批次。

## ERP 生产 WIP 按生产中记录释放落库边界补证
- `PR-238-PRODUCTION-WIP-RELEASE-RUNNING-ITEM-GUARD` / `DEV-238-PRODUCTION-WIP-RELEASE-RUNNING-ITEM-GUARD` 已录入系统：按 running item 释放 WIP 占用时必须实际更新匹配占用行，即使历史占用缺少工单行也不能只返回释放统计不落库。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_wip_release?host=/tmp&port=55562&sslmode=disable' go test ./internal/interfaces/http/production -run TestProduceWIPReservationReleaseAPIReleasesRunningReservationWithoutWorkOrderRow -count=1 -v` 曾失败，失败点为 `/api/produce/wip-reservations/release` 返回释放统计后，`work_order_material_reservations.status` 仍为 `reserved` 且 `returned_g=50`。
- GREEN 证据：同一真实 PostgreSQL API 测试修复后通过；`TestProduceWIPReservationReleaseAPIReleasesRunningReservationWithoutWorkOrderRow` 覆盖按 `running_item_id` 释放孤立占用后，响应 `released_count=1/released_g=350`，数据库行变为 `released/returned_g=400`。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18154/vue-shell?view=warehouseInventory`，打开 WIP 占用抽屉，对 `孤立WIP占用生豆UI` 点击“释放”；页面刷新后该占用行消失，真实 PostgreSQL 反查 `reservation_238903_status_released_returned_400_remaining_0`。证据标记：`PRODUCTION_WIP_RELEASE_RUNNING_ITEM_UI_CLICK_OK app=http://127.0.0.1:18154 pg=55624 evidence=warehouse_wip_drawer_release_running_item db=reservation_238903_released_returned_400`。
- 一致性证据：`ReleaseWIPReservations` 统计和更新共用同一个 `WHERE`；按工单号过滤时使用 `EXISTS`，按 `running_item_id` 过滤时不再依赖 `work_orders` join，避免统计和落库范围不一致。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充按生产中记录释放时即使历史占用缺少工单行也必须释放匹配占用，以及响应释放数量必须和数据库占用状态一致。

## ERP 生产 WIP 占用列表可用量口径补证
- `PR-239-PRODUCTION-WIP-LIST-ACTIVE-REMAINING-AVAILABILITY` / `DEV-239-PRODUCTION-WIP-LIST-ACTIVE-REMAINING-AVAILABILITY` 已录入系统：WIP 占用列表展示的 WIP 总量和可用量必须排除停用或已耗尽批次，避免操作员看到虚高可用库存。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_wip_list?host=/tmp&port=55563&sslmode=disable' go test ./internal/interfaces/http/production -run TestProduceWIPReservationsAPIExcludesInactiveAndDepletedBatchesFromAvailability -count=1 -v` 曾失败，失败点为 `/api/produce/wip-reservations` 返回 `wip_g=1900, available_g=1500`，把 `status='inactive'` 和 `remaining_g=0` 的 WIP 批次计入了列表可用量。
- GREEN 证据：同一真实 PostgreSQL API 测试修复后通过；`TestProduceWIPReservationsAPIExcludesInactiveAndDepletedBatchesFromAvailability` 覆盖列表响应返回 `wip_g=1000, available_g=600`，只统计 active、`remaining_g>0`、`qty_g>0` 且非 `hold/reject` 的 WIP 批次。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18154/vue-shell?view=warehouseInventory`，打开 WIP 占用抽屉，`WIP列表口径生豆UI` 行显示 `WIP 1000g / 可用 600g`；同库同时存在 `MB-WIP-LIST-INACTIVE-UI` 700g 和 `MB-WIP-LIST-DEPLETED-UI` 200g，均未计入可用量。证据标记：`PRODUCTION_WIP_LIST_ACTIVE_REMAINING_UI_CLICK_OK app=http://127.0.0.1:18154 pg=55624 evidence=warehouse_wip_drawer_list_active_remaining db=reservation_239904_wip_1000_available_600`。
- 口径一致证据：`ListWIPReservations` 的 WIP 汇总与调整返回、调整校验使用相同 active/remaining/quality 可用量口径，避免抽屉列表和调整成功响应不一致。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充 WIP 占用抽屉只统计 active、仍有剩余且非待处理/拒收冻结批次，已停用或已耗尽 WIP 批次不会计入可用量。

## ERP 生产开始陈旧计划重复开工边界补证
- `PR-243-PRODUCTION-START-STALE-PLAN-DUPLICATE-GUARD` / `DEV-243-PRODUCTION-START-STALE-PLAN-DUPLICATE-GUARD` 已录入系统：生产开始必须拒绝陈旧计划或重复点击造成的重复开工，避免同一订单重复生成生产中记录、生产工单和 WIP 占用。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_prod_start_stale?host=/tmp&port=55567&sslmode=disable' go test ./internal/interfaces/http/production -run TestProduceStartRepositoryRejectsStaleNeedAlreadyRunning -count=1 -v` 曾失败，失败点为同一 `StartExecutionCommand` 第二次进入 `Repository.Start` 时返回 `err=<nil>`。
- GREEN 证据：同一真实 PostgreSQL 测试修复后通过；`TestProduceStartRepositoryRejectsStaleNeedAlreadyRunning` 覆盖第二次陈旧开工返回 `production already started`，`produce_running_items` 和 `work_orders` 均只保留 1 条。
- 并发/陈旧计划证据：`Repository.Start` 在事务内先用 `FOR UPDATE` 锁定订单和代加工生产需求引用，再执行 `ensureStartRefsNotRunningTx`；发现同一订单或请求已存在 running item 时，在写 finished allocation、running item、work order、WIP reservation 之前返回错误。
- UI 点击证据：`PRODUCTION_START_STALE_UI_CLICK_OK app=http://127.0.0.1:18138 pg=55608 evidence=two_tabs_same_plan_first_start_then_stale_second_click response=400 error=no_startable_production_data db=running_1_work_orders_1_reservations_1 reserved_g=600`；Chrome CDP 两个标签同时打开同一旧生产计划，A 标签点击“开始生产”返回 `200 {"ok":true}` 并跳转生产中，B 标签在未刷新的旧计划再次点击“开始生产”返回 400 `没有可开始生产的数据`，页面显示同错误；真实 PostgreSQL 核对 `SO-DUP-START` 只生成 1 条 running、1 张 work order、1 条 600g WIP reservation，订单状态为 `生产中`。
- 回归证据：同库同跑 `TestProduceStartAPIRejectsEmptySelectionWithoutOpeningWork`、`TestProduceStartAPIRejectsMissingInputWithoutOpeningWork`、`TestProduceStartAPIUsesSubmittedInputG`、`TestProduceStartAPIMergesSameProductSpecsAndKeepsAllOrderNos`，确认开始生产原有正常/失败路径不受影响。
- 操作手册：`OP_MANUAL_PRODUCTION.md` 与 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md` 已补充重复点击或旧生产计划再次提交时返回 `production already started`，不会重复生成生产中记录、生产工单或 WIP 占用。

## ERP 财务未结账月份调整边界补证
- `PR-235-FINANCE-ADJUSTMENT-DRAFT-MONTH-API-GUARD` / `DEV-235-FINANCE-ADJUSTMENT-DRAFT-MONTH-API-GUARD` 已录入系统：未结账月份不能通过结账后调整接口新增金额调整，避免 draft 月份绕过月结流程留下调整记录。
- 真实 PostgreSQL API 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@127.0.0.1:55559/kferp_fin_adjust?sslmode=disable' go test ./internal/interfaces/http/finance -run 'TestFinance(AdjustmentAPIRejectsDraftMonthWithoutWritingAdjustment|TaxLedgerAPIReturnsBadRequestWhenServiceRejectsClosedMonth|ExpenseAndClosingAPI|ImprovementAPIs)' -count=1 -v` 通过；其中 `TestFinanceAdjustmentAPIRejectsDraftMonthWithoutWritingAdjustment` 覆盖 `/api/finance/adjustments` 对 draft 月份返回 400 `month must be closed before adjustment`。
- 落库证据：同一测试断言 `finance_adjustments` 中对应月份没有新增记录。
- UI 点击证据：`FINANCE_ADJUSTMENT_DRAFT_MONTH_UI_CLICK_OK app=http://127.0.0.1:18140 pg=55610 evidence=draft_month_disabled_then_forced_adjustment_click response=400 error=month_must_be_closed_before_adjustment db=finance_adjustments_0`；Chrome CDP 打开 `/vue-shell?view=financeClosing`，切到 `2026-06` draft 月份后“新增调整”按钮为 disabled；从浏览器上下文强制点击该按钮，页面显示 `month must be closed before adjustment`，真实 PostgreSQL 核对 `finance_adjustments` 中 `2026-06` 行数仍为 0。
- 操作手册：`OP_MANUAL_FINANCE.md` 与 `orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已补充未结账月份不能新增结账后调整，以及出现 `month must be closed before adjustment` 时应先完成月度结账或回到原来源单据处理。

## ERP 财务费用停用员工边界补证
- `PR-244-FINANCE-EXPENSE-INACTIVE-EMPLOYEE-GUARD` / `DEV-244-FINANCE-EXPENSE-INACTIVE-EMPLOYEE-GUARD` 已录入系统：财务新增费用只能选择在职员工，停用员工不能通过接口继续作为新费用经办人写入账目。
- RED 证据：`go test ./internal/application/finance -run TestCreateExpenseRejectsInactiveEmployee -count=1 -v` 曾失败，失败点为停用员工 `employee_id=7` 创建费用返回 `err=<nil>`；真实 PostgreSQL API 测试 `ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_finance_inactive_employee?host=/tmp&port=55568&sslmode=disable' go test ./internal/interfaces/http/finance -run TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense -count=1 -v` 曾返回 200，并把 `employee_name` 写成 `离职员工`。
- GREEN 证据：服务层新增 `ensureActiveExpenseEmployee`，在写入 `finance_expenses` 前通过员工列表校验 `employee_id` 仍为 active；停用员工返回 `employee inactive`，缺失员工返回 `employee not found`。
- 落库证据：真实 PostgreSQL `TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense` 覆盖 `/api/finance/expenses` 对停用员工返回 400 `employee inactive`，并断言 `finance_expenses` 中该员工费用行数为 0；同一测试也覆盖在职员工 `employee_id=8` 仍可正常写入。
- UI 点击证据：`FINANCE_EXPENSE_INACTIVE_EMPLOYEE_UI_CLICK_OK app=http://127.0.0.1:18143 pg=55613 evidence=active_employee_visible_inactive_hidden_then_tampered_employee_click response=400 error=employee_inactive db=finance_expenses_0`；Chrome CDP 打开 `/vue-shell?view=financeExpenses`，员工下拉框只显示 `PR244在职经办人` 且不显示 `PR244停用经办人`，随后从页面上下文篡改员工选择为停用员工并真实点击“保存”；页面显示 `employee inactive` 和“暂无费用记录”，真实 PostgreSQL 核对 `employee_id=2447` 的 `finance_expenses` 行数为 0。
- 操作手册：`OP_MANUAL_FINANCE.md` 与 `orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已补充费用只能关联在职员工，停用员工不能作为新费用经办人，历史费用仍可追溯。

## ERP 财务费用维度引用边界补证
- `PR-245-FINANCE-EXPENSE-DIMENSION-REFERENCE-GUARD` / `DEV-245-FINANCE-EXPENSE-DIMENSION-REFERENCE-GUARD` 已录入系统：财务新增费用的订单、客户、商品维度必须引用系统中真实存在的业务对象，避免费用归集到无法追溯的脏 ID。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_finance_dimensions?host=/tmp&port=55569&sslmode=disable' go test ./internal/interfaces/http/finance -run TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense -count=1 -v` 曾失败，缺失订单、客户、商品三个子用例均返回 200，并写入 3 条 `finance_expenses`。
- GREEN 证据：`FinanceRepository.CreateExpense` 新增 `validateExpenseDimensionRefs`，写入前分别校验 `orders`、`customers`、`products` 中是否存在对应 ID；缺失时返回 `finance dimension order not found`、`finance dimension customer not found` 或 `finance dimension product not found`。
- 落库证据：真实 PostgreSQL `TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense` 覆盖缺失维度全部返回 400，`finance_expenses` 行数仍为 0；同一测试覆盖真实存在的 `order_id/customer_id/product_id` 仍可正常写入并回显业务维度。
- UI 证据：Chrome CDP 操作本地 `http://127.0.0.1:18133/vue-shell?view=financeExpenses`，在费用管理页三次填写并点击保存：缺失订单 `order_id=999` 返回 `finance dimension order not found`、订单客户不一致 `order_id=256/customer_id=19` 返回 `finance dimension customer does not match order`、订单商品不一致 `order_id=256/product_id=10` 返回 `finance dimension product does not match order`；页面保持“暂无费用记录”，真实 PostgreSQL 反查 `finance_expenses` 中 `UI维度缺失/UI客户不一致/UI商品不一致` 行数为 0。证据标记：`FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK app=http://127.0.0.1:18133 pg=55603 evidence=missing_order_then_customer_mismatch_then_product_mismatch`。
- 操作手册：`OP_MANUAL_FINANCE.md` 与 `orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已补充费用维度 ID 必须能在系统中找到，出现 `finance dimension ... not found` 时先回到对应业务页面确认对象。

## ERP 财务费用订单客户一致性边界补证
- `PR-246-FINANCE-EXPENSE-ORDER-CUSTOMER-MATCH-GUARD` / `DEV-246-FINANCE-EXPENSE-ORDER-CUSTOMER-MATCH-GUARD` 已录入系统：财务新增费用同时填写订单和客户维度时，客户必须与订单归属客户一致，避免费用挂真实订单却归到错误客户。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_finance_match?host=/tmp&port=55570&sslmode=disable' go test ./internal/interfaces/http/finance -run TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense -count=1 -v` 曾失败，订单 `SO-CUSTOMER-MATCH` 属于客户 18，但提交 `customer_id=19` 时接口返回 200 并写入费用。
- GREEN 证据：`FinanceRepository.CreateExpense` 新增 `ensureExpenseOrderCustomerMatch`，当 `order_id` 和 `customer_id` 同时存在时读取 `orders.customer_id` 并与请求客户比对；不一致返回 `finance dimension customer does not match order`。
- 落库证据：真实 PostgreSQL `TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense` 覆盖不一致时返回 400 且 `finance_expenses` 行数为 0；同一测试覆盖客户与订单归属一致时仍可正常写入。
- 操作手册：`OP_MANUAL_FINANCE.md` 与 `orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已补充订单和客户同时填写时客户维度必须与订单归属客户一致。

## ERP 财务费用订单商品一致性边界补证
- `PR-247-FINANCE-EXPENSE-ORDER-PRODUCT-MATCH-GUARD` / `DEV-247-FINANCE-EXPENSE-ORDER-PRODUCT-MATCH-GUARD` 已录入系统：财务新增费用同时填写订单和商品维度时，商品必须属于该订单明细，避免费用挂真实订单却归到错误商品。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_finance_product_match?host=/tmp&port=55571&sslmode=disable' go test ./internal/interfaces/http/finance -run TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense -count=1 -v` 曾失败，订单 `SO-PRODUCT-MATCH` 只包含商品 9，但提交 `product_id=10` 时接口返回 200 并写入费用。
- GREEN 证据：`FinanceRepository.CreateExpense` 新增 `ensureExpenseOrderProductMatch`，当 `order_id` 和 `product_id` 同时存在时要求 `order_items` 中存在对应订单商品行；不一致返回 `finance dimension product does not match order`。
- 落库证据：真实 PostgreSQL `TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense` 覆盖不一致时返回 400 且 `finance_expenses` 行数为 0；同一测试覆盖商品属于订单明细时仍可正常写入。
- 操作手册：`OP_MANUAL_FINANCE.md` 与 `orderapp-remote/docs/OP_MANUAL_FINANCE.md` 已补充订单和商品同时填写时商品维度必须属于该订单明细。

## ERP 客户履约重复周期月结总额边界补证
- `PR-240-CUSTOMER-SETTLEMENT-DUPLICATE-PERIOD-TOTAL-GUARD` / `DEV-240-CUSTOMER-SETTLEMENT-DUPLICATE-PERIOD-TOTAL-GUARD` 已录入系统：客户履约同一周期重复生成月结时必须保持已有结算批次金额和费用行，避免重复点击把已结费用总额清零。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_customer_settlement?host=/tmp&port=55564&sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals -count=1 -v` 曾失败，失败点为第二次同周期 `CreateSettlement` 返回同一 `BatchID`，但 `FeeItems=0`、`TotalAmountCents=0`。
- GREEN 证据：同一真实 PostgreSQL 仓储测试修复后通过；`TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals` 覆盖重复生成同周期月结仍返回同一批次、2 条费用、`10400` 分，并且数据库 `customer_settlement_batches.total_amount` 保持 `104.00`。
- 一致性证据：`CreateSettlement` 先把本次未结费用绑定到批次，再按 `settlement_batch_id` 汇总全部已绑定费用重算总额，重复提交和后续补录费用并入同一周期都不会覆盖旧金额为 0。
- UI 点击证据：`CUSTOMER_SETTLEMENT_DUPLICATE_UI_CLICK_OK app=http://127.0.0.1:18137 pg=55607 evidence=initial_generate_then_duplicate_2026_05 response=200 fee_items=2 total_amount_cents=10400 visible_batch_amount=104.00`；Chrome CDP 在 `/vue-shell?view=customerFulfillment&customer_id=24001` 点击“生成月结”，同一 2026-05 周期重复点击仍返回同一批次 2 条费用、10400 分，页面结算批次显示 `2026-05-01 至 2026-05-31 / draft / 104.00`。
- 操作手册：`OP_MANUAL_CUSTOMER_FULFILLMENT.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已补充重复生成同一周期月结不会清零已结费用或结算批次金额，以及金额变 0 时的异常处理。

## ERP 客户履约空周期月结边界补证
- `PR-241-CUSTOMER-SETTLEMENT-EMPTY-PERIOD-GUARD` / `DEV-241-CUSTOMER-SETTLEMENT-EMPTY-PERIOD-GUARD` 已录入系统：客户履约月结周期内没有可结算费用时必须拒绝生成 0 元空结算批次，避免对账列表出现无业务依据的空单。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_customer_settlement_empty?host=/tmp&port=55565&sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch -count=1 -v` 曾失败，失败点为空周期 `CreateSettlement` 返回 nil error 并创建 0 元结算批次。
- GREEN 证据：同一真实 PostgreSQL 仓储测试修复后通过；`TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch` 覆盖空周期返回 `no fees for settlement period`，`customer_settlement_batches` 不写入记录，区间外未结费用仍保持未结。
- 回归证据：同库同跑 `TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals`、`TestCreateSettlementAggregatesUnsettledFees`、`TestCreateSettlementRejectsCustomerWithoutSettlementCapability`，确认正常月结、重复月结和能力 gate 不受影响。
- UI 点击证据：`CUSTOMER_SETTLEMENT_EMPTY_UI_CLICK_OK app=http://127.0.0.1:18137 pg=55607 evidence=click_2026_06_empty_period response=400 error=no_fees_for_settlement_period db=june_batches_0`；Chrome CDP 在客户履约运营台把周期切到 2026-06 后点击“生成月结”，页面显示 `no fees for settlement period`，数据库核对 2026-06 结算批次为 0。
- 操作手册：`OP_MANUAL_CUSTOMER_FULFILLMENT.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已补充没有费用可结算时不会生成 0 元空结算批次，以及出现 `no fees for settlement period` 时检查结算期间和费用导入。

## ERP 客户履约非草稿月结批次不可重复改动边界补证
- `PR-242-CUSTOMER-SETTLEMENT-NON-DRAFT-IMMUTABLE` / `DEV-242-CUSTOMER-SETTLEMENT-NON-DRAFT-IMMUTABLE` 已录入系统：客户履约已确认或已结算的月结批次不能被同周期重复生成改动，避免结算后补录费用篡改历史账单。
- RED 证据：`ORDERAPP_TEST_DATABASE_URL='postgres://yiiiple-work@/kferp_customer_settlement_status?host=/tmp&port=55566&sslmode=disable' go test ./internal/infrastructure/postgres/customerfulfillment -run TestCreateSettlementRejectsNonDraftExistingBatchWithoutChangingFees -count=1 -v` 曾失败，失败点为已有同周期批次被标记 `settled` 后，再次 `CreateSettlement` 返回 `err=<nil>`。
- GREEN 证据：同一真实 PostgreSQL 仓储测试修复后通过；`TestCreateSettlementRejectsNonDraftExistingBatchWithoutChangingFees` 覆盖非 draft 批次返回 `settlement batch is not draft`，原批次总额保持 `10400` 分，补录费用仍为 `unsettled` 且未绑定到已结算批次。
- 一致性证据：`CreateSettlement` 对已存在 `settlement_no` 使用 `FOR UPDATE` 锁定批次并读取状态；只有 `draft` 批次允许复用和并入新增费用，非 draft 在费用更新前返回错误并回滚。
- 回归证据：同库同跑 `TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch`、`TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals`、`TestCreateSettlementAggregatesUnsettledFees`、`TestCreateSettlementRejectsCustomerWithoutSettlementCapability`，确认空周期拒绝、草稿重复月结、正常月结和能力 gate 不受影响。
- UI 点击证据：`CUSTOMER_SETTLEMENT_NON_DRAFT_UI_CLICK_OK app=http://127.0.0.1:18137 pg=55607 evidence=mark_batch_settled_add_extra_fee_then_click_2026_05 response=400 error=settlement_batch_is_not_draft db=batch_settled_104_extra_fee_unsettled_5`；Chrome CDP 对已改为 `settled` 的 2026-05 批次再次点击“生成月结”，页面显示 `settlement batch is not draft`，数据库核对原批次保持 `settled|104.00`，后补 5.00 元费用仍为 `unsettled|settlement_batch_id=0`。
- 操作手册：`OP_MANUAL_CUSTOMER_FULFILLMENT.md` 与 `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 已补充已确认或已结算的月结批次不能通过重复生成同周期月结改动，以及出现 `settlement batch is not draft` 时应撤回/重开草稿或走后续期间调整。

## 完成度审计（当前结论：未完成）
目标拆成以下可交付成功标准：三种模板都要跑通客户档案/账号、模板能力、小程序端、订单生成和查询、生产影响、财务/结算影响、ERP 工作台；同时完成业务逻辑、产品操作、代码结构、测试性、维护性和数据安全优化。按 Van 最新要求，新增进度必须录入 PR/DEV；测试和验收证据按 Superpower/TDD 流程保留，不再强制新增 UT/API/REV 表项。

| 要求 | 当前证据 | 覆盖判断 |
| --- | --- | --- |
| 三种模板的能力边界 | `TestDefaultCapabilityTemplatesRuntimeBusinessContract`、`TestMiniAPITemplateBusinessContract`、PR/DEV/UT/API/REV-176 | 已覆盖应用层和小程序 HTTP API 的允许/拒绝矩阵。 |
| 账号/客户 | `TestThreeTemplateBusinessWalkthroughAcrossModules` 覆盖模板应用和 ERP 绑定；`TestUpsertPortalERPBindingRejectsRetailMallTemplate` 与 `TestUpsertPortalERPBindingRejectsUnknownTemplateKey` 覆盖客户门户配置入口禁止零售或未知模板 ERP 工作台绑定；`TestUpdatePortalVisibilityRejectsUnknownTemplateKey` 与 `TestPortalAdminVisibilityTemplateInvalidMapsToBadRequest` 覆盖客户门户配置保存未知模板 key 返回无效请求且不落库；`TestPortalAdminCustomerResponsesPreserveUnknownTemplateKeyForCorrection` 与 `TestPortalAdminAPIPreservesUnknownTemplateKeyForCorrection` 覆盖客户门户配置读路径保留历史未知模板 key；真实 PostgreSQL `TestUpsertRetailCustomerDeactivatesLegacyERPWorkbenchBinding` 覆盖批发客户切换零售/电商时自动应用 `retail_mall` 并停用历史 ERP 工作台绑定；`TestUpsertPortalERPBindingRejectsDisabledLoginAccount` 与 `TestPortalAdminDetailHidesDisabledLoginERPBinding` 覆盖禁用登录渠道账号不能绑定或展示为有效 ERP 绑定；`customer portal settings excludes disabled channel accounts from ERP binding selector` 覆盖配置页隐藏禁用渠道账号；`customer portal settings preserves unknown template keys for correction` 覆盖配置页保留并显式提示历史未知模板 key；`TestSaveCapabilityTemplateRejectsRetailMallERPWorkbenchFields` 和 `TestUpsertPortalERPBindingRejectsSavedRetailMallTemplateWithERPWorkbench` 覆盖自定义/历史零售模板不能暴露 ERP 工作台；`TestUpsertCustomerERPBindingRejectsTemplateWithoutERPWorkbench` 与 `TestUpsertCustomerERPBindingRejectsUnknownTemplateKey` 覆盖客户履约内部绑定入口不能绕过零售或未知模板工作台限制；`TestUpsertCustomerERPBindingRejectsDisabledLoginAccount` 与 `TestCustomerPortalContextRejectsDisabledLoginBinding` 覆盖客户履约内部绑定和历史上下文拒绝禁用登录账号；`TestCustomerPortalContextRejectsLegacyBindingWithoutERPWorkbench` 与 `TestCustomerPortalContextRejectsLegacyBindingWithUnknownTemplateKey` 覆盖历史 active 绑定不能绕过客户工作台上下文；`TestCustomerOptionsAPISkipsLegacyNonWorkbenchBinding`、`TestCustomerERPWorkbenchAvailableRejectsTemplateWithoutWorkbench` 和 `TestCustomerERPWorkbenchAvailableRejectsUnknownTemplateKey` 覆盖历史 active 绑定不能进入客户履约客户选择器；`CustomerPortalSettingsView.vue` 禁用不支持模板的绑定控件；真实 PostgreSQL `customerportal/customerfulfillment/customer` 包通过；本地真实后端应用三模板并由 Chrome 加载门户配置页；`TestCustomerPortalSubmitRequiresBoundCustomerCapability` 与 `CUSTOMER_ACCOUNT_ISOLATION_SMOKE_OK` 覆盖两个渠道账号隔离；`TestMiniappCurrentCustomerSwitchScopesOrderServicePage` 覆盖同一小程序用户切换当前客户后的会话边界；PR-210 覆盖停用客户旧绑定不再进入小程序当前客户上下文；微信开发者工具 GUI 登录进入 `三模板-代加工履约客户` | 应用层、前端交互、真实服务、持久化 schema、客户账号隔离、小程序当前客户切换隔离、停用客户绑定边界、禁用登录渠道账号绑定边界、客户类型切换停用历史 ERP 绑定、客户门户/客户履约内部绑定边界、历史 ERP 绑定上下文/客户选择器边界、未知模板 fail-closed、配置保存未知模板 invalid、配置 API 未知模板读路径保留、前端未知模板显式提示和零售模板工作台不变式已有证据；微信 GUI 已补三模板当前客户/入口主链路证据。 |
| 订单 | 小程序 API 矩阵覆盖商城单、现货单、代发单、代加工发货单；跨模块走查覆盖订单进入生产/财务视角；商城页补“我的订单”入口；真实 PostgreSQL `sales/customerportal/customerfulfillment/stock` 包通过；本地真实后端已创建代加工发货、现货和商城订单；Chrome DOM 验证订单列表显示 `SO-20260513-PAGE` 和零售客户；Chrome CDP 点击订单号打开订单详情抽屉；微信开发者工具 GUI 点击进入一件代发服务页并看到订单列表，且零售商城 GUI 已提交公开 SKU 订单 `SO-20260514-0006`；`ORDER_STOCK_SHIPMENT_DEDUCTION_UI_CLICK_OK` 覆盖库存待发货订单抽屉回填快递单号扣库存且重复点击幂等；`TestOrderAPIListRejectsInvalidScope` 与 `ORDER_SCOPE_BROWSER_FAIL_CLOSED_OK` 覆盖错误订单范围不能放宽为全量订单；PR-184 真实 PostgreSQL 覆盖公共 SKU 小程序订单非 454g 规格按小批量豆单重量档计价；PR-186 覆盖小程序当前客户切换后“我的订单”不泄露切换前客户订单；PR-187 覆盖现货下单只显示公共商品和当前客户专属商品，且不能手工提交其他客户专属商品 ID；PR-188 覆盖零售商城只展示/上架/销售公共商品，客户专属商品不能进入商城公共目录；PR-190 覆盖代发批次不能为空，订单行数为 0 时不生成待处理批次；PR-195 覆盖销售单收款码上传只接受图片文件，不能上传 HTML 或脚本文件作为收款码，收款码图片超过 8MB 时必须拒绝；PR-201 覆盖销售单收款码标签为空时不写入收款码资产；PR-202 覆盖销售单公章上传元数据失败或设置更新失败时不留下孤儿公章资产；PR-203 覆盖发票文件保存失败时清理刚写入的发票资产文件；PR-204 覆盖销售单 PDF/PNG 生成失败时清理刚写入的销售单文件；PR-205 覆盖出库单 PDF 生成失败时清理刚写入的出库单文件；PR-206 覆盖快递录单 Excel 生成失败时清理刚写入的导出文件；PR-196 覆盖物流单号 Excel 上传超过 20MB 时必须拒绝，不能解析超大物流回传 Excel；PR-197 覆盖客户档案资产图片上传超过 8MB 时必须拒绝；PR-198 覆盖发票文件上传缺失订单时不写入孤儿资产；PR-199 覆盖商城商品图片缺失商品或图片更新失败时不写入/不保留公开孤儿资产；PR-200 覆盖客户档案资产元数据保存失败时清理刚写入文件；PR-217 覆盖履约客户订单范围不会显示历史非工作台模板绑定订单；PR-229 覆盖小程序履约订单后端定价；PR-248 覆盖客户履约同一外部代发订单跨导入批次重传不重复写代发明细或 ERP 订单明细 | 自动化、真实 HTTP、真实页面渲染、订单主干点击、库存待发货订单抽屉回填扣库存且重复点击幂等、微信 GUI 服务页点击、零售商城 GUI 下单和数据库核对、范围参数 fail-closed、公共 SKU 小批量计价、当前客户切换订单隔离、现货商品可见范围隔离、商城公共目录隔离、代发空批次拒绝、履约客户订单范围模板边界、小程序履约订单客户端单价篡改防护、客户履约代发跨批次重传明细幂等、销售单收款码和公章资产防护、发票文件保存失败清理、销售单生成文件失败清理、出库单生成文件失败清理、快递录单 Excel 生成文件失败清理、物流回传 Excel 超限拒绝、客户档案图片超限拒绝、发票文件孤儿资产拒绝、商城图片孤儿资产拒绝和客户资产孤儿文件清理已覆盖。 |
| 订单履约范围登录启用边界 | PR-226、`orderListWhere`、真实 PostgreSQL `TestOrderAPIListFulfillmentScopeSkipsDisabledLoginBinding` | 履约订单范围除工作台模板 gate 外，已追加启用登录渠道账号 gate；禁用登录账号的历史 active 绑定不能让订单继续出现在履约范围。 |
| 客户履约概览有效绑定边界 | PR-227、`requireActiveCustomerERPWorkbenchBinding`、真实 PostgreSQL `TestInternalCustomerFulfillmentOverviewRequiresActiveERPBinding` | 手动 URL/API 指向未绑定客户时，客户履约内部概览不再绕过客户选择器直接展示工作台数据。 |
| 生产 | `TestThreeTemplateBusinessWalkthroughAcrossModules` 覆盖三模板订单进入生产计划并启动生产批次；真实 PostgreSQL `production` 包通过；本地真实 `/api/produce/unproduced?plan=1` 返回 `200`；Chrome DOM 验证生产计划页显示 `E2E商城咖啡豆`；Chrome CDP 点击全选库存不足商品并生成生产计划；追加 Chrome CDP 点击“开始生产”和“完成”后页面显示 `生产已完成`，数据库 `running_done=1`；PR-208 真实 PostgreSQL API 测试和 Chrome CDP 生产中页面点击覆盖多品项订单只完工一个品项时仍保持生产中；PR-230 服务层、真实 PostgreSQL API 测试和 Chrome 页面点击/强制请求覆盖空选择/缺投料开始生产 fail-closed 且不写生产中记录、工单或 WIP 占用；PR-232 真实 PostgreSQL API 测试和 Chrome CDP 生产中页面点击覆盖取消生产同步取消工单/工序卡并释放未消耗 WIP 占用；PR-233 真实 PostgreSQL API 测试和 Chrome CDP 生产中页面点击覆盖完成生产遇冻结 WIP 批次返回 quality block 且完成产物事务回滚；PR-234 真实 PostgreSQL API 测试和 Chrome CDP 生产中页面点击覆盖合并多规格生产单部分完工返回 400 且完成产物事务回滚；PR-236 真实 PostgreSQL API 测试和 Chrome CDP 生产中页面点击覆盖成品克重大于投料时返回 `finished output cannot exceed consumed input` 且完成产物事务回滚；PR-237 真实 PostgreSQL API 测试和 Chrome CDP 仓库 WIP 抽屉点击覆盖 WIP 占用调整成功响应的 `wip_g/available_g` 排除冻结 WIP 批次；PR-238 真实 PostgreSQL API 测试和 Chrome CDP 仓库 WIP 抽屉点击覆盖按 `running_item_id` 释放缺工单行的历史 WIP 占用实际落库；PR-239 真实 PostgreSQL API 测试和 Chrome CDP 仓库 WIP 抽屉点击覆盖 WIP 占用列表排除 inactive 和 depleted 批次；PR-243 真实 PostgreSQL 测试和 Chrome CDP 两标签旧计划点击覆盖陈旧计划或重复点击不能重复开工 | 应用层、生产持久化、真实 API、页面渲染、生成计划点击、开始生产、完工点击、多品项订单部分完工状态边界页面点击、空选择和缺投料 fail-closed 页面/浏览器强制请求证据、陈旧计划重复开工 fail-closed、取消生产释放 WIP 占用页面点击、冻结 WIP 批次完工 fail-closed 页面点击、合并多规格生产部分完工 fail-closed 页面点击、成品产出大于投料 fail-closed 页面点击、WIP 调整可用量质量口径页面点击、running item 释放孤立 WIP 占用页面点击、WIP 列表 active/remaining 可用量口径页面点击均有证据；仍未穷尽全部生产边缘流程。 |
| 财务/结算 | 跨模块走查覆盖收入、生产成本、费用、结算、经营报表和来源钻取；真实 PostgreSQL `finance/costing/customerfulfillment` 包通过；本地真实 `/api/finance/dashboard` 和 `/api/finance/reports/2026-05` 返回 `200`；Chrome DOM 验证财务首页和月度状态可渲染；Chrome CDP 从财务首页点击进入费用管理并保存一条费用；追加 Chrome CDP 点击经营报告来源明细、月度结账和结账后调整，数据库 `adjustments=1`、`monthly_status=adjusted`；PR-185 用真实 PostgreSQL 验证小程序结算服务页只返回当前客户费用明细和结算单；PR-195 覆盖销售单收款码上传只接受图片文件且超过 8MB 拒绝；PR-201 覆盖收款码标签为空时不写入资产；PR-207 覆盖历史订单只有 `total_amount` 且 `grand_total` 为默认 0 时财务收入不漏计、全额折扣 0 元订单仍按 0、作废订单排除；PR-209 覆盖已调整月份重复结账仍保持已调整，不会降回已结账；PR-211 覆盖客户履约账户生成月结必须已开通结算能力；PR-212 覆盖结算导入批次应用前必须已开通结算能力；PR-231 覆盖强锁账月份新增同月票税台账被拒绝，light-confirmation 仍允许，并追加 Chrome CDP 月结后票税台账保存被拒绝 UI 点击证据；PR-235 覆盖未结账月份新增结账后调整返回 400 且不写 adjustment，并追加月结页 draft 月份禁用/强制点击仍拒绝 UI 证据；PR-240 覆盖同一周期客户履约月结重复生成不会把已结费用总额清零；PR-241 覆盖空周期客户履约月结拒绝且不写 0 元批次；PR-242 覆盖非草稿客户履约月结批次不可重复生成改动 | 应用层、关键持久化、真实 API、页面渲染、费用录入、来源明细、结账、结账后调整点击、结算服务页隔离、客户履约结算能力 gate、客户履约导入应用能力 gate、客户履约重复周期月结总额保留、客户履约空周期月结 fail-closed、非草稿月结批次不可重复生成改动、收款码公开资产防护、历史订单收入回退、重复结账状态保留、票税台账强锁账边界和未结账月份调整 fail-closed 均有证据；仍未穷尽全部财务边缘流程。 |
| 小程序端 | `TestMiniAPITemplateBusinessContract` 覆盖小程序 API；`miniapp` 单测、类型检查、`build:mp-weixin` 通过；构建产物确认商城页有“我的订单”；本地真实后端小程序 HTTP smoke 覆盖三模板允许/拒绝和下单；追加 `127.0.0.1:18094` 真实 HTTP 36 项矩阵覆盖 processing/public SKU/retail mall 的登录、当前客户切换、允许入口、禁止入口、代发批次、代加工申请、代发订单、现货订单、商城订单和落库汇总；PR-228 用真实 PostgreSQL 和 miniapp 源码守卫覆盖服务表单选择器，避免客户手填系统 ID；PR-229 用真实 PostgreSQL、API 测试和 miniapp 源码守卫覆盖客户履约订单后端定价，避免客户侧手填或篡改单价；`WECHAT_GUI_LOCAL_API_READY` 覆盖真实 PostgreSQL 下 processing/public SKU/retail mall 当前客户入口模式为 `services/services/mall`；PR-183 追加 `/api/mini/services/:key` 返回 `miniapp_entry_mode` 且服务页前端保留商城入口模式，避免商城客户从“我的订单”退回服务台；PR-186 用真实 PostgreSQL 服务测试覆盖切换当前客户后订单服务页按新客户重新查询；PR-210 用真实 PostgreSQL 与 API 测试覆盖停用客户不能继续作为小程序当前客户或被旧绑定切换进入；PR-190 用 miniapp 源码守卫覆盖代发批次订单行数提交前校验；PR-192 覆盖客户专属豆单 PDF 只能由归属客户访问、官方豆单公共兜底；PR-193/PR-194/PR-199 覆盖商城商品图片上传只接受图片文件、超过 8MB 拒绝且缺失商品不留下孤儿资产；当前分支小程序已导入微信开发者工具，Service Port 已开启；`WECHAT_GUI_CLICK_OK` 覆盖 GUI 登录、processing 首页和一件代发服务页点击；`WECHAT_GUI_PUBLIC_SKU_CLICK_OK` 覆盖公共 SKU GUI 现货下单；`WECHAT_GUI_RETAIL_MALL_CLICK_OK` 覆盖零售商城 GUI 公开 SKU 下单 | API/构建/真实服务、当前客户切换隔离、停用客户绑定边界、空批次前端提示、服务表单选择器、客户履约订单后端定价、豆单 PDF 租户边界和商城图片公开资产类型/大小/缺失商品边界通过；微信开发者工具 GUI 可访问本地 API，并完成 processing 模板登录/服务页点击、public SKU 模板现货下单和 retail mall 模板公开 SKU 下单。 |
| ERP 工作台 | 跨模块走查覆盖 processing/public SKU 进入客户履约工作台；PR-178 禁止 retail mall 工作台绑定；PR-191 禁止自定义或历史零售模板通过 ERP 权限/视图重新暴露工作台；前端禁用不支持模板绑定控件；Chrome headless DOM 看到三模板客户和 retail mall 禁用提示；processing 客户页显示“提交加工工单/提交代发信息”，public SKU 客户页显示“提交代发信息”且不显示“提交加工工单”；Chrome CDP 在 processing 客户工作台点击提交加工工单和代发单，并验证 public SKU 不暴露加工工单；PR-211 固定客户履约月结必须具备结算能力；PR-212 固定客户履约应用导入批次必须具备对应能力；PR-213 固定客户履约手工提交必须具备对应能力；PR-214 固定客户履约托管库存调整必须具备 `inventory_custody` 能力；PR-215 固定客户履约内部 ERP 绑定必须具备暴露 ERP 工作台的模板；PR-216 固定历史 active ERP 工作台绑定也必须具备暴露 ERP 工作台的模板；PR-218 固定客户履约客户选择器也必须具备暴露 ERP 工作台的模板；PR-219 固定非空未知模板 fail-closed；PR-220 固定客户门户配置绑定入口对未知模板 fail-closed；PR-221 固定客户门户配置保存未知模板返回无效请求；PR-222 固定客户门户配置页未知模板不被前端吞掉；PR-223 固定客户门户配置 API 读取未知模板时保留原 key 供页面修正；PR-224 固定批发切零售/电商时历史 ERP 工作台绑定自动停用；PR-225 固定禁用登录渠道账号不能绑定或继续进入 ERP 工作台；PR-227 固定内部概览不能绕过客户选择器载入未绑定客户；服务端 `requireCustomerCapability` 防止绕 UI 提交错模板业务 | 应用/API/UI 源码、真实浏览器 smoke、客户履约主干点击、结算能力 gate、导入应用能力 gate、手工提交能力 gate、托管库存调整能力 gate、内部 ERP 绑定工作台 gate、内部概览有效绑定 gate、历史 ERP 绑定上下文/客户选择器 gate、客户类型切换停用历史 ERP 绑定、禁用登录渠道账号绑定 gate、客户门户配置绑定 fail-closed、客户门户配置保存未知模板 invalid、配置 API 未知模板读路径保留、前端未知模板显式提示、未知模板 fail-closed、服务端越权兜底和零售模板工作台不变式通过；零售商城微信端真实点击已补公开 SKU 下单链路。 |
| 数据安全 | 小程序 token 必需、模板越权 403、零售工作台绑定禁用、零售模板 ERP 工作台字段保存拒绝、客户门户配置和客户履约内部 ERP 绑定不能绕过模板工作台边界、历史 ERP 工作台绑定不能绕过模板工作台上下文和客户选择器边界、未知模板工作台 fail-closed、隐藏角色旧预期清理；客户类型切换为零售/电商会停用历史 ERP 工作台绑定；真实 PostgreSQL 持久化测试覆盖客户账号类型、授权依赖、订单物流迁移；客户履约 portal 提交在服务端校验绑定客户 `processing/direct_ship` 能力；客户履约月结提交在服务端校验客户 `settlement` 能力；客户履约 Excel 应用导入批次按导入类型校验客户 `processing/direct_ship/settlement` 能力；客户履约内部手工提交按目标客户校验 `processing/direct_ship` 能力；客户履约托管库存调整按目标客户校验 `inventory_custody` 能力；客户履约内部 ERP 绑定按目标客户校验模板是否暴露 ERP 工作台且未知模板 fail-closed；客户门户配置 ERP 绑定对未知模板 fail-closed；客户门户配置保存未知模板 key 返回 invalid request；客户门户配置 API 读取历史未知模板 key 时保留原值；客户门户配置页保留并显式提示历史未知模板 key；客户履约工作台上下文和客户选择器按历史 active 绑定目标客户校验模板是否暴露 ERP 工作台；订单列表履约客户范围按历史 active 绑定目标客户校验模板是否暴露 ERP 工作台；Chrome 隔离 smoke 证明两个渠道账号页面和 API 写入不串客户；订单列表非法 `scope` 返回 400，避免错误链接退化成全量订单；结算服务页按当前客户隔离 `customer_fee_items` 和 `customer_settlement_batches`；已调整月份重复结账不会隐藏结账后调整状态；小程序当前客户切换后订单服务页不泄露切换前客户订单；停用客户不能继续作为小程序当前客户或被旧绑定切换进入；现货下单列表和提交订单都按当前客户隔离客户专属商品；客户专属豆单 PDF 只能由归属客户访问，不能下载其他客户专属豆单；商城商品图片上传只接受图片文件，不能把 HTML 或脚本保存到公开商品图片资源，超过 8MB 不能截断后保存为公开商品图片，缺失商品或图片更新失败不能留下公开孤儿资产；销售单收款码上传只接受图片文件，不能上传 HTML 或脚本文件作为收款码，收款码图片超过 8MB 时必须拒绝且标签为空时不能写入资产；销售单公章上传或去除背景失败时必须清理刚写入资产；销售单 PDF/PNG 生成失败时必须清理刚写入文件；出库单 PDF 生成失败时必须清理刚写入文件；快递录单 Excel 生成失败时必须清理刚写入文件；客户档案资产图片上传超过 8MB 时必须拒绝，元数据保存失败必须清理刚写入文件；发票文件上传必须先确认订单存在且不写入孤儿资产，发票文件保存失败时必须清理刚写入文件；物流单号 Excel 上传超过 20MB 时必须拒绝，不能解析超大物流回传 Excel；零售商城公共目录只允许公共商品，历史错误配置不展示也不能下单；代发批次订单行数为 0 时不写入待处理批次 | 关键边界、持久化初始化、多渠道账号隔离、服务端能力兜底、客户履约结算能力 gate、客户履约导入应用能力 gate、客户履约手工提交能力 gate、客户履约托管库存调整能力 gate、客户履约内部 ERP 绑定工作台 gate、历史 ERP 绑定上下文/订单范围/客户选择器 gate、客户类型切换停用历史 ERP 绑定、客户门户配置绑定 fail-closed、客户门户配置保存未知模板 invalid、客户门户配置 API 未知模板读路径保留、前端未知模板显式提示、未知模板工作台 fail-closed、订单范围 fail-closed、结算财务隔离、财务调整状态保留、当前客户订单隔离、停用客户绑定边界、现货商品可见范围隔离、豆单 PDF 租户边界、商城图片/销售单收款码/销售单公章/销售单生成文件/出库单生成文件/快递录单 Excel 生成文件/客户档案图片/发票文件资产边界、物流回传 Excel 超限拒绝、商城公共目录隔离、空代发批次拒绝和零售模板工作台不变式已有自动化证据；微信开发者工具真实会话已补 processing、public SKU 和 retail mall 主链路。 |
| 操作手册和进度表 | `OP_MANUAL_CUSTOMER_PORTAL.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md`、PR/DEV 进度录入至 PR/DEV-271；PR-215 及以前历史 UT/API/REV 表项保留 | 已按新口径更新。 |
| 浏览器/真实环境全流程 | 一次性 PostgreSQL 真实库集成测试已补；本地真实后端、Chrome headless 多页面 DOM smoke 和 Chrome CDP 点击级主干 smoke 已补；追加订单抽屉回填快递单号扣库存、生产开始/完工、财务来源明细、月度结账和结账后调整点击闭环；追加客户履约多账号隔离和服务端越权拒绝 smoke；追加本地后端小程序真实 HTTP 36 项矩阵和落库核对；微信开发者工具 GUI 已导入当前分支小程序，Service Port 已开启，已完成登录、首页、processing 一件代发服务页点击、public SKU 现货下单和 retail mall 公开 SKU 下单；`docker info` daemon 仍不可用 | 未完成，不能把总目标判定为完成。 |

## 后续缺口
- DB/E2E 状态：真实 PostgreSQL DB 集成、本地真实后端/Chrome 多页面 DOM smoke 和 Chrome CDP 订单/生产/财务/客户履约主干点击 smoke 已补证，库存待发货订单抽屉回填快递单号扣库存且重复点击幂等、生产开始/完工、生产开始空选择/缺投料 fail-closed Chrome 页面点击/强制请求、生产开始陈旧计划重复开工 fail-closed、生产取消释放 WIP 页面点击、完成生产遇冻结 WIP 批次 fail-closed 生产中页面点击、合并多规格生产部分完工 fail-closed、成品产出大于投料 fail-closed 生产中页面点击、WIP 占用调整返回可用量质量口径仓库 WIP 抽屉点击、running item 释放孤立 WIP 占用实际落库仓库 WIP 抽屉点击、WIP 列表 active/remaining 可用量口径仓库 WIP 抽屉点击、客户履约重复周期月结总额保留、客户履约空周期月结 fail-closed、客户履约非草稿月结批次不可重复改动、客户履约代发重传运单号更正和空运单清空、客户履约代发订单头更正、客户履约代发发货状态更正、客户履约代加工托管流水数量更正、客户履约结算费用金额更正、客户履约客户 SKU 外部键更正、客户履约库存余额台账更正、财务来源明细、月结、调整点击闭环、票税台账强锁账边界和未结账月份调整 fail-closed 已补证；追加本地后端小程序真实 HTTP 36 项矩阵和落库核对。当前分支小程序已导入微信开发者工具并开启 Service Port，GUI 已完成登录、首页、processing 一件代发服务页点击、public SKU 现货下单和 retail mall 公开 SKU 下单；ERP 全量边缘点击矩阵仍未完成。
- 财务费用新增停用员工经办人 fail-closed 已补 PR/DEV-244：真实 PostgreSQL API 证明停用员工返回 `employee inactive` 且不写 `finance_expenses`，在职员工仍可新增费用；费用管理页 Chrome CDP 点击证明下拉框隐藏停用员工，页面篡改提交仍返回 `employee inactive` 且不落库。
- 财务费用订单/客户/商品维度引用 fail-closed 已补 PR/DEV-245：真实 PostgreSQL API 和费用管理页 Chrome CDP 点击证明缺失维度返回 `finance dimension ... not found` 且不写 `finance_expenses`，真实维度仍可新增费用。
- 财务费用订单客户一致性 fail-closed 已补 PR/DEV-246：真实 PostgreSQL API 和费用管理页 Chrome CDP 点击证明订单归属客户与填写客户不一致时返回 `finance dimension customer does not match order` 且不写 `finance_expenses`。
- 财务费用订单商品一致性 fail-closed 已补 PR/DEV-247：真实 PostgreSQL API 和费用管理页 Chrome CDP 点击证明订单明细不包含填写商品时返回 `finance dimension product does not match order` 且不写 `finance_expenses`。
- 客户履约代发清单跨批次重传幂等已补 PR-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY / DEV-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY：真实 PostgreSQL `TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems` 与客户履约运营台 Chrome CDP 点击证明同一外部代发订单第二个 Excel 批次应用后仍只有 1 张代发订单、2 条代发明细和 2 条 ERP `order_items`，不会从 2 行变 4 行。
- 客户履约代发清单少行修正版重传裁剪已补 PR-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM / DEV-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM：真实 PostgreSQL `TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems` 与客户履约运营台 Chrome CDP 点击证明同一外部代发订单从 2 行修正为 1 行后，代发明细和 ERP `order_items` 都只保留最新 1 行且数量更新为 3。
- 客户履约代发重传运单号更正已补 PR-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC / DEV-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC：真实 PostgreSQL API 与客户履约运营台 Chrome CDP 点击证明同一外部订单从 `SF-OLD-UI-001` 重传为 `SF-NEW-UI-001` 后，订单物流汇总只保留新运单，旧导入运单被裁剪。
- 客户履约代发重传空运单清空已补 PR-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR / DEV-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR：真实 PostgreSQL API 与客户履约运营台 Chrome CDP 点击证明最新 Excel 运单为空时，订单物流汇总清空，旧导入运单被裁剪。
- 客户履约代发订单头跨批次重传更正已补 PR-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION / DEV-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION：真实 PostgreSQL `TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot` 证明同一外部代发订单更正订单日期、收件信息和备注后，ERP 代发订单头显示最新 Excel。
- 客户履约代发发货状态跨批次重传更正已补 PR-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION / DEV-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION：真实 PostgreSQL `TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus` 证明同一外部代发订单发货状态从待发货修正为已发货后，ERP 代发订单发货状态显示最新 Excel。
- 客户履约代加工生产工单拼配投料重传边界已补 PR-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION / DEV-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet` 与客户履约运营台 Chrome CDP 点击证明同一外部生产工单可保留两种生豆投料，修正版 Excel 删除旧生豆后只保留最新投料集合。
- 客户履约库存转换工单单号重传更正已补 PR-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION / DEV-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob` 与客户履约运营台 Chrome CDP 点击证明同一转换单号更正转换前后产品或数量后只保留 1 条最新转换工单。
- 客户履约包装子工单工单编号重传更正已补 PR-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION / DEV-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob` 与客户履约运营台 Chrome CDP 点击证明同一包装子工单编号更正产品、包装耗材或数量后只保留 1 条最新包装子工单。
- 客户履约结算费用名称重传更正已补 PR-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION / DEV-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION：真实 PostgreSQL `TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem` 与客户履约运营台 Chrome CDP 点击证明同一结算费用行更正费用名称后只保留 1 条最新未结费用。
- 客户履约结算费用类型重传更正已补 PR-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION / DEV-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION：真实 PostgreSQL `TestApplySettlementImportReimportCorrectedFeeTypeUpdatesExistingFeeItem` 与客户履约运营台 Chrome CDP 点击证明同一结算费用行更正费用类型后只保留 1 条最新未结费用。
- 客户履约代加工托管流水生豆名称重传更正已补 PR-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION / DEV-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance` 与客户履约运营台 Chrome CDP 点击证明同一 Excel 行号的入库/出库流水从旧生豆更正到新生豆后，旧生豆余额回退为 0，新生豆余额和台账汇总为最新净增减量。
- 客户履约库存余额物料名称重传更正已补 PR-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION / DEV-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportCorrectedCustodyBalanceItemNameMovesLedgerAndBalance` 与客户履约运营台 Chrome CDP 点击证明同一 Excel 行号的生豆库存余额和耗材库存余额更正物料名称后，旧物料余额回退为 0，新物料余额和盘点调整台账以最新 Excel 为准。
- 客户履约代发订单序号重传更正已补 PR-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION / DEV-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION：真实 PostgreSQL `TestApplyDirectShipImportReimportCorrectedSequenceNoUpdatesExistingOrder` 与客户履约运营台 Chrome CDP 点击证明同一外部代发订单更正序号后，代发导入订单、ERP 订单和订单明细仍各只有 1 条并显示最新序号和收件快照。
- 生产管理菜单点击矩阵已补 PR-269-PRODUCTION-MENU-CLICK-MATRIX / DEV-269-PRODUCTION-MENU-CLICK-MATRIX：headless Chrome 点击证明 `workOrders`、`jobCards`、`qualityInspections`、`produceLogs`、`productionCosts` 五个剩余入口能承载数据态和关键操作，marker 为 `PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK`。
- 库存管理菜单点击矩阵已补 PR-270-INVENTORY-MENU-CLICK-MATRIX / DEV-270-INVENTORY-MENU-CLICK-MATRIX：headless Chrome 点击证明 `stockOperations`、`stockOutboundLogs`、`purchase`、`materials` 四个剩余入口能承载数据态和关键操作，marker 为 `INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK`。
- 商品与配方菜单点击矩阵已补 PR-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX / DEV-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX：headless Chrome 点击证明 `productSettings`、`mallSettings`、`costing`、`bom` 四个剩余入口能承载数据态和关键操作，marker 为 `PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK`。
- 客户履约代发清单运单号更正重传同步已补 PR-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC / DEV-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC：真实 PostgreSQL `TestApplyDirectShipImportReimportCorrectedWaybillReplacesImportedTrackings` 证明同一外部代发订单运单号从 `SF-OLD-001` 修正为 `SF-NEW-001` 后，订单只显示最新导入运单号。
- 客户履约代发清单空运单号修正版重传清空已补 PR-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR / DEV-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR：真实 PostgreSQL `TestApplyDirectShipImportReimportBlankWaybillClearsImportedTrackings` 证明同一外部代发订单运单号从 `SF-REMOVE-001` 修正为空后，订单不再显示旧导入运单号。
- 客户履约代加工物料出入库跨批次重传库存幂等已补 PR-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY / DEV-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY：真实 PostgreSQL `TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance` 与客户履约运营台 Chrome CDP 点击证明同一外部生豆入库流水第二个 Excel 批次应用后库存台账仍只有 1 条，托管生豆余额不会从 1500g 重复增加到 3000g。
- 客户履约代加工物料出入库跨批次重传数量更正已补 PR-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION / DEV-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta` 证明同一外部入库/出库流水更正数量后，库存台账仍各只有 1 条，台账增减量更新为最新值，托管余额按差额修正。
- 客户履约代加工库存余额跨批次重传台账更正已补 PR-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION / DEV-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta` 证明同一外部生豆余额和耗材余额更正盘点数后，托管库存余额与台账 delta 汇总都等于最新余额。
- 客户履约结算单费用跨批次重传幂等已补 PR-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY / DEV-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY：真实 PostgreSQL `TestApplySettlementImportReimportSameFeeDoesNotDuplicateFeeItems` 与客户履约运营台 Chrome CDP 点击证明同一外部结算费用行第二个 Excel 批次应用后费用明细仍只有 1 条，费用总额不会从 80 元重复增加到 160 元。
- 客户履约结算单费用跨批次重传金额更正已补 PR-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION / DEV-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION：真实 PostgreSQL `TestApplySettlementImportReimportCorrectedFeeUpdatesUnsettledFeeItem` 证明同一外部未结费用行金额从 8000 分修正为 9500 分后，费用明细仍只有 1 条且总额更新为 9500 分。
- 客户履约代加工 SKU 外部键跨批次重传更正已补 PR-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION / DEV-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION：真实 PostgreSQL `TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct` 证明同一外部 SKU 编码名称/烘焙度修正后仍只有 1 条客户专属商品，工作台 SKU 选项显示最新名称。
- 财务票税台账重复发票号边界已补 PR-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD / DEV-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD：真实 PostgreSQL API `TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger` 证明同类型非空发票号跨月份重复提交返回 `tax ledger invoice already exists`，且不会写第 2 条 `finance_tax_ledger`；Chrome CDP 点击票税台账页输出 `FINANCE_TAX_LEDGER_DUPLICATE_UI_CLICK_OK app=http://127.0.0.1:18131 pg=55601 evidence=save_2026_05_then_duplicate_2026_06 error=tax ledger invoice already exists`。
- 微信开发者工具已安装，Service Port 已开启，CLI/GUI 已能打开当前分支 `miniapp/dist/build/mp-weixin` 项目。`WECHAT_GUI_CLICK_OK app=http://127.0.0.1:18094 pg=55552 project=miniapp/dist/build/mp-weixin path=pages/service/service template=processing_fulfillment evidence=login->home->direct_ship_service no_unit_price_input product_picker_default_price_only`；`WECHAT_GUI_PUBLIC_SKU_CLICK_OK app=http://127.0.0.1:18094 pg=55552 project=miniapp/dist/build/mp-weixin path=pages/home/home->pages/service/service template=public_sku_direct_ship evidence=current_customer_102->home_entries_orders_productOrder_directShip_settlement->product_order_public_sku_only->submit_order SO-20260514-0007 total=127 no_unit_price_input`；`WECHAT_GUI_RETAIL_MALL_CLICK_OK app=http://127.0.0.1:18094 pg=55552 project=miniapp/dist/build/mp-weixin path=pages/mall/mall template=retail_mall evidence=current_customer_103->mall_page->add_public_sku->submit_order SO-20260514-0006 total=127`。
- 已尝试寻找浏览器替代运行时：临时补 `miniapp/index.html` 后，uni H5 build 先暴露 Vue 3.5/uni 5.07 内部 Vue 3.4 依赖不一致；对齐依赖后虽可 build，但 H5 产物只有约 882 bytes 且不包含小程序页面/路由字符串，不能作为三模板小程序点击级证据。该临时 harness 和依赖改动已撤回，避免制造虚假的验收信号。
- 下一轮优先补更多 ERP 边缘点击矩阵；微信开发者工具级三模板主链路已覆盖 processing、public SKU 和 retail mall。

## 2026-05-14 收尾审计
- 收尾边界：Van 指定约两小时后收尾；本轮从 `2026-05-14 17:03 CST` 起不再扩展新功能面，只做证据落盘、口径校准、验证和未完成目标整理。
- 最新完成：PR/DEV-211/212/213/214 已追加真实后端 API + PostgreSQL 证据，覆盖客户履约结算、导入应用、内部手工提交、托管库存调整四类能力缺失时 fail-closed 且不写库；PR/DEV-215 已追加真实后端 API + PostgreSQL 证据，覆盖 `retail_mall` 客户不能通过客户履约内部 ERP 绑定 API 写 active 工作台绑定或隐藏角色。
- 进度表状态：`req_store.go` 当前覆盖三模板审计主线 PR/DEV-175 至 PR/DEV-271；按 Van 最新规则，PR/DEV 用于系统进度查看，UT/API/REV 不再按旧表流程新增，测试和验收证据保留在 Superpower/TDD 证据链。
- 验证状态：PR/DEV-215 定向 support guard、客户履约 API 400 映射测试、support 全量、acceptance marker 覆盖检查、`git diff --check` 和 memory 尾随空白检查已通过；一次性后端 `18158` 与 PostgreSQL `55628` 已停止且端口空闲。
- 未完成工作：不能把总目标标记完成。原因不是三模板主链路缺失，而是 ERP 全量边缘点击矩阵仍未彻底穷尽；现有证据已覆盖微信 GUI 三模板主链路、真实 HTTP/API/PostgreSQL、Chrome DOM/CDP 主干和大量生产/财务/客户履约边界，但仍需要后续继续补更多 ERP 页面边缘点击和最终合并/部署前的全局验证。
- 下一目标：停止新增业务范围，优先整理最终收尾报告、列出剩余 ERP 边缘矩阵缺口，随后进入分支级验证、PR/合并准备和部署前审计。

### 收尾验证命令
- `go test ./...`（`orderapp-remote`）：通过。
- `npm test`（`miniapp`）：通过，9 个测试文件、37 个测试。
- `npm run typecheck`（`miniapp`）：通过。
- `VITE_KFERP_API_BASE=http://127.0.0.1:18094 npm run build:mp-weixin`（`miniapp`）：通过，产物可由微信开发者工具导入 `dist/build/mp-weixin`。
- `node --test src/lib/*.test.js`（`orderapp-remote/frontend-vue-shell`）：通过，124 个测试。
- `npm run build`（`orderapp-remote/frontend-vue-shell`）：通过，仅保留既有 chunk size warning。
- `go test ./internal/interfaces/http/support -run 'TestThreeTemplateBrowserClickSmoke(EvidenceExists|RequirementSeedsExist)' -count=1 -v`：通过，守住浏览器/微信 GUI 证据和剩余 ERP 点击矩阵目标。
- `go test ./internal/interfaces/http/support -count=1`：通过；在 `ERP_MENU_RENDER_MATRIX_SMOKE_OK` marker 和完整 29-view 列表落盘后，于 `2026-05-14 17:52 CST` 重新执行通过。
- `go test ./internal/interfaces/http/support -run 'TestProductionMenuClickMatrix(EvidenceExists|ViewsExposeActions)' -count=1 -v`：通过，守住 PR/DEV-269 生产管理点击矩阵证据和五个 Vue 入口关键动作。
- `go test ./internal/interfaces/http/support -count=1`：在 `PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK` marker 落盘后重新通过。
- `go test ./internal/interfaces/http/support -run 'TestInventoryMenuClickMatrix(EvidenceExists|ViewsExposeActions)' -count=1 -v`：通过，守住 PR/DEV-270 库存管理点击矩阵证据和四个 Vue 入口关键动作。
- `go test ./internal/interfaces/http/support -count=1`：在 `INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK` marker 落盘后重新通过。
- `go test ./internal/interfaces/http/support -run 'TestProductFormulaMenuClickMatrix(EvidenceExists|ViewsExposeActions)' -count=1 -v`：通过，守住 PR/DEV-271 商品与配方点击矩阵证据和四个 Vue 入口关键动作。
- `go test ./internal/interfaces/http/support -count=1`：在 `PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK` marker 落盘后重新通过。
- `comm -23 <(rg -oh '[A-Z][A-Z0-9_]+_OK' docs/acceptance/2026-05-13-three-template-business-audit.md | sort -u) <(rg -oh '[A-Z][A-Z0-9_]+_OK' orderapp-remote/internal/interfaces/http/support | sort -u)`：无输出，验收 `*_OK` marker 均有 support 守卫覆盖。
- `git diff --check`：通过。
- `awk '/[ \t]$/ {print FILENAME ":" FNR ": trailing whitespace"; bad=1} END {exit bad}' /Users/yiiiple-work/Documents/KFerp/memory/2026-05-14.md`：通过。
- 临时资源清理：PR215 本地后端 `18158` 与 PostgreSQL `55628` 均为空闲监听状态。
- 集成基线：PR/DEV-268 收束前 `git fetch origin` 后，`origin/develop` 为 `33ffb240432574723147bcc6fe1b969e30af3a3e`；当前分支已形成本地提交 `30ac035 Audit three customer capability templates` 和 `30fe03a Add production menu click matrix evidence`，随后追加 PR/DEV-270 工作树变更，尚未 push、开 PR、合并或部署。
- 真实浏览器菜单渲染 smoke：`ERP_MENU_RENDER_MATRIX_SMOKE_OK app=http://127.0.0.1:18159/vue-shell mock=auth_api_static views=29 evidence=all_MENU_RENDER_OK cleanup=port_18159_free covered=workOrders,jobCards,qualityInspections,produceLogs,productionCosts,stockOperations,stockOutboundLogs,purchase,materials,productSettings,mallSettings,costing,bom,order,customers,salesOrderSettings,senderSettings,orderInvoice,salesOrder,deliveryNote,financeSettings,customerPortalSettings,customerCapabilityTemplates,companyProfile,machines,userPermissions,employees,departments,audit`。使用已构建 Vue shell 产物、临时 mock auth/API 静态服务和 Chrome headless，逐个打开剩余矩阵全部 29 个 view；页面挂载后对应 API 均收到请求，结束后清理服务且 `18159` 空闲。该 smoke 证明菜单目标是可渲染入口，但不替代后续真实业务数据点击矩阵。
- 生产管理真实点击 smoke：`PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18160/vue-shell mock=auth_api_static chrome_cdp=9239 views=5 actions=status_filter,print,select_work_order,save_quality,batch_operator_filter,refresh quality_rows=2 cleanup=port_18160_free,port_9239_free covered=workOrders,jobCards,qualityInspections,produceLogs,productionCosts`。该 smoke 在真实浏览器里完成生产工单筛选和打印、工序卡筛选、生产质检选择工单并保存、生产日志筛选、生产成本刷新，并确认对应 `/api/produce/*` 请求被触发。
- 库存管理真实点击 smoke：`INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18161/vue-shell mock=auth_api_static chrome_cdp=9240 views=4 actions=tab_switch,filter,open_delivery_note,save_supplier,create_order,receive_order,material_search,stock_backfill receipts=1 materials=1 cleanup=port_18161_free,port_9240_free covered=stockOperations,stockOutboundLogs,purchase,materials`。该 smoke 在真实浏览器里完成库存作业 tab 切换、出库日志筛选并打开出库单抽屉、采购新增/建单/收货、物料档案搜索并库存补录，并确认对应 `/api/stock/*`、`/api/purchase/*`、`/api/materials` 请求被触发。
- 商品与配方真实点击 smoke：`PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK app=http://127.0.0.1:18162/vue-shell mock=auth_api_static chrome_cdp=9241 views=4 actions=create_public_product,save_category,select_customer_sku,save_product_basics,create_mall_product,open_costing_settings,save_costing_run,publish_costing_run,open_bean_list,select_bom_product,sync_bom,save_bom_item,save_bom_version,save_bag_mapping cleanup=port_18162_free,port_9241_free covered=productSettings,mallSettings,costing,bom`。该 smoke 在真实浏览器里完成产品设置、商城管理、价格与豆单、BOM 配方四个入口的数据态和关键点击，并确认对应 `/api/product-settings/*`、`/api/customer-portal/admin/mall-products`、`/api/costing/*`、`/api/bom/*` 请求被触发。

### 剩余 ERP 点击矩阵目标
- 生产管理剩余点击：`workOrders`、`jobCards`、`qualityInspections`、`produceLogs`、`productionCosts` 已补一轮真实浏览器数据态和关键点击 smoke；后续仍需真实后端业务数据下更深的失败态、权限态、导出/打印设备态和跨页回流矩阵。
- 库存管理剩余点击：`warehouseInventory` 已补 WIP 抽屉关键边界，`stockOperations`、`stockOutboundLogs`、`purchase`、`materials` 已补一轮真实浏览器数据态和关键点击 smoke；隐藏追溯页、采购失败态、出库单生成失败态、盘点权限态仍需后续更完整矩阵。
- 商品与配方剩余点击：`productSettings`、`mallSettings`、`costing`、`bom` 已补一轮真实浏览器数据态和关键点击 smoke；后续仍需真实后端业务数据下更深的图片上传失败态、BOM 删除/版本启用确认态、成本 PDF 打印态和客户专属豆单权限态矩阵。
- 订单销售剩余点击：`orders` 已补订单抽屉和发货回填扣库存，仍需继续补 `order` 录单、`customers` 客户档案、`salesOrderSettings`、`senderSettings`、发票/销售单/出库单隐藏页面的更多 UI 失败态。
- 财务剩余点击：`financeDashboard`、`financeExpenses`、`financeClosing`、`financeReport`、`financeTaxLedger` 已有主干和部分 fail-closed UI 证据，仍需补 `financeSettings` 和更多费用/票税/报表筛选导出失败态点击。
- 客户履约剩余点击：运营台导入重传边界已大量覆盖，仍需补 `customerPortalSettings`、`customerCapabilityTemplates` 和未知/禁用账号修复流程的完整页面点击矩阵；当前这些路径主要由应用/API/源码守卫覆盖。
- 系统和设置剩余点击：公司设置、设备产能、权限、员工、部门、操作日志等系统配置入口仍未作为三模板业务主线逐项点击跑完；后续应按“会影响三模板账号/订单/生产/财务的数据配置”优先级补证。
- 集成收尾：完成剩余点击矩阵后，还需要分支 PR、合并 develop、部署前远端 smoke 和部署 notes；当前本轮只做本地分支验证，没有执行合并或部署。
