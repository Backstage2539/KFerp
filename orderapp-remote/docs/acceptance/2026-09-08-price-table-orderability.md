# PR-638 价格表生成与录单可用性一致

状态：实现、生产部署和只读验证完成；待 Van 补齐生豆商品配置及业务验收。

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

## 合入与生产交付

- 开发实现提交 `7527d1b3545f5dd139a7efcee36a278b64aa8a83`，GitHub PR #77 合入 develop `5201c0a9b3ec2d8eb1d0e2ce2b1ea7c4905bf3b9`。开发分支包含整组发布校验，Vue 1116 项、完整 Go 门禁及真实 PostgreSQL 校验通过；本次未部署 development。
- 生产回补基于当时 main/生产 `ce58d4ec90df6fe2ea079f650362ccdacf7046ec`，提交 `b23a2468c43e9037fa1ee79484b514810473fb3c`，GitHub PR #78 合入 main `a9dd129ef41119d1cd7e82630440c62e4660df98`。回补仅包含单表生成/发布校验、提示、测试与手册，未带入开发分支的多表功能。
- 独立发布目录 `/private/tmp/kferp-pr638-production-release`：干净 main 与 origin/main 一致，运行 `./deploy_orderapp.sh production`，退出码 0。服务器全 Go 门禁、Vue 1084 项及构建、小程序 235 项、类型检查、production 构建和镜像内复核通过。
- 生产应用、数据库及文档转换容器均 running，数据库 healthy，应用重启次数 0；登录入口 HTTP 200；受保护订单表单未认证 401、认证 200。9 个关键源文件指纹与发布提交一致，容器手册及 Vue 文件与服务器一致，二进制与编译网页包含新校验接口。
- 只读 POST `/app/api/costing/bean-list/validate-orderability` 使用 #39 原始 config/content，返回 HTTP 400，准确包含父商品 #97、#100、#102、#104、#105、#106、#911 的名称及“未配置可用于录单的默认已发布 BOM 规格”原因。未调用发布、保存草稿或业务数据修复接口。
- 历史 GET `/app/api/costing/bean-list/publications/39/pdf?list_type=green&scope=official&product_type_category_id=8000000000000162` 返回 HTTP 200/application/pdf，28103 字节、有效 `%PDF` 文件头。发布状态、版本、配置/内容与 PDF 缓存键及文件哈希在部署及验证前后完全一致。
- PR-638、3 条 DEV-638 已可通过生产 PR/DEV 接口读取；实现两项 done，PR 和交付项 review，保留 Van 验收边界。

## 产物、回滚与待验收

- 生产小程序产物 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`：RELEASE_INFO 指向 `a9dd129ef41119d1cd7e82630440c62e4660df98`、production、`https://erp.qacoohee.com/app`；14 个页面及 PAGE_FILE_MANIFEST 56 个文件验证通过。本次只构建导出，未上传/发布微信版本。
- 原产物备份 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260908135125-a9dd129ef411`。
- 原生产源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20260908134451-a9dd129ef411`，回滚镜像 `kferp-orderapp-rollback:production-20260908134451-a9dd129ef411`。
- 日志：`pr638-production-deploy.log`、`production-after.txt`、`production-fingerprints.txt`、`production-snapshot-before.txt`，均归档本机证据目录；业务商品名在日志中脱敏。
- 待 Van：在价格表生成/发布界面核对错误提示；为这 7 个商品确认并发布可用的默认 BOM 规格，重新选规格及发布新版后验证录单。当前录单仍是 0 个生豆商品，不能将本次拦截修复表述为业务配置已修复。历史订单和价格没有回算。
