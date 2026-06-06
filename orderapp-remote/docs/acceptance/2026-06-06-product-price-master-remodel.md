# PR-439-PRODUCT-PRICE-MASTER-REMODEL 商品、分类、价格模型重构验收记录

## 范围
- Product Design brief 已由 Van 在计划中确认：采用现有 KFerp Vue 后台密集表格 + 右侧抽屉，不做营销式页面；字段文案、入口关系、空状态和旧模板兼容提示按计划执行。
- 本轮工程已完成三片：商品档案/客户商品普通新写入不再写旧模板字段；商品档案和客户商品列表展示价格摘要占位；复制为商品档案不复制 BOM、价格或价格表快照；商品价格管理有独立菜单入口，可维护最终价格记录和阶梯价格方案；商品价格表发布快照固化最终价、价格单位、来源价格记录和库存换算；ERP 录单、渠道客户下单和小程序履约订单只按已发布快照取价取单位。
- 旧模板字段不再进入新业务写入；旧表和旧字段保留用于历史兼容、迁移报告和旧数据查询。

## 验收点
- 商品档案普通表单不再出现商品配置模板、报价单位、录单单位、阶梯价模板或利润率覆盖。
- 客户商品普通表单不再出现商品配置模板、报价单位、录单单位、阶梯价模板或价格覆盖。
- 商品档案列表展示库存单位、整数库存和价格摘要；没有已发布价格表快照时显示 `暂无价格表价格`。
- 客户商品列表价格摘要来自客户已发布商品价格表快照；没有快照时显示 `暂无价格表价格`。
- 新增商品和保存商品不再把旧模板字段或商品内价格阶梯写入后端命令。
- 复制为商品档案不复制 BOM、价格、价格表快照或客户商品关系，只复制基础资料和行业字段。
- 商品价格管理维护最终价格记录，字段包含关联商品/客户商品、最终单价、价格单位、币种、价格分组、库存单位、库存换算、状态和备注。
- 阶梯价格方案的每个档位引用一条商品价格记录；保存时服务层从价格记录复制最终价、价格单位和币种，不接受档位提交的二次价格字段。
- 商品价格分组只作为页面组织和筛选，不参与业务取价。
- 商品与配方菜单提供 `商品价格管理` 独立入口；该入口复用 Vue 商品设置页并直接进入价格主数据 pane。
- 商品价格表发布价格档时必须带最终价、价格单位、来源价格记录、库存单位和库存换算；没有换算时禁止发布。
- ERP 录单不再从商品档案、客户商品或旧阶梯模板兜底取价；订单行单位、单价和价格来源来自已发布商品价格表快照。
- 渠道客户下单不再读商品默认价或旧商品阶梯价；订单行、价格来源和结算金额来自已发布商品价格表快照，缺少快照时拒绝提交。
- 小程序履约订单不展示或提交默认价；后端按已发布商品价格表快照写入 ERP 订单行、价格来源和结算数据，缺少快照或库存换算时拒绝提交。

## RED
- `node --test src/lib/product-settings.test.js`：旧前端 helper 仍提交 `product_config_template_id`，普通页面缺少价格摘要/库存字段，旧断言仍要求利润率覆盖。
- `go test ./internal/interfaces/http/catalog -run 'TestCustomerProductAliasAPIsListSaveAndDisableCustomerNames|TestProductSettingsAPIUpdatesProductIndustryFieldsWithoutLegacyTemplateWrites|TestProductSettingsAPICreatesPublicProduct'`：旧 API 命令仍传递商品配置模板、梯度模板、单位规则和价格阶梯。
- `go test ./internal/infrastructure/postgres/catalog -run TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates`：旧复制逻辑仍复制 `product_config_template_id`、`product_price_tiers`、生产配置和 BOM 绑定。
- `go test ./internal/interfaces/http/support -run TestDev439ProductPriceMasterRemodel -count=1`：PR-439 种子和文档尚未登记。
- `go test ./internal/application/catalog -run 'TestProductPriceRecordIsFinalPriceMasterData|TestProductTierPriceSchemeCopiesFinalPriceRecordsWithoutRecalculation' -count=1`：应用层缺少商品价格记录和阶梯价格方案类型。
- `go test ./internal/interfaces/http/catalog -run 'TestProductPriceRecordAPISavesFinalPriceMasterData|TestProductTierPriceSchemeAPIReferencesFinalPriceRecords' -count=1`：HTTP 层缺少商品价格管理 API。
- `go test ./internal/infrastructure/postgres/catalog -run TestProductPriceMasterSchemaPersistsFinalRecordsAndReferenceSchemes -count=1`：postgres schema/repository 缺少价格主数据表和保存方法。
- `node --test src/lib/product-settings.test.js`：前端缺少商品价格管理 helper 和面板。
- `go test ./internal/application/costing -run TestPublishBeanListRequiresFinalPriceSnapshotOnPriceTiers -count=1`：商品价格表发布尚未强校验价格档最终价、价格单位和库存换算快照。
- `go test ./internal/infrastructure/postgres/orderbeans -run TestPublishedPricingCarriesFinalPriceSnapshotMetadata -count=1`：已发布价格档解析尚未携带来源价格记录和库存换算元数据。
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormProductsUsePublishedPriceSnapshotsOnly|TestOrderSaveRequiresPublishedPriceSnapshotInsteadOfLegacyTierFallback' -count=1`：ERP 录单仍可能从商品默认价或旧阶梯价兜底。
- `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestCustomerDirectShipPricingUsesPublishedSnapshotsOnly|TestSubmitCustomerDirectShipOrderRejectsLegacyPriceFallbackWithoutSnapshot' -count=1`：渠道客户下单仍可能读取旧商品阶梯价或商品默认价。
- `go test ./internal/infrastructure/postgres/customerportal -run TestCustomerPortalFulfillmentPricingUsesPublishedSnapshotsOnly -count=1`：小程序履约仍可能读取旧商品阶梯价或旧默认价。
- `npm test -- src/utils/mall.test.ts src/utils/servicePage.test.ts`：小程序服务页仍展示默认价格文案。

## GREEN
- `node --test src/lib/product-settings.test.js`：117/117 通过。
- `go test ./internal/interfaces/http/catalog -run 'TestCustomerProductAliasAPIsListSaveAndDisableCustomerNames|TestProductSettingsAPIUpdatesProductIndustryFieldsWithoutLegacyTemplateWrites|TestProductSettingsAPICreatesPublicProduct'`：通过。
- `go test ./internal/infrastructure/postgres/catalog -run TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates`：通过。
- `go test ./internal/application/catalog -run 'TestProductPriceRecordIsFinalPriceMasterData|TestProductTierPriceSchemeCopiesFinalPriceRecordsWithoutRecalculation' -count=1`：通过。
- `go test ./internal/interfaces/http/catalog -run 'TestProductPriceRecordAPISavesFinalPriceMasterData|TestProductTierPriceSchemeAPIReferencesFinalPriceRecords' -count=1`：通过。
- `go test ./internal/infrastructure/postgres/catalog -run TestProductPriceMasterSchemaPersistsFinalRecordsAndReferenceSchemes -count=1`：通过。
- `go test ./internal/interfaces/http/support -run TestDev439ProductPriceMasterRemodel -count=1`：通过。
- `go test ./internal/interfaces/http/catalog`、`go test ./internal/infrastructure/postgres/catalog`、`go test ./internal/interfaces/http/support`：通过。
- `node --test src/lib/product-settings.test.js`、`npm run build`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- 浏览器 smoke：本地 Vue shell 使用 mocked API 打开 `http://127.0.0.1:5173/vue-shell/?view=productSettings`，商品档案页可见 `商品档案只维护商品资料`、`价格摘要`、`库存单位`、`整数库存`、`暂无价格表价格`；不可见 `利润率覆盖` 和客户商品配置模板字段；无 console error。
- 浏览器 smoke：本地 Vue shell 使用 mocked API 打开 `http://127.0.0.1:5173/vue-shell/?view=productPriceManagement`，菜单高亮 `商品价格管理`，页面可见 `商品价格记录`、`常规批发 · CNY 88/kg`、`阶梯价格方案`、`引用价格记录`；价格管理 pane 内不可见 `利润率` 或 `成本加成`；截图 `/tmp/pr439-product-price-management-smoke.png`。
- `go test ./internal/application/costing -run TestPublishBeanListRequiresFinalPriceSnapshotOnPriceTiers -count=1`：通过。
- `go test ./internal/infrastructure/postgres/orderbeans -run TestPublishedPricingCarriesFinalPriceSnapshotMetadata -count=1`：通过。
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormProductsUsePublishedPriceSnapshotsOnly|TestOrderSaveRequiresPublishedPriceSnapshotInsteadOfLegacyTierFallback' -count=1`：通过。
- `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestCustomerDirectShipPricingUsesPublishedSnapshotsOnly|TestCustomerFulfillmentPublishedPriceUnitTotals|TestSubmitCustomerDirectShipOrderUsesPublishedPriceSnapshot|TestSubmitCustomerDirectShipOrderRejectsLegacyPriceFallbackWithoutSnapshot' -count=1`：通过。
- `go test ./internal/infrastructure/postgres/customerportal -run 'TestCustomerPortalFulfillmentPricingUsesPublishedSnapshotsOnly|TestCreateFulfillmentOrder|TestPortalMallLinePricingUsesBagQuoteForDripBoxOrders' -count=1`：通过。
- `npm test -- src/utils/mall.test.ts src/utils/servicePage.test.ts`：20/20 通过。

## 待后续验收
- 商品分类模板物理入口下线和分类树独立页面。
- development 环境部署后的浏览器业务验收：新增商品分类和价格表、ERP 下单到账单、渠道客户下单到结算、小程序价格/结算与 ERP 一致。
