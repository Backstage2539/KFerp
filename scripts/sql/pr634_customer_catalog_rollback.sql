-- psql -v schema=orderapp -v migration_audit_id=<migrate_customer_catalog audit id> -f this-file
-- Run against the same environment and inspect the preview/audit evidence first.
BEGIN;
SELECT set_config('pr634.schema', :'schema', true);
SELECT set_config('pr634.audit_id', :'migration_audit_id', true);
DO $rollback$
DECLARE s text := current_setting('pr634.schema'); a bigint := current_setting('pr634.audit_id')::bigint;
 m jsonb; mismatch bigint;
BEGIN
 EXECUTE format('SELECT id FROM %I.customers ORDER BY id FOR UPDATE',s);
 EXECUTE format('LOCK TABLE %I.product_customer_references, %I.customer_product_catalog_nodes IN SHARE ROW EXCLUSIVE MODE',s,s);
 EXECUTE format('SELECT meta FROM %I.audit_logs WHERE id=$1 AND action=''migrate_customer_catalog'' FOR UPDATE',s) INTO m USING a;
 IF m IS NULL OR m->'before_references' IS NULL THEN RAISE EXCEPTION 'Migration audit with rollback evidence not found'; END IF;
 EXECUTE format('SELECT count(*) FROM jsonb_to_recordset($1->''after_references'') e(id bigint,customer_catalog_node_id bigint,catalog_sort_order int) LEFT JOIN %I.product_customer_references r ON r.id=e.id WHERE r.id IS NULL OR r.customer_catalog_node_id IS DISTINCT FROM e.customer_catalog_node_id OR r.catalog_sort_order IS DISTINCT FROM e.catalog_sort_order',s) INTO mismatch USING m;
 IF mismatch>0 THEN RAISE EXCEPTION 'References have changed after migration: %; rollback aborted',mismatch; END IF;
 EXECUTE format('SELECT count(*) FROM jsonb_array_elements($1->''created_nodes'') e LEFT JOIN %I.customer_product_catalog_nodes n ON n.id=(e->>''id'')::bigint WHERE to_jsonb(n) IS DISTINCT FROM e',s) INTO mismatch USING m;
 IF mismatch>0 THEN RAISE EXCEPTION 'Customer categories have changed: %; rollback aborted',mismatch; END IF;
 EXECUTE format('SELECT count(*) FROM %I.product_customer_references r WHERE r.customer_catalog_node_id IN (SELECT (e->>''id'')::bigint FROM jsonb_array_elements($1->''created_nodes'') e) AND r.id NOT IN (SELECT (e->>''id'')::bigint FROM jsonb_array_elements($1->''after_references'') e)',s) INTO mismatch USING m;
 IF mismatch>0 THEN RAISE EXCEPTION 'New references use migration categories: %; rollback aborted',mismatch; END IF;
 EXECUTE format('UPDATE %I.product_customer_references r SET customer_catalog_node_id=e.customer_catalog_node_id,catalog_sort_order=e.catalog_sort_order FROM jsonb_to_recordset($1->''before_references'') e(id bigint,customer_catalog_node_id bigint,catalog_sort_order int) WHERE r.id=e.id',s) USING m;
 EXECUTE format('DELETE FROM %I.customer_product_catalog_nodes WHERE id IN (SELECT (e->>''id'')::bigint FROM jsonb_array_elements($1->''created_nodes'') e)',s) USING m;
 EXECUTE format('INSERT INTO %I.audit_logs(actor,entity_type,action,meta) VALUES(current_user,''customer_product_catalog'',''rollback_customer_catalog'',jsonb_build_object(''migration_audit_id'',$1::bigint))',s) USING a;
END $rollback$;
COMMIT;
