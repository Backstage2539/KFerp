# PR-567-DEVELOPMENT-PUBLIC-DOMAIN 开发环境公网域名验收证据

## 范围

- `DEV-567-DEVELOPMENT-URL`：开发环境统一使用 `https://dev.qacoohee.com`，生产环境继续使用 `https://erp.qacoohee.com`。
- `DEV-567-PUBLIC-INGRESS`：唯一公网 Caddy 将两个域名分别转发到开发和生产应用容器，开发域名使用公开证书。
- `DEV-567-DEPLOYMENT-GUARD`：入口配置先校验、备份再热加载；严格 TLS 探针验证域名和证书。
- 不修改数据库和业务数据；不重启生产应用或生产数据库。

## RED

- 2026-08-01：`go test ./internal/interfaces/http/support -run TestDev567DevelopmentPublicDomainContract -count=1` 失败，报告缺少 `scripts/Caddyfile.public`。同时现有发布脚本仍包含旧开发域名，证明新域名合同尚未实现。

## GREEN 与发布

- [ ] 定向 Go 合同与 Shell 语法通过。
- [ ] 功能分支远程预检通过，未改变运行环境。
- [ ] 合入 `develop` 并完成 development 发布。
- [ ] `dev.qacoohee.com` 严格 TLS、HTTP 和开发容器路由通过。
- [ ] `erp.qacoohee.com` 严格 TLS、HTTP 和生产容器路由保持通过。
- [ ] 生产应用、生产数据库启动时间在入口更新前后保持不变。

## 回滚

- 入口脚本输出 `previous_caddy` 时间戳备份；需要回滚时复制该文件覆盖 `/opt/stacks/erp-production/Caddyfile`，再对 `erp_prod_caddy` 执行 Caddy 热加载。
- development 应用发布继续使用发布日志中的 `previous_source` 和 `rollback_image`；生产应用与数据库不在本次变更范围内。
