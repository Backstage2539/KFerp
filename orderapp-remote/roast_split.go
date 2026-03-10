package main

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoastSplitRow struct {
	Material string
	Machine  string
	BatchKg  string
	Batches  int64
	TotalKg  string
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
		// 烘焙建议按生豆计算：raw_g = ceil(finished_g / yieldRate)，再按 kg 向上取整。
		rawG := int64(math.Ceil(float64(r.GapG) / yieldRate))
		rawKg := ceilDiv64(rawG, 1000)
		pick, batches := pickMachineAndBatches(rawKg, machines)
		if pick.CapacityG <= 0 {
			out = append(out, RoastSplitRow{Material: r.Product, Machine: "未匹配设备", BatchKg: "0", Batches: 0, TotalKg: strconv.FormatInt(rawKg, 10)})
			continue
		}
		out = append(out, RoastSplitRow{
			Material: r.Product,
			Machine:  pick.Name,
			BatchKg:  formatBatchPlanKgInt(batches),
			Batches:  int64(len(batches)),
			TotalKg:  strconv.FormatInt(rawKg, 10),
		})
	}
	return out
}

func pickMachineAndBatches(totalKg int64, machines []RoastMachine) (RoastMachine, []int64) {
	best := RoastMachine{Name: "未匹配设备", CapacityG: 0, MinRoastG: 0}
	bestBatches := []int64(nil)
	for _, m := range machines {
		if m.CapacityG <= 0 {
			continue
		}
		loads := machineLoadsKg(m)
		if len(loads) == 0 {
			continue
		}
		batches, ok := splitPreferEqual(totalKg, loads)
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

// machineLoadsKg returns available roast load options in kg from device config.
// allowed_specs is used as "载量设置(g,逗号分隔)".
// If empty, fallback to [min..max] with 1kg step.
func machineLoadsKg(m RoastMachine) []int64 {
	minKg := ceilDiv64(max64(m.MinRoastG, 1), 1000)
	maxKg := m.CapacityG / 1000
	if maxKg <= 0 || minKg > maxKg {
		return nil
	}

	loads := make([]int64, 0)
	seen := map[int64]bool{}
	raw := strings.TrimSpace(m.AllowedSpecs)
	if raw != "" {
		for _, p := range strings.Split(raw, ",") {
			g, _ := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if g <= 0 {
				continue
			}
			kg := ceilDiv64(g, 1000)
			if kg < minKg || kg > maxKg || seen[kg] {
				continue
			}
			seen[kg] = true
			loads = append(loads, kg)
		}
	}
	if len(loads) == 0 {
		for kg := minKg; kg <= maxKg; kg++ {
			loads = append(loads, kg)
		}
	}
	sort.Slice(loads, func(i, j int) bool { return loads[i] < loads[j] })
	return loads
}

// splitPreferEqual: for multi-batch roasting, prefer same load per batch.
func splitPreferEqual(totalKg int64, loads []int64) ([]int64, bool) {
	if totalKg <= 0 || len(loads) == 0 {
		return nil, false
	}
	minLoad := loads[0]
	maxLoad := loads[len(loads)-1]
	if totalKg <= maxLoad {
		for _, l := range loads {
			if l >= totalKg {
				return []int64{l}, true
			}
		}
		return []int64{maxLoad}, true
	}

	minBatches := ceilDiv(totalKg, maxLoad)
	maxBatches := ceilDiv(totalKg, minLoad)
	for n := minBatches; n <= maxBatches; n++ {
		target := ceilDiv64(totalKg, int64(n))
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

func formatBatchPlanKgInt(batches []int64) string {
	if len(batches) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(batches))
	for _, b := range batches {
		parts = append(parts, strconv.FormatInt(b, 10))
	}
	return strings.Join(parts, " + ")
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
