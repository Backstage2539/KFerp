# PR-676 商品创建器 V4 商品产出 BOM 引用规格模板

## 目标与范围

商品规格模板固定配置在商品产出 BOM 节点。商品节点只负责商品档案；运行表单从 BOM 已连接的候选中选择一个规格主体，再将固定版本中的规格、包材、主体用量、工艺、损耗和默认规格应用到 ERP BOM。

物料产出 BOM 保留原有物料配方配置。V1–V3 已发布模板和既有运行不变；编辑旧模板时生成 V4 草稿，旧手工配置必须核对后才能发布。本需求只部署 development，production 与微信发布不在范围内；Van 的产品验收待办。

## 开发基线

- 功能分支：`codex/product-creator-bom-spec-template-20260930`
- 基线：`origin/develop@c8f0f4021027c0d26aa6ade953d073d31a9bea7f`
- 追踪：PR-676-PRODUCT-CREATOR-BOM-SPEC-TEMPLATE；DEV-707-PC-BOM-SPEC-TEMPLATE、DEV-708-PC-TEMPLATE-EXECUTION、DEV-709-PC-TEMPLATE-UI-DOCS-DELIVERY。
- 操作手册：`docs/OP_MANUAL_PRODUCT_CREATOR.md`，并从 Vue/Vite 的「创建器手册」入口打开。

## 自动化验证

- [x] RED：新增需求追踪 API 种子断言后，`TestProductCreatorV4SpecTemplateDeliveryRequirementSeeds` 因 PR/DEV 记录缺失而失败；补齐追踪记录后转绿。
- [x] Go 定向：`go test ./internal/application/productcreator ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/productcreator -count=1`。
- [x] HTTP/API 定向：商品创建器模块目录提供 V4，规格模板字段属于 BOM，不属于商品节点；`go test ./internal/interfaces/http/productcreator ./internal/interfaces/http/support -run 'TestProductCreator(ModuleCatalogAndTemplateLifecycleAPI|V4SpecTemplateDeliveryRequirementSeeds)$' -count=1`。
- [x] 前端定向：商品创建器 V4 图端口、模板迁移和变量测试通过。
- [x] 交付跟踪回归：PR-676 与三个 DEV 状态更新断言先失败，再将 PR 标为 `review`、DEV 标为 `done` 后通过。
- [x] 全量 Go / Vue：`scripts/verify_kferp.sh all` 通过；前端 Node 测试 1,284 项通过，Go 全包通过。
- [x] Vue/Vite 构建：`npm run build` 成功。构建保留既有 PDF 预览 chunk 大于 500 KB 的提示。
- [x] `git diff --check`。
- [x] `scripts/verify_kferp.sh changed` 合入前重跑；全量 Go/Vue 部署门禁在干净发布环境通过。开发工作站未配置 `ORDERAPP_TEST_DATABASE_URL` 或 `DATABASE_URL`，因此独立本地 PostgreSQL 集成测试未执行。
- [x] development 部署后登录、Vue 工作台和创建器页面可用；开发登录 HTTP 200，服务健康检查通过。

## 关键验收场景

- [x] 模板设计器仅在商品产出 BOM 提供规格模板选择；商品节点不展示规格/包装编辑；商品 BOM 选择已发布模板后显示模板规格明细。
- [x] 运行时从连接的半成品物料候选选择一个规格主体；提交预览展示模板 `test · V009` 的固定版本、主体和三种规格，未选候选没有进入 BOM。
- [x] 两级 BOM 先创建半成品，再按规格模板生成多规格商品；商品 BOM 发布版本 3320，规格 420–422 创建，454g袋装为模板默认规格；包材、工艺、主体比例和损耗由对应模板/BOM 应用。
- [x] 新对象默认 BOM 绑定可在结果和操作日志中确认：半成品 BOM 49999、商品 BOM 50000；商品 1099，原料 `PC-MAT-1EE470478F767445`，半成品 `PC-MAT-985659443B0AC62C`。
- [x] 模板 V2 发布及运行 #14 完成；正式结果页显示配置完成，并说明刷新/恢复不会重复创建。操作日志包含运行提交、BOM 发布、规格生成及默认绑定记录。
- [x] 首次运行 #13 在配置校验阶段失败，没有创建正式档案或 BOM；根据错误修正配方单位并调整模板后，V2 运行成功。
- [ ] V1–V3 历史运行与旧草稿冲突迁移的浏览器专项场景未在本次运行中手工操作；由定向测试覆盖，产品验收时可继续核对。

## 交付证据

- PR #164 合入 `develop`，运行版本 `892d99bc9284643f712271ae822a09793bdb0bdb`。
- Development 备份 `/opt/stacks/erp/orderapp.backup.deploy-20260930143701-892d99bc9284`；部署门禁、容器/数据库健康和登录 HTTP 200 通过。
- 浏览器：模板 `PR-676 V4 商品规格模板验收-半成品到成品-20260930` V2；运行 #14；商品 `/products/1099`；半成品 BOM 49999；商品 BOM 50000/发布版本 3320；规格 420–422。运行结果及对应操作日志已核对。
- Van 产品验收：待 Van 检查后更新。
