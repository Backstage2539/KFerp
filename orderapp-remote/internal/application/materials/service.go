package materials

import "context"

type BeanProfile struct {
	Origin            string `json:"origin"`
	ProcessingStation string `json:"processing_station"`
	Variety           string `json:"variety"`
	ProcessMethod     string `json:"process_method"`
	Grade             string `json:"grade"`
	Altitude          string `json:"altitude"`
	Flavor            string `json:"flavor"`
	BeanListNote      string `json:"bean_list_note"`
}

type Material struct {
	ID            int64        `json:"id"`
	Code          string       `json:"code"`
	Name          string       `json:"name"`
	Kind          string       `json:"kind"`
	Unit          string       `json:"unit"`
	BatchNo       string       `json:"batch_no"`
	PurchasePrice float64      `json:"purchase_price"`
	SalePrice     float64      `json:"sale_price"`
	OnhandG       int64        `json:"onhand_g"`
	OnhandUnits   int64        `json:"onhand_units"`
	MinLevelG     int64        `json:"min_level_g"`
	MinLevelUnits int64        `json:"min_level_units"`
	BeanProfile   *BeanProfile `json:"bean_profile,omitempty"`
	UpdatedAt     string       `json:"updated_at"`
}

type MaterialInput struct {
	Code          string       `json:"code"`
	Name          string       `json:"name"`
	Kind          string       `json:"kind"`
	Unit          string       `json:"unit"`
	BatchNo       string       `json:"batch_no"`
	PurchasePrice float64      `json:"purchase_price"`
	SalePrice     float64      `json:"sale_price"`
	OnhandG       int64        `json:"onhand_g"`
	OnhandUnits   int64        `json:"onhand_units"`
	MinLevelG     int64        `json:"min_level_g"`
	MinLevelUnits int64        `json:"min_level_units"`
	BeanProfile   *BeanProfile `json:"bean_profile,omitempty"`
}

type ListCommand struct {
	Query string
	Limit int
}

type UpdateCommand struct {
	Actor string
	ID    int64
	Input MaterialInput
}

type Repository interface {
	List(ctx context.Context, cmd ListCommand) ([]Material, error)
	Update(ctx context.Context, cmd UpdateCommand) (Material, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, cmd ListCommand) ([]Material, error) {
	return s.repo.List(ctx, cmd)
}

func (s *Service) Update(ctx context.Context, cmd UpdateCommand) (Material, error) {
	return s.repo.Update(ctx, cmd)
}
