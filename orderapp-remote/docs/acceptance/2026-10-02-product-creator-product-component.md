# PR-679 商品规格作为 BOM 配方来源的连线提示

## 需求

挂耳成盒模板中，商品“10g袋装挂耳”可作为 BOM 配方来源。商品档案本身没有具体规格身份，配方连线应使用商品节点的“商品规格”输出口；运行时再选择具体商品规格。

## 实现

- 商品节点右侧显示各输出口名称，包括“商品对象”和“商品规格”。
- 将商品对象误连到 BOM 配方输入时，提示改连商品规格输出，并说明需要在运行表单选择具体规格。
- 商品规格仍通过 `item.specs` 连入配方，未放宽为没有规格身份的商品档案引用。
- 操作手册：`orderapp-remote/docs/OP_MANUAL_PRODUCT_CREATOR.md`。

## 验证

- RED：针对性 Node 用例先失败，实际为 `incompatible_data_type`，而验收预期是明确的商品规格连线提示。
- GREEN：商品档案口返回 `product_object_requires_specification`；商品规格口可连接 BOM 配方输入。
- 前端：Vue shell 1,306/1,306 项通过；Vite build 成功，构建 6,650 个模块；`git diff --check` 通过。
- API：未改 API 或服务端合同，配方继续使用 `item.specs`。
- 发布后 smoke：待完成。

## 部署与业务验收

- 开发与生产部署版本、备份位置及 HTTP smoke 将在发布后补录。
- Van 的挂耳模板业务验收待进行。
