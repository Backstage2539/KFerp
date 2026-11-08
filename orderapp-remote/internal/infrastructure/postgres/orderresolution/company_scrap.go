package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
)

type CompanyScrapRecord struct {
	ID                  int64  `json:"id"`
	BatchCode           string `json:"batch_code"`
	Quantity            int64  `json:"quantity"`
	Unit                string `json:"unit"`
	CostCents           int64  `json:"cost_cents"`
	CostTreatment       string `json:"cost_treatment"`
	PostingDate         string `json:"posting_date"`
	Reason              string `json:"reason"`
	CostAdjustmentID    int64  `json:"cost_adjustment_id"`
	ExpenseAdjustmentID int64  `json:"expense_adjustment_id"`
}

func (r Repository) companyScrapRecords(ctx context.Context, q querier, requestID int64) ([]CompanyScrapRecord, error) {
	out := []CompanyScrapRecord{}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT id,batch_code,quantity,COALESCE(snapshot_json->>'unit',''),cost_cents,cost_treatment,to_char(posting_date,'YYYY-MM-DD'),reason,cost_adjustment_id,expense_adjustment_id FROM %s.order_resolution_company_scraps WHERE request_id=$1 AND status='posted' ORDER BY id`, r.schema), requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item CompanyScrapRecord
		if err = rows.Scan(&item.ID, &item.BatchCode, &item.Quantity, &item.Unit, &item.CostCents, &item.CostTreatment, &item.PostingDate, &item.Reason, &item.CostAdjustmentID, &item.ExpenseAdjustmentID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Only a posted production cost can be reclassified. The original cost row and
// original period remain intact; inventory loss must not become a second charge.
func (r Repository) companyScrapCostSource(ctx context.Context, q querier, s *domain.CompanyStockSource) error {
	var version string
	var proven bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT round(b.unit_cost*1000000)::bigint,c.id,to_char(c.created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM'),to_char(GREATEST(c.created_at,b.created_at) AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD'),md5(to_jsonb(c)::text),
 b.spec_g=0 AND b.qty_g=0 AND b.remaining_g=0 AND b.qty_units>0 AND b.unit_cost>=0 AND c.total_cost>=0
 AND (SELECT count(*) FROM %[1]s.work_orders other WHERE other.running_item_id=w.running_item_id)=1
 AND round((SELECT COALESCE(sum(CASE WHEN related.qty_g>0 THEN related.qty_g/1000.0*related.unit_cost ELSE related.qty_units*related.unit_cost END),0) FROM %[1]s.stock_batches related WHERE (related.source_doc_type='production_run' AND related.source_doc_id=w.running_item_id) OR (related.source_doc_type IN('production_operation_event','operation_completion') AND related.source_doc_id IN(SELECT id FROM %[1]s.production_operation_events WHERE work_order_id=w.id)))*100)<=round(c.total_cost*100)
 FROM %[1]s.stock_batches b JOIN %[1]s.work_orders w ON (b.source_doc_type='production_run' AND b.source_doc_id=w.running_item_id AND w.running_item_id>0) OR (b.source_doc_type IN('production_operation_event','operation_completion') AND b.source_doc_id IN(SELECT id FROM %[1]s.production_operation_events WHERE work_order_id=w.id))
 JOIN %[1]s.production_batch_costs c ON c.running_item_id=w.running_item_id WHERE b.id=$1`, r.schema), s.BatchID).Scan(&s.UnitCostMicros, &s.CostRecordID, &s.CostMonth, &s.CostDate, &version, &proven)
	if err == pgx.ErrNoRows {
		s.ScrapIssue = "原生产成本尚未形成可核对记录，请先完成批次成本核对"
		return nil
	}
	if err != nil {
		return err
	}
	if !proven {
		s.ScrapIssue = "产出批次、计价单位或成本合计与原生产成本不一致，请先核对"
	}
	s.Version = hash([]byte(s.Version + version + s.ScrapIssue))
	return nil
}

func (r Repository) applyCompanyScraps(ctx context.Context, tx pgx.Tx, orderID, requestID int64, cmd domain.Command, actor string) error {
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "scrap" {
			continue
		}
		for _, s := range d.StockSelections {
			var id int64
			err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_company_scraps(request_id,order_id,plan_id,batch_id,cost_record_id,product_id,bom_spec_id,bom_variant_id,warehouse,batch_code,quantity,unit_cost,cost_cents,cost_treatment,reason,posting_date,snapshot_json,actor)
 SELECT $1,$2,$3,b.id,c.id,b.item_id,b.bom_spec_id,b.bom_variant_id,(SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.bom_variant_id=b.bom_variant_id AND l.spec_g=0 AND l.owner_customer_id=0 AND l.source_batch_code=b.batch_code ORDER BY l.id DESC LIMIT 1),b.batch_code,$5::bigint,b.unit_cost,round($5::bigint*b.unit_cost*100),$6,$7,$8::date,jsonb_build_object('batch',to_jsonb(b),'cost',to_jsonb(c),'unit',(SELECT v.inventory_unit FROM %[1]s.production_bom_version_variants v WHERE v.id=b.bom_variant_id)),$9
 FROM %[1]s.stock_batches b JOIN %[1]s.work_orders w ON w.production_plan_id=$3 AND ((b.source_doc_type='production_run' AND b.source_doc_id=w.running_item_id) OR (b.source_doc_type IN('production_operation_event','operation_completion') AND b.source_doc_id IN(SELECT id FROM %[1]s.production_operation_events WHERE work_order_id=w.id))) JOIN %[1]s.production_batch_costs c ON c.running_item_id=w.running_item_id WHERE b.id=$4 RETURNING id`, r.schema), requestID, orderID, d.ID, s.BatchID, s.Quantity, d.CostTreatment, d.Reference, cmd.PostingDate, actor).Scan(&id)
			if err != nil {
				return err
			}
			if err = stockpg.NewRepository(r.pool, r.schema).PostResolutionCompanyScrapTx(ctx, tx, id, actor); err != nil {
				return err
			}
			if d.CostTreatment == "reclassify_loss" {
				var costID, expenseID int64
				for _, entry := range []struct {
					kind string
					sign int
				}{{"main_cost", -1}, {"expense", 1}} {
					var adjustmentID int64
					err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.finance_adjustments(month,type,amount,reason,note,actor) SELECT to_char(posting_date,'YYYY-MM'),$2,cost_cents/100.0*$3,'订单报损成本重分类 #'||id::text,'保留原生产成本；原批次 '||batch_code||'；'||reason,$4 FROM %[1]s.order_resolution_company_scraps WHERE id=$1 RETURNING id`, r.schema), id, entry.kind, entry.sign, actor).Scan(&adjustmentID)
					if err != nil {
						return err
					}
					if entry.sign < 0 {
						costID = adjustmentID
					} else {
						expenseID = adjustmentID
					}
				}
				if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_company_scraps SET cost_adjustment_id=$2,expense_adjustment_id=$3 WHERE id=$1`, r.schema), id, costID, expenseID); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_company_scraps SET status='posted' WHERE id=$1`, r.schema), id); err != nil {
				return err
			}
			if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_company_scrap", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "scrap_id": id, "batch_id": s.BatchID, "quantity": s.Quantity, "cost_treatment": d.CostTreatment, "reason": d.Reference}); err != nil {
				return err
			}
		}
	}
	return nil
}
