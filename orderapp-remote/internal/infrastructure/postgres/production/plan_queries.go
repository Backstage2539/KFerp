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
	ProductID    int64
	RoastLevel   string
	YieldRate    float64
	MaterialName string
	MaterialUnit string
	RatioPct     float64
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
			ProductID: row.ProductID,
			Product:   row.Product,
			OrderNos:  row.OrderNos,
			SpecG:     row.SpecG,
			NeedUnits: row.NeedUnits,
			NeedG:     row.NeedG,
			InvUnits:  row.InvUnits,
			InvLooseG: row.InvLooseG,
			InvG:      row.InvG,
			GapG:      row.GapG,
		})
	}
	return out
}

func defaultPlanParams() planParams {
	return planParams{YieldRate: 0.8, DripExtraG: 100, DripBoxSpec: 10, EnableDripBox: true}
}

func (r Repository) loadProductYieldRateMap(ctx context.Context) (map[int64]float64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.roast_level,''), COALESCE(b.yield_rate,0.8)
		FROM `+r.schema+`.products p
		LEFT JOIN `+r.schema+`.product_bom b ON b.product_id=p.id
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
		if err := rows.Scan(&productID, &roastLevel, &yieldRate); err != nil {
			return nil, err
		}
		out[productID] = catalogdomain.ResolveYieldRate(roastLevel, yieldRate)
	}
	return out, rows.Err()
}

func buildProducePlanDisplayRows(rows []productionapp.UnprodNeedRow, yieldByProductID map[int64]float64, inputByKey map[string]int64) []productionapp.ProducePlanDisplayRow {
	out := make([]productionapp.ProducePlanDisplayRow, 0, len(rows))
	for _, r := range rows {
		yieldRate := normalizeYieldRate(yieldByProductID[r.ProductID])
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
		yieldRate := normalizeYieldRate(yieldByProductID[r.ProductID])
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
			Key:           producePlanKey(r.ProductID, r.SpecG),
			ProductID:     r.ProductID,
			ProductName:   r.Product,
			SpecG:         r.SpecG,
			Machine:       machine.Name,
			BatchCount:    batchCount,
			BatchG:        batchG,
			FinalInputG:   finalInputG,
			NeedG:         r.GapG,
			YieldRate:     yieldRate,
			YieldPctStr:   fmt.Sprintf("%.0f%%", yieldRate*100),
			FinishedKgStr: formatKg(r.GapG),
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
		SELECT bi.product_id,
		       COALESCE(p.roast_level,''),
		       COALESCE(pb.yield_rate,0),
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0)
		FROM %s.product_bom_items bi
		LEFT JOIN %s.products p ON p.id=bi.product_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=bi.product_id
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		WHERE bi.product_id = ANY($1)
		ORDER BY bi.product_id, bi.id
	`, r.schema, r.schema, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, productIDs)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item planBomItem
		if err := rows.Scan(&item.ProductID, &item.RoastLevel, &item.YieldRate, &item.MaterialName, &item.MaterialUnit, &item.RatioPct); err != nil {
			return out, err
		}
		if strings.TrimSpace(item.MaterialName) == "" || item.RatioPct <= 0 {
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
			rawName := strings.TrimSpace(row.Product) + " 生豆"
			if strings.TrimSpace(rawName) == "生豆" {
				rawName = "咖啡豆(生豆/原豆)"
			}
			out = append(out, productionapp.RoastPlanMaterialRatio{
				Key:          producePlanKey(row.ProductID, row.SpecG),
				ProductID:    row.ProductID,
				SpecG:        row.SpecG,
				ProductName:  row.Product,
				MaterialName: rawName,
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
	add := func(name string, qty int64, unit string) {
		if qty <= 0 {
			return
		}
		item := m[name]
		item.Name = name
		item.Unit = unit
		item.Qty += qty
		m[name] = item
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
			yield := fallbackYield
			if len(items) > 0 && items[0].YieldRate > 0 && items[0].YieldRate <= 1 {
				yield = items[0].YieldRate
			}
			finalInputG = int64(math.Ceil(float64(row.GapG) / yield))
		}
		if len(items) == 0 {
			noBom := row
			noBom.GapG = finalInputG
			for _, item := range calcNoBomProducePlanMaterials(noBom, p) {
				if item.Unit == "个" && strings.Contains(item.Name, "豆袋") {
					item.Qty = ceilDiv(row.GapG, row.SpecG)
				}
				add(item.Name, item.Qty, item.Unit)
			}
			continue
		}

		unitsMissing := ceilDiv(row.GapG, row.SpecG)
		for _, bom := range items {
			unit := strings.TrimSpace(bom.MaterialUnit)
			if unit == "" {
				unit = "g"
			}
			ratioPct := bomdomain.NormalizeRatioPct(bom.RatioPct)
			switch {
			case strings.EqualFold(unit, "g"):
				add(bom.MaterialName, int64(math.Ceil(float64(finalInputG)*ratioPct/100.0)), "g")
			case strings.EqualFold(unit, "kg"):
				add(bom.MaterialName, int64(math.Ceil((float64(finalInputG)*ratioPct/100.0)/1000.0)), "kg")
			default:
				add(bom.MaterialName, int64(math.Ceil(float64(unitsMissing)*ratioPct/100.0)), unit)
			}
		}

		bagName := "豆袋"
		if p.BagNameBySpecG != nil {
			if v := strings.TrimSpace(p.BagNameBySpecG[row.SpecG]); v != "" {
				bagName = v
			}
		}
		add(bagName, unitsMissing, "个")
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
	if strings.Contains(name, "速溶") {
		return []productionapp.MaterialNeed{{Name: "速溶-盒子", Qty: unitsMissing, Unit: "个"}}
	}

	rawName := name + " 生豆"
	if strings.TrimSpace(rawName) == "生豆" {
		rawName = "咖啡豆(生豆/原豆)"
	}
	out := []productionapp.MaterialNeed{{
		Name: rawName,
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
		rawG := int64(math.Ceil(float64(row.GapG) / yieldRate))
		pick, batches := pickMachineAndBatches(rawG, machines)
		yieldPct := fmt.Sprintf("%.0f%%", yieldRate*100)
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
