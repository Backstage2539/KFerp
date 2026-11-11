package production

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
)

// packagingUnissuedCandidatesTx exposes only unreserved stock from the same
// source batches and owners as this card's confirmed input. It does not assign
// any of that shared stock to the work order.
func (r Repository) packagingUnissuedCandidatesTx(ctx context.Context, tx pgx.Tx, cardID int64, bean app.OperationMaterialUse) ([]app.PackagingUnissuedRemainder, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT c.warehouse,c.owner_customer_id,COALESCE(m.batch_code,s.batch_code,'') FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs b ON b.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches s ON s.id=c.stock_batch_id WHERE b.job_card_id=$1 AND b.status='posted' AND c.component_key=$2`, r.schema), cardID, bean.Key)
	if err != nil {
		return nil, err
	}
	type source struct {
		warehouse, code string
		owner           int64
	}
	var sources []source
	for rows.Next() {
		var x source
		if err = rows.Scan(&x.warehouse, &x.owner, &x.code); err != nil {
			rows.Close()
			return nil, err
		}
		sources = append(sources, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	itemType, itemID := "material", bean.MaterialID
	if bean.ProductID > 0 {
		itemType = "finished_product"
		itemID = bean.ProductID
	}
	warehouses := map[string]bool{}
	for _, x := range sources {
		warehouses[x.warehouse] = true
	}
	out := []app.PackagingUnissuedRemainder{}
	for warehouse := range warehouses {
		for offset := 0; ; offset += 500 {
			opts, e := stockrepo.NewRepository(r.pool, r.schema).ListTransferOptionsTx(ctx, tx, stockapp.TransferOptionsQuery{Warehouse: warehouse, ItemType: itemType, ItemID: itemID, BomSpecID: bean.BomSpecID, SpecG: bean.SpecG, Limit: 500, Offset: offset})
			if e != nil {
				return nil, e
			}
			for _, o := range opts.Rows {
				if o.ItemID != itemID || o.OwnerCustomerID != bean.OwnerCustomerID || o.BomSpecID != bean.BomSpecID || o.BomVariantID != bean.BomVariantID || o.SpecG != bean.SpecG || o.AvailableG <= 0 || o.UnavailableReason != "" {
					continue
				}
				for _, x := range sources {
					if x.warehouse == warehouse && x.code == o.BatchCode && x.owner == o.OwnerCustomerID {
						out = append(out, app.PackagingUnissuedRemainder{BatchCode: o.BatchCode, Warehouse: warehouse, OwnerCustomerID: o.OwnerCustomerID, AvailableG: o.AvailableG})
						break
					}
				}
			}
			if !opts.HasNext {
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Warehouse != out[j].Warehouse {
			return out[i].Warehouse < out[j].Warehouse
		}
		if out[i].BatchCode != out[j].BatchCode {
			return out[i].BatchCode < out[j].BatchCode
		}
		return out[i].OwnerCustomerID < out[j].OwnerCustomerID
	})
	return out, nil
}

func (r Repository) transferPackagingUnissuedTx(ctx context.Context, tx pgx.Tx, p app.OperationCompletionPreview, cmd app.JobCardActionCommand, eventID int64, bean app.OperationMaterialUse) error {
	if len(cmd.SelectedUnissuedRemainders) == 0 {
		return nil
	}
	if cmd.FinishOperation == nil || !*cmd.FinishOperation {
		return fmt.Errorf("请在工单完成后选择需要转移的共享库存")
	}
	target := strings.TrimSpace(cmd.RemainderWarehouse)
	if target == "" {
		target = "wip"
	}
	candidates := map[string]app.PackagingUnissuedRemainder{}
	for _, c := range p.PackagingUnissuedCandidates {
		candidates[fmt.Sprintf("%s\x00%s\x00%d", c.Warehouse, c.BatchCode, c.OwnerCustomerID)] = c
	}
	seen := map[string]bool{}
	for _, x := range cmd.SelectedUnissuedRemainders {
		key := fmt.Sprintf("%s\x00%s\x00%d", x.Warehouse, x.BatchCode, x.OwnerCustomerID)
		candidate, ok := candidates[key]
		if !ok || seen[key] || x.QuantityG <= 0 || x.QuantityG > candidate.AvailableG {
			return fmt.Errorf("所选未领库存的批次、货主或可用量已变化，请刷新")
		}
		seen[key] = true
		if x.Warehouse == target {
			continue
		}
		item := stockapp.StockDocumentItemCommand{ItemType: "material", MaterialID: bean.MaterialID, ItemName: bean.Name, InventoryUnit: bean.Unit, BomSpecID: bean.BomSpecID, BomVariantID: bean.BomVariantID, SpecG: bean.SpecG, FromWarehouse: x.Warehouse, ToWarehouse: target, OwnerCustomerID: x.OwnerCustomerID, BatchCode: x.BatchCode, QtyG: x.QuantityG}
		if bean.ProductID > 0 {
			item.ItemType = "finished_product"
			item.ProductID = bean.ProductID
			item.MaterialID = 0
		}
		_, err := stockrepo.NewRepository(r.pool, r.schema).CreateAndSubmitStockDocumentTx(ctx, tx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, SourceType: "packaging_unissued_remainder", SourceID: eventID, Operator: cmd.Operator, IdempotencyKey: fmt.Sprintf("packaging-unissued:%d:%s:%s:%d", eventID, x.Warehouse, x.BatchCode, x.OwnerCustomerID), Items: []stockapp.StockDocumentItemCommand{item}})
		if err != nil {
			return err
		}
	}
	return nil
}
