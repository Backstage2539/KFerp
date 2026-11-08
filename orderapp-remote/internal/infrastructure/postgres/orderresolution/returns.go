package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"time"

	domain "orderapp/internal/domain/orderresolution"
)

// Return candidates come from posted shipment facts, never a planned shipment
// or an order's display status. Each quantity retains its original batch cost.
func (r Repository) returnSources(ctx context.Context, q querier, c *domain.Context) error {
	if c.DeliveryMode == "customer_stock" {
		c.Unavailable["action:return"] = "客户货权退回仍待验证，请先核对实际客户库存和批次"
		return nil
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT d.id,b.id,i.id,b.batch_code,i.item_name,i.unit,
 d.deducted_units-COALESCE((SELECT sum(x.quantity) FROM %[1]s.order_resolution_stock_returns x WHERE x.source_kind='shipment' AND x.source_id=d.id AND x.status='posted'),0),
 b.owner_customer_id,md5(jsonb_build_object('deduction',to_jsonb(d),'batch',to_jsonb(b),'item',to_jsonb(i),'ledger',(SELECT jsonb_agg(to_jsonb(l) ORDER BY l.id) FROM %[1]s.stock_ledger_entries l WHERE l.source_doc_type='sales_order_shipment' AND l.source_doc_id=d.order_id AND l.source_batch_code=d.batch_code))::text)
 FROM %[1]s.order_stock_deductions d JOIN %[1]s.orders o ON o.id=d.order_id
 JOIN %[1]s.order_items i ON i.order_id=o.id AND i.product_id=d.product_id AND i.bom_spec_id=d.bom_spec_id
 JOIN %[1]s.stock_batches b ON b.id=d.batch_id AND b.batch_code=d.batch_code AND b.item_type='finished_product' AND b.item_id=d.product_id AND b.bom_spec_id=d.bom_spec_id AND b.bom_variant_id=d.bom_variant_id AND b.spec_g=0
 JOIN %[1]s.production_bom_version_variants v ON v.id=b.bom_variant_id AND v.bom_spec_id=b.bom_spec_id AND v.inventory_unit=i.unit
 WHERE d.order_id=$1 AND d.spec_g=0 AND d.bom_spec_id>0 AND d.deducted_units>0 AND d.deducted_g=0 AND d.source_doc_type='sales_order_shipment_inventory_sync_v2' AND d.source_doc_id=o.id
 AND NOT o.is_void AND ((COALESCE(o.fulfillment_mode,'')='' AND b.owner_customer_id=0) OR (o.fulfillment_mode='stock' AND b.owner_customer_id=o.customer_id))
 AND (SELECT count(*) FROM %[1]s.order_items x WHERE x.order_id=o.id AND x.product_id=i.product_id AND x.bom_spec_id=i.bom_spec_id)=1
 AND EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=o.id AND a.order_item_id=i.id AND a.batch_id=b.id AND a.batch_code=b.batch_code AND a.product_id=b.item_id AND a.bom_spec_id=b.bom_spec_id AND a.bom_variant_id=b.bom_variant_id AND a.spec_g=0 AND a.allocated_g=0 AND a.allocated_units>=d.deducted_units AND a.warehouse<>'')
 AND (SELECT COALESCE(sum(-l.qty_change_units),0) FROM %[1]s.stock_ledger_entries l WHERE l.source_doc_type='sales_order_shipment' AND l.source_doc_id=o.id AND l.source_batch_code=b.batch_code AND l.item_type='finished_product' AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.bom_variant_id=b.bom_variant_id AND l.spec_g=0 AND l.owner_customer_id=b.owner_customer_id AND l.qty_change_units<0 AND l.qty_change_g=0 AND l.qty_after_units=l.qty_before_units+l.qty_change_units
 AND EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=o.id AND a.order_item_id=i.id AND a.batch_id=b.id AND a.warehouse=l.warehouse))
 =(SELECT sum(all_d.deducted_units) FROM %[1]s.order_stock_deductions all_d WHERE all_d.order_id=o.id AND all_d.batch_id=b.id AND all_d.bom_variant_id=b.bom_variant_id)
 ORDER BY d.id`, r.schema), c.OrderID)
	if err != nil {
		return err
	}
	type candidate struct {
		source domain.ReturnSource
		owner  int64
	}
	var candidates []candidate
	for rows.Next() {
		var x candidate
		var code, name string
		x.source.Kind = "shipment"
		if err = rows.Scan(&x.source.ID, &x.source.BatchID, &x.source.ItemID, &code, &name, &x.source.Unit, &x.source.AvailableQuantity, &x.owner, &x.source.Version); err != nil {
			rows.Close()
			return err
		}
		x.source.Label = fmt.Sprintf("%s · 原出库 #%d · %s", name, x.source.ID, code)
		if x.source.AvailableQuantity > 0 {
			candidates = append(candidates, x)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, x := range candidates {
		destinations, err := q.Query(ctx, fmt.Sprintf(`SELECT code,name,COALESCE(customer_id,0) FROM %[1]s.warehouses WHERE active AND kind IN('finished','customer_finished','customer_processing','customer') AND COALESCE(customer_id,0)=$1 AND NOT EXISTS(SELECT 1 FROM %[1]s.finished_inventory f JOIN %[1]s.stock_batches b ON b.id=$2 AND b.item_id=f.product_id AND b.bom_spec_id=f.bom_spec_id AND b.spec_g=f.spec_g WHERE f.warehouse=warehouses.code AND f.owner_customer_id<>$1 AND (f.onhand_units<>0 OR f.onhand_loose_g<>0)) ORDER BY sort_order,code`, r.schema), x.owner, x.source.BatchID)
		if err != nil {
			return err
		}
		for destinations.Next() {
			var d domain.ReturnDestination
			d.Ownership = "company"
			if x.owner > 0 {
				d.Ownership = "customer"
			}
			if err = destinations.Scan(&d.Warehouse, &d.Label, &d.OwnerCustomerID); err != nil {
				destinations.Close()
				return err
			}
			x.source.Destinations = append(x.source.Destinations, d)
		}
		err = destinations.Err()
		destinations.Close()
		if err != nil {
			return err
		}
		if len(x.source.Destinations) > 0 {
			c.ReturnSources = append(c.ReturnSources, x.source)
		}
	}
	if len(c.ReturnSources) == 0 {
		c.Unavailable["action:return"] = "没有可核对的原出库批次，请先核对真实发货、货权和退回仓库"
	}
	raw, err := json.Marshal(c.ReturnSources)
	if err != nil {
		return err
	}
	c.Version = hash([]byte(c.Version + string(raw)))
	return nil
}

func (r Repository) lockReturnIdentities(ctx context.Context, tx pgx.Tx, cmd domain.Command) error {
	ids := []int64{}
	for _, s := range cmd.ReturnSelections {
		ids = append(ids, s.BatchID)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT item_id,bom_spec_id,spec_g FROM %s.stock_batches WHERE id=ANY($1::bigint[]) ORDER BY 1,2,3`, r.schema), ids)
	if err != nil {
		return err
	}
	var identities [][3]int64
	for rows.Next() {
		var x [3]int64
		if err = rows.Scan(&x[0], &x[1], &x[2]); err != nil {
			rows.Close()
			return err
		}
		identities = append(identities, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, x := range identities {
		if err = infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, x[0], x[1], x[2]); err != nil {
			return err
		}
	}
	return nil
}
func returnSourceFor(c domain.Context, s domain.ReturnSelection) *domain.ReturnSource {
	for n := range c.ReturnSources {
		x := &c.ReturnSources[n]
		if x.Kind == s.SourceKind && x.ID == s.SourceID && x.BatchID == s.BatchID {
			return x
		}
	}
	return nil
}
func (r Repository) postReturnStock(ctx context.Context, tx pgx.Tx, orderID, requestID, stepID int64, original domain.Context, cmd domain.Command, plan domain.Plan, confirmation StepCommand, actor string) error {
	if confirmation.Choice != "receive" || strings.TrimSpace(confirmation.Note) == "" || strings.TrimSpace(confirmation.Reference) == "" {
		return fmt.Errorf("请明确确认实际收回，并填写验收凭据和仓库说明")
	}
	if _, err := time.Parse("2006-01-02", confirmation.PostingDate); err != nil {
		return fmt.Errorf("请选择实际退回入库日期")
	}
	if err := r.checkPeriod(ctx, tx, confirmation.PostingDate); err != nil {
		return err
	}
	if err := r.lockReturnIdentities(ctx, tx, cmd); err != nil {
		return err
	}
	if err := r.revalidatePlan(ctx, tx, orderID, requestID, original, cmd, plan); err != nil {
		return err
	}
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return err
	}
	for _, s := range cmd.ReturnSelections {
		source := returnSourceFor(current, s)
		prior := returnSourceFor(original, s)
		if source == nil || prior == nil || source.Version != prior.Version || s.SourceKind != "shipment" {
			return fmt.Errorf("%w：原出库、批次或可退数量已变化，请重新核对", ErrStale)
		}
		var owner int64
		found := false
		for _, d := range source.Destinations {
			if d.Warehouse == s.Warehouse && d.Ownership == s.Ownership {
				owner = d.OwnerCustomerID
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("退回仓库或货权无效")
		}
		var recordID int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_stock_returns(request_id,step_id,order_id,order_item_id,source_kind,source_id,batch_id,product_id,bom_spec_id,bom_variant_id,source_warehouse,target_warehouse,source_owner_customer_id,target_owner_customer_id,quantity,unit_cost,quality,source_version,status,actor,posting_date)
 SELECT $1,$2,$3,$4,$5,$6,b.id,b.item_id,b.bom_spec_id,b.bom_variant_id,(SELECT min(l.warehouse) FROM %[1]s.stock_ledger_entries l WHERE l.source_doc_type='sales_order_shipment' AND l.source_doc_id=$3 AND l.source_batch_code=b.batch_code),$8,b.owner_customer_id,$9,$10,b.unit_cost,$11,$12,'posting',$13,$14::date FROM %[1]s.stock_batches b WHERE b.id=$7 RETURNING id`, r.schema), requestID, stepID, orderID, source.ItemID, s.SourceKind, s.SourceID, s.BatchID, s.Warehouse, owner, s.Quantity, s.Quality, source.Version, actor, confirmation.PostingDate).Scan(&recordID)
		if err != nil {
			return err
		}
		batchID, err := stockpg.NewRepository(r.pool, r.schema).PostOrderReturnTx(ctx, tx, recordID, actor)
		if err != nil {
			return err
		}
		if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_return_stock", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "return_id": recordID, "target_batch_id": batchID, "selection": s, "confirmation": confirmation}); err != nil {
			return err
		}
	}
	return nil
}

// A posted receipt remains a fact while finance completes its separate task.
// Revalidation restores only this request's already-proven physical quantity;
// it never resets money, unrelated stock or another processing request.
func (r Repository) normalizePostedReturns(ctx context.Context, tx pgx.Tx, orderID, requestID int64, original domain.Context, cmd domain.Command, current *domain.Context) error {
	if cmd.Action != "return" {
		return nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT x.source_kind,x.source_id,x.batch_id,x.order_item_id,x.quantity,x.target_warehouse,x.target_owner_customer_id,x.quality,x.source_version,
 EXISTS(SELECT 1 FROM %[1]s.stock_batches b JOIN %[1]s.stock_ledger_entries l ON l.source_doc_type='order_resolution_return' AND l.source_doc_id=x.id AND l.source_batch_code=b.batch_code AND l.item_type='finished_product' AND l.item_id=x.product_id AND l.bom_spec_id=x.bom_spec_id AND l.bom_variant_id=x.bom_variant_id AND l.spec_g=0 AND l.owner_customer_id=x.target_owner_customer_id AND l.warehouse=x.target_warehouse AND l.qty_change_units=x.quantity AND l.qty_change_g=0 AND l.qty_after_units=l.qty_before_units+x.quantity WHERE b.id=x.target_batch_id AND b.batch_code=x.target_batch_code AND b.source_doc_type='order_resolution_return' AND b.source_doc_id=x.id AND b.item_type='finished_product' AND b.item_id=x.product_id AND b.bom_spec_id=x.bom_spec_id AND b.bom_variant_id=x.bom_variant_id AND b.spec_g=0 AND b.owner_customer_id=x.target_owner_customer_id AND b.qty_units=x.quantity AND b.unit_cost=x.unit_cost)
 FROM %[1]s.order_resolution_stock_returns x WHERE x.order_id=$1 AND x.request_id=$2 AND x.status='posted' ORDER BY x.id`, r.schema), orderID, requestID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var s domain.ReturnSelection
		var item, owner int64
		var version string
		var backed bool
		if err = rows.Scan(&s.SourceKind, &s.SourceID, &s.BatchID, &item, &s.Quantity, &s.Warehouse, &owner, &s.Quality, &version, &backed); err != nil {
			return err
		}
		s.Ownership = "company"
		if owner > 0 {
			s.Ownership = "customer"
		}
		selected := false
		for _, choice := range cmd.ReturnSelections {
			if choice == s {
				selected = true
				break
			}
		}
		source := returnSourceFor(original, s)
		if !selected || !backed || source == nil || source.Version != version {
			return fmt.Errorf("%w：已执行退回记录不能改选范围、批次、货权或仓库", ErrStale)
		}
		restored := *source
		restored.AvailableQuantity = s.Quantity
		replaced := false
		for n := range current.ReturnSources {
			if current.ReturnSources[n].Kind == s.SourceKind && current.ReturnSources[n].ID == s.SourceID {
				current.ReturnSources[n] = restored
				replaced = true
				break
			}
		}
		if !replaced {
			current.ReturnSources = append(current.ReturnSources, restored)
		}
		for n := range current.Lines {
			if current.Lines[n].ID == item {
				current.Lines[n].Returned -= float64(s.Quantity)
				if current.Lines[n].Returned < 0 {
					return ErrStale
				}
			}
		}
	}
	if len(current.ReturnSources) > 0 {
		delete(current.Unavailable, "action:return")
	}
	return rows.Err()
}
