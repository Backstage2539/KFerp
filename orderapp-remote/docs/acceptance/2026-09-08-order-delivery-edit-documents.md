# PR-641 订单配送、编辑与销售单交付证据

状态：自动验证通过，待双环境上线验收；按本次授权由 Codex 完成业务验收，不直接修写生产数据库。

## 复现与修复

生产订单 SO-20260908-0002 第三行数据库保存的是商品48、规格242（酒心可可），编辑器却按 SKU/规格编号48匹配到商品57（菠浪清甜）。现在按商品、BOM规格和客户引用分别匹配并保留报价快照。客户资料默认类型已改批发，旧订单类型仍是赠送；明确采用下次编辑保存同步的规则。

同一批实现快递/自提/本地送货、订单摘要与来源去重、普通/组合单末页收款区、订单备注、中文换行和紧凑付款底色。

## RED / GREEN

- RED：order-experience.test.js 4个用例失败；PDF旧页码和缺失订单备注用例失败；配送仓储验证缺少非快递模式；页码覆层测试复现额外生成页面。日志保存在 /private/tmp/pr640-*-red.log。
- GREEN：订单前端相关168个测试通过；TestOrderExperience 接口回归实际使用本地 PostgreSQL，覆盖配送保存/重开、默认类型同步、操作日志和拒绝非快递回填；PDF包包含合并单多页和PNG像素底色测试通过。
- 视觉样张：/private/tmp/pr640-artifacts/single.pdf、single.png、combined.pdf、combined.png，全部使用合成测试客户和收款图。
- 常规全量检查：前端 1120/1120、Vue/Vite 构建通过；后端全量通过（含同步最新开发分支后的复验）。数据库专用回归 TestOrderExperience、TestOrderDelivery、TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns、TestOrderSaveAudit 通过。
- 扩展 PostgreSQL 全套存在旧夹具失败：缺 product_bom_spec_authorities、customer_order_production_demands 等表及旧列位置断言；在未修改 origin/develop=3083e187 基线上复现，未把这些结果记为通过。详见 /private/tmp/pr640-baseline-db.log。

## 手册与交付

手册：orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md；Vue订单、销售单预览和设置帮助同步更新。快照新增 order_note、组合单 render_version；复用已有配送字段，无数据库迁移。新生成单据采用 sales-order-last-page-notes-v5，历史归档保留。

目标：develop/main、开发/生产。部署提交、备份路径、健康检查及上线后的页面验收待补充。未直接修改生产订单数据。

## 上线业务回归（2026-09-09）

- 生产 SO-20260908-0002 经正常订单保存接口完成一次打开、保存、重开：墨照啡石（商品58/规格69）、菠浪清甜（商品57/规格48）、酒心可可（商品48/规格242），均5袋，单价56/92/59；商品、规格、数量、单位、价格和完整报价快照保持一致，无需重新选择。按客户默认类型从赠送6同步为批发3，其余订单字段保持不变；操作日志4094记录本次保存。没有直接写生产数据库。
- 开发样例 SO-20260909-0001 已在页面完成自提、本地送货和快递保存重开；本地送货可无物流产品/快递单号确认交付。样例备注明确标记PR641自动验收，最终为快递/未发货。
- 页面检查：桌面状态、费用各一行；390px窄屏摘要宽度与滚动宽度均331px，无横向溢出；报价和生产来源每次打开均折叠。
- 生产组合销售单旧设置的说明X=232mm曾使内容越出A4，现统一限制为页内8mm安全范围，预览坐标一致。真实快照重渲染为3页，个性化说明完整且仅出现在第3页。
- 中文逐字回归另复现旧PDF库换行将80个汉字画成69个；统一使用无损换行测量/绘制后恢复80个，实际说明全文去空白比对通过。RED证据：/private/tmp/pr641-wrap-red.log；最终Go全量：/private/tmp/pr641-v5-backend.log。
- 多收款码与长描述合成样张已检查PDF和PNG，收款区仅在末页/长图末尾，超页容量返回明确错误。旧归档文件保留；新预览/导出按v5版本重新生成。
