package production

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
)

// Replenishment starts with the operator's actual deficit, not the amount of
// the old reservation still outside WIP. Reserved sources remain first, then
// eligible free batches can cover additional input or supply extra free WIP.
func (r Repository) previewActualReplenishmentTx(ctx context.Context, tx pgx.Tx, cmd app.BatchMaterialIssueCommand, ignoredDrafts []int64) (app.BatchMaterialIssuePreview, error) {
	legacy := cmd
	legacy.Workflow = ""
	base, err := r.previewBatchMaterialIssueIgnoringDraftsTx(ctx, tx, legacy, ignoredDrafts)
	if err != nil {
		return base, err
	}
	limits, err := r.replenishmentActualLimitsTx(ctx, tx, cmd)
	if err != nil {
		return base, err
	}
	blocked := map[int64]bool{}
	for _, b := range base.Blocked {
		blocked[b.JobCardID] = true
	}
	tasks, err := r.batchIssueTasksTx(ctx, tx, cmd.JobCardIDs, ignoredDrafts)
	if err != nil {
		return base, err
	}
	type demand struct {
		material app.OperationMaterialUse
		task     batchIssueTask
		required int64
	}
	demands := map[string]*demand{}
	versions := []string{base.Version}
	for _, task := range tasks {
		if !task.selected || blocked[task.id] {
			continue
		}
		p, _, e := r.operationCompletionContextTx(ctx, tx, task.id, false)
		if e != nil {
			return base, e
		}
		versions = append(versions, p.Version)
		materials := p.Materials
		if cmd.StartInput != nil {
			materials = cmd.StartInput.Materials
		}
		for _, m := range materials {
			if m.Warehouse != "wip" {
				continue
			}
			key := replenishmentMaterialKey(m.Key, m.BomVariantID, m.OwnerCustomerID, m.Unit)
			if limits[key] <= 0 {
				continue
			}
			g := app.BatchMaterialIssueGroup{Unit: m.Unit, QuantityBasis: "count"}
			if app.OperationUnitFactor(m.Unit) > 0 {
				g.QuantityBasis = "weight"
			}
			qty, e := app.BatchMaterialIssueBaseQuantity(g, m.Qty)
			if e != nil {
				return base, e
			}
			if demands[key] == nil {
				demands[key] = &demand{material: m, task: task}
			}
			demands[key].required += qty
		}
	}
	groups := map[string]*app.BatchMaterialIssueGroup{}
	frozenRemaining, frozenAvailable := map[string]int64{}, map[string]int64{}
	for _, g := range base.Groups {
		key := replenishmentGroupKey(g)
		frozenRemaining[key] += g.Remaining
		frozenAvailable[key] += g.Available
	}
	reservedGroups := map[string]bool{}
	seenBatch := map[string]bool{}
	batchKey := func(g app.BatchMaterialIssueGroup, code string) string {
		return fmt.Sprintf("%s/%d/%d/%d/%d/%d/%s/%s", g.ItemType, g.ItemID, g.BomSpecID, g.BomVariantID, g.SpecG, g.OwnerCustomerID, g.FromWarehouse, code)
	}
	// Reuse the existing own-reservation budget (which already excludes other
	// tasks' reservations), without offering a physical batch twice.
	for _, g := range base.Groups {
		if demands[replenishmentGroupKey(g)] == nil || g.FromWarehouse == "" {
			continue
		}
		copy := g
		copy.Allocations = append([]app.BatchMaterialIssueAllocation(nil), g.Allocations...)
		for _, a := range copy.Allocations {
			seenBatch[batchKey(g, a.BatchCode)] = true
		}
		groups[g.Key] = &copy
		reservedGroups[g.Key] = true
	}
	warehouses := []string{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT code FROM %s.warehouses WHERE active AND code<>'wip' ORDER BY sort_order,code`, r.schema))
	if err != nil {
		return base, err
	}
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			rows.Close()
			return base, err
		}
		warehouses = append(warehouses, code)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return base, err
	}
	repo := stockpg.NewRepository(r.pool, r.schema)
	keys := make([]string, 0, len(demands))
	for key := range demands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		d := demands[key]
		m := d.material
		// A new free batch can supplement extra actual demand. It must never
		// silently replace a missing/blocked frozen source allocation.
		if frozenRemaining[key] > frozenAvailable[key] || limits[key] <= frozenRemaining[key] {
			continue
		}
		kind, item := "material", m.MaterialID
		if m.ProductID > 0 {
			kind, item = "finished_product", m.ProductID
		}
		basis := "count"
		if app.OperationUnitFactor(m.Unit) > 0 {
			basis = "weight"
		}
		rr := batchIssueReservation{item: item, kind: kind, name: m.Name, unit: m.Unit, basis: basis, bomSpec: m.BomSpecID, bomVariant: m.BomVariantID, spec: m.SpecG, owner: m.OwnerCustomerID}
		if kind == "finished_product" {
			rr.kind = "product"
		}
		for _, warehouse := range warehouses {
			var warehouseOwner int64
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.warehouses WHERE code=$1`, r.schema), warehouse).Scan(&warehouseOwner); err != nil {
				return base, err
			}
			if warehouseOwner != 0 && warehouseOwner != m.OwnerCustomerID {
				continue
			}
			query := stockapp.TransferOptionsQuery{Warehouse: warehouse, ItemType: kind, ItemID: item, BomSpecID: m.BomSpecID, SpecG: m.SpecG, Limit: 500}
			for {
				page, e := repo.ListTransferOptionsTx(ctx, tx, query)
				if e != nil {
					return base, e
				}
				for _, row := range page.Rows {
					if row.ItemID != item || row.OwnerCustomerID != m.OwnerCustomerID || row.BomSpecID != m.BomSpecID || row.BomVariantID != m.BomVariantID || row.SpecG != m.SpecG || row.BatchCode == "" || row.UnavailableReason != "" {
						continue
					}
					qty := row.AvailableUnits
					if basis == "weight" {
						qty = row.AvailableG
					}
					if qty <= 0 {
						continue
					}
					g := batchIssueGroup(rr, warehouse, m.OwnerCustomerID)
					if seenBatch[batchKey(g, row.BatchCode)] {
						continue
					}
					seenBatch[batchKey(g, row.BatchCode)] = true
					if groups[g.Key] == nil {
						g.OwnerName = "工厂自有"
						if m.OwnerCustomerID > 0 {
							if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), m.OwnerCustomerID).Scan(&g.OwnerName); e != nil {
								return base, e
							}
						}
						groups[g.Key] = &g
					}
					target := groups[g.Key]
					target.Allocations = append(target.Allocations, app.BatchMaterialIssueAllocation{JobCardID: d.task.id, WorkOrderID: d.task.work, WorkOrderNo: d.task.no, ReservationID: m.ReservationID, BatchCode: row.BatchCode, ExtraAvailable: qty, Priority: d.task.priority, PlannedStart: d.task.start})
					versions = append(versions, app.BatchMaterialIssueHash(row))
				}
				if !page.HasNext {
					break
				}
				query.Offset += query.Limit
			}
		}
	}
	out := app.BatchMaterialIssuePreview{JobCardIDs: cmd.JobCardIDs, Groups: []app.BatchMaterialIssueGroup{}, Blocked: base.Blocked}
	ordered := make([]string, 0, len(groups))
	for k := range groups {
		ordered = append(ordered, k)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if reservedGroups[ordered[i]] != reservedGroups[ordered[j]] {
			return reservedGroups[ordered[i]]
		}
		return ordered[i] < ordered[j]
	})
	remaining := map[string]int64{}
	shownTotals := map[string]bool{}
	for k, v := range limits {
		remaining[k] = v
	}
	for _, gkey := range ordered {
		g := groups[gkey]
		key := replenishmentGroupKey(*g)
		g.Required = 0
		g.WIP = 0
		g.Remaining = 0
		g.Available = 0
		g.MaxQuantity = 0
		for i := range g.Allocations {
			a := &g.Allocations[i]
			capacity := a.Available + a.ExtraAvailable
			a.Available = min(capacity, remaining[key])
			remaining[key] -= a.Available
			a.ExtraAvailable = capacity - a.Available
			a.Required = a.Available
			a.Remaining = a.Available
			a.WIP = 0
			g.Required += a.Available
			g.Remaining += a.Available
			g.Available += a.Available
			g.MaxQuantity += capacity
		}
		if g.MaxQuantity <= 0 {
			continue
		}
		g.Required, g.WIP, g.Remaining = 0, 0, 0
		if !shownTotals[key] {
			g.Required = demands[key].required
			g.WIP = max(0, g.Required-limits[key])
			g.Remaining = limits[key]
			shownTotals[key] = true
		}
		g.Version = app.BatchMaterialIssueHash(struct {
			Group    app.BatchMaterialIssueGroup
			Versions []string
		}{*g, versions})
		out.Groups = append(out.Groups, *g)
	}
	for _, key := range keys {
		if remaining[key] > 0 {
			d := demands[key]
			out.Blocked = append(out.Blocked, app.BatchMaterialIssueBlocked{JobCardID: d.task.id, WorkOrderNo: d.task.no, Reason: fmt.Sprintf("%s 可调库存不足，仍缺 %g %s", d.material.Name, float64(remaining[key])/max(1, app.OperationUnitFactor(d.material.Unit)), d.material.Unit)})
		}
	}
	out.Version = app.BatchMaterialIssueHash(struct {
		Preview  app.BatchMaterialIssuePreview
		Versions []string
		Input    *app.ReplenishmentStartInput
	}{out, versions, cmd.StartInput})
	return out, nil
}
