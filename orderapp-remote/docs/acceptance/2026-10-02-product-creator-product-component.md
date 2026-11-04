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
- 开发环境部署：`c07697e2bc5f4bc6277990994e21ec63c0975321`；登录页 HTTP 200；备份 `/opt/stacks/erp/orderapp.backup.deploy-20261002145234-c07697e2bc5f`；回滚镜像 `kferp-orderapp-rollback:development-20261002145234-c07697e2bc5f`。
- 生产环境部署：`adb8a691cda64d4aa37a887342c5f9dcb13ff8b8`；登录页 HTTP 200；备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261002150237-adb8a691cda6`；回滚镜像 `kferp-orderapp-rollback:production-20261002150237-adb8a691cda6`。
- 开发、生产部署门禁均通过 Vue、小程序、Go/PostgreSQL/API 测试与构建；未上传或发布微信小程序。

## 部署与业务验收

- 开发已合入 `develop`，生产已合入 `main`；版本、回滚锚点及登录 HTTP smoke 如上。
- Van 的挂耳模板业务验收待进行。
