# PR-411 客户商品配置模板与计价方式收敛验收记录

## 范围
- 客户商品名用户侧更名为客户商品。
- 客户商品不再直接维护阶梯价模板和单位模板，只选择商品配置模板；留空时继承绑定商品档案的商品配置模板。
- 商品配置模板只在计价方式为“按阶梯价模板”时展示并保存阶梯价模板；固定单价和成本加成会清空阶梯价模板。
- 客户范围产品价格表取价顺序调整为：客户商品配置模板 → 商品档案配置模板 → 旧直接字段兼容 fallback。

## RED 证据
- `node --test src/lib/product-settings.test.js`：缺少 `productConfigTemplateNeedsGradientTemplate` 时失败。
- `go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/support -run 'TestCustomerProductAlias|TestLoadProductInputs|TestDev411|TestProductSettingsAPI' -count=1`：客户商品缺少 `product_config_template_id`、价格表 SQL 仍优先旧直接字段、PR-411 种子缺失时失败。

## GREEN 证据
- `node --test src/lib/product-settings.test.js`：108/108 通过。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/view-routing.test.js`：149/149 通过。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`：通过。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 待补部署证据
- feature branch push
- merge to `develop`
- development stack deploy
- smoke：`/app/vue-shell`、`/app/api/product-settings`、`/app/api/customer-product-aliases?active=all&q=`、requirements API PR-411
