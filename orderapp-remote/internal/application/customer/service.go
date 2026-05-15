package customer

import (
	"context"
	"io"
	"strings"
)

const (
	CustomerTypeRetail    = "retail"
	CustomerTypeEcommerce = "ecommerce"
	CustomerTypeWholesale = "wholesale"
)

type UpsertCommand struct {
	Name               string
	RawName            string
	CustomerType       string
	CompanyName        string
	CompanyAddress     string
	CompanyPhone       string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             string
}

type InlineUpdateCommand struct {
	Name               string
	CustomerType       string
	CompanyName        string
	CompanyAddress     string
	CompanyPhone       string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             string
}

type Prefs struct {
	ID              int64   `json:"id"`
	DefaultSourceID *int    `json:"default_source_id"`
	SourceName      *string `json:"source_name"`
	DefaultTypeID   *int    `json:"default_order_type_id"`
	TypeName        *string `json:"order_type_name"`
	Address         *string `json:"address"`
}

type SaveAssetCommand struct {
	CustomerID  int64
	Kind        string
	Reader      io.Reader
	ContentType string
	Filename    string
	MaxBytes    int64
	Actor       string
}

type SaveAssetResult struct {
	CustomerID int64
	ObjectKey  string
	Bytes      int64
	SHA256     string
}

type DeleteAssetResult struct {
	CustomerID int64
	ObjectKey  string
}

type ListQuery struct {
	Query         string
	Limit         int
	Offset        int
	CustomerType  string
	Active        *bool
	SortBy        string
	SortDirection string
}

type ListResult struct {
	Rows       []CustomerRow
	Sources    []Option
	OrderTypes []Option
	HasNext    bool
}

type CustomerRow struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	CustomerType       string  `json:"customer_type"`
	CompanyName        string  `json:"company_name"`
	CompanyAddress     string  `json:"company_address"`
	CompanyPhone       string  `json:"company_phone"`
	Contact            *string `json:"contact"`
	Phone              *string `json:"phone"`
	Address            *string `json:"address"`
	Active             bool    `json:"active"`
	DefaultSourceID    *int    `json:"default_source_id"`
	DefaultOrderTypeID *int    `json:"default_order_type_id"`
	Updated            string  `json:"updated"`
}

type CustomerEditData struct {
	ID                 int64
	Name               string
	RawName            string
	CustomerType       string
	CompanyName        string
	CompanyAddress     string
	CompanyPhone       string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             bool
}

type CustomerAsset struct {
	ID          int64
	CustomerID  int64
	Kind        string
	ObjectKey   string
	ContentType string
	Bytes       int64
	Sha256      string
	CreatedAt   string
}

type CustomerDashboard struct {
	TotalOrders     int
	UnpaidOrders    int
	UnshippedOrders int
	InProduction    int
	InShipping      int
	Completed       int
}

type Option struct {
	ID   int64
	Name string
}

type EditorData struct {
	Customer   CustomerEditData
	Sources    []Option
	OrderTypes []Option
	Assets     []CustomerAsset
	Dashboard  CustomerDashboard
}

type AssetObject struct {
	ObjectKey   string
	ContentType string
}

type Repository interface {
	Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error)
	List(ctx context.Context, query ListQuery) (ListResult, error)
	Editor(ctx context.Context, id int64) (*EditorData, error)
	Prefs(ctx context.Context, id int64) (*Prefs, error)
	AssetObject(ctx context.Context, assetID int64) (AssetObject, error)
	SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error)
	DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error)
	InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error
	Delete(ctx context.Context, actor string, id int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error) {
	cmd.CustomerType = NormalizeCustomerType(cmd.CustomerType)
	return s.repo.Upsert(ctx, actor, id, cmd)
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	query.CustomerType = NormalizeCustomerTypeFilter(query.CustomerType)
	query.SortBy = NormalizeCustomerSortBy(query.SortBy)
	query.SortDirection = NormalizeCustomerSortDirection(query.SortDirection)
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return s.repo.List(ctx, query)
}

func (s *Service) Editor(ctx context.Context, id int64) (*EditorData, error) {
	return s.repo.Editor(ctx, id)
}

func (s *Service) Prefs(ctx context.Context, id int64) (*Prefs, error) {
	return s.repo.Prefs(ctx, id)
}

func (s *Service) AssetObject(ctx context.Context, assetID int64) (AssetObject, error) {
	return s.repo.AssetObject(ctx, assetID)
}

func (s *Service) SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error) {
	return s.repo.SaveAsset(ctx, cmd)
}

func (s *Service) DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error) {
	return s.repo.DeleteAsset(ctx, actor, assetID)
}

func (s *Service) InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error {
	cmd.CustomerType = NormalizeCustomerType(cmd.CustomerType)
	return s.repo.InlineUpdate(ctx, actor, id, cmd)
}

func (s *Service) Delete(ctx context.Context, actor string, id int64) error {
	return s.repo.Delete(ctx, actor, id)
}

func NormalizeCustomerType(value string) string {
	switch strings.TrimSpace(value) {
	case CustomerTypeWholesale:
		return CustomerTypeWholesale
	case CustomerTypeEcommerce:
		return CustomerTypeEcommerce
	default:
		return CustomerTypeRetail
	}
}

func NormalizeCustomerTypeFilter(value string) string {
	switch strings.TrimSpace(value) {
	case CustomerTypeWholesale, CustomerTypeEcommerce, CustomerTypeRetail:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func NormalizeCustomerSortBy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "name", "updated":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "name"
	}
}

func NormalizeCustomerSortDirection(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "desc", "descending":
		return "desc"
	default:
		return "asc"
	}
}
