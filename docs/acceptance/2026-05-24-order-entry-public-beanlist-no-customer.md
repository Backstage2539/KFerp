# PR-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER

## Requirement
When an operator opens a new order form before selecting a customer, the bean-list summary must not show all bean lists as "暂无" if public published bean lists exist. Public published commercial, green, and drip bean lists should be available as the initial default choices. After a customer is selected, customer-owned bean lists still take precedence by type; otherwise the public selection remains the fallback.

## Evidence
- RED: `node --test src/lib/order-entry.test.js --test-name-pattern "beanListVersionOptionsForCustomer deduplicates repeated public fallbacks"` failed because no-customer filtering returned no public versions.
- RED: `go test ./internal/infrastructure/postgres/sales -run TestOrderFormBeanListVersionOptionsIncludeGlobalPublicFallback -count=1` failed because `/api/order/form` did not expose global public rows.
- GREEN: `node --test src/lib/order-entry.test.js --test-name-pattern "beanListVersionOptionsForCustomer exposes public published versions before a customer is selected|beanListVersionOptionsForCustomer deduplicates repeated public fallbacks|OrderEntryView shows selected bean lists as readable rows"` passed.
- GREEN: `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormBeanListVersionOptionsIncludeGlobalPublicFallback|TestOrderFormBeanListVersionOptionsUseOnlyPublishedSnapshots' -count=1` passed.
- GREEN/API: `go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsGlobalPublicBeanListVersionsBeforeCustomerSelected -count=1` covers the `/api/order/form` response when no customer is selected.

## Acceptance Checklist
- [x] `/api/order/form` includes `customer_id=0` public published bean-list version rows.
- [x] Frontend no-customer filtering shows public versions instead of an empty list.
- [x] Frontend deduplicates repeated public fallback rows from older API shapes.
- [x] The "选择豆单" button is enabled when public bean-list options are available before choosing a customer.
- [x] Operation manual updated: `OP_MANUAL_ORDER_SALES.md`.
