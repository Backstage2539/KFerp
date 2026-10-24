package production

import (
	"context"
	"fmt"
)

type OperationMaterialBinding struct {
	NodeKey         string `json:"node_key"`
	SourceWarehouse string `json:"source_warehouse"`
}
type OperationMaterialConfigRow struct {
	Key     string                   `json:"key"`
	Name    string                   `json:"name"`
	Binding OperationMaterialBinding `json:"binding"`
}
type OperationMaterialConfigNode struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Sequence int    `json:"sequence"`
}
type OperationMaterialConfig struct {
	Version string                        `json:"version"`
	Rows    []OperationMaterialConfigRow  `json:"rows"`
	Nodes   []OperationMaterialConfigNode `json:"nodes"`
}
type SaveOperationMaterialConfig struct {
	VariantID       int64                               `json:"variant_id"`
	VersionID       int64                               `json:"version_id"`
	ExpectedVersion string                              `json:"expected_version"`
	Bindings        map[string]OperationMaterialBinding `json:"bindings"`
	Actor           string                              `json:"-"`
}
type operationMaterialRepository interface {
	GetOperationMaterialConfig(context.Context, int64, int64) (OperationMaterialConfig, error)
	SaveOperationMaterialConfig(context.Context, SaveOperationMaterialConfig) (OperationMaterialConfig, error)
}

func (s *Service) GetOperationMaterialConfig(ctx context.Context, id, variantID int64) (OperationMaterialConfig, error) {
	if r, ok := s.repo.(operationMaterialRepository); ok {
		return r.GetOperationMaterialConfig(ctx, id, variantID)
	}
	return OperationMaterialConfig{}, fmt.Errorf("用料工序配置不可用")
}
func (s *Service) SaveOperationMaterialConfig(ctx context.Context, c SaveOperationMaterialConfig) (OperationMaterialConfig, error) {
	if r, ok := s.repo.(operationMaterialRepository); ok {
		return r.SaveOperationMaterialConfig(ctx, c)
	}
	return OperationMaterialConfig{}, fmt.Errorf("用料工序配置不可用")
}
