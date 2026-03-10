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
		minKg := ceilDiv64(max64(m.MinRoastG, 1), 1000)
		maxKg := m.CapacityG / 1000
		if maxKg <= 0 {
			continue
		}
		if minKg > maxKg {
			continue
		}
		batches, ok := splitByRange(totalKg, minKg, maxKg, 1)
		if !ok || len(batches) == 0 {
			continue
		}
		if len(bestBatches) == 0 || len(batches) < len(bestBatches) || (len(batches) == len(bestBatches) && m.CapacityG > best.CapacityG) {
			best = m
			bestBatches = batches
		}
	}
	return best, bestBatches
}

// splitByRange splits total into N batches for one machine only.
func splitByRange(total, minV, maxV, step int64) ([]int64, bool) {
	if total <= 0 || maxV <= 0 {
		return nil, false
	}
	if step <= 0 {
		step = 1
	}
	if minV <= 0 {
		minV = step
	}
	if maxV < minV {
		minV = maxV
	}
	if total <= maxV {
		if total < minV {
			return []int64{minV}, true
		}
		return []int64{total}, true
	}

	minBatches := ceilDiv(total, maxV)
	maxBatches := ceilDiv(total, minV)
	for n := minBatches; n <= maxBatches; n++ {
		base := int64(n) * minV
		if base > total {
			continue
		}
		extra := total - base
		cap := int64(n) * (maxV - minV)
		if extra > cap {
			continue
		}

		batches := make([]int64, n)
		for i := range batches {
			batches[i] = minV
		}

		if extra >= step {
			for i := 0; i < n && extra >= step; i++ {
				room := maxV - batches[i]
				steps := room / step
				need := extra / step
				if steps <= 0 || need <= 0 {
					continue
				}
				if need < steps {
					steps = need
				}
				add := steps * step
				batches[i] += add
				extra -= add
			}
		}

		for i := 0; extra > 0 && i < n; i++ {
			room := maxV - batches[i]
			if room <= 0 {
				continue
			}
			add := room
			if extra < add {
				add = extra
			}
			batches[i] += add
			extra -= add
		}

		if extra == 0 {
			sort.Slice(batches, func(i, j int) bool { return batches[i] > batches[j] })
			return batches, true
		}
	}
	return nil, false
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
