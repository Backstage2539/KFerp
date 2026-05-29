# PR-376 商品配置特殊 KV 与产品价格表修正

## 范围
- 产品价格表预览卡片按 SKU 产品类型动态生成。
- 商品配置模板支持固定单价、成本加成必填校验和特殊 KV 定义。
- SKU 保存特殊 KV 值，产品价格表快照和 PDF 展示已勾选 KV。
- 客户复制公共商品配置模板保持客户上下文并幂等；复制后客户模板列表隐藏已派生的公共原模板。

## 验收步骤
1. 在 SKU设置 → 商品配置模板新增或编辑“速溶盒装配置”，阶梯价模板选择“盒”单位模板。
2. 在价格表生成规则选择“固定单价”，不填固定单价时保存应失败；填写每盒单价后保存成功。
3. 改成“成本加成”，不填加成比例时保存应失败；填写比例后保存成功。
4. 在特殊 KV 定义中新增 `roast_level / 烘焙度 / 下拉：浅烘、中烘、中深烘、深烘` 并勾选“产品价格表展示”。
5. 在绑定该商品配置的 SKU 行填写“烘焙度=中深烘”，保存后刷新仍保留。
6. 进入产品价格表，确认“生成价格表”下方卡片来自产品类型，例如“速溶咖啡产品价格表”，不是固定旧四张卡片。
7. 生成并发布速溶咖啡产品价格表，确认单位为“盒”，产品信息和 PDF 中出现“烘焙度：中深烘”。
8. 切到某个履约客户，复制公共商品配置模板；确认页面仍在该客户上下文，只新增一条客户模板，已派生的公共原模板从该客户模板列表隐藏。重复复制同一公共模板时返回已有客户模板。

## 自动化证据
- 2026-05-26：`go test ./internal/appmain ./internal/application/catalog ./internal/domain/costing ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` 通过。
- 2026-05-26：`node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js` 通过，84/84。
- 2026-05-26：`npm run build` 通过。
- 2026-05-26：本地 API `/api/costing/bean-list` 返回两款速溶 SKU，固定单价为 `18/盒`，成本加成为 `13/盒`，`product_attributes` 含 `烘焙度：中深烘` 和 `条装规格：1条10g；20条/盒`。
- 2026-05-26：连续两次调用 `/api/product-settings/product-config-templates/derive` 复制同一公共商品配置，均返回同一客户模板 `id=3`，公共模板数量仍为 2，客户复制结果只有 1 条。

## 浏览器验收
- 截图：`/tmp/kferp-special-kv-price-list.png`。
- 截图：`/tmp/kferp-special-kv-customer-config.png`。
- 产品价格表页面显示“速溶咖啡产品价格表”，不再出现旧固定四卡片标题。
- 价格表卡片显示 `速溶盒装固定价测试`、`速溶盒装成本加成测试`，单位为 `盒`，价格分别为 `18/盒` 和 `13/盒`。
- 价格表卡片显示特殊 KV：`烘焙度：中深烘`、`条装规格：1条10g；20条/盒`。
- 客户 SKU 的商品配置页仍保持 `客户 #1 SKU` 上下文，`客户复制速溶配置测试` 可见，已派生的公共原模板不再出现在客户模板列表。
