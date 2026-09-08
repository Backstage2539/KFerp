# 商品主体 BOM 发布校验修复

- 原因：商品主体版本保存了模板来源、商品 ID 和已发布规格 ID，`main_input_material_id=0`；发布及默认绑定校验仍只读取物料 ID，误报“规格模板来源和规格主体物料必须同时配置”。
- 修复：按版本中的主体组件类型校验商品及已发布规格。手工规格组、历史已发布模板和物料主体继续兼容；缺失来源、失效商品、未发布或不匹配的规格仍被拒绝。没有数据库迁移或业务数据修正。
- RED：新增 PostgreSQL 发布测试在修改前复现上述错误；无模板但保留商品主体来源的异常版本还会错误放行。
- GREEN：真实 PostgreSQL 中 20 个定向场景通过，覆盖创建模板、创建商品主体 BOM、完整发布事务、两种默认绑定、发布日志，以及失败保持草稿且无发布日志。
- API/应用层：`go test ./internal/application/bom ./internal/interfaces/http/bom -count=1` 通过。
- 定向数据库验证：`ORDERAPP_TEST_DATABASE_URL=<isolated-local-db> go test ./internal/infrastructure/postgres/bom -run '^(TestProductMainInputTemplateBOMPublishPostgres|TestManualProductBOMSpecGroupPublishesWithoutTemplateProvenancePostgres|TestProductBOMSpecGroupRejectsPartialTemplateProvenancePostgres|TestArchivedPublishedTemplateProvenanceRemainsValidAcrossPublishAndBindingsPostgres|TestProductionBomMaterialOwnershipRulesPostgres|TestProductBOMOutputRebindingAcceptsManualSpecGroupAndRejectsInvalidTemplateProvenancePostgres)$' -count=1 -v`。
- 扩展数据库套件：修复前基线有 14 个既有失败，涉及旧物料取得方式、规格模式、版本扫描和旧测试结构；不将这些基线失败记作本次通过，也不在本次扩大修复。
- 手册：`OP_MANUAL_PRODUCTION.md` 的规格主体组件和手工规格组说明已校正；没有新增操作入口。
- 交付授权：修复合入 `develop` 和 `main`，依次部署开发、生产。业务验收由 Van 执行，不自动发布生产 BOM。
