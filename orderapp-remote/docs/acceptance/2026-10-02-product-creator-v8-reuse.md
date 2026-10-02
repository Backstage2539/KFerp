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
- PR #177 合入 develop，功能提交 `4adcb2d4bd524ad42544ca7d9075704c2b673eb2`，首次部署 `efb2a2eb8371e2ec4199de2b565b615f5157be81`。部署执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`，服务器 Go、Vue 1,310 项测试、miniapp 264 项测试、类型检查和构建通过。
- 首次备份 `/opt/stacks/erp/orderapp.backup.deploy-20261002203637-efb2a2eb8371`；回滚镜像 `kferp-orderapp-rollback:development-20261002203637-efb2a2eb8371`。
- 真实模板 #6 V1：`PR-680 V8 挂耳复用验收-20261002`，复制模板 #5 的当前草稿并升级，原模板修订与草稿未改变。入口 `/app/vue-shell?view=productCreator`。
- 运行 #63：复用自制熟豆 #160，停用生豆、烘焙 BOM 和专用工艺，仅创建袋装商品 #1144/BOM #51613、盒装商品 #1145/BOM #51614。运行 #64：复用袋装商品 #1142/已发布规格 #450，停用全部专用上游，仅创建盒装商品 #1146/BOM #51615/规格 #454。
- 两次预览前后业务表数量不变；两次提交合计物料 94→94、商品 574→577、BOM 228→231。没有上游新档案或 BOM。正式步骤为 succeeded/skipped，引用动作及停用关系已记入操作日志 `product_creator_run/commit_configuration` 的 execution_plan。
- 真实浏览器从模板记录继续 #63/#64，预览及提交成功，结果可见，页面无异常。768 宽度下收起工作台菜单后检查表单布局；命名恢复、有效变量和失效规格重选另有本地真实组件交互证据。
- Smoke：未登录 `/app/` 跳转登录（303），创建器 API 未登录 401；登录后 shell/API 200；模块目录含 V8。开发及生产容器健康，生产未参与本次发布。
- 收尾权限测试 RED/GREEN：已有商品投入只要求 products.read，旧版本仍沿用原权限。HTTP 创建器测试全组通过。该修正随收尾提交再次部署 development。
- PR #178 部署 `2f3f411bd344e6579b05a05a467bda1cc796905e`，备份 `/opt/stacks/erp/orderapp.backup.deploy-20261002210019-2f3f411bd344`。重启核对暴露既有启动兼容回填问题：正式提交只创建 3 个下游 BOM，后续启动额外回填 3 个旧式草稿。补充 `TestStartupBackfillDoesNotDuplicateExplicitProductBOM`，RED 为 explicit=2；GREEN 验证已有独立 BOM 不重复、旧式配方正常回填、重复启动幂等。修正与原 repository repair 使用一致的独立有效 BOM 保护条件。
- 回填、PR600 相关 PostgreSQL测试与标准 Go 全包通过。额外运行的两个 PR598 旧测试因外购物料制造校验失败，在未改动的 develop 基线同样失败，未计作通过；日志 `/tmp/pc-v8-backfill-{green,baseline,backend}.log`。
- 多余草稿 #51682/#51648/#51675 来自 system-backfill，未发布、未被默认绑定；修正前快照 `/tmp/pc-v8-startup-before.json`。只对本次验收对象的这些草稿通过现有 BOM 维护 API 停用，日志可查；原 #51613/#51614/#51615 和默认绑定保持不变。历史业务对象未调整。
- Van 业务验收：待确认。生产发布另行安排。
