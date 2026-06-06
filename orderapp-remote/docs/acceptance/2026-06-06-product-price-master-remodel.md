# PR-439-PRODUCT-PRICE-MASTER-REMODEL 商品、分类、价格模型重构验收记录

## 范围
- Product Design brief 已由 Van 在计划中确认：采用现有 KFerp Vue 后台密集表格 + 右侧抽屉，不做营销式页面；字段文案、入口关系、空状态和旧模板兼容提示按计划执行。
- 本轮工程已完成三片：商品档案/客户商品普通新写入不再写旧模板字段；商品档案和客户商品列表展示价格摘要占位；复制为商品档案不复制 BOM、价格或价格表快照；商品分类管理和商品价格管理有独立菜单入口；旧商品配置模板、阶梯价模板、单位模板不再出现在普通主菜单和客户工作区；商品价格管理可维护最终价格记录和阶梯价格方案；商品价格表发布快照固化最终价、价格单位、来源价格记录和库存换算；ERP 录单、渠道客户下单和小程序履约订单只按已发布快照取价取单位。
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
- 商品与配方菜单提供 `商品分类管理` 和 `商品价格管理` 独立入口；价格入口复用 Vue 商品设置页并直接进入价格主数据 pane。
- 普通主菜单和客户工作区不再展示 `商品配置和分类模板`、`阶梯价模板`、`单位模板` 旧入口；旧 view key 只保留为历史直达链接兼容。
- 商品价格表发布价格档时必须带最终价、价格单位、来源价格记录、库存单位和库存换算；没有换算时禁止发布。
- 商品价格表候选和已发布版本入口按商品档案/客户商品直接分类生成商品类型；旧 `product_type_category_id/product_type_name` 仍只作为历史兼容字段，不把已有直接分类的商品打到 `未分类商品`。
- 商品价格表预览把商品价格管理里的最终价记录和阶梯价格方案投影成可发布价格档；已有最终价记录的商品不再显示旧“未设置计价方式，请到商品配置模板”提示。
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
- `go test ./internal/infrastructure/postgres/customerportal -run TestCustomerPortalFulfillmentOrderLineUnitComesFromPublishedPriceUnit -count=1`：部署验收中小程序订单 `1524 / SO-20260606-0022` 已按发布快照计算 `88.5/kg` 和 `177.00`，但订单行 `unit` 仍写成旧展示单位 `件`。
- `npm test -- src/utils/mall.test.ts src/utils/servicePage.test.ts`：小程序服务页仍展示默认价格文案。
- 浏览器 follow-up：已部署 商品价格表 页面仍把 PR-439 商品 `538/539` 放在 `未分类商品`，已发布版本区显示 `暂无`，预览卡片仍提示到旧 `商品配置和分类模板` 设置计价方式。
- `node --test src/lib/product-price-list-types.test.js`：前端价格表类型 helper 只认旧 `classification_template_id/name`，未把直接 `product_category_id` 投影为当前价格表分类。
- `go test ./internal/domain/costing -run TestProductPriceSnapshotsPublishCommercialTiersWithoutLegacyTemplate -count=1`：领域层已有最终价快照时仍不生成可发布价格档，并继续报旧缺计价方式 warning。
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsReadsFinalPriceTierSchemes -count=1`：costing 查询尚未把阶梯价格方案及来源最终价记录投影到价格表快照。
- 浏览器 follow-up 2：已部署 商品价格表 已能按 `咖啡烘焙豆` 展示商品和最终价档，但 PR-439 历史发布版本仍是 `product_type_category_id=0`，按直接分类查看时已发布版本区仍显示 `当前发布 暂无`。
- `go test ./internal/infrastructure/postgres/costing -run TestBeanListPublicationQueriesFallbackToLegacyListTypeRows -count=1`：发布版本查询尚未在按直接分类查找时兼容历史 `list_type=commercial` 发布行。
- 浏览器 follow-up 3：后端兼容返回历史 `product_type_category_id=0` 发布行后，前端发布版本列表仍按当前分类二次过滤掉该行。
- `node --test src/lib/product-price-list-types.test.js`：历史全局 `commercial` 发布行尚不能作为直接分类下的兼容版本展示。
- 浏览器 follow-up 4：公共 `PR439-20260606182321-OFFICIAL` 版本进入 `咖啡烘焙豆` 筛选后，版本表“类型”列仍显示历史 `未分类商品`。
- `node --test src/lib/costing-bean-list-version-ui.test.js`：兼容历史全局发布行尚未在当前筛选下显示当前商品类型名称。
- 现场 follow-up 5：商品 `539` 的 `product_config_template_id` 清为 0 后，部署/重启又从分类 `7` 的历史 `product_config_template_id=4` 回填，说明启动迁移仍把旧分类模板字段写回商品档案。
- `go test ./internal/infrastructure/postgres/catalog -run TestProductConfigTemplateDoesNotBackfillFromCategoryOnStartup -count=1`：实现前失败于 `schema.go` 仍存在 `SET product_config_template_id=COALESCE(pc.product_config_template_id,0)`。
- 浏览器 follow-up 6：商品档案内容和价格摘要正确，但左侧商品与配方主菜单仍展示 `商品配置和分类模板`、`阶梯价模板`、`单位模板` 旧入口。
- `node --test src/lib/menu-ia.test.js src/lib/workspace-mode.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js`：实现前失败于主菜单和客户工作区仍把旧模板入口作为普通页面。

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
- `go test ./internal/infrastructure/postgres/customerportal -run TestCustomerPortalFulfillmentOrderLineUnitComesFromPublishedPriceUnit -count=1`：通过。
- `go test ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal ./internal/application/customerportal -count=1`：通过。
- `go test ./internal/infrastructure/postgres/customerfulfillment ./internal/interfaces/http/customerfulfillment ./internal/application/customerfulfillment -count=1`：通过。
- `go test ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales ./internal/application/sales -count=1`：通过。
- `go test ./...`：通过。
- `node --test src/lib/product-settings.test.js`：118/118 通过。
- `npm run build` in `frontend-vue-shell`：通过，仅保留既有 chunk-size warning。
- `npm test -- src/utils/mall.test.ts src/utils/servicePage.test.ts`、`npm run typecheck`、`npm run build:mp-weixin` in `miniapp`：通过。
- `scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- follow-up：`node --test src/lib/product-price-list-types.test.js`：通过。
- follow-up：`node --test src/lib/product-bean-list-split.test.js src/lib/bean-list-pdf.test.js src/lib/product-price-list-types.test.js`：47/47 通过。
- follow-up：`go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing -count=1`：通过。
- follow-up 2：`go test ./internal/infrastructure/postgres/costing ./internal/application/costing ./internal/interfaces/http/costing -count=1`：通过。
- follow-up 2：`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- follow-up 3：`node --test src/lib/product-price-list-types.test.js`：6/6 通过。
- follow-up 3：`node --test src/lib/product-bean-list-split.test.js src/lib/bean-list-pdf.test.js src/lib/product-price-list-types.test.js`：47/47 通过。
- follow-up 3：`npm run build` in `frontend-vue-shell`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- follow-up 4：`node --test src/lib/costing-bean-list-version-ui.test.js`：13/13 通过。
- follow-up 4：`node --test src/lib/product-bean-list-split.test.js src/lib/bean-list-pdf.test.js src/lib/product-price-list-types.test.js src/lib/costing-bean-list-version-ui.test.js`：60/60 通过。
- follow-up 4：`npm run build` in `frontend-vue-shell`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- follow-up 5：`go test ./internal/infrastructure/postgres/catalog -run 'TestProductConfigTemplateDoesNotBackfillFromCategoryOnStartup|TestUpdateProductBasicsClearsLegacyTemplateColumns|TestProductConfigOverridesRemainReadableButProductUpdateDoesNotWrite' -count=1`：通过。
- follow-up 5：`go test ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -count=1`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：通过。
- follow-up 6：`node --test src/lib/menu-ia.test.js src/lib/workspace-mode.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js`：155/155 通过。
- follow-up 6：`node --test src/lib/menu-ia.test.js src/lib/workspace-mode.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js src/lib/product-price-list-types.test.js src/lib/costing-bean-list-version-ui.test.js`：174/174 通过。
- follow-up 6：`npm run build` in `frontend-vue-shell`、`go test ./...`、`scripts/verify_kferp.sh changed`：通过。

## Development 部署与现场验收
- 代码部署基线：`3cfe484e851ae91552ce73cfe5dc3f6667de90ef`。
- 部署备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607033448`。
- 部署脚本证据：Vue shell build、小程序 `typecheck` / `build:mp-weixin`、Docker build、镜像内 `go test ./...` 均完成；`erp_orderapp` 重新创建并启动。
- Smoke：`docker compose ps` 显示 `erp_orderapp` Up、`erp_postgres` healthy；`https://erp.qacoohee.com/app/` 返回 303；部署文档中可检索 `PR-439-PRODUCT-PRICE-MASTER-REMODEL`。
- 商品档案 API：商品 `538`、`539` 的 `product_config_template_id=0`，价格摘要分别来自官方价格表 `57 / PR439-20260606182321-OFFICIAL`；`538` 为 `88.5/kg · 1kg+`，`539` 为 `39.9/lb · 2lb+`。
- 客户商品 API：客户商品 `82`（Karen）和 `83`（渠道）旧模板字段均为 0，价格摘要分别来自 `56 / PR439-20260606182321-KAREN` 和 `55 / PR439-20260606182321-CHANNEL`。
- 商品价格管理 API：价格记录 `1` 为 `88.5/kg`、库存单位 `kg`、换算 `{"kg":{"kg":1}}`；价格记录 `2` 为 `39.9/lb`、库存单位 `kg`、换算 `{"lb":{"kg":0.454}}`。
- ERP 录单验收：`1523 / SO-20260607-0001`，客户 `19`，商品 `538`，数量 `2kg`，单价 `88.50`，金额 `177.00`，价格来源为 Karen 发布快照 `56` 和来源价格记录 `1`。
- 小程序履约验收：`1525 / SO-20260606-0023`，客户 `122`，服务 `direct_ship`，商品 `538`，数量 `2`，订单行 `unit=kg`、`spec=1000g`、`unit_price=88.50`、`line_total=177.00`、`bean_list_publication_id=55`、`source_price_record_id=1`、库存换算 `{"kg":{"kg":1}}`；小程序订单页和结算页按订单号均返回 `177.00`。
- 操作日志：商品 `539` 通过 `PUT /api/products/539` 清理旧模板字段，操作日志 `4738` 记录 `product update / product_basics`。
- follow-up 最终部署：`origin/develop=22577913175a4f76ce387cc6b7d1a8900cd5ccdf` 已部署到 development；备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607043416`。部署脚本完成 Vue build、小程序 `typecheck` / `build:mp-weixin`、Docker build 和镜像内 `go test ./...`。
- follow-up smoke：`erp_orderapp` Up，`erp_postgres` healthy，`https://erp.qacoohee.com/app/` 返回 303；服务器验收文档可检索 follow-up 4 证据。
- follow-up 浏览器验收：商品价格表加载 `index-B7GjMOL8.js`；公共豆单显示 `咖啡烘焙豆（2款）`、当前发布 `PR439-20260606182321-OFFICIAL`，版本表类型列显示 `咖啡烘焙豆`，无旧 `未设置计价方式` 提示；客户范围分别显示 `PR439-20260606182321-KAREN` 和 `PR439-20260606182321-CHANNEL`。
- follow-up 商品档案验收：商品档案页面显示商品 `538/539` 归类为 `咖啡烘焙豆 / 精品意式拼配`、`咖啡烘焙豆 / 工厂量单`，价格摘要来自官方快照 `57`；数据库确认 `538/539` 的 `product_config_template_id=0`、`classification_template_id=0`、`gradient_template_id_override=0`。
- follow-up 客户商品/渠道验收：客户商品 `82/83` 旧模板字段均为 0，分别进入 Karen 和渠道客户价格表；ERP 订单 `1523` 和渠道/小程序履约订单 `1525` 均为 `unit=kg`、`unit_price=88.50`、`line_total=177.00`、来源价格记录 `1`，订单行价格表快照分别为 `56 / PR439-...-KAREN` 和 `55 / PR439-...-CHANNEL`。
- follow-up 页面验收：客户履约运营台和客户履约工作台均显示 `SO-20260606-0023` 与 `177.00`；客户履约运营台显示客户豆单 `PR439-20260606182321-CHANNEL`。
- follow-up 数据修复验收：启动迁移回填已删除后，商品 `539` 的历史 `product_config_template_id` 再次从 `4` 清理为 `0`，操作日志 `4740` 记录 `4 -> 0`；随后重启 `erp_orderapp`，DB 仍显示商品 `538/539` 的 `product_config_template_id=0`、`classification_template_id=0`、`gradient_template_id_override=0`，即使分类 `7` 仍保留历史 `product_config_template_id=4` 也不会回填商品。

## 兼容说明
- 旧商品配置模板、单位模板、阶梯价模板和分类模板仍作为历史兼容、迁移排查或直达 URL 兼容入口保留；普通主菜单、客户工作区、商品档案、客户商品和新录单取价不再从这些字段写入或决定价格/单位。
- PR-439 前后过渡期内，历史已发布商品价格表可能仍保存为 `product_type_category_id=0`。新页面按商品直接分类查看时，系统优先读取同分类发布版本；没有同分类版本时，仍兼容读取同归属、同用途的历史 `commercial` 已发布版本。
