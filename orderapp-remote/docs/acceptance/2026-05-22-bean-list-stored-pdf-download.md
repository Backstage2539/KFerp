# 2026-05-22 豆单系统 PDF 下载验收

- 需求：PR-317-BEAN-LIST-STORED-PDF-DOWNLOAD
- 范围：产品豆单“豆单版本列表”的 PDF 下载。

## 验收要点
- 点击任一有内容快照的豆单版本“下载 PDF”时，不出现浏览器打印窗口。
- 前端先调用 `POST /api/costing/bean-list/publications/:id/pdf` 生成或复用 PDF，再按返回的 `download_url` 下载文件。
- 第一次生成写入 `bean_list_publication_assets(publication_id, asset_type='pdf')`，同一版本再次下载复用缓存。
- 下载内容来自该行 `content/config` 快照，不受当前抽屉配置、当前商品实时价格或筛选状态影响。
- 客户专属豆单只能在对应客户范围下载；公共豆单按公共范围下载；本人草稿按本人范围下载。

## 测试证据
- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing`
- `node --test src/lib/costing-bean-list-version-ui.test.js`
