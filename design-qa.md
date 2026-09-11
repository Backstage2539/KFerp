# Design QA Archive

## PR-596 Inline Category Lists

日期：2026-08-10

环境：development

视口：四张参考图、四张 development 实现图和四张逐页对比图均为 1536×1024。

字体基线：system Chinese sans；浏览器使用 macOS 系统中文无衬线字体栈（`-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif）进行对照，不引入参考图之外的展示字体。

截图像素/1x 密度归一：参考图、development 实现图和逐页对比图均已核对为 1536×1024 像素；QA 统一按 1x CSS 像素密度比较，不用 Retina 倍率差异、浏览器缩放或二次重采样解释布局偏差。

最终状态：passed

## 测试 state

- 环境 state：最终业务代码 `8e0aa8bfe86e26e4d0009603231752afb95d4ef2` 已部署 development，登录页 HTTP 200；浏览器使用已登录 ERP 会话，只做读取与本地界面操作。
- 生产 BOM state：生产 BOM 列表显示内联模板/分类，点击 BOM 名称后的完整设置抽屉保持打开，用于核对列表、版本和配方区域。
- 物料档案 state：物料内联分类列表与物料详情抽屉同时可见，用于核对重复表头、名称入口和详情布局。
- 商品档案 state：父商品内联分类列表与既有 `商品档案配置` 抽屉同时可见，销售规格仍收在父商品语义下。
- 仓库库存 state：选中具体仓库且非客户库存上下文，展示仓内物品/规格的内联分类和分类独立分页；全部仓库与客户库存分支不进入该 state。
- 操作 state：检查分类展开/收起、分类内分页、筛选控件及抽屉打开/关闭；未执行移动归类、批量失效、保存或其他业务写入。

## Fidelity surfaces

- typography：对照 system Chinese sans 的字号层级、字重、行高、表头与正文密度；四页标题、分类标题、表头和抽屉文本层级一致。
- spacing/layout rhythm：对照页面边距、工具栏间距、分类缩进、标题到表格距离、表格行高和抽屉内区块节奏；不同业务列宽可以随该页原表头变化，但层级节奏一致。
- colors/tokens：复用 ERP 既有蓝灰背景、边框、正文、次要文字、状态和禁用色 token；移动模式及分类层级不引入脱离现有色系的新颜色。
- assets：迭代 1 后统一使用 Tabler chevron/folder/folder-off，替代文本 +/- 和无文件夹图标的状态；图标尺寸、描边与文本基线一致。
- copy/content：核对页面标题、筛选项、`移动到分类`、分类标题、重复表头和抽屉关键文案；参考图使用确认阶段示例记录，development 实现图使用当前开发环境记录，因此商品/BOM/物料名称、编号、行数、分类数量和状态等开发数据差异属于预期，不作为视觉 fidelity 缺陷。

### 参考图、实现图与对比图

- 生产 BOM 参考图：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-8766b84c-1c9f-40d0-955b-be61088d973f.png`
- 物料档案参考图：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-785663f3-5d66-414a-9ed2-1193b3e235f6.png`
- 商品档案参考图：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-9963bf95-ebb8-4fa9-9f53-823c0de1a936.png`
- 仓库库存参考图：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-10ff9690-6394-46d5-a24f-1f352ac4146b.png`
- 生产 BOM 实现图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/production-bom-drawer.png`
- 物料档案实现图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/materials-drawer.png`
- 商品档案实现图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/products-drawer.png`
- 仓库库存实现图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/warehouse.png`
- 生产 BOM 对比图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-bom.png`
- 物料档案对比图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-materials.png`
- 商品档案对比图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-products.png`
- 仓库库存对比图：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-warehouse.png`

### 迭代记录

- 迭代 1：P2。差异为分类标题仍使用文本 +/-，且分类层级无文件夹图标；修正为统一的 Tabler chevron/folder/folder-off。
- 迭代 2：无 P0/P1/P2。

### 交互只读检查

- 在 development 完成页面加载、分类展开/收起、分类内分页、名称抽屉打开/关闭和筛选控件检查，未执行业务写入。
- 最终结论：`passed`

---

## PR-651 Production Plan Capacity Workspace

**Source visual truth**

- `/var/folders/p7/235nxhzn3_bbbyjsb56491xr0000gq/T/codex-clipboard-e2847a83-d25b-4cbf-bf7c-2ad373461bd9.png`

**Browser-rendered implementation evidence**

- Local component, first fixture operation: `/private/tmp/kferp-pr651-qa/capacity-operation-1.png`
- Local component, second fixture operation: `/private/tmp/kferp-pr651-qa/capacity-operation-2.png`
- Development `PP-0000000109`, upstream operation: `/private/tmp/kferp-pr651-acceptance/pp109-capacity-roast.png`
- Development `PP-0000000109`, downstream operation and source-task review: `/private/tmp/kferp-pr651-acceptance/pp109-capacity-package.png`

**Viewport and normalization**

- Source pixels: `1488 × 1058`.
- Local implementation pixels: `1488 × 1058`; CSS viewport `1488 × 1058`; `deviceScaleFactor: 1`.
- Final development screenshots: `1468 × 851`, captured from the existing authenticated Chrome ERP shell without resampling.

**Final development state**

- Draft production plan `PP-0000000109`; acceptance was read-only.
- The frozen route supplies the actual names `咖啡烘焙+除石` and `包装`.
- The dependency-aware sequence is `咖啡烘焙+除石 → 包装` even though each item-specific route starts its own operation numbering at 1.
- The upstream task is `初晓 5.64kg`; the downstream operation preserves `10袋 / 8袋 / 2袋` as three task groups with six source orders and customer traceability.
- All tasks are covered. Seven material/source issues remain visible for later plan-detail handling and do not prevent saving the capacity split.

**Full-view comparison evidence**

- Fonts and typography: both views use a compact Chinese system sans-serif hierarchy with a strong plan number, medium section titles, and subdued explanatory copy.
- Spacing and layout rhythm: the implementation keeps the reference's white panels, light borders, restrained radii, green section markers, right-side review region, and sticky action footer. The focused workspace gives allocation controls more horizontal room than the source plan-detail composition.
- Colors and visual tokens: semantic green, pale green surfaces, muted gray text, amber shortage state, and white canvas follow the reference palette and existing KFerp tokens.
- Copy and content: the implementation uses frozen route operation names, separates `计划数量 / 已安排 / 还需安排`, preserves each source order, and explains that confirmation saves the split without submitting the plan or generating work orders.
- Overall composition: the development page matches the target's dense ERP hierarchy while keeping operation selection, task allocation, source trace, review, and actions visible in one workspace.

**Primary interactions tested**

- Opened `PP-0000000109` from the production-plan list and switched between both actual operation tabs.
- Verified operation 01 is `咖啡烘焙+除石`, operation 02 is `包装`, and each tab changes the task content.
- Verified package task groups `10袋 / 8袋 / 2袋`, their source order numbers, customer names, and right-side per-task review.
- Confirmed the page explains that `确认安排` does not submit the plan or generate work orders. No save or confirmation write was performed during acceptance.

**Console and service checks**

- Browser console error and warning log: empty.
- Development container is running, PostgreSQL is healthy, internal login returns 200, external login returns 200, and recent application logs contain no panic/fatal/error line.

**Comparison history**

- Iteration 1: local component comparison at `1488 × 1058` found no actionable visual mismatch.
- Iteration 2: first development acceptance found a P2 sequence error: two item-specific routes both started at sequence 1, so lexical fallback placed downstream `包装` before upstream `咖啡烘焙+除石`.
- Iteration 3: added a RED regression using the manufacturing-plan dependency edge, sorted by dependency stage before route sequence, redeployed, and captured both corrected live states. No actionable P0/P1/P2 remains.

**Implementation checklist**

- [x] Use frozen operation names instead of fixed process labels.
- [x] Order operations by the plan dependency graph across item-specific routes.
- [x] Preserve source-order task quantities and customer traceability.
- [x] Show native weight and sales units.
- [x] Keep draft save available and gate confirmation on per-task coverage.
- [x] Match the reference's restrained green ERP style.
- [x] Verify the implementation in the actual development ERP shell.

**Follow-up polish**

- None required for handoff.

final result: passed
