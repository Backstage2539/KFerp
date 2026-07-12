# PR-530 商品菜单与业务/系统设置整合验收

## 范围
- `商品与配方` 改名为 `商品`。
- 新增 `业务设置` 五 Tab：销售单设置、物流设置、发货人设置、分组模板、全局单位字典。
- 系统设置增加通知设置 Tab；设备产能配置和分散子设置从主菜单移除。
- 旧设置和设备产能地址保留兼容，不删除既有配置数据或 API。

## TDD 证据
- RED：`node --test src/lib/business-settings-ia.test.js`，实现前 4/4 失败，分别证明旧菜单名、业务设置页、独立全局单位组件和 App 路由缺失。
- GREEN：`node --test src/lib/business-settings-ia.test.js src/lib/menu-ia.test.js src/lib/group-settings-separation.test.js src/lib/menu-permissions.test.js src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/materials-ui.test.js src/lib/bom.test.js src/lib/operation-manuals.test.js`，通过 230/230。
- Build：`npm run build` 通过。
- Full frontend：681/687 通过；6 个 workspace-context 旧契约失败在干净 `origin/develop=dde51bc1` 同样复现，本需求未新增失败。
- Backend：`scripts/verify_kferp.sh backend` 全量通过。
- API：本需求不改变设置 API 合约；沿用销售单、物流、发货人、分组、单位、系统基础和通知设置的既有 JSON API。Go support 契约测试覆盖 PR/DEV、路由兼容与手册标记。

## 验收步骤
1. 打开开发环境，确认左侧一级菜单显示 `商品`，不显示 `商品与配方`。
2. 展开设置，确认存在 `业务设置` 和 `系统设置`，不存在销售单设置、物流设置、发货人设置、分组模板、通知配置、设备产能配置等重复入口。
3. 进入业务设置，依次切换五个 Tab，确认每个原设置页面正常加载。
4. 进入系统设置，切换系统基础设置和通知设置，确认全局单位字典不再出现在系统设置。
5. 检查旧 `view=salesOrderSettings`、`logisticsSettings`、`senderSettings`、`groupTemplates`、`groupManagement`、`notificationSettings`、`machines` 地址仍能打开兼容页面。

## 部署证据
- Application commit：`f5b4421e06095c82a6d150fe9af94bda68e9c317`，已快进合并 `origin/develop`。
- Deploy：`./deploy_orderapp.sh development`；备份 `/opt/stacks/erp/orderapp.backup.deploy-20260712143617`。
- Containers：`erp_orderapp` up，`erp_postgres` healthy，Docker build 内置 `go test ./...` 通过；最近五分钟 error/panic/fatal 行数为 0。
- HTTP/API：本机开发地址 `/app/` 未认证返回 303；服务端 resolve 后认证业务设置页、系统设置页、`/api/auth/me` 和 `/api/req/product?limit=500` 均返回 200，需求响应含 `PR-530-BUSINESS-SETTINGS-IA`。
- Browser：开发环境左侧显示 `商品`；业务设置依次显示销售单设置、物流设置、发货人设置、分组模板、全局单位字典五个 Tab；系统设置显示系统基础设置和通知设置，通知 Tab 打开通知配置内容。
- Browser menu：主菜单不存在销售单设置、物流设置、发货人设置、分组模板、通知配置或设备产能配置的独立入口；console error 0。
