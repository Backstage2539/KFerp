package stock

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var ErrWarehouseConflict = errors.New("仓库配置已变更，请重新载入后再保存")
var ErrVirtualWarehouseNotFound = errors.New("虚拟仓库不存在或已删除")

type VirtualWarehouseQuery struct {
	Q          string
	CustomerID *int64
	Limit      int
	Offset     int
}

type VirtualWarehouseResult struct {
	Rows       []WarehouseRow `json:"rows"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
	TotalPages int            `json:"total_pages"`
	HasNext    bool           `json:"has_next"`
}

type CreateVirtualWarehouseCommand struct {
	Name        string
	Kind        string
	Description string
	CustomerID  int64
	Actor       string
}

type UpdateVirtualWarehouseCommand struct {
	Code             string
	Name             string
	Description      string
	CustomerID       int64
	ExpectedRevision int64
	Actor            string
}

type DeleteVirtualWarehouseCommand struct {
	Code             string
	ExpectedRevision int64
	Actor            string
}

type VirtualWarehouseRepository interface {
	ListVirtualWarehouses(context.Context, VirtualWarehouseQuery) (VirtualWarehouseResult, error)
	CreateVirtualWarehouse(context.Context, CreateVirtualWarehouseCommand) (WarehouseRow, error)
	UpdateVirtualWarehouse(context.Context, UpdateVirtualWarehouseCommand) (WarehouseRow, error)
	DeleteVirtualWarehouse(context.Context, DeleteVirtualWarehouseCommand) (WarehouseRow, error)
}

func (s *Service) virtualWarehouseRepository() (VirtualWarehouseRepository, error) {
	r, ok := s.repo.(VirtualWarehouseRepository)
	if !ok {
		return nil, fmt.Errorf("虚拟仓库管理不可用")
	}
	return r, nil
}

func (s *Service) ListVirtualWarehouses(ctx context.Context, query VirtualWarehouseQuery) (VirtualWarehouseResult, error) {
	if query.CustomerID != nil && *query.CustomerID < 0 {
		return VirtualWarehouseResult{}, fmt.Errorf("客户编号必须为非负整数")
	}
	query.Q = strings.TrimSpace(query.Q)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 20, 100)
	r, err := s.virtualWarehouseRepository()
	if err != nil {
		return VirtualWarehouseResult{}, err
	}
	return r.ListVirtualWarehouses(ctx, query)
}

func validateVirtualWarehouseFields(name, description string, customerID int64) error {
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return fmt.Errorf("仓库名称必填，最多100字")
	}
	if utf8.RuneCountInString(description) > 1000 {
		return fmt.Errorf("仓库说明最多1000字")
	}
	if customerID < 0 {
		return fmt.Errorf("客户编号必须为非负整数")
	}
	return nil
}

func (s *Service) CreateVirtualWarehouse(ctx context.Context, cmd CreateVirtualWarehouseCommand) (WarehouseRow, error) {
	cmd.Name, cmd.Description, cmd.Kind = strings.TrimSpace(cmd.Name), strings.TrimSpace(cmd.Description), strings.TrimSpace(cmd.Kind)
	if err := validateVirtualWarehouseFields(cmd.Name, cmd.Description, cmd.CustomerID); err != nil {
		return WarehouseRow{}, err
	}
	switch cmd.Kind {
	case "raw", "packaging", "finished":
	default:
		return WarehouseRow{}, fmt.Errorf("请选择原料、包材或成品仓库类型")
	}
	r, err := s.virtualWarehouseRepository()
	if err != nil {
		return WarehouseRow{}, err
	}
	return r.CreateVirtualWarehouse(ctx, cmd)
}

func (s *Service) UpdateVirtualWarehouse(ctx context.Context, cmd UpdateVirtualWarehouseCommand) (WarehouseRow, error) {
	cmd.Code, cmd.Name, cmd.Description = strings.TrimSpace(cmd.Code), strings.TrimSpace(cmd.Name), strings.TrimSpace(cmd.Description)
	if cmd.Code == "" || cmd.ExpectedRevision < 1 {
		return WarehouseRow{}, fmt.Errorf("仓库编号及配置版本必填")
	}
	if err := validateVirtualWarehouseFields(cmd.Name, cmd.Description, cmd.CustomerID); err != nil {
		return WarehouseRow{}, err
	}
	r, err := s.virtualWarehouseRepository()
	if err != nil {
		return WarehouseRow{}, err
	}
	return r.UpdateVirtualWarehouse(ctx, cmd)
}

func (s *Service) DeleteVirtualWarehouse(ctx context.Context, cmd DeleteVirtualWarehouseCommand) (WarehouseRow, error) {
	cmd.Code = strings.TrimSpace(cmd.Code)
	if cmd.Code == "" || cmd.ExpectedRevision < 1 {
		return WarehouseRow{}, fmt.Errorf("仓库编号及配置版本必填")
	}
	r, err := s.virtualWarehouseRepository()
	if err != nil {
		return WarehouseRow{}, err
	}
	return r.DeleteVirtualWarehouse(ctx, cmd)
}
