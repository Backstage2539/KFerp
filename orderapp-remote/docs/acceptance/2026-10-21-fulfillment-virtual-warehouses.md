# PR-710 客户代发配置与虚拟仓库管理

- 状态：实现、本地回归与独立复审通过；开发环境交付进行中，Van 业务验收待确认。
- 开发分支：`codex/fulfillment-virtual-warehouses-20261021`，基线 `5b838315`。
- DEV-819：代发场景运费、仓库、服务费与审计整体保存，兼容旧客户端。
- DEV-820：虚拟子仓归类、分页、增改删、客户隔离及业务引用保护。
- DEV-821：开启后显示场景配置、仓库右抽屉、普通发货政策搬到业务物流设置。
- DEV-822：回归、手册、审核与开发环境交付。

## 验证边界

使用本机隔离 PostgreSQL 数据库与测试独立 schema；不修改真实客户配置、余额或订单。虚拟父级只展示分类，子仓沿用现有库存账。开发部署完成与 Van 业务验收分别记录。

## RED / GREEN 证据

- 原子保存：应用/HTTP 测试先失败，再验证嵌套政策、无 sale 依赖、政策或仓库错误整体回滚、审计故障整体回滚、关闭保留与旧格式兼容；单独修改 sale 不改变代发版本。证据 `/tmp/kferp-pr710-atomic-{unit-red,api-red,sale-red,target-green,final}.log`。
- 客户隔离：实际 PostgreSQL 先复现未绑定虚拟仓被当公共代发/加工来源，再覆盖材料、半成品、预览及提交锁定修复。证据 `/tmp/kferp-pr710-atomic-{scope-red,processing-scope-red,processing-scope-green}.log`。
- 虚拟仓：新路由先返回404；数据库测试覆盖历史仓归类及余额不变、CRUD/分页、版本冲突、审计回滚、删除引用保护、已完成业务可停用、历史不可改绑、引用写入共享锁及旧货主写入拒绝。证据 `/tmp/kferp-pr710-warehouse-{api-red,pg-red,green,final}.log`。
- Go 常规全包通过：`env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`，证据 `/tmp/kferp-pr710-backend-full-final.log`。启用数据库的相关回归单独执行，使用本机隔离测试库；两次完整空库启动与重复建表通过，证据 `/tmp/kferp-pr710-schema-final.log`。
- Vue 全量1472项通过、构建通过，证据 `/tmp/kferp-pr710-vue-final.log`、`/tmp/kferp-pr710-build-final.log`；行为测试 RED 证据 `/tmp/kferp-pr710-ui-{red,runtime-red,candidates-red}.log`。
- 本地浏览器模拟数据验证通过：关闭场景隐藏字段、开启显示同区域配置、包邮/零服务费、统一保存、新仓自动选中、费用草稿保留、未开门户普通客户政策维护。桌面和390px iframe视口的右抽屉已截图检查。截图保存在 `/Users/yiiiple-work/Documents/Codex/2026-10-21/fulfillment-virtual-warehouses/`，不代表真实客户业务验收。
- 独立代码复审通过：按真实 schema 核对引用列及完成状态，修复工单 released、订单状态ID、已撤回计划、已完成加工和货主仅变更更新的边界；无剩余阻塞项。

### 原有数据库夹具限制

尝试完整数据库包时，原有夹具存在缺少 `product_bom_spec_authorities`、物流重量、`work_orders.target_warehouse` 等错误，部分旧成品转仓夹具缺BOM权威配置。已从干净基线独立源码复现，未修改无关业务以绕过失败。相关日志为 `/tmp/kferp-pr710-atomic-{baseline,processing-baseline}.log` 与 `/tmp/kferp-pr710-warehouse-baseline.log`；本次相关数据库、接口和迁移回归均单独通过。

## 操作手册

- `docs/OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md`：场景统一保存和仓库抽屉。
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`：履约入口与流程。
- `docs/OP_MANUAL_FREIGHT.md`：普通发货与代发政策入口。
- `docs/OP_MANUAL_STOCK.md`：虚拟分类、客户绑定及删除保护。

上述 Markdown 由原有 Vue/Vite 手册入口展示，不增加根目录手册副本。

## 开发交付

待验证、分支推送、develop 集成和 development 部署后填写实际提交、备份和运行检查。
