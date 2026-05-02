package sales

import "testing"

func TestNextDeliveryNoteVersion(t *testing.T) {
	if got := NextDeliveryNoteVersion(nil); got != 1 {
		t.Fatalf("NextDeliveryNoteVersion(nil) = %d, want 1", got)
	}
	if got := NextDeliveryNoteVersion([]int{1, 2, 4}); got != 5 {
		t.Fatalf("NextDeliveryNoteVersion([1,2,4]) = %d, want 5", got)
	}
}

func TestDeliveryNoteSnapshotValidate(t *testing.T) {
	s := DeliveryNoteSnapshot{
		OrderID:             9,
		OrderNo:             "SO-20260502-0001",
		DeliveryNoteNo:      "DN-SO-20260502-0001",
		CompanyName:         "棵凡咖啡",
		CustomerName:        "某某咖啡馆",
		SourceWarehouse:     "finished_goods",
		SourceWarehouseName: "成品仓",
		Items: []DeliveryNoteSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", Warehouse: "finished_goods", WarehouseName: "成品仓",
		}},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDeliveryNoteSnapshotValidateRequiresWarehouseAndItems(t *testing.T) {
	s := DeliveryNoteSnapshot{OrderID: 9, OrderNo: "SO-20260502-0001", DeliveryNoteNo: "DN-SO-20260502-0001", CompanyName: "棵凡咖啡"}
	if err := s.Validate(); err == nil {
		t.Fatalf("Validate() error = nil, want missing warehouse/items error")
	}
}
