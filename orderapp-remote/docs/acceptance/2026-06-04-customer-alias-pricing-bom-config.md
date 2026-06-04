# PR-409 客户商品名报价覆盖与商品档案 BOM 绑定修正验收记录

## 范围
- PR：`PR-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG`
- DEV：
  - `DEV-409-CUSTOMER-ALIAS-PRICING-UNIT`
  - `DEV-409-PRICE-LIST-ALIAS-OVERRIDE`
  - `DEV-409-PRODUCT-BOM-SELECTOR`
  - `DEV-409-INDUSTRY-FIELD-LEGACY-SAVE`
  - `DEV-409-MANUAL-DOCS`

## 验收点
- 客户商品名可维护客户侧阶梯价模板和单位模板；列表展示当前覆盖状态。
- 客户范围商品价格表优先读取客户商品名覆盖的阶梯价模板和单位模板。
- 商品档案配置抽屉绑定生产 BOM 时只展示启用 BOM，支持模糊搜索并显示版本号。
- 修改商品档案生产 BOM 绑定时，旧产品信息字段没有行业字段模板不会再导致保存失败。

## RED Evidence
- `node --test src/lib/product-settings.test.js`：新增 BOM 搜索/有效状态/版本号断言后，旧页面仍使用普通 `select production_bom_id`，测试失败。
- `go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing -count=1`：客户商品名模板字段和价格表读取客户覆盖字段缺失，源码守卫失败。

## GREEN Evidence
- `node --test src/lib/product-settings.test.js`：通过。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing -count=1`：通过。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`：通过。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`：通过。
- `npm run build`：通过，保留既有 chunk-size warning。
- `scripts/verify_kferp.sh changed`：通过。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 备注
- 本轮按当前约定不做浏览器/人工验收。
- Development smoke：容器运行；未认证 GET `/app/` 返回 303；认证 `/app/vue-shell` 返回 200；需求 API 暴露 `PR-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG`；认证 `/app/api/product-settings`、`/app/api/customer-product-aliases?active=all&q=`、`/app/api/production-boms?status=all` 均返回 200。
