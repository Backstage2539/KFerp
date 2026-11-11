package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stock "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	"strings"
)

// Frozen packaged-product weight and count are equivalent representations,
// while stock documents use qty_g for loose stock and qty_units for packages.
func normalizePlanTransferBatch(source app.ProductionPlanComponentSource, b app.PlanMaterialTransferBatch) (app.PlanMaterialTransferBatch, int64, error) {
	if b.QtyG < 0 || b.QtyUnits < 0 || (b.QtyG == 0 && b.QtyUnits == 0) {
		return b, 0, fmt.Errorf("请填写有效的批次数量")
	}
	kind := source.ComponentType
	if (kind == "product" || kind == "finished_product") && source.ComponentSpecG > 0 && source.RequiredUnits > 0 {
		n := b.QtyUnits
		if n == 0 {
			if b.QtyG%source.ComponentSpecG != 0 {
				return b, 0, fmt.Errorf("冻结规格商品须按完整件数调拨")
			}
			n = b.QtyG / source.ComponentSpecG
		}
		const maxInt64 = int64(^uint64(0) >> 1)
		if n > maxInt64/source.ComponentSpecG {
			return b, 0, fmt.Errorf("调拨数量过大")
		}
		weight := n * source.ComponentSpecG
		if b.QtyG > 0 && b.QtyG != weight {
			return b, 0, fmt.Errorf("重量与冻结规格件数不一致")
		}
		b.QtyG = 0
		b.QtyUnits = n
		if source.RequiredG == 0 {
			weight = 0
		}
		return b, weight, nil
	}
	if b.QtyG > 0 && b.QtyUnits > 0 {
		return b, 0, fmt.Errorf("散装重量和件数须分别调拨")
	}
	return b, b.QtyG, nil
}

func planTransferOptionMatchesSource(s app.ProductionPlanComponentSource, row stock.TransferOptionRow) bool {
	kind := s.ComponentType
	if kind == "product" {
		kind = "finished_product"
	}
	if row.ItemType != kind || row.ItemID != s.ComponentID {
		return false
	}
	if kind == "finished_product" && (row.BomSpecID != s.ComponentBOMSpecID || row.BomVariantID != s.ComponentBOMVariantID || row.SpecG != s.ComponentSpecG) {
		return false
	}
	for _, option := range s.Options {
		if option.Warehouse == row.Warehouse && option.OwnerCustomerID == row.OwnerCustomerID {
			return true
		}
	}
	return false
}

// CreatePlanMaterialTransfers creates all selected drafts in one plan-locked
// transaction. It never moves or reserves stock.
func (r Repository) CreatePlanMaterialTransfers(ctx context.Context, cmd app.PlanMaterialTransferCommand) (app.PlanMaterialTransferResult, error) {
	return r.executePlanMaterialTransfers(ctx, cmd, false)
}
func filterPlanTransferPreviewItems(items []app.PlanMaterialTransferItem, validate func([]app.PlanMaterialTransferItem) error) ([]app.PlanMaterialTransferItem, []app.PlanMaterialTransferUnavailable) {
	accepted := []app.PlanMaterialTransferItem{}
	unavailable := []app.PlanMaterialTransferUnavailable{}
	for _, item := range items {
		proposed := append(append([]app.PlanMaterialTransferItem{}, accepted...), item)
		if err := validate(proposed); err != nil {
			unavailable = append(unavailable, app.PlanMaterialTransferUnavailable{SourceID: item.SourceID, Reason: err.Error()})
			continue
		}
		accepted = proposed
	}
	return accepted, unavailable
}
func (r Repository) PreviewPlanMaterialTransfers(ctx context.Context, cmd app.PlanMaterialTransferCommand) (app.PlanMaterialTransferPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.PlanMaterialTransferPreview{}, err
	}
	defer tx.Rollback(ctx)
	detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ProductionPlanID)
	if err != nil {
		return app.PlanMaterialTransferPreview{}, err
	}
	if detail.Status != "draft" || detail.DraftToken != cmd.DraftToken {
		return app.PlanMaterialTransferPreview{}, fmt.Errorf("草稿版本已变化，请重新核对")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.entry_no,e.status,l.component_source_id,SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),SUM(l.qty_units) FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id WHERE l.production_plan_id=$1 AND e.status='draft' GROUP BY e.id,e.entry_no,e.status,l.component_source_id ORDER BY e.id,l.component_source_id`, r.schema), cmd.ProductionPlanID)
	if err != nil {
		return app.PlanMaterialTransferPreview{}, err
	}
	pending := map[int64][2]int64{}
	pendingDocs := []app.PlanMaterialTransferPending{}
	for rows.Next() {
		var p app.PlanMaterialTransferPending
		if err = rows.Scan(&p.ID, &p.EntryNo, &p.Status, &p.SourceID, &p.QtyG, &p.QtyUnits); err != nil {
			rows.Close()
			return app.PlanMaterialTransferPreview{}, err
		}
		pendingDocs = append(pendingDocs, p)
		q := pending[p.SourceID]
		pending[p.SourceID] = [2]int64{q[0] + p.QtyG, q[1] + p.QtyUnits}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return app.PlanMaterialTransferPreview{}, err
	}
	rows.Close()
	tx.Rollback(ctx)
	automatic := len(cmd.Items) == 0
	items := cmd.Items
	if automatic {
		items = planTransferSuggestions(detail.ComponentSources, pending)
	}
	var fatal error
	accepted, rejected := filterPlanTransferPreviewItems(items, func(selected []app.PlanMaterialTransferItem) error {
		probe := cmd
		probe.Items = selected
		_, e := r.executePlanMaterialTransfers(ctx, probe, true)
		if e != nil && !strings.Contains(e.Error(), "不可调拨") && !strings.Contains(e.Error(), "库存不足") && !strings.Contains(e.Error(), "超过该物料待补数量") {
			fatal = e
		}
		return e
	})
	if fatal != nil {
		return app.PlanMaterialTransferPreview{}, fatal
	}
	result := app.PlanMaterialTransferPreview{Groups: []app.PlanMaterialTransferGroup{}, Unavailable: rejected, PendingDocuments: pendingDocs}
	if len(accepted) > 0 {
		probe := cmd
		probe.Items = accepted
		valid, e := r.executePlanMaterialTransfers(ctx, probe, true)
		if e != nil {
			return app.PlanMaterialTransferPreview{}, e
		}
		result.Groups = valid.Groups
		result.DocumentCount = valid.DocumentCount
		result.PendingDocuments = valid.PendingDocuments
	}
	if automatic {
		offered := map[int64][2]int64{}
		for _, it := range accepted {
			q := offered[it.SourceID]
			for _, b := range it.Batches {
				q[0] += b.QtyG
				q[1] += b.QtyUnits
			}
			offered[it.SourceID] = q
		}
		for _, source := range detail.ComponentSources {
			q := offered[source.ID]
			p := pending[source.ID]
			if source.DemandG > source.WIPCoveredG+p[0]+q[0] || source.DemandUnits > source.WIPCoveredUnits+p[1]+q[1] {
				found := false
				for _, entry := range result.Unavailable {
					if entry.SourceID == source.ID {
						found = true
						break
					}
				}
				if !found {
					reason := "暂无足额可调库存，请先补齐供应"
					if source.UpstreamG > 0 || source.UpstreamUnits > 0 {
						reason = "部分物料由前道生产，尚无现货可调拨"
					}
					result.Unavailable = append(result.Unavailable, app.PlanMaterialTransferUnavailable{SourceID: source.ID, Reason: reason})
				}
			}
		}
	}
	return result, nil
}
func (r Repository) executePlanMaterialTransfers(ctx context.Context, cmd app.PlanMaterialTransferCommand, preview bool) (app.PlanMaterialTransferResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.PlanMaterialTransferResult{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	var revision int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,revision FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), cmd.ProductionPlanID).Scan(&status, &revision); err != nil {
		return app.PlanMaterialTransferResult{}, err
	}
	var hash string
	if !preview {
		requestJSON, e := json.Marshal(struct {
			DraftToken string                         `json:"draft_token"`
			Items      []app.PlanMaterialTransferItem `json:"items"`
		}{DraftToken: cmd.DraftToken, Items: cmd.Items})
		if e != nil {
			return app.PlanMaterialTransferResult{}, e
		}
		sum := sha256.Sum256(requestJSON)
		hash = hex.EncodeToString(sum[:])
		var previousHash string
		var previous []byte
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash,result FROM %s.production_plan_material_transfer_requests WHERE production_plan_id=$1 AND idempotency_key=$2`, r.schema), cmd.ProductionPlanID, cmd.IdempotencyKey).Scan(&previousHash, &previous)
		if e == nil {
			if previousHash != hash {
				return app.PlanMaterialTransferResult{}, fmt.Errorf("重复请求内容不一致，请重新核对")
			}
			var result app.PlanMaterialTransferResult
			if e = json.Unmarshal(previous, &result); e != nil {
				return result, e
			}
			return result, nil
		}
		if e != pgx.ErrNoRows {
			return app.PlanMaterialTransferResult{}, e
		}
	}
	if status != "draft" {
		return app.PlanMaterialTransferResult{}, fmt.Errorf("仅草稿生产计划可生成调拨单")
	}
	detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ProductionPlanID)
	if err != nil {
		return app.PlanMaterialTransferResult{}, err
	}
	if detail.DraftToken != cmd.DraftToken {
		return app.PlanMaterialTransferResult{}, fmt.Errorf("草稿版本已变化，请重新核对")
	}
	previewResult := app.PlanMaterialTransferResult{Groups: []app.PlanMaterialTransferGroup{}, Unavailable: []app.PlanMaterialTransferUnavailable{}, PendingDocuments: []app.PlanMaterialTransferPending{}}
	if preview {
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.entry_no,e.status,l.component_source_id,SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),SUM(l.qty_units) FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id WHERE l.production_plan_id=$1 AND e.status='draft' GROUP BY e.id,e.entry_no,e.status,l.component_source_id ORDER BY e.id,l.component_source_id`, r.schema), cmd.ProductionPlanID)
		if e != nil {
			return previewResult, e
		}
		pending := map[int64][2]int64{}
		for rows.Next() {
			var p app.PlanMaterialTransferPending
			if e = rows.Scan(&p.ID, &p.EntryNo, &p.Status, &p.SourceID, &p.QtyG, &p.QtyUnits); e != nil {
				rows.Close()
				return previewResult, e
			}
			previewResult.PendingDocuments = append(previewResult.PendingDocuments, p)
			old := pending[p.SourceID]
			pending[p.SourceID] = [2]int64{old[0] + p.QtyG, old[1] + p.QtyUnits}
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return previewResult, e
		}
		rows.Close()
		if len(cmd.Items) == 0 {
			cmd.Items = planTransferSuggestions(detail.ComponentSources, pending)
			offered := map[int64][2]int64{}
			for _, it := range cmd.Items {
				q := offered[it.SourceID]
				for _, b := range it.Batches {
					q[0] += b.QtyG
					q[1] += b.QtyUnits
				}
				offered[it.SourceID] = q
			}
			for _, source := range detail.ComponentSources {
				q := offered[source.ID]
				p := pending[source.ID]
				if source.DemandG > source.WIPCoveredG+p[0]+q[0] || source.DemandUnits > source.WIPCoveredUnits+p[1]+q[1] {
					reason := "暂无足额可调库存，请先补齐供应"
					if source.UpstreamG > 0 || source.UpstreamUnits > 0 {
						reason = "部分物料由前道生产，尚无现货可调拨"
					}
					previewResult.Unavailable = append(previewResult.Unavailable, app.PlanMaterialTransferUnavailable{SourceID: source.ID, Reason: reason})
				}
			}
		}
		if len(cmd.Items) == 0 {
			return previewResult, nil
		}
	}
	sources := map[int64]app.ProductionPlanComponentSource{}
	for _, s := range detail.ComponentSources {
		sources[s.ID] = s
	}
	type group struct {
		warehouse   string
		owner       int64
		items       []stock.StockDocumentItemCommand
		sourceIDs   []int64
		equivalentG []int64
	}
	groups := []group{}
	groupIndex := map[string]int{}
	usedSourceG := map[int64]int64{}
	usedSourceUnits := map[int64]int64{}
	usedBatchG := map[string]int64{}
	usedBatchUnits := map[string]int64{}
	pendingBatch := map[string][2]int64{}
	usedSelectedSourceBatch := map[string][2]int64{}
	stockRepo := stockpg.NewRepository(r.pool, r.schema)
	// Lock every physical identity before reading availability, matching stock posting's lock order.
	lockItems := []stock.StockDocumentItemRow{}
	for _, choice := range cmd.Items {
		source, ok := sources[choice.SourceID]
		if !ok {
			return app.PlanMaterialTransferResult{}, fmt.Errorf("物料来源已变化，请重新核对")
		}
		row := stock.StockDocumentItemRow{ItemType: source.ComponentType, BomSpecID: source.ComponentBOMSpecID, BomVariantID: source.ComponentBOMVariantID, SpecG: source.ComponentSpecG, InventoryUnit: source.Unit, FromWarehouse: choice.Warehouse, ToWarehouse: "wip", OwnerCustomerID: choice.OwnerCustomerID}
		if row.ItemType == "material" {
			row.MaterialID = source.ComponentID
		} else if row.ItemType == "product" || row.ItemType == "finished_product" {
			row.ItemType = "finished_product"
			row.ProductID = source.ComponentID
		} else {
			return app.PlanMaterialTransferResult{}, fmt.Errorf("物料类型不支持调拨")
		}
		lockItems = append(lockItems, row)
	}
	if err := stockRepo.LockPlanTransferInventoryTx(ctx, tx, lockItems); err != nil {
		return app.PlanMaterialTransferResult{}, err
	}
	for _, choice := range cmd.Items {
		source, ok := sources[choice.SourceID]
		if !ok {
			return app.PlanMaterialTransferResult{}, fmt.Errorf("物料来源已变化，请重新核对")
		}
		var pendingG, pendingUnits int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),0),COALESCE(SUM(l.qty_units),0) FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id WHERE l.production_plan_id=$1 AND l.component_source_id=$2 AND e.status='draft'`, r.schema), cmd.ProductionPlanID, choice.SourceID).Scan(&pendingG, &pendingUnits)
		if err != nil {
			return app.PlanMaterialTransferResult{}, err
		}
		kind := source.ComponentType
		if kind == "product" {
			kind = "finished_product"
		}
		candidates, err := stockRepo.ListTransferOptionsTx(ctx, tx, stock.TransferOptionsQuery{Warehouse: choice.Warehouse, ItemType: kind, ItemID: source.ComponentID, BomSpecID: source.ComponentBOMSpecID, SpecG: source.ComponentSpecG, Limit: 100000})
		if err != nil {
			return app.PlanMaterialTransferResult{}, err
		}
		for _, batch := range choice.Batches {
			normalized, equivalentG, e := normalizePlanTransferBatch(source, batch)
			if e != nil {
				return app.PlanMaterialTransferResult{}, e
			}
			batch = normalized
			var candidate *stock.TransferOptionRow
			for i := range candidates.Rows {
				row := &candidates.Rows[i]
				if row.BatchCode == batch.BatchCode && row.OwnerCustomerID == choice.OwnerCustomerID && row.UnavailableReason == "" && planTransferOptionMatchesSource(source, *row) {
					candidate = row
					break
				}
			}
			if candidate == nil {
				return app.PlanMaterialTransferResult{}, fmt.Errorf("来源批次不可调拨，请重新核对")
			}
			if source.AllocationMode == "semi_finished_selected" {
				key := fmt.Sprintf("%d:%s:%d:%s", choice.SourceID, choice.Warehouse, choice.OwnerCustomerID, batch.BatchCode)
				used := usedSelectedSourceBatch[key]
				var pendingG, pendingN int64
				err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0),COALESCE(SUM(i.qty_units),0) FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id AND e.status='draft' JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id WHERE l.production_plan_id=$1 AND l.component_source_id=$2 AND i.from_warehouse=$3 AND i.owner_customer_id=$4 AND i.batch_code=$5`, r.schema), cmd.ProductionPlanID, choice.SourceID, choice.Warehouse, choice.OwnerCustomerID, batch.BatchCode).Scan(&pendingG, &pendingN)
				if err != nil {
					return app.PlanMaterialTransferResult{}, err
				}
				if err = validateSelectedSemiFinishedTransfer(source, choice.Warehouse, choice.OwnerCustomerID, batch.BatchCode, batch.QtyG, batch.QtyUnits, used[0]+pendingG, used[1]+pendingN); err != nil {
					return app.PlanMaterialTransferResult{}, err
				}
				usedSelectedSourceBatch[key] = [2]int64{used[0] + batch.QtyG, used[1] + batch.QtyUnits}
			}
			key := fmt.Sprintf("%s:%s:%d:%d:%d:%d:%d:%s", choice.Warehouse, kind, source.ComponentID, source.ComponentBOMSpecID, source.ComponentBOMVariantID, source.ComponentSpecG, choice.OwnerCustomerID, batch.BatchCode)
			pending, found := pendingBatch[key]
			if !found {
				err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0),COALESCE(SUM(i.qty_units),0)
                FROM %[1]s.production_plan_material_transfer_links l
                JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id AND e.status='draft'
                JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id AND i.stock_entry_id=e.id
                WHERE l.production_plan_id=$1 AND i.from_warehouse=$2 AND i.item_type=$3
                  AND (($3='material' AND i.material_id=$4) OR ($3='finished_product' AND i.product_id=$4 AND i.bom_spec_id=$5 AND i.bom_variant_id=$6 AND i.spec_g=$7))
                  AND i.owner_customer_id=$8 AND i.batch_code=$9`, r.schema), cmd.ProductionPlanID, choice.Warehouse, kind, source.ComponentID, source.ComponentBOMSpecID, source.ComponentBOMVariantID, source.ComponentSpecG, choice.OwnerCustomerID, batch.BatchCode).Scan(&pending[0], &pending[1])
				if err != nil {
					return app.PlanMaterialTransferResult{}, err
				}
				pendingBatch[key] = pending
			}
			usedBatchG[key] += batch.QtyG
			usedBatchUnits[key] += batch.QtyUnits
			if usedBatchG[key]+pending[0] > candidate.AvailableG || usedBatchUnits[key]+pending[1] > candidate.AvailableUnits {
				if batch.QtyUnits > 0 && candidate.AvailableUnits == 0 && candidate.AvailableG > 0 {
					return app.PlanMaterialTransferResult{}, fmt.Errorf("该批次只有散装库存，不能按件调拨")
				}
				return app.PlanMaterialTransferResult{}, fmt.Errorf("批次可用库存不足，请重新核对")
			}
			usedSourceG[choice.SourceID] += equivalentG
			usedSourceUnits[choice.SourceID] += batch.QtyUnits
			if usedSourceG[choice.SourceID]+pendingG > source.RequiredG-source.WIPCoveredG || usedSourceUnits[choice.SourceID]+pendingUnits > source.RequiredUnits-source.WIPCoveredUnits {
				return app.PlanMaterialTransferResult{}, fmt.Errorf("调拨数量超过该物料待补数量")
			}
			gkey := fmt.Sprintf("%s:%d", choice.Warehouse, choice.OwnerCustomerID)
			idx, exists := groupIndex[gkey]
			if !exists {
				idx = len(groups)
				groupIndex[gkey] = idx
				groups = append(groups, group{warehouse: choice.Warehouse, owner: choice.OwnerCustomerID})
			}
			item := stock.StockDocumentItemCommand{ItemType: kind, ItemName: source.ComponentName, BomSpecID: source.ComponentBOMSpecID, BomVariantID: source.ComponentBOMVariantID, SpecG: source.ComponentSpecG, InventoryUnit: source.Unit, FromWarehouse: choice.Warehouse, ToWarehouse: "wip", OwnerCustomerID: choice.OwnerCustomerID, TargetOwnerCustomerID: choice.OwnerCustomerID, QtyG: batch.QtyG, QtyUnits: batch.QtyUnits, BatchCode: batch.BatchCode}
			if kind == "material" {
				item.MaterialID = source.ComponentID
			} else if kind == "finished_product" {
				item.ProductID = source.ComponentID
			} else {
				return app.PlanMaterialTransferResult{}, fmt.Errorf("物料类型不支持调拨")
			}
			groups[idx].items = append(groups[idx].items, item)
			groups[idx].sourceIDs = append(groups[idx].sourceIDs, choice.SourceID)
			groups[idx].equivalentG = append(groups[idx].equivalentG, equivalentG)
		}
	}
	if preview {
		groupIndex := map[string]int{}
		for _, item := range cmd.Items {
			key := fmt.Sprintf("%s:%d", item.Warehouse, item.OwnerCustomerID)
			idx, ok := groupIndex[key]
			if !ok {
				idx = len(previewResult.Groups)
				groupIndex[key] = idx
				previewResult.Groups = append(previewResult.Groups, app.PlanMaterialTransferGroup{Warehouse: item.Warehouse, OwnerCustomerID: item.OwnerCustomerID, Items: []app.PlanMaterialTransferItem{}})
			}
			previewResult.Groups[idx].Items = append(previewResult.Groups[idx].Items, item)
		}
		previewResult.DocumentCount = len(previewResult.Groups)
		return previewResult, nil
	}
	result := app.PlanMaterialTransferResult{Documents: []stock.StockDocumentDetail{}}
	for _, g := range groups {
		doc, err := stockRepo.CreatePlanStockDocumentDraftTx(ctx, tx, stock.StockDocumentCommand{Purpose: stock.PurposeMaterialTransfer, EntryType: stock.PurposeMaterialTransfer, SourceType: "production_plan_material_transfer", SourceID: cmd.ProductionPlanID, Operator: cmd.Operator, Items: g.items})
		if err != nil {
			return result, err
		}
		if len(doc.Items) != len(g.sourceIDs) {
			return result, fmt.Errorf("调拨单物料关联不完整")
		}
		for i, row := range doc.Items {
			expected := g.items[i]
			if row.ItemType != expected.ItemType || row.MaterialID != expected.MaterialID || row.ProductID != expected.ProductID || row.BomSpecID != expected.BomSpecID || row.BomVariantID != expected.BomVariantID || row.SpecG != expected.SpecG || row.QtyG != expected.QtyG || row.QtyUnits != expected.QtyUnits || row.BatchCode != expected.BatchCode || row.FromWarehouse != expected.FromWarehouse || row.ToWarehouse != expected.ToWarehouse || row.OwnerCustomerID != expected.OwnerCustomerID || row.TargetOwnerCustomerID != expected.TargetOwnerCustomerID {
				return result, fmt.Errorf("调拨单物料身份已变化，请重新核对")
			}
			source := sources[g.sourceIDs[i]]
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_material_transfer_links(production_plan_id,production_plan_item_id,component_source_id,stock_entry_id,stock_entry_item_id,plan_revision,qty_g,qty_units,equivalent_qty_g,source_snapshot) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,jsonb_build_object('required_g',s.required_g,'required_units',s.required_units,'unit',s.unit,'bom_version_id',s.bom_version_id,'allocation_mode',s.allocation_mode,'allocations_json',s.allocations_json,'source_owner_customer_id',s.source_owner_customer_id) FROM %[1]s.production_plan_component_sources s WHERE s.id=$3 AND s.production_plan_id=$1 AND s.production_plan_item_id=$2`, r.schema), cmd.ProductionPlanID, source.ProductionPlanItemID, source.ID, doc.ID, row.ID, revision, row.QtyG, row.QtyUnits, g.equivalentG[i])
			if err != nil {
				return result, err
			}
		}
		result.Documents = append(result.Documents, doc)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_plan_material_transfer_requests(production_plan_id,idempotency_key,request_hash,result) VALUES($1,$2,$3,$4)`, r.schema), cmd.ProductionPlanID, cmd.IdempotencyKey, hash, resultJSON)
	if err != nil {
		return result, err
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.ProductionPlanID, "create_material_transfers", postgresinfra.StrPtr("stock_entries"), nil, nil, postgresinfra.AuditMeta{"count": len(result.Documents)}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func planTransferSuggestions(sources []app.ProductionPlanComponentSource, pending map[int64][2]int64) []app.PlanMaterialTransferItem {
	out := []app.PlanMaterialTransferItem{}
	for _, s := range sources {
		remainingG := s.RequiredG - s.WIPCoveredG - pending[s.ID][0]
		remainingUnits := s.RequiredUnits - s.WIPCoveredUnits - pending[s.ID][1]
		if remainingG < 0 {
			remainingG = 0
		}
		if remainingUnits < 0 {
			remainingUnits = 0
		}
		for _, allocation := range s.Allocations {
			if allocation.Warehouse == "wip" {
				continue
			}
			item := app.PlanMaterialTransferItem{SourceID: s.ID, Warehouse: allocation.Warehouse, OwnerCustomerID: allocation.OwnerCustomerID}
			for _, batch := range allocation.Batches {
				g := batch.QtyG
				if g > remainingG {
					g = remainingG
				}
				n := batch.QtyUnits
				if n > remainingUnits {
					n = remainingUnits
				}
				if g <= 0 && n <= 0 {
					continue
				}
				item.Batches = append(item.Batches, app.PlanMaterialTransferBatch{BatchCode: batch.BatchCode, QtyG: g, QtyUnits: n})
				remainingG -= g
				remainingUnits -= n
			}
			if len(item.Batches) > 0 {
				out = append(out, item)
			}
		}
	}
	return out
}

func loadPlanMaterialTransferDocumentsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]app.PlanMaterialTransferPending, error) {
	out := []app.PlanMaterialTransferPending{}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass(format('%I.%I',$1::text,$2::text)) IS NOT NULL`, schema, "production_plan_material_transfer_links").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return out, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.entry_no,e.status,l.component_source_id,SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),SUM(l.qty_units)
 FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id
 WHERE l.production_plan_id=$1 GROUP BY e.id,e.entry_no,e.status,l.component_source_id ORDER BY e.id,l.component_source_id`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p app.PlanMaterialTransferPending
		if err := rows.Scan(&p.ID, &p.EntryNo, &p.Status, &p.SourceID, &p.QtyG, &p.QtyUnits); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
