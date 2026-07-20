# PR-542 商品价格表行式规格与商品名/规格分离验收

## 范围

- 商品价格表规格选择改为横向紧凑排列。
- 新预览、新草稿、新发布、公开页和 PDF 保持商品名，销售规格作为独立属性。
- 具体 SKU、固定价、阶梯价和试算隔离规则不变；历史快照不重写。

## TDD 证据

- RED support contract：`go test ./internal/interfaces/http/support -run TestDev542PriceListInlineSpecDisplayContracts -count=1` 在实现前失败，首个缺失项为 `PR-542-PRICE-LIST-INLINE-SPEC-DISPLAY` 种子。
- RED 行式规格/平铺行：`node --test src/lib/costing-price-list-workflow.test.js src/lib/costing-bean-list-version-ui.test.js` 首次运行 62 项中 6 项失败，确认旧实现仍为纵向规格卡片、计价标题拼接商品和 SKU、平铺行缺少独立规格说明。
- RED 名称/规格快照：`node --test src/lib/product-price-list-selection.test.js src/lib/bean-list-pdf.test.js` 首次运行 48 项中 2 项失败，确认选中 SKU 后仍以子 SKU 名作为商品名。
- GREEN 定向前端：上述四个文件合跑 110/110；覆盖父商品名、客户显示名、227g/454g 双规格、独立 `sales_spec / 规格` 属性、属性替换去重、错误上下文、横向自动换行及全宽计价面板。
- GREEN API/公开页/PDF：`go test ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` 通过；新增草稿/发布请求透传、公开页双规格和服务端 PDF 文档映射断言。
- GREEN 后端全量：`go test ./... -count=1` 通过。
- GREEN 前端全量：功能分支 745 项中 739 通过、6 项失败；干净 `origin/develop` `b4553da0` 为 740 项中 734 通过、相同 6 项失败，均为既有 workspace context/mode 基线失败，不属于 PR-542。
- GREEN 构建：`npm run build` 通过，Vite 8.0.10 构建 401 个模块；仅保留既有大 chunk 警告。
- GREEN 仓库校验：`./scripts/verify_kferp.sh changed` 和 `git diff --check` 通过。

## PDF 视觉证据

- 使用 Go `-overlay` 临时 QA 测试调用真实服务端链路 `renderBeanListPublicationPDF → beanListPublicationPDFDocument → BeanListRenderer.Render`，不修改工作树、不访问数据库。
- 生成 `/tmp/pr542-pdf-qa/pr542-white-moon-specs.pdf`：1 页，306.14 × 544.25 pt；以 180 DPI 渲染 PNG 后人工检查。
- 两张卡片商品名均为“白月光瑰夏”，属性分别为“规格：227g”和“规格：454g”，价格单位分别为 227g 和 454g；未发现名称拼规格、文字重叠、截断或黑块。

## 开发环境验收

- [ ] 白月光瑰夏的227g、454g规格横向排列，计价配置在规格行下方全宽展开。
- [ ] 平铺价格行、预览、公开页和PDF只显示商品名“白月光瑰夏”，规格分别作为属性展示。
- [ ] 两个规格的 SKU、固定价、阶梯价、试算和手工覆盖互不串用。
- [x] 新快照保存父商品名和独立规格字段，客户显示名兼容且不追加规格。
- [ ] 开发环境浏览器和PDF冒烟通过，现有价格表没有重新发布。
- [x] 生产环境未部署、未写入、未切换入口；历史发布、历史PDF、订单和财务快照未迁移。
