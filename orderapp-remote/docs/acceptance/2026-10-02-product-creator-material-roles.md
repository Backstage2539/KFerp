# PR-678 商品创建器物料角色与命名简化

## 根因与范围

2026-10-02 只读检查 development 的模板 #4 `PR-676 V4 商品规格模板验收-半成品到成品-20260930`：修订 37、已发布 V13、草稿格式 V6。烘焙半成品节点为 output，但未存 supply_mode；界面默认显示自制，发布校验直接比较空值，造成误报。修正为同一默认解析，明确配置外购仍会拒绝。

新建/编辑草稿使用 V7，取消投入默认行及名称预设，运行默认选择已有外购物料，必要时在行内临时新建。物料节点顶部统一取得方式；外购无输入口，自制必须连接且只能连接一个生成 BOM。真实已有物料属性由数据库校验，不能靠请求中的取得方式伪装。

命名默认编辑器改为「＋文本／＋变量」，按点击顺序追加，变量搜索/新增后跨节点共用；连续文字输入即时预览，不再排列移动箭头和常驻搜索区。切换取得方式后端口立即刷新；撤销恢复连线和配置。采购、分类、成本及历史 V1–V6 运行保持原规则。

## RED / GREEN

- `TestLegacyV6OutputImplicitManufactureMatchesDisplayedDefault`：修复前合法旧输出遭拒，修复后通过；显式外购仍阻止。
- `TestV7MaterialModeControlsPortsAndRequiresManufacturingBOM`、`TestV7InputDoesNotCreateTemplatePresetMaterials`：修复前外购仍有入口、旧行被实例化；修复后通过。
- 新组件挂载测试先因缺组件失败，完成后验证连续文字、既有变量选择、新变量创建及预览示例隔离。
- `TestMaterialSourceReadsRealModeAndPublishedBindingPostgres` 新增 V7 断言先失败；修复后数据库真实自制物料不再能作为外购来源。
- `TestMaterialAcquisitionPublicationAPI`：V6 隐式自制和 V7 显式自制发布成功，缺少生成 BOM 返回 422，且不增加模板版本。

## 已完成技术检查

- Go 领域、API、PostgreSQL productcreator 三包通过。独立本机临时 PostgreSQL（端口 55438、独立数据目录）执行来源检查及已有事务测试，通过后停止；未连接开发/生产业务数据库执行写入测试。
- Node 创建器定向 42/42，补充实际运行表单渲染 2/2；完整 Vue 测试原 1303/1303，补充后完整门禁 1305/1305 及构建通过；`scripts/verify_kferp.sh all` 标准环境通过，Vue 生产构建通过。
- 额外尝试为全仓库 Go 测试启用 PostgreSQL，发现现有无关 costing/customer 测试的精简表结构缺失 `customer_order_price_table_bindings`、`estimated_unit_price`、`customer_assets` 等，且存在旧错误文案断言；该扩展检查未通过，未将其计为通过。受影响的创建器 PostgreSQL 测试单独通过。标准发布门禁未启用这些可选全仓数据库测试。
- 本机 Chrome 使用该真实模板的只读副本和拦截 API（无线上写入）验证：升级、外购无口、自制有口、命名逐字预览、变量新建、保存草稿、切换取得方式、撤销及保存重开。修正检查中发现的 VueFlow 端口刷新问题后通过。
- 命名区域在 1440 / 1024 / 768 / 666 像素下检查无横向溢出；无页面脚本错误。截图保存在本机 `/tmp/pc-v7-name-*.png`。
- 运行表单补充检查发现 V6 遗留的 `variableName` 缺失：含变量的产出名称会导致渲染异常。新增实际 SFC 渲染测试在 V6/V7 均 RED，补齐变量标签解析后均 GREEN。Chrome 进一步验证 V7 初始为引用已有、临时新建外购物料名称为空、无模板命名预设、新增行仍为引用已有，保存草稿保留名称和外购属性。修正物料行操作列挤压数据字段的问题，666px 下每个输入/选择控件宽度至少 150px，暂估价格标签可换行。
- 手册：`docs/OP_MANUAL_PRODUCT_CREATOR.md`；Vue 菜单 `productCreatorManual` 继续读取该唯一来源；手册索引已更新。

## 交付边界

仅合并 develop 并部署 development。保留原 checkout 未提交工作；没有修改线上模板、商品、物料、BOM 或生产环境。Van 刷新页面后重新打开模板编辑，保存并发布即可使用 V7；原已发布版本和既有运行不会自动切换规则。Van 业务验收待确认。

部署版本及服务验证将在完成部署后补记。
