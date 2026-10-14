# PR-691 小程序价格表全文搜索、行业筛选与标签

## 实现及权限

三个小程序入口使用 PriceListContent，共用当前表全文及行业/标签筛选。搜索只消费客户接口的已授权发布快照；结构化 product_attributes 和 attribute_lines 从同一 content_json 读取，不读取实时商品档案或修改旧快照。标签紧跟名称，行业字段沿用冻结模板身份及顺序；历史文字行不推断结构化筛选字段。接口为兼容字段新增，无数据库迁移。

缓存格式升级到 schema_version=2；进入客户价格表先重新校验服务接口，撤回、空结果和权限失败时不保留旧缓存；账号切换或离页使迟到的响应失效。查询条件仅保存在小程序运行内存，按环境、身份、客户和入口隔离。

## RED / GREEN

- Go RED：/tmp/kferp-price-search-go-red.log；发布快照行业字段及历史文本缺失。GREEN：/tmp/kferp-price-search-go-green.log。
- 小程序 RED：/tmp/kferp-price-search-mini-red.log，8 条失败；GREEN：/tmp/kferp-price-search-mini-green.log，11 条搜索/缓存测试通过。
- HTTP/PostgreSQL RED：/tmp/kferp-price-search-api-red.log，直接页面返回丢失冻结浅烘字段；GREEN：/tmp/kferp-price-search-api-green.log。测试使用独立 test_page_access_* schema，结束自动清理；开发/生产业务记录未改写。
- 295 项小程序测试及类型检查、开发构建通过；Vue 1348 项测试及生产构建通过。全量 Go 发现两项旧源结构测试仍查找 service.vue 内联渲染；更新为共享组件接入校验后，受影响后端包通过，完整 Go 门禁最终通过（/tmp/kferp-price-search-go-final.log）。

## 界面与手册

375px 本地同组件测试数据：部分英文和规格 AND 查询结果 1/4；筛选面板可多选浅烘/中烘，再加奶咖和推荐，结果为芒霜 1/4；标签紧跟名称，字段与长文字可换行；两列卡片 DOM 宽度检查 4/4 无水平溢出，表格布局检查完成。截图位于 /Users/yiiiple-work/KFerp-price-search-delivery-20261014。该证据是本地组件 UI 验证，不能替代微信真机业务验收。

手册：OP_MANUAL_BEAN_LIST_CENTER.md、OP_MANUAL_COSTING.md、OP_MANUAL_PAGE_ENTRIES.md，Vue 使用现有对应帮助入口。

## 交付边界

功能分支 codex/miniapp-price-search-20261014，经 PR 合入 develop 后部署 development，再经 release PR 合入 main 并部署 production。正式包需使用 wx6512f8684f584828 和生产 API，在 DevTools 导入、编译。部署提交、备份和最终结果另记交付收据及需求 UI。

Van 真机验收、微信代码上传/审核/发布待完成，PR/REV 不标记 done。公众号菜单不因本功能改动。
