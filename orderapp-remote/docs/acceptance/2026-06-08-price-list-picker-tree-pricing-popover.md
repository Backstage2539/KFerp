# PR-460 商品价格表选品树与计价弹出优化

## 范围
- 商品价格表“选择分类和产品”按商品分组模板显示树形选品：父类、子类、商品逐级缩进，分类支持收缩/展开。
- 分类和商品 `计价` 摘要改为按钮附近的轻量菜单，不再打开右下角计价弹窗。
- 预览和生成 PDF 使用与“选择分类和产品”同源的分类行和商品行，避免旧 bean-list 分类把同一选品分类拆散。
- 不改后端 API、数据库表或发布快照结构；发布解析仍为 `商品 > 子类 > 父类 > 价格表`。

## 验收点
- 父类保持左侧基准，子类向后缩进，商品相对子类再向后缩进。
- 父类和子类都可以收缩/展开；收缩只隐藏商品行，保留分类勾选、计价按钮和选中数/总数。
- 点击父类、子类或商品的 `计价 继承分类`，在按钮附近弹出 `继承分类`、`按阶梯模板价计算`、`按价格模板计算`、`固定价`。
- 选择 `继承分类` 会清掉当前分类或商品自己的计价覆盖；选择其他模式后在同一个菜单内选择阶梯模板、价格模板或固定价。
- 父类按钮只写父类覆盖，子类按钮只写子类覆盖，商品按钮只写商品覆盖；商品 `展示 无标签` 仍走原展示弹窗。
- 下方预览和生成 PDF 的分类标题、分类过滤、商品归属和商品顺序与“选择分类和产品”保持一致；同一选品分类下的商品不会再按旧 bean-list 分类拆成多个预览分类。

## 本地验证
- RED：`node --test src/lib/costing-bean-list-version-ui.test.js` 先失败，缺少树形缩进、收缩、计价 popover 和分类 target helper。
- RED：`go test ./internal/interfaces/http/support -run TestDev460 -count=1` 先失败，缺少 PR-460 种子和文档标记。
- GREEN：`node --test src/lib/costing-bean-list-version-ui.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js` 通过 163/163。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev460 -count=1` 通过。
- GREEN：`go test ./internal/interfaces/http/support -run 'TestDev(449|460)' -count=1` 通过，确认 PR-449 合同已跟随新计价 popover 语义。
- GREEN：`npm run build` 通过，保留既有 Vite chunk-size warning。
- GREEN：`git diff --check` 通过。
- RED follow-up：`node --test src/lib/bean-list-pdf.test.js` 先失败，缺少 `buildBeanListPdfGroupsFromCategoryRows`；`node --test src/lib/costing-bean-list-version-ui.test.js` 先失败，`pdfGroups` 仍未从选品树分类行生成。
- GREEN follow-up：`node --test src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js` 通过，预览/PDF 改为跟随选品树分类和商品行。
- Browser：本轮按 Van 要求不合并、不部署，未做 development ERP 业务数据验收；本地 Vite mock 页面尝试使用 bundled Playwright + 本机 Chrome 渲染，但本地 mock 未成功出现选品分类，因此不声明浏览器验收通过。

## 部署状态
- Van 要求本轮开发完不合并、不部署；development 浏览器验收需在后续合并部署后执行。
