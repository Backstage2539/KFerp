package orderweight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type Item struct {
	Spec                                      string
	ProductID, BomSpecID, BomVariantID, SpecG int64
	Qty                                       float64
	Unit                                      string
}

type Snapshot struct {
	ResolvedBomSpecID    int64       `json:"resolved_bom_spec_id,omitempty"`
	ResolvedBomVariantID int64       `json:"resolved_bom_variant_id,omitempty"`
	RuleVersion          int         `json:"rule_version"`
	ProductID            int64       `json:"product_id"`
	BomSpecID            int64       `json:"bom_spec_id"`
	BomVariantID         int64       `json:"bom_variant_id"`
	SpecG                int64       `json:"spec_g"`
	Unit                 string      `json:"unit"`
	Spec                 string      `json:"spec"`
	BomVersionID         int64       `json:"bom_version_id,omitempty"`
	Qty                  float64     `json:"qty"`
	MaterialUnitWeightG  *float64    `json:"material_unit_weight_g"`
	MaterialItemWeightG  *float64    `json:"material_item_weight_g"`
	LogisticsUnitWeightG *float64    `json:"logistics_unit_weight_g"`
	LogisticsItemWeightG *float64    `json:"logistics_item_weight_g"`
	Components           []Component `json:"components,omitempty"`
	Reason               string      `json:"reason,omitempty"`
	HistoricalUnknown    bool        `json:"historical_unknown,omitempty"`
}

func newSnapshot(i Item) Snapshot {
	return Snapshot{RuleVersion: 1, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, Unit: i.Unit, Spec: i.Spec, Qty: i.Qty}
}

func exists(ctx context.Context, q Queryer, schema, table string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, pgx.Identifier{schema, table}.Sanitize()).Scan(&ok)
	return ok, err
}

// Resolve reads the selected published specification. No price, display name, or
// mutable conversion group is a source of manually configured logistics weight.
func Resolve(ctx context.Context, q Queryer, schema string, item Item) (Snapshot, error) {
	s := newSnapshot(item)
	if !validPositive(item.Qty) || item.Qty > 1e6 {
		return s, fmt.Errorf("商品数量无效")
	}
	if item.ProductID <= 0 {
		s.Reason = "missing_product"
		return s, nil
	}
	for _, table := range []string{"production_bom_versions", "production_bom_output_bindings", "production_bom_version_variants", "production_bom_specs"} {
		ok, err := exists(ctx, q, schema, table)
		if err != nil {
			return s, err
		}
		if !ok {
			s.Reason = "no_published_spec"
			return s, nil
		}
	}
	db := pgx.Identifier{schema}.Sanitize()
	var outputQty float64
	var outputUnit string
	var resolvedSpecID, resolvedVariantID int64
	err := q.QueryRow(ctx, fmt.Sprintf(`
	 SELECT v.id,variant.bom_spec_id,variant.id,variant.inventory_unit,
	        (to_jsonb(variant)->>'logistics_weight_g')::float8
	 FROM %[1]s.production_bom_output_bindings binding
	 JOIN %[1]s.production_bom_versions v ON v.bom_id=binding.bom_id AND v.status='published'
	 JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=v.id
	 WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
	   AND ($2::bigint=0 OR variant.bom_spec_id=$2)
	   AND (($3::bigint>0 AND variant.id=$3) OR ($3::bigint=0 AND v.id=binding.bom_version_id))
	   AND ($2::bigint>0 OR variant.is_default=true)
	 ORDER BY variant.id LIMIT 1`, db), item.ProductID, item.BomSpecID, item.BomVariantID).Scan(&s.BomVersionID, &resolvedSpecID, &resolvedVariantID, &outputUnit, &s.LogisticsUnitWeightG)
	if errors.Is(err, pgx.ErrNoRows) {
		// Legacy single-output recipes can still supply reference material weight.
		err = q.QueryRow(ctx, fmt.Sprintf(`SELECT v.id,v.output_qty::float8,v.output_unit FROM %[1]s.production_bom_output_bindings b JOIN %[1]s.production_bom_versions v ON v.id=b.bom_version_id AND v.status='published' WHERE b.output_type='product' AND b.output_id=$1 AND b.is_default=true AND $2::bigint=0`, db), item.ProductID, item.BomSpecID).Scan(&s.BomVersionID, &outputQty, &outputUnit)
		if errors.Is(err, pgx.ErrNoRows) {
			s.Reason = "no_published_spec"
			return s, nil
		}
		if err != nil {
			return s, err
		}
	} else if err != nil {
		return s, err
	} else {
		outputQty = 1
	}
	if s.LogisticsUnitWeightG != nil && (!validPositive(*s.LogisticsUnitWeightG) || *s.LogisticsUnitWeightG > 1e9) {
		s.LogisticsUnitWeightG = nil
	}
	if s.LogisticsUnitWeightG == nil {
		s.Reason = "missing_logistics_weight"
	}
	factor, netG := 0.0, 0.0
	if resolvedVariantID > 0 {
		// Variant material quantities are already per one inventory unit.
		factor = 1
		netG = gramsPerUnit(outputUnit)
	} else if validPositive(outputQty) {
		if g := gramsPerUnit(outputUnit); g > 0 && item.SpecG > 0 {
			factor = float64(item.SpecG) / (outputQty * g)
			netG = float64(item.SpecG)
		} else if g := gramsPerUnit(outputUnit); g > 0 && gramsPerUnit(item.Unit) > 0 {
			netG = gramsPerUnit(item.Unit)
			factor = netG / (outputQty * g)
		} else if strings.EqualFold(strings.TrimSpace(item.Unit), strings.TrimSpace(outputUnit)) {
			factor = 1 / outputQty
			netG = gramsPerUnit(outputUnit)
		}
	}
	ok, err := exists(ctx, q, schema, "production_bom_version_items")
	if err != nil {
		return s, err
	}
	if ok && factor > 0 {
		// to_jsonb keeps old schemas with optional component fields readable.
		rows, e := q.Query(ctx, fmt.Sprintf(`
		 SELECT i.id,COALESCE(NULLIF(i.consume_unit,''),'ratio_pct'),i.qty_per_unit::float8,i.ratio_pct::float8,
		 CASE WHEN COALESCE(to_jsonb(i)->>'component_type','material') IN ('product','finished_product')
		 THEN COALESCE(NULLIF(component_spec.inventory_unit,''),to_jsonb(cp)->'unit_rule_override_json'->>'inventory_unit','')
		 ELSE COALESCE(m.unit,'') END
		 FROM %[1]s.production_bom_version_items i
		 LEFT JOIN %[1]s.materials m ON m.id=i.material_id
		 LEFT JOIN %[1]s.products cp ON cp.id=COALESCE((to_jsonb(i)->>'component_product_id')::bigint,0)
		 LEFT JOIN %[1]s.production_bom_specs component_spec ON component_spec.id=COALESCE((to_jsonb(i)->>'component_bom_spec_id')::bigint,0)
		 WHERE i.version_id=$1 AND ($2::bigint=0 OR COALESCE((to_jsonb(i)->>'variant_id')::bigint,0)=$2)
		 ORDER BY i.id`, db), s.BomVersionID, resolvedVariantID)
		if e != nil {
			return s, e
		}
		for rows.Next() {
			var c Component
			if e = rows.Scan(&c.ID, &c.ConsumeUnit, &c.Quantity, &c.RatioPct, &c.Unit); e != nil {
				rows.Close()
				return s, e
			}
			s.Components = append(s.Components, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return s, err
		}
		s.MaterialUnitWeightG = MaterialReference(s.Components, factor, netG, outputUnit)
	}
	s.ResolvedBomSpecID = resolvedSpecID
	s.ResolvedBomVariantID = resolvedVariantID
	s.setTotals(item.Qty)
	return s, nil
}

func (s *Snapshot) setTotals(qty float64) {
	s.Qty = qty
	s.MaterialItemWeightG = multiply(s.MaterialUnitWeightG, qty)
	s.LogisticsItemWeightG = multiply(s.LogisticsUnitWeightG, qty)
}
func multiply(v *float64, qty float64) *float64 {
	if v == nil {
		return nil
	}
	n := round(*v * qty)
	return &n
}

// MarkHistoricalUnknown must run before editing existing lines. A missing old
// snapshot represents unknown historical weight, not permission to recalculate it.
func MarkHistoricalUnknown(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items i SET weight_snapshot_json=jsonb_build_object(
	 'rule_version',1,'historical_unknown',true,'product_id',COALESCE(product_id,0),
	 'bom_spec_id',COALESCE((to_jsonb(i)->>'bom_spec_id')::bigint,0),
	 'bom_variant_id',COALESCE((to_jsonb(i)->>'bom_variant_id')::bigint,0),
	 'spec_g',COALESCE((to_jsonb(i)->>'spec_g')::bigint,0),'unit',COALESCE(unit,''),'spec',COALESCE(spec,''))
	 WHERE order_id=$1 AND weight_snapshot_json='{}'::jsonb`, pgx.Identifier{schema}.Sanitize()), orderID)
	return err
}

// FreezeOrder is called within the business write transaction, covering every
// entry point while leaving unrelated historical orders untouched.
func FreezeOrder(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	db := pgx.Identifier{schema}.Sanitize()
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(product_id,0),COALESCE((to_jsonb(i)->>'bom_spec_id')::bigint,0),COALESCE((to_jsonb(i)->>'bom_variant_id')::bigint,0),COALESCE((to_jsonb(i)->>'spec_g')::bigint,0),qty::float8,COALESCE(unit,''),COALESCE(spec,''),material_unit_weight_g,logistics_unit_weight_g,weight_snapshot_json FROM %s.order_items i WHERE order_id=$1 ORDER BY id FOR UPDATE`, db), orderID)
	if err != nil {
		return err
	}
	type stored struct {
		id                  int64
		i                   Item
		material, logistics *float64
		raw                 []byte
	}
	var all []stored
	for rows.Next() {
		var v stored
		if err = rows.Scan(&v.id, &v.i.ProductID, &v.i.BomSpecID, &v.i.BomVariantID, &v.i.SpecG, &v.i.Qty, &v.i.Unit, &v.i.Spec, &v.material, &v.logistics, &v.raw); err != nil {
			rows.Close()
			return err
		}
		all = append(all, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range all {
		if !validPositive(v.i.Qty) || v.i.Qty > 1e6 {
			return fmt.Errorf("商品数量无效")
		}
		var s Snapshot
		if len(v.raw) > 0 {
			if err = json.Unmarshal(v.raw, &s); err != nil {
				return err
			}
		}
		if s.RuleVersion > 0 && sameIdentity(s, v.i) {
			s.MaterialUnitWeightG = v.material
			s.LogisticsUnitWeightG = v.logistics
			s.setTotals(v.i.Qty)
		} else {
			s, err = Resolve(ctx, tx, schema, v.i)
			if err != nil {
				return err
			}
		}
		if s.LogisticsItemWeightG != nil && (*s.LogisticsItemWeightG > 1e9 || math.IsInf(*s.LogisticsItemWeightG, 0)) {
			return fmt.Errorf("商品物流重量超出可处理范围")
		}
		raw, e := json.Marshal(s)
		if e != nil {
			return e
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET material_unit_weight_g=$2,material_item_weight_g=$3,logistics_unit_weight_g=$4,logistics_item_weight_g=$5,weight_snapshot_json=$6 WHERE id=$1`, db), v.id, s.MaterialUnitWeightG, s.MaterialItemWeightG, s.LogisticsUnitWeightG, s.LogisticsItemWeightG, raw)
		if err != nil {
			return err
		}
	}
	return nil
}

func sameIdentity(s Snapshot, i Item) bool {
	if s.ProductID != i.ProductID || s.BomSpecID != i.BomSpecID {
		return false
	}
	if i.BomSpecID > 0 {
		return true
	}
	return s.BomVariantID == i.BomVariantID && s.SpecG == i.SpecG && s.Unit == i.Unit && s.Spec == i.Spec
}
