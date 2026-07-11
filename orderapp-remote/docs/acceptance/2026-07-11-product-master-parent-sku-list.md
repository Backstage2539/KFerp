# PR-526 商品档案父商品与规格 SKU 分层展示验收

## 问题复现

- 商品“金色山脉”引用含 4 个销售规格的模板后，API 正确保存 1 个父商品和 4 个 `parent_product_id` 指向父商品的子 SKU。
- 修复前 Vue 商品档案列表直接遍历所有 product 行，显示为“金色山脉”及 4 个规格同级的 5 条商品。

## 验收口径

- 商品档案列表只展示父商品主行。
- 父商品名下可展开查看有效规格 SKU 的规格名和编号，并可点击进入原配置抽屉。
- 搜索、分页、数量和批量选择以父商品为单位；子 SKU 名称、规格和编号仍可搜索父商品。
- 不修改后端 SKU 数据模型、价格、BOM、库存或订单引用。

## 自动化证据

- RED：`node --test src/lib/product-settings.test.js` 因缺少 `productArchiveRowsWithSkus` 失败。
- GREEN：同命令通过 153/153，覆盖“金色山脉 + 4 个规格”聚合、子 SKU 搜索及 Vue 行内明细源码。
- 支持/API 合同：`go test ./internal/interfaces/http/support -count=1` 通过。
- 构建：`npm run build` 通过；`scripts/verify_kferp.sh changed` 通过。
- 完整前端基线：`scripts/verify_kferp.sh frontend-tests` 为 675/681；6 个失败属于既有客户工作区菜单契约，同一组测试在干净 `origin/develop` 提交 `69a742df` 上同样失败。
- 浏览器复现：正式环境商品档案在修复前确实渲染 5 条同级行：`金色山脉`、`金色山脉 100g袋装`、`金色山脉 227g袋装`、`金色山脉 Kg`、`金色山脉 磅`。
- 本次未按用户要求执行部署，因此修复后的正式环境浏览器验收待部署后完成。
