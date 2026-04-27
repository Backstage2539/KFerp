# Material Bean Profile Table Design

## Goal

Move coffee-bean-only fields out of the generic material archive and into a typed child table, so packaging and future material types do not inherit irrelevant columns.

## Design

- Keep `materials` focused on fields common to all material types: code, name, kind, unit, batch number, prices, stock, thresholds, and timestamps.
- Add `material_bean_profiles` as a one-to-one child table keyed by `material_id`.
- Store only coffee bean profile fields in the child table: origin, processing station, variety, process method, grade, altitude, flavor, and bean-list note.
- Return bean profile data inside `Material.bean_profile` in the material API. This keeps the JSON explicitly typed while allowing the frontend to edit coffee bean details when `kind === "bean"`.
- Preserve backward compatibility for data already deployed by migrating any existing `materials.origin`, `materials.flavor`, and related columns into `material_bean_profiles`. The old columns can remain physically present in existing databases, but application code must stop selecting, updating, or depending on them.
- Costing and bean-list generation must read bean details through `material_bean_profiles`, while continuing to read cost from `materials.purchase_price`.

## Testing

- Schema tests assert `material_bean_profiles` exists and the canonical `CREATE TABLE materials` statement does not define bean profile columns.
- Material repository tests assert blank batch numbers still default to today's `YYYYMMDD`.
- Costing repository/source tests assert bean-list metadata comes from `material_bean_profiles`.
- HTTP/API tests continue to verify costing routes and material JSON shape compiles through service boundaries.

