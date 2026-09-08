-- Explicitly authorized repair for PR-642. Run only after the production release.
-- psql ON_ERROR_STOP is required; BACKUP_FILE must be a private local psql path.
BEGIN;
SET LOCAL lock_timeout = '5s';
SELECT id FROM p2rms15pepb5ciz.orders
WHERE order_no IN ('SO-20260908-0002','SO-20260908-0003','SO-20260908-0004','SO-20260908-0005','SO-20260908-0006','SO-20260908-0007','SO-20260908-0008','SO-20260908-0009')
ORDER BY id FOR UPDATE;
DO $check$
BEGIN
 IF (SELECT count(*) FROM p2rms15pepb5ciz.orders WHERE id BETWEEN 42 AND 49 AND customer_id=14 AND order_no='SO-20260908-'||lpad((id-40)::text,4,'0') AND portal_service_code='direct_ship' AND order_type_id IN (3,6))<>8
 OR (SELECT count(*) FROM p2rms15pepb5ciz.orders WHERE order_no BETWEEN 'SO-20260908-0002' AND 'SO-20260908-0009')<>8
 THEN RAISE EXCEPTION 'Expected exactly the eight authorized EFS orders'; END IF;
 IF NOT EXISTS(SELECT 1 FROM p2rms15pepb5ciz.customers WHERE id=14 AND name='EFS咖啡' AND default_order_type_id=3)
 OR NOT EXISTS(SELECT 1 FROM p2rms15pepb5ciz.order_types WHERE id=3 AND name='批发')
 OR NOT EXISTS(SELECT 1 FROM p2rms15pepb5ciz.order_types WHERE id=6 AND name='赠送')
 THEN RAISE EXCEPTION 'Customer or order type precondition changed'; END IF;
END $check$;
SELECT id FROM p2rms15pepb5ciz.order_items WHERE order_id BETWEEN 42 AND 49 ORDER BY id FOR UPDATE;
CREATE TEMP TABLE efs_before ON COMMIT DROP AS
SELECT o.id, o.order_type_id, to_jsonb(o)-'order_type_id' AS stable_order,
 (SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.id),'[]') FROM p2rms15pepb5ciz.order_items i WHERE i.order_id=o.id) AS items
FROM p2rms15pepb5ciz.orders o WHERE o.id BETWEEN 42 AND 49;
-- Export before any mutation. The caller supplies the \copy command here.
-- EFS_BACKUP_COPY
DO $backup$ BEGIN
 IF current_setting('kferp.efs_backup_ready',true) IS DISTINCT FROM 'yes' THEN
 RAISE EXCEPTION 'Private backup must be exported successfully before modification';
 END IF;
END $backup$;
INSERT INTO p2rms15pepb5ciz.audit_logs(actor,entity_type,entity_id,action,field,old_value,new_value,meta)
SELECT 'Codex / Van授权 PR-642','order',id,'update','order_type_id','6 / 赠送','3 / 批发',
 jsonb_build_object('reason','指定八单按EFS默认批发归类，保留代发归属及全部价格快照','requirement','PR-642-EFS-FULFILLMENT-HOME','customer_id',14)
FROM efs_before WHERE order_type_id=6;
UPDATE p2rms15pepb5ciz.orders SET order_type_id=3 WHERE id BETWEEN 42 AND 49 AND order_type_id=6;
DO $check$
BEGIN
 IF EXISTS(SELECT 1 FROM efs_before b JOIN p2rms15pepb5ciz.orders o USING(id)
  WHERE o.order_type_id<>3 OR to_jsonb(o)-'order_type_id' IS DISTINCT FROM b.stable_order
  OR b.items IS DISTINCT FROM (SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.id),'[]') FROM p2rms15pepb5ciz.order_items i WHERE i.order_id=o.id))
 THEN RAISE EXCEPTION 'Historical order data changed unexpectedly'; END IF;
END $check$;
SELECT count(*) AS verified_orders, count(*) FILTER(WHERE order_type_id=6) AS adjusted_orders,
 md5(string_agg(stable_order::text||items::text,'' ORDER BY id)) AS preserved_snapshot_fingerprint FROM efs_before;
COMMIT;
