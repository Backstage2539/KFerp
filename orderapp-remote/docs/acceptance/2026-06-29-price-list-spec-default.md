# PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS 价格表规格显示、默认规格和行级错误

## 目标
- 产品价格表平铺价格行能看出具体销售规格，不再只显示 `/袋` 这类包装单位。
- 销售规格模板支持设置默认规格；保存后默认销售单位使用选中规格名称。
- 价格表发布校验错误定位到具体平铺价格行，不再在预览顶部只显示笼统提示。

## 验收场景
- `熟豆-白巧坚果拼配` 这类父商品下存在 `227g袋装`、`100g袋装` 等子 SKU 时，平铺价格行标题展示具体子 SKU，例如 `熟豆-白巧坚果拼配（227g袋装）`；价格输入框单位位置显示 `/227g` 这类规格标签，不再只显示 `/袋`。
- 价格行缺计价模式、阶梯档位、价格计算模板、最终价、价格单位、库存换算、分组快照或成本来源快照时，对应行下方直接展示行级错误。
- 进入 SKU 设置的销售规格模板明细，点击某行 `默认规格` 后保存；API 返回该规格作为 `default_sales_unit/sales_unit/quote_unit/order_unit`，每条 `sales_specs.sales_unit` 使用规格名称。

## 证据
- RED：新增前端 helper/source tests 时，缺少 `priceListFlatRowDisplayTitle`、`priceListFlatRowErrors`、行级错误列表和默认规格控件；新增 Go service/API tests 时，后端仍把 `袋` 保存为默认销售单位。
- RED follow-up：首次部署后浏览器烟测发现成本价格表中 `榛巧拼配` 仍展示 `/袋`，并提示缺少 `袋 -> g` 换算；补充回归测试证明成本仓储未读取销售规格模板默认 `sales_specs_json`，且 PDF 分组丢失 SKU/单位字段。
- GREEN targeted：
  - `node --test src/lib/costing-price-list-workflow.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/product-settings.test.js`
  - `node --test src/lib/costing-price-list-workflow.test.js src/lib/bean-list-pdf.test.js`
  - `go test ./internal/application/catalog -run 'TestServiceSavesSalesSpecTemplateWithoutInventoryConversion|TestServiceSavesSelectedDefaultSalesSpecTemplate' -count=1`
  - `go test ./internal/interfaces/http/catalog -run TestProductSettingsAPISavesSalesSpecTemplateContract -count=1`
  - `go test ./internal/infrastructure/postgres/costing -run TestProductSalesUnitResolversPreferProductDirectUnitTemplateBeforeLegacyTemplateChain -count=1`
  - `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`
  - `go test ./internal/interfaces/http/support -count=1`
- 手册：`orderapp-remote/docs/OP_MANUAL_COSTING.md`；`orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`。

## 状态
- 已合并 `develop` 并部署到 development。最终部署 commit `895746dc7f5183ac84c49fd3791166f3d86a0bcd`；服务器 smoke 确认 `榛巧拼配` 默认规格解析为 `227g袋装 -> kg 0.227`。
