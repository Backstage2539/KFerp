# PR-680 商品创建器 V8：引用产出停用上游

范围：自制物料或商品选择已有产出时，停止追溯其生成 BOM；各终点所需依赖取并集，共享来源继续执行。旧版本及历史运行保持原规则。无数据库表或结构迁移。

## 自动验证

- RED：`TestV8ReuseStopsUpstreamValidationAndPreview` 在改动前要求空的 raw.rows、roast.output_qty/output_unit/components 和 route.route_id；前端真实运行组件测试未显示停用原因及 disabled 字段。测试日志保存在执行机 `/tmp/pc-v8-frontend-red.log`。
- GREEN：application、postgres/productcreator、http/productcreator 的定向测试通过，包括缺失选择、共享依赖、多终点、连续复用、有效变量及 V7 兼容。
- 实际 PostgreSQL：`TestV8ReusedOutputPostgresNoWritesAndFreshSnapshot` 验证停用所有上游时没有建档/BOM服务调用、制造来源及正式规格身份，归属/单位/规格/绑定快照变化要求重新预览。
- 完整领域事务：`TestProductCreatorBOMCentricCommitsMaterialThenMultiSpecProduct/V8_reuse_material` 及 `/V8_reuse_product` 通过。复用旧路径建立的档案，只增加一个下游商品与一个 BOM；原料、原产出及旧 BOM 不变。失效下游单位使整体回滚，重复提交不重复写入。
- 前端：50 项创建器定向测试与 Vite 构建通过。运行草稿保留停用输入，新建内容另存于同一节点以便切回恢复；示例值不进入运行。
- 步骤结果沿用 succeeded/skipped 状态；action=reuse 和 execution_plan 记录复用及停用关系。事务提交操作日志包括 execution_plan。

## 手册与验收

- 单一来源手册：`docs/OP_MANUAL_PRODUCT_CREATOR.md`，“从已有半成品或商品继续创建”。
- Vue 帮助入口：`productCreatorManual`，标题“商品创建器手册”。
- 本地真实 Vue 运行组件浏览器：半成品复用、专用上游 disabled、仅有效变量、草稿重开、手动名称恢复、已有商品更换后失效规格清空；1440/1024/768 宽度无页面横向溢出，零页面异常。截图 `/tmp/pc-v8-reuse-{1440,1024,768}.png`。
- Go 全包测试通过；Vue shell 1,310/1,310 测试通过，Vite 构建通过。
- 开发环境部署及真实验收模板：待完成并登记实际证据。
- Van 业务验收：待确认。生产发布另行安排。
