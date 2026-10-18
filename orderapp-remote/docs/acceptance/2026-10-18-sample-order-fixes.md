# PR-698 样品订单收件、顶部状态操作与零元结清

## 需求与实现

- 根因：普通编辑未回填 receiver 字段而提交空值，并无条件发送 direct_ship。现已回填四项收件快照，切单清旧值，保存不推断订单来源；样品读取本单快照，不能用客户档案掩盖缺失资料。
- 已有订单通过 Vue Teleport 将同一个表单的收件、付款、物流、凭证及保存动作放到详情顶部。新建录单维持原布局，移除编辑状态下重复的快捷单号控件。
- 服务端只信任真实样品事务/关联；商品、运费、各其他费用和最终金额全零时 settlement_reason=free_sample、兼容已付款状态、无需方式/凭证、实际已收和待收为零。旧的独立头部编辑接口不能破坏免费结清；完整表单仍用原 /api/order。
- 免费转收费同一事务恢复未收款；已有真实收款不可清除。仅运费收费允许货款 0 并保留凭证要求。API、ERP、小程序和销售 PDF/图片显示无需收款，不展示收款二维码。
- 样品订单编辑沿用生产/库存/发货限制；缺收件字段由保存及发货服务阻断，数据库发货触发器覆盖旧接口。
- 开发专用修复脚本 scripts/repair_sample_order_receipt.sql：行锁核对未生产、未发货、未失效及员工确认快照，只补空值，审计前后值，重复执行幂等，更新后使旧导出缓存失效。非空资料、订单来源、金额、负责人和样品回访保持不变。

## RED → GREEN 与自动化证据

先运行失败测试：Vue 0 被序列化为空、缺失回填函数；Go 免费资格函数缺失；真实样品确认 API 未设置免费结清；PDF 与小程序免费字段缺失。日志 /tmp/kferp-samplefix-{red-vue,red-go,red-api,red-pdf,mini-red}.log。

在 origin/develop@94dca9cf 合入后：

| 检查 | 结果 | 证据 |
|---|---|---|
| 全量 Go | 通过 | /tmp/kferp-samplefix-final-go.log |
| Vue 全量测试与构建 | 1,370 项通过，构建成功 | /tmp/kferp-samplefix-final-vue.log |
| 小程序全量测试、类型检查、开发构建 | 330 项通过，类型/构建成功 | /tmp/kferp-samplefix-final-mini.log、final-mini-build.log |
| 隔离 PostgreSQL + race | 样品生命周期、并发、权限、价格表录单兼容及本次修复通过 | /tmp/kferp-samplefix-final-db.log |
| 本地页面 | 真实 OrdersView 打开测试订单；顶部收件回填、无需收款、物流和保存位置无重叠 | 本地合成数据页面观察，不修改业务数据 |
| 销售资料 | 免费 PDF/PNG 生成并查看，无重叠、无需收款 | /tmp/kferp-samplefix-artifacts/free-sample.pdf、free-sample.png |

重点测试：sample_settlement_integration_test.go 覆盖确认下单→ERP 读取→只改备注保存→地址/来源保留、旧修订拒绝、收费转换、零货款运费凭证、伪造免收标记、真实收款保护、缺地址发货阻断、补地址后库存发货；修复脚本模拟审计失败回滚及重复执行仅一条审计。order-entry-recipient-runtime.test.js 执行真实 Vue 编辑组件回填与切单，其他单元测试覆盖零/空/负数及普通订单不可免收。

## 手册及五表

更新 OP_MANUAL_ORDER_SALES.md、OP_MANUAL_SAMPLES.md、OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md 与 Vue 录单帮助；既有 Vue 手册入口读取同一 Markdown 源。REQUIREMENTS / ACCEPTANCE_TESTS 同步本次口径。

PR-698-SAMPLE-ORDER-FIXES、DEV-787-SAMPLE-ORDER-FIXES、UT-698-SAMPLE-ORDER-FIXES、API-698-SAMPLE-ORDER-FIXES、REV-698-SAMPLE-ORDER-FIXES。并行工作占用 PR-697，本次独立调整为 PR-698；旧的本次五表条目按完整 code 精确重命名，不修改其他需求。

## 开发交付

功能分支 codex/sample-order-fixes-20261018；独立集成目录用于避免触碰共享 develop 中其他未提交工作。推送、集成、开发部署和 SO-20261016-0001 修复执行记录待回填。

真人发货验收、生产发布、小程序微信上传/审核/发布未执行。自动化发货仅使用隔离数据库的合成订单。
