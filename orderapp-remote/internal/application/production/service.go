package production

import (
	"context"
	"time"
)

type CreateBatchCommand struct {
	OrderIDs             []int64
	Operator             string
	IdempotencyKey       string
	RequestUnitsByItemID map[int64]int64
}

type SummaryItem struct {
	ProductID   int64
	ProductName string
	SpecG       int64
	NeedUnits   int64
	NeedG       int64
	DeductedG   int64
	GapG        int64
}

type CreateBatchResult struct {
	BatchID    string
	OrderCount int
	Summary    []SummaryItem
}

type DeductPreviewItem struct {
	ProductID       int64
	ProductName     string
	SpecG           int64
	NeedUnits       int64
	NeedG           int64
	InvUnits        int64
	InvLooseG       int64
	InvTotalG       int64
	DeductedG       int64
	GapG            int64
	WarningLowStock bool
}

type DeductPreview struct {
	BatchID string
	Summary []DeductPreviewItem
}

type DeductConfirmResult struct {
	BatchID string
	Status  string
	Summary []SummaryItem
}

type RunningItem struct {
	ID            int64
	BatchID       string
	ProductID     int64
	ProductName   string
	SpecG         int64
	NeedG         int64
	InputG        int64
	BomYieldRate  float64
	PlanUnits     int64
	PlanLooseG    int64
	OrderNos      string
	StartedBy     string
	StartedAt     string
	StartedAtTime time.Time
}

type StartCommand struct {
	From       string
	To         string
	CustomerID int64
	Selected   map[string]bool
	InputByKey map[string]int64
	Operator   string
}

type StartResult struct {
	BatchID string
}

type FinishCommand struct {
	ID               int64
	FinishedUnits    int64
	FinishedLooseG   int64
	HasFinishedInput bool
	Operator         string
}

type CancelCommand struct {
	ID       int64
	Operator string
}

type Repository interface {
	CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error)
	PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error)
	ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error)
	ListRunning(ctx context.Context) ([]RunningItem, error)
	Start(ctx context.Context, cmd StartCommand) (StartResult, error)
	Finish(ctx context.Context, cmd FinishCommand) error
	Cancel(ctx context.Context, cmd CancelCommand) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error) {
	return s.repo.CreateBatch(ctx, cmd)
}

func (s *Service) PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error) {
	return s.repo.PreviewDeduct(ctx, batchID)
}

func (s *Service) ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error) {
	return s.repo.ConfirmDeduct(ctx, batchID, operator)
}

func (s *Service) ListRunning(ctx context.Context) ([]RunningItem, error) {
	return s.repo.ListRunning(ctx)
}

func (s *Service) Start(ctx context.Context, cmd StartCommand) (StartResult, error) {
	return s.repo.Start(ctx, cmd)
}

func (s *Service) Finish(ctx context.Context, cmd FinishCommand) error {
	return s.repo.Finish(ctx, cmd)
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
	return s.repo.Cancel(ctx, cmd)
}
