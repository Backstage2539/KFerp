package production

import (
	"fmt"
	"math"
	"strings"
)

const (
	ManufacturingSupplyInventory   = "inventory"
	ManufacturingSupplyManufacture = "manufacture"
	ManufacturingSupplyPurchase    = "purchase"
)

type ManufacturingItemRef struct {
	Type         string `json:"type"`
	ID           int64  `json:"id"`
	BomSpecID    int64  `json:"bom_spec_id,omitempty"`
	BomVariantID int64  `json:"bom_variant_id,omitempty"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
}

func (r ManufacturingItemRef) Key() string {
	if strings.EqualFold(strings.TrimSpace(r.Type), "product") && r.BomSpecID > 0 {
		return fmt.Sprintf("product:%d:bom_spec:%d", r.ID, r.BomSpecID)
	}
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(r.Type)), r.ID)
}

func (r ManufacturingItemRef) validate() error {
	switch strings.ToLower(strings.TrimSpace(r.Type)) {
	case "material", "product":
	default:
		return fmt.Errorf("manufacturing item type must be material or product")
	}
	if r.ID <= 0 {
		return fmt.Errorf("manufacturing item id required")
	}
	if strings.TrimSpace(r.Unit) == "" {
		return fmt.Errorf("manufacturing item unit required: %s", r.Key())
	}
	return nil
}

type ManufacturingDemand struct {
	Item            ManufacturingItemRef `json:"item"`
	Qty             float64              `json:"qty"`
	TargetWarehouse string               `json:"target_warehouse"`
}

type ManufacturingBOMComponent struct {
	Item     ManufacturingItemRef `json:"item"`
	Qty      float64              `json:"qty"`
	Fixed    bool                 `json:"fixed"`
	LossRate float64              `json:"loss_rate"`
}

type ManufacturingBOM struct {
	VersionID     int64                       `json:"version_id"`
	Output        ManufacturingItemRef        `json:"output"`
	OutputQty     float64                     `json:"output_qty"`
	OperationCost float64                     `json:"operation_cost"`
	Components    []ManufacturingBOMComponent `json:"components"`
}

type ManufacturingPlanNode struct {
	Item            ManufacturingItemRef `json:"item"`
	RequiredQty     float64              `json:"required_qty"`
	StockCoveredQty float64              `json:"stock_covered_qty"`
	ShortageQty     float64              `json:"shortage_qty"`
	Action          string               `json:"action"`
	BOMVersionID    int64                `json:"bom_version_id"`
	TargetWarehouse string               `json:"target_warehouse"`
	Blocking        bool                 `json:"blocking"`
}

type ManufacturingSupplyEdge struct {
	ConsumerKey     string  `json:"consumer_key"`
	SupplierKey     string  `json:"supplier_key"`
	RequiredQty     float64 `json:"required_qty"`
	StockCoveredQty float64 `json:"stock_covered_qty"`
	ShortageQty     float64 `json:"shortage_qty"`
}

type ManufacturingPlan struct {
	Nodes          []ManufacturingPlanNode   `json:"nodes"`
	Edges          []ManufacturingSupplyEdge `json:"edges"`
	ReservedByItem map[string]float64        `json:"reserved_by_item"`
	Blocking       bool                      `json:"blocking"`
}

type manufacturingPlanBuilder struct {
	boms      map[string]ManufacturingBOM
	available map[string]float64
	reserved  map[string]float64
	nodes     []ManufacturingPlanNode
	nodeIndex map[string]int
	edges     []ManufacturingSupplyEdge
	path      map[string]bool
	blocking  bool
}

// BuildMultilevelManufacturingPlan recursively explodes already-short root
// demand. All quantities must be normalized to the frozen inventory unit by
// the caller. Inventory is consumed from a private availability snapshot so a
// batch can never be counted twice across sibling demands.
func BuildMultilevelManufacturingPlan(demands []ManufacturingDemand, boms []ManufacturingBOM, available map[string]float64) (ManufacturingPlan, error) {
	builder := manufacturingPlanBuilder{
		boms:      make(map[string]ManufacturingBOM, len(boms)),
		available: make(map[string]float64, len(available)),
		reserved:  map[string]float64{},
		nodeIndex: map[string]int{},
		path:      map[string]bool{},
	}
	for key, qty := range available {
		if qty > 0 {
			builder.available[key] = qty
		}
	}
	for _, bom := range boms {
		if err := bom.Output.validate(); err != nil {
			return ManufacturingPlan{}, err
		}
		if bom.VersionID <= 0 || bom.OutputQty <= 0 {
			return ManufacturingPlan{}, fmt.Errorf("published BOM version and positive output quantity required: %s", bom.Output.Key())
		}
		for _, component := range bom.Components {
			if err := component.Item.validate(); err != nil {
				return ManufacturingPlan{}, err
			}
			if component.Qty <= 0 {
				return ManufacturingPlan{}, fmt.Errorf("positive component quantity required: %s", component.Item.Key())
			}
			if component.LossRate < 0 || component.LossRate >= 1 {
				return ManufacturingPlan{}, fmt.Errorf("component loss rate must be in [0,1): %s", component.Item.Key())
			}
		}
		key := bom.Output.Key()
		if _, exists := builder.boms[key]; exists {
			return ManufacturingPlan{}, fmt.Errorf("multiple default BOMs for %s", key)
		}
		builder.boms[key] = bom
	}
	for _, demand := range demands {
		if err := demand.Item.validate(); err != nil {
			return ManufacturingPlan{}, err
		}
		if demand.Qty <= 0 {
			return ManufacturingPlan{}, fmt.Errorf("positive manufacturing demand required: %s", demand.Item.Key())
		}
		if err := builder.addRoot(demand); err != nil {
			return ManufacturingPlan{}, err
		}
	}
	return ManufacturingPlan{
		Nodes:          builder.nodes,
		Edges:          builder.edges,
		ReservedByItem: builder.reserved,
		Blocking:       builder.blocking,
	}, nil
}

func (b *manufacturingPlanBuilder) addRoot(demand ManufacturingDemand) error {
	bom, ok := b.boms[demand.Item.Key()]
	node := ManufacturingPlanNode{
		Item:            demand.Item,
		RequiredQty:     demand.Qty,
		ShortageQty:     demand.Qty,
		Action:          ManufacturingSupplyPurchase,
		TargetWarehouse: firstNonEmptyManufacturing(strings.TrimSpace(demand.TargetWarehouse), defaultManufacturingWarehouse(demand.Item.Type)),
		Blocking:        !ok,
	}
	if ok {
		node.Action = ManufacturingSupplyManufacture
		node.BOMVersionID = bom.VersionID
	}
	b.upsertNode(node)
	if !ok {
		b.blocking = true
		return nil
	}
	return b.expand(demand.Item.Key(), bom, demand.Qty)
}

func (b *manufacturingPlanBuilder) expand(outputKey string, bom ManufacturingBOM, manufactureQty float64) error {
	if b.path[outputKey] {
		return fmt.Errorf("manufacturing BOM cycle detected at %s", outputKey)
	}
	b.path[outputKey] = true
	defer delete(b.path, outputKey)

	factor := manufactureQty / bom.OutputQty
	for _, component := range bom.Components {
		required := normalizeManufacturingQty(manufacturingComponentGrossQty(component) * factor)
		if required <= 0 {
			continue
		}
		key := component.Item.Key()
		covered := math.Min(required, math.Max(0, b.available[key]))
		covered = normalizeManufacturingQty(covered)
		shortage := normalizeManufacturingQty(required - covered)
		b.available[key] = normalizeManufacturingQty(b.available[key] - covered)
		b.reserved[key] = normalizeManufacturingQty(b.reserved[key] + covered)

		node := ManufacturingPlanNode{
			Item:            component.Item,
			RequiredQty:     required,
			StockCoveredQty: covered,
			ShortageQty:     shortage,
			Action:          ManufacturingSupplyInventory,
			TargetWarehouse: defaultManufacturingWarehouse(component.Item.Type),
		}
		componentBom, canManufacture := b.boms[key]
		if shortage > 0 {
			if canManufacture {
				node.Action = ManufacturingSupplyManufacture
				node.BOMVersionID = componentBom.VersionID
			} else {
				node.Action = ManufacturingSupplyPurchase
				node.Blocking = true
				b.blocking = true
			}
		}
		b.upsertNode(node)
		b.edges = append(b.edges, ManufacturingSupplyEdge{
			ConsumerKey: outputKey, SupplierKey: key,
			RequiredQty: required, StockCoveredQty: covered, ShortageQty: shortage,
		})
		if shortage > 0 && canManufacture {
			if b.path[key] {
				return fmt.Errorf("manufacturing BOM cycle detected: %s -> %s", outputKey, key)
			}
			if err := b.expand(key, componentBom, shortage); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *manufacturingPlanBuilder) upsertNode(add ManufacturingPlanNode) {
	key := add.Item.Key()
	if index, ok := b.nodeIndex[key]; ok {
		current := b.nodes[index]
		current.RequiredQty = normalizeManufacturingQty(current.RequiredQty + add.RequiredQty)
		current.StockCoveredQty = normalizeManufacturingQty(current.StockCoveredQty + add.StockCoveredQty)
		current.ShortageQty = normalizeManufacturingQty(current.ShortageQty + add.ShortageQty)
		if manufacturingActionPriority(add.Action) > manufacturingActionPriority(current.Action) {
			current.Action = add.Action
		}
		if add.BOMVersionID > 0 {
			current.BOMVersionID = add.BOMVersionID
		}
		if current.TargetWarehouse == "" {
			current.TargetWarehouse = add.TargetWarehouse
		}
		current.Blocking = current.Blocking || add.Blocking
		b.nodes[index] = current
		return
	}
	b.nodeIndex[key] = len(b.nodes)
	b.nodes = append(b.nodes, add)
}

func manufacturingActionPriority(action string) int {
	switch action {
	case ManufacturingSupplyPurchase:
		return 3
	case ManufacturingSupplyManufacture:
		return 2
	default:
		return 1
	}
}

func defaultManufacturingWarehouse(itemType string) string {
	if strings.EqualFold(strings.TrimSpace(itemType), "product") {
		return "finished_goods"
	}
	return "wip"
}

func firstNonEmptyManufacturing(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeManufacturingQty(qty float64) float64 {
	if math.Abs(qty) < 0.000000001 {
		return 0
	}
	return math.Round(qty*1000000) / 1000000
}

func manufacturingComponentGrossQty(component ManufacturingBOMComponent) float64 {
	if component.Fixed || component.LossRate <= 0 {
		return component.Qty
	}
	return component.Qty / (1 - component.LossRate)
}
