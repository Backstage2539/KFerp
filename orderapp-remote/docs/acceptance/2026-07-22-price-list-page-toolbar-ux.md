# PR-547 商品价格表标题与已发布版本工具栏精简

## 范围

- 删除商品价格表顶部“模型 / Price List / Item Price”指标。
- 将“Price List / Item Price 生成规则”改为“计价规则”。
- 精简已发布价格表标题栏与过滤区，不修改价格表 API、发布快照或业务数据。

## TDD 证据

### RED

- `node --test src/lib/costing-bean-list-version-ui.test.js`：新增紧凑工具栏合同后，因顶部仍存在“模型”而失败。
- `go test ./internal/interfaces/http/support -run '^TestDev547PriceListPageToolbarUXContracts$' -count=1`：因缺少 PR-547 需求种子而失败。

### GREEN

- `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/product-bean-list-split.test.js`：59/59 通过。
- `go test ./internal/interfaces/http/support -count=1`：通过。
- `scripts/verify_kferp.sh backend`：第一次发现旧源码合同仍要求旧标题；同步改为“计价规则”后完整 Go 测试通过。
- `scripts/verify_kferp.sh frontend-build`：Vue/Vite production build 通过，共转换 401 个模块；仅保留既有 chunk-size 提示。
- `scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。

## 验收口径

- 顶部不显示“模型”；计价区域标题为“计价规则”。
- 已发布价格表不显示旧说明和“刷新版本”。
- 上下双箭头按钮位于标题左侧；商品类型位于搜索左侧，两个控件均为 38px 高。
- 归档列表、下载 PDF、生成新版、撤回、归档和历史快照行为不变。

## 交付边界

- 本需求没有新增业务写操作，不新增操作日志。
- 功能分支提交 `44716a88`，合并后的 `develop` 为 `ac61d4d4af5667a5316a6da00187529bf628115c`，已通过 `./deploy_orderapp.sh development` 部署；备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260722233552`。
- 开发栈 `erp_orderapp` 正常运行、`erp_postgres` healthy；公开入口未认证返回 401，BasicAuth shell 返回 200；开发数据库存在 PR-547，服务器源码与 dist 均包含 `publication-list-collapse-toggle` / “展开已发布价格表”标记。
- 浏览器检查到达系统登录页；没有使用或索取业务账号绕过登录，最终页面布局与收起/展开交互等待 Van 在开发环境手工确认。
- production 未部署，也没有发布、撤回或归档价格表。
