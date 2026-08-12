# PR-596 Design QA

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

## 参考图、实现图与对比图

### 生产 BOM

- 参考图绝对路径：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-8766b84c-1c9f-40d0-955b-be61088d973f.png`
- development 实现图绝对路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/production-bom-drawer.png`
- 对比图路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-bom.png`

### 物料档案

- 参考图绝对路径：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-785663f3-5d66-414a-9ed2-1193b3e235f6.png`
- development 实现图绝对路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/materials-drawer.png`
- 对比图路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-materials.png`

### 商品档案

- 参考图绝对路径：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-9963bf95-ebb8-4fa9-9f53-823c0de1a936.png`
- development 实现图绝对路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/products-drawer.png`
- 对比图路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-products.png`

### 仓库库存

- 参考图绝对路径：`/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-10ff9690-6394-46d5-a24f-1f352ac4146b.png`
- development 实现图绝对路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/warehouse.png`
- 对比图路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-warehouse.png`

### 汇总对比

- 汇总对比图路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison.png`
- 对比页路径：`/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison.html`

## 迭代记录

### 迭代 1

- 分级结果：P2。
- 差异：分类标题仍使用文本 +/-，且分类层级无文件夹图标，视觉语义弱于确认效果图。
- 修正：统一改成 Tabler chevron/folder/folder-off；展开、收起、普通分类和未分类使用一致的线性图标语言。

### 迭代 2

- 迭代 2：无 P0/P1/P2。
- 四页层级缩进、分类标题、重复表头、抽屉布局及图标语义与确认效果一致，无需继续视觉修正。

## 交互只读检查

- 在 development 完成四页交互只读检查：页面加载、分类展开/收起、分类内分页、名称抽屉打开/关闭和筛选控件可用。
- 检查未执行 `移动到分类`、归类保存、批量失效或其他业务写入；REV 仍由 Van 完成最终业务验收。

## 结论

最终结论：`passed`
