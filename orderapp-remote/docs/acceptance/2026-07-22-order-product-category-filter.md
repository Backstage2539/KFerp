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
- 首次 `./deploy_orderapp.sh development` 在镜像内全量 Go 门禁复现 PR-340 旧测试对单行 `<label>` 的格式依赖；`75f0a48f` 将断言拆为稳定的 class 和 Vue 状态标记，定向测试及本机 `go test ./...` 全通过，再由 merge `0f95ad06` 合入 `develop`。

## 开发环境部署

- `./deploy_orderapp.sh development`：Vue shell、微信小程序类型检查/构建、Docker 镜像内 `go test ./...` 和 Go binary build 全通过，`erp_orderapp` 重建启动成功。
- 功能代码基线：`origin/develop` `f574d991aaa5e403bb6867040dc1dc4f2d58ae68`；手工验收关闭状态随 `origin/develop` `8967d7053d65f4240854d2321879a7a20a9d4cd1` 部署；最新回滚备份：`/opt/stacks/erp/orderapp.backup.deploy-20260722225950`。
- `erp_orderapp`、`erp_docconvert` 正常运行，`erp_postgres` healthy；应用日志显示监听 `:8080`，没有启动错误。
- 开发入口认证后 `/app/` 跟随跳转为 200；`/app/api/req/product?limit=500` 返回 200 并包含 `PR-546-ORDER-PRODUCT-CATEGORY-FILTER`；服务器源码包含 `product-kind-filter`，部署手册包含 PR-546。
- 自动化 in-app browser 因开发域现有 `ERR_CERT_AUTHORITY_INVALID` 无法进入页面，Chrome 扩展当时不可用；未绕过 TLS。Van 于 2026-07-22 在开发环境手工验证分类过滤和点击外部自动收起均通过，REV-546 与 PR-546 据此标记为 `done` 并关闭；部署后数据库复核两条状态均为 `done`。

## 交互与兼容边界

- 分类沿用候选原有商品标签；只根据当前客户和当前启用价格表过滤后的候选生成，先过滤 publication scope，再做分类和搜索，最后按客户常购排序并截取。
- 分类属于每条空白商品行的临时界面状态，不进入订单 payload，不修改已选 SKU、规格、价格或 publication。
- 点击当前商品框内部保留本行菜单；点击另一行只保留目标行，点击所有商品框外关闭全部。Tab 或方向键切换商品输入框时也只展开当前行；分类按钮可由 Enter/Space 激活。组件卸载时移除 document 监听，并继续执行原草稿保存 hook。
- 功能分支 `codex/pr546-order-product-category-filter` 通过 integration merge `26789d35ed02` 合并，部署门禁修正由 `0f95ad06` 合入 `develop`；development 已部署，production 未部署，也未保存订单或修改价格表数据。
