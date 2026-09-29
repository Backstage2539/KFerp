# PR-675 商品创建器 V3 命名变量、多物料输入与表单优化

## 范围

在商品创建器 V3 中支持模板级命名变量、多个配方来源输入和可读的运行表单；旧模板版本与历史运行沿用原始语义。本次只面向 development，不涉及 production。

## 自动验证

- [x] RED：新增动态连线和命名变量测试后，旧 V2 画布回归用例曾因物料端口变化失败；保留 V2 端口合同后，定向 Node 测试转为 GREEN。
- [x] Node 定向测试：`node --test src/lib/product-creator-*.test.js src/api/product-creator.test.js`，15/15 通过；全量 Vue shell 1,275/1,275 通过。
- [x] Go 定向测试：应用、HTTP、PostgreSQL repository 与 support 包通过；`go test ./...` 全量通过。
- [x] Go 变量测试覆盖缺少变量、默认名称、手动覆盖、持久化能力缺失、草稿变量一起保存、提交摘要变量隔离；图验证覆盖 4 个配方来源、工艺路线、重复来源和稳定连线 ID。
- [x] PostgreSQL：变量与节点输入同修订保存、审计行及旧修订冲突测试通过；ProductCreator 数据库集成场景验证两级 BOM、多规格、两条工艺、分类中性值和默认绑定。
- [x] Vite：`npm run build` 成功；构建报告一个大 chunk 提示。
- [x] 自动整理布局回归：节点列间距覆盖 300px 节点宽度；新增连接端口不会被相邻列的卡片遮住。
- [x] 动态配方端口回归：连接后等待 Vue 更新端口，再刷新 Vue Flow 节点内部尺寸与句柄位置，避免运行数据已有配方来源但画布连线未显示。
- [x] 动态端口针对性测试：先确认缺少节点内部刷新时测试 RED，再验证 watcher 调用 `updateNodeInternals` 后 GREEN。
- [x] Vue shell 全量测试：1,277/1,277 通过；Vite 构建成功。
- [x] `git diff --check` 通过。
- [x] `scripts/verify_kferp.sh changed` 与 `scripts/verify_kferp.sh backend` 通过。

## 手册与界面入口

- 创建器操作手册：`orderapp-remote/docs/OP_MANUAL_PRODUCT_CREATOR.md`。
- Vue/Vite 菜单入口：商品 → 创建器手册；对应定义在 `frontend-vue-shell/src/lib/menu-ia.js` 与 `operation-manuals.js`。
- 验收清单：根目录 `ACCEPTANCE_TESTS.md` 与本目录的 `ACCEPTANCE_TESTS.md`。
- PR/DEV 系统表通过 `internal/interfaces/http/support/req_store.go` 种子维护。

## development 验收

- [ ] 合入并部署后的登录、创建器页面和 PR/DEV 接口冒烟。
- [ ] 1440、1024、768 像素画布和运行表单布局；确认变量、名称、BOM 身份、物料行和配方行完整可读。
- [ ] 通过模板画布连接四个配方物料来源和一条工艺，保存、重开、删除、撤销及重做后连接不变。
- [ ] 建立验收模板：三个物料、两条工艺、两个 BOM、半成品到成品；运行填写变量并创建商品，核对 BOM 配方/规格/默认绑定及分类为未分类。
- [ ] 模板保存与业务预览不新增正式数据；草稿恢复和提交结果可追溯。
- [ ] Van 产品业务验收；PR-675 保持 review，生产未发布。

## 交付记录

- 记录更新：2026-09-30。开发版上一轮部署和登录页冒烟已完成；此次动态端口修复待合入部署。浏览器多来源连线、模板发布及真实商品创建记录待补。
