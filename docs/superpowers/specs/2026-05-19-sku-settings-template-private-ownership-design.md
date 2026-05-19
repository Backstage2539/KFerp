# SKU Settings Template + Customer Ownership Design

Date: 2026-05-19

## Context

SKU Settings is already the canonical configuration surface for public SKU, customer SKU, product categories, gradient pricing templates, and the inputs used by customer bean lists. The current public usage switches are too shallow:

- `use_public_categories=true` displays public categories as read-only references.
- `use_public_sku=true` displays public SKU as read-only references.
- Customer-specific SKU cannot be placed inside public categories because `products.customer_id` and `product_categories.customer_id` must match.

That ownership check is correct. A customer SKU must not directly write into public category rows. The product gap is that public rows are only references, while the user expectation is to use them as templates: reuse public category structure, gradient templates, SKU names, BOM, and pricing, then change only what the customer needs.

Browser verification on the development environment confirmed the issue with customer `岩师傅`:

- The customer has both public SKU and public category usage enabled.
- The page shows the public category tree as `公共引用`.
- The customer has four customer SKU, all still uncategorized.
- The customer has no customer-owned category rows.
- Dragging customer SKU toward an existing public category does not create a usable customer category.

## Goal

Upgrade SKU Settings from "read-only public references" to a mature template ownership model:

> Public SKU, public categories, public gradient templates, BOM, and price tiers are standard templates. Customers can reference them, derive private copies from them, customize names and pricing, and generate customer bean lists without polluting public templates.

The implementation target is the complete workflow for customer category organization, SKU customization, gradient template customization, bean list generation, and auditability.

## Design Principles

1. Public data remains canonical and protected.
2. Customer data is customer-owned once the customer modifies or places something into it.
3. References do not create data by themselves.
4. Copy-on-write turns public templates into customer-owned rows only when a user action needs ownership.
5. Source relationships are explicit IDs, not inferred from names.
6. UI must distinguish public template, inherited customer version, and fully customer-owned rows.
7. Existing customer data must migrate without creating duplicate names or losing customer edits.

## Ownership Model

### Public Template

Rows with `customer_id=0`:

- Public product categories.
- Public SKU.
- Public gradient templates.
- Public BOM and price tiers.

Public templates are editable only in the public SKU context.

### Customer Reference

Customer switches only control availability:

- `use_public_sku`
- `use_public_categories`
- `use_public_gradient_templates`

References do not create copies. Closing a switch hides unused public templates, but keeps already-derived customer-owned rows.

### Customer-Owned Rows

Rows with `customer_id>0`:

- Customer categories.
- Customer SKU.
- Customer gradient templates.

They may either be created from scratch or derived from public templates. Customer-owned rows can be renamed, reordered, moved, edited, disabled, and used in customer bean lists.

### Source Relationship

Derived rows keep source IDs:

- `product_categories.source_category_id`
- `products.base_product_id`
- `pricing_gradient_templates.source_template_id`

Source IDs are used for deduplication, display, migration, and source-change review.

## Data Model

### product_categories

Add:

- `source_category_id BIGINT NOT NULL DEFAULT 0`
- `template_state TEXT NOT NULL DEFAULT 'customer_owned'`

Allowed states:

- `public_template`: row is public canonical data.
- `derived_from_public`: customer row created from public template.
- `customer_owned`: customer row created from scratch or detached from source.

Rules:

- Public rows always have `customer_id=0`, `source_category_id=0`, `template_state='public_template'`.
- Customer rows derived from public rows have `customer_id>0`, `source_category_id=<public category id>`, `template_state='derived_from_public'`.
- A customer can have at most one active row per `source_category_id`.
- Category parent/child derivation must preserve path ownership: if a public secondary category is derived, its public primary parent must also be derived for the same customer.

### pricing_gradient_templates

Add:

- `customer_id BIGINT NOT NULL DEFAULT 0`
- `source_template_id BIGINT NOT NULL DEFAULT 0`
- `template_state TEXT NOT NULL DEFAULT 'customer_owned'`

Rules:

- Public templates have `customer_id=0`, `template_state='public_template'`.
- Customer templates derived from public templates have `customer_id>0`, `source_template_id=<public template id>`.
- Public context lists public templates.
- Customer context lists customer-owned templates plus public templates enabled through usage.
- Editing a public template from customer context must first derive a customer template.

### products

Keep current ownership fields:

- `customer_id`
- `base_product_id`
- `visibility`
- `custom_type`

Clarify states:

- Public SKU: `customer_id=0`, `visibility='public'`.
- Public SKU reference in customer context: no row is created.
- Customer alias from public SKU: `customer_id>0`, `base_product_id=<public product id>`, `custom_type='public_sku_alias'`.
- Customer roast / blend / green bean / drip bag: customer-owned product rows with explicit `custom_type`.

Rules:

- A customer SKU cannot be assigned to a public category ID.
- If a user puts a public SKU into a customer category, the system first derives a customer SKU row.
- If a user puts a customer SKU into a public category template, the system first derives the customer category path.
- Default deduplication: one active `public_sku_alias` per `customer_id + base_product_id` unless the user explicitly creates a separate custom version.

### customer_sku_public_usage

Keep:

- `use_public_sku`
- `use_public_categories`

Add:

- `use_public_gradient_templates BOOLEAN NOT NULL DEFAULT true`

Semantics:

- These switches control whether public templates are visible and selectable in customer context.
- They do not copy public rows.
- Turning a switch off hides unowned public templates/references, but never deletes customer-owned derived rows.

## API Design

### Derive Public Category Path

`POST /api/product-settings/customer-categories/derive`

Request:

```json
{
  "customer_id": 152,
  "source_category_id": 17
}
```

Behavior:

- Validates source category is active and public.
- Derives the full path needed by the target category:
  - Public primary -> customer primary.
  - Public secondary -> customer secondary under derived primary.
- Reuses existing derived rows for the same customer/source.
- Copies names, positions, and bound gradient template IDs as inherited bindings.
- Writes operation logs for each created category.

Response:

```json
{
  "category": {
    "id": 501,
    "customer_id": 152,
    "source_category_id": 17,
    "template_state": "derived_from_public"
  }
}
```

### Assign Product Category With Derivation

Enhance:

`POST /api/product-settings/products/:id/category`

Request:

```json
{
  "category_id": 17,
  "position": 1,
  "derive_public_category": true
}
```

Behavior:

- If product and category have the same `customer_id`, assign normally.
- If product is customer-owned and category is public:
  - derive the public category path for the product customer;
  - assign product to the derived customer category.
- If product is public and category is customer-owned:
  - derive a customer SKU alias first;
  - assign derived SKU to the customer category.
- If both product and category are public in public context, assign normally.
- Reject cross-customer assignments.

Response includes any derived rows:

```json
{
  "ok": true,
  "product_id": 420,
  "category_id": 501,
  "derived_category_id": 501,
  "derived_product_id": 420
}
```

### Derive Public SKU

`POST /api/product-settings/customer-products/derive`

Request:

```json
{
  "customer_id": 152,
  "base_product_id": 21,
  "name": "岩师傅初晓",
  "category_id": 501,
  "copy_bom": true,
  "copy_price_tiers": true
}
```

Behavior:

- Reuses existing unchanged alias when the action is "use existing customer version".
- Creates a new customer SKU when the user explicitly creates a custom roast/blend/version.
- Preserves product kind-specific fields:
  - roasted: roast level, BOM, yield rate.
  - green bean: green bean type and bound roasted BOM product.
  - drip bag: grams and box count.
- Optionally copies BOM and price tiers.
- Assigns to customer category when provided.
- Writes audit log.

### Derive Gradient Template

`POST /api/product-settings/customer-gradient-templates/derive`

Request:

```json
{
  "customer_id": 152,
  "source_template_id": 2,
  "name": "岩师傅 - 正常磅价模板"
}
```

Behavior:

- Copies active template and tiers.
- Sets `source_template_id`.
- Returns customer template.
- Used when a customer edits a public template or chooses "复制为客户模板".

### Update Customer Public Usage

Enhance:

`POST /api/product-settings/customer-public-usage`

Request:

```json
{
  "customer_id": 152,
  "use_public_sku": true,
  "use_public_categories": true,
  "use_public_gradient_templates": true
}
```

Behavior:

- Writes usage flags only.
- Does not copy or delete SKU/category/template rows.
- Audit log explicitly says public templates were shown/hidden.

## Frontend Design

### SKU Ownership Selector

The top context selector remains:

- Public SKU.
- Fulfillment customer.

Customer context shows three switches:

- `使用公共 SKU 模板`
- `使用公共商品分类模板`
- `使用公共梯度模板`

Switch helper text:

- Opening a switch only shows public templates.
- Closing a switch hides unused public templates.
- Customer-created versions remain.

### Category Tree

Rows show explicit badges:

- `公共模板`: public category shown in customer context.
- `来自公共模板`: customer-derived category.
- `客户自建`: customer-created category.

Actions:

- Public template category:
  - `复制为客户分类`
  - accepts customer SKU drop by deriving category path first
  - does not allow inline rename/delete/template binding
- Derived/customer category:
  - rename
  - reorder
  - add secondary
  - delete
  - bind customer or public gradient template
  - drag customer SKU in/out

Display rule:

- If a customer-derived category exists for a public source category, show the customer-derived category in the main customer tree.
- The matching public template can be hidden or shown as a small "source template" reference, but it must not appear as a duplicate normal category.

### Drag and Drop

Required flows:

1. Customer SKU -> public category template:
   - derive category path;
   - assign SKU to derived category;
   - toast: `已从公共模板创建客户分类，并移动 SKU`.

2. Public SKU -> customer category:
   - derive customer SKU alias;
   - assign alias to category;
   - toast: `已从公共 SKU 创建客户 SKU，并移动到分类`.

3. Public SKU -> public category in customer context:
   - derive both customer SKU and customer category path;
   - assign derived SKU to derived category.

4. Customer SKU -> customer category:
   - normal move.

5. Cross-customer SKU/category:
   - reject with clear message.

### SKU List

Rows show:

- Owner badge: `公共模板`, `客户版本`, `来自公共 SKU`, `客户自建`.
- Product kind: roasted, green bean, drip bag.
- Category path.
- Gradient source:
  - `继承分类模板`
  - `产品级覆盖`
  - `客户模板`

Public SKU rows in customer context are read-only but expose actions:

- `复制为客户 SKU`
- `改名为客户 SKU`
- `放入客户分类`

Customer SKU rows support:

- name
- remark
- category
- product kind fields
- BOM / bound roasted product
- margin override
- deactivate

### Gradient Templates

Customer context lists:

- customer templates;
- public templates if public gradient templates are enabled.

Public template in customer context:

- read-only;
- action `复制为客户模板`.

Category template binding:

- can bind to customer templates;
- can inherit public template when public templates are enabled;
- editing a public template from customer context derives a customer template first.

## Bean List Pricing Rules

Customer bean list generation should resolve product/category/template inputs in this order:

1. Customer SKU row, if a customer version exists for the selected public SKU.
2. Public SKU reference, if public SKU usage is enabled and no customer version exists.
3. Product-level margin override.
4. Customer category template.
5. Public category template inherited by a derived category.
6. Public template only when the customer has public template usage enabled.

The generated customer bean list must clearly store whether each line came from a customer SKU or a public SKU reference.

## Migration Strategy

### Existing Public Rows

Backfill:

- public categories -> `template_state='public_template'`
- public gradient templates -> `template_state='public_template'`
- public SKU remains unchanged

### Existing Customer Categories

For customer categories with the same level/name/path as public categories:

- set `source_category_id` to matching public category;
- set `template_state='derived_from_public'`;
- preserve customer row ID and all existing assignments.

If a customer category has no public match:

- set `template_state='customer_owned'`.

### Existing Customer SKU Aliases

For active `public_sku_alias` rows:

- ensure `base_product_id` points to public SKU.
- keep edited names/remarks.
- hide unchanged alias duplicates when the matching public SKU reference is visible and the alias has no customer-specific category, name, remark, BOM, price, or margin changes.

### Existing Customer Gradient Templates

If a customer-scoped template exists during migration or can be matched by source:

- set `source_template_id`.
- otherwise mark as `customer_owned`.

If current schema has only global templates, migration should first introduce customer scope and keep existing rows public unless explicitly customer-specific.

## Audit Logs

Every write path must create visible operation logs:

- Toggle public SKU/category/gradient template usage.
- Derive category path from public template.
- Derive SKU from public SKU.
- Derive gradient template from public template.
- Assign/move SKU to category.
- Rename category.
- Rename SKU.
- Bind/change category template.
- Edit template tiers.
- Set/clear product margin override.
- Deactivate SKU/category/template.

Audit metadata should include:

- `customer_id`
- `source_category_id`
- `source_product_id`
- `source_template_id`
- `derived_category_id`
- `derived_product_id`
- `derived_template_id`
- old and new category/template/product values where applicable

## Error Handling

- If public template usage is disabled, public templates should not be valid derivation targets from the customer UI.
- If a public source row is inactive, derivation must fail with a clear message.
- If a derived row already exists, reuse it instead of creating duplicates.
- If a source category path is partially derived, complete the missing path without duplicating existing rows.
- If name conflicts occur inside the same customer path, suffix the newly derived row only when the conflict is with a different source.
- Cross-customer assignment must be rejected server-side.

## Testing Plan

### Unit Tests

- Category context filtering:
  - show public templates when enabled;
  - hide public templates when disabled;
  - prefer customer-derived category over matching public template.
- SKU context filtering:
  - prefer customer SKU over public reference when `base_product_id` matches.
- Gradient template resolution:
  - customer template over inherited public template.
- Drag intent mapping:
  - product/category ownership combinations resolve to correct derive/move command.

### API Tests

- Derive public primary category.
- Derive public secondary category and parent path.
- Re-derive same source for same customer reuses existing rows.
- Assign customer SKU to public category derives category then assigns.
- Assign public SKU to customer category derives customer product then assigns.
- Assign public SKU to public category in customer context derives both sides.
- Edit public gradient in customer context derives customer template.
- Closing usage switch hides public references but keeps derived rows.
- Audit logs are written for every create/update/move path.

### Browser Acceptance Tests

Using a fixture equivalent to `岩师傅`:

1. Select customer.
2. Enable public SKU/category/template switches.
3. Drag an uncategorized customer SKU into public category `咖啡豆 / 客户定制`.
4. Verify a customer category appears with badge `来自公共模板`.
5. Verify SKU is no longer uncategorized.
6. Rename the customer category.
7. Verify public category name is unchanged in public SKU context.
8. Copy a public SKU into a customer category.
9. Verify no duplicate unchanged alias rows appear in the main list.
10. Copy a public gradient template, edit a tier, bind it to the customer category.
11. Generate or preview customer bean list and verify customer rows use customer category/template.

## Documentation Updates Required During Implementation

- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
- `OP_MANUAL_INVENTORY_MATERIALS.md`
- `OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/*` mirrors
- New acceptance note under `docs/acceptance/`
- PR/DEV seed rows for the complete SKU Settings ownership model

## Non-Goals

- Do not allow customer rows to write directly into public category/template/product IDs.
- Do not bulk-copy all public SKU/categories when a switch is enabled.
- Do not use names as the primary source relationship.
- Do not create a separate one-off customer SKU setup page; SKU Settings remains the canonical workspace.

## Open Decisions

1. Whether customer-derived categories should be shown in exactly the same visual position as their public source or grouped above public templates. Recommended: same visual position, with customer-derived row replacing the public template row in the main tree.
2. Whether multiple customer versions of the same public SKU should be allowed from the regular copy action. Recommended: default one alias per public SKU; advanced custom versions require an explicit "创建另一个定制版本" action.
3. Whether public template changes should notify customers with derived versions. Recommended: do not auto-update derived customer rows; expose source IDs and "public template changed" state for review.
