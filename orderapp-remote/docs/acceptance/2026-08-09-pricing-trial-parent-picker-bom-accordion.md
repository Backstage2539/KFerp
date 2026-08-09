# PR-588 价格试算主商品选择与生产 BOM 单展开分页验收记录

## 范围

- 需求：`PR-588-PRICING-TRIAL-PARENT-PICKER-BOM-ACCORDION`
- 分支：`codex/price-trial-bom-list-compact-20260809`
- 环境：只合入 `develop` 并最轻量部署 development；`main` 和 production 不在范围内。
- 验收方式：保留最小自动化和构建安全门禁，不做浏览器或 development 业务验证；部署后由 Van 人工验收。

## 需求合同

### 价格试算商品选择

- 商品价格管理的价格试算候选只包含启用主商品。`parent_product_id > 0` 的销售规格子 SKU 不作为独立商品候选，同一主商品只出现一次。
- 子 SKU 名称、规格或编码命中搜索时，也不能生成一条可单独选择的子 SKU 商品行；试算继续由主商品及其有效销售单位、销售规格和 BOM 上下文解析。
- 点击商品选择框后，候选包含多种形态时显示与录单一致的 `全部 / 熟豆 / 挂耳 / 生豆 / 速溶咖啡` 类型按钮。类型与关键词组合过滤，点击主商品即选中并关闭；仅一种形态时不额外占用类型按钮行。

### 生产 BOM 列表

- 删除“生产 BOM列表”标题下的“生产端主档案 / 归类保存到接口”说明，不删除页面顶部对生产 BOM 业务用途的总说明。
- 前往分组模板、分类与移动模板、移动到分类、目标分类、已选数量和设置分组模板等分组操作在桌面端保持一行，窄屏最多两行；新建 BOM、状态和搜索不混入该分组操作行。
- 顶层模板和全局 `未分类` 使用同一单展开手风琴。初始只展开所选模板顺序中的第一组；打开另一组时自动收起原组，任一时刻只有一个顶层组展开。
- 分页只统计并切片当前展开组的 BOM 行，模板、大类、小类表头和收起组不占分页条数。切换顶层组、状态或搜索后回到第 1 页；未选择模板时平铺 BOM 行并继续分页。
- 展开、过滤和分页是只读界面状态。只有明确执行 `移动到分类` 才继续调用既有业务分组归类接口并写原有操作日志。

## RED / GREEN 证据

- RED：价格试算定向合同首次运行时，现有候选仍直接来自商品行，缺少 `pricingRuleTrialMainProductOptions`，商品选择器也没有录单同款类型按钮、菜单头和类型标签交互，因此主商品与类型筛选合同失败。
- RED：生产 BOM 定向合同首次运行时，列表标题下仍有接口/主档描述，多个顶层模板可同时展开，也没有只针对当前展开组的分页状态，因此紧凑布局、单展开和分页合同失败。
- GREEN：`pricingRuleTrialMainProductOptions` 只保留唯一启用主商品；价格试算复用录单商品类型选项与组合过滤，并通过可搜索选择器菜单头完成点击筛选和主商品选择。
- GREEN：`productionBomAccordionPageState` 将模板与全局未分类组织成互斥顶层组，默认第一组展开；分页只切片当前展开组 BOM，无模板时平铺分页，切组、状态和搜索回第 1 页。
- GREEN：frontend-vue-shell 全量自动化测试 `901/901` passed；Vite build passed。
- GREEN：`go test ./internal/interfaces/http/support -run 'TestDev588PricingTrialParentPickerBomAccordionContracts|TestDev588BomListRemovesVerboseDescription' -count=1` passed。

## 文档同步

- `docs/OP_MANUAL_COSTING.md`：价格试算只选择主商品及录单同款类型过滤。
- `docs/OP_MANUAL_PRODUCTION.md`：紧凑分组工具栏、顶层单展开和当前组分页。
- `docs/OP_MANUAL_INVENTORY_MATERIALS.md`：生产 BOM 分组、分页与移动归类边界摘要。
- `REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、根目录同名需求/验收和 `ACTIVE_REQUIREMENTS.md`：PR/DEV、验收与部署范围。

## Van 人工验收

- [ ] 打开价格试算商品选择器，确认每个主商品只出现一次，227g、454g、磅、盒装等子规格不单独出现。
- [ ] 分别点击熟豆、挂耳、生豆、速溶咖啡并结合搜索，确认候选即时过滤，点击主商品后正确选中并关闭。
- [ ] 打开生产 BOM，确认列表标题下重复描述已删除，分组处理桌面一行、窄屏最多两行。
- [ ] 确认默认只展开第一顶层组；展开第二模板或未分类后，上一组自动收起。
- [ ] 确认分页总数只对应当前展开组 BOM，切组、状态或搜索后回到第 1 页，无模板时平铺分页正常。
- [ ] 确认仅浏览、筛选、展开和翻页不修改 BOM 归类；实际移动到分类后仍可在操作日志追溯。

## 交付状态

- 代码实现、定向 GREEN、frontend-vue-shell 全量 `901/901`、Vite build 和 PR-588 支持合同：已完成。
- 功能提交 `f2f25a57` 已通过合并提交 `7c96e62ef4e71d06c603ca5213a0937927ae56d8` 合入 `develop` 并部署到 development。
- 部署命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；服务器 Vue 测试 `922/922`、小程序测试 `195/195`、类型检查/构建、完整 Go 测试、镜像内 Go 测试和容器启动均通过；脚本自带外部探针返回 HTTP 200。
- 部署备份：`/opt/stacks/erp/orderapp.backup.deploy-20260809225406-7c96e62ef4e7`；回滚镜像：`kferp-orderapp-rollback:development-20260809225406-7c96e62ef4e7`。
- 浏览器和业务验收：按 Van 要求不执行，保留为上述人工验收项。
- production：未操作。
