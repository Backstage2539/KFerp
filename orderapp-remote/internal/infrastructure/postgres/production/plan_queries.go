package production

import (
	"context"
	"fmt"
	"math"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"sort"
	"strconv"
	"strings"

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
	ComponentType        string
	ComponentProductID   int64
	ComponentProductName string
	ComponentSpecG       int64
	ConsumeUnit          string
	QtyPerUnit           float64
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
	appRows := unprodRowsToApp(rows)
	dripRows, err := r.fetchDripPlanNeeds(ctx, query.From, query.To, query.CustomerID)
	if err != nil {
		return data, err
	}
	appRows = mergeDripPlanRows(appRows, dripRows)
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
		if row.GapG <= 0 {
			continue
		}
		planRows = append(planRows, row)
	}
	if selectedCount > 0 && len(planRows) == 0 {
		data.StockTip = "库存充足：当前已选商品库存均可满足；录单确认使用成品批次后会进入库存待发货，可直接发货。"
		return data, nil
	}

	yieldMap, err := r.loadProductYieldRateMap(ctx)
	if err != nil {
		return data, err
	}
	params := defaultPlanParams()
	if mappings, err := postgresinfra.ListBagSpecMappings(ctx, r.pool, r.schema); err == nil {
		params.BagNameBySpecG = bomdomain.MappingNameBySpec(mappings)
	}
	bomMap, _ := r.loadPlanBomItemsFromRows(ctx, planRows)
	machines, _ := r.ListMachines(ctx, true)
	if err := r.attachDripUpstreamShortages(ctx, planRows, bomMap); err != nil {
		return data, err
	}
	data.RoastPlans = buildRoastPlanRows(planRows, machines, yieldMap)
	data.MaterialRatios = buildRoastPlanMaterialRatios(planRows, bomMap)
	finalInputByKey := map[string]int64{}
	for _, row := range data.RoastPlans {
		finalInputByKey[row.Key] = row.FinalInputG
	}
	data.PlanRows = buildProducePlanDisplayRows(planRows, yieldMap, finalInputByKey)
	data.Materials = calcProducePlanMaterialsFromFinalInputs(planRows, finalInputByKey, bomMap, params)
	if materialPlan, err := r.MaterialPlan(ctx, productionapp.MaterialPlanQuery{
		From:       query.From,
		To:         query.To,
		CustomerID: query.CustomerID,
		Selected:   query.Selected,
		InputByKey: finalInputByKey,
	}); err == nil {
		data.Materials = mergeMaterialAvailability(data.Materials, materialPlan.Rows)
	}
	data.RoastSplits = calcRoastSplits(planRows, machines, params.YieldRate)
	return data, nil
}

func unprodRowsToApp(rows []UnprodNeedRow) []productionapp.UnprodNeedRow {
	out := make([]productionapp.UnprodNeedRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, productionapp.UnprodNeedRow{
			ProductID:                row.ProductID,
			Product:                  row.Product,
			OrderNos:                 row.OrderNos,
			SpecG:                    row.SpecG,
			NeedUnits:                row.NeedUnits,
			NeedG:                    row.NeedG,
			InvUnits:                 row.InvUnits,
			InvLooseG:                row.InvLooseG,
			InvG:                     row.InvG,
			GapG:                     row.GapG,
			ProductionKind:           catalogdomain.NormalizeProductKind(row.ProductionKind),
			ProductTypeCategoryID:    row.ProductTypeCategoryID,
			ProductSubtypeCategoryID: row.ProductSubtypeCategoryID,
			ProductTypeName:          row.ProductTypeName,
			ProductSubtypeName:       row.ProductSubtypeName,
			OperationTemplateID:      row.OperationTemplateID,
		})
	}
	return out
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
		WITH need AS (
			SELECT
				oi.product_id,
				COALESCE(p.name,'') AS product,
				STRING_AGG(DISTINCT COALESCE(o.order_no,''), ',' ORDER BY COALESCE(o.order_no,'')) AS order_nos,
				COALESCE(NULLIF(oi.unit_bean_g,0), NULLIF(p.drip_bag_grams,0), NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), '')::numeric, 0)::bigint AS spec_g,
				SUM(
					CASE WHEN COALESCE(NULLIF(oi.sales_unit,''), lower(oi.unit), '') = 'box'
						THEN COALESCE(oi.qty,0) * GREATEST(COALESCE(NULLIF(oi.unit_bag_count,0), NULLIF(p.drip_box_bag_count,0), 1), 1)
						ELSE COALESCE(oi.qty,0)
					END
				)::bigint AS need_bags,
				SUM(
					CASE WHEN COALESCE(NULLIF(oi.sales_unit,''), lower(oi.unit), '') = 'box'
						THEN COALESCE(oi.qty,0)
						ELSE 0
					END
				)::bigint AS need_boxes,
				SUM(
					CASE WHEN COALESCE(osd.decision,'') = 'produce' THEN
						CASE WHEN COALESCE(NULLIF(oi.sales_unit,''), lower(oi.unit), '') = 'box'
							THEN COALESCE(oi.qty,0) * GREATEST(COALESCE(NULLIF(oi.unit_bag_count,0), NULLIF(p.drip_box_bag_count,0), 1), 1)
							ELSE COALESCE(oi.qty,0)
						END
					ELSE 0 END
				)::bigint AS force_produce_bags
			FROM %s.order_items oi
			JOIN %s.orders o ON o.id = oi.order_id
			LEFT JOIN %s.products p ON p.id = oi.product_id
			LEFT JOIN %s.order_stock_decisions osd ON osd.order_id = o.id
			%s
			GROUP BY oi.product_id, p.name, COALESCE(NULLIF(oi.unit_bean_g,0), NULLIF(p.drip_bag_grams,0), NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), '')::numeric, 0)
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
	return planParams{YieldRate: 0.8, DripExtraG: 100, DripBoxSpec: 10, EnableDripBox: true}
}

func isInstantCoffeePlanRow(row productionapp.UnprodNeedRow) bool {
	return strings.TrimSpace(row.ProductionKind) == "instant_coffee" ||
		catalogdomain.NormalizeProductKind(row.ProductionKind) == catalogdomain.ProductKindInstantCoffee ||
		strings.Contains(strings.TrimSpace(row.ProductTypeName), "速溶") ||
		strings.Contains(strings.TrimSpace(row.ProductSubtypeName), "速溶") ||
		strings.Contains(strings.TrimSpace(row.Product), "速溶")
}

func yieldRateForPlanRow(row productionapp.UnprodNeedRow, yieldByProductID map[int64]float64) float64 {
	return normalizeYieldRate(yieldByProductID[row.ProductID])
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

func (r Repository) loadProductYieldRateMap(ctx context.Context) (map[int64]float64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.roast_level,''),
		       COALESCE(
		           CASE WHEN ppc.product_id IS NOT NULL THEN 1 - COALESCE(NULLIF(ppc.expected_loss_rate,0), 0) ELSE NULL END,
		           NULLIF(pbv.yield_rate,0),
		           NULLIF(b.yield_rate,0),
		           CASE WHEN COALESCE(NULLIF(p.product_kind,''),'roasted_bean')='instant_coffee' THEN 1 ELSE 0.8 END
		       ),
		       COALESCE(NULLIF(p.product_kind,''),'roasted_bean')
		FROM `+r.schema+`.products p
		LEFT JOIN `+r.schema+`.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN `+r.schema+`.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN `+r.schema+`.production_bom_versions pbv ON pbv.id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id)
		LEFT JOIN `+r.schema+`.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN `+r.schema+`.product_bom b ON b.product_id=CASE
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
			ELSE p.id
		END
		WHERE p.active=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]float64{}
	for rows.Next() {
		var productID int64
		var roastLevel string
		var yieldRate float64
		var productKind string
		if err := rows.Scan(&productID, &roastLevel, &yieldRate, &productKind); err != nil {
			return nil, err
		}
		out[productID] = catalogdomain.ResolveYieldRate(roastLevel, yieldRate)
	}
	return out, rows.Err()
}

func buildProducePlanDisplayRows(rows []productionapp.UnprodNeedRow, yieldByProductID map[int64]float64, inputByKey map[string]int64) []productionapp.ProducePlanDisplayRow {
	out := make([]productionapp.ProducePlanDisplayRow, 0, len(rows))
	for _, r := range rows {
		yieldRate := yieldRateForPlanRow(r, yieldByProductID)
		inputG := defaultProductionInputG(r.GapG, yieldRate)
		if v := inputByKey[producePlanKey(r.ProductID, r.SpecG)]; v > 0 {
			inputG = v
		}
		out = append(out, productionapp.ProducePlanDisplayRow{UnprodNeedRow: r, BomYieldRate: yieldRate, InputG: inputG})
	}
	return out
}

func buildRoastPlanRows(rows []productionapp.UnprodNeedRow, machines []productionapp.RoastMachine, yieldByProductID map[int64]float64) []productionapp.RoastPlanRow {
	out := make([]productionapp.RoastPlanRow, 0, len(rows))
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		yieldRate := yieldRateForPlanRow(r, yieldByProductID)
		rawG := defaultProductionInputG(r.GapG, yieldRate)
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

func (r Repository) loadPlanBomItemsFromRows(ctx context.Context, rows []productionapp.UnprodNeedRow) (map[int64][]planBomItem, error) {
	productIDs := make([]int64, 0, len(rows))
	seen := map[int64]bool{}
	for _, row := range rows {
		if row.ProductID > 0 && !seen[row.ProductID] {
			seen[row.ProductID] = true
			productIDs = append(productIDs, row.ProductID)
		}
	}
	return r.loadPlanBomItems(ctx, productIDs)
}

func (r Repository) loadPlanBomItems(ctx context.Context, productIDs []int64) (map[int64][]planBomItem, error) {
	out := map[int64][]planBomItem{}
	if len(productIDs) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`
		SELECT requested.product_id,
		       COALESCE(p.roast_level,''),
		       COALESCE(pbv.yield_rate, pb.yield_rate, 0),
		       COALESCE(bi.material_id,0),
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0),
		       COALESCE(NULLIF(p.drip_box_bag_count,0),10)
		FROM unnest($1::bigint[]) AS requested(product_id)
		JOIN %s.products p ON p.id=requested.product_id AND p.active=true
		LEFT JOIN %s.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN %s.production_bom_versions pbv ON pbv.id=pbb.bom_version_id
		JOIN LATERAL (
			SELECT pbi.id, pbi.material_id, pbi.ratio_pct, pbi.component_type, pbi.component_product_id, pbi.component_spec_g, pbi.consume_unit, pbi.qty_per_unit
			FROM %s.production_bom_version_items pbi
			WHERE pbb.product_id IS NOT NULL AND pbi.version_id=pbb.bom_version_id
			UNION ALL
			SELECT lbi.id, lbi.material_id, lbi.ratio_pct, lbi.component_type, lbi.component_product_id, lbi.component_spec_g, lbi.consume_unit, lbi.qty_per_unit
			FROM %s.product_bom_items lbi
			WHERE pbb.product_id IS NULL AND lbi.product_id=CASE
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
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, productIDs)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item planBomItem
		if err := rows.Scan(
			&item.ProductID, &item.RoastLevel, &item.YieldRate,
			&item.MaterialID, &item.MaterialName, &item.MaterialUnit, &item.RatioPct,
			&item.ComponentType, &item.ComponentProductID, &item.ComponentProductName, &item.ComponentSpecG,
			&item.ConsumeUnit, &item.QtyPerUnit, &item.DripBoxBagCount,
		); err != nil {
			return out, err
		}
		item.ComponentType = normalizeBomComponentType(item.ComponentType)
		item.ConsumeUnit = normalizeBomConsumeUnit(item.ConsumeUnit)
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

func buildRoastPlanMaterialRatios(rows []productionapp.UnprodNeedRow, bomMap map[int64][]planBomItem) []productionapp.RoastPlanMaterialRatio {
	out := make([]productionapp.RoastPlanMaterialRatio, 0)
	for _, row := range rows {
		items := bomMap[row.ProductID]
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
				Key:          producePlanKey(row.ProductID, row.SpecG),
				ProductID:    row.ProductID,
				SpecG:        row.SpecG,
				ProductName:  row.Product,
				MaterialName: item.MaterialName,
				MaterialUnit: item.MaterialUnit,
				RatioPct:     bomdomain.NormalizeRatioPct(item.RatioPct),
			})
		}
	}
	return out
}

func calcProducePlanMaterialsFromFinalInputs(rows []productionapp.UnprodNeedRow, finalInputByKey map[string]int64, bomMap map[int64][]planBomItem, p planParams) []productionapp.MaterialNeed {
	m := map[string]productionapp.MaterialNeed{}
	add := func(item productionapp.MaterialNeed) {
		name := strings.TrimSpace(item.Name)
		if item.Qty <= 0 {
			return
		}
		key := materialAvailabilityKey(name, item.Unit)
		existing := m[key]
		if existing.Name == "" {
			existing = item
			existing.Name = name
			existing.Qty = 0
		}
		existing.Qty += item.Qty
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
	fallbackYield := p.YieldRate
	if fallbackYield <= 0 || fallbackYield > 1 {
		fallbackYield = 0.8
	}

	for _, row := range rows {
		if row.GapG <= 0 || row.SpecG <= 0 {
			continue
		}
		finalInputG := finalInputByKey[producePlanKey(row.ProductID, row.SpecG)]
		items := bomMap[row.ProductID]
		if finalInputG <= 0 {
			if len(items) == 0 && isInstantCoffeePlanRow(row) {
				finalInputG = row.GapG
			}
			yield := fallbackYield
			if len(items) > 0 && items[0].YieldRate > 0 && items[0].YieldRate <= 1 {
				yield = items[0].YieldRate
			}
			if finalInputG <= 0 {
				finalInputG = int64(math.Ceil(float64(row.GapG) / yield))
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
			ratioPct := bomdomain.NormalizeRatioPct(bom.RatioPct)
			qty := int64(0)
			switch {
			case bom.ConsumeUnit == "g_per_bag":
				qty = int64(math.Ceil(float64(unitsMissing) * bom.QtyPerUnit))
			case bom.ConsumeUnit == "unit_per_bag":
				qty = int64(math.Ceil(float64(unitsMissing) * bom.QtyPerUnit))
			case bom.ConsumeUnit == "unit_per_box":
				qty = int64(math.Ceil(float64(dripBoxesMissing(row, bom.DripBoxBagCount)) * bom.QtyPerUnit))
			case strings.EqualFold(unit, "g"):
				qty = int64(math.Ceil(float64(finalInputG) * ratioPct / 100.0))
			case strings.EqualFold(unit, "kg"):
				qty = int64(math.Ceil((float64(finalInputG) * ratioPct / 100.0) / 1000.0))
			default:
				qty = int64(math.Ceil(float64(unitsMissing) * ratioPct / 100.0))
			}
			if bom.ComponentType == "finished_product" {
				add(productionapp.MaterialNeed{
					Name:              bom.MaterialName,
					Qty:               qty,
					Unit:              "g",
					ComponentType:     "finished_product",
					UpstreamProductID: bom.ComponentProductID,
				})
			} else {
				add(productionapp.MaterialNeed{Name: bom.MaterialName, Qty: qty, Unit: unit})
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

func (r Repository) attachDripUpstreamShortages(ctx context.Context, rows []productionapp.UnprodNeedRow, bomMap map[int64][]planBomItem) error {
	for i := range rows {
		if rows[i].ProductionKind != "drip_bag" || rows[i].GapG <= 0 || rows[i].SpecG <= 0 {
			continue
		}
		bagsMissing := dripOrPackedUnitsMissing(rows[i])
		for _, item := range bomMap[rows[i].ProductID] {
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
	if len(materials) == 0 || len(planRows) == 0 {
		return materials
	}
	availabilityByKey := map[string]productionapp.MaterialPlanRow{}
	for _, row := range planRows {
		availabilityByKey[materialAvailabilityKey(row.MaterialName, row.Unit)] = row
	}
	for i := range materials {
		row, ok := availabilityByKey[materialAvailabilityKey(materials[i].Name, materials[i].Unit)]
		if !ok {
			continue
		}
		materials[i].WIPG = row.WIPG
		materials[i].AvailableG = row.AvailableG
		materials[i].RawG = row.RawG
		materials[i].ReservedG = row.ReservedG
		materials[i].WIPTransferSuggestionG = row.WIPTransferSuggestionG
		materials[i].ShortageG = row.ShortageG
		materials[i].PurchaseSuggestionG = row.PurchaseSuggestionG
	}
	return materials
}

func materialAvailabilityKey(name string, unit string) string {
	return strings.TrimSpace(name) + "::" + strings.ToLower(strings.TrimSpace(unit))
}

func normalizeBomComponentType(value string) string {
	if strings.TrimSpace(value) == "finished_product" {
		return "finished_product"
	}
	return "material"
}

func normalizeBomConsumeUnit(value string) string {
	switch strings.TrimSpace(value) {
	case "g_per_bag", "unit_per_bag", "unit_per_box":
		return strings.TrimSpace(value)
	default:
		return "ratio_pct"
	}
}

func calcNoBomProducePlanMaterials(row productionapp.UnprodNeedRow, p planParams) []productionapp.MaterialNeed {
	if row.GapG <= 0 || row.SpecG <= 0 {
		return nil
	}
	if p.YieldRate <= 0 || p.YieldRate > 1 {
		p.YieldRate = 0.8
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
			{Name: "咖啡豆(烘焙)", Qty: int64(math.Ceil(float64(row.GapG) / p.YieldRate)), Unit: "g"},
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
		Qty:  int64(math.Ceil(float64(row.GapG) / p.YieldRate)),
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

func calcRoastSplits(rows []productionapp.UnprodNeedRow, machines []productionapp.RoastMachine, yieldRate float64) []productionapp.RoastSplitRow {
	if yieldRate <= 0 || yieldRate > 1 {
		yieldRate = 0.8
	}
	out := make([]productionapp.RoastSplitRow, 0)
	for _, row := range rows {
		if row.GapG <= 0 {
			continue
		}
		rowYieldRate := yieldRate
		rawG := int64(math.Ceil(float64(row.GapG) / rowYieldRate))
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
