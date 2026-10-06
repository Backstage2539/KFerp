# PR-683 新登录环境入口与合同文件认证下载

日期：2026-10-06。修复覆盖后端 BasicAuth 中间件、合同盖章下载按钮和订单销售操作手册。没有改动认证密码配置，也没有访问或部署正式环境。

## 验收行为

- `/login` 与 `/app/login` 都明确作为公开系统登录入口；其它受保护业务页面仍可触发原有外层 BasicAuth。
- `/contracts/...` 文件路由继续受服务器身份与权限校验保护。无有效系统令牌时返回 JSON 401，不附加 `WWW-Authenticate`，因此文件请求不会再触发浏览器 Basic 密码框。
- 合同盖章页通过共享 `downloadCustomerFile()` 发起下载，由 `apiFetch()` 自动附带 ERP Bearer 令牌。失效令牌提示打开 `/app/login` 重新登录；权限不足提示当前账号无权下载。
- 合同文件未加入匿名访问白名单。

## RED / GREEN

- RED：新增的 Go 中间件测试在原代码上复现 `/app/login` 返回带 Basic challenge 的 401，以及合同文件 401 带 `WWW-Authenticate`。
- RED：前端下载错误测试在实现前因缺少 401/403 分类提示而失败。
- GREEN：`go test ./...` 通过；Vue shell 全量 Node 测试通过（144 个测试文件）；`npm run build` 通过；`scripts/verify_kferp.sh changed` 通过。
- 定向检查：`go test ./internal/interfaces/http/support ./internal/interfaces/http/contracts -count=1` 通过；`node --test src/api/customer-account.test.js` 通过。

## 未执行项

- 没有生产配置、容器日志或真实浏览器请求样本，因此无法确认生产环境 BasicAuth 密码循环是否还存在环境变量或网关配置问题。
- 没有部署生产环境。本次代码让 `/app/login` 可直接访问，并修复合同下载认证链路；若用户输入正确 BasicAuth 密码仍在其它受保护页面循环，需要再核对生产 `APP_USER` / `APP_PASS` 与代理实际落点。
