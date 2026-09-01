package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	catalogdomain "orderapp/internal/domain/catalog"
	inventorydomain "orderapp/internal/domain/inventory"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type miniDirectShipQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type miniFinishedInventoryRow struct {
	ProductID       int64
	BomSpecID       int64
	BomVariantID    int64
	BomSpecKey      string
	BomSpecName     string
	InventoryUnit   string
	IsDefaultSpec   bool
	ProductName     string
	ParentProductID int64
	SKUCode         string
	SpecG           int64
	Warehouse       string
	WarehouseName   string
	TotalQty        int64
	LegacyQty       int64
	ReservedQty     int64
	UpdatedAt       time.Time
}

type miniFinishedBatchRow struct {
	BatchID       int64
	BatchCode     string
	ProductID     int64
	BomSpecID     int64
	BomVariantID  int64
	BomSpecKey    string
	BomSpecName   string
	InventoryUnit string
	ProductName   string
	SKUCode       string
	SpecG         int64
	Warehouse     string
	WarehouseName string
	OriginalQty   int64
	PhysicalQty   int64
	ReservedQty   int64
	QualityStatus string
	SourceDocType string
	SourceDocID   int64
	CreatedAt     time.Time
	InboundAt     *time.Time
}

type miniStockCandidate struct {
	ProductID    int64
	BomSpecID    int64
	BomVariantID int64
	SpecG        int64
	Warehouse    string
	BatchID      int64
	BatchCode    string
	AvailableQty int64
	CreatedAt    time.Time
	Legacy       bool
}

type miniStockSnapshot struct {
	Inventory  []miniFinishedInventoryRow
	Batches    []miniFinishedBatchRow
	Candidates []miniStockCandidate
}

type miniPlannedAllocation struct {
	ProductID    int64
	BomSpecID    int64
	BomVariantID int64
	SpecG        int64
	Warehouse    string
	BatchID      int64
	BatchCode    string
	Qty          int64
}

func (r *Repository) MiniDirectShipCatalog(ctx context.Context, query app.MiniDirectShipCatalogQuery) (app.MiniDirectShipCatalog, error) {
	summaries, err := r.ListCustomerCentralInventory(ctx, query.CustomerID)
	if err != nil {
		return app.MiniDirectShipCatalog{}, err
	}
	productIDs := make([]int64, 0, len(summaries))
	for _, row := range summaries {
		if row.AvailableQty > 0 {
			productIDs = append(productIDs, row.ProductID)
		}
	}
	if len(productIDs) == 0 {
		return app.MiniDirectShipCatalog{CurrentCustomerID: query.CustomerID, ProductFamilies: []map[string]any{}}, nil
	}

	type catalogRow struct {
		ProductID       int64
		ParentProductID int64
		ParentName      string
		ProductName     string
		SKUName         string
		SKUCode         string
		SpecLabel       string
		NetContentQty   float64
		NetContentUnit  string
		DefaultSKU      bool
		ProductKind     string
		CategoryID      int64
		CategoryName    string
		AliasName       string
		CustomerCode    string
	}
	aliasSelect := "'' AS alias_name, '' AS customer_code"
	aliasJoin := ""
	if relationExists(ctx, r.pool, fmt.Sprintf("%s.customer_product_aliases", r.schema)) {
		aliasSelect = "COALESCE(a.display_name,'') AS alias_name, COALESCE(a.customer_item_code,'') AS customer_code"
		aliasJoin = fmt.Sprintf(`LEFT JOIN LATERAL (
			SELECT display_name, customer_item_code
			FROM %s.customer_product_aliases
			WHERE customer_id=$2 AND product_id=p.id AND active=true
			ORDER BY sort_order, id
			LIMIT 1
		) a ON true`, r.schema)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT p.id,
		       COALESCE(NULLIF(p.parent_product_id,0), NULLIF(p.base_product_id,0), p.id),
		       COALESCE(NULLIF(parent.name,''), NULLIF(p.name,''), ''),
		       COALESCE(NULLIF(p.name,''),''),
		       COALESCE(NULLIF(p.sku_name,''), NULLIF(p.name,''), ''),
		       COALESCE(p.sku_code,''), COALESCE(p.spec_label,''),
		       COALESCE(p.net_content_qty,0)::float8, COALESCE(p.net_content_unit,''),
		       COALESCE(p.is_default_sku,false), COALESCE(p.product_kind,''),
		       COALESCE(pc.id,0), COALESCE(pc.name,''),
		       %s
		FROM %s.products p
		LEFT JOIN %s.products parent ON parent.id=COALESCE(NULLIF(p.parent_product_id,0), NULLIF(p.base_product_id,0))
		LEFT JOIN %s.product_categories pc ON pc.id=p.product_category_id
		%s
		WHERE p.id=ANY($1::bigint[]) AND p.active=true
		ORDER BY COALESCE(parent.name,p.name), p.net_content_qty, p.id
	`, aliasSelect, r.schema, r.schema, r.schema, aliasJoin), productIDs, query.CustomerID)
	if err != nil {
		return app.MiniDirectShipCatalog{}, err
	}
	defer rows.Close()
	meta := make(map[int64]catalogRow, len(productIDs))
	for rows.Next() {
		var row catalogRow
		if err := rows.Scan(&row.ProductID, &row.ParentProductID, &row.ParentName, &row.ProductName, &row.SKUName, &row.SKUCode, &row.SpecLabel, &row.NetContentQty, &row.NetContentUnit, &row.DefaultSKU, &row.ProductKind, &row.CategoryID, &row.CategoryName, &row.AliasName, &row.CustomerCode); err != nil {
			return app.MiniDirectShipCatalog{}, err
		}
		meta[row.ProductID] = row
	}
	if err := rows.Err(); err != nil {
		return app.MiniDirectShipCatalog{}, err
	}

	type familyState struct {
		row   map[string]any
		specs []map[string]any
	}
	families := make([]*familyState, 0)
	byParent := map[int64]*familyState{}
	categories := make([]app.MiniDirectShipCategory, 0)
	seenCategories := map[string]bool{}
	q := strings.ToLower(strings.TrimSpace(query.Q))
	categoryFilter := strings.TrimSpace(query.Category)
	for _, inventory := range summaries {
		if inventory.AvailableQty <= 0 {
			continue
		}
		row, ok := meta[inventory.ProductID]
		if !ok {
			continue
		}
		categoryKey := ""
		if row.CategoryID > 0 {
			categoryKey = strconv.FormatInt(row.CategoryID, 10)
			if !seenCategories[categoryKey] {
				seenCategories[categoryKey] = true
				categories = append(categories, app.MiniDirectShipCategory{Key: categoryKey, Label: row.CategoryName})
			}
		}
		if categoryFilter != "" && categoryFilter != categoryKey && !strings.EqualFold(categoryFilter, row.CategoryName) {
			continue
		}
		displayName := strings.TrimSpace(row.ParentName)
		if strings.TrimSpace(row.AliasName) != "" {
			displayName = strings.TrimSpace(row.AliasName)
		}
		searchText := strings.ToLower(strings.Join([]string{
			displayName, row.ParentName, row.ProductName, row.SKUName, row.SKUCode, row.CustomerCode,
			inventory.BomSpecName, inventory.BomSpecKey, inventory.SKUCode,
			catalogdomain.SearchPinyin(displayName + " " + row.SKUName),
			catalogdomain.SearchInitials(displayName + " " + row.SKUName),
		}, " "))
		if q != "" && !strings.Contains(searchText, q) {
			continue
		}
		state := byParent[row.ParentProductID]
		if state == nil {
			state = &familyState{row: map[string]any{
				"parent_product_id":             row.ParentProductID,
				"parent_product_name":           row.ParentName,
				"name":                          displayName,
				"alias_name":                    row.AliasName,
				"customer_product_display_name": row.AliasName,
				"customer_item_code":            row.CustomerCode,
				"code":                          row.CustomerCode,
				"product_code":                  row.SKUCode,
				"product_type_name":             row.CategoryName,
				"product_kind":                  row.ProductKind,
				"py":                            catalogdomain.SearchPinyin(displayName),
				"pyi":                           catalogdomain.SearchInitials(displayName),
			}}
			byParent[row.ParentProductID] = state
			families = append(families, state)
		}
		netQty := row.NetContentQty
		netUnit := strings.TrimSpace(row.NetContentUnit)
		if inventory.BomSpecID > 0 {
			netQty = 1
			netUnit = inventory.InventoryUnit
		} else if netQty <= 0 {
			netQty = float64(inventory.SpecG)
			netUnit = "g"
		}
		specLabel := strings.TrimSpace(row.SpecLabel)
		if inventory.BomSpecID > 0 {
			specLabel = inventory.BomSpecName
		} else if specLabel == "" {
			specLabel = fmt.Sprintf("%dg", inventory.SpecG)
		}
		skuCode := row.SKUCode
		skuName := row.SKUName
		if inventory.BomSpecID > 0 {
			skuCode = inventory.SKUCode
			skuName = strings.TrimSpace(row.ParentName + " " + inventory.BomSpecName)
		}
		state.specs = append(state.specs, map[string]any{
			"product_id":       inventory.ProductID,
			"sku_id":           inventory.ProductID,
			"bom_spec_id":      inventory.BomSpecID,
			"bom_variant_id":   inventory.BomVariantID,
			"bom_spec_key":     inventory.BomSpecKey,
			"inventory_unit":   inventory.InventoryUnit,
			"sku_code":         skuCode,
			"sku_name":         skuName,
			"py":               catalogdomain.SearchPinyin(skuName + " " + specLabel),
			"pyi":              catalogdomain.SearchInitials(skuName + " " + specLabel),
			"spec_label":       specLabel,
			"net_content_qty":  netQty,
			"net_content_unit": netUnit,
			"is_default_sku":   row.DefaultSKU || inventory.IsDefaultSpec,
			"product_kind":     row.ProductKind,
			"sales_unit": func() string {
				if inventory.BomSpecID > 0 {
					return inventory.InventoryUnit
				}
				return "bag"
			}(),
			"available_qty": inventory.AvailableQty,
		})
	}
	outFamilies := make([]map[string]any, 0, len(families))
	for _, family := range families {
		if len(family.specs) == 0 {
			continue
		}
		family.row["specs"] = family.specs
		outFamilies = append(outFamilies, family.row)
	}
	sort.SliceStable(categories, func(i, j int) bool { return categories[i].Label < categories[j].Label })
	return app.MiniDirectShipCatalog{CurrentCustomerID: query.CustomerID, Categories: categories, ProductFamilies: outFamilies}, nil
}

func (r *Repository) PreviewMiniDirectShip(ctx context.Context, cmd app.MiniDirectShipCommand) (app.MiniDirectShipPreview, error) {
	items, err := r.resolveMiniDirectShipItems(ctx, r.pool, cmd.Items)
	if err != nil {
		return app.MiniDirectShipPreview{}, err
	}
	cmd.Items = items
	snapshot, err := r.loadMiniCustomerFinishedStock(ctx, r.pool, cmd.CustomerID, false)
	if err != nil {
		return app.MiniDirectShipPreview{}, err
	}
	allocations, shortages := planMiniDirectShipAllocations(cmd.Items, snapshot.Candidates)
	preview := miniDirectShipPreview(allocations, shortages)
	products, err := r.loadMiniDirectShipProductSnapshots(ctx, r.pool, cmd.Items)
	if err != nil {
		return app.MiniDirectShipPreview{}, err
	}
	for warehouseIndex := range preview.Warehouses {
		for itemIndex := range preview.Warehouses[warehouseIndex].Items {
			item := &preview.Warehouses[warehouseIndex].Items[itemIndex]
			product := products[miniStockKey(item.ProductID, item.BomSpecID, item.SpecG)]
			item.ProductName, item.SKUCode, item.SpecLabel = product.ProductName, product.SKUCode, product.SpecLabel
			item.BomSpecKey, item.InventoryUnit = product.BomSpecKey, product.InventoryUnit
		}
	}
	return preview, nil
}

func miniDirectShipPreview(allocations []miniPlannedAllocation, shortages []app.MiniDirectShipShortage) app.MiniDirectShipPreview {
	type warehouseState struct {
		row   app.MiniDirectShipPreviewWarehouse
		byKey map[string]int
	}
	states := make([]*warehouseState, 0)
	byWarehouse := map[string]*warehouseState{}
	for _, allocation := range allocations {
		state := byWarehouse[allocation.Warehouse]
		if state == nil {
			state = &warehouseState{row: app.MiniDirectShipPreviewWarehouse{Warehouse: allocation.Warehouse, Items: []app.MiniDirectShipItemCommand{}}, byKey: map[string]int{}}
			byWarehouse[allocation.Warehouse] = state
			states = append(states, state)
		}
		key := miniStockKey(allocation.ProductID, allocation.BomSpecID, allocation.SpecG)
		if idx, ok := state.byKey[key]; ok {
			state.row.Items[idx].Qty += allocation.Qty
		} else {
			state.byKey[key] = len(state.row.Items)
			state.row.Items = append(state.row.Items, app.MiniDirectShipItemCommand{ProductID: allocation.ProductID, BomSpecID: allocation.BomSpecID, BomVariantID: allocation.BomVariantID, SpecG: allocation.SpecG, Qty: allocation.Qty})
		}
	}
	warehouses := make([]app.MiniDirectShipPreviewWarehouse, 0, len(states))
	for _, state := range states {
		warehouses = append(warehouses, state.row)
	}
	return app.MiniDirectShipPreview{CanSubmit: len(shortages) == 0, Warehouses: warehouses, Shortages: shortages}
}

func planMiniDirectShipAllocations(items []app.MiniDirectShipItemCommand, candidates []miniStockCandidate) ([]miniPlannedAllocation, []app.MiniDirectShipShortage) {
	available := append([]miniStockCandidate(nil), candidates...)
	allocations := make([]miniPlannedAllocation, 0)
	shortages := make([]app.MiniDirectShipShortage, 0)
	for _, item := range items {
		remaining := item.Qty
		availableBefore := int64(0)
		for idx := range available {
			candidate := &available[idx]
			if candidate.ProductID != item.ProductID || candidate.BomSpecID != item.BomSpecID || candidate.SpecG != item.SpecG || candidate.AvailableQty <= 0 {
				continue
			}
			availableBefore += candidate.AvailableQty
			if remaining <= 0 {
				continue
			}
			take := candidate.AvailableQty
			if take > remaining {
				take = remaining
			}
			allocations = append(allocations, miniPlannedAllocation{
				ProductID: item.ProductID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, SpecG: item.SpecG, Warehouse: candidate.Warehouse,
				BatchID: candidate.BatchID, BatchCode: candidate.BatchCode, Qty: take,
			})
			candidate.AvailableQty -= take
			remaining -= take
		}
		if remaining > 0 {
			shortages = append(shortages, app.MiniDirectShipShortage{ProductID: item.ProductID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, SpecG: item.SpecG, Qty: item.Qty, AvailableQty: availableBefore})
		}
	}
	return allocations, shortages
}

func (r *Repository) loadMiniCustomerFinishedStock(ctx context.Context, q miniDirectShipQuerier, customerID int64, lock bool) (miniStockSnapshot, error) {
	if !relationExists(ctx, q, fmt.Sprintf("%s.warehouses", r.schema)) || !relationExists(ctx, q, fmt.Sprintf("%s.finished_inventory", r.schema)) {
		return miniStockSnapshot{Inventory: []miniFinishedInventoryRow{}, Batches: []miniFinishedBatchRow{}, Candidates: []miniStockCandidate{}}, nil
	}
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF fi"
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT fi.product_id,COALESCE(fi.bom_spec_id,0),COALESCE(fi.bom_variant_id,0),
		       CASE WHEN COALESCE(fi.bom_spec_id,0)>0
		         THEN btrim(COALESCE(NULLIF(p.name,''),'') || ' ' || COALESCE(NULLIF(variant.spec_name_snapshot,''),NULLIF(spec.name,''),''))
		         ELSE COALESCE(NULLIF(p.sku_name,''), NULLIF(p.name,''), '') END,
		       COALESCE(NULLIF(p.parent_product_id,0), NULLIF(p.base_product_id,0), p.id, 0),
		       CASE WHEN COALESCE(fi.bom_spec_id,0)>0 THEN COALESCE(spec.code,'') ELSE COALESCE(p.sku_code,'') END,
		       COALESCE(spec.spec_key,''),COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,''),COALESCE(variant.is_default,false),
		       fi.spec_g, fi.warehouse, COALESCE(w.name,fi.warehouse),
		       GREATEST(COALESCE(fi.onhand_units,0),0), GREATEST(COALESCE(fi.onhand_loose_g,0),0), fi.updated_at
		FROM %s.finished_inventory fi
		JOIN %s.warehouses w ON w.code=fi.warehouse AND w.active=true
		  AND w.kind IN ('finished','customer_processing','customer_finished','customer') AND w.customer_id=$1
		LEFT JOIN %s.products p ON p.id=fi.product_id
		LEFT JOIN %s.production_bom_specs spec ON spec.id=fi.bom_spec_id
		LEFT JOIN %s.production_bom_version_variants variant ON variant.id=fi.bom_variant_id AND variant.bom_spec_id=fi.bom_spec_id
		WHERE COALESCE(fi.onhand_units,0)>0 OR COALESCE(fi.onhand_loose_g,0)>0
		ORDER BY fi.product_id, fi.spec_g, w.sort_order, fi.warehouse%s
	`, r.schema, r.schema, r.schema, r.schema, r.schema, lockClause), customerID)
	if err != nil {
		return miniStockSnapshot{}, err
	}
	inventory := make([]miniFinishedInventoryRow, 0)
	for rows.Next() {
		var row miniFinishedInventoryRow
		var units, looseG int64
		if err := rows.Scan(&row.ProductID, &row.BomSpecID, &row.BomVariantID, &row.ProductName, &row.ParentProductID, &row.SKUCode, &row.BomSpecKey, &row.BomSpecName, &row.InventoryUnit, &row.IsDefaultSpec, &row.SpecG, &row.Warehouse, &row.WarehouseName, &units, &looseG, &row.UpdatedAt); err != nil {
			rows.Close()
			return miniStockSnapshot{}, err
		}
		row.TotalQty = units
		if row.BomSpecID <= 0 && row.SpecG > 0 {
			row.TotalQty += looseG / row.SpecG
		}
		inventory = append(inventory, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return miniStockSnapshot{}, err
	}
	rows.Close()
	for idx := range inventory {
		if inventory[idx].BomSpecID <= 0 {
			continue
		}
		identity, identityErr := resolveCustomerFulfillmentBOMSpecIdentityTx(
			ctx, q, r.schema, inventory[idx].ProductID, inventory[idx].BomSpecID, 0,
		)
		if identityErr != nil {
			return miniStockSnapshot{}, identityErr
		}
		if identity.BomSpecID <= 0 {
			continue
		}
		inventory[idx].ProductID = identity.ProductID
		inventory[idx].ParentProductID = identity.ProductID
		inventory[idx].BomVariantID = identity.BomVariantID
		inventory[idx].ProductName = strings.TrimSpace(identity.ProductName + " " + identity.BomSpecName)
		inventory[idx].SKUCode = identity.SpecCode
		inventory[idx].BomSpecKey = identity.BomSpecKey
		inventory[idx].BomSpecName = identity.BomSpecName
		inventory[idx].InventoryUnit = identity.InventoryUnit
	}
	if len(inventory) == 0 {
		return miniStockSnapshot{Inventory: []miniFinishedInventoryRow{}, Batches: []miniFinishedBatchRow{}, Candidates: []miniStockCandidate{}}, nil
	}

	allReservations, legacyReservations, batchReservations, err := r.loadMiniStockReservations(ctx, q, customerID)
	if err != nil {
		return miniStockSnapshot{}, err
	}
	for idx := range inventory {
		inventory[idx].ReservedQty = allReservations[miniWarehouseStockKey(inventory[idx].ProductID, inventory[idx].BomSpecID, inventory[idx].SpecG, inventory[idx].Warehouse)]
	}

	batches, err := r.loadMiniFinishedBatches(ctx, q, customerID, batchReservations, lock)
	if err != nil {
		return miniStockSnapshot{}, err
	}
	remainingByWarehouse := map[string]int64{}
	for _, batch := range batches {
		key := miniWarehouseStockKey(batch.ProductID, batch.BomSpecID, batch.SpecG, batch.Warehouse)
		remainingByWarehouse[key] += batch.PhysicalQty
	}
	historicalUnsyncedG, err := r.loadMiniHistoricalUnsyncedTraceableDeductions(ctx, q, customerID)
	if err != nil {
		return miniStockSnapshot{}, err
	}
	// finished_inventory is the current aggregate for new shipments, while old
	// traceable shipments only reduced stock_batches. Subtract only those
	// identifiable historical batch-only deductions; the remainder beyond the
	// current traceable batches is genuine legacy inventory.
	effectiveInventory := inventory[:0]
	for idx := range inventory {
		key := miniWarehouseStockKey(inventory[idx].ProductID, inventory[idx].BomSpecID, inventory[idx].SpecG, inventory[idx].Warehouse)
		historicalUnsyncedQty := int64(0)
		if inventory[idx].BomSpecID <= 0 && inventory[idx].SpecG > 0 {
			historicalUnsyncedQty = historicalUnsyncedG[key] / inventory[idx].SpecG
		}
		legacyPhysical := inventory[idx].TotalQty - remainingByWarehouse[key] - historicalUnsyncedQty
		if legacyPhysical < 0 {
			legacyPhysical = 0
		}
		inventory[idx].LegacyQty = legacyPhysical
		inventory[idx].TotalQty = remainingByWarehouse[key] + legacyPhysical
		if inventory[idx].ReservedQty > inventory[idx].TotalQty {
			inventory[idx].ReservedQty = inventory[idx].TotalQty
		}
		if inventory[idx].TotalQty > 0 {
			effectiveInventory = append(effectiveInventory, inventory[idx])
		}
	}
	inventory = effectiveInventory
	candidates := make([]miniStockCandidate, 0, len(batches)+len(inventory))
	for _, batch := range batches {
		if !miniDirectShipQualityAvailable(batch.QualityStatus) {
			continue
		}
		available := batch.PhysicalQty - batch.ReservedQty
		if available <= 0 {
			continue
		}
		candidates = append(candidates, miniStockCandidate{
			ProductID: batch.ProductID, BomSpecID: batch.BomSpecID, BomVariantID: batch.BomVariantID, SpecG: batch.SpecG, Warehouse: batch.Warehouse,
			BatchID: batch.BatchID, BatchCode: batch.BatchCode, AvailableQty: available, CreatedAt: batch.CreatedAt,
		})
	}
	for _, row := range inventory {
		key := miniWarehouseStockKey(row.ProductID, row.BomSpecID, row.SpecG, row.Warehouse)
		legacyAvailable := row.LegacyQty - legacyReservations[key]
		if legacyAvailable <= 0 {
			continue
		}
		candidates = append(candidates, miniStockCandidate{
			ProductID: row.ProductID, BomSpecID: row.BomSpecID, BomVariantID: row.BomVariantID, SpecG: row.SpecG, Warehouse: row.Warehouse,
			BatchCode: miniLegacyBatchCode(row.Warehouse, row.ProductID, row.BomSpecID, row.SpecG), AvailableQty: legacyAvailable,
			CreatedAt: row.UpdatedAt, Legacy: true,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Legacy != candidates[j].Legacy {
			return !candidates[i].Legacy
		}
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		if candidates[i].Warehouse != candidates[j].Warehouse {
			return candidates[i].Warehouse < candidates[j].Warehouse
		}
		return candidates[i].BatchID < candidates[j].BatchID
	})
	capacity := make(map[string]int64, len(inventory))
	for _, row := range inventory {
		key := miniWarehouseStockKey(row.ProductID, row.BomSpecID, row.SpecG, row.Warehouse)
		capacity[key] = row.TotalQty - row.ReservedQty
		if capacity[key] < 0 {
			capacity[key] = 0
		}
	}
	capped := candidates[:0]
	for _, candidate := range candidates {
		key := miniWarehouseStockKey(candidate.ProductID, candidate.BomSpecID, candidate.SpecG, candidate.Warehouse)
		if capacity[key] <= 0 {
			continue
		}
		if candidate.AvailableQty > capacity[key] {
			candidate.AvailableQty = capacity[key]
		}
		capacity[key] -= candidate.AvailableQty
		if candidate.AvailableQty > 0 {
			capped = append(capped, candidate)
		}
	}
	candidates = capped
	return miniStockSnapshot{Inventory: inventory, Batches: batches, Candidates: candidates}, nil
}

func (r *Repository) loadMiniHistoricalUnsyncedTraceableDeductions(ctx context.Context, q miniDirectShipQuerier, customerID int64) (map[string]int64, error) {
	out := map[string]int64{}
	if customerID <= 0 || !relationExists(ctx, q, fmt.Sprintf("%s.orders", r.schema)) {
		return out, nil
	}
	hasDeductions := relationExists(ctx, q, fmt.Sprintf("%s.order_stock_deductions", r.schema))
	if hasDeductions {
		rows, err := q.Query(ctx, fmt.Sprintf(`
			SELECT d.product_id,COALESCE(d.bom_spec_id,0),d.spec_g,
			       COALESCE(NULLIF(o.source_warehouse,''),'finished_goods') AS warehouse,
			       SUM(d.deducted_g)::bigint
			FROM %s.order_stock_deductions d
			JOIN %s.orders o ON o.id=d.order_id AND o.customer_id=$1
			JOIN %s.warehouses w ON w.code=COALESCE(NULLIF(o.source_warehouse,''),'finished_goods')
			  AND w.active=true AND w.kind IN ('finished','customer_processing','customer_finished','customer')
			  AND w.customer_id=$1
			WHERE d.batch_id>0 AND d.deducted_g>0
			  AND COALESCE(d.source_doc_type,'')<>$2
			GROUP BY d.product_id,COALESCE(d.bom_spec_id,0),d.spec_g,COALESCE(NULLIF(o.source_warehouse,''),'finished_goods')
		`, r.schema, r.schema, r.schema), customerID, inventorydomain.TraceableShipmentAggregateSyncSource)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var productID, bomSpecID, specG, deductedG int64
			var warehouse string
			if err := rows.Scan(&productID, &bomSpecID, &specG, &warehouse, &deductedG); err != nil {
				rows.Close()
				return nil, err
			}
			out[miniWarehouseStockKey(productID, bomSpecID, specG, warehouse)] += deductedG
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	if !relationExists(ctx, q, fmt.Sprintf("%s.stock_ledger_entries", r.schema)) || !relationExists(ctx, q, fmt.Sprintf("%s.stock_batches", r.schema)) {
		return out, nil
	}
	deductionExclusion := ""
	if hasDeductions {
		deductionExclusion = fmt.Sprintf(`
			AND NOT EXISTS (
				SELECT 1 FROM %s.order_stock_deductions d
				WHERE d.order_id=l.source_doc_id
				  AND d.product_id=l.item_id AND d.bom_spec_id=l.bom_spec_id AND d.spec_g=l.spec_g
				  AND d.batch_id=b.id AND d.batch_code=l.source_batch_code
			)`, r.schema)
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT l.item_id,COALESCE(l.bom_spec_id,0),l.spec_g,l.warehouse,SUM(-l.qty_change_g)::bigint
		FROM %s.stock_ledger_entries l
		JOIN %s.stock_batches b ON b.id>0 AND b.item_type='finished_product'
		  AND b.id=(SELECT b2.id FROM %s.stock_batches b2
		            WHERE b2.batch_code=l.source_batch_code AND b2.item_type=l.item_type
		              AND b2.item_id=l.item_id AND b2.bom_spec_id=l.bom_spec_id AND b2.spec_g=l.spec_g
		            ORDER BY b2.id LIMIT 1)
		JOIN %s.orders o ON o.id=l.source_doc_id AND o.customer_id=$1
		JOIN %s.warehouses w ON w.code=l.warehouse AND w.active=true
		  AND w.kind IN ('finished','customer_processing','customer_finished','customer') AND w.customer_id=$1
		WHERE l.source_doc_type=$2 AND l.item_type='finished_product' AND l.qty_change_g<0
		%s
		GROUP BY l.item_id,COALESCE(l.bom_spec_id,0),l.spec_g,l.warehouse
	`, r.schema, r.schema, r.schema, r.schema, r.schema, deductionExclusion), customerID, inventorydomain.ShipmentStockDeductionSource)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var productID, bomSpecID, specG, deductedG int64
		var warehouse string
		if err := rows.Scan(&productID, &bomSpecID, &specG, &warehouse, &deductedG); err != nil {
			return nil, err
		}
		out[miniWarehouseStockKey(productID, bomSpecID, specG, warehouse)] += deductedG
	}
	return out, rows.Err()
}

func (r *Repository) loadMiniStockReservations(ctx context.Context, q miniDirectShipQuerier, customerID int64) (map[string]int64, map[string]int64, map[int64]int64, error) {
	all := map[string]int64{}
	legacy := map[string]int64{}
	byBatch := map[int64]int64{}
	if !relationExists(ctx, q, fmt.Sprintf("%s.order_stock_batch_allocations", r.schema)) {
		return all, legacy, byBatch, nil
	}
	hasDeductions := relationExists(ctx, q, fmt.Sprintf("%s.order_stock_deductions", r.schema))
	deductionPredicate := "WHERE COALESCE(o.is_void,false)=false"
	if hasDeductions {
		deductionPredicate += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM %s.order_stock_deductions d WHERE d.order_id=a.order_id)", r.schema)
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT a.product_id,COALESCE(a.bom_spec_id,0),a.spec_g,
		       COALESCE(NULLIF(a.warehouse,''), NULLIF(o.source_warehouse,''), 'finished_goods') AS warehouse,
		       a.batch_id,
		       SUM(CASE WHEN COALESCE(a.bom_spec_id,0)>0 THEN COALESCE(a.allocated_units,0)
		                WHEN a.spec_g>0 THEN a.allocated_g/a.spec_g ELSE 0 END)::bigint AS qty
		FROM %s.order_stock_batch_allocations a
		JOIN %s.orders o ON o.id=a.order_id
		JOIN %s.warehouses w ON w.code=COALESCE(NULLIF(a.warehouse,''), NULLIF(o.source_warehouse,''), 'finished_goods')
		  AND w.active=true AND w.kind IN ('finished','customer_processing','customer_finished','customer') AND w.customer_id=$1
		%s
		GROUP BY a.product_id,COALESCE(a.bom_spec_id,0),a.spec_g,
		         COALESCE(NULLIF(a.warehouse,''), NULLIF(o.source_warehouse,''), 'finished_goods'),
		         a.batch_id
	`, r.schema, r.schema, r.schema, deductionPredicate), customerID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var productID, bomSpecID, specG, batchID, qty int64
		var warehouse string
		if err := rows.Scan(&productID, &bomSpecID, &specG, &warehouse, &batchID, &qty); err != nil {
			return nil, nil, nil, err
		}
		key := miniWarehouseStockKey(productID, bomSpecID, specG, warehouse)
		all[key] += qty
		if batchID > 0 {
			byBatch[batchID] += qty
		} else {
			legacy[key] += qty
		}
	}
	return all, legacy, byBatch, rows.Err()
}

func (r *Repository) loadMiniFinishedBatches(ctx context.Context, q miniDirectShipQuerier, customerID int64, reservations map[int64]int64, lock bool) ([]miniFinishedBatchRow, error) {
	if !relationExists(ctx, q, fmt.Sprintf("%s.stock_batches", r.schema)) || !relationExists(ctx, q, fmt.Sprintf("%s.stock_ledger_entries", r.schema)) {
		return []miniFinishedBatchRow{}, nil
	}
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF b"
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,b.item_id,COALESCE(b.bom_spec_id,0),COALESCE(b.bom_variant_id,0),
		       CASE WHEN COALESCE(b.bom_spec_id,0)>0
		         THEN btrim(COALESCE(NULLIF(p.name,''),'') || ' ' || COALESCE(NULLIF(variant.spec_name_snapshot,''),NULLIF(spec.name,''),''))
		         ELSE COALESCE(NULLIF(p.sku_name,''),NULLIF(b.item_name,''),NULLIF(p.name,''),'') END,
		       CASE WHEN COALESCE(b.bom_spec_id,0)>0 THEN COALESCE(spec.code,'') ELSE COALESCE(p.sku_code,'') END,
		       COALESCE(spec.spec_key,''),COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,''),b.spec_g,
		       latest.warehouse, COALESCE(w.name,latest.warehouse),
		       GREATEST(COALESCE(b.qty_units,0), CASE WHEN b.spec_g>0 THEN COALESCE(b.qty_g,0)/b.spec_g ELSE 0 END,
		                COALESCE(b.remaining_units,0), CASE WHEN b.spec_g>0 THEN COALESCE(b.remaining_g,0)/b.spec_g ELSE 0 END),
		       GREATEST(COALESCE(b.remaining_units,0), CASE WHEN b.spec_g>0 THEN COALESCE(b.remaining_g,0)/b.spec_g ELSE 0 END),
		       COALESCE(NULLIF(b.quality_status,''),'unchecked'), b.source_doc_type, b.source_doc_id, b.created_at,
		       inbound.inbound_at
		FROM %s.stock_batches b
		JOIN LATERAL (
			SELECT l.warehouse
			FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
			  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g
			ORDER BY l.id DESC LIMIT 1
		) latest ON true
		JOIN %s.warehouses w ON w.code=latest.warehouse AND w.active=true
		  AND w.kind IN ('finished','customer_processing','customer_finished','customer') AND w.customer_id=$1
		LEFT JOIN %s.products p ON p.id=b.item_id
		LEFT JOIN %s.production_bom_specs spec ON spec.id=b.bom_spec_id
		LEFT JOIN %s.production_bom_version_variants variant ON variant.id=b.bom_variant_id AND variant.bom_spec_id=b.bom_spec_id
		LEFT JOIN LATERAL (
			SELECT MIN(l.created_at) AS inbound_at
			FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.warehouse=latest.warehouse
			  AND (l.qty_change_units>0 OR l.qty_change_g>0)
		) inbound ON true
		WHERE b.item_type='finished_product'
		  AND (COALESCE(b.qty_units,0)>0 OR COALESCE(b.qty_g,0)>0 OR COALESCE(b.remaining_units,0)>0 OR COALESCE(b.remaining_g,0)>0)
		ORDER BY b.created_at, b.id%s
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, lockClause), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]miniFinishedBatchRow, 0)
	for rows.Next() {
		var row miniFinishedBatchRow
		if err := rows.Scan(
			&row.BatchID, &row.BatchCode, &row.ProductID, &row.BomSpecID, &row.BomVariantID,
			&row.ProductName, &row.SKUCode, &row.BomSpecKey, &row.BomSpecName, &row.InventoryUnit,
			&row.SpecG, &row.Warehouse, &row.WarehouseName, &row.OriginalQty, &row.PhysicalQty,
			&row.QualityStatus, &row.SourceDocType, &row.SourceDocID, &row.CreatedAt, &row.InboundAt,
		); err != nil {
			return nil, err
		}
		row.ReservedQty = reservations[row.BatchID]
		out = append(out, row)
	}
	return out, rows.Err()
}

func miniDirectShipQualityAvailable(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "hold", "held", "frozen", "blocked", "rejected", "reject", "failed", "冻结", "冻结批次", "不合格", "拒收":
		return false
	default:
		return true
	}
}

func miniStockKey(productID, bomSpecID, specG int64) string {
	return fmt.Sprintf("%d:%d:%d", productID, bomSpecID, specG)
}

func miniWarehouseStockKey(productID, bomSpecID, specG int64, warehouse string) string {
	return fmt.Sprintf("%d:%d:%d:%s", productID, bomSpecID, specG, strings.TrimSpace(warehouse))
}

func miniLegacyBatchCode(warehouse string, productID, bomSpecID, specG int64) string {
	return fmt.Sprintf("LEGACY:%s:%d:%d:%d", strings.TrimSpace(warehouse), productID, bomSpecID, specG)
}

func miniDirectShipRequestHash(cmd app.MiniDirectShipCommand) (string, error) {
	type itemHash struct {
		ProductID int64 `json:"product_id"`
		BomSpecID int64 `json:"bom_spec_id,omitempty"`
		SpecG     int64 `json:"spec_g"`
		Qty       int64 `json:"qty"`
	}
	items := make([]itemHash, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		items = append(items, itemHash{ProductID: item.ProductID, BomSpecID: item.BomSpecID, SpecG: item.SpecG, Qty: item.Qty})
	}
	payload := struct {
		RecipientName    string     `json:"recipient_name"`
		RecipientPhone   string     `json:"recipient_phone"`
		Province         string     `json:"province"`
		City             string     `json:"city"`
		District         string     `json:"district"`
		DetailAddress    string     `json:"detail_address"`
		RecipientCompany string     `json:"recipient_company"`
		Items            []itemHash `json:"items"`
		Note             string     `json:"note"`
	}{cmd.RecipientName, cmd.RecipientPhone, cmd.Province, cmd.City, cmd.District, cmd.DetailAddress, cmd.RecipientCompany, items, cmd.Note}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type miniDirectShipProductSnapshot struct {
	ProductID     int64
	BomSpecID     int64
	BomVariantID  int64
	BomSpecKey    string
	ProductName   string
	SKUCode       string
	SpecLabel     string
	InventoryUnit string
	ProductKind   string
}

func (r *Repository) resolveMiniDirectShipItems(ctx context.Context, q miniDirectShipQuerier, items []app.MiniDirectShipItemCommand) ([]app.MiniDirectShipItemCommand, error) {
	out := append([]app.MiniDirectShipItemCommand(nil), items...)
	for idx := range out {
		item := &out[idx]
		identity, err := resolveCustomerFulfillmentBOMSpecIdentityTx(ctx, q, r.schema, item.ProductID, item.BomSpecID, item.BomVariantID)
		if err != nil {
			return nil, err
		}
		if identity.ProductID <= 0 {
			continue
		}
		item.ProductID = identity.ProductID
		item.BomSpecID = identity.BomSpecID
		item.BomVariantID = identity.BomVariantID
		item.BomSpecKey = identity.BomSpecKey
		item.ProductName = strings.TrimSpace(identity.ProductName + " " + identity.BomSpecName)
		item.SKUCode = identity.SpecCode
		item.SpecLabel = identity.BomSpecName
		item.InventoryUnit = identity.InventoryUnit
		item.SpecG = 0
	}
	return out, nil
}

func (r *Repository) SubmitMiniDirectShip(ctx context.Context, cmd app.MiniDirectShipCommand) (app.MiniDirectShipRequest, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("mini-direct-ship:%d", cmd.CustomerID)); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	items, err := r.resolveMiniDirectShipItems(ctx, tx, cmd.Items)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	cmd.Items = items
	requestHash, err := miniDirectShipRequestHash(cmd)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}

	var existingID int64
	var existingHash string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, request_hash
		FROM %s.customer_direct_ship_requests
		WHERE customer_id=$1 AND idempotency_key=$2
		FOR UPDATE
	`, r.schema), cmd.CustomerID, cmd.IdempotencyKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != requestHash {
			return app.MiniDirectShipRequest{}, app.ErrMiniDirectShipIdempotency
		}
		return r.loadMiniDirectShipRequest(ctx, tx, cmd.CustomerID, existingID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return app.MiniDirectShipRequest{}, err
	}

	snapshot, err := r.loadMiniCustomerFinishedStock(ctx, tx, cmd.CustomerID, true)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	allocations, shortages := planMiniDirectShipAllocations(cmd.Items, snapshot.Candidates)
	if len(shortages) > 0 {
		return app.MiniDirectShipRequest{}, &miniDirectShipStockError{Shortages: shortages}
	}
	products, err := r.loadMiniDirectShipProductSnapshots(ctx, tx, cmd.Items)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}

	var requestID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_direct_ship_requests(
			customer_id, employee_id, mini_user_id, idempotency_key, request_hash,
			recipient_name, recipient_phone, province, city, district, detail_address,
			recipient_company, status, note, created_by
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'reserved',$13,$14)
		RETURNING id
	`, r.schema), cmd.CustomerID, cmd.EmployeeID, cmd.MiniUserID, cmd.IdempotencyKey, requestHash,
		cmd.RecipientName, cmd.RecipientPhone, cmd.Province, cmd.City, cmd.District, cmd.DetailAddress,
		cmd.RecipientCompany, cmd.Note, cmd.Actor).Scan(&requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	requestNo := fmt.Sprintf("DSR-%s-%06d", time.Now().Format("20060102"), requestID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_requests SET request_no=$2 WHERE id=$1`, r.schema), requestID, requestNo); err != nil {
		return app.MiniDirectShipRequest{}, err
	}

	requestItemIDs := make(map[string]int64, len(cmd.Items))
	for idx, item := range cmd.Items {
		key := miniStockKey(item.ProductID, item.BomSpecID, item.SpecG)
		product := products[key]
		snapshotJSON := mustPayloadJSON(map[string]any{
			"product_id": item.ProductID, "bom_spec_id": item.BomSpecID, "bom_variant_id": item.BomVariantID,
			"bom_spec_key": product.BomSpecKey, "product_name": product.ProductName,
			"sku_code": product.SKUCode, "spec_label": product.SpecLabel, "inventory_unit": product.InventoryUnit,
			"spec_g": item.SpecG, "qty": item.Qty,
		})
		var requestItemID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_direct_ship_request_items(
				request_id,line_no,product_id,bom_spec_id,bom_variant_id,bom_spec_key,
				product_name,sku_code,spec_label,inventory_unit,spec_g,qty,snapshot
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
			RETURNING id
		`, r.schema), requestID, idx+1, item.ProductID, item.BomSpecID, item.BomVariantID, product.BomSpecKey,
			product.ProductName, product.SKUCode, product.SpecLabel, product.InventoryUnit, item.SpecG, item.Qty, snapshotJSON).Scan(&requestItemID); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		requestItemIDs[miniStockKey(item.ProductID, item.BomSpecID, item.SpecG)] = requestItemID
	}

	warehouseItems := miniDirectShipWarehouseItems(allocations)
	warehouses := make([]string, 0, len(warehouseItems))
	for warehouse := range warehouseItems {
		warehouses = append(warehouses, warehouse)
	}
	sort.Strings(warehouses)
	payStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "ship_statuses", "待发货", "未发货")
	processStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "order_process_statuses", "待处理", "待生产")
	fullAddress := strings.TrimSpace(cmd.Province + cmd.City + cmd.District + cmd.DetailAddress)

	requestOrderIDs := make(map[string]int64, len(warehouses))
	orderIDs := make(map[string]int64, len(warehouses))
	orderItemIDs := make(map[string]int64)
	for index, warehouse := range warehouses {
		orderNo := fmt.Sprintf("%s-%02d", requestNo, index+1)
		var orderID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.orders(
				order_no, order_date, customer_id, pay_status_id, ship_status_id, process_status_id,
				portal_service_code, source_warehouse, receiver_name, receiver_phone,
				receiver_address, receiver_company, notes, created_at
			) VALUES($1,now()::date,$2,$3,$4,$5,'direct_ship',$6,$7,$8,$9,$10,$11,now())
			RETURNING id
		`, r.schema), orderNo, cmd.CustomerID, nullableCustomerFulfillmentID(payStatusID), nullableCustomerFulfillmentID(shipStatusID), nullableCustomerFulfillmentID(processStatusID),
			warehouse, cmd.RecipientName, cmd.RecipientPhone, fullAddress, cmd.RecipientCompany, cmd.Note).Scan(&orderID); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		var requestOrderID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_direct_ship_request_orders(request_id,order_id,warehouse_code,order_no,status)
			VALUES($1,$2,$3,$4,'reserved') RETURNING id
		`, r.schema), requestID, orderID, warehouse, orderNo).Scan(&requestOrderID); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		requestOrderIDs[warehouse] = requestOrderID
		orderIDs[warehouse] = orderID
		items := warehouseItems[warehouse]
		for lineNo, item := range items {
			key := miniStockKey(item.ProductID, item.BomSpecID, item.SpecG)
			product := products[key]
			unit := "件"
			spec := fmt.Sprintf("%dg", item.SpecG)
			if item.BomSpecID > 0 {
				unit = product.InventoryUnit
				spec = product.SpecLabel
			}
			var orderItemID int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.order_items(
					order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,qty,unit,spec,unit_price,
					line_total_before_discount,discount_type,discount_value,discount_amount,line_total,product_kind
				) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,0,0,'',0,0,0,$10)
				RETURNING id
			`, r.schema), orderID, lineNo+1, item.ProductID, item.BomSpecID, item.BomVariantID,
				product.ProductName, item.Qty, unit, spec, product.ProductKind).Scan(&orderItemID); err != nil {
				return app.MiniDirectShipRequest{}, err
			}
			orderItemIDs[miniWarehouseStockKey(item.ProductID, item.BomSpecID, item.SpecG, warehouse)] = orderItemID
		}
	}

	for _, allocation := range allocations {
		requestItemID := requestItemIDs[miniStockKey(allocation.ProductID, allocation.BomSpecID, allocation.SpecG)]
		requestOrderID := requestOrderIDs[allocation.Warehouse]
		orderID := orderIDs[allocation.Warehouse]
		orderItemID := orderItemIDs[miniWarehouseStockKey(allocation.ProductID, allocation.BomSpecID, allocation.SpecG, allocation.Warehouse)]
		allocatedG := allocation.Qty * allocation.SpecG
		allocatedUnits := int64(0)
		if allocation.BomSpecID > 0 {
			allocatedG = 0
			allocatedUnits = allocation.Qty
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_direct_ship_request_allocations(
				request_id,request_item_id,request_order_id,order_id,order_item_id,
				product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,
				allocated_qty,allocated_units,allocated_g,status
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,'reserved')
		`, r.schema), requestID, requestItemID, requestOrderID, orderID, orderItemID,
			allocation.ProductID, allocation.BomSpecID, allocation.BomVariantID, allocation.SpecG, allocation.Warehouse,
			allocation.BatchID, allocation.BatchCode, allocation.Qty, allocatedUnits, allocatedG); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		needUnits := int64(0)
		if allocation.BomSpecID > 0 {
			needUnits = allocation.Qty
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_stock_batch_allocations(
				order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,
				batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$7,$11,$12,$13,$14)
		`, r.schema), orderID, orderItemID, allocation.ProductID, allocation.BomSpecID, allocation.BomVariantID,
			allocation.SpecG, allocatedG, needUnits, allocation.BatchID, allocation.BatchCode,
			allocatedUnits, allocation.Warehouse, requestID, cmd.Actor); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
	}

	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_direct_ship_request", &requestID, "submit", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("reserved"), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "request_no": requestNo, "item_count": len(cmd.Items),
		"package_count": len(warehouses), "warehouses": warehouses, "idempotency_key": cmd.IdempotencyKey,
	}); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	return r.GetMiniDirectShipRequest(ctx, cmd.CustomerID, requestID)
}

type miniDirectShipStockError struct {
	Shortages []app.MiniDirectShipShortage
}

func (e *miniDirectShipStockError) Error() string {
	return app.ErrMiniDirectShipStockInsufficient.Error()
}

func (e *miniDirectShipStockError) Unwrap() error {
	return app.ErrMiniDirectShipStockInsufficient
}

func miniDirectShipWarehouseItems(allocations []miniPlannedAllocation) map[string][]app.MiniDirectShipItemCommand {
	out := map[string][]app.MiniDirectShipItemCommand{}
	indexes := map[string]map[string]int{}
	for _, allocation := range allocations {
		if indexes[allocation.Warehouse] == nil {
			indexes[allocation.Warehouse] = map[string]int{}
		}
		key := miniStockKey(allocation.ProductID, allocation.BomSpecID, allocation.SpecG)
		if idx, ok := indexes[allocation.Warehouse][key]; ok {
			out[allocation.Warehouse][idx].Qty += allocation.Qty
			continue
		}
		indexes[allocation.Warehouse][key] = len(out[allocation.Warehouse])
		out[allocation.Warehouse] = append(out[allocation.Warehouse], app.MiniDirectShipItemCommand{
			ProductID: allocation.ProductID, BomSpecID: allocation.BomSpecID, BomVariantID: allocation.BomVariantID,
			SpecG: allocation.SpecG, Qty: allocation.Qty,
		})
	}
	return out
}

func (r *Repository) loadMiniDirectShipProductSnapshots(ctx context.Context, q miniDirectShipQuerier, items []app.MiniDirectShipItemCommand) (map[string]miniDirectShipProductSnapshot, error) {
	ids := make([]int64, 0, len(items))
	seen := map[int64]bool{}
	for _, item := range items {
		if !seen[item.ProductID] {
			seen[item.ProductID] = true
			ids = append(ids, item.ProductID)
		}
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(NULLIF(sku_name,''),NULLIF(name,''),''), COALESCE(sku_code,''),
		       COALESCE(NULLIF(spec_label,''), CASE WHEN net_content_qty>0 THEN trim(to_char(net_content_qty,'FM999999990.###')) || net_content_unit ELSE '' END, ''),
		       COALESCE(product_kind,'')
		FROM %s.products WHERE id=ANY($1::bigint[]) AND active=true
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	base := make(map[int64]miniDirectShipProductSnapshot, len(ids))
	for rows.Next() {
		var row miniDirectShipProductSnapshot
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.SKUCode, &row.SpecLabel, &row.ProductKind); err != nil {
			return nil, err
		}
		base[row.ProductID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if base[id].ProductID <= 0 {
			return nil, fmt.Errorf("product %d unavailable", id)
		}
	}
	out := make(map[string]miniDirectShipProductSnapshot, len(items))
	for _, item := range items {
		row := base[item.ProductID]
		row.BomSpecID = item.BomSpecID
		row.BomVariantID = item.BomVariantID
		if item.BomSpecID > 0 {
			row.BomSpecKey = item.BomSpecKey
			row.ProductName = item.ProductName
			row.SKUCode = item.SKUCode
			row.SpecLabel = item.SpecLabel
			row.InventoryUnit = item.InventoryUnit
		}
		out[miniStockKey(item.ProductID, item.BomSpecID, item.SpecG)] = row
	}
	return out, nil
}

func (r *Repository) ListMiniDirectShipRequests(ctx context.Context, query app.MiniDirectShipListQuery) (app.MiniDirectShipListResult, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	where := []string{"r.customer_id=$1"}
	args := []any{query.CustomerID}
	for _, keyword := range strings.Fields(strings.TrimSpace(query.Q)) {
		args = append(args, "%"+strings.ToLower(keyword)+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(`(
			LOWER(COALESCE(r.recipient_company,'')) LIKE %[1]s
			OR LOWER(COALESCE(r.recipient_name,'')) LIKE %[1]s
			OR LOWER(COALESCE(r.recipient_phone,'')) LIKE %[1]s
			OR LOWER(CONCAT_WS('',r.province,r.city,r.district,r.detail_address)) LIKE %[1]s
			OR LOWER(COALESCE(c.name,'')) LIKE %[1]s
		)`, placeholder))
	}
	shipmentWhere := make([]string, 0, 2)
	if value := strings.TrimSpace(query.ShippedFrom); value != "" {
		from, err := miniDirectShipShanghaiDate(value)
		if err != nil {
			return app.MiniDirectShipListResult{}, fmt.Errorf("shipped_from invalid")
		}
		args = append(args, from)
		shipmentWhere = append(shipmentWhere, fmt.Sprintf("effective.shipped_at >= $%d", len(args)))
	}
	if value := strings.TrimSpace(query.ShippedTo); value != "" {
		to, err := miniDirectShipShanghaiDate(value)
		if err != nil {
			return app.MiniDirectShipListResult{}, fmt.Errorf("shipped_to invalid")
		}
		args = append(args, to.AddDate(0, 0, 1))
		shipmentWhere = append(shipmentWhere, fmt.Sprintf("effective.shipped_at < $%d", len(args)))
	}
	if len(shipmentWhere) > 0 {
		effectiveShipmentTime := miniDirectShipEffectiveShipmentTimeSQL(
			r.schema,
			"ro.order_id",
			relationExists(ctx, r.pool, fmt.Sprintf("%s.order_shipment_orders", r.schema)),
			relationExists(ctx, r.pool, fmt.Sprintf("%s.order_shipping_trackings", r.schema)),
		)
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM %s.customer_direct_ship_request_orders ro
			CROSS JOIN LATERAL (SELECT %s AS shipped_at) effective
			WHERE ro.request_id=r.id AND effective.shipped_at IS NOT NULL AND %s
		)`, r.schema, effectiveShipmentTime, strings.Join(shipmentWhere, " AND ")))
	}
	fromClause := fmt.Sprintf(`
		FROM %s.customer_direct_ship_requests r
		LEFT JOIN %s.customers c ON c.id=r.customer_id
		WHERE %s
	`, r.schema, r.schema, strings.Join(where, " AND "))
	var total64 int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+fromClause, args...).Scan(&total64); err != nil {
		return app.MiniDirectShipListResult{}, err
	}
	total := int(total64)
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
		if page > totalPages {
			page = totalPages
		}
	} else {
		page = 1
	}
	offset := (page - 1) * limit
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id %s
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $%d OFFSET $%d
	`, fromClause, len(args)+1, len(args)+2), listArgs...)
	if err != nil {
		return app.MiniDirectShipListResult{}, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return app.MiniDirectShipListResult{}, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return app.MiniDirectShipListResult{}, err
	}
	rows.Close()
	out := make([]app.MiniDirectShipRequest, 0, len(ids))
	for _, id := range ids {
		row, err := r.loadMiniDirectShipRequest(ctx, r.pool, query.CustomerID, id)
		if err != nil {
			return app.MiniDirectShipListResult{}, err
		}
		out = append(out, row)
	}
	return app.MiniDirectShipListResult{
		Rows: out, Total: total, Page: page, Limit: limit, TotalPages: totalPages, HasNext: page < totalPages,
	}, nil
}

func miniDirectShipShanghaiDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.FixedZone("Asia/Shanghai", 8*60*60))
}

func miniDirectShipEffectiveShipmentTimeSQL(schema, orderIDExpression string, hasShipmentOrders, hasTrackings bool) string {
	shipmentTime := "NULL::timestamptz"
	if hasShipmentOrders {
		shipmentTime = fmt.Sprintf(`(
			SELECT MAX(effective_so.shipped_at)
			FROM %s.order_shipment_orders effective_so
			WHERE effective_so.order_id=%s
		)`, schema, orderIDExpression)
	}
	trackingTime := "NULL::timestamptz"
	if hasTrackings {
		trackingTime = fmt.Sprintf(`(
			SELECT MIN(effective_tracking.created_at)
			FROM %s.order_shipping_trackings effective_tracking
			WHERE effective_tracking.order_id=%s
		)`, schema, orderIDExpression)
	}
	if hasShipmentOrders && hasTrackings {
		return fmt.Sprintf("COALESCE(%s,%s)", shipmentTime, trackingTime)
	}
	if hasShipmentOrders {
		return shipmentTime
	}
	return trackingTime
}

func (r *Repository) GetMiniDirectShipRequest(ctx context.Context, customerID, requestID int64) (app.MiniDirectShipRequest, error) {
	return r.loadMiniDirectShipRequest(ctx, r.pool, customerID, requestID)
}

func (r *Repository) loadMiniDirectShipRequest(ctx context.Context, q miniDirectShipQuerier, customerID, requestID int64) (app.MiniDirectShipRequest, error) {
	var out app.MiniDirectShipRequest
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, request_no, status, recipient_name, recipient_phone, province, city, district,
		       detail_address, recipient_company, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), note
		FROM %s.customer_direct_ship_requests
		WHERE id=$1 AND customer_id=$2
	`, r.schema), requestID, customerID).Scan(&out.ID, &out.RequestNo, &out.Status, &out.RecipientName, &out.RecipientPhone,
		&out.Province, &out.City, &out.District, &out.DetailAddress, &out.RecipientCompany, &out.CreatedAt, &out.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.MiniDirectShipRequest{}, app.ErrMiniDirectShipRequestNotFound
	}
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	itemRows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,bom_spec_key,product_name,sku_code,
		       spec_label,inventory_unit,spec_g,qty
		FROM %s.customer_direct_ship_request_items
		WHERE request_id=$1 ORDER BY line_no,id
	`, r.schema), requestID)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	out.Items = make([]app.MiniDirectShipItemCommand, 0)
	for itemRows.Next() {
		var item app.MiniDirectShipItemCommand
		if err := itemRows.Scan(&item.ProductID, &item.BomSpecID, &item.BomVariantID, &item.BomSpecKey,
			&item.ProductName, &item.SKUCode, &item.SpecLabel, &item.InventoryUnit, &item.SpecG, &item.Qty); err != nil {
			itemRows.Close()
			return app.MiniDirectShipRequest{}, err
		}
		out.Items = append(out.Items, item)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return app.MiniDirectShipRequest{}, err
	}
	itemRows.Close()

	effectiveShipmentTime := miniDirectShipEffectiveShipmentTimeSQL(
		r.schema,
		"o.id",
		relationExists(ctx, q, fmt.Sprintf("%s.order_shipment_orders", r.schema)),
		relationExists(ctx, q, fmt.Sprintf("%s.order_shipping_trackings", r.schema)),
	)
	shipmentJoin := fmt.Sprintf("LEFT JOIN LATERAL (SELECT %s AS shipped_at) shipment ON true", effectiveShipmentTime)
	packageRows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT ro.id,ro.order_id,ro.order_no,ro.warehouse_code,ro.status,
		       COALESCE(o.ship_method,''),
		       COALESCE(NULLIF(tracking.tracking_no,''),NULLIF(o.ship_tracking_no,''),''),
		       COALESCE(to_char(shipment.shipped_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),'')
		FROM %s.customer_direct_ship_request_orders ro
		JOIN %s.orders o ON o.id=ro.order_id
		LEFT JOIN LATERAL (
			SELECT string_agg(t.tracking_no, '、' ORDER BY t.id) AS tracking_no
			FROM %s.order_shipping_trackings t WHERE t.order_id=o.id
		) tracking ON true
		%s
		WHERE ro.request_id=$1 ORDER BY ro.id
	`, r.schema, r.schema, r.schema, shipmentJoin), requestID)
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	out.Packages = make([]app.MiniDirectShipPackage, 0)
	for packageRows.Next() {
		var pkg app.MiniDirectShipPackage
		if err := packageRows.Scan(&pkg.ID, &pkg.OrderID, &pkg.OrderNo, &pkg.Warehouse, &pkg.Status, &pkg.CarrierName, &pkg.TrackingNo, &pkg.ShippedAt); err != nil {
			packageRows.Close()
			return app.MiniDirectShipRequest{}, err
		}
		if pkg.Status != "cancelled" && (pkg.ShippedAt != "" || pkg.TrackingNo != "") {
			pkg.Status = "shipped"
		}
		out.Packages = append(out.Packages, pkg)
	}
	if err := packageRows.Err(); err != nil {
		packageRows.Close()
		return app.MiniDirectShipRequest{}, err
	}
	packageRows.Close()
	for idx := range out.Packages {
		out.Packages[idx].Items, err = r.loadMiniDirectShipPackageItems(ctx, q, requestID, out.Packages[idx].OrderID)
		if err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		out.Packages[idx].Events, err = r.loadMiniDirectShipTrackingEvents(ctx, q, out.Packages[idx])
		if err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		for _, event := range out.Packages[idx].Events {
			status := strings.ToLower(strings.TrimSpace(event.Status))
			if strings.Contains(status, "delivered") || strings.Contains(status, "signed") || strings.Contains(event.Status, "签收") || strings.Contains(event.Status, "妥投") {
				out.Packages[idx].DeliveredAt = event.Time
				out.Packages[idx].Status = "delivered"
			}
		}
	}
	if out.Status != "cancelled" && len(out.Packages) > 0 {
		shipped := 0
		delivered := 0
		for _, pkg := range out.Packages {
			if pkg.Status == "delivered" {
				delivered++
				shipped++
			} else if pkg.Status == "shipped" {
				shipped++
			}
		}
		if delivered == len(out.Packages) {
			out.Status = "delivered"
		} else if shipped == len(out.Packages) {
			out.Status = "shipped"
		} else if shipped > 0 {
			out.Status = "partially_shipped"
		}
	}
	return out, nil
}

func (r *Repository) loadMiniDirectShipPackageItems(ctx context.Context, q miniDirectShipQuerier, requestID, orderID int64) ([]app.MiniDirectShipItemCommand, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT a.product_id,a.bom_spec_id,MAX(a.bom_variant_id),MAX(i.bom_spec_key),
		       MAX(i.product_name),MAX(i.sku_code),MAX(i.spec_label),MAX(i.inventory_unit),
		       a.spec_g,SUM(a.allocated_qty)::bigint
		FROM %s.customer_direct_ship_request_allocations a
		JOIN %s.customer_direct_ship_request_items i ON i.id=a.request_item_id
		WHERE a.request_id=$1 AND a.order_id=$2
		GROUP BY a.product_id,a.bom_spec_id,a.spec_g ORDER BY a.product_id,a.bom_spec_id,a.spec_g
	`, r.schema, r.schema), requestID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.MiniDirectShipItemCommand, 0)
	for rows.Next() {
		var item app.MiniDirectShipItemCommand
		if err := rows.Scan(&item.ProductID, &item.BomSpecID, &item.BomVariantID, &item.BomSpecKey,
			&item.ProductName, &item.SKUCode, &item.SpecLabel, &item.InventoryUnit, &item.SpecG, &item.Qty); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) loadMiniDirectShipTrackingEvents(ctx context.Context, q miniDirectShipQuerier, pkg app.MiniDirectShipPackage) ([]app.MiniDirectShipTrackingEvent, error) {
	out := make([]app.MiniDirectShipTrackingEvent, 0)
	if relationExists(ctx, q, fmt.Sprintf("%s.order_shipping_tracking_events", r.schema)) {
		rows, err := q.Query(ctx, fmt.Sprintf(`
				SELECT to_char(event_time AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),status,description,location
			FROM %s.order_shipping_tracking_events
			WHERE order_id=$1 ORDER BY event_time,id
		`, r.schema), pkg.OrderID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var event app.MiniDirectShipTrackingEvent
			if err := rows.Scan(&event.Time, &event.Status, &event.Description, &event.Location); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, event)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	if len(out) == 0 && pkg.ShippedAt != "" {
		out = append(out, app.MiniDirectShipTrackingEvent{
			Time: pkg.ShippedAt, Status: "shipped", Description: "包裹已从仓库发出", Location: pkg.Warehouse,
		})
	}
	return out, nil
}

func (r *Repository) CancelMiniDirectShipRequest(ctx context.Context, customerID, requestID int64, actor string) (app.MiniDirectShipRequest, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("mini-direct-ship:%d", customerID)); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status FROM %s.customer_direct_ship_requests
		WHERE id=$1 AND customer_id=$2 FOR UPDATE
	`, r.schema), requestID, customerID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return app.MiniDirectShipRequest{}, app.ErrMiniDirectShipRequestNotFound
	} else if err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if status == "cancelled" {
		return r.loadMiniDirectShipRequest(ctx, tx, customerID, requestID)
	}
	if relationExists(ctx, tx, fmt.Sprintf("%s.order_stock_deductions", r.schema)) {
		var deducted bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1 FROM %s.order_stock_deductions d
				JOIN %s.customer_direct_ship_request_orders ro ON ro.order_id=d.order_id
				WHERE ro.request_id=$1
			)
		`, r.schema, r.schema), requestID).Scan(&deducted); err != nil {
			return app.MiniDirectShipRequest{}, err
		}
		if deducted {
			return app.MiniDirectShipRequest{}, app.ErrMiniDirectShipCannotCancel
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_stock_batch_allocations WHERE request_id=$1`, r.schema), requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_direct_ship_request_allocations SET status='released'
		WHERE request_id=$1 AND status='reserved'
	`, r.schema), requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders SET is_void=true,voided_at=now(),void_reason='客户取消发货申请'
		WHERE id IN (SELECT order_id FROM %s.customer_direct_ship_request_orders WHERE request_id=$1)
	`, r.schema, r.schema), requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_orders SET status='cancelled',updated_at=now() WHERE request_id=$1`, r.schema), requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_direct_ship_requests
		SET status='cancelled',cancelled_at=now(),updated_at=now() WHERE id=$1
	`, r.schema), requestID); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_direct_ship_request", &requestID, "cancel", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(status), postgresinfra.StrPtr("cancelled"), postgresinfra.AuditMeta{
		"customer_id": customerID, "released_reservations": true,
	}); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.MiniDirectShipRequest{}, err
	}
	return r.GetMiniDirectShipRequest(ctx, customerID, requestID)
}

func (r *Repository) ListCustomerCentralInventory(ctx context.Context, customerID int64) ([]app.CustomerInventorySummary, error) {
	snapshot, err := r.loadMiniCustomerFinishedStock(ctx, r.pool, customerID, false)
	if err != nil {
		return nil, err
	}
	availableByWarehouse := map[string]int64{}
	for _, candidate := range snapshot.Candidates {
		availableByWarehouse[miniWarehouseStockKey(candidate.ProductID, candidate.BomSpecID, candidate.SpecG, candidate.Warehouse)] += candidate.AvailableQty
	}
	type state struct {
		row        app.CustomerInventorySummary
		warehouses map[string]bool
	}
	states := make([]*state, 0)
	byKey := map[string]*state{}
	for _, inventory := range snapshot.Inventory {
		key := miniStockKey(inventory.ProductID, inventory.BomSpecID, inventory.SpecG)
		current := byKey[key]
		if current == nil {
			current = &state{row: app.CustomerInventorySummary{
				ProductID: inventory.ProductID, BomSpecID: inventory.BomSpecID, BomVariantID: inventory.BomVariantID,
				BomSpecKey: inventory.BomSpecKey, BomSpecName: inventory.BomSpecName,
				InventoryUnit: inventory.InventoryUnit, IsDefaultSpec: inventory.IsDefaultSpec,
				ProductName: inventory.ProductName, ParentProductID: inventory.ParentProductID,
				SKUCode: inventory.SKUCode, SpecG: inventory.SpecG,
				Warehouses: []string{},
			}, warehouses: map[string]bool{}}
			byKey[key] = current
			states = append(states, current)
		}
		current.row.TotalQty += inventory.TotalQty
		current.row.ReservedQty += inventory.ReservedQty
		current.row.AvailableQty += availableByWarehouse[miniWarehouseStockKey(inventory.ProductID, inventory.BomSpecID, inventory.SpecG, inventory.Warehouse)]
		if !current.warehouses[inventory.WarehouseName] {
			current.warehouses[inventory.WarehouseName] = true
			current.row.Warehouses = append(current.row.Warehouses, inventory.WarehouseName)
		}
	}
	out := make([]app.CustomerInventorySummary, 0, len(states))
	for _, current := range states {
		sort.Strings(current.row.Warehouses)
		out = append(out, current.row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ProductName != out[j].ProductName {
			return out[i].ProductName < out[j].ProductName
		}
		if out[i].BomSpecID != out[j].BomSpecID {
			return out[i].BomSpecID < out[j].BomSpecID
		}
		return out[i].SpecG < out[j].SpecG
	})
	return out, nil
}

func (r *Repository) ListCustomerCentralInventoryBatches(ctx context.Context, query app.CustomerInventoryBatchQuery) ([]app.CustomerInventoryBatch, error) {
	snapshot, err := r.loadMiniCustomerFinishedStock(ctx, r.pool, query.CustomerID, false)
	if err != nil {
		return nil, err
	}
	availableByBatch := map[int64]int64{}
	availableLegacy := map[string]int64{}
	for _, candidate := range snapshot.Candidates {
		if !miniCustomerInventoryIdentityMatches(query, candidate.ProductID, candidate.BomSpecID, candidate.SpecG) {
			continue
		}
		if candidate.BatchID > 0 {
			availableByBatch[candidate.BatchID] += candidate.AvailableQty
		} else {
			availableLegacy[miniWarehouseStockKey(candidate.ProductID, candidate.BomSpecID, candidate.SpecG, candidate.Warehouse)] += candidate.AvailableQty
		}
	}
	productionDates := map[int64]string{}
	if relationExists(ctx, r.pool, fmt.Sprintf("%s.produce_running_items", r.schema)) {
		ids := make([]int64, 0)
		for _, batch := range snapshot.Batches {
			if miniCustomerInventoryIdentityMatches(query, batch.ProductID, batch.BomSpecID, batch.SpecG) && batch.SourceDocType == "production_run" && batch.SourceDocID > 0 {
				ids = append(ids, batch.SourceDocID)
			}
		}
		if len(ids) > 0 {
			rows, err := r.pool.Query(ctx, fmt.Sprintf(`
				SELECT id,COALESCE(to_char(finished_at,'YYYY-MM-DD'),'') FROM %s.produce_running_items WHERE id=ANY($1::bigint[])
			`, r.schema), ids)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var id int64
				var date string
				if err := rows.Scan(&id, &date); err != nil {
					rows.Close()
					return nil, err
				}
				productionDates[id] = date
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		}
	}
	out := make([]app.CustomerInventoryBatch, 0)
	for _, batch := range snapshot.Batches {
		if !miniCustomerInventoryIdentityMatches(query, batch.ProductID, batch.BomSpecID, batch.SpecG) || batch.PhysicalQty <= 0 {
			continue
		}
		inboundAt := ""
		if batch.InboundAt != nil {
			inboundAt = batch.InboundAt.Format("2006-01-02 15:04:05")
		}
		productionDate := ""
		if batch.SourceDocType == "production_run" {
			productionDate = productionDates[batch.SourceDocID]
		}
		out = append(out, app.CustomerInventoryBatch{
			BatchID: batch.BatchID, BatchNo: batch.BatchCode, ProductID: batch.ProductID,
			BomSpecID: batch.BomSpecID, BomVariantID: batch.BomVariantID, BomSpecKey: batch.BomSpecKey,
			BomSpecName: batch.BomSpecName, InventoryUnit: batch.InventoryUnit,
			ProductName: batch.ProductName, SKUCode: batch.SKUCode, SpecG: batch.SpecG,
			Warehouse: batch.WarehouseName, ProductionDate: productionDate, InboundAt: inboundAt,
			AvailableQty: availableByBatch[batch.BatchID], ReservedQty: batch.ReservedQty,
			QualityStatus: batch.QualityStatus, HistoricalWithoutProductionDate: productionDate == "",
		})
	}
	for _, inventory := range snapshot.Inventory {
		if !miniCustomerInventoryIdentityMatches(query, inventory.ProductID, inventory.BomSpecID, inventory.SpecG) {
			continue
		}
		key := miniWarehouseStockKey(inventory.ProductID, inventory.BomSpecID, inventory.SpecG, inventory.Warehouse)
		legacyTotal := inventory.LegacyQty
		if legacyTotal <= 0 {
			continue
		}
		legacyReserved := legacyTotal - availableLegacy[key]
		if legacyReserved < 0 {
			legacyReserved = 0
		}
		out = append(out, app.CustomerInventoryBatch{
			BatchID: 0, BatchNo: miniLegacyBatchCode(inventory.Warehouse, inventory.ProductID, inventory.BomSpecID, inventory.SpecG),
			ProductID: inventory.ProductID, BomSpecID: inventory.BomSpecID, BomVariantID: inventory.BomVariantID,
			BomSpecKey: inventory.BomSpecKey, BomSpecName: inventory.BomSpecName, InventoryUnit: inventory.InventoryUnit,
			ProductName: inventory.ProductName, SKUCode: inventory.SKUCode,
			SpecG: inventory.SpecG, Warehouse: inventory.WarehouseName,
			InboundAt:    inventory.UpdatedAt.Format("2006-01-02 15:04:05"),
			AvailableQty: availableLegacy[key], ReservedQty: legacyReserved, QualityStatus: "historical",
			HistoricalWithoutProductionDate: true,
		})
	}
	return out, nil
}

func miniCustomerInventoryIdentityMatches(query app.CustomerInventoryBatchQuery, productID, bomSpecID, specG int64) bool {
	if productID != query.ProductID {
		return false
	}
	if query.BomSpecID > 0 {
		return bomSpecID == query.BomSpecID
	}
	return bomSpecID <= 0 && query.SpecG > 0 && specG == query.SpecG
}
