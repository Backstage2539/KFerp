# Customer SKU BOM And Bean List Design

## Goal

Close the customer-specific product loop: custom SKUs must be visible, their BOM must be maintainable, and customer bean lists must only contain public SKUs plus that customer's own SKUs.

## Confirmed Business Rule

- Official bean lists can only include public SKUs.
- Customer bean lists are created for one selected customer.
- Customer bean lists can include public SKUs and SKUs whose `customer_id` equals the selected customer.
- A customer's SKU must never be shared into another customer's bean list. If customer A needs a SKU similar to customer B's SKU, create a copied SKU for customer A.
- Inventory, BOM, cost, and ordering remain keyed by each SKU's own `product_id`.

## Scope

1. BOM maintenance:
   - Existing BOM APIs already support list/detail/item save/item delete/version save/activate.
   - Add whole-BOM delete for the current product.
   - Make item save create the current BOM row if it does not exist yet.
   - Improve the Vue BOM page so users can clearly create/sync a BOM, add/delete items, delete the current BOM, and select products/materials with searchable controls.

2. Product settings:
   - Add a dedicated customer SKU list under the customer SKU creation form.
   - Show customer, SKU name, base product, custom type, roast level, BOM item count, and action links.
   - Support filtering the list by customer.
   - Add a "维护 BOM" action that opens the BOM page for that SKU.

3. Customer bean lists:
   - Add customer selection for customer-scope bean list generation.
   - Official product picker must only show public products.
   - Customer product picker must show public products plus SKUs owned by the selected customer.
   - Backend publish/draft APIs must enforce the same product-scope rule from `content.groups[].items[].productId`.
   - Customer portal already reads `owner_type='customer'` with `owner_key=<customer_id>` and falls back to official publications. The ERP publishing API should use that owner shape.

## Data Model

- Reuse `products.customer_id`, `products.base_product_id`, `products.visibility`, and `products.custom_type`.
- Add no new tables for this iteration.
- Add computed response fields:
  - `bom_item_count` on product settings product rows.
  - Product costing results carry `customer_id`, `base_product_id`, `visibility`, and `custom_type` so the frontend can filter bean-list candidates.

## API Changes

- `DELETE /api/bom/:product_id`
  - Deletes current `product_bom` and `product_bom_items` for the product.
  - Leaves BOM versions intact so a saved version can still be reactivated.

- Existing `/api/product-settings`
  - Adds `bom_item_count` to each product row.

- Existing bean list APIs:
  - `GET /api/costing/bean-list/publications?scope=customer&customer_id=<id>&list_type=...`
  - `POST /api/costing/bean-list/publications` and `/drafts` accept `scope:"customer"` and `customer_id`.
  - For `scope:"customer"`, owner is stored as `owner_type='customer'`, `owner_key='<customer_id>'`.
  - API rejects content containing another customer's SKU.

## UI

- Product settings remains the primary place to create and inspect customer SKUs.
- BOM remains the primary place to edit BOM details.
- Costing/bean list drawer gains a customer selector when the scope is customer.

## Acceptance

- A user can see all customer SKUs and filter by customer.
- A user can jump from a customer SKU to its BOM and maintain materials.
- A user can delete a current BOM and see item count return to zero.
- Official bean list candidates exclude customer-only SKUs.
- Customer A bean list candidates include public SKUs and customer A SKUs, but not customer B SKUs.
- Backend rejects a customer bean list payload containing a different customer's SKU.
