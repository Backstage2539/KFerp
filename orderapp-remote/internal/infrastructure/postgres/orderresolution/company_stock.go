package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	stockapp "orderapp/internal/application/stock"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
)

type companyBatch struct {
	ID, ProductID, SpecID, VariantID, SpecG int64
	Code, Warehouse, Version                string
}

// A production customer is a demand source, not an ownership declaration.
// Only posted, traceable company batches with matching frozen identities qualify.
func (r Repository) companyStockSources(ctx context.Context, q querier, c *domain.Context, items []map[string]json.RawMessage) error {
	tx, ok := q.(pgx.Tx)
	if !ok {
		return fmt.Errorf("成品分配必须在一致的库存快照内核对")
	}
	for n := range c.Dependencies {
		dep := &c.Dependencies[n]
		if dep.Kind != "production" || dep.Status != "completed" || dep.OwnershipIssue != "" {
			continue
		}
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT DISTINCT b.id,b.item_id,b.bom_spec_id,b.bom_variant_id,b.spec_g,b.batch_code,
 COALESCE((SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.bom_variant_id=b.bom_variant_id AND l.spec_g=b.spec_g AND l.owner_customer_id=0 ORDER BY l.id DESC LIMIT 1),''),md5(to_jsonb(b)::text)
 FROM %[1]s.stock_batches b JOIN %[1]s.work_orders w ON
 (b.source_doc_type='production_run' AND w.running_item_id=b.source_doc_id AND b.source_doc_id>0)
 OR (b.source_doc_type IN('production_operation_event','operation_completion') AND EXISTS(SELECT 1 FROM %[1]s.production_operation_events e WHERE e.id=b.source_doc_id AND e.work_order_id=w.id))
 WHERE w.production_plan_id=$1 AND $2=ANY(string_to_array(replace(w.order_nos,' ',''),','))
 AND b.item_type='finished_product' AND b.owner_customer_id=0 AND b.remaining_units>0
 AND b.item_id=w.product_id AND b.bom_spec_id=w.bom_spec_id AND b.bom_variant_id=w.bom_variant_id AND b.spec_g=w.spec_g ORDER BY b.id`, r.schema), dep.ID, c.OrderNo)
		if err != nil {
			return err
		}
		var batches []companyBatch
		for rows.Next() {
			var b companyBatch
			if err = rows.Scan(&b.ID, &b.ProductID, &b.SpecID, &b.VariantID, &b.SpecG, &b.Code, &b.Warehouse, &b.Version); err != nil {
				rows.Close()
				return err
			}
			batches = append(batches, b)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, b := range batches {
			// Weight/box conversions require an explicit conversion snapshot; do not infer one.
			if b.SpecG != 0 || b.SpecID <= 0 || b.VariantID <= 0 || b.Warehouse == "" {
				continue
			}
			var matched []map[string]json.RawMessage
			for _, i := range items {
				if int64(num(i, "product_id")) == b.ProductID && int64(num(i, "bom_spec_id")) == b.SpecID && int64(num(i, "bom_variant_id")) == b.VariantID {
					matched = append(matched, i)
				}
			}
			if len(matched) != 1 {
				continue
			}
			item := matched[0]
			itemID := int64(num(item, "id"))
			var limit float64
			for _, line := range c.Lines {
				if line.ID == itemID {
					limit = line.Quantity - line.Delivered - line.Closed
				}
			}
			if limit <= 0 || math.Trunc(limit) != limit {
				continue
			}
			options, err := stockpg.NewRepository(r.pool, r.schema).ListTransferOptionsTx(ctx, tx, stockapp.TransferOptionsQuery{Warehouse: b.Warehouse, ItemType: "finished_product", ItemID: b.ProductID, BomSpecID: b.SpecID, Limit: 1000000})
			if err != nil {
				return err
			}
			var physical stockapp.TransferOptionRow
			for _, row := range options.Rows {
				if row.BatchID == b.ID {
					physical = row
					break
				}
			}
			if physical.BatchID == 0 || physical.UnavailableReason != "" || physical.OwnerCustomerID != 0 || physical.BomVariantID != b.VariantID || strings.TrimSpace(str(item, "unit")) != physical.InventoryUnit {
				continue
			}
			uncertain, err := r.companyReservationIdentityIssue(ctx, q, b)
			if err != nil {
				return err
			}
			free := physical.AvailableUnits
			if uncertain {
				continue
			}
			protected, err := r.companySharedDemand(ctx, q, *c, dep.ID, b, physical.InventoryUnit)
			if err != nil {
				return err
			}
			var releases []domain.CompanyStockRelease
			var releasable, originalReserved int64
			for _, d := range c.Dependencies {
				if d.Kind == "stock_reservation" && d.ItemID == itemID && d.Quantity == math.Trunc(d.Quantity) {
					originalReserved += int64(d.Quantity)
					if d.BatchID == b.ID {
						releases = append(releases, domain.CompanyStockRelease{ID: d.ID, Quantity: int64(d.Quantity)})
						releasable += int64(d.Quantity)
					}
				}
			}
			available := max(int64(0), free+releasable-protected)
			if available == 0 {
				continue
			}
			targets, err := r.companyTargets(ctx, q, *c, b, physical.InventoryUnit)
			if err != nil {
				return err
			}
			dep.CompanyStock = append(dep.CompanyStock, domain.CompanyStockSource{Version: hash([]byte(b.Version + "|" + b.Warehouse)), FreeQuantity: free - protected, UnreservedQuantity: max(int64(0), int64(limit)-originalReserved), Releases: releases, BatchID: b.ID, SourceItemID: itemID, Label: fmt.Sprintf("%s · %s · %s · %s", str(item, "item_name"), b.Code, b.Warehouse, physical.InventoryUnit), AvailableQuantity: available, CloseLimit: int64(limit), Targets: targets})
		}
		raw, _ := json.Marshal(dep.CompanyStock)
		c.Version = hash([]byte(c.Version + string(raw)))
	}
	return nil
}

// Ambiguous reservation identities cannot be made available by guessing a batch.
func (r Repository) companyReservationIdentityIssue(ctx context.Context, q querier, b companyBatch) (bool, error) {
	var uncertain bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations WHERE product_id=$1 AND bom_spec_id=$2 AND (allocated_units>0 OR allocated_g>0) AND ((batch_id=0 AND batch_code='') OR ((batch_id=$3 OR batch_code=$4) AND (batch_id<>$3 OR batch_code<>$4 OR bom_variant_id<>$5 OR (warehouse<>'' AND warehouse<>$6) OR allocated_g>0))))`, r.schema), b.ProductID, b.SpecID, b.ID, b.Code, b.VariantID, b.Warehouse).Scan(&uncertain)
	return uncertain, err
}

// Conservatively protect the other demands linked to this shared plan even
// when their finished-stock reservation has not yet been posted.
func (r Repository) companySharedDemand(ctx context.Context, q querier, c domain.Context, planID int64, b companyBatch, unit string) (int64, error) {
	var uncertain bool
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE o.id<>$1 AND NOT o.is_void AND o.resolution_status NOT IN('closed','cancelled') AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.bom_variant_id=$5 AND (i.unit<>$6 OR i.qty<>trunc(i.qty)) AND EXISTS(SELECT 1 FROM %[1]s.production_plan_items p WHERE p.production_plan_id=$2 AND o.order_no=ANY(string_to_array(replace(p.order_nos,' ',''),','))))`, r.schema), c.OrderID, planID, b.ProductID, b.SpecID, b.VariantID, unit).Scan(&uncertain); err != nil {
		return 0, err
	}
	if uncertain {
		return math.MaxInt64 / 4, nil
	}
	var qty float64
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(GREATEST(0,i.qty-i.resolution_closed_qty-GREATEST(
 COALESCE((SELECT sum(a.allocated_units) FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=o.id AND a.product_id=i.product_id AND a.bom_spec_id=i.bom_spec_id),0),
 COALESCE((SELECT sum(d.deducted_units) FROM %[1]s.order_stock_deductions d WHERE d.order_id=o.id AND d.product_id=i.product_id AND d.bom_spec_id=i.bom_spec_id),0)))),0)
 FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE o.id<>$1 AND NOT o.is_void AND o.resolution_status NOT IN('closed','cancelled') AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.bom_variant_id=$5
 AND EXISTS(SELECT 1 FROM %[1]s.production_plan_items p WHERE p.production_plan_id=$2 AND o.order_no=ANY(string_to_array(replace(p.order_nos,' ',''),',')))`, r.schema), c.OrderID, planID, b.ProductID, b.SpecID, b.VariantID).Scan(&qty)
	return int64(math.Ceil(qty)), err
}

func (r Repository) companyTargets(ctx context.Context, q querier, c domain.Context, b companyBatch, unit string) ([]domain.CompanyStockTarget, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT o.id,i.id,o.order_no||' · '||COALESCE((SELECT name FROM %[1]s.customers WHERE id=o.customer_id),'')||' · '||i.item_name||' · '||i.unit,COALESCE(o.customer_id,0),md5((to_jsonb(o)||jsonb_build_object('item',to_jsonb(i)))::text),
 GREATEST(0,i.qty-i.resolution_closed_qty-GREATEST(
 COALESCE((SELECT sum(a.allocated_units) FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=o.id AND a.product_id=i.product_id AND a.bom_spec_id=i.bom_spec_id),0),
 COALESCE((SELECT sum(d.deducted_units) FROM %[1]s.order_stock_deductions d WHERE d.order_id=o.id AND d.product_id=i.product_id AND d.bom_spec_id=i.bom_spec_id),0))) remaining
 FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE o.id<>$1 AND NOT o.is_void AND NOT o.fulfillment_paused AND o.resolution_status NOT IN('closed','cancelled')
 AND COALESCE(o.fulfillment_mode,'')='' AND COALESCE(o.delivery_mode,'')<>'customer_stock' AND COALESCE(NULLIF(o.source_warehouse,''),'finished_goods')=$5
 AND i.product_id=$2 AND i.bom_spec_id=$3 AND i.bom_variant_id=$4 AND i.unit=$6 AND i.qty=trunc(i.qty)
 AND (SELECT count(*) FROM %[1]s.order_items x WHERE x.order_id=o.id AND x.product_id=i.product_id AND x.bom_spec_id=i.bom_spec_id)=1
 AND NOT EXISTS(SELECT 1 FROM %[1]s.production_plan_items p JOIN %[1]s.production_plans pp ON pp.id=p.production_plan_id WHERE pp.status<>'cancelled' AND o.order_no=ANY(string_to_array(replace(p.order_nos,' ',''),',')))
 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a LEFT JOIN %[1]s.stock_batches b ON b.id=a.batch_id AND b.batch_code=a.batch_code AND b.item_type='finished_product' AND b.item_id=a.product_id AND b.bom_spec_id=a.bom_spec_id AND b.bom_variant_id=a.bom_variant_id AND b.spec_g=a.spec_g
 WHERE a.order_id=o.id AND a.product_id=i.product_id AND a.bom_spec_id=i.bom_spec_id AND (a.allocated_units>0 OR a.allocated_g>0)
 AND (b.id IS NULL OR b.owner_customer_id<>0 OR a.bom_variant_id<>i.bom_variant_id OR a.spec_g<>0 OR a.allocated_g<>0 OR (a.warehouse<>'' AND a.warehouse<>$5)))
 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_stock_decisions osd WHERE osd.order_id=o.id AND osd.decision='produce')
 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests rr WHERE rr.order_id=o.id AND rr.status='pending') ORDER BY o.id,i.id`, r.schema), c.OrderID, b.ProductID, b.SpecID, b.VariantID, b.Warehouse, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CompanyStockTarget
	for rows.Next() {
		var target domain.CompanyStockTarget
		var qty float64
		if err = rows.Scan(&target.OrderID, &target.OrderItemID, &target.Label, &target.CustomerID, &target.Version, &qty); err != nil {
			return nil, err
		}
		if qty > 0 && qty == math.Trunc(qty) {
			target.RemainingQuantity = int64(qty)
			out = append(out, target)
		}
	}
	return out, rows.Err()
}

// Lock every affected customer's fulfilment before the common finance and
// order locks. Sorted acquisition avoids reverse source/target deadlocks.
func (r Repository) lockCompanyTargets(ctx context.Context, tx pgx.Tx, orderID int64, cmd domain.Command) error {
	ids := []int64{}
	for _, d := range cmd.Dispositions {
		if d.Kind == "production" && d.Choice == "allocate" {
			for _, s := range d.StockSelections {
				ids = append(ids, s.TargetItemID)
			}
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT COALESCE(customer_id,0) FROM %[1]s.orders WHERE id=$1 OR id IN(SELECT order_id FROM %[1]s.order_items WHERE id=ANY($2::bigint[])) ORDER BY 1`, r.schema), orderID, ids)
	if err != nil {
		return err
	}
	var customers []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		customers = append(customers, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range customers {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("mini-direct-ship:%d", id)); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) applyCompanyAllocations(ctx context.Context, tx pgx.Tx, orderID, requestID int64, cmd domain.Command, actor string) error {
	var selections []domain.StockSelection
	for _, d := range cmd.Dispositions {
		if d.Kind == "production" && d.Choice == "allocate" {
			selections = append(selections, d.StockSelections...)
		}
	}
	targetOrders := map[int64]bool{}
	sort.Slice(selections, func(i, j int) bool { return selections[i].BatchID < selections[j].BatchID })
	for _, s := range selections {
		var b companyBatch
		b.ID = s.BatchID
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT item_id,bom_spec_id,bom_variant_id,spec_g,batch_code FROM %s.stock_batches WHERE id=$1`, r.schema), b.ID).Scan(&b.ProductID, &b.SpecID, &b.VariantID, &b.SpecG, &b.Code); err != nil {
			return err
		}
		if err := infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, b.ProductID, b.SpecID, b.SpecG); err != nil {
			return err
		}
		var targetOrderID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT i.order_id,COALESCE(NULLIF(o.source_warehouse,''),'finished_goods') FROM %[1]s.order_items i JOIN %[1]s.orders o ON o.id=i.order_id WHERE i.id=$1 FOR UPDATE OF o,i`, r.schema), s.TargetItemID).Scan(&targetOrderID, &b.Warehouse); err != nil {
			return err
		}
		targetOrders[targetOrderID] = true
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_units,batch_id,batch_code,allocated_units,warehouse,operator)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$7,$10,$11)`, r.schema), targetOrderID, s.TargetItemID, b.ProductID, b.SpecID, b.VariantID, b.SpecG, s.Quantity, b.ID, b.Code, b.Warehouse, actor); err != nil {
			return err
		}
		if err := infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &targetOrderID, "resolution_stock_allocate", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "source_order_id": orderID, "selection": s}); err != nil {
			return err
		}
	}
	for id := range targetOrders {
		var ready bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT NOT EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=$1 AND GREATEST(0,i.qty-i.resolution_closed_qty) > COALESCE((SELECT sum(a.allocated_units) FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=i.order_id AND a.product_id=i.product_id AND a.bom_spec_id=i.bom_spec_id),0))`, r.schema), id).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			continue
		}
		var statusID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_process_statuses WHERE name='库存待发货' ORDER BY id LIMIT 1`, r.schema)).Scan(&statusID); err != nil {
			return fmt.Errorf("缺少库存待发货状态：%w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE id=$1`, r.schema), id, statusID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_decisions(order_id,decision,operator,updated_at)VALUES($1,'use_batch',$2,now()) ON CONFLICT(order_id) DO UPDATE SET decision=EXCLUDED.decision,operator=EXCLUDED.operator,updated_at=now()`, r.schema), id, actor); err != nil {
			return err
		}
		if err := infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &id, "resolution_stock_ready", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "source_order_id": orderID, "process_status": "库存待发货"}); err != nil {
			return err
		}
	}

	return nil
}

func verifyCompanySelectionIdentity(original, current domain.Context, cmd domain.Command) error {
	find := func(c domain.Context, planID, batchID, targetID int64) (string, string) {
		for _, d := range c.Dependencies {
			if d.Kind != "production" || d.ID != planID {
				continue
			}
			for _, s := range d.CompanyStock {
				if s.BatchID != batchID {
					continue
				}
				for _, t := range s.Targets {
					if t.OrderItemID == targetID {
						return s.Version, t.Version
					}
				}
			}
		}
		return "", ""
	}
	for _, d := range cmd.Dispositions {
		if d.Kind != "production" || d.Choice != "allocate" {
			continue
		}
		for _, s := range d.StockSelections {
			beforeSource, beforeTarget := find(original, d.ID, s.BatchID, s.TargetItemID)
			afterSource, afterTarget := find(current, d.ID, s.BatchID, s.TargetItemID)
			if beforeSource == "" || beforeTarget == "" || beforeSource != afterSource || beforeTarget != afterTarget {
				return fmt.Errorf("%w：成品批次或目标订单已变化，请重新核对分配", ErrStale)
			}
		}
	}
	return nil
}

func (r Repository) lockCompanyStockIdentities(ctx context.Context, tx pgx.Tx, cmd domain.Command) error {
	ids := []int64{}
	for _, d := range cmd.Dispositions {
		if d.Kind == "production" && d.Choice == "allocate" {
			for _, s := range d.StockSelections {
				ids = append(ids, s.BatchID)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	targetItems := []int64{}
	for _, d := range cmd.Dispositions {
		if d.Kind == "production" && d.Choice == "allocate" {
			for _, selected := range d.StockSelections {
				targetItems = append(targetItems, selected.TargetItemID)
			}
		}
	}
	// Sales shipment locks its order before the finished-stock identity. Follow
	// that same order so confirmation and shipment cannot wait on each other.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %[1]s.orders WHERE id IN(SELECT order_id FROM %[1]s.order_items WHERE id=ANY($1::bigint[])) ORDER BY id FOR UPDATE`, r.schema), targetItems); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT item_id,bom_spec_id,spec_g FROM %s.stock_batches WHERE id=ANY($1::bigint[]) ORDER BY 1,2,3`, r.schema), ids)
	if err != nil {
		return err
	}
	var identities [][3]int64
	for rows.Next() {
		var id [3]int64
		if err = rows.Scan(&id[0], &id[1], &id[2]); err != nil {
			rows.Close()
			return err
		}
		identities = append(identities, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range identities {
		if err = infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, id[0], id[1], id[2]); err != nil {
			return err
		}
	}
	return nil
}
