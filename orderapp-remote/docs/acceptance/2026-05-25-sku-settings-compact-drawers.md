# PR-364 SKU设置紧凑抽屉 Acceptance

## Scope
- SKU设置商品资料工作区压缩为商品分类滚动窗、分类编辑抽屉和新增 SKU 抽屉。
- 商品配置工作区内再拆成“商品配置模板”和“阶梯价模板”页签。
- 本次只调整 SKU设置页面组织方式，不新增后端接口，不改变商品分类、SKU、商品配置模板或产品价格表生成的数据契约。

## Acceptance
1. 进入 SKU设置 后默认停留在“商品资料”。
2. “商品分类”区域在固定高度窗口内滚动，页面不会因为分类和 SKU 太多继续拉长。
3. 商品分类搜索框可按产品类型、产品子类型、SKU 名称、SKU 编号和备注过滤分类树。
4. 点击“编辑产品类型”打开抽屉，可新增一级产品类型，也可在抽屉中改名、删除产品类型和产品子类型，并给产品类型新增产品子类型。
5. “新增SKU”入口位于客户SKU列表右上角，点击后打开抽屉；公共上下文创建公共 SKU，客户上下文创建客户专属 SKU。
6. “商品资料 / 商品配置”页签中，第二个页签显示为“商品配置”，不再显示“模板配置”。
7. 进入“商品配置”后默认显示“商品配置模板”；低频的阶梯价维护通过“阶梯价模板”页签进入。
8. 商品配置和阶梯价模板仍复用现有商品配置、梯度模板和产品价格表能力，不另起一套价格表。

## Evidence
- Unit/UI guard: `node --test src/lib/product-settings.test.js --test-name-pattern "workspaces|compact drawers|nested tabs"`
- API/support guard: `go test ./internal/interfaces/http/support -run 'TestDev363|TestDev364' -count=1`
- Frontend build: `npm run build`
