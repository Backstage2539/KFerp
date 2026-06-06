# PR-417 多层制造 BOM 主模型

## 范围
- 生产 BOM 改为制造主档：声明产出商品、产出数量/单位、版本和组件清单。
- 组件支持 `material` 和 `product`；旧 `finished_product` 作为历史兼容读取并规范化为商品组件。
- 商品档案配置抽屉只读展示“被哪些 BOM 使用”，不再编辑生产 BOM 绑定；PR-420 后该列表展示产出当前商品的 BOM，也展示把当前商品作为组件消耗的上层 BOM，并用关系标签区分。BOM 详情页仍只展示上层组件关系。
- 工单页增加按 BOM 预览生产需求，支持只看第一层、全部展开、按库存缺口展开。

## 验收点
- 创建 BOM 时必须选择产出商品；发布时校验产出商品、组件非空和循环引用。
- 已发布 BOM 版本只读；复制为新版草稿后才能调整产出基准或组件。
- 盒装速溶、盒装挂耳、发动机总成等结构可通过商品组件形成多层 BOM。
- 商品没有自己的产出 BOM 时，也能在商品档案中看到被上层 BOM 使用的关系；商品有产出 BOM 时，产出关系也会显示在商品档案“被哪些 BOM 使用”列表中。

## 证据
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1`
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js`
- 待本分支收尾时补充 Vue build、changed verifier 和浏览器验收结果。
