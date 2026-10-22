# PR-716 运费配置优先级

- DEV-843：独立系统默认配置表、读写API、乐观版本与操作日志；报价统一按场景、客户普通、系统默认回退。
- DEV-844：物流设置取消客户选择；客户档案编辑抽屉提供普通运费；代发场景可继承；手册同步。
- RED：TestFreightTemplatePriorityAPI 返回系统默认 GET 404；Vue 默认编辑器无客户不能保存；代发继承校验失败。
- GREEN：隔离 PostgreSQL API/报价测试以及 Vue 编译后运行测试。完整门禁结果见交付记录。
- 普通录单、代发估算及提交共用解析方法；编辑器读原始配置，不把继承值复制进客户/代发配置。
- 明确包邮按所选层级生效；继承时附加运费须为0；已选无效模板或缺费率不回退。历史订单继续引用原政策与模板快照。
- 手册：`docs/OP_MANUAL_FREIGHT.md`，通过既有 Vue 运费手册入口展示。
- 开发环境已部署并只读核对通过（见下方交付结果）；生产未发布，业务验收待 Van。

## 验证记录
- `go test ./...` 全包通过（无数据库的全包门禁，数据库专项另行运行）。
- `ORDERAPP_TEST_DATABASE_URL=... go test ./internal/infrastructure/postgres/customerfulfillment -run 'Test(Freight|OrderFreight|Fulfillment.*Settings|Fulfillment.*Fee|LegacyEmployee)' -count=1` 通过，41.116s；独立测试 schema，结束后清理。
- `TestFreightTemplatePriorityAPI` 另含真实代发设置保存、继承报价及服务费保留，2.196s 通过。
- Vue 1508 项全部通过；生产构建通过。18 项运费/履约组件与纯函数回归通过。
- 本地浏览器核对两类编辑器：全局配置无客户选择器，客户配置可继承，字段和保存按钮完整可见；使用合成数据，不涉及真实业务验收。

## 开发环境交付结果（2026-10-22）
- 实现分支：`codex/freight-template-priority-20261022`，已推送 `1da77322`。
- 已合并并推送 develop：`e3d588b856609e9e6d7cea8252fa78b318fccd5d`；合并树与验证分支完全一致。
- 执行：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`，服务器 6/6 成功；前端1508项测试、小程序测试/类型检查/24页面产物、全部Go测试和镜像内复检通过。
- 旧源码：`/opt/stacks/erp/orderapp.backup.deploy-20261022223720-e3d588b85660`。回滚镜像：`kferp-orderapp-rollback:development-20261022223720-e3d588b85660`。
- 运行镜像：`sha256:c23405551d742cfb145fcec3d533d3f3c91641f4a3d7a8f250e1b3257a736287`；容器 running，22:46:27 +08 重启。
- 只读核对：物流页面200；默认配置GET200（revision=0，尚未配置）；PR-716接口200且记录存在；运费手册raw=1返回200且含三层优先级；未登录入口303跳登录，配置API401。
- 未创建真实业务订单、未更改客户/运费业务配置；未发布生产或上传微信。系统默认模板需由业务人员选择并保存。业务验收待Van。
- 发布前后只清理可再生成构建缓存，保留数据库、业务文件及回滚版本；结束可用空间2.1GB。

## 已授权生产发布
- Van 授权合并 main 并部署 production。
- 合并保留 main 的生产专用 BOM 修复及既有发布记录，仅新增 PR-716 业务改动。
- 小程序未改客户端源码，本次不导出或上传微信包。
- 生产合并验证：所有 Go 包测试通过；隔离 PostgreSQL 运费/履约/历史运费专项通过（51.965s）；Vue 1508 项通过，构建通过；main 原有生产 BOM 代码与测试均未改动。
