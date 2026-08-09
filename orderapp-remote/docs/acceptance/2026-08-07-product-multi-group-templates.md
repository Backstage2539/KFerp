# PR-584 商品行业字段与功能自选分组模板验收记录

- 日期：2026-08-07
- 需求：`PR-584-PRODUCT-MULTI-GROUP-TEMPLATES`
- 产品状态：`review`，等待 Van 在 development 完成业务验收
- 环境边界：目标为合入 `develop` 并部署 development；`main`、production 不在范围内

## 范围

- 商品档案配置可同时引用多份行业字段模板，以有序 `industry_field_template_ids` 读写，并兼容旧单模板标量。
- 分组模板只维护模板和分类树；商品档案、物料档案、生产 BOM、仓库库存分别在自己的页面多选并有序保存本功能使用的模板。
- 商品档案合并展示所有已选模板的分类树；每个商品只出现一次，未归入当前已选模板的商品进入一个全局 `未分类`，没有选择模板时使用平铺列表。
- 商品价格表不维护自己的模板引用，按商品档案已选模板生成一一对应的商品类型，并忽略历史 `price_list` 引用。
- 修复商品档案分类折叠：收起大类必须隐藏所有后代分类标题和商品行，同时保留小类自身折叠状态、分页和勾选状态。
- 在功能页面取消选择只隐藏对应模板的分类展示和归类入口，不删除模板、分类、既有对象归类或历史业务快照。

## 验收口径修正

- 2026-08-07 首次交付把关系做成“分组模板选择功能”，与 Van 的验收口径相反。该模型及其模板页“功能引用”控件、`price_list` 独立引用均作废，不作为最终验收证据。
- 最终权威方向是“功能选择分组模板”：分组模板页无功能选择；商品档案多选自己使用的模板；商品价格表只继承商品档案选择。原行业字段多模板与父子折叠修复继续保留。
- 首次 development 部署 `bb6a3504` 只作为历史部署证据；本修正必须重新通过 RED/GREEN、合入最新 `develop` 并重新部署 development 后，才可进入 Van 业务验收。

## 验收合同

### 行业字段多模板

- 两份模板按用户选择顺序合并字段；重复 `field_key` 只保留一项，并以第一份出现该字段的模板定义为准。
- 删除一份引用后，只清理不再由剩余模板定义的当前字段；清空全部引用后返回空模板数组和空字段列表。
- 旧 `industry_field_template_id` 作为首个模板兼容字段；旧数据升级后不丢失合法模板引用和值。
- 模板引用、字段和值同事务保存；失败不留下半份配置或成功日志，成功日志记录完整有序模板列表。

### 功能自选模板与商品分组

- 分组模板页只维护模板和分类树，不出现“功能引用”选项，也不能在保存模板时改写功能侧选择。
- 商品档案可有序多选“商品-咖啡豆”和“商品-挂耳”等 active 模板；选择保存为原子替换并写操作日志。取消选择后原分类和对象归类保留，重新选择后可恢复展示。
- 只有被商品档案功能选择的 active 模板才进入商品档案；未选择模板、仅有历史模板侧 usage 或已停用模板都不得影响当前展示。
- 商品档案同时呈现所有当前已选模板的大类、小类和空分类；未选模板不进入，商品仍保持唯一当前归类，不在多个模板重复展示。
- 物料档案、生产 BOM、仓库库存各自只列出本功能已选模板供分类查看和移动；清空选择后使用平铺列表并停止提供移动归类，既有对象归类保留。
- 无选择模板时隐藏分类标题、折叠按钮和移动入口，只显示普通平铺商品列表。

### 商品价格表继承

- 商品价格表不显示独立模板选择，也不使用 `price_list` usage；它只读取商品档案当前选择的 active 模板。
- 商品档案选择咖啡豆、挂耳两份模板时，价格表商品类型恰好出现两个一一对应选项并保持相同顺序；取消一份后对应类型退出。
- 历史 `price_list` 引用不能产生额外类型；商品档案选择变化只作用于后续价格表编辑、预览和发布，已发布快照不回改。

### 父子折叠

- 收起父级大类后，其所有层级的子类标题、商品行和分页控件均不可见。
- 重新展开父级后，子类原有展开/收起状态、页码、每页条数及商品勾选状态恢复，不被父级操作重置。
- 批量选择范围只包含当前真实可见的商品，隐藏后代不得被误当作可见行批量操作。

## RED 证据

- 验收口径修正 RED：先把支持合同改成“功能自选模板 + 价格表继承”后运行 `go test ./internal/interfaces/http/support -run TestDev584ProductMultiGroupTemplatesDeliveryContracts -count=1`，退出码为 1，失败信息为 `req_store.go missing one-line req_dev seed DEV-584-PRICE-LIST-INHERIT-PRODUCT-GROUPS with status done and assignee Codex`，证明旧交付没有登记也没有约束价格表继承合同。
- 交付合同 RED：`go test ./internal/interfaces/http/support -run TestDev584ProductMultiGroupTemplatesDeliveryContracts -count=1` 首次退出码为 1，失败信息为 `req_store.go missing one-line req_product seed PR-584-PRODUCT-MULTI-GROUP-TEMPLATES with status review and assignee VA`。
- 行业字段应用 RED：`go test ./internal/application/catalog -run 'TestPR584' -count=1` 首次因 `ProductProductionConfig` 和 `SaveProductProductionConfigCommand` 尚无 `IndustryFieldTemplateIDs` 编译失败。
- 分组引用 RED：定向服务/仓储合同在实现前暴露显式空 usages 不能清空、assignment 会隐式创建 usage，以及旧迁移可能在重启后恢复已解绑 usage。
- Vue RED：`business-grouping.test.js`、`product-settings.test.js` 和 `group-settings-separation.test.js` 定向运行分别暴露未引用/停用 usage 仍被纳入、plural helper 缺失、多行业模板投影缺失、父级折叠后代仍可见与缺少“功能引用”。复审 RED 另锁定隐藏后代勾选保留、行业模板优先级可见和 `replace_usages` 兼容标志。

## GREEN 证据

- 首次交付历史 GREEN（所有权口径已被本次修正取代）：当时补齐 PR/DEV UI 行、三份单一来源手册和本验收记录后，旧支持合同通过；PR-534 历史合同也更新为只接受明确选择模板。
- 行业字段应用/API/PostgreSQL：`go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/stock ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` 八个目标包通过，覆盖数组 presence/旧标量、有序并集、清理、快照、原子审计与中文可读日志。
- 真实 PostgreSQL：一次性本地 PostgreSQL 16.13 隔离库实际执行 catalog 多模板并集/清理/模板并发锁、manufacturing 模板编辑后剩余模板接管字段元数据，sales 有序模板 ID 快照，三个包全部通过；测试库已停止且与开发/生产数据隔离。
- 首次交付的模板侧“功能引用”服务、HTTP 与仓储用例曾通过，但该方向已被“功能选择模板”的最终合同取代，不再作为本次修正的 GREEN。
- 首次交付历史 Vue/Vite：当时 `scripts/verify_kferp.sh frontend-tests` 共 887 项全部通过；该数字只对应已被更正的首次所有权模型，不作为本次修正的最终前端计数。
- 本次修正前端完整回归曾达到 900/900；终审随后发现“刷新后保留第二个同价目类型模板，但未立即加载该模板发布历史”的 P2，补上首次 ready 主动刷新后，相关价格表定向用例 51/51 通过。
- 功能侧选择 GREEN：商品档案、物料档案、生产 BOM、仓库库存分别读写自己的有序模板选择；分组模板页不再反向选择功能；清空选择平铺且保留既有归类。
- 商品价格表 GREEN：只继承商品档案 `product_catalog` 选择，每份模板恰好一种商品类型；同属 `commercial` 的两份模板使用独立发布身份、缓存和版本查询，历史 `price_list` 不增加类型。
- 后端应用、HTTP、PostgreSQL 16.13 隔离库与场景脚本均通过；父级折叠、模板选择顺序、权限和可读操作日志的定向回归通过。Van 已明确要求部署后由其人工验证，因此不再执行浏览器或 development 业务验收。

## 手册

- `docs/OP_MANUAL_INVENTORY_MATERIALS.md`：商品多行业模板、商品档案自选分组模板、无选择平铺、移动归类、价格表继承及父子折叠。
- `docs/OP_MANUAL_SETTINGS_AUDIT.md`：分组模板页只维护模板与分类树、各功能自己选择、取消选择边界和操作日志。
- `docs/OP_MANUAL_COSTING.md`：多模板行业字段有序合并、价格表继承商品档案分组模板、历史 `price_list` 忽略和已发布快照边界。

## 开发环境部署证据

- 本节以下 `bb6a3504` 为首次交付历史证据，其模板侧“功能引用”口径已被验收修正取代；本次纠正尚未部署，不能用该版本进行最终业务验收。
- 功能分支：`codex/product-multi-group-templates-20260807@eb5535598e495e2b276fbf94c1b3869bc3f79777` 已推送；独立集成分支 `codex/integrate-product-multi-group-templates-20260807` 已推送。
- `develop`：集成提交 `bb6a35041965003ef2c36c68a72fc52d1aea83cc` 已推送并作为本次 development 运行代码提交。
- 发布命令：从一次性干净 `develop` 克隆执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；服务器单工锁内完成 Vue 887/887、Vue 构建、miniapp 195/195、typecheck、development 构建、全量 Go 与 Docker 镜像内全量 Go，再只重建并重启 `erp_orderapp`。
- 回滚证据：源目录备份 `/opt/stacks/erp/orderapp.backup.deploy-20260807231446-bb6a35041965`；回滚镜像 `kferp-orderapp-rollback:development-20260807231446-bb6a35041965`。
- 服务器烟测：`erp_orderapp`、`erp_docconvert` 运行，`erp_postgres` healthy；外部 `/app/login` 返回 200，未登录和已登录 `/app/` 按现有登录流返回 303；需求 API 返回 200 且包含 PR-584；新表 `product_production_config_industry_templates` 存在；发布源同时包含 `industry_field_template_ids` 与 `replace_usages`；启动错误计数为 0。
- 浏览器烟测：打开商品档案路由后正确跳转系统登录页，登录页渲染成功且控制台 0 错误。当前浏览器没有已登录会话，未输入或借用业务账号，因此多模板选择、分类收纳、折叠和操作日志的登录后业务验收保留给 Van。
- 环境边界：本机已有脏 `develop` 工作区未改动；本机 miniapp 导出明确跳过；没有合入 `main`，没有部署或修改 production，也没有执行微信上传、审核或发布。
- 修正交付：功能分支推送、`develop` 合并和 development 最轻量重部署待完成；按 Van 指示不执行浏览器、登录后业务流或 development 人工验收。production 不在范围内。

## Van 业务验收

- [ ] 在商品档案配置同时选择两份行业字段模板，确认字段顺序、重复字段第一模板优先、已有值保留和保存后刷新一致。
- [ ] 取消其中一份模板，确认只移除不再被其他模板定义的字段；清空全部模板后字段区域为空。
- [ ] 在分组模板页维护“商品-咖啡豆”和“商品-挂耳”的模板与分类树，确认页面没有“功能引用”选项。
- [ ] 在商品档案同时选择咖啡豆、挂耳两份分组模板，确认商品档案同时显示两份分类树、刷新后顺序保持，未选模板不出现。
- [ ] 分别在物料档案、生产 BOM、仓库库存多选本功能所用模板，确认刷新后选择保留、未选模板不进入下拉；清空选择后列表平铺且移动归类不可用，既有归类未删除。
- [ ] 在商品档案取消一份模板，确认其分类隐藏但模板、分类、商品和原归类没有被删除；重新选择后原归类恢复。清空全部选择后列表平铺且分类、折叠和移动入口隐藏。
- [ ] 进入商品价格表，确认商品类型与商品档案已选模板一一对应；历史 `price_list` 引用不产生额外类型，页面没有独立模板选择。
- [ ] 收起包含小类的大类，确认全部后代标题、商品和分页隐藏；重新展开后小类自身折叠状态、页码及勾选状态保持。
- [ ] 在操作日志核对商品行业字段模板列表保存、商品档案分组模板选择增删和商品移动归类记录，确认失败操作没有成功日志。

## 状态

- UI 产品需求：`PR-584-PRODUCT-MULTI-GROUP-TEMPLATES` 为 `review`，验收人 VA。
- UI 开发需求：行业字段多模板、四个功能自选模板、商品档案合并折叠和价格表继承均为 `done`；development 最轻量重部署待完成。
- Van 业务验收：未完成。
