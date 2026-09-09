# PR-644 商品分类移动与 BOM 草稿、模板重套

日期：2026-09-09。验收人：Van。自动化验证已完成，人工业务验收待 Van；本次仅合并 develop，不部署、不调整生产业务数据。

## 范围与实现

- 独立功能分支：`codex/bom-category-fixes-20260909`，初始基线 `0e0408f275d04be447de7395251b6098d1c985df`。已同步开发分支 `76c1232880fc713f211e2ef39252a662d5b476a2`，本次实现提交 `dd479663`。
- 分类移动逐项消费原 assignment 接口返回，直接替换本地分类关系、更新列表及数量。移除工厂商品移动后的整页 `loadAll`；未分类只删除已知关系。批量失败保留失败勾选、显示成功/失败数量，立即显示正在移动并禁止重复提交。客户目录原有独立接口保持兼容。
- BOM 版本详情查询补齐四个主体组件字段，与列表共用解析器。复制完整保留模板与主体组件来源，并在提交前读取返回对象；同 BOM 版本编号加锁。前端成功后选中新草稿并加载规格。
- 新增 `production_bom_specs.source_spec_template_id/source_spec_template_key` 及模板、模板键、单位组合唯一索引。模板规格先匹配来源身份；来源或单位变更时新建 ID 和唯一内部键，重复重套稳定复用。旧规格只在当前操作有明确版本来源时采用映射，初始化不批量回填历史。
- 重套整组替换当前草稿、保留发布历史；同身份规格保留条码。创建及复制维护来源映射；整组审计记录新旧规格 ID、键、单位和模板来源。
- 编辑表单改变产出商品时复用替代草稿接口，同时提交目标商品、模板和主体组件。有已发布/归档历史的 BOM 允许选中草稿作为替代来源，来源 BOM 与版本均不改写。
- 已归档模板显示可操作错误并刷新候选、清空无效选择，不自动选新版。Vue 当前操作区域同步反馈。

## RED 证据

1. 分类新测试最初 4 项失败：保存后仍等待 `loadAll`，未分类额外读取，部分失败没有逐项保留，重复点击再次发送。
2. 真实 PostgreSQL 复制接口最初返回 HTTP 400：`number of field descriptions must equal number of destinations, got 18 and 22`；失败后版本数已从 1 变为 2。
3. 9 袋装规格重套 2 kg 规格最初触发旧规格单位不可变更错误，未能替换草稿。新 UI 输出变更与重复点击测试最初失败。
4. 补测当前草稿作为替代来源时，原接口返回 HTTP 400：`published source production BOM version not found`；修复后同一请求返回 HTTP 200。

## GREEN 证据

真实数据库为本机 PostgreSQL 16 独立库 `kferp_bom_category_test`，测试各用独立 schema 并清理；没有对开发或生产数据库执行本次测试。

```sh
cd orderapp-remote
ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_bom_category_test?host=/tmp' go test ./internal/infrastructure/postgres/bom -run 'TestDraftCopyResponseAndRollback|TestTemplateReapplyReplacesNine|TestTemplateSpecIdentityLegacy|TestReapplyProductBOMSpecTemplate|TestCreateProductionBomVersionPreservesSpecTemplateProvenance' -count=1 -v
```

上述 6 个顶层测试、3 个复制子场景全部 PASS，0 SKIP。覆盖真实 HTTP 提交、提交后独立读取、主体物料/主体商品/规格、模板来源完整及失败回滚。9→2 场景覆盖新默认项、主物料与 kg 用量、重复重套 ID 不变、已发布 9 袋装规格不变；校验失败、末尾响应读取失败、模板归档均不留下部分修改；替代 BOM 更换为另一个商品，错误分组校验整笔回滚。另验证历史来源兼容、跨模板同名键、单位变更新身份、条码、复制映射、初始化幂等且不重写历史，以及新旧身份审计。

- `category-move-feedback.test.js`：5 项通过；无关读取永不返回时仍完成反馈（比延迟 10 秒更严格），并覆盖普通分类、未分类、批量部分失败、取消、防重复提交。
- `bom-draft-feedback.test.js`：4 项通过；执行真实 Vue 页面操作函数，验证复制选择和加载新草稿、单次在途提交、输出变更替代请求、归档模板刷新且不自动选择新版。
- `scripts/verify_kferp.sh frontend`：1133/1133 PASS，0 SKIP；Vite 生产构建 PASS。仅既有分包体积提示。
- `scripts/verify_kferp.sh backend`：全部 Go 包通过标准门禁；此标准运行不设置数据库 DSN，其数据库测试不能计入上述真实数据库通过数。
- `scripts/verify_kferp.sh changed`：合并前检查空白及冲突标记。
- 临时完整日志：`/tmp/kferp-pr644-db-focused-final.log`、`/tmp/kferp-pr644-frontend-final.log`、`/tmp/kferp-pr644-backend-final.log`。

## 额外数据库整包检查的已知边界

额外运行真实数据库 BOM 整包，仍有 11 个顶层失败；独立未修改基线 `0e0408f2` 的同环境整包有 14 个失败，这 11 个全部属于既有失败。本次消除版本返回、模板重套及条码相关的 3 个原失败，没有新增失败。其余错误涉及旧 fixture 的取得方式/规格模式/默认绑定前置条件及旧并发锁期望，本次未扩大范围修改。

- `TestPR598MaterialOutputRepositoryAndCompatibilityMigrationPostgres`
- `TestPR598DefaultBindingSwitchRejectsTypedGraphCyclesPostgres`
- `TestProductComponentSelectedSpecUnitGovernsBOMDraftAndPublishPostgres`
- `TestPublishProductBOMVersionGuardsOnlyRemovedSpecsPostgres`
- `TestBindProductProductionBomUsesExplicitStructureAndSetsDirectIdentityPostgres`
- `TestConcurrentProductBOMDraftMutationAndPublishAvoidsDeadlockPostgres`
- `TestProductBOMRequiresSpecGroupFailsClosedWhenMigrationTableMissingPostgres`
- `TestUpdateProductionBomToMaterialOutputClearsDraftSpecGroupPostgres`
- `TestProductionBomDraftWorkspaceReplacesMaterialItemsIncludingEmptyListPostgres`
- `TestReplacementDraftKeepsPublishedSourceAndRollsBackAtomicallyPostgres`
- `TestCreateProductionBomRejectsDeprecatedOutputWithStableErrorPostgres`

整包另外有 2 个需要专门环境开关的迁移测试未执行，不计通过：
- `TestPR607AppliedCloneSurvivesLegacyBindingRepair`
- `TestRepairLegacyProductionBomBindingsPostgresOnce`

证据：`/tmp/kferp-pr644-db-all-final.jsonl`、`/tmp/kferp-pr644-baseline-bom.log`。不得将整包结果描述为全部数据库测试通过。

## 手册、记录与交付边界

- 手册：`docs/OP_MANUAL_PRODUCTION.md`、`docs/OP_MANUAL_COSTING.md`；BOM Vue 编辑区内置帮助与结果提示已同步。手册由现有 Vue 手册入口读取单一来源。
- PR/DEV：`PR-644-BOM-CATEGORY-FEEDBACK` 为 review，三个 DEV 自动实现完成；需求种子随代码交付，本次未部署，因此不会立即刷新线上 PR/DEV 页面。
- 不自动发布 BOM，不清理既有重复草稿，不替用户更改黄波旁水洗的实际产出或配置。未进行线上浏览器业务验收。
- 合并 develop 后仍需另行授权部署，来源列与索引届时才通过既有 schema 初始化应用。Van 再做真实商品业务验收。

## 集成复核

同步开发分支的 BOM 查询连接释放修复后，再次运行：6 个本次重点 PostgreSQL 测试全部通过、0 跳过；单连接池读取测试（含模板/版本两个子场景）通过；Go 全包标准检查通过；前端 1133/1133、构建和变更检查通过。合并代码提交 `91ab44e2`，没有冲突。日志分别为 `/tmp/kferp-pr644-db-integrated.log`、`/tmp/kferp-pr644-pool-integrated.log`、`/tmp/kferp-pr644-backend-integrated.log`、`/tmp/kferp-pr644-frontend-integrated.log`。本次使用 GitHub PR 合并到 develop；部署仍未执行。
