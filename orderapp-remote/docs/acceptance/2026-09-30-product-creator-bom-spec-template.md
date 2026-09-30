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
- [x] 全量 Go / Vue：`scripts/verify_kferp.sh all` 通过；前端 Node 测试 1,284 项通过，Go 全包通过。
- [x] Vue/Vite 构建：`npm run build` 成功。构建保留既有 PDF 预览 chunk 大于 500 KB 的提示。
- [x] `git diff --check`。
- [ ] PostgreSQL 事务集成测试：当前开发工作站未配置 `ORDERAPP_TEST_DATABASE_URL` 或 `DATABASE_URL`；部署门禁仍需在目标验证环境确认。
- [ ] `scripts/verify_kferp.sh changed` 合入前重跑；development 部署后浏览器及业务验证。

## 关键验收场景

- [ ] 模板设计器仅在商品产出 BOM 提供规格模板选择；商品节点不展示规格/包装编辑；物料产出 BOM 拒绝规格模板。
- [ ] 商品 BOM 选择有效已发布模板版本后显示整组规格和主体候选；工艺连线、手工规格/包装/数量/单位/损耗覆盖均被阻止。
- [ ] 运行时可从连接的物料或商品规格中选择唯一主体；商品主体需要具体已发布规格；未选候选不写入配方。
- [ ] 两级 BOM 先生成半成品，再生成使用规格模板的多规格商品；核对包材、主体用量、工艺、损耗和唯一默认规格。
- [ ] 已有对象保留原分类和默认绑定；新建对象由系统显示“未分类”，并只为新建产出对象设置默认 BOM。
- [ ] 模板版本固定；保存、预览不写正式业务数据；旧运行保持原语义；无效主体和请求篡改被服务端拒绝。
- [ ] 旧 V1–V3 草稿升级时保留旧手工配置并要求显式处理冲突。

## 交付证据

- PR / 合并提交：待更新。
- Development 运行版本、备份与健康检查：待更新。
- 登录后浏览器检查与 V4 验收模板/商品：待更新。
- Van 产品验收：待 Van 检查后更新。
