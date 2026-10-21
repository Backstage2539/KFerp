# PR-711 客户小程序价格表识别与授权库存

- 状态：首轮生产已交付；补库存与管理端摘要追补验证和独立复审完成，待最终生产更新；业务验收独立。
- 分支：codex/miniapp-customer-catalog-inventory-20261021，基线develop7277f43e。
- 用户澄清：任一库存/无库存代发开启，商品下单自动开启且不能单独关闭；全部代发关闭后恢复普通商品下单配置。

## 生产只读复现

- 运行版本867fe68c；吴国宏488，两入口direct_ship/product_order均绑定publication179 V3.0.68，未改绑或发布价格表。
- stock catalog的price_tables为空；on_demand正常返回已绑定表；product_order接口因有效能力false返回capability not enabled。由新规则修复依赖，不通过真实客户配置补丁绕过。
- 我的库存接口仅有rows、前端硬编码全部五分类；生产仅有吴国宏启用的虚拟成品仓，当前无库存。
- DevTools当前项目 /Users/yiiiple-work/Documents/Codex/2026-10-21/xia/outputs/KFerp-miniapp-production 的RELEASE_INFO为dce7b37c，早于当前服务器版本。后续需更新匹配产物并重新导入验证。
- 证据：/tmp/kferp-pr711-production-read.log、production-api-before.log、production-capabilities.log。

## TDD与验证

- ERP配置界面新增3项失败回归RED /tmp/kferp-pr711-erp-ui-red.log；能力摘要切换模板依赖回归RED /tmp/kferp-pr711-erp-summary-red.log；13项相关界面测试GREEN /tmp/kferp-pr711-erp-ui-green.log。
- 价格表元数据RED：/tmp/kferp-pr711-price-unit-red.log、price-api-red.log；能力依赖RED：capability-unit-red.log、capability-api-red.log；应用/真实PostgreSQL及Mini HTTP GREEN /tmp/kferp-pr711-backend-target-green.log。覆盖已绑/未绑、无库存、具体发布版本、客户与usage隔离、stock/on_demand/两者、全部关闭恢复普通配置、模板及配置false不绕过派生、共享模板不修改。
- 库存范围API RED /tmp/kferp-mini-inventory-api-red.log；兼容服务页实际隔离RED /tmp/kferp-mini-inventory-compat-red.log；真实PostgreSQL GREEN /tmp/kferp-mini-inventory-pg-green.log。覆盖授权空仓元数据、全类型与真实单位、同货主在其他客户/未绑定/停用/public仓仍不可见、列表/详情/ledger/旧URL同范围；授权仓待入库24与未授权仓100分离，physical仍为0。
- Miniapp helper RED /tmp/kferp-mini-catalog-scope-red.log；实际组件运行、全量401项、类型检查和production构建通过，日志 /tmp/kferp-mini-catalog-{green,full,typecheck,production-build}.log。
- Go无数据库环境全包通过 /tmp/kferp-pr711-backend-full.log，触及application/http全包复查通过 /tmp/kferp-pr711-backend-packages-green.log。Vue最终1478项与构建通过；摘要刷新实际组件3项RED→GREEN，/tmp/kferp-pr711-capability-refresh-{red,green}.log，避免合并在途GET导致保存后摘要停留旧值，父门户表单草稿保持。
- 独立库存复审CLEAR，真实schema字段及隐藏入口均核对，7组新隔离回归再次通过 /tmp/kferp-pr711-inventory-independent-review.log。

## 手册

- OP_MANUAL_CUSTOMER_PORTAL.md：自动能力、价格表状态、授权仓与分类、空仓核对及异常处理。
- OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md、OP_MANUAL_CUSTOMER_FULFILLMENT.md：代发与商品下单联动，仍保留独立价格绑定。
- OP_MANUAL_STOCK.md：列表/详情/流水统一客户虚拟仓权限。

## 验收边界

生产诊断仅GET/SELECT。自动依赖通过服务端有效能力派生，不修改共享模板或价格绑定；如需实际下单、库存或结算验收另行记录。服务器部署、DevTools包更新与微信上传/审核/发布分别验证。

## 发布前锚点

- 生产数据库备份 /opt/stacks/erp-production/backups/pr711-before-production-20261021235944.sql.gz，12MB、权限600、gzip校验通过。
- 发布前原生产镜像sha256:8522f5a3a2a772b25c19a5041c78060cdd22f2fb3dce0459e7a78ae74b8743e5，运行867fe68c；开发镜像sha256:3ca7db88b26b4753ef780c389033017872d9af00ea0cb86f630f0cff1e99f465，启动2026-10-21T14:56:44.684207283Z。
- 发布前7张业务表完整行计数/指纹已保留 /tmp/kferp-pr711-production-before-data.log，部署后逐项比较；包括仓库11、成品库存3、价格表绑定12、履约设置2、门户配置27、能力配置56、订单96。

- 独立价格表/权限复审CLEAR，4包相关真实数据库回归再次通过 /tmp/kferp-pr711-independent-price-review.log。未改共享模板、客户价格表或真实库存。

## 合并发布检查

- 功能分支实现ac794209已推送，develop集成8fa4d930；实现树与已验证功能树一致。main仅进度追加冲突，保留完整生产记录后追加PR711，无代码冲突。
- 独立最终合并审查CLEAR：39实现/测试文件完整；4个生产BOM保护文件及15个其他生产独有文件逐字保留，PR689既有验收状态不变。
- main合并后的Go全包、价格表/能力/库存真实PostgreSQL与Mini API、生产BOM回填保护均通过；日志 /tmp/kferp-pr711-main-{backend,database}.log。前端及小程序实现与已验证feature一致。
- 本线程生产交付授权延续，执行标准production脚本，导出匹配包并更新当前DevTools项目；实际业务下单与微信上传/审核/发布分别记录。

## 首次生产交付与追补回归

- 生产首次部署main3e3aaf15，源备份 /opt/stacks/erp-production/orderapp.backup.deploy-20261022001017-3e3aaf15f2a7，回滚镜像同production时间/提交标签。命令 ./deploy_orderapp.sh production。
- 第一轮因root清npm缓存与下一阶段依赖安装重叠中止，未切换生产；确认进程退出并清缓存后，以相同提交完整重跑全部闸门成功。后续仅在所有npm阶段结束后清理临时依赖与缓存，不再与安装并行。
- Vue1478、小程序401、typecheck/production build/24页/96文件、host/image Go均通过；生产登录HTTP200，matching包导出到 /Users/yiiiple-work/KFerp-miniapp-mp-weixin，旧目录保留备份。
- 当前DevTools项目同步到matching3e3aaf15，完整树哈希与原export一致、私有project配置保留，旧包rename保留；日志 /tmp/kferp-pr711-devtools-package.log。Van重新登录吴国宏；UI已观察商品下单识别V3.0.68。
- 七张业务表前后完整行指纹一致：仓库11、成品库存3、价格表绑定12、履约设置2、门户配置27、能力配置56、订单96。新Me自动启用商品下单；stock/on_demand/product_order目录均publication179 V3.0.68，两个价格预览各4规格；资产meta仅吴国宏虚拟成品仓、finished_product分类，无库存行。
- 首次完整冒烟暴露admin HTTP服务层再次套用模板原值，导致与Me不一致；已保留真实RED /tmp/kferp-pr711-admin-read.log，继续修复及补Service+HTTP回归，不把repo单测当作全链路验证。
- 用户随后在实际DevTools测试补库存提交，报告通用“发货信息不完整”错误。实际DevTools400提交参数为customer_stock、当前客户仓、商品7/BOM20/variant559/spec_g0/数量1，收件字段为空；生产只读预览200、can_submit=true、价格71。部署前生产备份恢复到本地独立数据库后复现submit原始错误 customer capability product_order unavailable：customerfulfillment仓库层仍按原始能力拒绝，而Me/preview按代发派生权限放行。该错误被统一提示覆盖，导致误以为缺发货信息。继续统一最终提交的有效能力校验，不改真实客户配置。未替用户创建真实生产订单。

## 补库存最终提交修复（2026-10-22）

- 根因RED：/tmp/kferp-pr712-retained-submit-dependency-red.log；隔离生产副本真实失败 /tmp/kferp-pr712-isolated-submit-raw.log。共用EffectiveFulfillmentCapabilities后，原客户/规格无地址提交成功 /tmp/kferp-pr712-isolated-submit-green.log，仅写本地隔离副本，生产订单未增加。
- 共享模板和客户原始能力不改；Me、管理端详情、履约仓库层最终提交使用同一规则。stock/on_demand任一开启派生product_order，全部关闭恢复普通配置。
- 新购货预览及事务提交均验证当前客户启用的虚拟成品仓；普通ship收件信息必填。成功请求幂等重试优先返回已保存结果，不因后续停仓或关闭stock重新建单或改价；新请求仍拒绝。
- 新API→真实PostgreSQL覆盖BOM规格spec_g0、免地址预览/提交/幂等、原始product_order=false、客户仓隔离与停用、场景关闭、只生待入库权益且实物0及单次操作日志。权限撤销403、商品规格缺项400、地址缺项400分开提示。仓授权及错误提示RED /tmp/kferp-pr712-retained-{api,error,retry}-red.log，五包GREEN /tmp/kferp-pr712-retained-final-green.log。
- 管理员Service+HTTP模板投影补回归RED/GREEN /tmp/kferp-pr711-admin-effective-{red,green}.log，普通cap配置及模板原值保留。
- 真实小程序组件新增无地址补购及普通ship保护；7项定向、403项全量、typecheck和正式构建GREEN /tmp/kferp-pr712-retained-mini-{green,full,typecheck,build}.log。前端原请求字段正确，未为伪造RED修改正确业务逻辑。
- 独立追补审查CLEAR，覆盖最终能力链、仓授权、幂等、错误映射、历史金额与事务。新数据库备份 /opt/stacks/erp-production/backups/pr711-retained-before-production-20261022004018.sql.gz，12MB、600、gzip校验通过；七表发布前指纹仍与首次发布前完全一致。
