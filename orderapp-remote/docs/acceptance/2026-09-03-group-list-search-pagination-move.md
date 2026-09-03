# PR-626 分组列表搜索、分页与移动交互验收记录

## 范围

- 页面：商品档案、物料档案、生产 BOM、选中具体工厂仓库后的仓库库存。
- 不变边界：各页原搜索字段、状态/类型/仓库筛选、对象身份、`business-group-assignments` 移动接口与操作日志。
- 排除：全部仓库/客户库存服务端分页、数据库结构、后端业务 API、production、微信上传和 Van 页面业务验收。

## TDD 证据

- RED：前端定向测试先因缺少 `businessGroupSearchCollapsedKeys`、四页 `search-query`、统一业务行定位标记、10 条分页阈值和紧凑移动模式而失败。support 合同测试随后因 PR/DEV 种子、需求、手册和本验收记录尚不存在而失败。
- GREEN：`business-grouping.test.js` 覆盖父类无直接命中但子类命中、空分支、零结果、0/10/11 条分页边界、分类独立页码和移动标题展示；页面合同覆盖四页各自搜索生效时机、首条业务行定位、分页栏条件和既有移动 payload。

## 自动验收结果

- [x] 搜索使用完整子树计数：命中后代时祖先展开，无命中模板/分类/未分类收起。
- [x] 第一条实际业务行带统一定位标记和程序化焦点；零结果无可聚焦行时不移动焦点。
- [x] 搜索开始前保存折叠及工作区、滚动容器和窗口位置；清空后恢复。
- [x] 商品档案与生产 BOM 实时应用搜索；物料档案和仓库库存只在请求成功后更新“已应用搜索词”。
- [x] 分类 0 或 10 条不切片且无分页栏；11 条起保留分类独立分页及当前页全选范围。
- [x] 移动模式显示所有模板、大类、小类/后代分类和未分类标题，不渲染业务行与分页；目标权限保持“分类/未分类可选，全部分类/模板不可选”。
- [x] 成功/取消退出后恢复浏览状态，失败路径不退出移动模式且不清空勾选。
- [x] 四页继续使用既有 `product_catalog/product`、`material_catalog/material`、`production_bom/production_bom`、`warehouse_inventory/warehouse_inventory_item + object_ref` 合同。

## 门禁结果

- [x] 相关前端定向测试：7 个测试文件，302/302 通过。
- [x] 完整前端测试：`scripts/verify_kferp.sh frontend-tests`，1060/1060 通过。
- [x] 完整后端测试：`scripts/verify_kferp.sh backend` 通过，包含 catalog、materials、bom、stock 与 support。
- [x] Vue 构建：`scripts/verify_kferp.sh frontend-build` 通过，6596 个模块完成构建；仅有既有大 chunk 提示。
- [x] 改动范围检查：`scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。

## 待人工验收

- [ ] Van 在 development 四个页面分别检查搜索定位、清空恢复、10/11 条分页边界和移动模式。
- [ ] production 未部署；微信小程序无改动、无上传。
