# PR-596 四列表分类与业务列表内联验收记录

日期：2026-08-10

功能分支：`codex/inline-category-lists-20260810`

交付跟踪分支：`codex/pr596-delivery-evidence-20260810`

范围：物料档案、生产 BOM、商品档案，以及选中具体仓库且非客户库存上下文的仓内物品列表。

## 覆盖边界

- PR-596 覆盖 PR-588 的 BOM 单展开手风琴、PR-595 的内层“左侧分类树 + 右侧业务表”布局，以及具体仓库“只分类当前服务端页并保留全局分页”的客户端编排：四页改为在业务列表中按“模板 → 父分类 → 后代分类 → 业务表”直接内联，具体仓库拉齐过滤结果后只显示分类独立分页。
- PR-588/PR-595 已确定的功能自选模板、即时移动、失败重试、各页独立筛选、对象 identity、assignment API 和操作日志不变；本需求不新增数据库表、归类接口或日志类型。
- 分组模板仍在 `设置 → 业务设置 → 分组模板` 维护。四个功能仍分别有序读写 `product_catalog`、`material_catalog`、`production_bom`、`warehouse_inventory` 的 feature selection；空选择不删除模板、分类、对象归类或历史快照。

## UI 验收口径

- 四页不再显示内层左侧分类树、右侧分类面包屑或永久右侧详情栏。所选模板、空分类、父分类、后代分类、一个统一的 `未分类` 和业务行按层级内联；父分类自身直接归属的对象必须显示，不能因存在子分类而丢失。
- 每个展开且含业务对象的分类重复该页面完整表头，并有自己的页码和每页条数。分类 A 翻页或改每页条数不影响分类 B；分类收起再展开保留当前页，搜索、状态、模板选择或业务上下文变化时按页面合同重置。
- 未选择模板时只显示一个 `全部分类` 平铺区，`移动到分类` 禁用，列表底部的 `前往分组模板` 和 `设置分组模板` 保留。
- 进入移动模式后，业务行、筛选、分类内分页和行操作置灰不可用；大类、小类/后代分类和 `未分类` 标题可直接点击并立即移动。`全部分类` 和模板标题不可作为目标；不显示目标模板/分类下拉，也不二次确认。成功清空勾选并退出，失败保留勾选和移动模式供重试，取消恢复正常浏览。

## 四页独立能力

- 物料档案保留名称/编码/批次搜索和启用状态过滤；点击物料名称打开物料详情抽屉。分类分页在该页筛选结果之后独立切片。
- 生产 BOM 保留状态及名称/编号搜索；点击 BOM 名称打开包含主档、版本和配方的完整设置抽屉。分类分页互相独立，不恢复 PR-588 的单展开全局分页。
- 商品档案保留名称/类型/备注、状态、父商品及销售规格语义；点击商品名称继续打开既有 `商品档案配置` 抽屉，不新增另一套商品详情抽屉。父商品参与分类分页，派生规格仍收在父商品明细中。
- 仓库页保留外层仓库选择器。全部仓库和客户库存上下文继续平铺且不可勾选移动；WIP、追溯、模板设置及其他既有抽屉不变。只有具体仓库且非客户库存上下文使用仓内内联分类。

## 仓库过滤结果、分页与身份

- 选中具体仓库且非客户库存上下文时，页面先按当前 `q/warehouse/item_type`、`page=1&limit=500` 请求 stock API；若响应 `total` 大于已取回行数，继续请求后续页直到拉齐该过滤条件下全部结果，再进行分类和每分类独立分页。具体仓库界面不显示全局服务端分页，也不发送 `group_id/group_item_id`，任一可见分页只服务其所在分类。
- 全部仓库和客户库存上下文继续按原 `q/warehouse/item_type/customer_id/page/limit` 使用服务端分页和平铺列表，不进入具体仓库的拉齐/内联分类分支。
- 仓库继续使用 `usage_key=warehouse_inventory`、`object_key=warehouse_inventory_item` 和精确 `object_ref=<warehouse code>:<item_type>:<item_id>:<spec_g>`。warehouse code 只是仓库命名空间，物品/规格才是 identity，同一 identity 的多个批次共享归类；移到未分类只删除该精确 assignment。
- 商品、物料、生产 BOM 分别继续使用 `product_catalog/product`、`material_catalog/material`、`production_bom/production_bom`。PR-442/PR-458 的仓库 code assignment 和 stock `group_id/group_item_id` 查询只保留历史兼容，不作为 PR-596 UI。

## 自动化合同

- 共享内联工作区合同覆盖：模板/分类层级、空分类、父分类直接行、统一未分类、无模板平铺、折叠、移动目标、成功/失败/取消和每分类独立分页。
- 四页接线合同覆盖：不再引用 PR-595 的内层分类工作区状态；保留各页原筛选、表头、行选择、名称抽屉及 feature selection 语义。
- 仓库专属合同覆盖：外层仓库选择器；具体仓库以 `limit=500` 起始并按 `total` 补齐全部 `q/warehouse/item_type` 过滤结果、只显示分类独立分页；全部仓库/客户库存继续原服务端分页；精确 `warehouse_inventory_item` object reference，以及 WIP/追溯抽屉保留。
- 最终整合验证：frontend `src/lib` 全量单元测试 943/943 通过；Vite production build 通过（400 modules，只有既有 chunk-size warning）；`go test -count=1 ./...` 全包通过；`scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。功能分支已通过整合提交 `cfc781df3e8cb540ec4d853bdd30ebf108caa26b` 合入最新 develop 基线并完成 development 首次部署；本次交付跟踪合同仍按 RED→GREEN 单独留证。

## RED 证据

- 先新增 `TestDev596InlineCategoryListsDeliveryContracts`，再运行 `go test ./internal/interfaces/http/support -run TestDev596InlineCategoryListsDeliveryContracts -count=1`。
- RED 如期出现：`req_store.go missing one-line req_product seed PR-596-INLINE-CATEGORY-LISTS with status review and assignee VA`；当时 PR-596 的产品、5 个 DEV、REV 种子及 deployment/visual-QA 跟踪尚未进入自动合同。

## GREEN 证据

- 补齐 PR/DEV/REV 种子、ACTIVE 状态和本验收记录后，运行同一定向命令得到 `ok orderapp/internal/interfaces/http/support 0.840s`，PR-596 交付合同 GREEN。

## development 首次部署证据

- 功能提交 `0e4de35ad220bc9c594e3fc717678bd431e452b4` 已通过 merge commit `cfc781df3e8cb540ec4d853bdd30ebf108caa26b` 整合到最新 develop 基线；development 首次部署已完成。
- 该检查点证明首次 development 代码交付已发生，不替代四页 rendered visual QA，也不表示本交付跟踪补丁已经提交。

## 待完成事项

- 视觉 QA 待完成：在 development 对物料、BOM、商品、具体仓库四页的层级、重复表头、独立分页、名称抽屉和移动模式做 rendered design QA。
- 最终跟踪补丁待收尾：当前 `DEV-596-DOCS-DEVELOPMENT-DELIVERY` 保持 doing，待本合同 GREEN、补丁提交及主任务最终记录后再关闭。
- `REV-596-INLINE-CATEGORY-LISTS` 保持 todo，等待 Van 验收；production 未部署。

## 人工验收清单

- [ ] 四页均无内层左树/右表分栏，模板、分类标题、重复表头和列表行的层级与缩进清楚。
- [ ] 两个有数据分类同时展开时，各自翻页、修改每页条数和全选互不影响；父分类直接对象、子分类对象和未分类对象均不丢失。
- [ ] 物料名称打开物料详情抽屉；BOM 名称打开完整 BOM 设置抽屉；商品名称打开现有商品档案配置抽屉。
- [ ] 四页移动模式的置灰、即时分类标题目标、成功退出、失败重试和取消恢复符合合同。
- [ ] 具体仓库以 `limit=500` 拉取并在需要时补齐多页后，所有符合 `q/warehouse/item_type` 的物品都进入内联分类；界面无全局服务端分页，两个分类各自翻页且互不影响。全部仓库和客户库存上下文仍显示原服务端分页。
- [ ] 全部仓库和客户库存上下文保持平铺不可移动，WIP、追溯及既有设置抽屉仍可用。

## 非证据项

- 本记录不代表已执行浏览器业务写入验收或 rendered visual QA；development 首次部署不等于 Van 最终验收。production 未部署。
