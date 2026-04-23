package sales

import (
	"context"
)

type SaveOrderCommand struct {
	Actor                 string
	EditID                int64
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
	ProductID             []string
	TierID                []string
	UnitPrice             []string
	ItemName              []string
	Qty                   []string
	Unit                  []string
	Spec                  []string
}

func (c SaveOrderCommand) GetMaterial() string  { return c.OutsourceMaterialFee }
func (c SaveOrderCommand) GetRoast() string     { return c.OutsourceRoastFee }
func (c SaveOrderCommand) GetPackaging() string { return c.OutsourcePackagingFee }
func (c SaveOrderCommand) GetManual() string    { return c.OutsourceManualFee }
func (c SaveOrderCommand) GetTax() string       { return c.OutsourceTaxFee }
func (c SaveOrderCommand) GetOther() string     { return c.OutsourceOtherFee }

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
