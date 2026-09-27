# PR-672 商品创建器技术验收记录

## 范围与状态

- 分支：`codex/product-creator-workflow`，基于 `origin/develop@b2a7729a`。
- 本记录覆盖模板图校验、真实 PostgreSQL 配置事务、采购收货、定价发布、幂等和失败回滚。
- `bash scripts/verify_kferp.sh all` 已通过：Go 全包、Vue/Node 1,263 项测试和 Vite 生产构建；`git diff --check` 已通过。
- 两条真实 PostgreSQL 商品创建器集成测试已通过，覆盖完整多级执行及中途失败回滚。
- development 已部署到 `8cfa2305b5cf47afbbc468ec4a924453126309ca`，登录 HTTP 200；Van 的业务验收尚未进行。
- 不涉及 production 数据或生产环境发布。

## 已完成的真实 PostgreSQL 业务场景

测试使用独立临时 schema 和真实 PostgreSQL；场景结束后自动删除该 schema。

1. **非咖啡三级装配商品**：发布通用工作流后，新建树脂原料、外购控制板/铭牌和自制外壳/驱动模块，创建并发布三级 BOM，分别绑定外壳、驱动模块及商品默认 BOM。商品 BOM 包含两个不同单位的规格。提交后检查实际商品、物料、BOM 版本、规格、默认绑定和运行审计。
2. **采购、收货与定价**：配置提交后检查采购单尚未影响库存和物料实际采购价；单独创建采购单、确认 9.5kg 实收后检查采购价、收货记录及批次。价格步骤等待收货，再试算、存草稿、发布两条规格价格，检查发布表信息及版本。
3. **丢失响应重试**：对配置提交、采购单、收货、价格试算、价格草稿和价格发布分别使用原幂等键重试，检查没有重复档案、采购单、入库批次或价格发布。
4. **复用半成品和包材**：另一模板引用既有外壳及铭牌，只新增一个商品 BOM；数据库断言物料数、批次数、既有半成品默认绑定均不变，BOM 组件仍指向原物料。
5. **配置事务回滚**：预览有效后停用物料单位，使执行在已经尝试创建商品和首个物料后失败；检查先前商品、物料以及提交审计一并回滚。

对应测试：

- `internal/appmain/product_creator_configuration_integration_test.go`
- `internal/application/productcreator/graph_test.go`
- `internal/application/productcreator/commit_test.go`
- `internal/infrastructure/postgres/productcreator/repository_test.go`
- `internal/infrastructure/postgres/productcreator/run_step_repository_test.go`
- `internal/interfaces/http/productcreator/routes_test.go`
- `frontend-vue-shell/src/lib/product-creator-graph.test.js`
- `frontend-vue-shell/src/api/product-creator.test.js`

真实数据库命令：

```sh
ORDERAPP_TEST_DATABASE_URL='user=<local-user> dbname=postgres host=/tmp' go test ./internal/appmain -run 'TestProductCreator(CommitsMultiLevelGenericProductConfigurationAtomically|ConfigurationRollsBackEarlierObjectsAfterCommitValidationFailure)$' -count=1
```

结果：以上两个 PostgreSQL 集成测试通过。`graph_test.go` 还覆盖不同类型连线、环路、悬空/重复标识、条件跳过和三类模板预览校验；HTTP 测试覆盖模块目录、模板生命周期及提交接口约束。Vite 提示 `PDFStampPreview` chunk 超过 500 KB（840 KB）；商品创建器 chunk 为 285 KB，构建成功。

## Development 交付与浏览器证据

- 部署环境：development；部署提交 `8cfa2305b5cf47afbbc468ec4a924453126309ca`（`origin/develop`）。
- 部署前代码备份：`/opt/stacks/erp/orderapp.backup.deploy-20260928021757-8cfa2305b5cf`。
- 回滚镜像：`kferp-orderapp-rollback:development-20260928021757-8cfa2305b5cf`。
- 部署脚本通过前端测试、微信小程序测试/类型检查/构建、服务端 Go 全量测试与镜像构建；PostgreSQL 健康检查通过，orderapp 已重建启动，development 登录 HTTP 200。
- 已登录页面确认「商品 → 商品创建器」入口、模板空列表和 Vue Flow 设计器可用；添加「商品档案」节点后撤销可用，撤销后重做可用，重做恢复节点。
- 本次浏览器回归的空白测试模板未保存；返回后列表仍为 0 个模板、0 条运行记录，未创建业务商品、物料或单据。
- 初次浏览器检查发现 Vue 响应式代理的克隆异常，随后修复；再次检查发现图编辑没有进入撤销历史，随后补齐各项图编辑的历史快照。最终 development 浏览器回归通过上述添加/撤销/重做路径。
- 画布拖动、连线及多分支交互、模板发布/版本列表、完整运行表单与草稿恢复/预览/错误定位尚未完成浏览器逐项验收。

## 尚待业务验收

- Van 按全新咖啡商品、复用半成品和非咖啡装配三条模板进行业务验收；复用与非咖啡三级装配另有真实 PostgreSQL 自动化证据，详见上文。
- 单独验证复杂失败恢复、安全权限边界及未发布价格不可销售等验收表中未勾选场景。
- development 部署完成；production 未部署。
- Verifier: `bash scripts/verify_kferp.sh all`; two real PostgreSQL integration tests; authenticated development browser check; deployment container/database/login smoke. No PDF/PNG output path changed.
