# PR-711 / DEV-828 多商品留厂购货提交修复

- 日期：2026-10-22（Asia/Shanghai）；分支codex/miniapp-multi-item-retained-purchase-20261022，基线develop19fb15d3，生产运行5e65091c。
- 用户报告：一件代发→商品下单，曜石2.0 454g袋装×1、初晓227g袋装×4、曜石2.0 227g袋装×4，提交购货订单internal error。
- 生产DevTools已观察三行及326元估算；网络面板此前未开启，无历史请求记录，未为抓请求点击真实提交。生产仅SELECT核对旧唯一约束。
- 隔离生产副本：preview can_submit=true/item_count3，最终提交SQLSTATE23505，约束customer_direct_ship_request_i_request_id_product_id_spec_g_key。旧键(request_id,product_id,spec_g)无法区分同商品canonical BOM两规格（spec_g均0）。现有schema迁移按完整长名DROP，未命中PG自动截断名。日志 /tmp/kferp-pr713-isolated-multi-raw.log、/tmp/kferp-pr711-multiline-production-constraints-before.log。
- 范围：精准移除已废弃旧列组合的唯一约束，保持新BOM与legacy唯一索引、其他约束、历史单据/金额/库存/客户权限；同一事务完整保存多行、权益及操作日志。
- 状态：已合并main并部署生产，自动化、独立复审与生产只读验证完成；未创建生产测试单，客户当前三行草稿保留。

## TDD与隔离数据验证

- 先RED：PG自动生成的旧约束名未被删除；真实API三行提交500。日志 /tmp/kferp-pr713-multi-red.log。
- 最小实现：schema.go按具体表、unique约束及精确三列集合查旧约束，用引用后的实际名字删除，不使用CASCADE；保持canonical(request_id,product_id,bom_spec_id)和legacy(spec_g、bom_spec_id0)的partial唯一索引、PK/FK/行号唯一约束。无业务DML。
- GREEN：迁移可重复执行；历史行保持；不同canonical规格和不同legacy克重可共存，同规格重复仍受约束；实际API三行201，各行订单/金额/待入库权益和客户仓一致，操作日志单次，幂等不增单；强制末尾审计失败整单/明细/权益/日志全部回滚。日志 /tmp/kferp-pr713-multi-green.log。
- 真实生产副本仅执行相同精准DO迁移、不EnsureSchema，迁移前后业务计数orders97/lines173/requests2/rights1/audit7372保持一致；之后用户同三行preview3行、submit成功。日志 /tmp/kferp-pr713-isolated-{before-migration,after-migration,multi-green}.log；仅本地隔离库写测试单。
- 前端业务组装原本正确，本次只补实际组件三行选择/包装/数量、preview与create完整一致、使用最新quote回归。定向8、全量404、typecheck及production build通过，日志 /tmp/kferp-pr713-multiline-mini-{targeted,full,typecheck,build}.log。
- 全Go无数据库通过 /tmp/kferp-pr711-multiline-full-go.log。
- 数据库新备份 /opt/stacks/erp-production/backups/pr711-multiline-before-production-20261022010540.sql.gz，12MB、600、gzip校验通过；16张业务表发布前指纹 /tmp/kferp-pr711-multiline-before-data.log。生产已有用户实际购货1袋，订单基线97，保留该业务记录。

## 生产交付（2026-10-22）

- feature6471a639已推送，develop7684c047，生产main运行c6a1d8b86ad115af4da0f39e32c0f9d52b7bf387。独立main审计CLEAR：本次9文件与feature一致，ACTIVE仅追加冲突保留完整既有回执；4个BOM保护文件及18个本次未触及的生产独有文件完整保留。
- 合并后全Go、真实PostgreSQL多行API/权限/BOM保护均GREEN /tmp/kferp-pr711-multiline-main-{go,pg}.log。./deploy_orderapp.sh production成功；服务器Vue1478、mini404、typecheck/build、24页96文件、host/image Go及HTTP readiness通过。
- 源备份 /opt/stacks/erp-production/orderapp.backup.deploy-20261022011104-c6a1d8b86ad1；回滚镜像 kferp-orderapp-rollback:production-20261022011104-c6a1d8b86ad1。运行镜像sha256:2a0323166aa5772c545b6f278aa7a824d89d5f7d810ff82461048d39d59d99e0，启动2026-10-21T17:20:48.860097555Z、重启0。开发镜像与启动时间不变。
- 生产SELECT确认旧legacy三列唯一约束不再存在，PK、FK、行号唯一及canonical/legacy两个partial唯一索引保留；/tmp/kferp-pr711-multiline-production-constraints-{before,after}.log。
- 生产同三行免地址预览200、can_submit=true、总价326，无运费；逐行身份58/69/1167×1、7/19/558×4、58/68/1166×4保持独立。价格绑定179 V3.0.68、Me/admin/最终仓库层有效能力、授权库存、匿名401和页面/手册200均通过。只读预览与接口未落生产测试单。
- 16张业务表完整行计数/指纹前后一致，包括订单97/明细173、请求及明细各2、库存权益1、audit7372/order_audit166；保留客户此前实际单商品购货。第一轮smoke仍假设旧入口零库存，因真实已购1袋而失败，已保留first日志并改为授权仓范围断言后全通过，无产品代码改动。
- 匹配miniapp正式包已导出。与当前DevTools项目对比，全体运行文件/manifest逐字相同，只有RELEASE_INFO不同；因此仅备份并更新发布标记，完整树hash与export一致，private配置不变，不重新加载正在编辑的页面。实际界面仍登录吴国宏，三行商品及数量1/4/4、提交按钮保留。旧提示来自上次失败提交，未点击生产真实提交。
- DevTools旧发布标记备份 /Users/yiiiple-work/Documents/Codex/2026-10-21/xia/outputs/KFerp-miniapp-production.release-info-backup-ea9c9666ae5c48c78efc2a252f3b5727；包校验 /tmp/kferp-pr711-multiline-devtools-package.log。未上传、审核或发布微信版本。
- 持久证据 /Users/yiiiple-work/Documents/Codex/2026-10-22/multi-item-retained-purchase/validation：RED/GREEN、隔离副本成功、部署/运行/迁移/业务指纹、包全树核对与草稿截图。所有运行验证后仅清可重建构建缓存，保留业务数据、源备份和带标签回滚镜像。
- 本回执仅文档记录，运行与RELEASE_INFO保持c6a1d8b8，不为回执再重启服务。
