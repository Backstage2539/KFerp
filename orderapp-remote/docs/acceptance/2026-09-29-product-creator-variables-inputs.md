# PR-675 商品创建器 V3 命名变量、多物料输入与表单优化

## 范围

在商品创建器 V3 中支持模板级命名变量、多个配方来源输入和可读的运行表单；旧模板版本与历史运行沿用原始语义。本次只面向 development，不涉及 production。

## 自动验证

- [x] RED：新增动态连线和命名变量测试后，旧 V2 画布回归用例曾因物料端口变化失败；保留 V2 端口合同后，定向 Node 测试转为 GREEN。
- [x] Node 定向测试：创建器图、变量、API 与动态端口用例通过；最终 Vue shell 1,281/1,281 通过。
- [x] Go 定向测试：应用、HTTP、PostgreSQL repository 与 support 包通过；`go test ./...` 全量通过。
- [x] Go 变量测试覆盖缺少变量、默认名称、手动覆盖、持久化能力缺失、草稿变量一起保存、提交摘要变量隔离；图验证覆盖 4 个配方来源、工艺路线、重复来源和稳定连线 ID。
- [x] PostgreSQL：变量与节点输入同修订保存、审计行及旧修订冲突测试通过；ProductCreator 数据库集成场景验证两级 BOM、多规格、两条工艺、分类中性值和默认绑定。
- [x] Vite：`npm run build` 成功；构建报告一个大 chunk 提示。
- [x] 自动整理布局回归：节点列间距覆盖 300px 节点宽度；新增连接端口不会被相邻列的卡片遮住。
- [x] 动态配方端口回归：连接后等待 Vue 更新端口，再刷新 Vue Flow 节点内部尺寸与句柄位置，避免运行数据已有配方来源但画布连线未显示。
- [x] 动态端口针对性测试：先确认缺少节点内部刷新时测试 RED，再验证 watcher 调用 `updateNodeInternals` 后 GREEN。
- [x] 连线状态回归：先在开发环境复现“配方表有来源、保存重开后连线消失”；连接逻辑改用 Vue Flow `addEdges` 动作后，源代码合同测试 RED/GREEN。
- [x] Vue shell 全量测试：1,281/1,281 通过；Vite 构建成功。
- [x] `git diff --check` 通过。
- [x] `scripts/verify_kferp.sh changed` 与 `scripts/verify_kferp.sh backend` 通过。

## 手册与界面入口

- 创建器操作手册：`orderapp-remote/docs/OP_MANUAL_PRODUCT_CREATOR.md`。
- Vue/Vite 菜单入口：商品 → 创建器手册；对应定义在 `frontend-vue-shell/src/lib/menu-ia.js` 与 `operation-manuals.js`。
- 验收清单：根目录 `ACCEPTANCE_TESTS.md` 与本目录的 `ACCEPTANCE_TESTS.md`。
- PR/DEV 系统表通过 `internal/interfaces/http/support/req_store.go` 种子维护。

## development 验收

- [x] 合入并部署后的开发环境服务健康、登录和商品创建器页面冒烟；PR/DEV 表状态随本次种子更新部署后核对。
- [ ] 1440、1024、768 像素画布和运行表单布局；确认变量、名称、BOM 身份、物料行和配方行完整可读。
- [x] 自动图验证覆盖四个配方物料来源、一条工艺、重复来源和稳定连线 ID；开发环境业务模板连接三个物料来源、两条工艺和两个 BOM 并成功发布、运行。
- [x] 建立并运行验收模板：三个物料、两条工艺、两个 BOM、半成品到成品；创建商品多规格，核对配方、发布、默认绑定及新对象位于未分类。
- [ ] 模板保存与业务预览不新增正式数据；草稿恢复和提交结果可追溯。
- [ ] Van 产品业务验收；PR-675 保持 review，生产未发布。

## 交付记录

- 开发部署：PR #159、#160 已合入；`7ebfbe2a154c6ecd3045f0efd0e041d5d5accef9` 于 2026-09-30 部署。备份：`/opt/stacks/erp/orderapp.backup.deploy-20260930061200-7ebfbe2a154c`；orderapp、PostgreSQL、登录健康检查通过。
- 前端与集成验证：最终 1,281 项 Vue shell 测试和 Vite build 通过；部署门禁中的 miniapp 42 个文件/264 项测试、类型检查/构建、全量 Go 测试（主机及容器）通过。未上传或发布微信小程序。
- 开发业务验证：模板 #3 V1 `商品创建器V3验收模板-变量多物料-20260930`，运行 #12 完成。创建商品 `模板验收豆-20260930-001`（ID 1097，SKU-001097），三项物料以及 BOM-049660/BOM-049661（ID 49660/49661）；两个 BOM 已发布并为新产出设置默认绑定。商品档案显示在未分类，多规格为 200g 默认及 500g。操作日志核对商品、物料、BOM、规格、发布和绑定。
- 采购/收货：模板未配置物料购入动作，本次没有创建采购单或收货单，也未改动库存。
- 验收边界：浏览器完成默认桌面窗口的模板运行、商品/物料档案及操作日志检查；1440/1024/768 专项尺寸、模板预览零写入和草稿恢复仍待验证。Van 产品业务验收待办，PR-675 保持 review，生产未发布。
