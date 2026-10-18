-- PR-698. Run only in the explicitly selected development schema, inside a transaction.
-- Caller must SET LOCAL search_path and kferp.repair_order_no. Never infer a customer address.
DO $repair$
DECLARE
  target text := current_setting('kferp.repair_order_no');
  old_row orders%ROWTYPE;
  after_row orders%ROWTYPE;
  confirmed jsonb;
  settled_id bigint;
  before_value jsonb;
  after_value jsonb;
  doc_table text;
BEGIN
  SELECT * INTO STRICT old_row FROM orders WHERE order_no=target FOR UPDATE;
  SELECT confirmed_json->'receiver' INTO STRICT confirmed
    FROM sample_work_orders WHERE order_id=old_row.id FOR UPDATE;
  IF old_row.is_void OR old_row.process_status_id IS NOT NULL OR
     EXISTS (SELECT 1 FROM ship_statuses WHERE id=old_row.ship_status_id AND name IN ('已发货','部分发货','已签收')) THEN
    RAISE EXCEPTION 'Repair requires an active, unprocessed, unshipped sample order';
  END IF;
  IF COALESCE(btrim(confirmed->>'name'),'')='' OR
     COALESCE(btrim(confirmed->>'phone'),'')='' OR
     COALESCE(btrim(confirmed->>'address'),'')='' THEN
    RAISE EXCEPTION 'Employee-confirmed recipient snapshot is incomplete';
  END IF;
  before_value := jsonb_build_object('receiver_name',old_row.receiver_name,'receiver_phone',old_row.receiver_phone,
    'receiver_address',old_row.receiver_address,'receiver_company',old_row.receiver_company,
    'pay_status_id',old_row.pay_status_id,'settlement_reason',old_row.settlement_reason);
  UPDATE orders SET
    receiver_name=CASE WHEN COALESCE(btrim(receiver_name),'')='' THEN confirmed->>'name' ELSE receiver_name END,
    receiver_phone=CASE WHEN COALESCE(btrim(receiver_phone),'')='' THEN confirmed->>'phone' ELSE receiver_phone END,
    receiver_address=CASE WHEN COALESCE(btrim(receiver_address),'')='' THEN confirmed->>'address' ELSE receiver_address END,
    receiver_company=CASE WHEN COALESCE(btrim(receiver_company),'')='' THEN COALESCE(confirmed->>'company','') ELSE receiver_company END
    WHERE id=old_row.id;
  IF COALESCE(old_row.total_amount,0)=0 AND COALESCE(old_row.shipping_amount,0)=0 AND COALESCE(old_row.grand_total,0)=0
    AND COALESCE(old_row.outsource_material_fee,0)=0 AND COALESCE(old_row.outsource_roast_fee,0)=0
    AND COALESCE(old_row.outsource_packaging_fee,0)=0 AND COALESCE(old_row.outsource_manual_fee,0)=0
    AND COALESCE(old_row.outsource_tax_fee,0)=0 AND COALESCE(old_row.outsource_other_fee,0)=0
    AND COALESCE(old_row.payment_goods_amount,0)=0 AND COALESCE(old_row.payment_shipping_amount,0)=0
    AND COALESCE(old_row.prepayment_amount,0)=0 AND COALESCE(old_row.payment_voucher_asset_id,0)=0
    AND NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=old_row.id AND unit_price<>0) THEN
    SELECT id INTO settled_id FROM pay_statuses WHERE name IN ('已收款','已付款')
      ORDER BY CASE name WHEN '已收款' THEN 0 ELSE 1 END,id LIMIT 1;
    IF settled_id IS NULL THEN RAISE EXCEPTION 'Missing settled payment status'; END IF;
    UPDATE orders SET settlement_reason='free_sample',pay_status_id=settled_id,payment_method=''
      WHERE id=old_row.id;
  END IF;
  SELECT * INTO STRICT after_row FROM orders WHERE id=old_row.id;
  after_value := jsonb_build_object('receiver_name',after_row.receiver_name,'receiver_phone',after_row.receiver_phone,
    'receiver_address',after_row.receiver_address,'receiver_company',after_row.receiver_company,
    'pay_status_id',after_row.pay_status_id,'settlement_reason',after_row.settlement_reason);
  IF before_value IS DISTINCT FROM after_value THEN
    INSERT INTO order_audit_logs(order_id,actor,field,old_value,new_value)
      VALUES(old_row.id,'Codex/PR-698','sample_receipt_repair',before_value::text,after_value::text);
    INSERT INTO audit_logs(actor,entity_type,entity_id,action,field,old_value,new_value,meta)
      VALUES('Codex/PR-698','order',old_row.id,'repair','sample_receipt_repair',before_value::text,after_value::text,
        jsonb_build_object('requirement','PR-698-SAMPLE-ORDER-FIXES','source','sample_work_orders.confirmed_json.receiver','only_empty_fields',true));
    FOREACH doc_table IN ARRAY ARRAY['sales_order_documents','sales_order_images','delivery_note_documents',
      'combined_sales_order_documents','combined_sales_order_images','combined_delivery_note_documents'] LOOP
      IF to_regclass(format('%I.%I',current_schema(),doc_table)) IS NOT NULL THEN
        IF doc_table LIKE 'combined_%' THEN
          EXECUTE format('UPDATE %I SET is_latest=false WHERE order_ids @> jsonb_build_array($1::bigint)',doc_table) USING old_row.id;
        ELSE
          EXECUTE format('UPDATE %I SET is_latest=false WHERE order_id=$1',doc_table) USING old_row.id;
        END IF;
      END IF;
    END LOOP;
  END IF;
END
$repair$;
