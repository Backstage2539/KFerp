# PR-663 商品价格表摘要加载与归档永久清理交付证据

## 交付范围

- `GET /api/costing/bean-list/publications?view=summary` 按版本批次返回分页摘要、总数、归档数量、当前版本和建议新版本。
- `GET /api/costing/bean-list/publications/:id` 按权限读取一张完整快照；`GET /api/costing/bean-list/publications/price-sources` 只读取客户初始化实际采用的来源。
- `POST /api/costing/bean-list/publications/delete-preview` 和 `/delete` 支持归档单张、勾选批量及当前归属、商品类型、用途范围清空。
- Vue 商品价格表使用服务端分页、300 毫秒搜索防抖、相同请求合并和过期响应隔离；归档展开后加载，复制价格时才读取完整详情。

## RED 与 GREEN

- RED：新增前端行为测试先因 `publication-summary-client.js` 不存在失败，证明原页面没有统一的请求合并和最新响应保护入口。
- GREEN：前端定向测试 59/59 通过，覆盖摘要 URL、相同请求合并、完成后不保留长期结果缓存、最新请求门禁、归档按需加载、按需详情及删除入口。
- GREEN：真实 PostgreSQL 定向测试覆盖摘要/完整字段一致、版本分组分页、搜索、客户隔离、历史补齐、来源选择、单张/清空删除、非归档拒绝、确认失效、幂等、审计、资源清除、事务回滚、版本号不重用及默认表回退。
- GREEN：成本 application、PostgreSQL repository 和 HTTP 接口包测试通过；Vue 生产构建通过。
- development 首次基础部署的历史摘要补齐暴露空快照布尔表达式可能为 `NULL`，容器启动失败后部署脚本自动恢复部署前源码和镜像并验证 HTTP。补充空历史快照回归后，缺少 `price_rows`/`groups` 的记录明确写入 `publication_has_content=false`。

## 数据安全与兼容性

- 每张 publication 是独立快照；客户初始带价、复制来源和旧来源字段不构成删除阻断。
- 删除事务按明确 publication ID 清空 `config`、`content` 和 PDF 资产，保留 ID、名称、归属、版本、批次、删除人和删除时间，并逐表写入操作日志。
- 客户确认记录、订单记录和其他 publication 不级联删除；删除重复提交返回第一次结果，过期或记录变化后的确认凭据拒绝执行。
- 原完整列表接口保留；已删除表从列表、报价来源和恢复入口排除，详情和 PDF 返回 410，版本分配仍考虑已删除历史。

## 隔离库性能

- 当前规模：53 个历史版本、每张约 100 KB 完整配置，摘要页约 4.22 KB；20 次服务端 P95 约 1.56 ms。
- 十倍历史规模：530 个历史版本、同等单张配置，摘要页约 4.22 KB；20 次服务端 P95 约 4.68 ms，约为当前规模的 2.99 倍。
- 两组均满足未压缩摘要不超过 50 KB；当前规模 P95 不超过 300 ms，十倍规模 P95 不超过 1 秒。该结果来自隔离 PostgreSQL，并不代替公网和生产应用复测。

## 待完成交付证据

- 完整仓库检查和最新 `origin/develop` 集成结果。
- development 兼容基础版本、最终页面版本、接口/页面验收和回滚版本。
- production 兼容基础版本、最终版本及三类摘要请求各 20 次的服务端、公网首字节和完整下载时间。
- 客户 450“曲奇”9 条规格、客户报价、公共报价、复制和 PDF 的只读一致性检查；正式归档不执行删除。
- Van 业务验收。
