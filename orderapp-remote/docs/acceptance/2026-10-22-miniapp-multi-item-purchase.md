# PR-711 / DEV-828 多商品留厂购货提交修复

- 日期：2026-10-22（Asia/Shanghai）；分支codex/miniapp-multi-item-retained-purchase-20261022，基线develop19fb15d3，生产运行5e65091c。
- 用户报告：一件代发→商品下单，曜石2.0 454g袋装×1、初晓227g袋装×4、曜石2.0 227g袋装×4，提交购货订单internal error。
- 生产DevTools已观察三行及326元估算；网络面板此前未开启，无历史请求记录，未为抓请求点击真实提交。生产仅SELECT核对旧唯一约束。
- 隔离生产副本：preview can_submit=true/item_count3，最终提交SQLSTATE23505，约束customer_direct_ship_request_i_request_id_product_id_spec_g_key。旧键(request_id,product_id,spec_g)无法区分同商品canonical BOM两规格（spec_g均0）。现有schema迁移按完整长名DROP，未命中PG自动截断名。日志 /tmp/kferp-pr713-isolated-multi-raw.log、/tmp/kferp-pr711-multiline-production-constraints-before.log。
- 范围：精准移除已废弃旧列组合的唯一约束，保持新BOM与legacy唯一索引、其他约束、历史单据/金额/库存/客户权限；同一事务完整保存多行、权益及操作日志。
- 状态：实现与自动验证完成，独立复审进行中；待集成发布，未创建生产测试单。

## TDD与隔离数据验证

- 先RED：PG自动生成的旧约束名未被删除；真实API三行提交500。日志 /tmp/kferp-pr713-multi-red.log。
- 最小实现：schema.go按具体表、unique约束及精确三列集合查旧约束，用引用后的实际名字删除，不使用CASCADE；保持canonical(request_id,product_id,bom_spec_id)和legacy(spec_g、bom_spec_id0)的partial唯一索引、PK/FK/行号唯一约束。无业务DML。
- GREEN：迁移可重复执行；历史行保持；不同canonical规格和不同legacy克重可共存，同规格重复仍受约束；实际API三行201，各行订单/金额/待入库权益和客户仓一致，操作日志单次，幂等不增单；强制末尾审计失败整单/明细/权益/日志全部回滚。日志 /tmp/kferp-pr713-multi-green.log。
- 真实生产副本仅执行相同精准DO迁移、不EnsureSchema，迁移前后业务计数orders97/lines173/requests2/rights1/audit7372保持一致；之后用户同三行preview3行、submit成功。日志 /tmp/kferp-pr713-isolated-{before-migration,after-migration,multi-green}.log；仅本地隔离库写测试单。
- 前端业务组装原本正确，本次只补实际组件三行选择/包装/数量、preview与create完整一致、使用最新quote回归。定向8、全量404、typecheck及production build通过，日志 /tmp/kferp-pr713-multiline-mini-{targeted,full,typecheck,build}.log。
- 全Go无数据库通过 /tmp/kferp-pr711-multiline-full-go.log。
- 数据库新备份 /opt/stacks/erp-production/backups/pr711-multiline-before-production-20261022010540.sql.gz，12MB、600、gzip校验通过；16张业务表发布前指纹 /tmp/kferp-pr711-multiline-before-data.log。生产已有用户实际购货1袋，订单基线97，保留该业务记录。
