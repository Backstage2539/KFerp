package stock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
)

const planMaterialTransferSource = "production_plan_material_transfer"

type planMaterialTransferState struct {
	PlanStatus                                                       string
	PlanRevision, LinkRevision                                       int64
	SourceExists, ItemExists, IdentityMatches, SourceSnapshotMatches bool
	QtyG, LinkQtyG, QtyUnits, LinkQtyUnits                           int64
}

func validatePlanMaterialTransferState(s planMaterialTransferState) error {
	if s.PlanStatus != "draft" || !s.SourceExists || !s.ItemExists || !s.IdentityMatches || !s.SourceSnapshotMatches || s.QtyG != s.LinkQtyG || s.QtyUnits != s.LinkQtyUnits {
		return fmt.Errorf("生产计划物料或调拨单已变化，请重新核对后生成调拨单")
	}
	return nil
}

func ensurePlanMaterialTransferLinks(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.production_plan_material_transfer_links (
 id BIGSERIAL PRIMARY KEY,
 production_plan_id BIGINT NOT NULL,
 production_plan_item_id BIGINT NOT NULL,
 component_source_id BIGINT NOT NULL,
 stock_entry_id BIGINT NOT NULL,
 stock_entry_item_id BIGINT NOT NULL UNIQUE,
 plan_revision BIGINT NOT NULL,
 qty_g BIGINT NOT NULL DEFAULT 0,
 qty_units BIGINT NOT NULL DEFAULT 0,
 equivalent_qty_g BIGINT NOT NULL DEFAULT 0,
 source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );
 ALTER TABLE %[1]s.production_plan_material_transfer_links ADD COLUMN IF NOT EXISTS equivalent_qty_g BIGINT NOT NULL DEFAULT 0;
 UPDATE %[1]s.production_plan_material_transfer_links SET equivalent_qty_g=qty_g WHERE equivalent_qty_g=0 AND qty_g<>0;
 ALTER TABLE %[1]s.production_plan_material_transfer_links ADD COLUMN IF NOT EXISTS source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
 CREATE INDEX IF NOT EXISTS production_plan_material_transfer_plan_idx ON %[1]s.production_plan_material_transfer_links(production_plan_id,stock_entry_id);
 CREATE TABLE IF NOT EXISTS %[1]s.production_plan_material_transfer_requests (
 production_plan_id BIGINT NOT NULL,
 idempotency_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 result JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (production_plan_id,idempotency_key)
 );`, schema))
	return err
}

// lockPlanMaterialTransferOwnerTx establishes the common plan-before-document
// lock order used by plan updates and stock document posting.
func (r Repository) lockPlanMaterialTransferOwnerTx(ctx context.Context, tx pgx.Tx, entryID int64) error {
	var sourceType string
	var planID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_type,source_id FROM %s.stock_entries WHERE id=$1`, r.schema), entryID).Scan(&sourceType, &planID); err != nil {
		return err
	}
	if sourceType != planMaterialTransferSource {
		return nil
	}
	var lockedID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), planID).Scan(&lockedID); err != nil {
		return fmt.Errorf("生产计划不存在或已变更，请重新核对")
	}
	return nil
}

// validatePlanMaterialTransferDocumentTx prevents stale or tampered plan-linked
// drafts from being posted through the ordinary stock-document endpoint.
func (r Repository) validatePlanMaterialTransferDocumentTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail) error {
	if detail.SourceType != planMaterialTransferSource {
		return nil
	}
	var status string
	var revision int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,revision FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), detail.SourceID).Scan(&status, &revision); err != nil {
		return fmt.Errorf("生产计划不存在或已变更，请重新核对")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT l.plan_revision,l.qty_g,l.qty_units,
   COALESCE(s.id,0)>0,COALESCE(p.id,0)>0,
   COALESCE(l.source_snapshot=jsonb_build_object('required_g',s.required_g,'required_units',s.required_units,'unit',s.unit,'bom_version_id',s.bom_version_id,'allocation_mode',s.allocation_mode,'allocations_json',s.allocations_json,'source_owner_customer_id',s.source_owner_customer_id),false),
   CASE WHEN s.component_type='material' THEN i.item_type='material' AND i.material_id=s.component_id
        WHEN s.component_type IN ('product','finished_product') THEN i.item_type='finished_product' AND i.product_id=s.component_id AND i.bom_spec_id=s.component_bom_spec_id AND i.bom_variant_id=s.component_bom_variant_id AND i.spec_g=s.component_spec_g
        ELSE false END,
   i.qty_g,i.qty_units
 FROM %[1]s.production_plan_material_transfer_links l
 JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id AND i.stock_entry_id=l.stock_entry_id
 LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id AND s.production_plan_id=l.production_plan_id AND s.production_plan_item_id=l.production_plan_item_id
 LEFT JOIN %[1]s.production_plan_items p ON p.id=l.production_plan_item_id AND p.production_plan_id=l.production_plan_id
 WHERE l.stock_entry_id=$1 AND l.production_plan_id=$2 ORDER BY l.id`, r.schema), detail.ID, detail.SourceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var s planMaterialTransferState
		s.PlanStatus = status
		s.PlanRevision = revision
		if err := rows.Scan(&s.LinkRevision, &s.LinkQtyG, &s.LinkQtyUnits, &s.SourceExists, &s.ItemExists, &s.SourceSnapshotMatches, &s.IdentityMatches, &s.QtyG, &s.QtyUnits); err != nil {
			return err
		}
		if err := validatePlanMaterialTransferState(s); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 || count != len(detail.Items) {
		return fmt.Errorf("调拨单与生产计划的物料关联不完整，请重新生成")
	}
	return nil
}

// LockPlanTransferInventoryTx uses the same identity and material locks as
// stock posting before a plan computes its live transfer candidates.
func (r Repository) LockPlanTransferInventoryTx(ctx context.Context, tx pgx.Tx, items []stockapp.StockDocumentItemRow) error {
	detail := stockapp.StockDocumentDetail{StockDocumentRow: stockapp.StockDocumentRow{Purpose: stockapp.PurposeMaterialTransfer}, Items: items}
	if err := lockStockDocumentWorkOrderAndFinishedIdentitiesTx(ctx, tx, r.schema, detail); err != nil {
		return err
	}
	return lockStockDocumentPostingTx(ctx, tx, r.schema, detail)
}

// SubmitPlanStockDocumentTx posts a linked plan transfer inside the caller's transaction.
func (r Repository) SubmitPlanStockDocumentTx(ctx context.Context, tx pgx.Tx, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	var sourceType string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_type FROM %s.stock_entries WHERE id=$1`, r.schema), id).Scan(&sourceType); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if sourceType != planMaterialTransferSource {
		return stockapp.StockDocumentDetail{}, fmt.Errorf("调拨单不属于生产计划")
	}
	return r.submitStockDocumentTx(ctx, tx, id, actor)
}
