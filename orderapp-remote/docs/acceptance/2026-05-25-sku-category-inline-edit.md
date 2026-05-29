# PR-365 SKU分类列表内编辑 Acceptance

## Scope
- 产品类型列表内直接操作，不再打开产品类型编辑弹窗。
- 复用现有商品分类保存、移动、删除接口，不新增后端数据模型。
- 子类型排序继续沿用原拖动方式，本次只调整新增、删除、改名入口和产品类型排序入口。

## Acceptance
1. 进入 SKU设置 → 商品资料，商品分类仍在固定高度滚动窗中展示。
2. 搜索框和产品类型列表之间显示加号和减号；点击加号直接新增产品类型，并让新名称进入可编辑状态。
3. 点击产品类型名称或产品子类型名称直接改名；页面不再显示单独“改名”按钮，也不打开产品类型编辑弹窗。
4. 点击产品类型工具栏减号后，每个可编辑产品类型右侧显示红色减号；点击红色减号可删除该产品类型。
5. 每个产品类型右侧显示上下箭头，可调整大类顺序；搜索状态下排序按钮禁用，避免按过滤后的列表错排。
6. 每个大类标题下方显示子类型加号和减号；加号新增子类型，减号进入该大类的子类型删除模式，并在子类型右侧显示红色减号。
7. 产品子类型仍按原拖动方式排序，拖动目标、停车场和 SKU 拖拽挂载逻辑不变。

## Evidence
- Unit/UI guard: `node --test src/lib/product-settings.test.js --test-name-pattern "inline category|compact"`
- API/support guard: `go test ./internal/interfaces/http/support -run 'TestDev365|TestDev364' -count=1`
- Frontend build: `npm run build`
