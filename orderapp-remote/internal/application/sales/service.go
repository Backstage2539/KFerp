package sales

import (
	"context"
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

type Repository interface {
	SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error)
	UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error
	InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error
	Void(ctx context.Context, id int64, actor, reason string) error
	Unvoid(ctx context.Context, id int64, actor string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	return s.repo.SaveOrder(ctx, cmd)
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
