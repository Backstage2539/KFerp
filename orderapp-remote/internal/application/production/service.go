package production

import (
	"context"
	"fmt"
	"strings"
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

type ListBatchesCommand struct {
	Limit    int
	Status   string
	Operator string
	From     string
	To       string
}

type BatchListItem struct {
	BatchID      string
	Status       string
	Operator     string
	CreatedAt    string
	OrderCount   int64
	DeductStatus string
	DeductedAt   string
	NeedG        int64
	DeductedG    int64
	GapG         int64

	CreatedBy       string
	CreatedTime     string
	StatusChangedAt string
	StatusText      string
	CreateTime      string
	DeductTime      string
	DeductState     string
}

type BatchDetail struct {
	BatchID      string
	Status       string
	Operator     string
	CreatedAt    string
	Orders       []int64
	Summary      []SummaryItem
	CreatedBy    string
	CreatedTime  string
	StatusSource string
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

type StartNeed struct {
	ProductID   int64
	ProductName string
	SpecG       int64
	GapG        int64
	OrderNos    string
}

type StartResult struct {
	BatchID string
}

type StartExecutionCommand struct {
	Needs      []StartNeed
	InputByKey map[string]int64
	Operator   string
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

type RoastMachine struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CapacityG    int64  `json:"capacity_g"`
	AllowedSpecs string `json:"allowed_specs"`
	MinRoastG    int64  `json:"min_roast_g"`
	Active       bool   `json:"active"`
}

type RoastMachineCommand struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CapacityG    int64  `json:"capacity_g"`
	AllowedSpecs string `json:"allowed_specs"`
	MinRoastG    int64  `json:"min_roast_g"`
	Active       bool   `json:"active"`
}

type Repository interface {
	CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error)
	ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error)
	Detail(ctx context.Context, batchID string) (BatchDetail, error)
	PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error)
	ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error)
	ListRunning(ctx context.Context) ([]RunningItem, error)
	ListStartNeeds(ctx context.Context, cmd StartCommand) ([]StartNeed, error)
	Start(ctx context.Context, cmd StartExecutionCommand) (StartResult, error)
	Finish(ctx context.Context, cmd FinishCommand) error
	Cancel(ctx context.Context, cmd CancelCommand) error
	ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error)
	SaveMachine(ctx context.Context, cmd RoastMachineCommand) error
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

func (s *Service) ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error) {
	if cmd.Limit <= 0 {
		cmd.Limit = 20
	}
	if cmd.Limit > 200 {
		cmd.Limit = 200
	}
	cmd.Status = strings.TrimSpace(cmd.Status)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.From = strings.TrimSpace(cmd.From)
	cmd.To = strings.TrimSpace(cmd.To)
	return s.repo.ListBatches(ctx, cmd)
}

func (s *Service) Detail(ctx context.Context, batchID string) (BatchDetail, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return BatchDetail{}, fmt.Errorf("batch_id required")
	}
	return s.repo.Detail(ctx, batchID)
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

func (s *Service) Finish(ctx context.Context, cmd FinishCommand) error {
	return s.repo.Finish(ctx, cmd)
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
	return s.repo.Cancel(ctx, cmd)
}

func (s *Service) ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error) {
	return s.repo.ListMachines(ctx, activeOnly)
}

func (s *Service) SaveMachine(ctx context.Context, cmd RoastMachineCommand) error {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" || cmd.CapacityG <= 0 {
		return fmt.Errorf("name and capacity_g required")
	}
	if cmd.MinRoastG <= 0 {
		cmd.MinRoastG = 1000
	}
	if cmd.MinRoastG > cmd.CapacityG {
		return fmt.Errorf("min_roast_g must be <= capacity_g")
	}
	loadSettings, ok := normalizeMachineLoadSettings(cmd.AllowedSpecs, cmd.MinRoastG, cmd.CapacityG)
	if !ok {
		return fmt.Errorf("invalid allowed_specs")
	}
	cmd.AllowedSpecs = loadSettings
	return s.repo.SaveMachine(ctx, cmd)
}
