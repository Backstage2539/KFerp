# PR-567-DEVELOPMENT-PUBLIC-DOMAIN 开发环境公网域名验收证据

## 范围

- `DEV-567-DEVELOPMENT-URL`：开发环境统一使用 `https://dev.qacoohee.com`，生产环境继续使用 `https://erp.qacoohee.com`。
- `DEV-567-PUBLIC-INGRESS`：唯一公网 Caddy 将两个域名分别转发到开发和生产应用容器，开发域名使用公开证书。
- `DEV-567-DEPLOYMENT-GUARD`：入口配置先校验、备份再热加载；严格 TLS 探针验证域名和证书。
- 不修改数据库和业务数据；不重启生产应用或生产数据库。

## RED

- 2026-08-01：`go test ./internal/interfaces/http/support -run TestDev567DevelopmentPublicDomainContract -count=1` 失败，报告缺少 `scripts/Caddyfile.public`。同时现有发布脚本仍包含旧开发域名，证明新域名合同尚未实现。

## GREEN 与发布

- [x] 定向 Go 合同、支持包完整测试、Shell 语法、`scripts/verify_kferp.sh changed` 和 Caddy `validate` 通过。
- [x] 功能分支 `225884eb0eddc6c40a3d040063629b9fd472527c` 远程预检通过：Vue 852 项、小程序 90 项、Go 全量和隔离 Docker 镜像均为 GREEN；没有提升源码、修改持久配置或重启容器。
- [x] 合入并推送 `origin/develop=cb46d61dcb2f3bf0229a23586351e147bc7b54a4`，随后完成 development 发布。开发源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260801120816-cb46d61dcb2f`，回滚镜像为 `kferp-orderapp-rollback:development-20260801120816-cb46d61dcb2f`。
- [x] `dev.qacoohee.com` 严格 TLS 返回 HTTP 200；证书主题和 SAN 均为 `dev.qacoohee.com`，签发者为 Let's Encrypt。Chrome 直接进入 `/app/login`，标题和页面正文为“系统登录”，没有证书拦截。
- [x] `erp.qacoohee.com` 严格 TLS 返回 HTTP 200，原 Let's Encrypt 证书和域名保持有效。带各自认证读取 `REQUIREMENTS.md` 时，开发域名包含一个 PR-567 标记，生产域名为零，证明 Host 分别命中开发与生产容器。
- [x] 开发小程序 `RELEASE_INFO` 为 `environment=development`、`api_base=https://dev.qacoohee.com/app`、提交 `cb46d61d...`；构建产物包含新域名的文件数为 2，旧 `dev.erp.qacoohee.com` 为 0。
- [x] 发布排队时另一独立 production 发布先持有互斥锁，本需求未绕过。该发布结束后重新记录基线：`erp_prod_orderapp=2026-08-01T04:07:23.209830413Z`、`erp_prod_postgres=2026-05-03T07:39:18.920132081Z`、`erp_prod_caddy=2026-07-10T03:42:22.175653457Z`；PR-567 网关热加载和 development 发布后上述三个时间均未改变。

## 回滚

- 入口备份为 `/opt/stacks/erp-production/Caddyfile.backup.domain-20260801121058`；需要回滚时复制该文件覆盖 `/opt/stacks/erp-production/Caddyfile`，再对 `erp_prod_caddy` 执行 Caddy 热加载。
- development 应用发布继续使用发布日志中的 `previous_source` 和 `rollback_image`；生产应用与数据库不在本次变更范围内。
