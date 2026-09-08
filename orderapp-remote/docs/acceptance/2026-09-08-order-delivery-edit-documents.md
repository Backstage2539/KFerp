# PR-641 订单配送、编辑与销售单交付证据

状态：自动验证通过，待双环境上线验收；按本次授权由 Codex 完成业务验收，不直接修写生产数据库。

## 复现与修复

生产订单 SO-20260908-0002 第三行数据库保存的是商品48、规格242（酒心可可），编辑器却按 SKU/规格编号48匹配到商品57（菠浪清甜）。现在按商品、BOM规格和客户引用分别匹配并保留报价快照。客户资料默认类型已改批发，旧订单类型仍是赠送；明确采用下次编辑保存同步的规则。

同一批实现快递/自提/本地送货、订单摘要与来源去重、普通/组合单末页收款区、订单备注、中文换行和紧凑付款底色。

## RED / GREEN

- RED：order-experience.test.js 4个用例失败；PDF旧页码和缺失订单备注用例失败；配送仓储验证缺少非快递模式；页码覆层测试复现额外生成页面。日志保存在 /private/tmp/pr640-*-red.log。
- GREEN：订单前端相关168个测试通过；TestOrderExperience 接口回归实际使用本地 PostgreSQL，覆盖配送保存/重开、默认类型同步、操作日志和拒绝非快递回填；PDF包包含合并单多页和PNG像素底色测试通过。
- 视觉样张：/private/tmp/pr640-artifacts/single.pdf、single.png、combined.pdf、combined.png，全部使用合成测试客户和收款图。
- 常规全量检查：前端 1119/1119、Vue/Vite 构建通过；后端全量通过（含同步最新开发分支后的复验）。数据库专用回归 TestOrderExperience、TestOrderDelivery、TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns、TestOrderSaveAudit 通过。
- 扩展 PostgreSQL 全套存在旧夹具失败：缺 product_bom_spec_authorities、customer_order_production_demands 等表及旧列位置断言；在未修改 origin/develop=3083e187 基线上复现，未把这些结果记为通过。详见 /private/tmp/pr640-baseline-db.log。

## 手册与交付

手册：orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md；Vue订单、销售单预览和设置帮助同步更新。快照新增 order_note、组合单 render_version；复用已有配送字段，无数据库迁移。新生成单据采用 sales-order-last-page-notes-v3，历史归档保留。

目标：develop/main、开发/生产。部署提交、备份路径、健康检查及上线后的页面验收待补充。未直接修改生产订单数据。
