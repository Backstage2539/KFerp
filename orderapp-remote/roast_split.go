package main

import (
	"context"
	"math"
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
	q := "SELECT id,COALESCE(name,''),COALESCE(capacity_g,0),COALESCE(allowed_specs,''),COALESCE(min_roast_g,0),COALESCE(active,true) FROM " + schema + ".roast_machines WHERE active=true ORDER BY capacity_g ASC,id ASC"
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

func supportsSpec(allowed string, spec int64) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" {
		return true
	}
	for _, p := range strings.Split(allowed, ",") {
		v, _ := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if v == spec {
			return true
		}
	}
	return false
}

func calcRoastSplits(rows []UnprodNeedRow, machines []RoastMachine) []RoastSplitRow {
	out := make([]RoastSplitRow, 0)
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		need := float64(r.GapG) / 1000.0
		pick := RoastMachine{Name: "未匹配设备", CapacityG: 0, MinRoastG: 0}
		for _, m := range machines {
			if m.CapacityG <= 0 || !supportsSpec(m.AllowedSpecs, r.SpecG) {
				continue
			}
			pick = m
			break
		}
		if pick.CapacityG <= 0 {
			out = append(out, RoastSplitRow{Material: r.Product, Machine: pick.Name, BatchKg: "0", Batches: 0, TotalKg: formatKg(need)})
			continue
		}
		batchG := float64(pick.CapacityG)
		if pick.MinRoastG > 0 && float64(pick.MinRoastG) > batchG {
			batchG = float64(pick.MinRoastG)
		}
		if batchG <= 0 {
			batchG = float64(pick.CapacityG)
		}
		batches := int64(math.Ceil(float64(r.GapG) / batchG))
		out = append(out, RoastSplitRow{
			Material: r.Product,
			Machine:  pick.Name,
			BatchKg:  formatKg(batchG / 1000.0),
			Batches:  batches,
			TotalKg:  formatKg(need),
		})
	}
	return out
}

func formatKg(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
