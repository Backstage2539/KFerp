package customer

import (
	"context"
	"io"
)

type UpsertCommand struct {
	Name               string
	RawName            string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             string
}

type InlineUpdateCommand struct {
	Name               string
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

type Repository interface {
	Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error)
	Prefs(ctx context.Context, id int64) (*Prefs, error)
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
	return s.repo.Upsert(ctx, actor, id, cmd)
}

func (s *Service) Prefs(ctx context.Context, id int64) (*Prefs, error) {
	return s.repo.Prefs(ctx, id)
}

func (s *Service) SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error) {
	return s.repo.SaveAsset(ctx, cmd)
}

func (s *Service) DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error) {
	return s.repo.DeleteAsset(ctx, actor, assetID)
}

func (s *Service) InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error {
	return s.repo.InlineUpdate(ctx, actor, id, cmd)
}

func (s *Service) Delete(ctx context.Context, actor string, id int64) error {
	return s.repo.Delete(ctx, actor, id)
}
