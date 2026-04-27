package production

import (
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	bomdomain "orderapp/internal/domain/bom"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type UnprodSummaryPageData = productionapp.PlanSummaryData
type ProducePlanDisplayRow = productionapp.ProducePlanDisplayRow
type UnprodSummaryQuery = productionapp.PlanSummaryQuery
type RoastPlanRow = productionapp.RoastPlanRow
type RoastPlanMaterialRatio = productionapp.RoastPlanMaterialRatio

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
				RatioPct:     bomdomain.NormalizeRatioPct(item.RatioPct),
			})
		}
	}
	return out
}

func registerUnprodSummaryPages(e *echo.Echo) {
	e.GET("/produce/unproduced", func(c echo.Context) error {
		target := "/vue-shell?view=producePlan"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})
}
