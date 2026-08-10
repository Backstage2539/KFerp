# PR-595 四列表统一分类移动交互验收记录

日期：2026-08-10

分支：`codex/four-list-category-move-20260810`
范围：物料档案、生产 BOM、商品档案、选中仓库的仓内物品列表。只实现、自动化验证并提交当前功能分支；不部署、不合并 `develop`。

## 验收口径

- 四页共用常驻左侧分类树、右侧面包屑和独立 `移动到分类` 按钮；分类树按“全部分类 → 模板 → 大类 → 小类/后代分类”显示数量、缩进、折叠和滚动。未选择任何模板时仍显示工作区，左树仅保留 `全部分类`，右侧平铺、移动禁用且底部两个模板入口可用。
- 进入移动模式后，右侧列表及查询、分页、行操作置灰；左侧显示 `请选择要移动到的分类` 并展开全部分支。全部分类和模板标题不可作为目标，大类、小类/后代分类及未分类可点击并立即移动，不二次确认。
- 成功后刷新归类、清空勾选并退出；失败后保留勾选和移动模式供重试；成功或取消恢复此前浏览分类、展开状态和滚动位置。
- 四页先执行各自既有过滤，再按左树分类过滤，不统一或替换原搜索条件。仓库继续按 `q/warehouse/item_type/customer_id/page/limit` 获取 stock API 当前服务端页，分类只过滤当前页 `rows`，不发送 `group_id/group_item_id`，`total/page/limit` 不随分类改变。
- 仓库在选中具体仓库且非客户库存上下文时显示分类工作区；对象身份为 `warehouse_inventory_item` 和精确 `object_ref=<warehouse code>:<item_type>:<item_id>:<spec_g>`。warehouse code 仅作为仓库命名空间前缀，实际 identity 是物品/规格，同一 identity 的多个批次只移动一次；移到未分类只删除该精确对象的 assignment。全部仓库和客户库存上下文仅在分类层面平铺且不可勾选移动，既有 WIP/追溯等上下文能力不变。
- PR-442/PR-458 的 warehouse-code `group_id/group_item_id` stock API 查询仅作为历史兼容保留，PR-595 当前仓内物品分类 UI 不读取或发送这两个参数。

## RED 证据

- 新增公共状态/树行为测试和四页接线测试后，首次运行 6 项全部失败：公共 helper、共享分类工作区和四页新状态均不存在。
- 补入多模板树顺序用例后，首次运行 5 项中 1 项失败，确认旧构造会先输出全部模板标题、再输出分类，不能保持每份模板的分类紧随模板。
- 新增 PR-595 支持契约后，首次 Go 定向测试因 `req_store.go` 缺少 `PR-595-UNIFIED-CATEGORY-MOVE-INTERACTION` 种子而失败。

## GREEN 证据

- 公共分类树、分类过滤、移动快照/恢复及多模板先序排列测试通过。
- 四页接线测试通过；物料、BOM、商品和仓库原有受影响源契约已更新到 `BusinessGroupWorkspace`、即时目标与失败重试语义。
- frontend-vue-shell 全量 `node --test src/lib/*.test.js`：925/925 通过。
- `go test ./internal/interfaces/http/support -count=1` 与 `go test ./internal/interfaces/http/materials -count=1`：通过。
- `scripts/verify_kferp.sh frontend-build`：Vite production build 通过，400 个 modules 完成转换；仅保留既有大 chunk 提示。
- `git diff --check`：通过。功能分支已重放到 `origin/develop` `aa22bf9b`，PR-588～594 为上游既有需求，本功能改用仓库确认的下一编号 PR-595。

## 自动化检查范围

- Vue：`src/lib/business-group-move.test.js`、`unified-category-move-ui.test.js`、`business-grouping.test.js`、`materials-ui.test.js`、`bom.test.js`、`product-settings.test.js`、`feature-group-selection-ui.test.js`，以及全量 `src/lib/*.test.js`。
- Go 支持契约：`TestDev595UnifiedCategoryMoveInteractionDeliveryContracts` 及受影响的历史分组页面契约。
- 构建：`scripts/verify_kferp.sh frontend-build`。
- 不执行浏览器/API 业务流或 development 人工验收；由 Van 后续按页面验收清单执行。

## 未执行事项

- 未部署 development 或 production。
- 未合并 `develop`，只准备当前功能分支供后续合并。
- 未执行浏览器手工验收、业务数据移动或线上写入。
- Van 业务验收待办。
