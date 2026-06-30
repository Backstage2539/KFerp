# PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW

## Scope

Browser/API acceptance for the ERP main manufacturing loop: 原料 -> 商品 / SKU -> 下单 -> 生产. The run starts at raw-material setup and receipt, creates or reuses a sellable and manufacturable product/SKU, saves an order, submits production, completes execution/inventory steps where possible, and verifies inventory/order/log traceability.

## Baseline

- Branch: `codex/e2e-raw-material-order-production-flow-20260630`
- Backend baseline GREEN: `go test ./internal/interfaces/http/stock ./internal/interfaces/http/materials ./internal/interfaces/http/bom ./internal/interfaces/http/catalog ./internal/interfaces/http/sales ./internal/interfaces/http/production ./internal/application/production ./internal/application/materials ./internal/application/stock -count=1`
- Support baseline GREEN: `go test ./internal/interfaces/http/support -run 'TestDev50[0-7]|TestOperation|TestAudit' -count=1`
- Frontend baseline GREEN: targeted Node suite for materials, product settings, BOM, order entry, production, stock execution, manuals, and quality passed 337/337 after `npm ci`.
- Contract RED: `go test ./internal/interfaces/http/support -run TestDev509 -count=1 -v` failed before PR-509 docs/seed/manual markers were added.
- Merge/rename GREEN after `origin/develop` took PR-508 for BOM material loss ratio: `go test ./internal/interfaces/http/support -run TestDev509 -count=1 -v`; `go test ./internal/interfaces/http/support -count=1`; BOM/production/costing targeted Go packages; `node --test src/lib/bom.test.js src/lib/produce-plan.test.js src/lib/product-settings.test.js`; `git diff --check`.

## Browser/API Checklist

- [x] 原料建档/入库：browser created `PR508原料-20260630051716` (`PR508-RAW-20260630051716`, `kg`, purchase price `42.5`) on `view=materials`; browser submitted material receipt `MB-0000000011` on `view=materialReceipts` for `25 kg`, supplier `PR508供应商`.
- [x] 商品 / SKU：browser created parent product `PR508商品-20260630051716` with sales spec template `PR505跟进销售规格20260628225110`; parent SKU `SKU-000581`; derived child SKUs `SKU-000582` (`227g袋装`) and `SKU-000583` (`100g袋装`).
- [x] 生产 BOM：browser created `BOM-006578` / `PR508-BOM-20260630051716`, published `V001`, output `SKU-000582 PR508商品-20260630051716 227g袋装`, component `PR508原料-20260630051716 1 kg`, and set it as output SKU default BOM.
- [x] 下单：new E2E test product could not be ordered because the public price-list publication path is blocked by existing unrelated incomplete rows; order-chain acceptance continued with already published product `榛巧拼配`. Browser created order `SO-20260630-0001` for customer `CodexE2ETest`, line `榛巧拼配 454g x 1`, price `59.92/kg`, amount `27.20`, payment `已付款 / 微信支付`.
- [x] 生产计划：browser `view=producePlan&demand_status=unplanned` showed `SO-20260630-0001` as pending demand; created/submitted plan `PP-0000000058`; generated work order `WO-PP-0000000058-0000000041` (`work_order_id=36`) with BOM version `#861`, route `#4 标准烘焙`, planned output `558g`.
- [x] 生产执行：start initially blocked by `WIP stock insufficient: 哥斯达黎加盖博 need 1000g, available 0g`; browser used `view=stockOperations&tab=wip&work_order_id=36` to submit WIP transfer `MT-0000000020`, creating WIP batch `MB-0000000010 哥斯达黎加盖博 1000g`. Re-start succeeded with batch `A20260630-054217-18`, `running_item_id=45`, reserved WIP `1,453g`.
- [x] 工序卡/完工入库：browser completed job cards `#55 咖啡烘焙+除石` and `#56 色选` at `2026-06-30 13:43`; work order completed at `2026-06-30 13:45`, actual output `454g`, actual loss `104g / 18.6%`, consumed WIP `1,453g`, cost `113.63`.
- [x] 库存/订单履约：browser verified finished inventory on `view=warehouseInventory`: finished batch `FP-0000000045`, `榛巧拼配 454g`, `1 件 / 454g`, warehouse `成品仓`, updated `2026-06-30 13:45`. Browser generated shipping Excel `SHIP-20260630-0001`, filled tracking `SFPR508202606300001`, and order row became `发货：已发货 / 生产：生产完成`.
- [x] 出库/追溯/操作日志：browser generated delivery note PDF `V1` for `SO-20260630-0001` at `2026-06-30 13:53:23`; `view=stockOutboundLogs` shows `V1`, warehouse `成品仓`, delivery `顺丰发货`, tracking `SFPR508202606300001`. `view=audit&q=SO-20260630-0001` shows 操作日志 for `新建订单` and `生成出库单PDF`.

## Blocking Log

- New-product order blocker: `PR508商品-20260630051716` was created, grouped, and visible in price-list preview (`/227g`, preview price `1.56`, conversion `1 227g袋装 = 0.227 kg`), but publishing the public price table failed with existing unrelated rows: `榛巧拼配：缺少 227g袋装 到 g 的换算`, `PR439-20260606182321 工厂量单商品：缺少 lb 到 袋 的换算`, and fixed-price zero-price validation. This was not expanded inside PR-509 because it would change shared public price-list data beyond the raw-material -> product -> order -> production main-flow proof. The order/production/handoff path was completed with already published `榛巧拼配`.
- Search/select usability note: BOM material selector filtering matches material name, not code; selecting `PR508原料-20260630051716` worked, while searching `PR508-RAW-20260630051716` did not. This is recorded for follow-up but did not block the flow.
- UI interaction note: execution hub action `完工入库` only navigates focus; the actual work-order row `完工入库` button must be clicked after closing the drawer. This was worked through in-browser and did not require a code change.
- Transient infrastructure note: the first delivery-note PDF generation returned `请求失败` while `erp_orderapp` had just restarted. After reload, the same browser action generated `V1` successfully and outbound logs confirmed the document.

## Browser URLs

- Raw materials: `https://erp.qacoohee.com/app/vue-shell?view=materials`
- Material receipts: `https://erp.qacoohee.com/app/vue-shell?view=materialReceipts`
- Product master: `https://erp.qacoohee.com/app/vue-shell?view=productMaster`
- BOM: `https://erp.qacoohee.com/app/vue-shell?view=bom`
- Order list: `https://erp.qacoohee.com/app/vue-shell?view=orders&q=SO-20260630-0001`
- Production plan: `https://erp.qacoohee.com/app/vue-shell?view=producePlan&demand_status=unplanned`
- Work order: `https://erp.qacoohee.com/app/vue-shell?view=workOrders&work_order_id=36`
- Job cards: `https://erp.qacoohee.com/app/vue-shell?view=jobCards&work_order_id=36`
- WIP transfer: `https://erp.qacoohee.com/app/vue-shell?view=stockOperations&tab=wip&work_order_id=36`
- Finished inventory: `https://erp.qacoohee.com/app/vue-shell?view=warehouseInventory&warehouse=finished_goods&item_type=finished_product`
- Delivery note: `https://erp.qacoohee.com/app/vue-shell?view=deliveryNote&order_id=1555`
- Outbound logs: `https://erp.qacoohee.com/app/vue-shell?view=stockOutboundLogs`
- Audit log: `https://erp.qacoohee.com/app/vue-shell?view=audit&q=SO-20260630-0001`
