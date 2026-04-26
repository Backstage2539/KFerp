package sales

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SaveOrderCommand struct {
	Actor                 string
	EditID                int64
	OrderDate             time.Time
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
	Notes                 string
	ShippingAmount        float64
	DiscountAmount        float64
	RoundToInt            bool
	ExpressFee            string
	OutsourceMaterialFee  float64
	OutsourceRoastFee     float64
	OutsourcePackagingFee float64
	OutsourceManualFee    float64
	OutsourceTaxFee       float64
	OutsourceOtherFee     float64
	Items                 []OrderItemCommand
}

type OrderItemCommand struct {
	ProductID   *int64
	TierID      *int64
	ManualPrice *float64
	Name        string
	Units       int64
	Unit        string
	SpecG       int64
}

type SaveOrderResult struct {
	OrderID int64
	OrderNo string
	Edited  bool
}

type UpdateHeaderCommand struct {
	Actor                 string
	OrderDate             string
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
	Notes                 string
	ShippingAmount        string
	DiscountAmount        string
	RoundToInt            string
	ExpressFee            string
	OutsourceMaterialFee  string
	OutsourceRoastFee     string
	OutsourcePackagingFee string
	OutsourceManualFee    string
	OutsourceTaxFee       string
	OutsourceOtherFee     string
	ItemID                []string
	Qty                   []string
	UnitPrice             []string
}

type InlineUpdateCommand struct {
	OrderTypeID     string
	PayStatusID     string
	ShipStatusID    string
	ProcessStatusID string
	Notes           string
}

type OrderListQuery struct {
	Q               string
	From            string
	To              string
	Void            string
	CustomerID      int64
	PayStatusID     int64
	ShipStatusID    int64
	ProcessStatusID int64
	UnproducedOnly  bool
	CompletedOnly   bool
	Limit           int
	Offset          int
}

type OrderListResult struct {
	Rows            []OrderRow
	Summary         OrdersSummary
	OrderTypes      []Option
	PayStatuses     []Option
	ShipStatuses    []Option
	ProcessStatuses []Option
	HasNext         bool
}

type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type OrdersSummary struct {
	Orders    int `json:"orders"`
	Customers int `json:"customers"`
}

type OrderRow struct {
	ID                int64  `json:"id"`
	OrderNo           string `json:"order_no"`
	OrderDate         string `json:"order_date"`
	CustomerID        int64  `json:"customer_id"`
	Customer          string `json:"customer"`
	GrandTotal        string `json:"grand_total"`
	OrderType         string `json:"order_type"`
	PayStatus         string `json:"pay_status"`
	ShipStatus        string `json:"ship_status"`
	OrderTypeID       int64  `json:"order_type_id"`
	PayStatusID       int64  `json:"pay_status_id"`
	ShipStatusID      int64  `json:"ship_status_id"`
	ProcessStatusID   int64  `json:"process_status_id"`
	ProcessStatus     string `json:"process_status"`
	CreatedByEmployee string `json:"created_by_employee"`
	Notes             string `json:"notes"`
	IsVoid            bool   `json:"is_void"`
}

type OutsourceTemplate struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	IsDefault         bool    `json:"is_default"`
	RoastUnitPrice    float64 `json:"roast_unit_price"`
	BeanPackUnitPrice float64 `json:"bean_pack_unit_price"`
	DripPackUnitPrice float64 `json:"drip_pack_unit_price"`
	SCUnitPrice       float64 `json:"sc_unit_price"`
}

type SaveOutsourceTemplateCommand struct {
	Name              string  `json:"name"`
	IsDefault         bool    `json:"is_default"`
	RoastUnitPrice    float64 `json:"roast_unit_price"`
	BeanPackUnitPrice float64 `json:"bean_pack_unit_price"`
	DripPackUnitPrice float64 `json:"drip_pack_unit_price"`
	SCUnitPrice       float64 `json:"sc_unit_price"`
}

type Repository interface {
	SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error)
	UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error
	InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error
	Void(ctx context.Context, id int64, actor, reason string) error
	Unvoid(ctx context.Context, id int64, actor string) error
	ListOrders(ctx context.Context, query OrderListQuery) (OrderListResult, error)
	ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error)
	SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	if err := validateSaveOrderCommand(cmd); err != nil {
		return SaveOrderResult{}, err
	}
	return s.repo.SaveOrder(ctx, cmd)
}

func validateSaveOrderCommand(cmd SaveOrderCommand) error {
	if cmd.OrderDate.IsZero() {
		return fmt.Errorf("invalid order_date")
	}
	if cmd.CustomerID <= 0 {
		return fmt.Errorf("customer required")
	}
	valid := false
	for _, item := range cmd.Items {
		if item.ProductID == nil && strings.TrimSpace(item.Name) == "" {
			continue
		}
		if item.ProductID == nil {
			return fmt.Errorf("product required")
		}
		if item.SpecG <= 0 {
			return fmt.Errorf("spec required")
		}
		if item.Units <= 0 {
			return fmt.Errorf("qty required")
		}
		valid = true
	}
	if !valid {
		return fmt.Errorf("at least one item required")
	}
	return nil
}

func (s *Service) UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error {
	return s.repo.UpdateHeader(ctx, id, cmd)
}

func (s *Service) InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error {
	return s.repo.InlineUpdate(ctx, id, actor, cmd)
}

func (s *Service) Void(ctx context.Context, id int64, actor, reason string) error {
	return s.repo.Void(ctx, id, actor, reason)
}

func (s *Service) Unvoid(ctx context.Context, id int64, actor string) error {
	return s.repo.Unvoid(ctx, id, actor)
}

func (s *Service) ListOrders(ctx context.Context, query OrderListQuery) (OrderListResult, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	if strings.TrimSpace(query.Void) == "" {
		query.Void = "normal"
	}
	return s.repo.ListOrders(ctx, query)
}

func (s *Service) ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error) {
	return s.repo.ListOutsourceTemplates(ctx)
}

func (s *Service) SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return fmt.Errorf("name required")
	}
	if cmd.RoastUnitPrice < 0 || cmd.BeanPackUnitPrice < 0 || cmd.DripPackUnitPrice < 0 || cmd.SCUnitPrice < 0 {
		return fmt.Errorf("prices must be non-negative")
	}
	return s.repo.SaveOutsourceTemplate(ctx, cmd)
}
