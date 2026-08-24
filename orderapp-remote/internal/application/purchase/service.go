package purchase

import (
	"context"
	"fmt"
	"strings"

	stockapp "orderapp/internal/application/stock"
)

type Supplier struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Contact string `json:"contact"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Active  bool   `json:"active"`
}

type SaveSupplierCommand struct {
	ID      int64
	Name    string
	Contact string
	Phone   string
	Address string
	Active  bool
}

type PurchaseOrder struct {
	ID           int64   `json:"id"`
	OrderNo      string  `json:"order_no"`
	SupplierID   int64   `json:"supplier_id"`
	SupplierName string  `json:"supplier_name"`
	MaterialID   int64   `json:"material_id"`
	MaterialName string  `json:"material_name"`
	QtyG         int64   `json:"qty_g"`
	UnitCost     float64 `json:"unit_cost"`
	Status       string  `json:"status"`
	Operator     string  `json:"operator"`
	CreatedAt    string  `json:"created_at"`
}

type CreatePurchaseOrderCommand struct {
	SupplierID int64
	MaterialID int64
	QtyG       int64
	UnitCost   float64
	Note       string
	Operator   string
}

type PurchaseReceipt struct {
	ID              int64   `json:"id"`
	ReceiptNo       string  `json:"receipt_no"`
	PurchaseOrderID int64   `json:"purchase_order_id"`
	SupplierID      int64   `json:"supplier_id"`
	SupplierName    string  `json:"supplier_name"`
	MaterialID      int64   `json:"material_id"`
	MaterialName    string  `json:"material_name"`
	QtyG            int64   `json:"qty_g"`
	UnitCost        float64 `json:"unit_cost"`
	StockReceiptID  int64   `json:"stock_receipt_id"`
	StockBatchCode  string  `json:"stock_batch_code"`
	Operator        string  `json:"operator"`
	CreatedAt       string  `json:"created_at"`
	Note            string  `json:"note"`
}

type CreatePurchaseReceiptCommand struct {
	PurchaseOrderID int64
	SupplierID      int64
	SupplierName    string
	MaterialID      int64
	QtyG            int64
	UnitCost        float64
	Note            string
	Operator        string
}

type Repository interface {
	ListSuppliers(ctx context.Context) ([]Supplier, error)
	SaveSupplier(ctx context.Context, cmd SaveSupplierCommand) (Supplier, error)
	ListPurchaseOrders(ctx context.Context) ([]PurchaseOrder, error)
	CreatePurchaseOrder(ctx context.Context, cmd CreatePurchaseOrderCommand) (PurchaseOrder, error)
	ListPurchaseReceipts(ctx context.Context) ([]PurchaseReceipt, error)
	CreatePurchaseReceipt(ctx context.Context, cmd CreatePurchaseReceiptCommand, stockResult stockapp.MaterialReceiptResult) (PurchaseReceipt, error)
	UpdateMaterialPurchasePrice(ctx context.Context, materialID int64, unitCost float64) error
}

// MaterialPurchaseLocker is optional so non-Postgres adapters and focused
// application tests do not need to implement database locking. The Postgres
// repository uses it to keep a material-level session lock across the stock,
// purchase-price, and purchase-receipt transactions.
type MaterialPurchaseLocker interface {
	WithMaterialPurchaseLock(ctx context.Context, materialID int64, fn func(context.Context) (PurchaseReceipt, error)) (PurchaseReceipt, error)
}

type StockReceiver interface {
	ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error)
}

type Service struct {
	repo  Repository
	stock StockReceiver
}

func NewService(repo Repository, stock StockReceiver) *Service {
	return &Service{repo: repo, stock: stock}
}

func (s *Service) ListSuppliers(ctx context.Context) ([]Supplier, error) {
	return s.repo.ListSuppliers(ctx)
}

func (s *Service) SaveSupplier(ctx context.Context, cmd SaveSupplierCommand) (Supplier, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Contact = strings.TrimSpace(cmd.Contact)
	cmd.Phone = strings.TrimSpace(cmd.Phone)
	cmd.Address = strings.TrimSpace(cmd.Address)
	if cmd.Name == "" {
		return Supplier{}, fmt.Errorf("supplier name required")
	}
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SaveSupplier(ctx, cmd)
}

func (s *Service) ListPurchaseOrders(ctx context.Context) ([]PurchaseOrder, error) {
	return s.repo.ListPurchaseOrders(ctx)
}

func (s *Service) CreatePurchaseOrder(ctx context.Context, cmd CreatePurchaseOrderCommand) (PurchaseOrder, error) {
	if cmd.SupplierID <= 0 {
		return PurchaseOrder{}, fmt.Errorf("supplier required")
	}
	if cmd.MaterialID <= 0 {
		return PurchaseOrder{}, fmt.Errorf("material required")
	}
	if cmd.QtyG <= 0 {
		return PurchaseOrder{}, fmt.Errorf("qty_g required")
	}
	if cmd.UnitCost < 0 {
		return PurchaseOrder{}, fmt.Errorf("unit_cost must be >= 0")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "purchase"
	}
	return s.repo.CreatePurchaseOrder(ctx, cmd)
}

func (s *Service) ListPurchaseReceipts(ctx context.Context) ([]PurchaseReceipt, error) {
	return s.repo.ListPurchaseReceipts(ctx)
}

func (s *Service) CreatePurchaseReceipt(ctx context.Context, cmd CreatePurchaseReceiptCommand) (PurchaseReceipt, error) {
	if s.stock == nil {
		return PurchaseReceipt{}, fmt.Errorf("stock receiver required")
	}
	if cmd.SupplierID <= 0 && strings.TrimSpace(cmd.SupplierName) == "" {
		return PurchaseReceipt{}, fmt.Errorf("supplier required")
	}
	if cmd.MaterialID <= 0 {
		return PurchaseReceipt{}, fmt.Errorf("material required")
	}
	if cmd.QtyG <= 0 {
		return PurchaseReceipt{}, fmt.Errorf("qty_g required")
	}
	if cmd.UnitCost < 0 {
		return PurchaseReceipt{}, fmt.Errorf("unit_cost must be >= 0")
	}
	cmd.SupplierName = strings.TrimSpace(cmd.SupplierName)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "purchase"
	}
	create := func(receiptCtx context.Context) (PurchaseReceipt, error) {
		stockResult, err := s.stock.ReceiveMaterial(receiptCtx, stockapp.MaterialReceiptCommand{
			MaterialID: cmd.MaterialID,
			Supplier:   cmd.SupplierName,
			QtyG:       cmd.QtyG,
			UnitCost:   cmd.UnitCost,
			Note:       cmd.Note,
			Operator:   cmd.Operator,
		})
		if err != nil {
			return PurchaseReceipt{}, err
		}
		if err := s.repo.UpdateMaterialPurchasePrice(receiptCtx, cmd.MaterialID, cmd.UnitCost); err != nil {
			return PurchaseReceipt{}, err
		}
		return s.repo.CreatePurchaseReceipt(receiptCtx, cmd, stockResult)
	}
	if locker, ok := s.repo.(MaterialPurchaseLocker); ok {
		return locker.WithMaterialPurchaseLock(ctx, cmd.MaterialID, create)
	}
	return create(ctx)
}
