# PR-690 公众号菜单请求编码修复

## 故障与范围
生产发布记录 11 返回微信 40033。订单路径 `pages/service/service?key=orders&source=official` 在双重 JSON 编码后含 `\u0026`；微信拒绝该转义。服务号凭证和白名单无需变更。

保留用户选择的「豆单获取」「我的订单」两个按钮、目标小程序和现有预览令牌校验。没有新增按钮、数据库迁移或小程序改动。

## 自动化证据
- RED：`TestPublishMenuPreservesUTF8AndURLSeparators` 的原始、存储转义两种输入均被 HTTP mock 拒绝；错误提示测试发现错误的白名单指引。日志 `/tmp/kferp-wechat-menu-red.log`。
- GREEN：`go test ./internal/interfaces/http/officialaccount ./internal/application/officialaccount -count=1` 通过。日志 `/tmp/kferp-wechat-menu-green.log`。
- 全量后端：`go test ./...` 通过，日志 `/tmp/kferp-wechat-menu-go-all.log`。
- HTTP 回归比较真实请求体的解码语义和原始中文/参数分隔符，验证历史草稿无需重建即可发布。
- 独立手册 `OP_MANUAL_WECHAT_OFFICIAL.md` 已更新，Vue 共用手册入口保持有效。

## 交付及验收边界
功能分支 `codex/wechat-menu-encoding-20261013`，经 PR 合入 develop 后，再经发布 PR 合入 main。部署脚本执行 Go、Vue、小程序门禁后更新生产后端。具体提交和运行结果记录于生产交付记录及需求 API。

生产部署、真实微信发布/读回仍待执行。Van 手机菜单显示与跳转验收独立待办。无需重新上传小程序。
