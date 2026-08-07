# PR-584 商品行业字段与功能分组多模板引用验收记录

- 日期：2026-08-07
- 需求：`PR-584-PRODUCT-MULTI-GROUP-TEMPLATES`
- 产品状态：`review`，等待 Van 在 development 完成业务验收
- 环境边界：目标为合入 `develop` 并部署 development；`main`、production 不在范围内

## 范围

- 商品档案配置可同时引用多份行业字段模板，以有序 `industry_field_template_ids` 读写，并兼容旧单模板标量。
- 分组模板的“功能引用”支持多选；同一功能可引用多份模板，只有明确启用引用的模板才进入相应业务页面。
- 商品档案合并展示所有明确引用模板的分类树；每个商品只出现一次，未归入当前引用模板的商品进入一个全局 `未分类`，没有引用模板时使用平铺列表。
- 修复商品档案分类折叠：收起大类必须隐藏所有后代分类标题和商品行，同时保留小类自身折叠状态、分页和勾选状态。
- 解绑功能引用只隐藏对应模板的分类展示和归类入口，不删除模板、分类、既有对象归类或历史业务快照。

## 验收合同

### 行业字段多模板

- 两份模板按用户选择顺序合并字段；重复 `field_key` 只保留一项，并以第一份出现该字段的模板定义为准。
- 删除一份引用后，只清理不再由剩余模板定义的当前字段；清空全部引用后返回空模板数组和空字段列表。
- 旧 `industry_field_template_id` 作为首个模板兼容字段；旧数据升级后不丢失合法模板引用和值。
- 模板引用、字段和值同事务保存；失败不留下半份配置或成功日志，成功日志记录完整有序模板列表。

### 功能引用与商品分组

- “商品-咖啡豆”和“商品-挂耳”等模板可同时明确引用 `product_catalog`；未引用通用模板和仅引用其他功能的模板不得出现在商品档案。
- 功能引用保存为原子替换并写操作日志。取消引用后原分类和对象归类保留，重新引用后可恢复展示。
- 商品档案同时呈现所有当前引用模板的大类、小类和空分类；商品仍保持唯一当前归类，不在多个模板重复展示。
- 无引用模板时隐藏分类标题、折叠按钮和移动入口，只显示普通平铺商品列表。

### 父子折叠

- 收起父级大类后，其所有层级的子类标题、商品行和分页控件均不可见。
- 重新展开父级后，子类原有展开/收起状态、页码、每页条数及商品勾选状态恢复，不被父级操作重置。
- 批量选择范围只包含当前真实可见的商品，隐藏后代不得被误当作可见行批量操作。

## RED 证据

- 交付合同 RED：`go test ./internal/interfaces/http/support -run TestDev584ProductMultiGroupTemplatesDeliveryContracts -count=1` 首次退出码为 1，失败信息为 `req_store.go missing one-line req_product seed PR-584-PRODUCT-MULTI-GROUP-TEMPLATES with status review and assignee VA`。
- 行业字段应用 RED：`go test ./internal/application/catalog -run 'TestPR584' -count=1` 首次因 `ProductProductionConfig` 和 `SaveProductProductionConfigCommand` 尚无 `IndustryFieldTemplateIDs` 编译失败。
- 分组引用 RED：定向服务/仓储合同在实现前暴露显式空 usages 不能清空、assignment 会隐式创建 usage，以及旧迁移可能在重启后恢复已解绑 usage。
- Vue RED：`business-grouping.test.js`、`product-settings.test.js` 和 `group-settings-separation.test.js` 定向运行分别暴露未引用/停用 usage 仍被纳入、plural helper 缺失、多行业模板投影缺失、父级折叠后代仍可见与缺少“功能引用”。复审 RED 另锁定隐藏后代勾选保留、行业模板优先级可见和 `replace_usages` 兼容标志。

## GREEN 证据

- 交付合同与手册：补齐 PR/DEV UI 行、三份单一来源手册和本验收记录后，`go test ./internal/interfaces/http/support -run TestDev584ProductMultiGroupTemplatesDeliveryContracts -count=1` 通过；将 PR-534 历史合同明确更新为由 PR-584 的“仅显式引用”口径替代后，`go test ./internal/interfaces/http/support -run 'TestDev(451ProductMasterSubcategoryHeadersContracts|534ProductGenericGroupTemplateOptionsContracts|536ProductIndustryTemplateOnlyContracts|584ProductMultiGroupTemplatesDeliveryContracts)$' -count=1` 通过。
- 行业字段应用/API/PostgreSQL：`go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/stock ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` 八个目标包通过，覆盖数组 presence/旧标量、有序并集、清理、快照、原子审计与中文可读日志。
- 真实 PostgreSQL：一次性本地 PostgreSQL 16.13 隔离库实际执行 catalog 多模板并集/清理/模板并发锁、manufacturing 模板编辑后剩余模板接管字段元数据，sales 有序模板 ID 快照，三个包全部通过；测试库已停止且与开发/生产数据隔离。
- 分组引用：服务、HTTP 与仓储用例通过，覆盖五种 usage、多模板同功能、显式空清空、旧页面空数组 no-op、重启不恢复已解绑 usage、未引用 assignment 拒绝和审计。
- Vue/Vite：同步最新 `origin/develop` 后，`scripts/verify_kferp.sh frontend-tests` 共 887 项全部通过；`scripts/verify_kferp.sh frontend-build` 构建通过，仅保留既有大 chunk warning。
- 合并前门禁：`go test ./... -count=1`、`scripts/verify_kferp.sh backend`、`scripts/verify_kferp.sh changed` 与 `git diff --check` 全部通过；独立复核最终无未解决 P0/P1/P2。

## 手册

- `docs/OP_MANUAL_INVENTORY_MATERIALS.md`：商品多行业模板、已引用分组模板合并展示、无引用平铺、移动归类及父子折叠。
- `docs/OP_MANUAL_SETTINGS_AUDIT.md`：分组模板“功能引用”多选、保存校验、解绑边界和操作日志。
- `docs/OP_MANUAL_COSTING.md`：多模板行业字段在新价格表预览/快照中的有序合并、重复字段优先级和历史快照边界。

## 开发环境部署证据

- 功能分支：`codex/product-multi-group-templates-20260807@eb5535598e495e2b276fbf94c1b3869bc3f79777` 已推送；独立集成分支 `codex/integrate-product-multi-group-templates-20260807` 已推送。
- `develop`：集成提交 `bb6a35041965003ef2c36c68a72fc52d1aea83cc` 已推送并作为本次 development 运行代码提交。
- 发布命令：从一次性干净 `develop` 克隆执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；服务器单工锁内完成 Vue 887/887、Vue 构建、miniapp 195/195、typecheck、development 构建、全量 Go 与 Docker 镜像内全量 Go，再只重建并重启 `erp_orderapp`。
- 回滚证据：源目录备份 `/opt/stacks/erp/orderapp.backup.deploy-20260807231446-bb6a35041965`；回滚镜像 `kferp-orderapp-rollback:development-20260807231446-bb6a35041965`。
- 服务器烟测：`erp_orderapp`、`erp_docconvert` 运行，`erp_postgres` healthy；外部 `/app/login` 返回 200，未登录和已登录 `/app/` 按现有登录流返回 303；需求 API 返回 200 且包含 PR-584；新表 `product_production_config_industry_templates` 存在；发布源同时包含 `industry_field_template_ids` 与 `replace_usages`；启动错误计数为 0。
- 浏览器烟测：打开商品档案路由后正确跳转系统登录页，登录页渲染成功且控制台 0 错误。当前浏览器没有已登录会话，未输入或借用业务账号，因此多模板选择、分类收纳、折叠和操作日志的登录后业务验收保留给 Van。
- 环境边界：本机已有脏 `develop` 工作区未改动；本机 miniapp 导出明确跳过；没有合入 `main`，没有部署或修改 production，也没有执行微信上传、审核或发布。

## Van 业务验收

- [ ] 在商品档案配置同时选择两份行业字段模板，确认字段顺序、重复字段第一模板优先、已有值保留和保存后刷新一致。
- [ ] 取消其中一份模板，确认只移除不再被其他模板定义的字段；清空全部模板后字段区域为空。
- [ ] 在分组模板给“商品-咖啡豆”和“商品-挂耳”同时勾选商品档案引用，确认商品档案同时显示两份分类树；未引用模板不出现。
- [ ] 取消一份功能引用，确认其分类在商品档案隐藏，但模板、分类、商品和原归类没有被删除；重新引用后原归类恢复。
- [ ] 清空商品档案全部分组模板引用，确认列表平铺且分类、折叠和移动入口隐藏。
- [ ] 收起包含小类的大类，确认全部后代标题、商品和分页隐藏；重新展开后小类自身折叠状态、页码及勾选状态保持。
- [ ] 在操作日志核对商品行业字段模板列表保存、分组模板功能引用增删和商品移动归类记录，确认失败操作没有成功日志。

## 状态

- UI 产品需求：`PR-584-PRODUCT-MULTI-GROUP-TEMPLATES` 为 `review`，验收人 VA。
- UI 开发需求：四个 `DEV-584-*` 合同登记为 `done`；代码、测试、合并和 development 部署证据已在本记录中独立列明。
- Van 业务验收：未完成。
