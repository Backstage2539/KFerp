package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type UnprodSummaryPageData struct {
	From           string                   `json:"from"`
	To             string                   `json:"to"`
	CustomerID     int64                    `json:"customer_id"`
	Rows           []UnprodNeedRow          `json:"rows"`
	PlanRows       []ProducePlanDisplayRow  `json:"plan_rows"`
	Materials      []MaterialNeed           `json:"materials"`
	RoastSplits    []RoastSplitRow          `json:"roast_splits"`
	RoastPlans     []RoastPlanRow           `json:"roast_plans"`
	MaterialRatios []RoastPlanMaterialRatio `json:"material_ratios"`
	Selected       map[string]bool          `json:"selected"`
	PlanReady      bool                     `json:"plan_ready"`
	StockTip       string                   `json:"stock_tip"`
	Error          string                   `json:"error"`
}

type ProducePlanDisplayRow struct {
	UnprodNeedRow
	BomYieldRate float64 `json:"bom_yield_rate"`
	InputG       int64   `json:"input_g"`
}

type UnprodSummaryQuery struct {
	From       string
	To         string
	CustomerID int64
	Selected   map[string]bool
	Plan       bool
}

type RoastPlanRow struct {
	Key           string  `json:"key"`
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	SpecG         int64   `json:"spec_g"`
	Machine       string  `json:"machine"`
	BatchCount    int64   `json:"batch_count"`
	BatchG        int64   `json:"batch_g"`
	FinalInputG   int64   `json:"final_input_g"`
	NeedG         int64   `json:"need_g"`
	YieldRate     float64 `json:"yield_rate"`
	YieldPctStr   string  `json:"yield_pct_str"`
	FinishedKgStr string  `json:"finished_kg_str"`
}

type RoastPlanMaterialRatio struct {
	Key          string  `json:"key"`
	ProductID    int64   `json:"product_id"`
	SpecG        int64   `json:"spec_g"`
	ProductName  string  `json:"product_name"`
	MaterialName string  `json:"material_name"`
	MaterialUnit string  `json:"material_unit"`
	RatioPct     float64 `json:"ratio_pct"`
}

func buildProducePlanDisplayRows(rows []UnprodNeedRow, yieldByProductID map[int64]float64, inputByKey map[string]int64) []ProducePlanDisplayRow {
	out := make([]ProducePlanDisplayRow, 0, len(rows))
	for _, r := range rows {
		yieldRate := normalizeYieldRate(yieldByProductID[r.ProductID])
		inputG := defaultProductionInputG(r.GapG, yieldRate)
		if v := inputByKey[producePlanKey(r.ProductID, r.SpecG)]; v > 0 {
			inputG = v
		}
		out = append(out, ProducePlanDisplayRow{
			UnprodNeedRow: r,
			BomYieldRate:  yieldRate,
			InputG:        inputG,
		})
	}
	return out
}

func loadProductYieldRateMap(ctx context.Context, pool *pgxpool.Pool, schema string) (map[int64]float64, error) {
	rows, err := pool.Query(ctx, "SELECT product_id, COALESCE(yield_rate,0.8) FROM "+schema+".product_bom")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]float64{}
	for rows.Next() {
		var productID int64
		var yieldRate float64
		if err := rows.Scan(&productID, &yieldRate); err != nil {
			return nil, err
		}
		out[productID] = normalizeYieldRate(yieldRate)
	}
	return out, rows.Err()
}

func buildRoastPlanRows(rows []UnprodNeedRow, machines []RoastMachine, yieldByProductID map[int64]float64) []RoastPlanRow {
	out := make([]RoastPlanRow, 0, len(rows))
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
		out = append(out, RoastPlanRow{
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

func parseUnprodSummaryQuery(c echo.Context) UnprodSummaryQuery {
	q := UnprodSummaryQuery{
		From:     strings.TrimSpace(c.QueryParam("from")),
		To:       strings.TrimSpace(c.QueryParam("to")),
		Selected: map[string]bool{},
		Plan:     strings.TrimSpace(c.QueryParam("plan")) == "1",
	}
	if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			q.CustomerID = n
		}
	}
	if sel := strings.TrimSpace(c.QueryParam("selected")); sel != "" {
		for _, k := range strings.Split(sel, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				q.Selected[k] = true
			}
		}
	}
	return q
}

func loadUnprodSummaryData(ctx context.Context, pool *pgxpool.Pool, schema string, q UnprodSummaryQuery) (UnprodSummaryPageData, error) {
	data := UnprodSummaryPageData{
		From:       q.From,
		To:         q.To,
		CustomerID: q.CustomerID,
		Selected:   q.Selected,
		PlanReady:  q.Plan && len(q.Selected) > 0,
	}
	rows, err := fetchUnproducedNeeds(ctx, pool, schema, q.From, q.To, q.CustomerID)
	if err != nil {
		return data, err
	}
	data.Rows = rows
	if !data.PlanReady {
		return data, nil
	}

	planRows := make([]UnprodNeedRow, 0)
	selectedCount := 0
	for _, r := range rows {
		key := producePlanKey(r.ProductID, r.SpecG)
		if !q.Selected[key] {
			continue
		}
		selectedCount++
		if r.GapG <= 0 {
			continue
		}
		planRows = append(planRows, r)
	}
	if selectedCount > 0 && len(planRows) == 0 {
		data.StockTip = "库存充足：当前已选商品库存均可满足，无需补产。"
		return data, nil
	}

	yieldMap, err := loadProductYieldRateMap(ctx, pool, schema)
	if err != nil {
		return data, err
	}
	params := defaultProducePlanParams()
	if mappings, err := listBagSpecMappings(ctx, pool, schema); err == nil {
		params.BagNameBySpecG = mappingNameBySpec(mappings)
	}
	bomMap, _ := loadBomNeedItemsFromRows(ctx, pool, schema, planRows)
	machines, _ := loadActiveMachines(ctx, pool, schema)
	data.RoastPlans = buildRoastPlanRows(planRows, machines, yieldMap)
	data.MaterialRatios = buildRoastPlanMaterialRatios(planRows, bomMap)
	finalInputByKey := map[string]int64{}
	for _, row := range data.RoastPlans {
		finalInputByKey[row.Key] = row.FinalInputG
	}
	data.PlanRows = buildProducePlanDisplayRows(planRows, yieldMap, finalInputByKey)
	data.Materials = calcProducePlanMaterialsFromFinalInputs(planRows, finalInputByKey, bomMap, params)
	data.RoastSplits = calcRoastSplits(planRows, machines, params.YieldRate)
	return data, nil
}

func loadBomNeedItemsFromRows(ctx context.Context, pool *pgxpool.Pool, schema string, rows []UnprodNeedRow) (map[int64][]bomNeedItem, error) {
	productIDs := make([]int64, 0, len(rows))
	seen := map[int64]bool{}
	for _, row := range rows {
		if row.ProductID > 0 && !seen[row.ProductID] {
			seen[row.ProductID] = true
			productIDs = append(productIDs, row.ProductID)
		}
	}
	return loadBomNeedItems(ctx, pool, schema, productIDs)
}

func buildRoastPlanMaterialRatios(rows []UnprodNeedRow, bomMap map[int64][]bomNeedItem) []RoastPlanMaterialRatio {
	out := make([]RoastPlanMaterialRatio, 0)
	for _, row := range rows {
		items := bomMap[row.ProductID]
		if len(items) == 0 {
			rawName := strings.TrimSpace(row.Product) + " 生豆"
			if strings.TrimSpace(rawName) == "生豆" {
				rawName = "咖啡豆(生豆/原豆)"
			}
			out = append(out, RoastPlanMaterialRatio{
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
			out = append(out, RoastPlanMaterialRatio{
				Key:          producePlanKey(row.ProductID, row.SpecG),
				ProductID:    row.ProductID,
				SpecG:        row.SpecG,
				ProductName:  row.Product,
				MaterialName: item.MaterialName,
				MaterialUnit: item.MaterialUnit,
				RatioPct:     normalizeBomRatioPct(item.RatioPct),
			})
		}
	}
	return out
}

func (d UnprodSummaryPageData) JSONBootstrap() string {
	b, _ := json.Marshal(d)
	return string(b)
}

func registerUnprodSummaryPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/unproduced", func(c echo.Context) error {
		target := "/vue-shell?view=producePlan"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})
}
