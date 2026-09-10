# PR-648 生产计划、需求归并与多层 BOM

业务范围：订单驱动生产及独立半成品备货。交付目标为开发环境；生产发布和 Van 业务验收另行进行。

## 实现

- Vue 预览在草稿为空、加载、失败、取消选择和撤销后安全渲染，失败可重试；来源仓仅在草稿存在时展示。
- 商品档案 / 规格分组展示件数、可换算重量、库存覆盖、缺口及结构化客户订单数量，保留底层冻结身份；分页和三态勾选保留实际需求选择，刷新重新校验状态。
- 新增只读 `POST /api/production-plans/preview`，订单旧预览复用同一展开；`source_type=stock` 以物料档案单位及默认已发布 BOM 生成独立备货草稿。
- 继续递归展开多层 BOM。现货优先，其次同物料、BOM 版本、工艺、货主的已下达/生产中剩余产出，最后新建缺口生产。来源仓及供应目标仓均按原仓库规则验证，各供应工单独立保留目标仓；草稿仅记录建议，不锁占在途。创建时间、工单编号和 ID 保证分配顺序稳定。
- 提交以事务锁重新验证供应余量及所选来源仓批次。竞争失败保留草稿，可刷新供应后重新拆分上游产能。
- 物料产出支持部分及最后一次入库，每次记录投入、产出、独立批次、库存单据和增量成本。工序实绩保留物料计量单位；部分入库不会提前关闭工单。
- 已预留的各组件批次足额且质检/工序条件满足后，包装可提前开始。每批先兑现下游分配，超产留库；少产显示待补产数量。取消下游释放分配，仍有未履行关联的上游禁止直接取消。
- 历史冻结 BOM/工单/批次不改写；新增分配兑现计数不使已足额预留的历史依赖重新阻塞。备货自身不推进无关销售订单。
- 操作日志覆盖创建、刷新供应、提交、入库及取消；计划行增量保存订单行追溯，独立备货通过供应关联追溯后续订单。

## 数据变更

只做增量建表/加列：`production_supply_allocations`、`production_request_keys`、`production_material_receipts`，计划展开图、计划行订单来源及依赖兑现计数。未迁改历史 BOM、库存或成本数据。创建与分批入库使用请求标识，提交使用已锁定计划状态保证重复调用不生成第二批工单。

## 验证证据

1. Vue 实际模板渲染 RED：有预览但 `currentPlan=null` 时读取 `component_sources` 抛错。修复后覆盖加载、失败、取消状态；执行实际 mounted 回调验证带 `plan=1&selected=...` 刷新路径。订单表实际渲染验证 `quantity`、客户和订单行 ID。分组、混合销售单位、同名不同商品、半选和历史异常共 5 个针对测试通过。
2. 领域 RED：新增在途展开方法/字段不存在；GREEN 验证 20kg 需求按 5kg 现货、8kg 在途、7kg 新生产拆分，原料只按 7kg 缺口展开，共享供应不重复计入。
3. 隔离 PostgreSQL：30 个顶层相关 API 测试通过，0 失败、0 跳过。覆盖独立备货只读预览与幂等创建、同商品各规格冻结数量、客户货主隔离、现货与另一可用仓在途合并、跨计划并发提交、刷新保留草稿、取消释放、多层/多规格共享上游、缺料阻止提交、历史 BOM 变体冻结、分批入库提前包装、少产/超产及冻结目标仓。
4. 完整批次成本案例：30kg 独立熟豆备货，分两次入库 22.7kg、7.3kg，实际投入累计 37.5kg；投入批次单价 50 元/kg，工序费用累计 30 元，产出批次均 63.5 元/kg，累计成本 1905 元。第一批足额预留后包装提前开工；包装消耗 22.7kg 熟豆和 100 个包材，实际材料成本 1541.45 元，成品入库保留订单行，剩余熟豆 7.3kg。
5. 额外 RED/GREEN 修复：最后入库覆盖工序 kg 实绩、跨仓现货导致漏算在途、上游追溯带入无关订单。各针对验证通过。
6. 标准 `scripts/verify_kferp.sh all` 通过：全部 Go 测试、Vue 1164/1164、生产构建、冲突/空白检查。标准 Go 门禁未配置测试数据库；数据库覆盖由上面的显式隔离测试提供。

隔离数据库使用本机 PostgreSQL，每次独立 schema，结束清理；没有对在线业务库创建测试订单、计划或库存。

### 扩展旧测试的限制

额外运行全部生产数据库测试并对比原版 `origin/develop=2bd15db4`。原版公共测试订单缺 `receiver_name` 等现行字段；仅修复公共 fixture 后，原版仍有 31 个 HTTP 顶层失败及 1 个仓储失败，主要是旧测试未选择组件来源仓、旧快照缺 SKU 身份和客户加工旧状态。此次对相关多层、BOM 变体及库存单据测试补齐显式来源仓动作后，上述 30 项通过。未把剩余旧测试失败宣称为通过，也未放宽生产校验来迎合旧 fixture。

本次命令的本地日志：`/private/tmp/pr648-frontend-red.log`、`pr648-domain-red.log`、`pr648-receipt-units-red.log`、`pr648-mixed-warehouse-red.log`、`pr648-order-render-red.log`、`pr648-trace-red.log`、`pr648-verified-api.jsonl`、`pr648-complete-chain.log`、`pr648-all-gate.log`、`pr648-baseline-fixture-api.log`。

## 手册与验收

生产手册 `OP_MANUAL_PRODUCTION.md`、库存手册 `OP_MANUAL_STOCK.md` 和统一索引已更新。生产计划“操作说明”打开生产手册，沿用返回来源导航。

Van 验收入口：生产流程 → 生产计划。建议核对真实订单分组与规格数量，再验收半成品备货、分批入库及包装执行。自动验证、发布技术核对和 Van 业务验收分别记录；本记录不表示业务验收完成。

## 开发发布记录

- 主实现 GitHub PR #107，功能提交 `bbba14b3df6bde222146bbc585d1c2e93ed20669`，开发合并 `c243cc190a50fe0d31f722bed35e8685b3715981`。
- 首次 `./deploy_orderapp.sh development` 返回 `Release completed`；服务端 Go、Vue 1164/1164、miniapp 238/238、类型检查和构建通过。登录页、需求进度、计划、工单、备货预览校验及手册可访问。
- 发布后只读需求查询返回 HTTP 500 / SQLSTATE 42883。开发库 `orders.source` 文本列遮蔽 JSON 数组元素的临时别名；已添加带此真实字段的隔离 API 回归测试，RED 重现相同错误，改用显式 `demand_source.value` 后 GREEN。未修改业务数据或字段类型。补充修复后 29 项隔离 API 全部通过，完整后端门禁通过。
- 发布前数据库备份：`/opt/stacks/erp/backups/pr648-planning-predeploy-20260910214240.dump`（15,777,910 bytes）。首次源代码回滚目录 `/opt/stacks/erp/orderapp.backup.deploy-20260910215318-c243cc190a50`，镜像 `kferp-orderapp-rollback:development-20260910215318-c243cc190a50`。
- GitHub PR #108 修复名称遮蔽后，开发 `14a0ae32f4cf655dca4c8665440115943f702ff8` 发布成功；只读需求恢复 HTTP 200，5 个商品组 / 9 条需求。源文件 SHA-256 与本地一致，容器和 RELEASE_INFO 对应新版本。该版回滚目录 `/opt/stacks/erp/orderapp.backup.deploy-20260910221208-14a0ae32f4cf`。
- 实际选择预览发现已有 BOM 缺物料明细，后端已明确报错。补充列表校验同一冻结 BOM 身份，配置异常逐行显示并禁止勾选，其他需求继续保留；旧选择参数刷新显示错误且不建草稿。新增 API 测试 RED/GREEN 后，30 项隔离 API / 全后端门禁通过。最终开发发布与只读核对见下节。
- 开发环境原有 `DISABLE_BASIC_AUTH=true`，无凭证生产计划 API 返回 200；未改动该环境配置，不将历史清单中的“未登录 401”宣称通过。正式环境该变量未设置且容器启动时间未改变。
-自动浏览器连接未完成现场交互核验；实际 Vue 模板渲染测试通过，Van 页面和业务验收仍待进行。正式环境和微信上传发布不在本次范围。

补充日志：`/private/tmp/pr648-live-query-red.log`、`/private/tmp/pr648-verified-api-followup.jsonl`、`/private/tmp/pr648-followup-backend.log`、`/private/tmp/pr648-dev-deploy.log`。

补充配置校验日志：`/private/tmp/pr648-bom-selection-red.log`、`/private/tmp/pr648-verified-api-final.jsonl`、`/private/tmp/pr648-final-backend.log`。

## 最终交付

- 功能分支 `codex/production-planning-multilevel-20260910`，最终功能提交 `8a0d6cac31395fe768275a89db442196c92c3816`。GitHub PR #107 / #108 / #109 均已合并；最终开发发布提交 `fe4036f7f46eee74b746313444a5c426519a4933`。
- 2026-09-10 22:36（Asia/Shanghai），从独立干净 `develop`、HEAD 与 `origin/develop` 一致的工作目录执行 `./deploy_orderapp.sh development`，退出 0，明确返回 `Release completed`。完整服务器 Go 检查、镜像内 Go 检查、Vue 1164/1164、miniapp 238/238、类型检查和构建全部通过。
- 开发入口：<https://dev.qacoohee.com/app/>。认证只读检查：应用、PR-648 进度、需求、计划、工单和生产手册均 HTTP 200；备货空输入按契约返回 HTTP 400。需求为 5 商品组 / 9 行，其中 3 行可选、6 行配置异常禁选；正常需求的原 GET 选择预览 HTTP 200、`plan_ready=true`，统一 POST 预览 HTTP 200、2 个计划预览行。在线核验未创建草稿、工单或库存记录。
- 开发运行镜像 `sha256:7aabc6bae6cb190f9d5dfc67d24f6a829771fcb8a2c53f2c15777a8c6183b277`，启动时间 `2026-09-10T14:36:09.535366988Z`；服务日志无新增错误。生产容器仍为 `2026-09-10T02:18:21.538861941Z` 启动的原镜像，没有重启生产服务。
- `RELEASE_INFO` 对应 `fe4036f7` / development；需求实现文件本地与服务器 SHA-256 同为 `02ac41c61099b8b8b0df973cec5e1a3a032a33dd1b550342ab323111b9e48458`。
- 当前回滚源 `/opt/stacks/erp/orderapp.backup.deploy-20260910222928-fe4036f7f46e`，回滚镜像 `kferp-orderapp-rollback:development-20260910222928-fe4036f7f46e`。初次发布前数据库备份保留在上文路径。
- 微信开发构建已按发布脚本导出到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，14 个页面 / 56 个文件校验通过；没有上传或发布微信版本。
- 最终日志 `/private/tmp/pr648-dev-deploy-delivered.log`、`/private/tmp/pr648-dev-smoke-delivered.log`。期间 SSH 短暂断连后已恢复，最终认证 API、容器和源文件核验已完成。开发环境既有免鉴权配置及扩展旧数据库测试的基线失败仍按上文单独记录。
- Van 浏览器页面与真实业务验收待进行，PR/DEV 保持 review。本次开发合并/发布占用已释放。此节为发布后的文档记录；后续文档提交不改变已部署应用版本。
