package stock

import (
	"context"
	"testing"
)

type virtualWarehouseFakeRepo struct {
	fakeRepo
	created CreateVirtualWarehouseCommand
	updated UpdateVirtualWarehouseCommand
	deleted DeleteVirtualWarehouseCommand
	listed  VirtualWarehouseQuery
}

func (f *virtualWarehouseFakeRepo) ListVirtualWarehouses(_ context.Context, q VirtualWarehouseQuery) (VirtualWarehouseResult, error) {
	f.listed = q
	return VirtualWarehouseResult{}, nil
}
func (f *virtualWarehouseFakeRepo) CreateVirtualWarehouse(_ context.Context, c CreateVirtualWarehouseCommand) (WarehouseRow, error) {
	f.created = c
	return WarehouseRow{Code: "virtual_generated", Name: c.Name, Kind: c.Kind, CustomerID: c.CustomerID, IsVirtual: true, Revision: 1}, nil
}
func (f *virtualWarehouseFakeRepo) UpdateVirtualWarehouse(_ context.Context, c UpdateVirtualWarehouseCommand) (WarehouseRow, error) {
	f.updated = c
	return WarehouseRow{}, nil
}
func (f *virtualWarehouseFakeRepo) DeleteVirtualWarehouse(_ context.Context, c DeleteVirtualWarehouseCommand) (WarehouseRow, error) {
	f.deleted = c
	return WarehouseRow{}, nil
}

func TestVirtualWarehouseValidationDoesNotInferCustomerOrKind(t *testing.T) {
	r := &virtualWarehouseFakeRepo{}
	s := NewService(r)
	ctx := context.Background()
	for _, c := range []CreateVirtualWarehouseCommand{{Name: " ", Kind: "finished"}, {Name: "仓库", Kind: "virtual"}, {Name: "仓库", Kind: "finished", CustomerID: -1}} {
		if _, err := s.CreateVirtualWarehouse(ctx, c); err == nil {
			t.Fatalf("invalid create accepted: %+v", c)
		}
	}
	row, err := s.CreateVirtualWarehouse(ctx, CreateVirtualWarehouseCommand{Name: " 未绑定成品仓 ", Kind: "finished", Description: " 说明 "})
	if err != nil {
		t.Fatal(err)
	}
	if row.CustomerID != 0 || r.created.Name != "未绑定成品仓" || r.created.Description != "说明" {
		t.Fatalf("creation changed customer/defaults: %+v", r.created)
	}
	if _, err := s.UpdateVirtualWarehouse(ctx, UpdateVirtualWarehouseCommand{Code: row.Code, Name: row.Name}); err == nil {
		t.Fatal("update omitted revision accepted")
	}
	if _, err := s.DeleteVirtualWarehouse(ctx, DeleteVirtualWarehouseCommand{Code: row.Code}); err == nil {
		t.Fatal("delete omitted revision accepted")
	}
	if _, err := s.ListVirtualWarehouses(ctx, VirtualWarehouseQuery{Limit: 9999, Offset: -1}); err != nil {
		t.Fatal(err)
	}
	if r.listed.Limit != 20 || r.listed.Offset != 0 || r.listed.CustomerID != nil {
		t.Fatalf("list limits/scope=%+v", r.listed)
	}
}
