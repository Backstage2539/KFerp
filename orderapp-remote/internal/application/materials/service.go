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

type PackProfile struct {
	SizeSpec   string `json:"size_spec"`
	Dimensions string `json:"dimensions"`
	Material   string `json:"material"`
	Capacity   string `json:"capacity"`
	Color      string `json:"color"`
	Note       string `json:"note"`
}

type Material struct {
	ID                         int64                        `json:"id"`
	Code                       string                       `json:"code"`
	Name                       string                       `json:"name"`
	Kind                       string                       `json:"kind"`
	Unit                       string                       `json:"unit"`
	BatchNo                    string                       `json:"batch_no"`
	PurchasePrice              float64                      `json:"purchase_price"`
	SalePrice                  float64                      `json:"sale_price"`
	OnhandG                    int64                        `json:"onhand_g"`
	OnhandUnits                int64                        `json:"onhand_units"`
	StockQty                   float64                      `json:"stock_qty"`
	MinLevelG                  int64                        `json:"min_level_g"`
	MinLevelUnits              int64                        `json:"min_level_units"`
	MinLevelQty                float64                      `json:"min_level_qty"`
	IndustryFieldTemplateID    int64                        `json:"industry_field_template_id"`
	IndustryFields             []MaterialIndustryFieldValue `json:"industry_fields"`
	ClassificationGroupID      int64                        `json:"classification_group_id"`
	ClassificationGroupName    string                       `json:"classification_group_name"`
	ClassificationCategoryID   int64                        `json:"classification_category_id"`
	ClassificationCategoryName string                       `json:"classification_category_name"`
	BeanProfile                *BeanProfile                 `json:"bean_profile,omitempty"`
	PackProfile                *PackProfile                 `json:"pack_profile,omitempty"`
	UpdatedAt                  string                       `json:"updated_at"`
	DeprecatedAt               string                       `json:"deprecated_at,omitempty"`
}

type MaterialInput struct {
	Code                    string                       `json:"code"`
	Name                    string                       `json:"name"`
	Kind                    string                       `json:"kind"`
	Unit                    string                       `json:"unit"`
	BatchNo                 string                       `json:"batch_no"`
	PurchasePrice           float64                      `json:"purchase_price"`
	SalePrice               float64                      `json:"sale_price"`
	OnhandG                 int64                        `json:"onhand_g"`
	OnhandUnits             int64                        `json:"onhand_units"`
	MinLevelG               int64                        `json:"min_level_g"`
	MinLevelUnits           int64                        `json:"min_level_units"`
	MinLevelQty             float64                      `json:"min_level_qty"`
	IndustryFieldTemplateID int64                        `json:"industry_field_template_id"`
	IndustryFields          []MaterialIndustryFieldValue `json:"industry_fields"`
	BeanProfile             *BeanProfile                 `json:"bean_profile,omitempty"`
	PackProfile             *PackProfile                 `json:"pack_profile,omitempty"`
}

type MaterialIndustryFieldValue struct {
	FieldKey  string `json:"field_key"`
	ValueText string `json:"value_text"`
}

type MaterialClassificationCategory struct {
	ID        int64  `json:"id"`
	GroupID   int64  `json:"group_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type MaterialClassificationGroup struct {
	ID         int64                            `json:"id"`
	Name       string                           `json:"name"`
	SortOrder  int                              `json:"sort_order"`
	Categories []MaterialClassificationCategory `json:"categories"`
}

type ListCommand struct {
	Query             string
	Active            string
	Limit             int
	IncludeDeprecated bool
}

type CreateCommand struct {
	Actor string
	Input MaterialInput
}

type UpdateCommand struct {
	Actor string
	ID    int64
	Input MaterialInput
}

type DeprecateCommand struct {
	Actor string
	ID    int64
}

type SaveClassificationGroupCommand struct {
	Actor     string
	ID        int64
	Name      string
	SortOrder int
}

type DeleteClassificationGroupCommand struct {
	Actor string
	ID    int64
}

type SaveClassificationCategoryCommand struct {
	Actor     string
	ID        int64
	GroupID   int64
	Name      string
	SortOrder int
}

type DeleteClassificationCategoryCommand struct {
	Actor string
	ID    int64
}

type AssignClassificationCommand struct {
	Actor       string
	MaterialIDs []int64
	GroupID     int64
	CategoryID  int64
}

type Repository interface {
	List(ctx context.Context, cmd ListCommand) ([]Material, error)
	Create(ctx context.Context, cmd CreateCommand) (Material, error)
	Update(ctx context.Context, cmd UpdateCommand) (Material, error)
	Deprecate(ctx context.Context, cmd DeprecateCommand) (Material, error)
	ListClassificationGroups(ctx context.Context) ([]MaterialClassificationGroup, error)
	SaveClassificationGroup(ctx context.Context, cmd SaveClassificationGroupCommand) (MaterialClassificationGroup, error)
	DeleteClassificationGroup(ctx context.Context, cmd DeleteClassificationGroupCommand) error
	SaveClassificationCategory(ctx context.Context, cmd SaveClassificationCategoryCommand) (MaterialClassificationCategory, error)
	DeleteClassificationCategory(ctx context.Context, cmd DeleteClassificationCategoryCommand) error
	AssignClassification(ctx context.Context, cmd AssignClassificationCommand) error
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

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (Material, error) {
	return s.repo.Create(ctx, cmd)
}

func (s *Service) Update(ctx context.Context, cmd UpdateCommand) (Material, error) {
	return s.repo.Update(ctx, cmd)
}

func (s *Service) Deprecate(ctx context.Context, cmd DeprecateCommand) (Material, error) {
	return s.repo.Deprecate(ctx, cmd)
}

func (s *Service) ListClassificationGroups(ctx context.Context) ([]MaterialClassificationGroup, error) {
	return s.repo.ListClassificationGroups(ctx)
}

func (s *Service) SaveClassificationGroup(ctx context.Context, cmd SaveClassificationGroupCommand) (MaterialClassificationGroup, error) {
	return s.repo.SaveClassificationGroup(ctx, cmd)
}

func (s *Service) DeleteClassificationGroup(ctx context.Context, cmd DeleteClassificationGroupCommand) error {
	return s.repo.DeleteClassificationGroup(ctx, cmd)
}

func (s *Service) SaveClassificationCategory(ctx context.Context, cmd SaveClassificationCategoryCommand) (MaterialClassificationCategory, error) {
	return s.repo.SaveClassificationCategory(ctx, cmd)
}

func (s *Service) DeleteClassificationCategory(ctx context.Context, cmd DeleteClassificationCategoryCommand) error {
	return s.repo.DeleteClassificationCategory(ctx, cmd)
}

func (s *Service) AssignClassification(ctx context.Context, cmd AssignClassificationCommand) error {
	return s.repo.AssignClassification(ctx, cmd)
}
