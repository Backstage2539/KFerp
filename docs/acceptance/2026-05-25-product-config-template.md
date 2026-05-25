# PR-362 Product Config Template Acceptance

## Scope

- SKU设置 uses one visible template concept: 商品配置.
- 商品配置 includes阶梯价模板、工序模板、价格表生成规则、库存单位、报价单位、录单单位、单位换算、整数单位.
- 产品子类型 binds a 商品配置 template; it no longer exposes raw JSON or a price-list inclusion checkbox.
- Public/customer copies are copy-on-write: copying a public subtype copies/binds the source 商品配置 for that customer.
- Public category visibility and public SKU visibility remain independent switches.

## Acceptance Scenario

1. Open SKU设置 and keep SKU归属 as 公共SKU.
2. In 商品配置, create `盒装速溶配置`.
3. Select a阶梯价模板, set 工序模板ID as needed, set price generation fields by dropdowns, set 库存单位 `kg`, 报价单位 `盒`, 录单单位 `盒`, add conversion `1 盒 = 0.2 kg`, enable 整数单位, then save.
4. In 商品分类, create or open `速溶咖啡 / 冻干速溶`, select `盒装速溶配置` directly from the 商品配置 dropdown on the subtype row.
5. Select a customer, enable only `是否使用公共商品分类`, keep `是否使用公共SKU` off. The category tree shows public product types/subtypes but no public SKU chips.
6. Click `复制为客户分类` on `冻干速溶`. The derived customer subtype keeps a bound customer 商品配置 copied from the public template.
7. Product price table inclusion is controlled on 产品价格表, not by 商品配置.

## Evidence

- Backend targeted tests: `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -run 'ProductConfig|ProductSettingsAPI' -count=1`
- Frontend targeted tests: `node --test src/lib/product-settings.test.js`
- Vue build: `npm run build`
