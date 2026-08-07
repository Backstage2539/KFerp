package customerportal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"

	"github.com/jackc/pgx/v5"
)

type processingAvailabilitySource struct {
	WarehouseCode    string
	WarehouseKind    string
	OwnerType        string
	SourceCustomerID int64
	AvailableG       int64
	AvailableUnits   int64
}

type preparedProcessingRequest struct {
	Preview      customerportalapp.ProcessingRequestPreview
	Resolved     []postgresproduction.CustomerProcessingResolvedItem
	Warehouse    string
	SourcesByKey map[string][]processingAvailabilitySource
}

var processingTargetWeightPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(kg|g|lb|克|千克|公斤|磅)`)

func authoritativeProcessingTargetSpecG(netContentQty float64, netContentUnit, label string) int64 {
	toGrams := func(qty float64, unit string) int64 {
		factor := 0.0
		switch strings.ToLower(strings.TrimSpace(unit)) {
		case "g", "克":
			factor = 1
		case "kg", "千克", "公斤":
			factor = 1000
		case "lb", "磅":
			factor = 453.59237
		}
		if qty <= 0 || factor <= 0 {
			return 0
		}
		return int64(math.Round(qty * factor))
	}
	if grams := toGrams(netContentQty, netContentUnit); grams > 0 {
		return grams
	}
	match := processingTargetWeightPattern.FindStringSubmatch(strings.TrimSpace(label))
	if len(match) != 3 {
		return 0
	}
	qty, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	return toGrams(qty, match[2])
}

func validateProcessingTargetSpecG(productID, requestedSpecG, authoritativeSpecG int64) error {
	if authoritativeSpecG <= 0 {
		return fmt.Errorf("target product has no authoritative specification")
	}
	if requestedSpecG != authoritativeSpecG {
		return fmt.Errorf("target specification mismatch: SKU %d is %dg, got %dg", productID, authoritativeSpecG, requestedSpecG)
	}
	return nil
}

func isCustomerProcessingFinishedWarehouseKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "finished", "customer_processing", "customer_finished", "customer":
		return true
	default:
		return false
	}
}

func processingNeedKey(componentType string, materialID, componentSpecG int64) string {
	return fmt.Sprintf("%s:%d:%d", firstNonEmpty(strings.TrimSpace(componentType), "material"), materialID, componentSpecG)
}

func (r Repository) PreviewProcessingRequest(ctx context.Context, cmd customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequestPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return customerportalapp.ProcessingRequestPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	prepared, err := r.prepareProcessingRequestTx(ctx, tx, cmd, false)
	if err != nil {
		return customerportalapp.ProcessingRequestPreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.ProcessingRequestPreview{}, err
	}
	return prepared.Preview, nil
}

func (r Repository) FilterProcessingCatalogProductIDs(ctx context.Context, customerID int64, productIDs []int64) ([]int64, error) {
	if customerID <= 0 || len(productIDs) == 0 {
		return []int64{}, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	allowed := make([]int64, 0, len(productIDs))
	for _, productID := range productIDs {
		if err := r.ensureProcessingTargetProductTx(ctx, tx, customerID, productID); err != nil {
			if strings.Contains(err.Error(), "target product unavailable") {
				continue
			}
			return nil, err
		}
		configured, err := postgresproduction.HasUsableCustomerProcessingBomTx(ctx, tx, r.schema, productID)
		if err != nil {
			return nil, err
		}
		if configured {
			allowed = append(allowed, productID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return allowed, nil
}

func (r Repository) createProcessingRequestV2(ctx context.Context, cmd customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	if cmd.CustomerID <= 0 {
		return customerportalapp.ProcessingRequest{}, fmt.Errorf("customer required")
	}
	if len(cmd.Items) == 0 {
		return customerportalapp.ProcessingRequest{}, fmt.Errorf("items required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	prepared, err := r.prepareProcessingRequestTx(ctx, tx, cmd, true)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if !prepared.Preview.CanSubmit {
		return customerportalapp.ProcessingRequest{}, &customerportalapp.ProcessingMaterialsUnavailableError{Preview: prepared.Preview}
	}

	first := prepared.Resolved[0]
	var requestID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_job_requests(
			customer_id,input_material_id,input_qty_g,target_product_id,target_spec_g,target_qty,
			status,note,created_by_mini_user_id
		)
		VALUES($1,0,0,$2,$3,$4,'awaiting_schedule',$5,$6)
		RETURNING id
	`, r.schema), cmd.CustomerID, first.ProductID, first.SpecG, first.Qty, strings.TrimSpace(cmd.Note), cmd.CreatedByMiniUserID).Scan(&requestID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	requestNo := fmt.Sprintf("PJ-%010d", requestID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.processing_job_requests SET request_no=$2 WHERE id=$1`, r.schema), requestID, requestNo); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}

	mutableSources := make(map[string][]processingAvailabilitySource, len(prepared.SourcesByKey))
	for key, rows := range prepared.SourcesByKey {
		mutableSources[key] = append([]processingAvailabilitySource(nil), rows...)
	}
	reservationCount := 0
	for _, item := range prepared.Resolved {
		var requestItemID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.processing_job_request_items(
				request_id,line_no,product_id,parent_product_id,product_name,spec_g,target_qty,need_g,
				target_warehouse,bom_version_id,bom_version_no,bom_source_product_id,bom_inherited,
				material_snapshot_json,status,created_at,updated_at
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,'awaiting_schedule',now(),now())
			RETURNING id
		`, r.schema), requestID, item.LineNo, item.ProductID, item.ParentProductID, item.ProductName,
			item.SpecG, item.Qty, item.NeedG, prepared.Warehouse, item.BomVersionID, item.BomVersionNo,
			item.BomSourceProductID, item.BomInherited, item.MaterialSnapshot).Scan(&requestItemID); err != nil {
			return customerportalapp.ProcessingRequest{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_production_demands(
				request_id,request_item_id,request_no,customer_id,product_id,product_name,spec_g,target_qty,
				need_g,target_warehouse,status,created_at,updated_at
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'planned',now(),now())
		`, r.schema), requestID, requestItemID, requestNo, cmd.CustomerID, item.ProductID, item.ProductName,
			item.SpecG, item.Qty, item.NeedG, prepared.Warehouse); err != nil {
			return customerportalapp.ProcessingRequest{}, err
		}
		for _, need := range item.Materials {
			key := processingNeedKey(need.ComponentType, need.MaterialID, need.ComponentSpecG)
			allocations, remaining, ok := allocateProcessingSources(mutableSources[key], need.RequiredG, need.RequiredUnits)
			if !ok {
				return customerportalapp.ProcessingRequest{}, &customerportalapp.ProcessingMaterialsUnavailableError{Preview: prepared.Preview}
			}
			mutableSources[key] = remaining
			for _, allocation := range allocations {
				if _, err := tx.Exec(ctx, fmt.Sprintf(`
					INSERT INTO %s.customer_processing_material_reservations(
						request_id,request_item_id,customer_id,material_id,component_type,component_product_id,
						component_spec_g,required_g,required_units,reserved_g,reserved_units,
						source_owner_type,source_customer_id,source_warehouse_code,material_batch_id,status,
						created_at,updated_at
					)
					VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$8,$9,$10,$11,$12,0,'reserved',now(),now())
				`, r.schema), requestID, requestItemID, cmd.CustomerID, need.MaterialID,
					firstNonEmpty(need.ComponentType, "material"), need.ComponentProductID, need.ComponentSpecG,
					allocation.AvailableG, allocation.AvailableUnits, allocation.OwnerType,
					allocation.SourceCustomerID, allocation.WarehouseCode); err != nil {
					return customerportalapp.ProcessingRequest{}, err
				}
				reservationCount++
			}
		}
	}
	actor := portalMiniActor(cmd.CreatedByMiniUserID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "processing_job_request", &requestID, "mini_submit", nil, nil, postgresinfra.StrPtr("awaiting_schedule"), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "request_no": requestNo, "item_count": len(prepared.Resolved),
	}); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_processing_material_reservation", &requestID, "reserve", nil, nil, postgresinfra.StrPtr(fmt.Sprintf("%d", reservationCount)), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "request_no": requestNo, "reservation_count": reservationCount,
	}); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	return r.GetProcessingRequest(ctx, cmd.CustomerID, requestID)
}

func (r Repository) prepareProcessingRequestTx(ctx context.Context, tx pgx.Tx, cmd customerportalapp.CreateProcessingRequestCommand, lock bool) (preparedProcessingRequest, error) {
	if cmd.CustomerID <= 0 {
		return preparedProcessingRequest{}, fmt.Errorf("customer required")
	}
	if len(cmd.Items) == 0 {
		return preparedProcessingRequest{}, fmt.Errorf("items required")
	}
	targets := make([]postgresproduction.CustomerProcessingTarget, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		if err := r.ensureProcessingTargetProductTx(ctx, tx, cmd.CustomerID, item.ProductID); err != nil {
			return preparedProcessingRequest{}, err
		}
		var netContentQty float64
		var netContentUnit, specLabel, skuName, derivedSpecName string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(net_content_qty,0)::float8,COALESCE(net_content_unit,''),
			       COALESCE(spec_label,''),COALESCE(sku_name,''),COALESCE(derived_spec_name,'')
			FROM %s.products WHERE id=$1 AND active=true
		`, r.schema), item.ProductID).Scan(&netContentQty, &netContentUnit, &specLabel, &skuName, &derivedSpecName); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return preparedProcessingRequest{}, fmt.Errorf("target product unavailable")
			}
			return preparedProcessingRequest{}, err
		}
		authoritativeSpecG := authoritativeProcessingTargetSpecG(netContentQty, netContentUnit, firstNonEmpty(specLabel, skuName, derivedSpecName))
		if err := validateProcessingTargetSpecG(item.ProductID, item.SpecG, authoritativeSpecG); err != nil {
			return preparedProcessingRequest{}, err
		}
		targets = append(targets, postgresproduction.CustomerProcessingTarget{ProductID: item.ProductID, SpecG: item.SpecG, Qty: item.Qty})
	}
	warehouse, err := r.processingWarehouseForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return preparedProcessingRequest{}, err
	}
	resolved, err := postgresproduction.ResolveCustomerProcessingTargetsTx(ctx, tx, r.schema, targets)
	if err != nil {
		return preparedProcessingRequest{}, err
	}

	type aggregateNeed struct {
		Name               string
		Unit               string
		ComponentType      string
		ComponentProductID int64
		ComponentSpecG     int64
		RequiredG          int64
		RequiredUnits      int64
	}
	aggregated := map[string]aggregateNeed{}
	keys := make([]string, 0)
	for _, item := range resolved {
		for _, need := range item.Materials {
			key := processingNeedKey(need.ComponentType, need.MaterialID, need.ComponentSpecG)
			current, exists := aggregated[key]
			if !exists {
				current = aggregateNeed{Name: need.MaterialName, Unit: need.Unit, ComponentType: need.ComponentType, ComponentProductID: need.ComponentProductID, ComponentSpecG: need.ComponentSpecG}
				keys = append(keys, key)
			}
			current.RequiredG += need.RequiredG
			current.RequiredUnits += need.RequiredUnits
			aggregated[key] = current
		}
	}
	sort.Strings(keys)
	previewByKey := map[string]customerportalapp.ProcessingMaterialPreview{}
	sourcesByKey := map[string][]processingAvailabilitySource{}
	canSubmit := true
	materials := make([]customerportalapp.ProcessingMaterialPreview, 0, len(keys))
	for _, key := range keys {
		need := aggregated[key]
		parts := strings.Split(key, ":")
		materialID := int64(0)
		_, _ = fmt.Sscan(parts[1], &materialID)
		sources, preview, err := r.processingAvailabilityTx(ctx, tx, cmd.CustomerID, materialID, need.ComponentType, need.ComponentSpecG, lock)
		if err != nil {
			return preparedProcessingRequest{}, err
		}
		preview.MaterialID = materialID
		preview.MaterialName = need.Name
		preview.Unit = need.Unit
		preview.ComponentType = firstNonEmpty(need.ComponentType, "material")
		preview.ComponentProductID = need.ComponentProductID
		preview.ComponentSpecG = need.ComponentSpecG
		preview.RequiredG = need.RequiredG
		preview.RequiredUnits = need.RequiredUnits
		preview.ShortageG = nonnegativeInt64(need.RequiredG - preview.AvailableG)
		preview.ShortageUnits = nonnegativeInt64(need.RequiredUnits - preview.AvailableUnits)
		if preview.ShortageG > 0 || preview.ShortageUnits > 0 {
			canSubmit = false
		}
		previewByKey[key] = preview
		sourcesByKey[key] = sources
		materials = append(materials, preview)
	}

	items := make([]customerportalapp.ProcessingRequestItem, 0, len(resolved))
	for _, item := range resolved {
		row := customerportalapp.ProcessingRequestItem{
			LineNo: item.LineNo, ProductID: item.ProductID, ParentProductID: item.ParentProductID,
			ProductName: item.ProductName, SpecG: item.SpecG, Qty: item.Qty, NeedG: item.NeedG,
			TargetWarehouse: warehouse, BomVersionID: item.BomVersionID, BomVersionNo: item.BomVersionNo,
			BomSourceProductID: item.BomSourceProductID, BomInherited: item.BomInherited,
			MaterialSnapshot: item.MaterialSnapshot, Status: "awaiting_schedule", MaxProducibleQty: math.MaxInt64,
		}
		for _, need := range item.Materials {
			key := processingNeedKey(need.ComponentType, need.MaterialID, need.ComponentSpecG)
			material := previewByKey[key]
			material.RequiredG = need.RequiredG
			material.RequiredUnits = need.RequiredUnits
			material.ShortageG = nonnegativeInt64(need.RequiredG - material.AvailableG)
			material.ShortageUnits = nonnegativeInt64(need.RequiredUnits - material.AvailableUnits)
			row.Materials = append(row.Materials, material)
			if need.RequiredG > 0 {
				row.MaxProducibleQty = minPositiveInt64(row.MaxProducibleQty, safeProductionQty(material.AvailableG, item.Qty, need.RequiredG))
			}
			if need.RequiredUnits > 0 {
				row.MaxProducibleQty = minPositiveInt64(row.MaxProducibleQty, safeProductionQty(material.AvailableUnits, item.Qty, need.RequiredUnits))
			}
		}
		if row.MaxProducibleQty == math.MaxInt64 {
			row.MaxProducibleQty = 0
		}
		items = append(items, row)
	}
	return preparedProcessingRequest{
		Preview:  customerportalapp.ProcessingRequestPreview{CanSubmit: canSubmit, Items: items, Materials: materials},
		Resolved: resolved, Warehouse: warehouse, SourcesByKey: sourcesByKey,
	}, nil
}

func (r Repository) processingAvailabilityTx(ctx context.Context, tx pgx.Tx, customerID, materialID int64, componentType string, componentSpecG int64, lock bool) ([]processingAvailabilitySource, customerportalapp.ProcessingMaterialPreview, error) {
	componentType = firstNonEmpty(strings.TrimSpace(componentType), "material")
	if lock {
		lockKey := fmt.Sprintf("customer-processing:%s:%d:%d", componentType, materialID, componentSpecG)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
			return nil, customerportalapp.ProcessingMaterialPreview{}, err
		}
		if err := r.lockProcessingStockRowsTx(ctx, tx, customerID, materialID, componentType, componentSpecG); err != nil {
			return nil, customerportalapp.ProcessingMaterialPreview{}, err
		}
	}
	var rows pgx.Rows
	var err error
	if componentType == "finished_product" {
		rows, err = tx.Query(ctx, fmt.Sprintf(`
			WITH batches AS (
				SELECT COALESCE(last_ledger.warehouse,'finished_goods') AS warehouse,
				       SUM(b.remaining_g)::bigint AS qty_g,SUM(b.remaining_units)::bigint AS qty_units
				FROM %s.stock_batches b
				LEFT JOIN LATERAL (
					SELECT l.warehouse FROM %s.stock_ledger_entries l
					WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type
					  AND l.item_id=b.item_id AND l.spec_g=b.spec_g
					ORDER BY l.id DESC LIMIT 1
				) last_ledger ON true
				WHERE b.item_type='finished_product' AND b.item_id=$1
				  AND ($2::bigint=0 OR b.spec_g=$2)
				  AND (b.remaining_g>0 OR b.remaining_units>0)
				  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
				GROUP BY COALESCE(last_ledger.warehouse,'finished_goods')
			)
			SELECT batches.warehouse,COALESCE(w.kind,''),COALESCE(w.customer_id,0),batches.qty_g,batches.qty_units
			FROM batches JOIN %s.warehouses w ON w.code=batches.warehouse AND w.active=true
			WHERE w.customer_id=$3 OR (w.customer_id=0 AND w.kind IN ('raw','wip','packaging','finished','customer'))
		`, r.schema, r.schema, r.schema), materialID, componentSpecG, customerID)
	} else {
		rows, err = tx.Query(ctx, fmt.Sprintf(`
			SELECT l.warehouse,COALESCE(w.kind,''),COALESCE(w.customer_id,0),
			       SUM(l.qty_g)::bigint,SUM(l.qty_units)::bigint
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			JOIN %s.warehouses w ON w.code=l.warehouse AND w.active=true
			WHERE l.material_id=$1
			  AND (l.qty_g>0 OR l.qty_units>0)
			  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			  AND (w.customer_id=$2 OR (w.customer_id=0 AND w.kind IN ('raw','wip','packaging','customer')))
			GROUP BY l.warehouse,w.kind,w.customer_id
		`, r.schema, r.schema, r.schema), materialID, customerID)
	}
	if err != nil {
		return nil, customerportalapp.ProcessingMaterialPreview{}, err
	}
	defer rows.Close()
	sources := make([]processingAvailabilitySource, 0)
	for rows.Next() {
		var source processingAvailabilitySource
		if err := rows.Scan(&source.WarehouseCode, &source.WarehouseKind, &source.SourceCustomerID, &source.AvailableG, &source.AvailableUnits); err != nil {
			return nil, customerportalapp.ProcessingMaterialPreview{}, err
		}
		if source.SourceCustomerID == customerID {
			source.OwnerType = "customer"
		} else {
			source.OwnerType = "factory"
			source.SourceCustomerID = 0
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, customerportalapp.ProcessingMaterialPreview{}, err
	}

	reservedByWarehouse := map[string][2]int64{}
	reservationRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT CASE
		         WHEN component_type='finished_product' AND finished_stock_batch_id>0 THEN 'wip'
		         WHEN component_type<>'finished_product' AND material_batch_id>0 THEN 'wip'
		         ELSE source_warehouse_code
		       END AS effective_warehouse,
		       SUM(GREATEST(0,reserved_g-consumed_g-returned_g))::bigint,
		       SUM(GREATEST(0,reserved_units-consumed_units-returned_units))::bigint
		FROM %s.customer_processing_material_reservations
		WHERE material_id=$1 AND component_type=$2 AND component_spec_g=$3 AND status='reserved'
		GROUP BY effective_warehouse
	`, r.schema), materialID, firstNonEmpty(componentType, "material"), componentSpecG)
	if err != nil {
		return nil, customerportalapp.ProcessingMaterialPreview{}, err
	}
	for reservationRows.Next() {
		var warehouse string
		var g, units int64
		if err := reservationRows.Scan(&warehouse, &g, &units); err != nil {
			reservationRows.Close()
			return nil, customerportalapp.ProcessingMaterialPreview{}, err
		}
		reservedByWarehouse[warehouse] = [2]int64{g, units}
	}
	reservationRows.Close()

	var workOrderReservedG, workOrderReservedUnits int64
	if componentType != "finished_product" {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,
			       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint
			FROM %s.work_order_material_reservations wr
			WHERE wr.material_id=$1 AND wr.status='reserved'
			  AND NOT EXISTS (
				SELECT 1 FROM %s.customer_processing_material_reservations cpr
				WHERE cpr.work_order_id=wr.work_order_id AND cpr.material_id=wr.material_id AND cpr.status='reserved'
			  )
		`, r.schema, r.schema), materialID).Scan(&workOrderReservedG, &workOrderReservedUnits); err != nil {
			return nil, customerportalapp.ProcessingMaterialPreview{}, err
		}
	}

	sort.SliceStable(sources, func(i, j int) bool {
		return processingSourcePriority(sources[i]) < processingSourcePriority(sources[j]) ||
			(processingSourcePriority(sources[i]) == processingSourcePriority(sources[j]) && sources[i].WarehouseCode < sources[j].WarehouseCode)
	})
	preview := customerportalapp.ProcessingMaterialPreview{}
	for index := range sources {
		source := &sources[index]
		reserved := reservedByWarehouse[source.WarehouseCode]
		source.AvailableG = nonnegativeInt64(source.AvailableG - reserved[0])
		source.AvailableUnits = nonnegativeInt64(source.AvailableUnits - reserved[1])
		preview.ReservedG += reserved[0]
		preview.ReservedUnits += reserved[1]
		if source.OwnerType == "factory" && source.WarehouseKind == "wip" {
			deductG := minInt64(source.AvailableG, workOrderReservedG)
			deductUnits := minInt64(source.AvailableUnits, workOrderReservedUnits)
			source.AvailableG -= deductG
			source.AvailableUnits -= deductUnits
			workOrderReservedG -= deductG
			workOrderReservedUnits -= deductUnits
			preview.ReservedG += deductG
			preview.ReservedUnits += deductUnits
		}
		preview.AvailableG += source.AvailableG
		preview.AvailableUnits += source.AvailableUnits
		switch {
		case source.OwnerType == "customer" && source.WarehouseKind == "wip":
			preview.CustomerWIPG += source.AvailableG
			preview.CustomerWIPUnits += source.AvailableUnits
		case source.OwnerType == "customer":
			preview.CustomerInventoryG += source.AvailableG
			preview.CustomerInventoryUnits += source.AvailableUnits
		case source.WarehouseKind == "wip":
			preview.FactoryWIPG += source.AvailableG
			preview.FactoryWIPUnits += source.AvailableUnits
		default:
			preview.FactoryInventoryG += source.AvailableG
			preview.FactoryInventoryUnits += source.AvailableUnits
		}
	}
	return sources, preview, nil
}

func (r Repository) lockProcessingStockRowsTx(ctx context.Context, tx pgx.Tx, customerID, materialID int64, componentType string, componentSpecG int64) error {
	var rows pgx.Rows
	var err error
	if componentType == "finished_product" {
		rows, err = tx.Query(ctx, fmt.Sprintf(`
			SELECT b.id
			FROM %s.stock_batches b
			WHERE b.item_type='finished_product' AND b.item_id=$1
			  AND ($2::bigint=0 OR b.spec_g=$2)
			  AND (b.remaining_g>0 OR b.remaining_units>0)
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			ORDER BY b.id
			FOR UPDATE OF b
		`, r.schema), materialID, componentSpecG)
	} else {
		rows, err = tx.Query(ctx, fmt.Sprintf(`
			SELECT b.id,l.warehouse
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			JOIN %s.warehouses w ON w.code=l.warehouse AND w.active=true
			WHERE l.material_id=$1 AND (l.qty_g>0 OR l.qty_units>0)
			  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			  AND (w.customer_id=$2 OR (w.customer_id=0 AND w.kind IN ('raw','wip','packaging','customer')))
			ORDER BY b.id,l.warehouse
			FOR UPDATE OF b,l
		`, r.schema, r.schema, r.schema), materialID, customerID)
	}
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func processingSourcePriority(source processingAvailabilitySource) int {
	switch {
	case source.OwnerType == "customer" && source.WarehouseKind == "wip":
		return 0
	case source.OwnerType == "customer":
		return 1
	case source.WarehouseKind == "wip":
		return 2
	default:
		return 3
	}
}

func allocateProcessingSources(sources []processingAvailabilitySource, requiredG, requiredUnits int64) ([]processingAvailabilitySource, []processingAvailabilitySource, bool) {
	remaining := append([]processingAvailabilitySource(nil), sources...)
	allocations := make([]processingAvailabilitySource, 0)
	needG, needUnits := requiredG, requiredUnits
	for index := range remaining {
		if needG <= 0 && needUnits <= 0 {
			break
		}
		allocatedG := minInt64(remaining[index].AvailableG, needG)
		allocatedUnits := minInt64(remaining[index].AvailableUnits, needUnits)
		if allocatedG <= 0 && allocatedUnits <= 0 {
			continue
		}
		allocation := remaining[index]
		allocation.AvailableG = allocatedG
		allocation.AvailableUnits = allocatedUnits
		allocations = append(allocations, allocation)
		remaining[index].AvailableG -= allocatedG
		remaining[index].AvailableUnits -= allocatedUnits
		needG -= allocatedG
		needUnits -= allocatedUnits
	}
	return allocations, remaining, needG <= 0 && needUnits <= 0
}

func safeProductionQty(available, requestedQty, required int64) int64 {
	if available <= 0 || requestedQty <= 0 || required <= 0 {
		return 0
	}
	if available > math.MaxInt64/requestedQty {
		return math.MaxInt64
	}
	return available * requestedQty / required
}

func minPositiveInt64(current, candidate int64) int64 {
	if candidate < current {
		return candidate
	}
	return current
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func nonnegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func (r Repository) ListProcessingRequests(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProcessingRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id,r.request_no,r.input_material_id,COALESCE(m.name,''),r.input_qty_g,
		       r.target_product_id,COALESCE(p.name,''),r.target_spec_g,r.target_qty,
		       r.status,r.note,to_char(r.created_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(r.accepted_at,'YYYY-MM-DD HH24:MI'),''),r.linked_work_order_id
		FROM %s.processing_job_requests r
		LEFT JOIN %s.materials m ON m.id=r.input_material_id
		LEFT JOIN %s.products p ON p.id=r.target_product_id
		WHERE r.customer_id=$1
		ORDER BY r.created_at DESC,r.id DESC
		LIMIT $2
	`, r.schema, r.schema, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProcessingRequest, 0)
	for rows.Next() {
		var row customerportalapp.ProcessingRequest
		if err := rows.Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputMaterialName, &row.InputQtyG,
			&row.TargetProductID, &row.TargetProductName, &row.TargetSpecG, &row.TargetQty,
			&row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
			return nil, err
		}
		row.Items, err = r.listProcessingRequestItems(ctx, customerID, row.ID)
		if err != nil {
			return nil, err
		}
		applyProcessingRequestDerivedFields(&row)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetProcessingRequest(ctx context.Context, customerID, requestID int64) (customerportalapp.ProcessingRequest, error) {
	var row customerportalapp.ProcessingRequest
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id,r.request_no,r.input_material_id,COALESCE(m.name,''),r.input_qty_g,
		       r.target_product_id,COALESCE(p.name,''),r.target_spec_g,r.target_qty,
		       r.status,r.note,to_char(r.created_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(r.accepted_at,'YYYY-MM-DD HH24:MI'),''),r.linked_work_order_id
		FROM %s.processing_job_requests r
		LEFT JOIN %s.materials m ON m.id=r.input_material_id
		LEFT JOIN %s.products p ON p.id=r.target_product_id
		WHERE r.customer_id=$1 AND r.id=$2
	`, r.schema, r.schema, r.schema), customerID, requestID).Scan(
		&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputMaterialName, &row.InputQtyG,
		&row.TargetProductID, &row.TargetProductName, &row.TargetSpecG, &row.TargetQty,
		&row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.ProcessingRequest{}, fmt.Errorf("processing request not found")
	}
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	row.Items, err = r.listProcessingRequestItems(ctx, customerID, requestID)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	applyProcessingRequestDerivedFields(&row)
	return row, nil
}

func (r Repository) listProcessingRequestItems(ctx context.Context, customerID, requestID int64) ([]customerportalapp.ProcessingRequestItem, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT i.id,i.line_no,i.product_id,i.parent_product_id,i.product_name,i.spec_g,i.target_qty,i.need_g,
		       i.target_warehouse,i.bom_version_id,i.bom_version_no,i.bom_source_product_id,i.bom_inherited,
		       COALESCE(i.material_snapshot_json,'[]'::jsonb)::text,
		       i.production_plan_id,i.production_plan_item_id,i.linked_work_order_id,COALESCE(wo.work_order_no,''),
		       CASE
		         WHEN COALESCE(wo.status,'')='completed' THEN 'completed'
		         WHEN COALESCE(wo.status,'')='partially_completed' THEN 'partially_completed'
		         WHEN COALESCE(wo.status,'')='paused' THEN 'paused'
		         WHEN COALESCE(wo.status,'')='running' THEN 'running'
		         WHEN COALESCE(wo.status,'')='released' THEN 'released'
		         WHEN COALESCE(wo.status,'')='cancelled' THEN 'cancelled'
		         WHEN COALESCE(pp.status,'')='cancelled' OR i.status='cancelled' THEN 'cancelled'
		         WHEN i.production_plan_item_id>0 THEN 'planned'
		         ELSE 'awaiting_schedule'
		       END
		FROM %s.processing_job_request_items i
		JOIN %s.processing_job_requests r ON r.id=i.request_id AND r.customer_id=$1
		LEFT JOIN %s.production_plans pp ON pp.id=i.production_plan_id
		LEFT JOIN %s.work_orders wo ON wo.id=i.linked_work_order_id
		WHERE i.request_id=$2
		ORDER BY i.line_no,i.id
	`, r.schema, r.schema, r.schema, r.schema), customerID, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProcessingRequestItem, 0)
	for rows.Next() {
		var row customerportalapp.ProcessingRequestItem
		if err := rows.Scan(&row.ID, &row.LineNo, &row.ProductID, &row.ParentProductID, &row.ProductName,
			&row.SpecG, &row.Qty, &row.NeedG, &row.TargetWarehouse, &row.BomVersionID, &row.BomVersionNo,
			&row.BomSourceProductID, &row.BomInherited, &row.MaterialSnapshot,
			&row.ProductionPlanID, &row.ProductionPlanItemID, &row.LinkedWorkOrderID, &row.WorkOrderNo, &row.Status); err != nil {
			return nil, err
		}
		row.WorkOrderID = row.LinkedWorkOrderID
		out = append(out, row)
	}
	return out, rows.Err()
}

func applyProcessingRequestDerivedFields(row *customerportalapp.ProcessingRequest) {
	if row == nil || len(row.Items) == 0 {
		return
	}
	first := row.Items[0]
	row.TargetProductID = first.ProductID
	row.TargetProductName = first.ProductName
	row.TargetSpecG = first.SpecG
	row.TargetQty = int(first.Qty)
	row.LinkedWorkOrderID = first.LinkedWorkOrderID
	priority := map[string]int{"cancelled": 0, "awaiting_schedule": 1, "planned": 2, "released": 3, "paused": 4, "running": 5, "partially_completed": 6, "completed": 7}
	bestStatus, bestPriority := "awaiting_schedule", -1
	completed := 0
	for _, item := range row.Items {
		if item.Status == "completed" {
			completed++
		}
		if priority[item.Status] > bestPriority {
			bestPriority = priority[item.Status]
			bestStatus = item.Status
		}
	}
	if completed > 0 && completed < len(row.Items) {
		bestStatus = "partially_completed"
	}
	row.Status = bestStatus
}
