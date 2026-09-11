package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r Repository) SaveProductionPlanDraft(ctx context.Context, cmd productionapp.SaveProductionPlanDraftCommand) (productionapp.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan not found")
		}
		return productionapp.ProductionPlanDetail{}, err
	}
	if status != "draft" {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan must be draft to save")
	}
	current, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if current.DraftToken != cmd.DraftToken {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan draft has changed; reload before saving")
	}

	itemByID := make(map[int64]productionapp.ProductionPlanItem, len(current.Items))
	for _, item := range current.Items {
		itemByID[item.ID] = item
	}
	for _, requested := range cmd.Items {
		item, ok := itemByID[requested.ID]
		if !ok {
			return productionapp.ProductionPlanDetail{}, fmt.Errorf("production_plan_item_id does not belong to production plan")
		}
		if err := validateProductionPlanTargetWarehouseTx(ctx, tx, r.schema, requested.TargetWarehouse, item.CustomerID); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET target_warehouse=$3 WHERE id=$1 AND production_plan_id=$2`, r.schema), requested.ID, cmd.ID, requested.TargetWarehouse); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		item.TargetWarehouse = requested.TargetWarehouse
		itemByID[item.ID] = item
	}

	items := make([]productionapp.ProductionPlanItem, 0, len(itemByID))
	for _, item := range current.Items {
		items = append(items, itemByID[item.ID])
	}
	if err := syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, cmd.ID, items); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	for _, requested := range cmd.ComponentSources {
		stored, ok, err := productionPlanComponentSourceForIdentityTx(ctx, tx, r.schema, requested.ProductionPlanItemID,
			requested.ComponentType, requested.ComponentID, requested.ComponentBOMSpecID, requested.ComponentSpecG)
		if err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		if !ok || stored.ProductionPlanID != cmd.ID {
			return productionapp.ProductionPlanDetail{}, fmt.Errorf("component source does not belong to production plan item")
		}
		warehouse := strings.TrimSpace(requested.SourceWarehouse)
		if warehouse == "" {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.production_plan_component_sources
				SET source_warehouse='',source_owner_customer_id=0,available_g_snapshot=0,available_units_snapshot=0,
				    selected_at=NULL,selected_by='',updated_at=now()
				WHERE id=$1 AND production_plan_id=$2
			`, r.schema), stored.ID, cmd.ID); err != nil {
				return productionapp.ProductionPlanDetail{}, err
			}
			continue
		}
		item, ok := itemByID[stored.ProductionPlanItemID]
		if !ok {
			return productionapp.ProductionPlanDetail{}, fmt.Errorf("component source item not found")
		}
		warehouseOwner, err := warehouseCustomerID(ctx, tx, r.schema, warehouse)
		if err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		ownerCustomerID, err := validateComponentSourceOwner(item.CustomerID, warehouseOwner, requested.SourceOwnerCustomerID)
		if err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		if err := validateMaterialComponentSourceOwnerTx(ctx, tx, r.schema, stored.ComponentType, stored.ComponentID, ownerCustomerID); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		availableG, availableUnits, err := componentSourceAvailabilityTx(ctx, tx, r.schema, stored.ComponentType, stored.ComponentID,
			stored.ComponentBOMSpecID, stored.ComponentSpecG, warehouse, ownerCustomerID, false)
		if err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_plan_component_sources
			SET source_warehouse=$3,source_owner_customer_id=$4,available_g_snapshot=$5,available_units_snapshot=$6,
			    selected_at=now(),selected_by=$7,updated_at=now()
			WHERE id=$1 AND production_plan_id=$2
		`, r.schema), stored.ID, cmd.ID, warehouse, ownerCustomerID, availableG, availableUnits, cmd.Operator); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
	}

	if _, err := replaceProductionPlanOperationSplitsTx(ctx, tx, r.schema, cmd.ID, cmd.OperationSplits); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.ID, "save_draft", postgresinfra.StrPtr("workspace"), nil,
		postgresinfra.StrPtr("saved"), postgresinfra.AuditMeta{
			"item_count": len(cmd.Items), "component_source_count": len(cmd.ComponentSources), "operation_split_count": len(cmd.OperationSplits),
		}); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	return detail, nil
}

func validateProductionPlanTargetWarehouseTx(ctx context.Context, tx pgx.Tx, schema, warehouse string, itemCustomerID int64) error {
	var active bool
	var warehouseCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active,COALESCE(customer_id,0) FROM %s.warehouses WHERE code=$1`, schema), warehouse).Scan(&active, &warehouseCustomerID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("target warehouse not found: %s", warehouse)
		}
		return err
	}
	if !active {
		return fmt.Errorf("target warehouse is inactive: %s", warehouse)
	}
	if warehouseCustomerID > 0 && warehouseCustomerID != itemCustomerID {
		return fmt.Errorf("target warehouse belongs to another customer")
	}
	return nil
}
