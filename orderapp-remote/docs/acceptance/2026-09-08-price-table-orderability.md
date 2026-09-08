# PR-638 价格表生成与录单可用性一致

状态：实现及本地验证完成，生产回补交付处理中，待 Van 验收。

## 生产只读复现

2026-09-08，生产代码 ce58d4ec。咖啡生豆价格表 #39 / V3.0.8 已发布，包含 7 个父商品、14 条价格。快照仍使用旧子商品 SKU，没有 BOM 规格身份。生产 GET `/app/api/order/form` 返回 200，商品总数 34，其中咖啡生豆 0、对应 BOM 规格 0；7 个父商品在 `product_bom_spec_authorities` 中均 configured=false。录单按该视图及当前默认已发布 BOM 规格过滤，而价格表之前只校验旧 SKU 存在和价格，导致发布后不能使用。

全程未修改生产商品、BOM、价格、发布记录或订单。生豆商品仍需补齐有效 BOM 规格并重新选规格、发布新版，修复不会凭空创建业务配置。

## RED/GREEN 和实现

- 新服务测试先复现：单表校验返回 nil 并进入发布；无效草稿直接命中 PDF 缓存；预校验 HTTP 路由 404。均已修复。
- 服务仓储接口强制实现录单可用性校验，单表和多表整组发布保存前执行；检查选择及实际打印/价格内容，省略选择数组也不能绕过。
- 共享录单 `product_bom_spec_authorities` 视图，核对商品启用、当前默认已发布 BOM 的具体规格和版本、名称与库存单位及客户范围。旧子商品规格明确要求重选当前 BOM 规格。
- 只读 POST `/api/costing/bean-list/validate-orderability` 在生成 PDF 保存草稿前执行；服务端草稿 PDF 生成和缓存读取均再次校验。历史 published/withdrawn/archived 文件按冻结快照读取，渲染样式和缓存键不变。
- 多表任一表错误会阻止整组写入。草稿编辑保存仍可进行。新增校验本身不写业务记录；原发布/草稿/PDF写入沿用现有操作日志。
- 真实 PostgreSQL 使用当前 authority view：无 BOM、发布但无变体、旧子规格、过期变体、停用、跨客户拒绝；有效规格和匹配客户通过；预校验 publication/audit 数均为 0。
- Vue 定向测试及 Chrome 合成目录验证：预览区显示具体商品/BOM修复提示，点击发布被阻止，无运行时错误。未进行生产业务写入验收。

## 证据与手册

本机日志及截图 `/private/tmp/kferp-pr638-evidence`。手册更新 `OP_MANUAL_COSTING.md`、`OP_MANUAL_ORDER_SALES.md`，Vue 提示同步。功能分支 `codex/price-table-orderability-20260908`，从 d69b0bb3 建立；仅将本修复回补当前生产代码，不带入其他开发功能。

生产回补分支 `codex/price-table-orderability-production-20260908` 基于 `ce58d4ec`；回补未包含多表功能，生产单表发布/生成校验、后端完整门禁、Vue 1084 项及构建、真实 PostgreSQL 当前规格检查通过。开发分支（含整组发布）Vue 1116 项与后端门禁通过。
