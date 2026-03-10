package main

import (
	"context"
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

func calcRoastSplits(rows []UnprodNeedRow, machines []RoastMachine) []RoastSplitRow {
	out := make([]RoastSplitRow, 0)
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		needKg := float64(r.GapG) / 1000.0
		pick, batches := pickMachineAndBatches(r, machines)
		if pick.CapacityG <= 0 {
			out = append(out, RoastSplitRow{Material: r.Product, Machine: "未匹配设备", BatchKg: "0", Batches: 0, TotalKg: formatKg(needKg)})
			continue
		}
		out = append(out, RoastSplitRow{
			Material: r.Product,
			Machine:  pick.Name,
			BatchKg:  formatBatchPlanKg(batches),
			Batches:  int64(len(batches)),
			TotalKg:  formatKg(needKg),
		})
	}
	return out
}

func pickMachineAndBatches(r UnprodNeedRow, machines []RoastMachine) (RoastMachine, []int64) {
	best := RoastMachine{Name: "未匹配设备", CapacityG: 0, MinRoastG: 0}
	bestBatches := []int64(nil)
	for _, m := range machines {
		if m.CapacityG <= 0 {
			continue
		}
		batches, ok := splitByRange(r.GapG, m.MinRoastG, m.CapacityG, 1000)
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

// splitByRange splits total grams into N batches for one machine only.
// Constraints: each batch in [minG,maxG], prefer 1000g step; a tiny non-step remainder is allowed in one batch.
func splitByRange(totalG, minG, maxG, stepG int64) ([]int64, bool) {
	if totalG <= 0 || maxG <= 0 {
		return nil, false
	}
	if stepG <= 0 {
		stepG = 1000
	}
	if minG <= 0 {
		minG = stepG
	}
	if maxG < minG {
		minG = maxG
	}
	if totalG <= maxG {
		if totalG < minG {
			return []int64{minG}, true
		}
		return []int64{totalG}, true
	}

	minBatches := ceilDiv(totalG, maxG)
	maxBatches := ceilDiv(totalG, minG)
	for n := minBatches; n <= maxBatches; n++ {
		base := int64(n) * minG
		if base > totalG {
			continue
		}
		extra := totalG - base
		cap := int64(n) * (maxG - minG)
		if extra > cap {
			continue
		}

		batches := make([]int64, n)
		for i := range batches {
			batches[i] = minG
		}

		if extra >= stepG {
			for i := 0; i < n && extra >= stepG; i++ {
				room := maxG - batches[i]
				steps := room / stepG
				need := extra / stepG
				if steps <= 0 || need <= 0 {
					continue
				}
				if need < steps {
					steps = need
				}
				add := steps * stepG
				batches[i] += add
				extra -= add
			}
		}

		for i := 0; extra > 0 && i < n; i++ {
			room := maxG - batches[i]
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

func formatBatchPlanKg(batches []int64) string {
	if len(batches) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(batches))
	for _, b := range batches {
		parts = append(parts, formatKg(float64(b)/1000.0))
	}
	return strings.Join(parts, " + ")
}

func ceilDiv(a, b int64) int {
	if b <= 0 {
		return 0
	}
	return int((a + b - 1) / b)
}

func formatKg(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
