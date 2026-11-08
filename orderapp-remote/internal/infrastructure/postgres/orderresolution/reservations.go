package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
)

func quantityOf(row map[string]json.RawMessage, prefix string) float64 {
	q := num(row, prefix+"_units")
	if q == 0 && num(row, "spec_g") > 0 {
		q = num(row, prefix+"_g") / num(row, "spec_g")
	}
	return q
}
func rowVersion(row map[string]json.RawMessage) string { raw, _ := json.Marshal(row); return hash(raw) }

func reservationDependencies(items, allocations, deductions []map[string]json.RawMessage) ([]domain.Dependency, string) {
	out := []domain.Dependency{}
	used := map[int64]float64{}
	for _, d := range deductions {
		var matches []map[string]json.RawMessage
		for _, a := range allocations {
			if num(a, "request_id") == 0 && quantityOf(a, "allocated") > 0 && num(d, "product_id") == num(a, "product_id") && num(d, "bom_spec_id") == num(a, "bom_spec_id") && num(d, "spec_g") == num(a, "spec_g") && num(d, "batch_id") == num(a, "batch_id") && str(d, "batch_code") == str(a, "batch_code") {
				matches = append(matches, a)
			}
		}
		if len(matches) > 1 {
			return out, "同一批次存在多笔预留，发货无法唯一对应，请先核对预留与出库记录"
		}
		if len(matches) == 1 {
			used[int64(num(matches[0], "id"))] += quantityOf(d, "deducted")
		}
	}
	for _, a := range allocations {
		if num(a, "request_id") > 0 || (num(a, "allocated_units") == 0 && num(a, "allocated_g") == 0) {
			continue
		}
		matched := matchingOrderItems(items, a)
		if len(matched) != 1 {
			return out, "库存预留无法唯一对应商品，请先核对商品、规格和批次"
		}
		q := quantityOf(a, "allocated") - used[int64(num(a, "id"))]
		if q <= 0 {
			continue
		}
		i := matched[0]
		out = append(out, domain.Dependency{BatchID: int64(num(a, "batch_id")), Kind: "stock_reservation", ID: int64(num(a, "id")), ItemID: int64(num(i, "id")), Version: rowVersion(a), Quantity: q, Unit: str(i, "unit"), Status: "reserved", Label: fmt.Sprintf("%s · 批次 %s · 未发货预留 %g %s", str(i, "item_name"), str(a, "batch_code"), q, str(i, "unit"))})
	}
	return out, ""
}

func (r Repository) releaseReservations(ctx context.Context, tx pgx.Tx, orderID, requestID int64, original domain.Context, cmd domain.Command, actor string) error {
	for _, d := range cmd.Dispositions {
		if d.Kind != "stock_reservation" || d.Choice != "release" {
			continue
		}
		var prior domain.Dependency
		for _, dep := range original.Dependencies {
			if dep.Kind == d.Kind && dep.ID == d.ID {
				prior = dep
				break
			}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(a)FROM %s.order_stock_batch_allocations a WHERE id=$1 AND order_id=$2 AND COALESCE(request_id,0)=0 FOR UPDATE`, r.schema), d.ID, orderID).Scan(&raw); err != nil {
			return err
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(raw, &row); err != nil {
			return err
		}
		if prior.Version == "" || prior.Version != rowVersion(row) {
			return fmt.Errorf("%w：库存预留已变化", ErrStale)
		}
		units, grams := num(row, "allocated_units"), num(row, "allocated_g")
		releaseUnits := float64(0)
		if units > 0 {
			releaseUnits = d.Quantity
			if math.Abs(releaseUnits-math.Round(releaseUnits)) > 1e-9 {
				return fmt.Errorf("按件预留须释放整数件")
			}
		}
		releaseGrams := float64(0)
		if grams > 0 {
			originalQuantity := quantityOf(row, "allocated")
			if originalQuantity <= 0 {
				return fmt.Errorf("预留计量不完整，请核对批次")
			}
			releaseGrams = grams * d.Quantity / originalQuantity
			if math.Abs(releaseGrams-math.Round(releaseGrams)) > 1e-9 {
				return fmt.Errorf("释放重量须为整数克，请核对计量")
			}
		}
		if d.Quantity <= 0 || d.Quantity > prior.Quantity+1e-9 || releaseUnits > units || releaseGrams > grams {
			return fmt.Errorf("释放数量超过未发货预留")
		}
		row["allocated_units"], _ = json.Marshal(int64(math.Round(units - releaseUnits)))
		row["allocated_g"], _ = json.Marshal(int64(math.Round(grams - releaseGrams)))
		after, _ := json.Marshal(row)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_reservation_releases(request_id,order_id,allocation_id,snapshot_json,after_json,actor)VALUES($1,$2,$3,$4,$5,$6)`, r.schema), requestID, orderID, d.ID, raw, after, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_stock_batch_allocations SET allocated_units=$2,allocated_g=$3 WHERE id=$1`, r.schema), d.ID, int64(math.Round(units-releaseUnits)), int64(math.Round(grams-releaseGrams))); err != nil {
			return err
		}
	}
	return nil
}
