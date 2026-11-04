# PR-717-SHIPPING-RETURN 验证记录

日期：2026-10-23。分支：`codex/shipping-return-import-20261023`。产品状态：review，待 Van 业务验收。

## 实现

- 订单列表支持预置顺丰寄件回传与可保存的列映射模板；默认 C 单号、S 实际物流成本，0 元有效。
- 收件人可见姓名及手机号前 3 后 4 位匹配所有正常、已接受、可发货且无单号订单；索引手机号后检查姓名。
- 双向唯一自动回填；多候选持久化并由订单行弹窗选择。选择时检查当前订单版本、收件信息、发货资格和运单占用；自动确认在事务锁内复查唯一性。
- 单号、成本、运费差额、发货状态、库存和操作日志在同一订单事务提交。单订单失败回滚；重复文件和重复选择保持幂等。
- 普通订单与代发共用确认规则；保护已收运费、免运费、历史不明收款，使用独立差额费项保留原订单凭证。原实际运费导入也使用该确认路径。
- 订单列表和代发详情显示最终运费、已收、待收、实际成本；在订单上传区提供手册入口并保留返回来源。

## RED / GREEN

- 先添加匹配、解析和运费确认测试：实现前缺少领域方法，编译失败；实现后通过。
- 额外 RED：`TestParseRejectsUnsafeTrackingNumbers` 接收 `SF1,SF2` 为 pending；增加单票、科学计数法及 Excel 长数值精度校验后 GREEN。
- 历史收款 RED：订单状态为已付款、明细仅已收货款时曾错误推断运费也已收；限制仅无拆分明细的历史全额付款才推断原运费，未分配的预付款进入核对分支后 GREEN。
- 权限边界 RED：渠道客户即使被授予 orders.read 仍可进入候选 API；加入内部物流接口边界后 GREEN，覆盖模板、候选、上传和选择。
- 旧手填正数运费测试按本次确认规则修订：未收清时实际成本变化会产生有符号差额；手填零、全额实收仍受保护。

## 自动验证

- `scripts/verify_kferp.sh backend`：全部 Go 包通过（未配置数据库的集成测试按原设计跳过）。
- 独立本地测试数据库 `kferp_shipping_return_20261023`，测试为各自临时 schema，未接触业务数据库。
- PostgreSQL 定向集：`TestShippingReturn*`、`TestFreight*`、`TestOrderFreight*`、`TestFillShipmentTracking*` 通过。
- 覆盖：模板保存及过期版本拒绝、唯一匹配、多个候选、人工选择、重复文件/选择、跨新旧入口不重复收费、历史独立运费费项不被当成免运费、全额运费实收、部分实收、仅商品货款已付、手填零、费用低于实收、历史分配不明、金融校验失败回滚。
- API/权限：员工边界、客户范围拒绝、模板/候选读取、上传零元记录、选择版本、订单读写/设置写权限。
- Vue：1,512 项测试通过；生产构建通过。既有大包体积提示不影响构建。
- `scripts/verify_kferp.sh changed`：通过。

## 页面验证和人工审查

本地隔离页面使用虚构收寄信息检查模板编辑、复制入口与选择弹窗。已观察弹窗的七项业务字段及“使用此单”按钮；实际提交成功/冲突行为由 Vue 运行测试及 API/数据库测试验证。

截图：`2026-10-23-shipping-return-picker.png`（本地测试画面）。未将用户寄件文件上传至任何业务环境。

审查重点：候选与运单不得越权或重复占用；成本与应收分离；已收不被覆写；订单锁及结算费项/付款批次锁保护最终确认；刷新过期版本后可重新选择；前端异步加载不回写过期列表。

## 手册和交付

- `docs/OP_MANUAL_SHIPPING_RETURN.md`；订单手册及总索引已更新。
- PR-717-SHIPPING-RETURN：review；DEV-717-01/02/03：done。
- 开发环境已部署并完成只读检查，版本与回滚位置见下方。
- Van 已于 2026-10-23 授权合并 main 并部署正式环境；真实 Excel 导入及业务验收另行操作。

## 开发环境部署证据

- 功能分支：`codex/shipping-return-import-20261023`，功能及收款边界修正均已推送；本记录为部署后的证据补充。
- 已合并并推送的 `origin/develop` / 部署代码版本：`dccaa17a46964e6d148c6ad14b8c0d2aca3b2e0a`。
- 在独立干净 develop checkout 中执行：`KFERP_SKIP_MINIAPP_EXPORT=1 bash deploy_orderapp.sh development`。
- 部署时间：2026-10-23 北京时间；仅更新开发应用。服务器端 Vue 1,512 项测试、构建、Go 全包测试、镜像构建及内外部就绪检查通过。
- 开发入口：`https://dev.qacoohee.com/app/vue-shell/?view=orders`。
- 原源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20261023010512-dccaa17a4696`。
- 回滚镜像：`kferp-orderapp-rollback:development-20261023010512-dccaa17a4696`。
- 运行容器镜像：`sha256:ae849276f77c4cd699c383203f7f5cb2c219e88480a2f62f009a2fd2a21e9850`，与本次构建产物一致。
- 运行界面文件：`OrdersView-WsiRjY2i.js` 含 `shipping-return-selection`；同批构建的 RELEASE_INFO 标识上述代码版本。
- 无认证 `/app/` 返回 303 到 `/app/orders`（当前入口路由行为）；无认证回传模板 API 返回 401。
- 认证后的订单 Vue 页面、模板 API、手册、产品需求 API 均 200；PR-717 可见；预置模板确认为 C 单号 / S 金额。
- 原始检查摘要见同目录 `2026-10-23-shipping-return-runtime.json`。
- 清理的内容仅为可重新生成的 Go/Docker 构建缓存及无标签闲置镜像；数据库、业务文件、原源码及回滚镜像保留。部署后可用空间约 3.1GB。
- 未导入用户真实 Excel、未进行真实订单发货验收、未部署正式环境。PR 继续 review，待 Van 验收。

## 正式发布准备

- Van 授权合并 main 并部署生产 ERP；生产集成分支 `codex/shipping-return-production-20261023`。
- 在最新 `origin/main` 上合并已验证并推送的 `origin/develop`（含开发交付证据）。仅 ACTIVE_REQUIREMENTS 交付记录冲突，保留双方记录；生产独有 BOM 修复和原有需求状态完整保留。
- 合并后复检全部通过：后端全包、独立 PostgreSQL 的寄件回传/运费/发货回归、Vue 1,512 项测试与生产构建；部署脚本仍执行服务器端完整门禁。
- 本次仅发布 ERP 服务，真实寄件文件尚未导入，业务验收状态继续 review。
