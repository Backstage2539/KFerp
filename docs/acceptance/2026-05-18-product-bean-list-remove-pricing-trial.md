# 2026-05-18 产品豆单移除价格试算验收

## 范围
- 产品豆单页面不再展示价格试算工作区。
- 产品豆单仍保留豆单预览、生成豆单、豆单版本列表和客户上下文。
- SKU设置继续只维护 SKU、商品分类和梯度模板。

## 验收点
- [x] 产品豆单源码和浏览器界面均不再出现“价格试算”“保存试算”“发布价格”“试算批次”。
- [x] 产品豆单仍展示“豆单版本列表”，可看到已发布版本并进入生成豆单流程。
- [x] SKU设置不嵌入产品豆单工作区，不显示豆单版本列表。
- [x] 操作手册、REQUIREMENTS、ACCEPTANCE_TESTS 和 PR/DEV 种子均同步为产品豆单只承接豆单预览、版本和发布。

## 验证证据
- `node --test src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/menu-ia.test.js src/lib/operation-manuals.test.js`
- `go test ./internal/interfaces/http/support -count=1`
- `go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPI|TestBeanListPublicationAPISupportsCustomerScope|TestCostingPriceExplanationAPI' -count=1`
- `npm run build`
- Browser smoke：`productSettings.hasPricingTrial=false`、`productSettings.hasVersionList=false`、`costing.hasVersionList=true`、`costing.hasPublishedVersion=true`、`costing.hasPricingTrial=false`、`costing.hasSaveTrial=false`、`costing.hasPublishPrice=false`。
