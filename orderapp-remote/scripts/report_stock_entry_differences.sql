\set ON_ERROR_STOP on
\pset pager off

\if :{?schema}
\else
\echo 'ERROR: pass -v schema=<DB_SCHEMA>; this report never writes inventory.'
\quit
\endif

\echo 'PR-552 read-only stock difference report'
SELECT now() AS generated_at, :'schema' AS schema_name;

\echo '1. Material master balance versus authoritative batch remaining'
WITH batch_balance AS (
  SELECT material_id,
         COALESCE(SUM(remaining_g), 0)::bigint AS batch_g,
         COALESCE(SUM(remaining_units), 0)::bigint AS batch_units
  FROM :"schema".material_batches
  WHERE status <> 'cancelled'
  GROUP BY material_id
)
SELECT m.id AS material_id,
       m.code,
       m.name,
       m.onhand_g AS master_g,
       COALESCE(b.batch_g, 0) AS batch_g,
       m.onhand_g - COALESCE(b.batch_g, 0) AS difference_g,
       m.onhand_units AS master_units,
       COALESCE(b.batch_units, 0) AS batch_units,
       m.onhand_units - COALESCE(b.batch_units, 0) AS difference_units
FROM :"schema".materials m
LEFT JOIN batch_balance b ON b.material_id = m.id
WHERE m.onhand_g <> COALESCE(b.batch_g, 0)
   OR m.onhand_units <> COALESCE(b.batch_units, 0)
ORDER BY m.id;

\echo '2. Material batch remaining versus sum of warehouse locations'
WITH location_balance AS (
  SELECT material_batch_id,
         COALESCE(SUM(qty_g), 0)::bigint AS location_g,
         COALESCE(SUM(qty_units), 0)::bigint AS location_units
  FROM :"schema".material_batch_locations
  GROUP BY material_batch_id
)
SELECT b.id AS material_batch_id,
       b.batch_code,
       b.material_id,
       b.remaining_g,
       COALESCE(l.location_g, 0) AS location_g,
       b.remaining_g - COALESCE(l.location_g, 0) AS difference_g,
       b.remaining_units,
       COALESCE(l.location_units, 0) AS location_units,
       b.remaining_units - COALESCE(l.location_units, 0) AS difference_units
FROM :"schema".material_batches b
LEFT JOIN location_balance l ON l.material_batch_id = b.id
WHERE b.remaining_g <> COALESCE(l.location_g, 0)
   OR b.remaining_units <> COALESCE(l.location_units, 0)
ORDER BY b.id;

\echo '3. Latest ledger after-balance versus current warehouse balance'
WITH actual AS (
  SELECT 'material'::text AS item_type,
         l.material_id AS item_id,
         0::bigint AS spec_g,
         l.warehouse,
         COALESCE(SUM(l.qty_g), 0)::bigint AS qty_g,
         COALESCE(SUM(l.qty_units), 0)::bigint AS qty_units
  FROM :"schema".material_batch_locations l
  GROUP BY l.material_id, l.warehouse
  UNION ALL
  SELECT 'finished_product',
         f.product_id,
         f.spec_g,
         f.warehouse,
         (f.onhand_units * f.spec_g + f.onhand_loose_g)::bigint,
         f.onhand_units::bigint
  FROM :"schema".finished_inventory f
),
latest_ledger AS (
  SELECT item_type,item_id,spec_g,warehouse,qty_after_g,qty_after_units,
         source_doc_type,source_doc_id,created_at
  FROM (
    SELECT l.*,
           row_number() OVER (
             PARTITION BY item_type,item_id,spec_g,warehouse
             ORDER BY created_at DESC,id DESC
           ) AS row_no
    FROM :"schema".stock_ledger_entries l
  ) ranked
  WHERE row_no = 1
)
SELECT COALESCE(a.item_type, l.item_type) AS item_type,
       COALESCE(a.item_id, l.item_id) AS item_id,
       COALESCE(a.spec_g, l.spec_g) AS spec_g,
       COALESCE(a.warehouse, l.warehouse) AS warehouse,
       COALESCE(a.qty_g, 0) AS actual_g,
       COALESCE(l.qty_after_g, 0) AS ledger_after_g,
       COALESCE(a.qty_g, 0) - COALESCE(l.qty_after_g, 0) AS difference_g,
       COALESCE(a.qty_units, 0) AS actual_units,
       COALESCE(l.qty_after_units, 0) AS ledger_after_units,
       COALESCE(a.qty_units, 0) - COALESCE(l.qty_after_units, 0) AS difference_units,
       l.source_doc_type,
       l.source_doc_id,
       l.created_at AS latest_ledger_at
FROM actual a
FULL OUTER JOIN latest_ledger l
  ON l.item_type = a.item_type
 AND l.item_id = a.item_id
 AND l.spec_g = a.spec_g
 AND l.warehouse = a.warehouse
WHERE COALESCE(a.qty_g, 0) <> COALESCE(l.qty_after_g, 0)
   OR COALESCE(a.qty_units, 0) <> COALESCE(l.qty_after_units, 0)
ORDER BY 1,2,3,4;

\echo '4. Submitted unified Stock Entry without an actual ledger row'
\set has_unified_lifecycle false
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = :'schema'
    AND table_name = 'stock_entries'
    AND column_name = 'purpose'
) AS has_unified_lifecycle
\gset
\if :has_unified_lifecycle
SELECT se.id,
       se.entry_no,
       se.purpose,
       se.work_order_id,
       se.submitted_at,
       se.operator
FROM :"schema".stock_entries se
WHERE se.status = 'submitted'
  AND COALESCE(se.legacy, false) = false
  AND NOT EXISTS (
    SELECT 1
    FROM :"schema".stock_ledger_entries l
    WHERE l.source_doc_type = 'stock_entry'
      AND l.source_doc_id = se.id
  )
ORDER BY se.id;
\else
\echo 'Skipped before the unified lifecycle migration: stock_entries.purpose is not present.'
\endif

\echo '5. Historical parallel documents retained for read-only compatibility'
SELECT source_type, document_count
FROM (
  SELECT 'material_receipt' AS source_type, count(*)::bigint AS document_count
  FROM :"schema".material_receipts
  UNION ALL
  SELECT 'material_transfer', count(*)::bigint
  FROM :"schema".material_transfers
  UNION ALL
  SELECT 'finished_product_transfer', count(*)::bigint
  FROM :"schema".finished_product_transfers
) history
ORDER BY source_type;

\echo 'Report complete. No INSERT, UPDATE or DELETE statement was executed.'
