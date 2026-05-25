# PR-363 SKU Settings Workbench Layout Acceptance

## Scope
- SKU设置页面排版优化，不改产品设置 API、SKU 数据结构、商品配置规则或价格表生成逻辑。
- 日常商品资料维护和模板配置维护分成两个页内工作区。

## Acceptance
1. 进入 SKU设置 后默认停留在“商品资料”。
2. “商品资料”中可连续完成新增公共产品或客户专属 SKU、维护商品分类、查看停车场、维护客户SKU列表。
3. 商品分类和客户SKU列表在商品资料工作区中相邻展示；桌面宽度下分类在左、SKU 列表在右，窄屏下上下排列。
4. 切换到“模板配置”后，梯度模板和商品配置在同一模板工作区维护。
5. 梯度模板和商品配置不再夹在商品分类和客户SKU列表之间。
6. 产品子类型仍在商品资料中选择“更换商品配置”；模板本体的新增、复制和编辑统一在模板配置中完成。

## Evidence
- Unit/UI guard: `node --test src/lib/product-settings.test.js --test-name-pattern "SKU settings groups master data"`
- API/support guard: `go test ./internal/interfaces/http/support -run TestDev363 -count=1`
