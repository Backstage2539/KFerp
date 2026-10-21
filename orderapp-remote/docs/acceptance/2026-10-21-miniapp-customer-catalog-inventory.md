# PR-711 客户小程序价格表识别与授权库存

- 状态：实现、自动化验证与独立复审完成；生产交付和业务验收待完成。
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
