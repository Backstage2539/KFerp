package production

import (
	"context"
	"fmt"
	"math"
	productiondomain "orderapp/internal/domain/production"
	"strings"
)

type StockProductionTarget struct {
	OutputType       string  `json:"output_type"`
	OutputMaterialID int64   `json:"output_material_id"`
	OutputQty        float64 `json:"output_qty"`
	TargetWarehouse  string  `json:"target_warehouse"`
}

type ProductionSupplyAllocation struct {
	ID                  int64  `json:"id"`
	PlanItemID          int64  `json:"production_plan_item_id"`
	WorkOrderID         int64  `json:"work_order_id"`
	SupplierWorkOrderID int64  `json:"supplier_work_order_id"`
	SupplierWorkOrderNo string `json:"supplier_work_order_no"`
	MaterialID          int64  `json:"material_id"`
	MaterialName        string `json:"material_name"`
	Warehouse           string `json:"warehouse"`
	OwnerCustomerID     int64  `json:"owner_customer_id"`
	RequiredG           int64  `json:"required_g"`
	RequiredUnits       int64  `json:"required_units"`
	DeliveredG          int64  `json:"delivered_g"`
	DeliveredUnits      int64  `json:"delivered_units"`
	Status              string `json:"status"`
}

type ProductionPlanPreview struct {
	Items             []ProductionPlanItem               `json:"items"`
	ManufacturingPlan productiondomain.ManufacturingPlan `json:"manufacturing_plan"`
	SupplyAllocations []ProductionSupplyAllocation       `json:"supply_allocations"`
	MaterialSummary   []MaterialNeed                     `json:"material_summary"`
}

func validateStockProductionTargets(cmd CreateProductionPlanCommand) error {
	if len(cmd.Items) == 0 {
		return fmt.Errorf("请选择自制物料并填写目标产出量")
	}
	if len(cmd.Selected) > 0 {
		return fmt.Errorf("备货计划不能同时选择订单需求")
	}
	for _, item := range cmd.Items {
		if (item.OutputType != "" && item.OutputType != "material") || item.OutputMaterialID <= 0 || item.OutputQty <= 0 || math.IsNaN(item.OutputQty) || math.IsInf(item.OutputQty, 0) || item.OutputQty > 1e9 {
			return fmt.Errorf("备货计划需要有效物料和正数目标产出量")
		}
	}
	return nil
}

func (s *Service) PreviewProductionPlan(ctx context.Context, cmd CreateProductionPlanCommand) (ProductionPlanPreview, error) {
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	if cmd.SourceType != "" && cmd.SourceType != "stock" && cmd.SourceType != "erp_order" {
		return ProductionPlanPreview{}, fmt.Errorf("invalid production plan source")
	}
	if cmd.SourceType == "stock" {
		if err := validateStockProductionTargets(cmd); err != nil {
			return ProductionPlanPreview{}, err
		}
	}
	repo, ok := s.repo.(interface {
		PreviewProductionPlan(context.Context, CreateProductionPlanCommand) (ProductionPlanPreview, error)
	})
	if !ok {
		return ProductionPlanPreview{}, fmt.Errorf("production planning preview unavailable")
	}
	return repo.PreviewProductionPlan(ctx, cmd)
}

type ProductionDemandOrder struct {
	OrderItemID  int64   `json:"order_item_id"`
	OrderID      int64   `json:"order_id"`
	OrderNo      string  `json:"order_no"`
	CustomerID   int64   `json:"customer_id"`
	CustomerName string  `json:"customer_name"`
	Quantity     float64 `json:"quantity"`
	SalesUnit    string  `json:"sales_unit"`
	ForceProduce bool    `json:"force_produce"`
}
type ProductionDemandSpecGroup struct {
	Key               string          `json:"key"`
	SpecLabel         string          `json:"spec_label"`
	SalesUnit         string          `json:"sales_unit"`
	SalesSpecCount    float64         `json:"sales_spec_count"`
	GapSalesSpecCount float64         `json:"gap_sales_spec_count"`
	Rows              []UnprodNeedRow `json:"rows"`
}
type ProductionDemandProductGroup struct {
	ProductID   int64                       `json:"product_id"`
	ProductName string                      `json:"product_name"`
	Specs       []ProductionDemandSpecGroup `json:"specs"`
}

func (s *Service) RefreshProductionPlanSupply(ctx context.Context, id int64, operator string) (ProductionPlanDetail, error) {
	if id <= 0 || strings.TrimSpace(operator) == "" {
		return ProductionPlanDetail{}, fmt.Errorf("plan and operator required")
	}
	repo, ok := s.repo.(interface {
		RefreshProductionPlanSupply(context.Context, int64, string) (ProductionPlanDetail, error)
	})
	if !ok {
		return ProductionPlanDetail{}, fmt.Errorf("supply refresh unavailable")
	}
	return repo.RefreshProductionPlanSupply(ctx, id, operator)
}

func planningUnitWeightGrams(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
		return 1
	case "kg", "千克", "公斤":
		return 1000
	case "lb", "磅":
		return 453.59237
	case "oz", "盎司":
		return 28.349523125
	}
	return 0
}
