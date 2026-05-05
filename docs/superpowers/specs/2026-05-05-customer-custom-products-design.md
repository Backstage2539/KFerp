# Customer Custom Products Design

## Goal

Support repeatable customer-specific coffee products that can be ordered repeatedly, stocked as finished goods, planned for production, and costed independently while consuming the same shared raw materials as public products.

## Design

Customer customization is modeled as a formal product SKU. Public products remain shared SKUs. A customer-specific product gets its own `products.id`, links back to the customer and optional base product, and carries its own roast level, BOM, prices, inventory, and costing identity.

This keeps the existing production, inventory, BOM, costing, and sales flows centered on `product_id`. Raw material stock stays shared because BOM items still reference the same `materials` rows. Finished goods stay separate because finished inventory already keys by `product_id + spec_g + warehouse`.

## Data Model

Add product metadata:

- `customer_id`: `0` for public SKUs, customer ID for customer-only SKUs.
- `base_product_id`: `0` or the public product copied from.
- `visibility`: `public` or `customer_only`.
- `custom_type`: empty, `custom_blend`, or `custom_roast`.

## Product Settings

Product settings adds a creation flow for customer-specific SKUs:

- Select customer.
- Select base product.
- Enter custom product name.
- Select custom type.
- Select roast level.
- Choose whether to copy BOM and price tiers from the base product.

The API creates the product, copies selected price/BOM data, initializes BOM yield from roast level, and returns the new product for immediate use.

## Sales Order Entry

Order entry should show:

- Public products for every customer.
- Products where `product.customer_id` equals the selected customer.
- No products owned by other customers.

If the customer changes, existing selected product lines are not silently removed; users can still save existing valid lines, but the dropdown will only offer products valid for the current customer.

## Production, Inventory, Costing

No new variant key is introduced. Customer-specific products use their own `product_id`, so existing flows naturally separate:

- Production plan groups by customer SKU.
- Finished inventory separates customer SKU from public SKU.
- Cost calculation treats customer SKU as a product and reads its own BOM.
- Material demand aggregates shared `materials` across all product SKUs.

## Acceptance

- A customer-only SKU can be created from a public product and copies BOM/price data when requested.
- `/api/order/form?customer_id=<id>` only returns public products plus that customer's SKUs.
- `/api/order/form` still returns all active public and customer products with ownership metadata for Vue filtering.
- Product settings response exposes customer/base/visibility fields.
- Existing BOM, production, inventory, and costing continue to work by `product_id`.
