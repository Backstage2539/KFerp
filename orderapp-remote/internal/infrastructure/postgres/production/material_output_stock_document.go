package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
)

func finalizeMaterialOutputStockDocumentTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	stockDocumentID, workOrderID, runningItemID, materialID int64,
	materialName, outputUnit, warehouse string,
	ownerCustomerID, finishedG, finishedUnits int64,
	batchCode, operator, note string,
) (productionapp.StockEntryDetail, error) {
	var status, purpose string
	var isReturn bool
	var boundWorkOrderID, boundRunningItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,purpose,is_return,work_order_id,running_item_id
		FROM %s.stock_entries WHERE id=$1 FOR UPDATE
	`, schema), stockDocumentID).Scan(&status, &purpose, &isReturn, &boundWorkOrderID, &boundRunningItemID); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.StockEntryDetail{}, fmt.Errorf("stock document not found")
		}
		return productionapp.StockEntryDetail{}, err
	}
	if status != "draft" {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document must be draft")
	}
	if purpose != "manufacture" || isReturn || boundWorkOrderID != workOrderID {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document does not match work order completion")
	}
	if boundRunningItemID > 0 && boundRunningItemID != runningItemID {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document running item does not match work order")
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,material_id,product_id,item_type,spec_g,inventory_unit,COALESCE(owner_customer_id,0),
		       from_warehouse,to_warehouse,qty_g,qty_units
		FROM %s.stock_entry_items
		WHERE stock_entry_id=$1
		ORDER BY id
		FOR UPDATE
	`, schema), stockDocumentID)
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	type frozenItem struct {
		ID                         int64
		MaterialID, ProductID      int64
		ItemType                   string
		SpecG                      int64
		InventoryUnit              string
		OwnerCustomerID            int64
		FromWarehouse, ToWarehouse string
		QtyG, QtyUnits             int64
	}
	items := make([]frozenItem, 0, 1)
	for rows.Next() {
		var item frozenItem
		if err := rows.Scan(
			&item.ID, &item.MaterialID, &item.ProductID, &item.ItemType, &item.SpecG, &item.InventoryUnit,
			&item.OwnerCustomerID, &item.FromWarehouse, &item.ToWarehouse, &item.QtyG, &item.QtyUnits,
		); err != nil {
			rows.Close()
			return productionapp.StockEntryDetail{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return productionapp.StockEntryDetail{}, err
	}
	rows.Close()
	if len(items) != 1 {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document must contain exactly one frozen output item")
	}
	item := items[0]
	if item.ItemType != stockItemTypeMaterial || item.MaterialID != materialID || item.ProductID != 0 || item.SpecG != 0 {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document output identity was changed")
	}
	if !sameMaterialOutputInventoryUnit(item.InventoryUnit, outputUnit) {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document inventory unit was changed")
	}
	if strings.TrimSpace(item.FromWarehouse) != "" || strings.TrimSpace(item.ToWarehouse) != strings.TrimSpace(warehouse) {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document target warehouse was changed")
	}
	if item.OwnerCustomerID != ownerCustomerID {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document owner was changed")
	}
	if item.QtyG != finishedG || item.QtyUnits != finishedUnits {
		return productionapp.StockEntryDetail{}, fmt.Errorf("material output stock document quantity changed during completion")
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entries
		SET entry_type='finished_receipt',purpose='manufacture',is_return=false,status='submitted',
		    work_order_id=$2,running_item_id=$3,source_type='work_order_complete',source_id=$2,
		    operator=$4,note=$5,legacy=false,submitted_at=now(),updated_at=now()
		WHERE id=$1
	`, schema), stockDocumentID, workOrderID, runningItemID, operator, note); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entry_items
		SET material_id=$2,product_id=0,item_type='material',item_name=$3,spec_g=0,
		    inventory_unit=$4,from_warehouse='',to_warehouse=$5,owner_customer_id=$6,
		    qty_g=$7,qty_units=$8,batch_code=$9
		WHERE id=$1
	`, schema), item.ID, materialID, materialName, outputUnit, warehouse, ownerCustomerID, finishedG, finishedUnits, batchCode); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(
		ctx, tx, schema, operator, "stock_entry", &stockDocumentID, "submit",
		postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("submitted"),
		postgresinfra.AuditMeta{
			"purpose": "manufacture", "work_order_id": workOrderID, "running_item_id": runningItemID,
			"output_type": "material", "output_material_id": materialID, "warehouse": warehouse,
		},
	); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	return loadStockEntryDetailTx(ctx, tx, schema, stockDocumentID)
}

func sameMaterialOutputInventoryUnit(left, right string) bool {
	if productionWeightUnitGrams(left) > 0 && productionWeightUnitGrams(right) > 0 {
		return true
	}
	normalize := func(unit string) string {
		unit = strings.ToLower(strings.TrimSpace(unit))
		switch unit {
		case "克":
			return "g"
		case "千克", "公斤":
			return "kg"
		case "磅":
			return "lb"
		case "ounce", "ounces", "盎司":
			return "oz"
		default:
			return unit
		}
	}
	return normalize(left) != "" && normalize(left) == normalize(right)
}

func finalizeProductOutputStockDocumentTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	stockDocumentID, runningItemID, productID, specG int64,
	productName, warehouse string,
	finishedUnits, finishedLooseG, finishedTotalG int64,
	batchCode, operator, note string,
	actualCost float64,
) error {
	var status, purpose string
	var isReturn bool
	var workOrderID, boundRunningItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,purpose,is_return,work_order_id,running_item_id
		FROM %s.stock_entries WHERE id=$1 FOR UPDATE
	`, schema), stockDocumentID).Scan(&status, &purpose, &isReturn, &workOrderID, &boundRunningItemID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("stock document not found")
		}
		return err
	}
	if status != "draft" {
		return fmt.Errorf("product output stock document must be draft")
	}
	if purpose != "manufacture" || isReturn || workOrderID <= 0 {
		return fmt.Errorf("product output stock document does not match work order completion")
	}
	if boundRunningItemID > 0 && boundRunningItemID != runningItemID {
		return fmt.Errorf("product output stock document running item does not match work order")
	}
	var linked bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s.work_orders WHERE id=$1 AND running_item_id=$2)
	`, schema), workOrderID, runningItemID).Scan(&linked); err != nil {
		return err
	}
	if !linked {
		return fmt.Errorf("product output stock document does not belong to running work order")
	}

	type frozenProductItem struct {
		ID                         int64
		MaterialID, ProductID      int64
		ItemType                   string
		SpecG                      int64
		FromWarehouse, ToWarehouse string
		QtyG, QtyUnits             int64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,material_id,product_id,item_type,spec_g,from_warehouse,to_warehouse,qty_g,qty_units
		FROM %s.stock_entry_items WHERE stock_entry_id=$1 ORDER BY id FOR UPDATE
	`, schema), stockDocumentID)
	if err != nil {
		return err
	}
	items := make([]frozenProductItem, 0, 1)
	for rows.Next() {
		var item frozenProductItem
		if err := rows.Scan(
			&item.ID, &item.MaterialID, &item.ProductID, &item.ItemType, &item.SpecG,
			&item.FromWarehouse, &item.ToWarehouse, &item.QtyG, &item.QtyUnits,
		); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(items) != 1 {
		return fmt.Errorf("product output stock document must contain exactly one frozen output item")
	}
	item := items[0]
	if item.ItemType != stockItemTypeFinishedProduct || item.MaterialID != 0 || item.ProductID != productID || item.SpecG != specG {
		return fmt.Errorf("product output stock document output identity was changed")
	}
	if strings.TrimSpace(item.FromWarehouse) != "" || strings.TrimSpace(item.ToWarehouse) != strings.TrimSpace(warehouse) {
		return fmt.Errorf("product output stock document target warehouse was changed")
	}
	if item.QtyUnits != finishedUnits || item.QtyG != finishedLooseG {
		return fmt.Errorf("product output stock document quantity changed during completion")
	}

	unitCost := float64(0)
	if finishedTotalG > 0 {
		unitCost = actualCost / (float64(finishedTotalG) / 1000)
	} else if finishedUnits > 0 {
		unitCost = actualCost / float64(finishedUnits)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entries
		SET entry_type='finished_receipt',purpose='manufacture',is_return=false,status='submitted',
		    running_item_id=$2,source_type='work_order_complete',source_id=work_order_id,
		    operator=$3,note=$4,legacy=false,submitted_at=now(),updated_at=now()
		WHERE id=$1
	`, schema), stockDocumentID, runningItemID, operator, note); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entry_items
		SET material_id=0,product_id=$2,item_type='finished_product',item_name=$3,spec_g=$4,
		    from_warehouse='',to_warehouse=$5,qty_g=$6,qty_units=$7,batch_code=$8,
		    unit_cost=$9,total_cost=$10
		WHERE id=$1
	`, schema), item.ID, productID, productName, specG, warehouse, finishedLooseG, finishedUnits, batchCode, unitCost, actualCost); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(
		ctx, tx, schema, operator, "stock_entry", &stockDocumentID, "submit",
		postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("submitted"),
		postgresinfra.AuditMeta{
			"purpose": "manufacture", "work_order_id": workOrderID, "running_item_id": runningItemID,
			"output_type": "product", "output_product_id": productID, "warehouse": warehouse,
		},
	)
}
