package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoastSplitRow struct {
	Material    string
	Machine     string
	BatchKg     string
	Batches     int64
	TotalKg     string // 熟豆总需求
	YieldPctStr string // 损耗比展示
}

func loadActiveMachines(ctx context.Context, pool *pgxpool.Pool, schema string) ([]RoastMachine, error) {
	q := "SELECT id,COALESCE(name,''),COALESCE(capacity_g,0),COALESCE(allowed_specs,''),COALESCE(min_roast_g,0),COALESCE(active,true) FROM " + schema + ".roast_machines WHERE active=true ORDER BY capacity_g DESC,id ASC"
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RoastMachine, 0)
	for rows.Next() {
		var m RoastMachine
		if err := rows.Scan(&m.ID, &m.Name, &m.CapacityG, &m.AllowedSpecs, &m.MinRoastG, &m.Active); err == nil {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

func calcRoastSplits(rows []UnprodNeedRow, machines []RoastMachine, yieldRate float64) []RoastSplitRow {
	if yieldRate <= 0 || yieldRate > 1 {
		yieldRate = 0.8
	}
	out := make([]RoastSplitRow, 0)
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		// 烘焙建议按生豆计算：raw_g = ceil(finished_g / yieldRate)。
		rawG := int64(math.Ceil(float64(r.GapG) / yieldRate))
		pick, batches := pickMachineAndBatches(rawG, machines)
		finishedG := r.GapG
		yieldPct := fmt.Sprintf("%.0f%%", yieldRate*100)
		if pick.CapacityG <= 0 {
			out = append(out, RoastSplitRow{Material: r.Product, Machine: "未匹配设备", BatchKg: "0", Batches: 0, TotalKg: formatKg(finishedG), YieldPctStr: yieldPct})
			continue
		}
		out = append(out, RoastSplitRow{
			Material:    r.Product,
			Machine:     pick.Name,
			BatchKg:     formatBatchPlanKg(batches),
			Batches:     int64(len(batches)),
			TotalKg:     formatKg(finishedG),
			YieldPctStr: yieldPct,
		})
	}
	return out
}

func pickMachineAndBatches(totalG int64, machines []RoastMachine) (RoastMachine, []int64) {
	best := RoastMachine{Name: "未匹配设备", CapacityG: 0, MinRoastG: 0}
	bestBatches := []int64(nil)
	pickList := machines
	// 规则：需要烘焙生豆总量 <20kg 时，不使用布勒烘焙机。
	if totalG > 0 && totalG < 20000 {
		filtered := make([]RoastMachine, 0, len(machines))
		for _, m := range machines {
			n := strings.ToLower(strings.TrimSpace(m.Name))
			if strings.Contains(n, "布勒") || strings.Contains(n, "buhler") || strings.Contains(n, "bühler") {
				continue
			}
			filtered = append(filtered, m)
		}
		pickList = filtered
	}
	for _, m := range pickList {
		if m.CapacityG <= 0 {
			continue
		}
		loads := machineLoadsG(m)
		if len(loads) == 0 {
			continue
		}
		batches, ok := splitPreferEqual(totalG, loads)
		if !ok || len(batches) == 0 {
			continue
		}
		if len(bestBatches) == 0 || len(batches) < len(bestBatches) || (len(batches) == len(bestBatches) && sum64(batches) < sum64(bestBatches)) {
			best = m
			bestBatches = batches
		}
	}
	return best, bestBatches
}

// machineLoadsG returns available roast load options in grams from device config.
// allowed_specs is used as "载量设置(g,逗号分隔)".
// If empty, fallback to [min..max] with 1000g step.
func machineLoadsG(m RoastMachine) []int64 {
	minG := max64(m.MinRoastG, 1)
	maxG := m.CapacityG
	if maxG <= 0 || minG > maxG {
		return nil
	}

	loads := make([]int64, 0)
	seen := map[int64]bool{}
	raw := strings.TrimSpace(m.AllowedSpecs)
	if raw != "" {
		for _, p := range strings.Split(raw, ",") {
			g, _ := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
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

// splitPreferEqual: for multi-batch roasting, prefer same load per batch.
func splitPreferEqual(totalG int64, loads []int64) ([]int64, bool) {
	if totalG <= 0 || len(loads) == 0 {
		return nil, false
	}
	minLoad := loads[0]
	maxLoad := loads[len(loads)-1]
	if totalG <= maxLoad {
		for _, l := range loads {
			if l >= totalG {
				return []int64{l}, true
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
	for _, l := range loads {
		if l >= target {
			return l, true
		}
	}
	return 0, false
}

func formatBatchPlanKg(batches []int64) string {
	if len(batches) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(batches))
	for _, b := range batches {
		parts = append(parts, formatKg(b))
	}
	return strings.Join(parts, " + ")
}

func formatKg(g int64) string {
	kg := float64(g) / 1000.0
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", kg), "0"), ".")
}

func sum64(a []int64) int64 {
	var s int64
	for _, v := range a {
		s += v
	}
	return s
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
