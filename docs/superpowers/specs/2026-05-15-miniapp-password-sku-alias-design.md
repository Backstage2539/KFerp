# Miniapp Password Masking And Customer SKU Alias Design

## Goal

Fix two customer-facing miniapp issues:

- The ERP password login field must mask typed characters in WeChat mini program builds.
- If a customer has a customer-only SKU whose `base_product_id` points to a public SKU, the miniapp customer service product list must show the customer SKU instead of the base public SKU.

## Design

### Password Masking

The miniapp login page already uses an ERP password login form. For uni-app / WeChat mini program compatibility, the password input should use the native boolean `password` prop instead of relying only on `type="password"`.

The source guard test will assert that `login.vue` contains the `password` input prop and does not use a plain text password field.

### Customer SKU Alias Replacement

This is a general customer SKU replacement rule, not a hard-coded 岩师傅 rule.

When loading products visible to a customer, public SKUs are normally visible and customer-only SKUs owned by that customer are also visible. The replacement rule adds one exclusion: if the current customer has an active `customer_only` product with `base_product_id = public_product.id`, the public base SKU is hidden for that customer. The customer SKU remains visible.

For 岩师傅, current production data has customer `152` product `351` `兰卡拼配`, `base_product_id=146`, replacing public product `146` `曲奇拼配`. The same rule will support future customers without code changes.

### Scope

This change applies to customer service product selectors backed by `Repository.listProducts`, including product order, direct ship, and processing service pages. It does not publish or synthesize `mall_products`; mall visibility stays controlled by existing mall product publishing.

## Testing

- Miniapp source guard: password field uses the native password mask prop.
- PostgreSQL repository test: customer A with an alias SKU sees the alias SKU and does not see its base public SKU; unrelated public SKUs and that customer's own SKU remain visible.
- Existing mall tests remain unchanged because mall publishing is a separate product surface.

## User Manual

Update the customer portal operation manual to state that customer-specific replacement SKUs hide the base public SKU in miniapp service product selectors.
