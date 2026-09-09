\set ON_ERROR_STOP on
\if :{?apply}
\else
  \set apply false
\endif
\if :{?expected_manifest}
\else
  \set expected_manifest ''
\endif
\if :{?actor}
\else
  \set actor 'PR-645-material-inventory-cleanup'
\endif
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='30s';
\if :apply
LOCK TABLE :"schema".materials, :"schema".material_batches, :"schema".material_batch_locations,
           :"schema".stock_batches, :"schema".customer_inventory_items IN SHARE ROW EXCLUSIVE MODE;
\endif
CREATE TEMP TABLE cleanup_snapshot ON COMMIT DROP AS
WITH ids AS (
 SELECT id material_id FROM :"schema".materials WHERE onhand_g<>0 OR onhand_units<>0
 UNION SELECT material_id FROM :"schema".material_batches WHERE remaining_g<>0 OR remaining_units<>0
 UNION SELECT material_id FROM :"schema".material_batch_locations
 UNION SELECT item_id FROM :"schema".stock_batches WHERE item_type='material' AND (remaining_g<>0 OR remaining_units<>0)
 UNION SELECT item_id FROM :"schema".customer_inventory_items WHERE item_type='material'
)
SELECT ids.material_id,COALESCE(m.name,'missing material #'||ids.material_id) material_name,
 jsonb_build_object(
  'material',to_jsonb(m),
  'batches',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.id) FROM :"schema".material_batches b WHERE b.material_id=ids.material_id),'[]'::jsonb),
  'locations',COALESCE((SELECT jsonb_agg(to_jsonb(l) ORDER BY l.material_batch_id,l.warehouse) FROM :"schema".material_batch_locations l WHERE l.material_id=ids.material_id),'[]'::jsonb),
  'stock_batches',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM :"schema".stock_batches s WHERE s.item_type='material' AND s.item_id=ids.material_id),'[]'::jsonb),
  'customer_inventory',COALESCE((SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM :"schema".customer_inventory_items c WHERE c.item_type='material' AND c.item_id=ids.material_id),'[]'::jsonb)
 ) old_state
FROM ids LEFT JOIN :"schema".materials m ON m.id=ids.material_id
WHERE m.id IS NULL OR m.deprecated_at IS NOT NULL;
SELECT md5(COALESCE(jsonb_agg(to_jsonb(s) ORDER BY material_id),'[]'::jsonb)::text) AS manifest FROM cleanup_snapshot s \gset
SELECT jsonb_build_object('manifest',:'manifest','count',count(*),'candidates',COALESCE(jsonb_agg(jsonb_build_object(
 'material_id',material_id,'name',material_name,
 'onhand_g',old_state->'material'->'onhand_g','onhand_units',old_state->'material'->'onhand_units',
 'location_count',jsonb_array_length(old_state->'locations'),
 'batch_remaining_g',(SELECT COALESCE(sum((b->>'remaining_g')::bigint),0) FROM jsonb_array_elements(old_state->'batches') b),
 'batch_remaining_units',(SELECT COALESCE(sum((b->>'remaining_units')::bigint),0) FROM jsonb_array_elements(old_state->'batches') b)
 ) ORDER BY material_id),'[]'::jsonb)) FROM cleanup_snapshot;
\if :apply
SELECT :'manifest'=:'expected_manifest' AND :'expected_manifest'<>'' AS manifest_matches \gset
\if :manifest_matches
\else
  DO $$ BEGIN RAISE EXCEPTION 'Inventory preview changed; run a fresh preview before applying.'; END $$;
\endif
-- Keep the complete before-state in the existing operation log. Historical
-- receipt quantities, costs, stock ledgers and business documents are retained.
INSERT INTO :"schema".audit_logs(actor,entity_type,entity_id,action,field,old_value,new_value,meta)
SELECT :'actor','material',material_id,'remove_orphan_inventory','inventory',old_state::text,
       '{"remaining_g":0,"remaining_units":0,"inventory_rows_removed":true}',
       jsonb_build_object('requirement','PR-645','manifest',:'manifest','material_name',material_name,
                          'reason','按用户要求清理物料档案已失效或不存在的库存残留')
FROM cleanup_snapshot;
UPDATE :"schema".materials m SET onhand_g=0,onhand_units=0,updated_at=now()
FROM cleanup_snapshot s WHERE m.id=s.material_id AND (m.onhand_g<>0 OR m.onhand_units<>0);
UPDATE :"schema".material_batches b SET remaining_g=0,remaining_units=0,status='consumed'
FROM cleanup_snapshot s WHERE b.material_id=s.material_id AND (b.remaining_g<>0 OR b.remaining_units<>0);
DELETE FROM :"schema".material_batch_locations l USING cleanup_snapshot s WHERE l.material_id=s.material_id;
UPDATE :"schema".stock_batches b SET remaining_g=0,remaining_units=0
FROM cleanup_snapshot s WHERE b.item_type='material' AND b.item_id=s.material_id AND (b.remaining_g<>0 OR b.remaining_units<>0);
DELETE FROM :"schema".customer_inventory_items c USING cleanup_snapshot s WHERE c.item_type='material' AND c.item_id=s.material_id;
COMMIT;
\else
ROLLBACK;
\endif
