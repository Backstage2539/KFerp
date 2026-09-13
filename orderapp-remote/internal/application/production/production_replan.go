package production

import (
	"context"
	"fmt"
	"strings"
)

// ProductionReplanPreviewCommand identifies independently recoverable, unstarted
// root plan items and any new, still-unplanned order demand to merge with them.
type ProductionReplanPreviewCommand struct {
	ProductionPlanID      int64
	Revision              int64
	ProductionPlanItemIDs []int64
	From                  string
	To                    string
	CustomerID            int64
	Selected              map[string]bool
	InputByKey            map[string]int64
}

type ProductionReplanCommand struct {
	ProductionReplanPreviewCommand
	RequestID string
	Operator  string
}

type ProductionReplanDemand struct {
	ProductionPlanItemID int64                   `json:"production_plan_item_id"`
	ProductID            int64                   `json:"product_id"`
	ProductName          string                  `json:"product_name"`
	SpecLabel            string                  `json:"spec_label"`
	Quantity             float64                 `json:"quantity"`
	Unit                 string                  `json:"unit"`
	QuantityG            int64                   `json:"quantity_g"`
	OrderNos             string                  `json:"order_nos"`
	Orders               []ProductionDemandOrder `json:"orders"`
}

type ProductionReplanWorkOrder struct {
	ID                   int64  `json:"id"`
	WorkOrderNo          string `json:"work_order_no"`
	ProductionPlanItemID int64  `json:"production_plan_item_id"`
	Status               string `json:"status"`
	Workstation          string `json:"workstation"`
	BatchCount           int64  `json:"batch_count"`
}

type ProductionReplanPreview struct {
	ProductionPlanID  int64                       `json:"production_plan_id"`
	ProductionPlanNo  string                      `json:"production_plan_no"`
	Revision          int64                       `json:"revision"`
	RootItemIDs       []int64                     `json:"root_item_ids"`
	AffectedItemIDs   []int64                     `json:"affected_item_ids"`
	ScopeExpanded     bool                        `json:"scope_expanded"`
	OriginalDemands   []ProductionReplanDemand    `json:"original_demands"`
	AdditionalDemands []ProductionReplanDemand    `json:"additional_demands"`
	WorkOrders        []ProductionReplanWorkOrder `json:"work_orders"`
	TotalQuantityG    int64                       `json:"total_quantity_g"`
	BlockingReasons   []string                    `json:"blocking_reasons"`
	WIPNotice         string                      `json:"wip_notice"`
	CanReplan         bool                        `json:"can_replan"`
}

type ProductionReplanResult struct {
	PreviousPlanID int64                `json:"previous_plan_id"`
	PreviousPlanNo string               `json:"previous_plan_no"`
	NewPlan        ProductionPlanDetail `json:"new_plan"`
}

type productionReplanRepository interface {
	PreviewProductionReplan(context.Context, ProductionReplanPreviewCommand) (ProductionReplanPreview, error)
	ReplanProduction(context.Context, ProductionReplanCommand) (ProductionReplanResult, error)
}

func validateProductionReplanScope(planID, revision int64, itemIDs []int64) error {
	if planID <= 0 || revision <= 0 {
		return fmt.Errorf("production plan id and revision required")
	}
	if len(itemIDs) == 0 {
		return fmt.Errorf("请选择需要撤回重排的商品需求")
	}
	seen := map[int64]bool{}
	for _, id := range itemIDs {
		if id <= 0 {
			return fmt.Errorf("production plan item id required")
		}
		if seen[id] {
			return fmt.Errorf("撤回重排范围包含重复商品")
		}
		seen[id] = true
	}
	return nil
}

func (s *Service) PreviewProductionReplan(ctx context.Context, cmd ProductionReplanPreviewCommand) (ProductionReplanPreview, error) {
	if err := validateProductionReplanScope(cmd.ProductionPlanID, cmd.Revision, cmd.ProductionPlanItemIDs); err != nil {
		return ProductionReplanPreview{}, err
	}
	repo, ok := s.repo.(productionReplanRepository)
	if !ok {
		return ProductionReplanPreview{}, fmt.Errorf("撤回重排暂不可用")
	}
	return repo.PreviewProductionReplan(ctx, cmd)
}

func (s *Service) ReplanProduction(ctx context.Context, cmd ProductionReplanCommand) (ProductionReplanResult, error) {
	if err := validateProductionReplanScope(cmd.ProductionPlanID, cmd.Revision, cmd.ProductionPlanItemIDs); err != nil {
		return ProductionReplanResult{}, err
	}
	if strings.TrimSpace(cmd.RequestID) == "" {
		return ProductionReplanResult{}, fmt.Errorf("request_id required")
	}
	if strings.TrimSpace(cmd.Operator) == "" {
		return ProductionReplanResult{}, fmt.Errorf("operator required")
	}
	repo, ok := s.repo.(productionReplanRepository)
	if !ok {
		return ProductionReplanResult{}, fmt.Errorf("撤回重排暂不可用")
	}
	return repo.ReplanProduction(ctx, cmd)
}
