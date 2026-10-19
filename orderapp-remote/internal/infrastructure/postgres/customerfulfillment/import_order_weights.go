package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"orderapp/internal/infrastructure/postgres/orderweight"
	"strings"
)

func (r *Repository) resolveImportedWeightIdentity(ctx context.Context, tx pgx.Tx, productID, specID, variantID int64) (customerFulfillmentBOMSpecIdentity, error) {
	if productID <= 0 || specID <= 0 || !relationExists(ctx, tx, r.schema+".product_bom_spec_authorities") {
		return customerFulfillmentBOMSpecIdentity{}, nil
	}
	identity, err := resolveCustomerFulfillmentBOMSpecIdentityTx(ctx, tx, r.schema, productID, specID, variantID)
	if err != nil {
		// Unknown imported identity remains unknown. SQL/infrastructure failures still abort.
		for _, message := range []string{"product_bom_spec_not_configured", "bom_spec_id required", "BOM spec is not published"} {
			if strings.Contains(err.Error(), message) {
				return customerFulfillmentBOMSpecIdentity{}, nil
			}
		}
		return customerFulfillmentBOMSpecIdentity{}, err
	}
	return identity, nil
}

func markUnresolvedImportWeightTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, lineNo int, productID int64, spec, unit string, previous orderweight.Snapshot) error {
	if previous.RuleVersion > 0 && previous.ProductID == productID && previous.BomSpecID == 0 && previous.BomVariantID == 0 && previous.Spec == spec && previous.Unit == unit {
		return nil
	}
	snapshot := orderweight.Snapshot{RuleVersion: 1, ProductID: productID, Spec: spec, Unit: unit, Reason: "unresolved_import_spec"}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET material_unit_weight_g=NULL,material_item_weight_g=NULL,logistics_unit_weight_g=NULL,logistics_item_weight_g=NULL,weight_snapshot_json=$3::jsonb WHERE order_id=$1 AND line_no=$2`, schema), orderID, lineNo, raw)
	return err
}
