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
- development 最终版首次只读烟测又发现未选择商品分类时 PostgreSQL 无法推断保留参数 `$4` 的类型。新增真实 PostgreSQL 回归先稳定复现 `SQLSTATE 42P18`，再将无分类分支显式约束为 bigint 零值；修复后无分类公共、个人及带分类公共、客户摘要均返回 200。

## 数据安全与兼容性

- 每张 publication 是独立快照；客户初始带价、复制来源和旧来源字段不构成删除阻断。
- 删除事务按明确 publication ID 清空 `config`、`content` 和 PDF 资产，保留 ID、名称、归属、版本、批次、删除人和删除时间，并逐表写入操作日志。
- 客户确认记录、订单记录和其他 publication 不级联删除；删除重复提交返回第一次结果，过期或记录变化后的确认凭据拒绝执行。
- 原完整列表接口保留；已删除表从列表、报价来源和恢复入口排除，详情和 PDF 返回 410，版本分配仍考虑已删除历史。

## 隔离库性能

- 当前规模：53 个历史版本、每张约 100 KB 完整配置，摘要页约 4.22 KB；20 次服务端 P95 约 1.56 ms。
- 十倍历史规模：530 个历史版本、同等单张配置，摘要页约 4.22 KB；20 次服务端 P95 约 4.68 ms，约为当前规模的 2.99 倍。
- 两组均满足未压缩摘要不超过 50 KB；当前规模 P95 不超过 300 ms，十倍规模 P95 不超过 1 秒。该结果来自隔离 PostgreSQL，并不代替公网和生产应用复测。

## development 交付

- 最终应用提交：`e8fcb7828c88c65a80d3f90862245714f4ac6c7b`；源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260914160642-e8fcb7828c88`；回滚镜像 `kferp-orderapp-rollback:development-20260914160642-e8fcb7828c88`。
- 发布门禁：Vue 1240/1240、微信端 246/246、完整 Go 测试、Vite 与 production image 构建通过；`erp_orderapp` running、restart count 0，登录 HTTP 200，发布后错误日志命中 0。
- 四类摘要只读烟测均为 HTTP 200：无分类公共 73 ms/7.3 KB、无分类个人 79 ms/0.9 KB、工厂量单公共 41 ms/95 B、客户 450 工厂量单 63 ms/88 B。摘要字段未出现 `config`、`content`、`config_json` 或 `content_json`。
- 实际 Chrome 页面展开归档区后可见“删除选中”“清空当前归档”及两条单表“删除”；未执行删除。当前开发数据没有客户 450 对应工厂量单来源，客户 450“曲奇”规格改在生产只读验证。

## production 兼容基础版本

- 兼容基础提交：`2580b99eb96580bb30821bcd16b782fb600c83a2`；源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260914161920-2580b99eb965`；回滚镜像 `kferp-orderapp-rollback:production-20260914161920-2580b99eb965`。
- 基础版保留旧页面，仅增加摘要、按需详情、报价来源、删除标识和归档删除后端能力。旧页面 1236/1236、微信端 246/246、完整 Go 测试和镜像构建通过。
- 发布后四类摘要 HTTP 200，公网完整耗时 34–61 ms、响应 88 B–7.3 KB，均未携带完整快照；`erp_prod_orderapp` running、restart count 0，错误日志命中 0，回滚镜像存在。

## production 最终交付

- 最终应用提交：`bc960916a1366eb6ab6819fc2221b2c6d882e4c6`；源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260914163145-bc960916a136`；兼容回滚镜像 `kferp-orderapp-rollback:production-20260914163145-bc960916a136`。该回滚镜像对应已能识别删除标识的基础运行版本。
- 发布门禁：Vue 1240/1240、微信端 246/246、完整 Go 测试、Vite 与 production image 构建通过。`erp_prod_orderapp` running、restart count 0，发布后错误日志命中 0；前端资源 `/vue-shell/assets/index-BNT1yD7i.js` 包含“删除选中”和“清空当前归档”。未上传或发布微信包。
- 生产页面只读验收：公共烘焙咖啡豆当前发布 V3.0.39，共 7 个版本、28 条归档；展开归档后按 10 条分页，显示“删除选中（0）”“清空当前归档”和逐表“删除”。未勾选、预览或删除任何生产记录。

## production 20 次并发只读性能

每轮同时请求无分类公共、工厂量单公共、工厂量单个人和客户 450 工厂量单摘要，共运行 20 轮：

| 请求 | 应用内 P95 | 公网首字节 P95 | 公网完整 P95 | 响应大小 |
|---|---:|---:|---:|---:|
| 无分类公共 | 13.6 ms | 71.6 ms | 73.1 ms | 7,298 B |
| 工厂量单公共 | 9.9 ms | 64.5 ms | 67.3 ms | 5,413 B |
| 工厂量单个人 | 22.6 ms | 68.7 ms | 68.8 ms | 88 B |
| 客户 450 工厂量单 | 18.1 ms | 69.0 ms | 69.1 ms | 3,317 B |

全部 80 个请求为 HTTP 200，摘要均未返回 `config` 或 `content`，满足单请求服务端 P95 不超过 1 秒及未压缩摘要不超过 50 KB 的目标。

## 客户 450 与按需内容只读复测

- 完整 `/api/costing/bean-list?customer_id=450` 公网首字节约 248 ms、完整约 252 ms，应用内约 113 ms；顶层“曲奇”保持 9 条规格，BOM 规格 ID 为 71、72、73、74、75、76、77、219、222，商品、成本、阶梯价、客户覆盖值和警示结构均正常返回。
- 客户初始化报价来源只返回实际采用的两张完整快照：客户表“四川 9.9coffee lab 供应价格表”V3.0.7（3 条价格行）和公共表“棵凡咖啡烘焙咖啡豆商品价格表”V3.0.39（65 条价格行）。两张来源的 `config`、`content` 与按 ID 获取的详情逐字段相等，证明复制价格读取的是所选快照。
- 客户表 ID 72 和公共表 ID 65 的摘要与详情在 ID、发布批次、表名、默认表、版本、状态和归属上全部一致；摘要分别为 3,317 B 和 5,413 B，完整详情按需加载后分别为 21,127 B 和 397,762 B。
- 只读取已有 PDF，未调用生成接口：客户表 PDF 22,155 B，公共表 PDF 44,970 B，均为 HTTP 200、`application/pdf`。

## 待业务验收

- 技术、接口、性能、生产只读页面与回滚证据已完成；由 Van 在 production 完成实际业务验收。

## 交付状态同步

- 验收证据与需求状态同步到 development 提交 `5c48f4e872fdde916a477a786939496d1e1f1533`；源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260914165015-5c48f4e872fd`，回滚镜像 `kferp-orderapp-rollback:development-20260914165015-5c48f4e872fd`。
- 验收证据与需求状态同步到 production 提交 `55413e93423721e4212a6ed35ca1cab151043bbf`；源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260914170318-55413e934237`，回滚镜像 `kferp-orderapp-rollback:production-20260914170318-55413e934237`。兼容基础回滚镜像 `kferp-orderapp-rollback:production-20260914163145-bc960916a136` 同时保留。
- 同步后生产客户 450 摘要 HTTP 200、73.7 ms、3,317 B；容器 running、restart count 0、错误日志命中 0。需求接口显示 PR-663 为 `review`、负责人 VA，DEV-679 为 `done`。
