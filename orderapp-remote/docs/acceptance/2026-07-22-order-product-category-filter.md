# PR-546 录单商品分类过滤与下拉自动收起

## 问题与目标

- 录单商品候选只支持名称、拼音和首字母搜索，商品较多时无法先按熟豆、挂耳、生豆或速溶咖啡缩小范围。
- 商品下拉打开后只有选择候选才会关闭；点击规格、数量或页面空白不会改变 `product_open`，菜单持续遮挡后续字段。
- 本需求只调整 Vue/Vite 录单交互，不修改订单表单 API、数据库、价格表 publication、SKU 或历史订单快照。

## TDD 证据

### RED

- `node --test src/lib/order-entry.test.js` 首次因 `order-entry.js` 未导出 `closeOrderProductDropdowns` 失败，证明分类与外部关闭 helper 尚不存在。
- `go test ./internal/interfaces/http/support -run TestDev546OrderProductCategoryFilterContracts -count=1` 首次因缺少 `PR-546-ORDER-PRODUCT-CATEGORY-FILTER` 需求种子失败。

### GREEN

- `node --test src/lib/order-entry.test.js`：119/119 通过，覆盖分类项生成、分类与文字搜索交集、鼠标和键盘切行只保留当前商品框、分类按钮键盘激活，以及 Vue document pointerdown 注册/清理。
- `node --test src/lib/form-draft-cache.test.js`：3/3 通过，原 `onBeforeUnmount(saveOrderEntryDraft)` 合同保持有效。
- `go test ./internal/interfaces/http/support -run '^TestDev546OrderProductCategoryFilterContracts$' -count=1`：通过。
- `go test ./internal/interfaces/http/sales -run '^TestAPIProductFamilies' -count=1`：通过。
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormProductsExposeProductTypeAndUnitRuleFields' -count=1`：通过。
- `scripts/verify_kferp.sh frontend-build`：首次因干净工作树尚未安装依赖而报 `vite: command not found`；按锁文件执行 `npm ci` 后重跑通过，Vite 8.0.10 完成 401 个模块的 production build。
- `scripts/verify_kferp.sh frontend-tests`：功能分支 796/802，新增 3 个测试全部通过；最新 `origin/develop` 独立干净工作树为 793/799，二者均为相同 6 个 workspace-context 既有失败，本需求没有新增失败。

## 交互与兼容边界

- 分类沿用候选原有商品标签；只根据当前客户和当前启用价格表过滤后的候选生成，先过滤 publication scope，再做分类和搜索，最后按客户常购排序并截取。
- 分类属于每条空白商品行的临时界面状态，不进入订单 payload，不修改已选 SKU、规格、价格或 publication。
- 点击当前商品框内部保留本行菜单；点击另一行只保留目标行，点击所有商品框外关闭全部。Tab 或方向键切换商品输入框时也只展开当前行；分类按钮可由 Enter/Space 激活。组件卸载时移除 document 监听，并继续执行原草稿保存 hook。
- 本轮合并到 `develop` 后停止，未部署 development 或 production，也未保存订单或修改价格表数据。
