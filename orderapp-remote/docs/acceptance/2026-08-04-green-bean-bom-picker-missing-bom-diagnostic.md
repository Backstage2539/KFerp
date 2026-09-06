# PR-578 生豆生产 BOM 候选与无 BOM 试算提示验收记录

## 当前状态

- 应用与 Vue 定向 RED/GREEN、完整 Go/Vue/Vite 回归和独立复核已完成；已合入 develop/main 并部署 development/production，Van 浏览器业务验收仍待完成。
- 本需求及部署不自动新增、发布或绑定任何真实 BOM。

## 现场只读复现

- development 商品档案存在启用的“萨琪姆 生豆”父商品和“萨琪姆 生豆 Kg”具体规格，两者当前都没有 production BOM。
- 原 `/api/bom/products` 服务层和 Vue `isBomProductCandidate` 都排除 `product_kind=green_bean`，所以新建生产 BOM 时输入“生豆”显示“没有可用商品档案”。仓储本来已经读取全部启用商品，无需修改 SQL 或数据库。
- 无 BOM 商品使用价格计算模板试算时，原实现返回 HTTP 200、`final_unit_price=0` 和“该商品暂无可试算的标准制造成本”警告，没有指出需要新增或发布 BOM。

## 修复合同

- `/api/bom/products` 返回启用生豆；Vue “产出商品”使用独立的启用状态过滤。BOM 内“商品组件”继续沿用原非生豆候选，客户范围和历史列表逻辑不扩大。
- 已读取生产 BOM 选项但没有任何 published 版本、没有正数临时基础成本且没有可独立计价的正数工序成本时，单次试算返回 4xx，批量返回行级错误：`该商品未配置可用于试算的已发布生产 BOM，无法计算标准制造成本；请到 生产管理 → 生产 BOM 新增或发布 BOM 后再试算`。旧 `BomStatus/BomVersionID` 残留和已失效版本 ID 不能绕过该判断；未被当前商品已发布 BOM 选项验证的版本会在加载成本前清除，仓储查询也再次校验版本属于当前商品或父商品的启用 production BOM 且状态为 published。不支持新版选项仓储的历史调用仍保留兼容判定。
- 正数临时基础成本和正数工序成本继续允许；负数参数校验、PR-577 空 published/非空 draft 专用提示和不可追溯旧成本禁用规则保持不变。
- 选择生豆作为产出商品只创建现有流程的 BOM 草稿；真实保存/发布继续执行权限、校验和操作日志。本需求没有新增业务写入口或日志类型。

## TDD 证据

- RED：BOM service/API 均删除生豆，Vue 缺少生豆产出候选 helper；无 BOM 单次/API/批量分别返回 0 元成功、HTTP 200 和成功结果行。
- Focused GREEN：BOM service/API、costing service/API 单次/批量以及 Vue BOM helper/source tests 全部通过。
- Full GREEN：`go test ./... -count=1`、Vue 871 项完整测试、Vue BOM 18 项定向测试、Vite build、`scripts/verify_kferp.sh changed` 和 `git diff --check` 通过；两轮独立复核最终无阻断项。

## 待完成验收

- [x] 完整 Go、Vue 测试与构建、支持合同及格式检查通过。
- [ ] development `/api/bom/products` 只读返回“萨琪姆 生豆”及 Kg 规格；浏览器新建 BOM 输入“生豆”可见且不保存。
- [ ] development 无 BOM 试算显示明确新增/发布提示，不显示可用 0 元结果；不保存价格或 BOM。
- [x] 功能分支推送，合入最新 develop/main；development/production 发布后容器、入口和日志正常。

## 部署边界

- development：已随 `origin/develop@49489cbd3a7c205dbb033d4690d1d9672faf149c` 部署。
- production：release merge `a06aa95ebe38d7b91806cd234032c0cc3bb62a7e` 已部署；发布只切换应用，没有新增、发布或绑定 BOM。无 BOM 提示和生豆产出商品搜索仍需 Van 使用真实登录态手工验收。
