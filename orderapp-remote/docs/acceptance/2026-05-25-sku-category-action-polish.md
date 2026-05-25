# PR-367 SKU分类操作按钮优化验收

## 范围

- 只调整 SKU设置 → 商品资料 → 商品分类中的新增、删除、排序控件样式和删除交互。
- 不改变商品分类 API、SKU 分类归属、子类型拖动排序、商品配置模板或单位模板逻辑。

## 验收步骤

1. 进入 SKU设置，停留在“商品资料”。
2. 在左侧“商品分类”区域确认搜索框下方的产品类型加号/减号是紧凑胶囊按钮，不再是大方块按钮。
3. 查看任意产品类型行，确认上移/下移按钮在右侧并使用紧凑胶囊样式。
4. 点击产品类型工具区的减号进入删除模式，确认红色删除减号显示在产品类型行右侧。
5. 点击产品类型行右侧红色减号，系统不再弹出二次确认，直接删除该分类；分类内 SKU 回到未分类。
6. 在任意产品类型标题下点击子类型减号进入子类型删除模式，确认红色删除减号显示在产品子类型行右侧。
7. 点击子类型行右侧红色减号，系统不再弹出二次确认，直接删除该子类型；子类型内 SKU 回到未分类。

## 预期结果

- 加减号和排序按钮视觉更轻，不再占用大块空间。
- 一级产品类型和二级产品子类型的红色删除按钮都在右侧。
- 删除分类不再需要二次确认。
- 现有点击名称改名、一级分类排序和二级分类拖动排序仍可使用。

## 验证证据

- Unit/UI guard: `node --test src/lib/product-settings.test.js`
- API/support guard: `go test ./internal/interfaces/http/support -run 'TestDev367|TestDev365|TestProductSettingsVueSupportsCategoryDelete' -count=1`
- Backend regression: `go test ./...`
- Frontend build: `npm run build`
- Diff hygiene: `git diff --check`
