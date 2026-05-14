# Customer Template Live Orders Design

## Goal

Make customer fulfillment accounts see the same ERP-style order list that ERP operators see, and make customer capability templates behave as live references instead of copied customer snapshots.

## Product Behavior

- A customer profile stores a `capability_template_key`.
- The customer's active capabilities, theme, entry mode, ERP workbench access, and capability config are resolved from the current template every time the portal or ERP customer workbench reads them.
- Editing template `a` immediately changes the behavior of every customer that references template `a`.
- Child templates are created only when an administrator explicitly clicks copy. Copy creates a new template record with a parent key and a new editable key/name.
- A template can be marked inactive. Customers referencing inactive or unknown templates are blocked with a clear invalid-template error until an administrator selects an active template.
- The customer-side ERP fulfillment portal no longer displays the old `overview.direct_ship_orders` table. It reads the ERP order list for the bound customer and shows the same order fields, fees, detail drawer, sales order drawer, and delivery note drawer as the ERP fulfillment workbench.

## Architecture

- `customer_capability_templates` becomes the canonical source for template capabilities and gains `parent_template_key`, `active`, and `sort_order`.
- Built-in templates are seeded at runtime as active root templates. Database rows may override built-in template config and may define child templates.
- Customer capability reads use the active template first. Existing `customer_service_capabilities` remains for legacy data and historical compatibility, but it is no longer authoritative for customers that reference a valid template.
- The customer-side ERP portal uses the same frontend order API helper and document drawers as the internal fulfillment workbench, scoped to the current portal customer returned by `/api/customer-processing/portal/overview`.

## Error Handling

- Unknown or inactive template references return `capability template invalid`.
- Admin customer configuration shows invalid template references and disables save/binding until the admin selects an active template.
- Portal login, `/api/customer-processing/portal/overview`, `/api/customer-processing/portal/options`, and customer-side submit endpoints refuse to continue when the referenced template is invalid.

## Testing

- Unit tests cover template normalization, live template override behavior, inactive template rejection, and child-template shape.
- API/repository tests cover list/save/copy/inactivate template metadata and customer portal capability resolution.
- Frontend tests/source guards cover customer portal order list parity and tree UI markers.
- Operation manuals and requirement/acceptance docs document the changed workflow.
