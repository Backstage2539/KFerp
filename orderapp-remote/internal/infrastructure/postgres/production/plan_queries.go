package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	productionapp "orderapp/internal/application/production"
)

type planBomItem struct {
	ProductID            int64
	RoastLevel           string
	YieldRate            float64
	MaterialID           int64
	MaterialName         string
	MaterialUnit         string
	RatioPct             float64
	MaterialLossRate     float64
	ComponentType        string
	ComponentProductID   int64
	ComponentProductName string
	ComponentSpecG       int64
	ConsumeUnit          string
	QtyPerUnit           float64
	OutputQty            float64
	OutputUnit           string
	DripBoxBagCount      int64
}

type planParams struct {
	YieldRate      float64
	DripExtraG     int64
	DripBoxSpec    int64
	EnableDripBox  bool
	BagNameBySpecG map[int64]string
}

func (r Repository) PlanSummary(ctx context.Context, query productionapp.PlanSummaryQuery) (productionapp.PlanSummaryData, error) {
	data := productionapp.PlanSummaryData{
		From:       query.From,
		To:         query.To,
		CustomerID: query.CustomerID,
		Selected:   query.Selected,
		PlanReady:  query.Plan && len(query.Selected) > 0,
	}
	rows, err := fetchUnproducedNeeds(ctx, r.pool, r.schema, query.From, query.To, query.CustomerID)
	if err != nil {
		return data, err
	}
	rows, err = r.splitUnproducedNeedsByProductionPlan(ctx, rows)
	if err != nil {
		return data, err
	}
	appRows := unprodRowsToApp(rows)
	dripRows, err := r.fetchDripPlanNeeds(ctx, query.From, query.To, query.CustomerID)
	if err != nil {
		return data, err
	}
	appRows = mergeDripPlanRows(appRows, dripRows)
	if err := r.attachProductionDemandStatuses(ctx, appRows); err != nil {
		return data, err
	}
	appRows = filterProductionDemandRows(appRows, query.DemandStatus)
	data.Rows = appRows
	if !data.PlanReady {
		return data, nil
	}

	planRows := make([]productionapp.UnprodNeedRow, 0)
	selectedCount := 0
	for _, row := range appRows {
		key := producePlanKey(row.ProductID, row.SpecG)
		if !query.Selected[key] {
			continue
		}
		selectedCount++
		if strings.TrimSpace(row.BlockingReason) != "" {
			data.PlanReady = false
			data.Error = row.BlockingReason
			return data, nil
		}
		if row.GapG <= 0 || row.DemandStatus != "unplanned" {
			continue
		}
		planRows = append(planRows, row)
	}
	if selectedCount > 0 && len(planRows) == 0 {
		data.StockTip = "库存充足：当前已选商品库存均可满足；录单确认使用成品批次后会进入库存待发货，可直接发货。"
		return data, nil
	}

	bomSummaries, err := r.loadResolvedPlanBomSummaries(ctx, planRows)
	if err != nil {
		return data, err
	}
	theoreticalInputByKey := map[string]int64{}
	bomVersionByDemandKey := map[string]int64{}
	bomLossByDemandKey := map[string]float64{}
	skipMaterialPlanDemandKeys := map[string]bool{}
	includedMaterialPlanDemandKeys := map[string]bool{}
	for i, row := range planRows {
		demandKey := producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)
		includedMaterialPlanDemandKeys[demandKey] = true
		if i >= len(bomSummaries) || bomSummaries[i].BomVersionID <= 0 {
			continue
		}
		theoreticalInputByKey[demandKey] = bomSummaries[i].InputG
		bomVersionByDemandKey[demandKey] = bomSummaries[i].BomVersionID
		bomLossByDemandKey[demandKey] = bomSummaries[i].MaterialLossRate
	}
	params := defaultPlanParams()
	if mappings, err := postgresinfra.ListBagSpecMappings(ctx, r.pool, r.schema); err == nil {
		params.BagNameBySpecG = bomdomain.MappingNameBySpec(mappings)
	}
	bomMap, err := r.loadPlanBomItemsFromRows(ctx, planRows, bomSummaries)
	if err != nil {
		return data, err
	}
	for i, row := range planRows {
		demandKey := producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)
		if i >= len(bomSummaries) ||
			(bomSummaries[i].BomVersionID <= 0 &&
				(!bomSummaries[i].LegacyCompatible || len(bomMap[demandKey]) == 0)) {
			skipMaterialPlanDemandKeys[demandKey] = true
		}
	}
	machines, _ := r.ListMachines(ctx, true)
	if err := r.attachDripUpstreamShortages(ctx, planRows, bomMap); err != nil {
		return data, err
	}
	materialPreviewRows := make([]productionapp.UnprodNeedRow, 0, len(planRows))
	for i, row := range planRows {
		if i < len(bomSummaries) &&
			bomSummaries[i].BomVersionID <= 0 &&
			strings.TrimSpace(bomSummaries[i].Error) != "" &&
			!bomSummaries[i].LegacyCompatible {
			continue
		}
		materialPreviewRows = append(materialPreviewRows, row)
	}
	data.RoastPlans = buildRoastPlanRows(planRows, machines, nil, theoreticalInputByKey)
	data.MaterialRatios = buildRoastPlanMaterialRatios(materialPreviewRows, bomMap)
	data.PlanRows = buildProducePlanDisplayRows(planRows, nil, theoreticalInputByKey)
	for i := range data.PlanRows {
		if i < len(bomSummaries) {
			data.PlanRows[i].BomMaterialLossRate = bomSummaries[i].MaterialLossRate
			data.PlanRows[i].BomSummaryError = bomSummaries[i].Error
		}
	}
	data.Materials = calcProducePlanMaterialsFromFinalInputs(materialPreviewRows, theoreticalInputByKey, bomMap, params)
	materialPlan, err := r.MaterialPlan(ctx, productionapp.MaterialPlanQuery{
		From:                  query.From,
		To:                    query.To,
		CustomerID:            query.CustomerID,
		Selected:              query.Selected,
		IncludedDemandKeys:    includedMaterialPlanDemandKeys,
		InputByDemandKey:      theoreticalInputByKey,
		BomVersionByDemandKey: bomVersionByDemandKey,
		BomLossByDemandKey:    bomLossByDemandKey,
		SkipDemandKeys:        skipMaterialPlanDemandKeys,
	})
	if err != nil {
		return data, err
	}
	data.Materials = mergeMaterialAvailability(data.Materials, materialPlan.Rows)
	data.RoastSplits = calcRoastSplits(planRows, machines, params.YieldRate)
	return data, nil
}

func unprodRowsToApp(rows []UnprodNeedRow) []productionapp.UnprodNeedRow {
	out := make([]productionapp.UnprodNeedRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, productionapp.UnprodNeedRow{
			ProductID:                row.ProductID,
			ParentProductID:          row.ParentProductID,
			Product:                  row.Product,
			OrderNos:                 row.OrderNos,
			SpecLabel:                row.SpecLabel,
			SalesUnit:                row.SalesUnit,
			SpecG:                    row.SpecG,
			NeedUnits:                row.NeedUnits,
			NeedG:                    row.NeedG,
			InvUnits:                 row.InvUnits,
			InvLooseG:                row.InvLooseG,
			InvG:                     row.InvG,
			GapG:                     row.GapG,
			SalesSpecCount:           row.SalesSpecCount,
			InventoryQtyPerSalesUnit: row.InventoryQtyPerSalesUnit,
			InventoryUnit:            row.InventoryUnit,
			NeedInventoryQty:         row.NeedInventoryQty,
			AvailableInventoryQty:    row.AvailableInventoryQty,
			GapInventoryQty:          row.GapInventoryQty,
			GapSalesSpecCount:        row.GapSalesSpecCount,
			SalesSpecSnapshotJSON:    row.SalesSpecSnapshotJSON,
			ProductionKind:           catalogdomain.NormalizeProductKind(row.ProductionKind),
			ProductTypeCategoryID:    row.ProductTypeCategoryID,
			ProductSubtypeCategoryID: row.ProductSubtypeCategoryID,
			ProductTypeName:          row.ProductTypeName,
			ProductSubtypeName:       row.ProductSubtypeName,
			OperationTemplateID:      row.OperationTemplateID,
			DemandStatus:             row.DemandStatus,
			DemandStatusLabel:        row.DemandStatusLabel,
			DemandSelectable:         row.DemandSelectable,
			BlockingReason:           row.BlockingReason,
			ProductionPlanID:         row.ProductionPlanID,
			ProductionPlanNo:         row.ProductionPlanNo,
			WorkOrderID:              row.WorkOrderID,
			WorkOrderNo:              row.WorkOrderNo,
		})
	}
	return out
}

type productionDemandPlanState struct {
	Status           string
	ProductionPlanID int64
	ProductionPlanNo string
	WorkOrderID      int64
	WorkOrderNo      string
}

type productionDemandPart struct {
	ProductID         int64
	SpecG             int64
	OrderNo           string
	NeedUnits         int64
	ForceProduceUnits int64
	State             productionDemandPlanState
}

func (r Repository) splitUnproducedNeedsByProductionPlan(ctx context.Context, rows []UnprodNeedRow) ([]UnprodNeedRow, error) {
	return r.splitUnproducedNeedsByProductionPlanQuery(ctx, r.pool, rows)
}

func (r Repository) splitUnproducedNeedsByProductionPlanQuery(ctx context.Context, queryer productionDemandQueryer, rows []UnprodNeedRow) ([]UnprodNeedRow, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	parts, err := r.fetchProductionDemandPartsQuery(ctx, queryer, rows)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		for i := range rows {
			rows[i].DemandStatus = "unplanned"
			rows[i].DemandStatusLabel = productionDemandStatusLabel("unplanned")
			rows[i].DemandSelectable = rows[i].GapG > 0 && strings.TrimSpace(rows[i].BlockingReason) == ""
		}
		return rows, nil
	}
	partsByKey := map[string][]productionDemandPart{}
	for _, part := range parts {
		key := producePlanKey(part.ProductID, part.SpecG)
		partsByKey[key] = append(partsByKey[key], part)
	}
	out := make([]UnprodNeedRow, 0, len(rows))
	for _, row := range rows {
		rowParts := filterProductionDemandPartsForRow(partsByKey[producePlanKey(row.ProductID, row.SpecG)], row.OrderNos)
		if len(rowParts) == 0 {
			row.DemandStatus = "unplanned"
			row.DemandStatusLabel = productionDemandStatusLabel("unplanned")
			row.DemandSelectable = row.GapG > 0 && strings.TrimSpace(row.BlockingReason) == ""
			out = append(out, row)
			continue
		}
		out = append(out, splitProductionDemandRowByParts(row, rowParts)...)
	}
	return out, nil
}

func filterProductionDemandPartsForRow(parts []productionDemandPart, orderNos string) []productionDemandPart {
	if len(parts) == 0 {
		return nil
	}
	allowed := splitProductionDemandOrderNos(orderNos)
	out := make([]productionDemandPart, 0, len(parts))
	for _, part := range parts {
		if allowed[part.OrderNo] {
			out = append(out, part)
		}
	}
	return out
}

type productionDemandPartGroup struct {
	row               UnprodNeedRow
	orderNos          map[string]bool
	forceProduceUnits int64
}

func splitProductionDemandRowByParts(row UnprodNeedRow, parts []productionDemandPart) []UnprodNeedRow {
	groups := map[string]*productionDemandPartGroup{}
	order := make([]string, 0)
	for _, part := range parts {
		status := part.State.Status
		if status == "" {
			status = "unplanned"
		}
		groupKey := status
		if status != "unplanned" {
			groupKey = fmt.Sprintf("%s:%d:%d", status, part.State.ProductionPlanID, part.State.WorkOrderID)
		}
		group := groups[groupKey]
		if group == nil {
			next := row
			next.OrderNos = ""
			next.NeedUnits = 0
			next.NeedG = 0
			next.GapG = 0
			next.DemandStatus = status
			next.DemandStatusLabel = productionDemandStatusLabel(status)
			next.DemandSelectable = false
			next.ProductionPlanID = part.State.ProductionPlanID
			next.ProductionPlanNo = part.State.ProductionPlanNo
			next.WorkOrderID = part.State.WorkOrderID
			next.WorkOrderNo = part.State.WorkOrderNo
			group = &productionDemandPartGroup{row: next, orderNos: map[string]bool{}}
			groups[groupKey] = group
			order = append(order, groupKey)
		}
		group.row.NeedUnits += part.NeedUnits
		group.forceProduceUnits += part.ForceProduceUnits
		group.orderNos[part.OrderNo] = true
	}
	out := make([]UnprodNeedRow, 0, len(groups))
	for _, groupKey := range order {
		group := groups[groupKey]
		group.row.OrderNos = joinProductionDemandOrderNos(group.orderNos)
		if group.row.InventoryQtyPerSalesUnit > 0 {
			group.row.SalesSpecCount = float64(group.row.NeedUnits)
			group.row.NeedInventoryQty = productiondomain.SalesSpecCountToInventoryQuantity(group.row.SalesSpecCount, group.row.InventoryQtyPerSalesUnit)
			group.row.NeedG = productiondomain.InventoryQuantityToLegacyGrams(group.row.NeedInventoryQty, group.row.InventoryUnit)
		} else {
			group.row.NeedG = group.row.NeedUnits * group.row.SpecG
		}
		if group.row.DemandStatus == "unplanned" {
			group.row.GapG = calcProductionDemandGap(group.row.SpecG, group.row.NeedUnits, group.forceProduceUnits, row.AvailableG)
			if group.row.InventoryQtyPerSalesUnit > 0 && group.row.SpecG > 0 {
				group.row.GapSalesSpecCount = float64(group.row.GapG) / float64(group.row.SpecG)
				group.row.GapInventoryQty = productiondomain.SalesSpecCountToInventoryQuantity(group.row.GapSalesSpecCount, group.row.InventoryQtyPerSalesUnit)
			}
			group.row.DemandSelectable = group.row.GapG > 0 && strings.TrimSpace(group.row.BlockingReason) == ""
		} else {
			group.row.GapG = group.row.NeedG
			if group.row.InventoryQtyPerSalesUnit > 0 {
				group.row.GapSalesSpecCount = group.row.SalesSpecCount
				group.row.GapInventoryQty = group.row.NeedInventoryQty
			}
			group.row.DemandSelectable = false
		}
		out = append(out, group.row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := productionDemandStatusPriority(out[i].DemandStatus), productionDemandStatusPriority(out[j].DemandStatus)
		if left != right {
			return left < right
		}
		return out[i].OrderNos < out[j].OrderNos
	})
	return out
}

func calcProductionDemandGap(specG, needUnits, forceProduceUnits, availableG int64) int64 {
	if specG <= 0 || needUnits <= 0 {
		return 0
	}
	if forceProduceUnits < 0 {
		forceProduceUnits = 0
	}
	if forceProduceUnits > needUnits {
		forceProduceUnits = needUnits
	}
	forceG := forceProduceUnits * specG
	nonForceG := (needUnits - forceProduceUnits) * specG
	if availableG < 0 {
		availableG = 0
	}
	return forceG + maxInt64(0, nonForceG-availableG)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func joinProductionDemandOrderNos(values map[string]bool) string {
	if len(values) == 0 {
		return ""
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func (r Repository) fetchProductionDemandPartsQuery(ctx context.Context, queryer productionDemandQueryer, rows []UnprodNeedRow) ([]productionDemandPart, error) {
	productIDs, specGs, orderNos := productionDemandPartRequestTuples(rows)
	if len(productIDs) == 0 {
		return nil, nil
	}
	q := fmt.Sprintf(`
		WITH requested AS (
			SELECT *
			FROM unnest($1::bigint[],$2::bigint[],$3::text[]) AS requested(product_id,spec_g,order_no)
		),
		source_need AS (
			SELECT
				oi.product_id,
				requested.spec_g,
				COALESCE(o.order_no,'') AS order_no,
				SUM(COALESCE(oi.qty,0))::bigint AS need_units,
				SUM(CASE WHEN COALESCE(osd.decision,'') = 'produce' THEN COALESCE(oi.qty,0) ELSE 0 END)::bigint AS force_produce_units
			FROM %s.order_items oi
			JOIN %s.orders o ON o.id=oi.order_id
			JOIN requested
			  ON requested.product_id=oi.product_id
			 AND requested.order_no=COALESCE(o.order_no,'')
			LEFT JOIN %s.order_stock_decisions osd ON osd.order_id=o.id
			GROUP BY oi.product_id, requested.spec_g, COALESCE(o.order_no,'')
			UNION ALL
			SELECT
				d.product_id,
				requested.spec_g,
				COALESCE(d.request_no,'') AS order_no,
				SUM(COALESCE(d.target_qty,0))::bigint AS need_units,
				SUM(COALESCE(d.target_qty,0))::bigint AS force_produce_units
			FROM %s.customer_processing_production_demands d
			JOIN requested
			  ON requested.product_id=d.product_id
			 AND requested.spec_g=d.spec_g
			 AND requested.order_no=COALESCE(d.request_no,'')
			WHERE d.status='planned'
			GROUP BY d.product_id,requested.spec_g,COALESCE(d.request_no,'')
		)
		SELECT
			sn.product_id,sn.spec_g,sn.order_no,sn.need_units,sn.force_produce_units,
			COALESCE(pm.plan_id,0),COALESCE(pm.plan_no,''),COALESCE(pm.plan_status,''),
			COALESCE(pm.work_order_id,0),COALESCE(pm.work_order_no,''),COALESCE(pm.work_order_status,'')
		FROM source_need sn
		LEFT JOIN LATERAL (
			SELECT
				pp.id AS plan_id,
				pp.plan_no,
				COALESCE(pp.status,'') AS plan_status,
				COALESCE(wo.id,0) AS work_order_id,
				COALESCE(wo.work_order_no,'') AS work_order_no,
				COALESCE(wo.status,'') AS work_order_status,
				CASE
					WHEN COALESCE(wo.status,'')='completed' OR COALESCE(pp.status,'')='completed' THEN 3
					WHEN COALESCE(wo.status,'') IN ('released','running','partially_completed','paused') OR COALESCE(pp.status,'') IN ('draft','submitted','in_progress') THEN 2
					ELSE 1
				END AS priority
			FROM %s.production_plan_items pi
			JOIN %s.production_plans pp ON pp.id=pi.production_plan_id
			LEFT JOIN %s.work_orders wo ON wo.production_plan_item_id=pi.id
			WHERE pi.product_id=sn.product_id
			  AND pi.spec_g=sn.spec_g
			  AND COALESCE(pp.status,'') <> 'cancelled'
			  AND sn.order_no = ANY(string_to_array(replace(COALESCE(pi.order_nos,''),' ',''), ','))
			ORDER BY priority DESC,pp.created_at DESC,pp.id DESC,wo.id DESC
			LIMIT 1
		) pm ON true
		ORDER BY sn.product_id,sn.spec_g,sn.order_no
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema)
	sqlRows, err := queryer.Query(ctx, q, productIDs, specGs, orderNos)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()
	parts := make([]productionDemandPart, 0)
	for sqlRows.Next() {
		var part productionDemandPart
		var planStatus, workOrderStatus string
		if err := sqlRows.Scan(
			&part.ProductID,
			&part.SpecG,
			&part.OrderNo,
			&part.NeedUnits,
			&part.ForceProduceUnits,
			&part.State.ProductionPlanID,
			&part.State.ProductionPlanNo,
			&planStatus,
			&part.State.WorkOrderID,
			&part.State.WorkOrderNo,
			&workOrderStatus,
		); err != nil {
			return nil, err
		}
		part.State.Status = productionDemandStatusFromPlan(planStatus, workOrderStatus)
		parts = append(parts, part)
	}
	return parts, sqlRows.Err()
}

func productionDemandPartRequestTuples(rows []UnprodNeedRow) ([]int64, []int64, []string) {
	seen := map[string]bool{}
	productIDs := make([]int64, 0)
	specGs := make([]int64, 0)
	orderNos := make([]string, 0)
	for _, row := range rows {
		if row.ProductID <= 0 || row.SpecG <= 0 {
			continue
		}
		refs := make([]string, 0)
		for orderNo := range splitProductionDemandOrderNos(row.OrderNos) {
			refs = append(refs, orderNo)
		}
		sort.Strings(refs)
		for _, orderNo := range refs {
			key := fmt.Sprintf("%d\x1f%d\x1f%s", row.ProductID, row.SpecG, orderNo)
			if seen[key] {
				continue
			}
			seen[key] = true
			productIDs = append(productIDs, row.ProductID)
			specGs = append(specGs, row.SpecG)
			orderNos = append(orderNos, orderNo)
		}
	}
	return productIDs, specGs, orderNos
}

func filterProductionDemandRows(rows []productionapp.UnprodNeedRow, status string) []productionapp.UnprodNeedRow {
	// demand_status is the API filter key for derived production demand state.
	status = normalizeProductionDemandStatusFilter(status)
	if status == "" {
		return rows
	}
	out := make([]productionapp.UnprodNeedRow, 0, len(rows))
	for _, row := range rows {
		if row.DemandStatus == status {
			out = append(out, row)
		}
	}
	return out
}

func normalizeProductionDemandStatusFilter(status string) string {
	switch strings.TrimSpace(status) {
	case "unplanned", "in_production", "completed":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func productionDemandStatusLabel(status string) string {
	switch status {
	case "in_production":
		return "生产中"
	case "completed":
		return "生产完成"
	default:
		return "待计划"
	}
}

func (r Repository) attachProductionDemandStatuses(ctx context.Context, rows []productionapp.UnprodNeedRow) error {
	return r.attachProductionDemandStatusesQuery(ctx, r.pool, rows)
}

func (r Repository) attachProductionDemandStatusesQuery(ctx context.Context, queryer productionDemandQueryer, rows []productionapp.UnprodNeedRow) error {
	missing := make([]productionapp.UnprodNeedRow, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.DemandStatus) == "" {
			missing = append(missing, row)
		}
	}
	states := map[string]productionDemandPlanState{}
	if len(missing) > 0 {
		var err error
		states, err = r.productionDemandStatusByKeyQuery(ctx, queryer, missing)
		if err != nil {
			return err
		}
	}
	for i := range rows {
		if strings.TrimSpace(rows[i].DemandStatus) == "" {
			key := producePlanKey(rows[i].ProductID, rows[i].SpecG)
			state := states[key]
			if state.Status == "" {
				state.Status = "unplanned"
			}
			rows[i].DemandStatus = state.Status
			rows[i].ProductionPlanID = state.ProductionPlanID
			rows[i].ProductionPlanNo = state.ProductionPlanNo
			rows[i].WorkOrderID = state.WorkOrderID
			rows[i].WorkOrderNo = state.WorkOrderNo
		}
		rows[i].DemandStatus = normalizeProductionDemandStatusFilter(rows[i].DemandStatus)
		if rows[i].DemandStatus == "" {
			rows[i].DemandStatus = "unplanned"
		}
		rows[i].DemandStatusLabel = productionDemandStatusLabel(rows[i].DemandStatus)
		rows[i].DemandSelectable = rows[i].GapG > 0 &&
			rows[i].DemandStatus == "unplanned" &&
			strings.TrimSpace(rows[i].BlockingReason) == ""
	}
	return nil
}

func (r Repository) productionDemandStatusByKeyQuery(ctx context.Context, queryer productionDemandQueryer, rows []productionapp.UnprodNeedRow) (map[string]productionDemandPlanState, error) {
	out := map[string]productionDemandPlanState{}
	productSeen := map[int64]bool{}
	specSeen := map[int64]bool{}
	orderNosByKey := map[string]map[string]bool{}
	productIDs := make([]int64, 0)
	specGs := make([]int64, 0)
	for _, row := range rows {
		if row.ProductID <= 0 {
			continue
		}
		key := producePlanKey(row.ProductID, row.SpecG)
		if !productSeen[row.ProductID] {
			productSeen[row.ProductID] = true
			productIDs = append(productIDs, row.ProductID)
		}
		if !specSeen[row.SpecG] {
			specSeen[row.SpecG] = true
			specGs = append(specGs, row.SpecG)
		}
		orderNosByKey[key] = splitProductionDemandOrderNos(row.OrderNos)
		out[key] = productionDemandPlanState{Status: "unplanned"}
	}
	if len(productIDs) == 0 || len(specGs) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`
		SELECT pi.product_id,pi.spec_g,COALESCE(pi.order_nos,''),
		       pp.id,pp.plan_no,COALESCE(pp.status,''),
		       COALESCE(wo.id,0),COALESCE(wo.work_order_no,''),COALESCE(wo.status,'')
		FROM %s.production_plan_items pi
		JOIN %s.production_plans pp ON pp.id=pi.production_plan_id
		LEFT JOIN %s.work_orders wo ON wo.production_plan_item_id=pi.id
		WHERE pi.product_id = ANY($1::bigint[])
		  AND pi.spec_g = ANY($2::bigint[])
		  AND COALESCE(pp.status,'') <> 'cancelled'
		ORDER BY pp.created_at DESC,pp.id DESC,wo.id DESC
	`, r.schema, r.schema, r.schema)
	sqlRows, err := queryer.Query(ctx, q, productIDs, specGs)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()
	for sqlRows.Next() {
		var productID, specG, planID, workOrderID int64
		var planOrderNos, planNo, planStatus, workOrderNo, workOrderStatus string
		if err := sqlRows.Scan(&productID, &specG, &planOrderNos, &planID, &planNo, &planStatus, &workOrderID, &workOrderNo, &workOrderStatus); err != nil {
			return nil, err
		}
		key := producePlanKey(productID, specG)
		if !productionDemandOrderNosOverlap(orderNosByKey[key], planOrderNos) {
			continue
		}
		next := productionDemandPlanState{
			Status:           productionDemandStatusFromPlan(planStatus, workOrderStatus),
			ProductionPlanID: planID,
			ProductionPlanNo: planNo,
			WorkOrderID:      workOrderID,
			WorkOrderNo:      workOrderNo,
		}
		current := out[key]
		if productionDemandStatusPriority(next.Status) >= productionDemandStatusPriority(current.Status) {
			out[key] = next
		}
	}
	return out, sqlRows.Err()
}

func productionDemandStatusFromPlan(planStatus, workOrderStatus string) string {
	switch strings.TrimSpace(workOrderStatus) {
	case "completed":
		return "completed"
	case "released", "running", "partially_completed", "paused":
		return "in_production"
	}
	switch strings.TrimSpace(planStatus) {
	case "completed":
		return "completed"
	case "draft", "submitted", "in_progress":
		return "in_production"
	default:
		return "unplanned"
	}
}

func splitProductionDemandOrderNos(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(strings.ReplaceAll(value, " ", ""), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func productionDemandOrderNosOverlap(demandOrderNos map[string]bool, planOrderNos string) bool {
	if len(demandOrderNos) == 0 {
		return true
	}
	for orderNo := range splitProductionDemandOrderNos(planOrderNos) {
		if demandOrderNos[orderNo] {
			return true
		}
	}
	return false
}

func productionDemandStatusPriority(status string) int {
	switch status {
	case "in_production":
		return 3
	case "completed":
		return 2
	default:
		return 1
	}
}

func mergeDripPlanRows(rows []productionapp.UnprodNeedRow, dripRows []productionapp.UnprodNeedRow) []productionapp.UnprodNeedRow {
	if len(dripRows) == 0 {
		return rows
	}
	dripByKey := map[string]productionapp.UnprodNeedRow{}
	for _, row := range dripRows {
		dripByKey[producePlanKey(row.ProductID, row.SpecG)] = row
	}
	out := make([]productionapp.UnprodNeedRow, 0, len(rows)+len(dripRows))
	seen := map[string]bool{}
	for _, row := range rows {
		key := producePlanKey(row.ProductID, row.SpecG)
		if drip, ok := dripByKey[key]; ok {
			out = append(out, drip)
			seen[key] = true
			continue
		}
		out = append(out, row)
	}
	for _, row := range dripRows {
		key := producePlanKey(row.ProductID, row.SpecG)
		if !seen[key] {
			out = append(out, row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GapG != out[j].GapG {
			return out[i].GapG > out[j].GapG
		}
		if out[i].Product != out[j].Product {
			return out[i].Product < out[j].Product
		}
		return out[i].SpecG < out[j].SpecG
	})
	return out
}

func (r Repository) fetchDripPlanNeeds(ctx context.Context, from, to string, customerID int64) ([]productionapp.UnprodNeedRow, error) {
	where := fmt.Sprintf(`WHERE o.is_void=false AND %s
	AND COALESCE(oi.product_id,0) > 0
	AND COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted_bean') = 'drip_bag'
	AND NOT EXISTS (
		SELECT 1 FROM %s.ship_statuses ss
		WHERE ss.id=o.ship_status_id
		  AND ss.name='已发货'
	)`, productionPlanOpenStatusFilter(r.schema, "o"), r.schema)
	args := []any{}
	argn := 1
	if customerID > 0 {
		where += fmt.Sprintf(" AND o.customer_id = $%d", argn)
		args = append(args, customerID)
		argn++
	}
	if s := strings.TrimSpace(from); s != "" {
		where += fmt.Sprintf(" AND o.order_date >= $%d", argn)
		args = append(args, s)
		argn++
	}
	if s := strings.TrimSpace(to); s != "" {
		where += fmt.Sprintf(" AND o.order_date <= $%d", argn)
		args = append(args, s)
		argn++
	}

	q := fmt.Sprintf(`
		WITH line_need AS (
			SELECT
				oi.product_id,
				COALESCE(p.name,'') AS product,
				COALESCE(o.order_no,'') AS order_no,
				COALESCE(NULLIF(oi.unit_bean_g,0), NULLIF(p.drip_bag_grams,0), NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), '')::numeric, 0)::numeric AS spec_g,
				COALESCE(oi.qty,0)::numeric AS qty,
				COALESCE(NULLIF(oi.sales_unit,''), NULLIF(oi.unit,''), oi.price_source_json->>'price_unit') AS sales_unit,
				COALESCE(NULLIF(oi.price_source_json->>'inventory_unit',''), 'kg') AS inventory_unit,
				COALESCE(NULLIF(
					oi.price_source_json->'inventory_conversion_json'
						->COALESCE(NULLIF(oi.sales_unit,''), NULLIF(oi.unit,''), oi.price_source_json->>'price_unit')
						->>COALESCE(NULLIF(oi.price_source_json->>'inventory_unit',''), 'kg'),
					'')::numeric, 0) AS conversion_to_inventory,
				GREATEST(COALESCE(NULLIF(oi.unit_bag_count,0), NULLIF(p.drip_box_bag_count,0), 1), 1)::numeric AS saved_bag_count,
				COALESCE(osd.decision,'') AS stock_decision
			FROM %s.order_items oi
			JOIN %s.orders o ON o.id = oi.order_id
			LEFT JOIN %s.products p ON p.id = oi.product_id
			LEFT JOIN %s.order_stock_decisions osd ON osd.order_id = o.id
			%s
		)
		, normalized_need AS (
			SELECT
				product_id,
				product,
				order_no,
				spec_g::bigint AS spec_g,
				qty,
				stock_decision,
				CASE
					WHEN conversion_to_inventory > 0 AND spec_g > 0 AND lower(inventory_unit) IN ('kg','kilogram','公斤','千克')
						THEN conversion_to_inventory * 1000 / spec_g
					WHEN conversion_to_inventory > 0 AND spec_g > 0 AND lower(inventory_unit) IN ('g','gram','克')
						THEN conversion_to_inventory / spec_g
					WHEN saved_bag_count > 1 THEN saved_bag_count
					ELSE 1
				END AS bag_count_per_sales_unit
			FROM line_need
		)
		, need AS (
			SELECT
				product_id,
				product,
				STRING_AGG(DISTINCT order_no, ',' ORDER BY order_no) AS order_nos,
				spec_g,
				CEIL(SUM(qty * bag_count_per_sales_unit))::bigint AS need_bags,
				SUM(CASE WHEN bag_count_per_sales_unit > 1 THEN qty ELSE 0 END)::bigint AS need_boxes,
				CEIL(SUM(CASE WHEN stock_decision = 'produce' THEN qty * bag_count_per_sales_unit ELSE 0 END))::bigint AS force_produce_bags
			FROM normalized_need
			GROUP BY product_id, product, spec_g
		)
		, reserved AS (
			SELECT product_id, spec_g, SUM(allocated_g)::bigint AS reserved_g
			FROM %s.order_stock_batch_allocations
			GROUP BY product_id, spec_g
		)
		SELECT
			n.product_id,
			n.product,
			COALESCE(n.order_nos,'') AS order_nos,
			n.spec_g,
			n.need_bags,
			n.need_boxes,
			(n.need_bags * n.spec_g)::bigint AS need_g,
			COALESCE(fi.onhand_units,0) AS inv_units,
			COALESCE(fi.onhand_loose_g,0) AS inv_loose_g,
			(COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0))::bigint AS inv_g,
			(
				(n.force_produce_bags * n.spec_g)
				+ GREATEST(
					0,
					((n.need_bags - n.force_produce_bags) * n.spec_g)
					- GREATEST(0, (COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0)) - COALESCE(reserved.reserved_g,0))
				)
			)::bigint AS gap_g
		FROM need n
		LEFT JOIN %s.finished_inventory fi
			ON fi.product_id = n.product_id AND fi.spec_g = n.spec_g AND fi.warehouse='finished_goods'
		LEFT JOIN reserved
			ON reserved.product_id = n.product_id AND reserved.spec_g = n.spec_g
		WHERE n.spec_g > 0
	`, r.schema, r.schema, r.schema, r.schema, where, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.UnprodNeedRow, 0)
	for rows.Next() {
		var row productionapp.UnprodNeedRow
		if err := rows.Scan(&row.ProductID, &row.Product, &row.OrderNos, &row.SpecG, &row.NeedUnits, &row.NeedBoxes, &row.NeedG, &row.InvUnits, &row.InvLooseG, &row.InvG, &row.GapG); err != nil {
			return nil, err
		}
		row.ProductionKind = "drip_bag"
		row.NeedBags = row.NeedUnits
		out = append(out, row)
	}
	return out, rows.Err()
}

func defaultPlanParams() planParams {
	return planParams{YieldRate: 1, DripExtraG: 100, DripBoxSpec: 10, EnableDripBox: true}
}

func isInstantCoffeePlanRow(row productionapp.UnprodNeedRow) bool {
	return strings.TrimSpace(row.ProductionKind) == "instant_coffee" ||
		catalogdomain.NormalizeProductKind(row.ProductionKind) == catalogdomain.ProductKindInstantCoffee ||
		strings.Contains(strings.TrimSpace(row.ProductTypeName), "速溶") ||
		strings.Contains(strings.TrimSpace(row.ProductSubtypeName), "速溶") ||
		strings.Contains(strings.TrimSpace(row.Product), "速溶")
}

func yieldRateForPlanRow(row productionapp.UnprodNeedRow, yieldByProductID map[int64]float64) float64 {
	return 1
}

func noBomRawMaterialName(row productionapp.UnprodNeedRow) string {
	if isInstantCoffeePlanRow(row) {
		return "速溶咖啡"
	}
	rawName := strings.TrimSpace(row.Product) + " 生豆"
	if strings.TrimSpace(rawName) == "生豆" {
		return "咖啡豆(生豆/原豆)"
	}
	return rawName
}

func buildProducePlanDisplayRows(rows []productionapp.UnprodNeedRow, yieldByProductID map[int64]float64, inputByKey map[string]int64) []productionapp.ProducePlanDisplayRow {
	out := make([]productionapp.ProducePlanDisplayRow, 0, len(rows))
	for _, r := range rows {
		yieldRate := yieldRateForPlanRow(r, yieldByProductID)
		inputG := r.GapG
		if v := inputByKey[producePlanDemandKey(r.ProductID, r.ParentProductID, r.SpecG, r.SalesSpecSnapshotJSON)]; v > 0 {
			inputG = v
		}
		out = append(out, productionapp.ProducePlanDisplayRow{UnprodNeedRow: r, BomYieldRate: yieldRate, InputG: inputG})
	}
	return out
}

type productionPlanBomSummary struct {
	MaterialLossRate float64
	InputG           int64
	BomVersionID     int64
	LegacyCompatible bool
	Error            string
}

func (r Repository) loadResolvedPlanBomSummaries(ctx context.Context, rows []productionapp.UnprodNeedRow) ([]productionPlanBomSummary, error) {
	out := make([]productionPlanBomSummary, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	for i, row := range rows {
		if row.ProductID <= 0 {
			continue
		}
		resolved, err := resolveProductionBomForDemandProductPreviewTx(
			ctx,
			tx,
			r.schema,
			row.ProductID,
			row.ParentProductID,
			row.Product,
		)
		if err != nil {
			if isProductionBomConfigurationError(err) {
				out[i].Error = err.Error()
				out[i].LegacyCompatible = isProductionBomNotConfiguredError(err)
				continue
			}
			return nil, err
		}
		out[i].MaterialLossRate = productionPlanBomMaterialLossRate(resolved)
		out[i].InputG = productionInputGFromBomMaterialLoss(row.GapG, out[i].MaterialLossRate)
		out[i].BomVersionID = resolved.BomVersionID
		if resolved.ProcessRouteID <= 0 || strings.TrimSpace(resolved.ProcessRouteName) == "" {
			out[i].Error = productionBomMissingRouteConfigurationError(resolved, row.Product).Error()
		}
	}
	return out, nil
}

func productionPlanBomMaterialLossRate(resolved latestUsableBomRoute) float64 {
	rate := resolved.BomMaterialLossRate
	if rate <= 0 || rate >= 1 {
		return 0
	}
	return rate
}

func buildRoastPlanRows(rows []productionapp.UnprodNeedRow, machines []productionapp.RoastMachine, yieldByProductID map[int64]float64, inputByKey map[string]int64) []productionapp.RoastPlanRow {
	out := make([]productionapp.RoastPlanRow, 0, len(rows))
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		yieldRate := yieldRateForPlanRow(r, yieldByProductID)
		rawG := inputByKey[producePlanDemandKey(r.ProductID, r.ParentProductID, r.SpecG, r.SalesSpecSnapshotJSON)]
		if rawG <= 0 {
			rawG = r.GapG
		}
		machine, batches := pickMachineAndBatches(rawG, machines)
		batchCount := int64(len(batches))
		batchG := int64(0)
		finalInputG := sum64(batches)
		if batchCount > 0 {
			batchG = batches[0]
		}
		if finalInputG <= 0 {
			finalInputG = rawG
		}
		if batchCount <= 0 {
			batchCount = 1
		}
		if batchG <= 0 {
			batchG = finalInputG
		}
		out = append(out, productionapp.RoastPlanRow{
			Key:                 producePlanKey(r.ProductID, r.SpecG),
			ProductID:           r.ProductID,
			ProductName:         r.Product,
			SpecG:               r.SpecG,
			Machine:             machine.Name,
			BatchCount:          batchCount,
			BatchG:              batchG,
			FinalInputG:         finalInputG,
			NeedG:               r.GapG,
			OperationTemplateID: r.OperationTemplateID,
			YieldRate:           yieldRate,
			YieldPctStr:         fmt.Sprintf("%.0f%%", yieldRate*100),
			FinishedKgStr:       formatKg(r.GapG),
		})
	}
	return out
}

func (r Repository) loadPlanBomItemsFromRows(ctx context.Context, rows []productionapp.UnprodNeedRow, summaries []productionPlanBomSummary) (map[string][]planBomItem, error) {
	out := map[string][]planBomItem{}
	legacyProductIDs := make([]int64, 0, len(rows))
	legacyDemandProductIDs := map[string]int64{}
	seenProducts := map[int64]bool{}
	seenDemands := map[string]bool{}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for i, row := range rows {
		demandKey := producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)
		if row.ProductID <= 0 || seenDemands[demandKey] {
			continue
		}
		seenDemands[demandKey] = true
		if i >= len(summaries) || summaries[i].BomVersionID <= 0 {
			if i >= len(summaries) || !summaries[i].LegacyCompatible {
				continue
			}
			legacyDemandProductIDs[demandKey] = row.ProductID
			if !seenProducts[row.ProductID] {
				seenProducts[row.ProductID] = true
				legacyProductIDs = append(legacyProductIDs, row.ProductID)
			}
			continue
		}
		snapshotJSON, err := buildMaterialSnapshotForBomVersionTx(
			ctx,
			tx,
			r.schema,
			ProduceRunRow{Product: row.Product, ProductID: row.ProductID},
			summaries[i].BomVersionID,
			false,
		)
		if err != nil {
			return nil, err
		}
		var snapshotRows []materialSnapshotRow
		if err := json.Unmarshal(snapshotJSON, &snapshotRows); err != nil {
			return nil, err
		}
		var roastLevel string
		var dripBoxBagCount int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(roast_level,''),COALESCE(NULLIF(drip_box_bag_count,0),10)
			FROM %s.products
			WHERE id=$1
		`, r.schema), row.ProductID).Scan(&roastLevel, &dripBoxBagCount); err != nil {
			return nil, err
		}
		for _, snapshot := range snapshotRows {
			out[demandKey] = append(out[demandKey], planBomItem{
				ProductID:          row.ProductID,
				RoastLevel:         roastLevel,
				YieldRate:          1,
				MaterialID:         snapshot.MaterialID,
				MaterialName:       snapshot.MaterialName,
				MaterialUnit:       snapshot.Unit,
				RatioPct:           snapshot.RatioPct,
				MaterialLossRate:   snapshot.MaterialLossRate,
				ComponentType:      snapshot.ComponentType,
				ComponentProductID: snapshot.ComponentProductID,
				ComponentSpecG:     snapshot.ComponentSpecG,
				ConsumeUnit:        snapshot.ConsumeUnit,
				QtyPerUnit:         snapshot.QtyPerUnit,
				OutputQty:          snapshot.OutputQty,
				OutputUnit:         snapshot.OutputUnit,
				DripBoxBagCount:    dripBoxBagCount,
			})
		}
	}
	if len(legacyProductIDs) > 0 {
		legacyRows, err := r.loadPlanBomItems(ctx, legacyProductIDs)
		if err != nil {
			return nil, err
		}
		for productID, items := range legacyRows {
			for demandKey, demandProductID := range legacyDemandProductIDs {
				if demandProductID == productID {
					out[demandKey] = items
				}
			}
		}
	}
	return out, nil
}

func (r Repository) loadPlanBomItems(ctx context.Context, productIDs []int64) (map[int64][]planBomItem, error) {
	out := map[int64][]planBomItem{}
	if len(productIDs) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`
		SELECT requested.product_id,
		       COALESCE(p.roast_level,''),
		       1::float8,
		       COALESCE(bi.material_id,0),
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0),
		       COALESCE(bi.material_loss_rate,0),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0),
		       COALESCE(pbv.output_qty,1)::float8,
		       COALESCE(NULLIF(pbv.output_unit,''),'unit'),
		       COALESCE(NULLIF(p.drip_box_bag_count,0),10)
		FROM unnest($1::bigint[]) AS requested(product_id)
		JOIN %s.products p ON p.id=requested.product_id AND p.active=true
		LEFT JOIN %s.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN LATERAL (
			SELECT latest.id AS bom_version_id
			FROM %s.production_boms pbom
			JOIN LATERAL (
				SELECT v.id, v.published_at, v.created_at
				FROM %s.production_bom_versions v
				WHERE v.bom_id=pbom.id
				  AND v.status='published'
				  AND EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)
				ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) latest ON true
			WHERE pbom.output_product_id=p.id
			  AND COALESCE(NULLIF(pbom.status,''),'active')='active'
			ORDER BY CASE WHEN pbom.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0) THEN 0 ELSE 1 END,
			         latest.published_at DESC NULLS LAST, latest.created_at DESC, latest.id DESC, pbom.id DESC
			LIMIT 1
		) output_bom ON true
		LEFT JOIN %s.production_bom_versions pbv ON pbv.id=output_bom.bom_version_id
		JOIN LATERAL (
			SELECT pbi.id, pbi.material_id, pbi.ratio_pct, pbi.material_loss_rate, pbi.component_type, pbi.component_product_id, pbi.component_spec_g, pbi.consume_unit, pbi.qty_per_unit
			FROM %s.production_bom_version_items pbi
			WHERE COALESCE(output_bom.bom_version_id,0)>0 AND pbi.version_id=output_bom.bom_version_id
			UNION ALL
			SELECT lbi.id, lbi.material_id, lbi.ratio_pct, 0 AS material_loss_rate, lbi.component_type, lbi.component_product_id, lbi.component_spec_g, lbi.consume_unit, lbi.qty_per_unit
			FROM %s.product_bom_items lbi
			WHERE COALESCE(output_bom.bom_version_id,0)=0 AND lbi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
		) bi ON true
		LEFT JOIN %s.product_bom pb ON pb.product_id=CASE
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
			ELSE p.id
		END
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		LEFT JOIN %s.products cp ON cp.id=bi.component_product_id
		ORDER BY requested.product_id, bi.id
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, productIDs)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item planBomItem
		if err := rows.Scan(
			&item.ProductID, &item.RoastLevel, &item.YieldRate,
			&item.MaterialID, &item.MaterialName, &item.MaterialUnit, &item.RatioPct, &item.MaterialLossRate,
			&item.ComponentType, &item.ComponentProductID, &item.ComponentProductName, &item.ComponentSpecG,
			&item.ConsumeUnit, &item.QtyPerUnit, &item.OutputQty, &item.OutputUnit, &item.DripBoxBagCount,
		); err != nil {
			return out, err
		}
		item.ComponentType = normalizeBomComponentType(item.ComponentType)
		item.ConsumeUnit = normalizeBomConsumeUnit(item.ConsumeUnit)
		item.MaterialLossRate = normalizeMaterialLossRate(item.MaterialLossRate)
		if item.ComponentType == "finished_product" {
			if item.ComponentProductID <= 0 || item.QtyPerUnit <= 0 {
				continue
			}
			item.MaterialName = strings.TrimSpace(item.ComponentProductName)
			if item.MaterialName == "" {
				item.MaterialName = fmt.Sprintf("finished product %d", item.ComponentProductID)
			}
			item.MaterialUnit = "g"
		} else if strings.TrimSpace(item.MaterialName) == "" || (item.RatioPct <= 0 && item.QtyPerUnit <= 0) {
			continue
		}
		item.RatioPct = bomdomain.NormalizeRatioPct(item.RatioPct)
		out[item.ProductID] = append(out[item.ProductID], item)
	}
	return out, rows.Err()
}

func buildRoastPlanMaterialRatios(rows []productionapp.UnprodNeedRow, bomMap map[string][]planBomItem) []productionapp.RoastPlanMaterialRatio {
	out := make([]productionapp.RoastPlanMaterialRatio, 0)
	for _, row := range rows {
		items := bomMap[producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)]
		if len(items) == 0 {
			out = append(out, productionapp.RoastPlanMaterialRatio{
				Key:          producePlanKey(row.ProductID, row.SpecG),
				ProductID:    row.ProductID,
				SpecG:        row.SpecG,
				ProductName:  row.Product,
				MaterialName: noBomRawMaterialName(row),
				MaterialUnit: "g",
				RatioPct:     100,
			})
			continue
		}
		for _, item := range items {
			out = append(out, productionapp.RoastPlanMaterialRatio{
				Key:              producePlanKey(row.ProductID, row.SpecG),
				ProductID:        row.ProductID,
				SpecG:            row.SpecG,
				ProductName:      row.Product,
				MaterialName:     item.MaterialName,
				MaterialUnit:     item.MaterialUnit,
				RatioPct:         bomdomain.NormalizeRatioPct(item.RatioPct),
				MaterialLossRate: item.MaterialLossRate,
			})
		}
	}
	return out
}

func calcProducePlanMaterialsFromFinalInputs(rows []productionapp.UnprodNeedRow, finalInputByKey map[string]int64, bomMap map[string][]planBomItem, p planParams) []productionapp.MaterialNeed {
	m := map[string]productionapp.MaterialNeed{}
	add := func(item productionapp.MaterialNeed) {
		name := strings.TrimSpace(item.Name)
		exactQty := item.ExactQty
		if exactQty <= 0 {
			exactQty = float64(item.Qty)
		}
		if exactQty <= 0 {
			return
		}
		key := materialAvailabilityKey(name, item.Unit)
		existing := m[key]
		if existing.Name == "" {
			existing = item
			existing.Name = name
			existing.Qty = 0
			existing.ExactQty = 0
		}
		existing.ExactQty += exactQty
		existing.Qty = int64(math.Ceil(existing.ExactQty))
		if existing.ComponentType == "" {
			existing.ComponentType = item.ComponentType
		}
		if existing.UpstreamProductID == 0 {
			existing.UpstreamProductID = item.UpstreamProductID
		}
		existing.UpstreamShortageG += item.UpstreamShortageG
		m[key] = existing
	}
	ceilDiv := func(a, b int64) int64 {
		if b <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}
	fallbackYield := 1.0

	for _, row := range rows {
		if row.GapG <= 0 || row.SpecG <= 0 {
			continue
		}
		demandKey := producePlanDemandKey(row.ProductID, row.ParentProductID, row.SpecG, row.SalesSpecSnapshotJSON)
		finalInputG, inputIncludesBomMaterialLoss := finalInputByKey[demandKey]
		items := bomMap[demandKey]
		if finalInputG <= 0 {
			if len(items) == 0 && isInstantCoffeePlanRow(row) {
				finalInputG = row.GapG
			}
			if finalInputG <= 0 {
				finalInputG = int64(math.Ceil(float64(row.GapG) / fallbackYield))
			}
		}
		if len(items) == 0 {
			noBom := row
			noBom.GapG = finalInputG
			for _, item := range calcNoBomProducePlanMaterials(noBom, p) {
				if item.Unit == "个" && strings.Contains(item.Name, "豆袋") {
					item.Qty = ceilDiv(row.GapG, row.SpecG)
				}
				add(item)
			}
			continue
		}

		unitsMissing := dripOrPackedUnitsMissing(row)
		for _, bom := range items {
			unit := strings.TrimSpace(bom.MaterialUnit)
			if unit == "" {
				unit = "g"
			}
			materialLossRate := bom.MaterialLossRate
			if inputIncludesBomMaterialLoss {
				materialLossRate = 0
			}
			qty := componentConsumptionQtyWithMaterialLoss(
				bom.ConsumeUnit, bom.QtyPerUnit, bom.RatioPct, unit, finalInputG, row.GapG,
				unitsMissing, dripBoxesMissing(row, bom.DripBoxBagCount), bom.OutputQty,
				bom.OutputUnit, materialLossRate,
			)
			exactQty := float64(qty)
			weightGrams := int64(0)
			if isWeightMaterialUnit(unit) {
				weightGrams = componentConsumptionWeightGramsWithMaterialLoss(
					bom.ConsumeUnit, bom.QtyPerUnit, bom.RatioPct, unit, finalInputG, row.GapG,
					unitsMissing, dripBoxesMissing(row, bom.DripBoxBagCount), bom.OutputQty,
					bom.OutputUnit, materialLossRate,
				)
				exactQty = float64(weightGrams) / productionWeightUnitGrams(unit)
				qty = int64(math.Ceil(exactQty))
			}
			if bom.ComponentType == "finished_product" {
				if weightGrams > 0 {
					exactQty = float64(weightGrams)
					qty = weightGrams
				}
				add(productionapp.MaterialNeed{
					Name:              bom.MaterialName,
					Qty:               qty,
					ExactQty:          exactQty,
					Unit:              "g",
					ComponentType:     "finished_product",
					UpstreamProductID: bom.ComponentProductID,
				})
			} else {
				add(productionapp.MaterialNeed{Name: bom.MaterialName, Qty: qty, ExactQty: exactQty, Unit: unit})
			}
		}

		bagName := "豆袋"
		if p.BagNameBySpecG != nil {
			if v := strings.TrimSpace(p.BagNameBySpecG[row.SpecG]); v != "" {
				bagName = v
			}
		}
		if row.ProductionKind != "drip_bag" {
			add(productionapp.MaterialNeed{Name: bagName, Qty: unitsMissing, Unit: "个"})
		}
	}

	out := make([]productionapp.MaterialNeed, 0, len(m))
	for _, item := range m {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Unit != out[j].Unit {
			return out[i].Unit < out[j].Unit
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (r Repository) attachDripUpstreamShortages(ctx context.Context, rows []productionapp.UnprodNeedRow, bomMap map[string][]planBomItem) error {
	for i := range rows {
		if rows[i].ProductionKind != "drip_bag" || rows[i].GapG <= 0 || rows[i].SpecG <= 0 {
			continue
		}
		bagsMissing := dripOrPackedUnitsMissing(rows[i])
		for _, item := range bomMap[producePlanDemandKey(rows[i].ProductID, rows[i].ParentProductID, rows[i].SpecG, rows[i].SalesSpecSnapshotJSON)] {
			if item.ComponentType != "finished_product" || item.ComponentProductID <= 0 {
				continue
			}
			demandG := componentDemandQty(item, rows[i].GapG, bagsMissing, dripBoxesMissing(rows[i], item.DripBoxBagCount), "g")
			if demandG <= 0 {
				continue
			}
			availableG, err := r.finishedProductAvailableG(ctx, item.ComponentProductID, item.ComponentSpecG)
			if err != nil {
				return err
			}
			shortageG := demandG - availableG
			if shortageG < 0 {
				shortageG = 0
			}
			rows[i].UpstreamProductID = item.ComponentProductID
			rows[i].UpstreamRoastDemandG += demandG
			rows[i].UpstreamShortageG += shortageG
			rows[i].FinishedProductComponentShortageG += shortageG
		}
	}
	return nil
}

func (r Repository) finishedProductAvailableG(ctx context.Context, productID, specG int64) (int64, error) {
	var totalG int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(onhand_units * $2 + onhand_loose_g),0)::bigint
		FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods'
	`, r.schema), productID, specG).Scan(&totalG)
	if err != nil {
		if strings.Contains(err.Error(), "finished_inventory") {
			return 0, nil
		}
		return 0, err
	}
	return totalG, nil
}

func dripOrPackedUnitsMissing(row productionapp.UnprodNeedRow) int64 {
	if row.SpecG <= 0 || row.GapG <= 0 {
		return 0
	}
	return ceilDiv64(row.GapG, row.SpecG)
}

func dripBoxesMissing(row productionapp.UnprodNeedRow, bagsPerBox int64) int64 {
	if row.ProductionKind != "drip_bag" {
		return 0
	}
	if row.NeedBoxes <= 0 {
		return 0
	}
	if bagsPerBox <= 0 {
		bagsPerBox = 10
	}
	return ceilDiv64(dripOrPackedUnitsMissing(row), bagsPerBox)
}

func componentDemandQty(item planBomItem, inputG, bagUnits, boxUnits int64, unit string) int64 {
	switch item.ConsumeUnit {
	case "g_per_bag", "unit_per_bag":
		return int64(math.Ceil(float64(bagUnits) * item.QtyPerUnit))
	case "unit_per_box":
		return int64(math.Ceil(float64(boxUnits) * item.QtyPerUnit))
	default:
		ratioPct := bomdomain.NormalizeRatioPct(item.RatioPct)
		if ratioPct <= 0 {
			return 0
		}
		if strings.EqualFold(unit, "kg") || unit == "千克" {
			return int64(math.Ceil((float64(inputG) * ratioPct / 100.0) / 1000.0))
		}
		return int64(math.Ceil(float64(inputG) * ratioPct / 100.0))
	}
}

func mergeMaterialAvailability(materials []productionapp.MaterialNeed, planRows []productionapp.MaterialPlanRow) []productionapp.MaterialNeed {
	if len(materials) == 0 {
		return materials
	}
	if len(planRows) == 0 {
		for i := range materials {
			applyNonWeightMaterialAvailabilityFallback(&materials[i], productionapp.MaterialPlanRow{})
		}
		return materials
	}
	availabilityByKey := map[string]productionapp.MaterialPlanRow{}
	for _, row := range planRows {
		availabilityByKey[materialAvailabilityKey(row.MaterialName, row.Unit)] = row
	}
	for i := range materials {
		row, ok := availabilityByKey[materialAvailabilityKey(materials[i].Name, materials[i].Unit)]
		if !ok {
			applyNonWeightMaterialAvailabilityFallback(&materials[i], productionapp.MaterialPlanRow{})
			continue
		}
		materials[i].WIPG = row.WIPG
		materials[i].AvailableG = row.AvailableG
		materials[i].RawG = row.RawG
		materials[i].ReservedG = row.ReservedG
		materials[i].WIPTransferSuggestionG = row.WIPTransferSuggestionG
		materials[i].ShortageG = row.ShortageG
		materials[i].PurchaseSuggestionG = row.PurchaseSuggestionG
		applyNonWeightMaterialAvailabilityFallback(&materials[i], row)
	}
	return materials
}

func applyNonWeightMaterialAvailabilityFallback(item *productionapp.MaterialNeed, row productionapp.MaterialPlanRow) {
	if item == nil || isWeightMaterialUnit(item.Unit) {
		return
	}
	requiredUnits := row.RequiredUnits
	if requiredUnits <= 0 {
		requiredUnits = item.Qty
	}
	if requiredUnits <= 0 {
		return
	}
	if item.ShortageG <= 0 {
		item.ShortageG = requiredUnits
	}
	if item.PurchaseSuggestionG <= 0 {
		item.PurchaseSuggestionG = item.ShortageG
	}
}

func materialAvailabilityKey(name string, unit string) string {
	return strings.TrimSpace(name) + "::" + strings.ToLower(strings.TrimSpace(unit))
}

func normalizeBomComponentType(value string) string {
	switch strings.TrimSpace(value) {
	case "finished_product", "product":
		return "finished_product"
	default:
		return "material"
	}
}

func normalizeBomConsumeUnit(value string) string {
	switch strings.TrimSpace(value) {
	case "ratio_pct", "g_per_bag", "unit_per_bag", "unit_per_box", "fixed_qty", "unit", "g", "kg", "length", "area":
		return strings.TrimSpace(value)
	default:
		return "ratio_pct"
	}
}

func calcNoBomProducePlanMaterials(row productionapp.UnprodNeedRow, p planParams) []productionapp.MaterialNeed {
	if row.GapG <= 0 || row.SpecG <= 0 {
		return nil
	}
	if p.DripBoxSpec <= 0 {
		p.DripBoxSpec = 10
	}
	ceilDiv := func(a, b int64) int64 {
		if b <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}
	unitsMissing := ceilDiv(row.GapG, row.SpecG)
	name := strings.TrimSpace(row.Product)
	if isInstantCoffeePlanRow(row) {
		return []productionapp.MaterialNeed{
			{Name: "速溶咖啡", Qty: row.GapG, Unit: "g"},
			{Name: "速溶-盒子", Qty: unitsMissing, Unit: "个"},
		}
	}
	if strings.Contains(name, "挂耳") {
		out := []productionapp.MaterialNeed{
			{Name: "咖啡豆(烘焙)", Qty: row.GapG, Unit: "g"},
			{Name: "挂耳-过滤袋", Qty: unitsMissing, Unit: "个"},
			{Name: "挂耳-卷膜", Qty: unitsMissing, Unit: "个"},
			{Name: "挂耳-封口贴", Qty: unitsMissing, Unit: "张"},
		}
		if p.EnableDripBox {
			out = append(out, productionapp.MaterialNeed{Name: "挂耳-盒彩", Qty: ceilDiv(unitsMissing, p.DripBoxSpec), Unit: "个"})
		}
		if p.DripExtraG > 0 {
			out = append(out, productionapp.MaterialNeed{Name: "咖啡豆(烘焙)", Qty: p.DripExtraG, Unit: "g"})
		}
		return out
	}
	out := []productionapp.MaterialNeed{{
		Name: noBomRawMaterialName(row),
		Qty:  row.GapG,
		Unit: "g",
	}}
	bagName := "豆袋"
	if p.BagNameBySpecG != nil {
		if v := strings.TrimSpace(p.BagNameBySpecG[row.SpecG]); v != "" {
			bagName = v
		}
	}
	out = append(out, productionapp.MaterialNeed{Name: bagName, Qty: unitsMissing, Unit: "个"})
	return out
}

func calcRoastSplits(rows []productionapp.UnprodNeedRow, machines []productionapp.RoastMachine, _ float64) []productionapp.RoastSplitRow {
	out := make([]productionapp.RoastSplitRow, 0)
	for _, row := range rows {
		if row.GapG <= 0 {
			continue
		}
		rowYieldRate := 1.0
		rawG := row.GapG
		pick, batches := pickMachineAndBatches(rawG, machines)
		yieldPct := fmt.Sprintf("%.0f%%", rowYieldRate*100)
		if pick.CapacityG <= 0 {
			out = append(out, productionapp.RoastSplitRow{Material: row.Product, Machine: "未匹配设备", BatchKg: "0", Batches: 0, TotalKg: formatKg(row.GapG), YieldPctStr: yieldPct})
			continue
		}
		out = append(out, productionapp.RoastSplitRow{
			Material:    row.Product,
			Machine:     pick.Name,
			BatchKg:     formatBatchPlanKg(batches),
			Batches:     int64(len(batches)),
			TotalKg:     formatKg(row.GapG),
			YieldPctStr: yieldPct,
		})
	}
	return out
}

func pickMachineAndBatches(totalG int64, machines []productionapp.RoastMachine) (productionapp.RoastMachine, []int64) {
	best := productionapp.RoastMachine{Name: "未匹配设备", CapacityG: 0, MinRoastG: 0}
	var bestBatches []int64
	pickList := machines
	if totalG > 0 && totalG < 20000 {
		filtered := make([]productionapp.RoastMachine, 0, len(machines))
		for _, machine := range machines {
			name := strings.ToLower(strings.TrimSpace(machine.Name))
			if strings.Contains(name, "布勒") || strings.Contains(name, "buhler") || strings.Contains(name, "bühler") {
				continue
			}
			filtered = append(filtered, machine)
		}
		pickList = filtered
	}
	for _, machine := range pickList {
		if machine.CapacityG <= 0 {
			continue
		}
		loads := machineLoadsG(machine)
		if len(loads) == 0 {
			continue
		}
		batches, ok := splitPreferEqual(totalG, loads)
		if !ok || len(batches) == 0 {
			continue
		}
		if len(bestBatches) == 0 || len(batches) < len(bestBatches) || (len(batches) == len(bestBatches) && sum64(batches) < sum64(bestBatches)) {
			best = machine
			bestBatches = batches
		}
	}
	return best, bestBatches
}

func machineLoadsG(machine productionapp.RoastMachine) []int64 {
	minG := max64(machine.MinRoastG, 1)
	maxG := machine.CapacityG
	if maxG <= 0 || minG > maxG {
		return nil
	}
	loads := make([]int64, 0)
	seen := map[int64]bool{}
	raw := strings.TrimSpace(machine.AllowedSpecs)
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			g, _ := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if g < minG || g > maxG || seen[g] {
				continue
			}
			seen[g] = true
			loads = append(loads, g)
		}
	}
	if len(loads) == 0 {
		start := ((minG + 999) / 1000) * 1000
		for g := start; g <= maxG; g += 1000 {
			loads = append(loads, g)
		}
	}
	sort.Slice(loads, func(i, j int) bool { return loads[i] < loads[j] })
	return loads
}

func splitPreferEqual(totalG int64, loads []int64) ([]int64, bool) {
	if totalG <= 0 || len(loads) == 0 {
		return nil, false
	}
	minLoad := loads[0]
	maxLoad := loads[len(loads)-1]
	if totalG <= maxLoad {
		for _, load := range loads {
			if load >= totalG {
				return []int64{load}, true
			}
		}
		return []int64{maxLoad}, true
	}

	minBatches := ceilDiv(totalG, maxLoad)
	maxBatches := ceilDiv(totalG, minLoad)
	for n := minBatches; n <= maxBatches; n++ {
		target := ceilDiv64(totalG, int64(n))
		load, ok := pickSmallestAtLeast(loads, target)
		if !ok {
			continue
		}
		batches := make([]int64, n)
		for i := range batches {
			batches[i] = load
		}
		return batches, true
	}
	return nil, false
}

func pickSmallestAtLeast(loads []int64, target int64) (int64, bool) {
	for _, load := range loads {
		if load >= target {
			return load, true
		}
	}
	return 0, false
}

func producePlanKey(productID, specG int64) string {
	return fmt.Sprintf("%d-%d", productID, specG)
}

func producePlanDemandKey(productID, parentProductID, specG int64, salesSpecSnapshotJSON string) string {
	return fmt.Sprintf("%d-%d-%d-%s", productID, parentProductID, specG, strings.TrimSpace(salesSpecSnapshotJSON))
}

func formatBatchPlanKg(batches []int64) string {
	if len(batches) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(batches))
	for _, batch := range batches {
		parts = append(parts, formatKg(batch))
	}
	return strings.Join(parts, " + ")
}

func formatKg(g int64) string {
	kg := float64(g) / 1000.0
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", kg), "0"), ".")
}

func sum64(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func ceilDiv(a, b int64) int {
	if b <= 0 {
		return 0
	}
	return int((a + b - 1) / b)
}

func ceilDiv64(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
