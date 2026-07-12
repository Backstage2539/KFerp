# PR-535 删除过时成本参数设置验收

## 验收目标

- 商品价格管理只保留价格计算模板，不再展示成本参数设置。
- 商品价格表/成本核算不再提供参数设置按钮或快捷抽屉。
- 旧成本参数直达页面和前端专用组件删除；历史参数数据和旧成本记录不做破坏性修改。

## 自动验证

- 前端 RED/GREEN：`node --test src/lib/obsolete-cost-parameters.test.js src/lib/settings-entry-consolidation.test.js src/lib/product-settings.test.js`。
- 支持合同：`go test ./internal/interfaces/http/support -run 'TestDev535|TestDev531|TestDev271' -count=1`。
- 构建与变更验证：`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh frontend-build`、`git diff --check`。

## 浏览器验收

1. 打开开发或生产环境 `商品 / 商品价格管理`，确认页面只显示价格计算模板、价格试算和新建价格计算模板。
2. 确认页面不显示成本参数设置文字、Tab、参数列表、刷新或保存入口。
3. 打开商品价格表/成本核算，确认顶部没有 `参数设置`，页面中没有快速成本参数抽屉。
4. 打开旧 `view=costingSettings`，确认不再提供成本参数设置页面。

## 实现前后证据

- RED：删除契约 0/3 通过；三处入口、旧路由和专用前端文件均仍存在。
- GREEN：删除实现后目标前端组合测试 160/160 通过。
- 商品价格边界：与 PR-535 之前的 `cab7b5b2` 对比，商品价格管理净变化只有移除嵌入的成本参数组件、import 和样式；Pricing Rule 列表、试算、新建、编辑、复制、失效和保存逻辑均未修改。
- 支持合同：PR-535、历史 PR-531 和 DEV-271 源码合同测试通过。
- 构建：Vue/Vite production build 通过；完整前端 695/701，通过数变化来自删除 1 条过时 helper 测试，失败仍是原有六条 workspace-context 基线，无新增失败。
- 数据边界：没有执行数据库迁移或参数数据删除；历史成本记录和价格计算模板不改写。

## 开发环境部署证据

- 已合并并推送 `origin/develop=1d988ca8a653a0465ed8c81742aaa39e5e9752dd`，随后完成替换部署；备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260713000119`。
- Docker 构建门禁执行 `go test ./...` 并通过；部署后 `erp_orderapp` 运行、`erp_postgres` healthy。
- 认证访问商品价格管理页面和需求接口均返回 200；PR-535 标记存在，部署源码中的 Pricing Rule 列表、试算和表单标记存在，成本参数组件、快捷设置和旧路由标记均不存在，最近应用错误行数为 0。
- 开发域名浏览器访问受 `ERR_CERT_AUTHORITY_INVALID` 阻断，未绕过证书告警；本次以认证页面/API、部署源码和容器日志完成开发环境验收。

## 生产环境部署证据

- `develop` 已合并到 `main` 并推送 `origin/main=57f01fed48e74e3c88df45a8aa6a30c17061c5dd`，随后执行 `./deploy_orderapp.sh production`；备份为 `/opt/stacks/erp-production/orderapp.backup.deploy-20260713002103`。
- Vue 与小程序构建通过，生产 Docker 构建门禁执行 `go test ./...` 并通过；`erp_prod_orderapp` 运行、`erp_prod_postgres` healthy，生产 Caddy 正常占用公网 443。
- 公网未认证访问返回 401，认证商品价格管理页面返回 200；部署源码保留 Pricing Rule 列表、试算和表单标记，不含成本参数组件、快捷设置或旧路由标记，最近应用错误行数为 0。
- 生产浏览器确认商品价格管理显示价格计算模板、价格试算和新建价格计算模板且不显示成本参数；商品价格表没有 `参数设置` 按钮或快速成本参数区域；console errors 0。
