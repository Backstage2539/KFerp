package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureCompanyScrapSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_company_scraps(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),plan_id BIGINT NOT NULL,
 batch_id BIGINT NOT NULL REFERENCES %[1]s.stock_batches(id),cost_record_id BIGINT NOT NULL REFERENCES %[1]s.production_batch_costs(id),product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,bom_variant_id BIGINT NOT NULL,
 warehouse TEXT NOT NULL,batch_code TEXT NOT NULL,quantity BIGINT NOT NULL CHECK(quantity>0),unit_cost NUMERIC(20,6) NOT NULL CHECK(unit_cost>=0),cost_cents BIGINT NOT NULL CHECK(cost_cents>=0),
 cost_treatment TEXT NOT NULL CHECK(cost_treatment IN('retain_cost','reclassify_loss')),reason TEXT NOT NULL CHECK(btrim(reason)<>''),posting_date DATE NOT NULL,
 snapshot_json JSONB NOT NULL,status TEXT NOT NULL DEFAULT 'posting' CHECK(status IN('posting','posted')),cost_adjustment_id BIGINT NOT NULL DEFAULT 0,expense_adjustment_id BIGINT NOT NULL DEFAULT 0,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(request_id,batch_id));
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_scrap_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '报损记录不能删除';END IF;
 IF TG_OP='INSERT' THEN
 IF NEW.status<>'posting' OR NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests r WHERE r.id=NEW.request_id AND r.order_id=NEW.order_id AND r.status='pending' AND NOT r.applied AND r.command_json->>'action'='close' AND r.command_json->>'posting_date'=NEW.posting_date::text
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(r.command_json->'dispositions') d CROSS JOIN LATERAL jsonb_array_elements(d->'stock_selections') s WHERE d->>'kind'='production' AND d->>'choice'='scrap' AND (d->>'id')::bigint=NEW.plan_id AND d->>'cost_treatment'=NEW.cost_treatment AND d->>'reference'=NEW.reason AND (s->>'batch_id')::bigint=NEW.batch_id AND (s->>'quantity')::bigint=NEW.quantity AND COALESCE((s->>'target_item_id')::bigint,0)=0)
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps s WHERE s.request_id=r.id AND s.source_id=NEW.plan_id AND s.kind='production' AND s.choice='scrap' AND s.permission='stock.resolve' AND s.status='completed' AND btrim(s.result_json->>'reference')<>'')
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps s WHERE s.request_id=r.id AND s.source_id=NEW.plan_id AND s.kind='production_finance' AND s.choice=NEW.cost_treatment AND s.permission='finance.resolve' AND s.status='completed' AND btrim(s.result_json->>'reference')<>'')) THEN RAISE EXCEPTION '报损须完成原方案的仓库与财务批准';END IF;
 ELSE
 IF OLD.status='posted' OR to_jsonb(NEW)-ARRAY['status','cost_adjustment_id','expense_adjustment_id']<>to_jsonb(OLD)-ARRAY['status','cost_adjustment_id','expense_adjustment_id'] THEN RAISE EXCEPTION '已确认的报损事实不可改写';END IF;
 IF NEW.status='posted' THEN
 IF NOT EXISTS(SELECT 1 FROM %[1]s.stock_ledger_entries l JOIN %[1]s.stock_batches b ON b.id=NEW.batch_id WHERE l.source_doc_type='order_resolution_company_scrap' AND l.source_doc_id=NEW.id AND l.source_batch_code=NEW.batch_code AND l.item_type='finished_product' AND l.item_id=NEW.product_id AND l.bom_spec_id=NEW.bom_spec_id AND l.bom_variant_id=NEW.bom_variant_id AND l.owner_customer_id=0 AND l.warehouse=NEW.warehouse AND l.qty_change_units=-NEW.quantity AND l.qty_after_units=l.qty_before_units-NEW.quantity AND l.qty_change_g=0 AND b.remaining_units=(NEW.snapshot_json->'batch'->>'remaining_units')::bigint-NEW.quantity) THEN RAISE EXCEPTION '报损缺少原批次出库事实';END IF;
 IF NEW.cost_treatment='retain_cost' AND (NEW.cost_adjustment_id<>0 OR NEW.expense_adjustment_id<>0) THEN RAISE EXCEPTION '保留原成本不能重复计费';END IF;
 IF NEW.cost_treatment='reclassify_loss' AND NOT EXISTS(SELECT 1 FROM %[1]s.finance_adjustments c JOIN %[1]s.finance_adjustments e ON e.id=NEW.expense_adjustment_id WHERE c.id=NEW.cost_adjustment_id AND c.type='main_cost' AND e.type='expense' AND c.amount=-NEW.cost_cents/100.0 AND e.amount=NEW.cost_cents/100.0 AND c.month=to_char(NEW.posting_date,'YYYY-MM') AND e.month=c.month) THEN RAISE EXCEPTION '报损成本重分类须成对记账';END IF;
 END IF;END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_scrap_guard ON %[1]s.order_resolution_company_scraps;
 CREATE TRIGGER order_resolution_scrap_guard BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.order_resolution_company_scraps FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_scrap_guard();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_scrap_ledger_allowed(doc JSONB)RETURNS BOOLEAN LANGUAGE sql STABLE AS $proof$
 SELECT EXISTS(SELECT 1 FROM %[1]s.order_resolution_company_scraps x JOIN %[1]s.order_resolution_requests r ON r.id=x.request_id AND r.status='pending' AND NOT r.applied
 JOIN %[1]s.stock_batches b ON b.id=x.batch_id AND b.batch_code=x.batch_code AND b.item_type='finished_product' AND b.item_id=x.product_id AND b.bom_spec_id=x.bom_spec_id AND b.bom_variant_id=x.bom_variant_id AND b.owner_customer_id=0 AND b.spec_g=0 AND b.unit_cost=x.unit_cost
 JOIN %[1]s.finished_inventory inventory ON inventory.product_id=x.product_id AND inventory.bom_spec_id=x.bom_spec_id AND inventory.bom_variant_id=x.bom_variant_id AND inventory.spec_g=0 AND inventory.owner_customer_id=0 AND inventory.warehouse=x.warehouse
 WHERE x.id=(doc->>'source_doc_id')::bigint AND x.status='posting' AND doc->>'source_doc_type'='order_resolution_company_scrap'
 AND doc->>'item_type'='finished_product' AND (doc->>'item_id')::bigint=x.product_id AND (doc->>'bom_spec_id')::bigint=x.bom_spec_id AND (doc->>'bom_variant_id')::bigint=x.bom_variant_id AND (doc->>'owner_customer_id')::bigint=0 AND (doc->>'spec_g')::bigint=0
 AND doc->>'source_batch_code'=x.batch_code AND doc->>'warehouse'=x.warehouse AND (doc->>'qty_change_units')::bigint=-x.quantity AND (doc->>'qty_change_g')::bigint=0 AND (doc->>'qty_after_units')::bigint=(doc->>'qty_before_units')::bigint-x.quantity
 AND (doc->>'qty_after_units')::bigint=inventory.onhand_units AND b.remaining_units=(x.snapshot_json->'batch'->>'remaining_units')::bigint-x.quantity
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps step WHERE step.request_id=x.request_id AND step.source_id=x.plan_id AND step.kind='production' AND step.choice='scrap' AND step.status='completed')
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps step WHERE step.request_id=x.request_id AND step.source_id=x.plan_id AND step.kind='production_finance' AND step.choice=x.cost_treatment AND step.status='completed')
 AND NOT EXISTS(SELECT 1 FROM %[1]s.stock_ledger_entries previous WHERE previous.source_doc_type='order_resolution_company_scrap' AND previous.source_doc_id=x.id));
 $proof$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_scrap_ledger_insert_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN IF NEW.source_doc_type='order_resolution_company_scrap' AND NOT %[1]s.order_resolution_scrap_ledger_allowed(to_jsonb(NEW)) THEN RAISE EXCEPTION '报损库存流水缺少已批准的原批次处置';END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_scrap_ledger_insert_guard ON %[1]s.stock_ledger_entries;
 CREATE TRIGGER order_resolution_scrap_ledger_insert_guard BEFORE INSERT ON %[1]s.stock_ledger_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_scrap_ledger_insert_guard();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_scrap_history_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE doc JSONB:=to_jsonb(OLD);BEGIN
 IF TG_OP='UPDATE' AND doc=to_jsonb(NEW) THEN RETURN NEW;END IF;
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_company_scraps x WHERE x.status='posted' AND
 ((TG_TABLE_NAME='stock_batches' AND x.batch_id=(doc->>'id')::bigint AND (TG_OP='DELETE' OR to_jsonb(NEW)-ARRAY['remaining_units','remaining_g','quality_status','operator','updated_at']<>doc-ARRAY['remaining_units','remaining_g','quality_status','operator','updated_at']))
 OR (TG_TABLE_NAME='production_batch_costs' AND x.cost_record_id=(doc->>'id')::bigint)
 OR (TG_TABLE_NAME='finance_adjustments' AND (doc->>'id')::bigint IN(x.cost_adjustment_id,x.expense_adjustment_id))
 OR (TG_TABLE_NAME='stock_ledger_entries' AND doc->>'source_doc_type'='order_resolution_company_scrap' AND x.id=(doc->>'source_doc_id')::bigint))) THEN RAISE EXCEPTION '报损原成本、库存流水和调整凭据须保留，请另办更正';END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;END $fn$;
 `, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"production_batch_costs", "finance_adjustments", "stock_ledger_entries", "stock_batches"} {
		var exists bool
		if err = pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS order_resolution_scrap_history_guard ON %[1]s.%[2]s;CREATE TRIGGER order_resolution_scrap_history_guard BEFORE UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_scrap_history_guard()`, schema, table)); err != nil {
			return err
		}
	}
	return nil
}
