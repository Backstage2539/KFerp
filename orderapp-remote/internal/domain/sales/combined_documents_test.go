package sales

import "testing"

func TestCombinedSalesOrderSnapshotValidateRequiresSameCustomerOrderGroups(t *testing.T) {
	snapshot := CombinedSalesOrderSnapshot{
		CombinationKey:      "combo-1-2",
		CombinedNo:          "CSO-SO-001-SO-002",
		CustomerID:          3,
		CustomerName:        "测试客户",
		CompanyName:         "棵凡咖啡",
		CustomerCompanyName: "测试客户公司",
		OrderIDs:            []int64{1, 2},
		OrderNos:            []string{"SO-001", "SO-002"},
		Groups: []CombinedSalesOrderGroup{
			{
				OrderID:      1,
				OrderNo:      "SO-001",
				DocumentDate: "2026-05-24",
				OrderDate:    "2026-05-20",
				Items: []SalesOrderSnapshotItem{
					{Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00"},
				},
				TotalAmount: "134.00",
				GrandTotal:  "134.00",
			},
			{
				OrderID:      2,
				OrderNo:      "SO-002",
				DocumentDate: "2026-05-24",
				OrderDate:    "2026-05-21",
				Items: []SalesOrderSnapshotItem{
					{Name: "白月光-瑰夏", Spec: "227g", Qty: "1", Unit: "件", UnitPrice: "190.00", LineTotal: "190.00"},
				},
				TotalAmount: "190.00",
				GrandTotal:  "190.00",
			},
		},
		TotalAmount: "324.00",
		GrandTotal:  "324.00",
	}

	if err := snapshot.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	snapshot.Groups = snapshot.Groups[:1]
	if err := snapshot.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want at least two grouped orders")
	}
}

func TestCombinedDeliveryNoteSnapshotValidateRequiresGroupedItems(t *testing.T) {
	snapshot := CombinedDeliveryNoteSnapshot{
		CombinationKey:      "combo-1-2",
		DeliveryNoteNo:      "CDN-SO-001-SO-002",
		CustomerID:          3,
		CustomerName:        "测试客户",
		CompanyName:         "棵凡咖啡",
		CustomerCompanyName: "测试客户公司",
		OrderIDs:            []int64{1, 2},
		OrderNos:            []string{"SO-001", "SO-002"},
		Groups: []CombinedDeliveryNoteGroup{
			{
				OrderID:         1,
				OrderNo:         "SO-001",
				DocumentDate:    "2026-05-24",
				OrderDate:       "2026-05-20",
				PostingDate:     "2026-05-24",
				ReceiverName:    "张三",
				TrackingNo:      "SF001",
				SourceWarehouse: "finished_goods",
				Items: []DeliveryNoteSnapshotItem{
					{Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", Warehouse: "finished_goods"},
				},
			},
			{
				OrderID:         2,
				OrderNo:         "SO-002",
				DocumentDate:    "2026-05-24",
				OrderDate:       "2026-05-21",
				PostingDate:     "2026-05-24",
				ReceiverName:    "李四",
				TrackingNo:      "SF002",
				SourceWarehouse: "finished_goods",
				Items: []DeliveryNoteSnapshotItem{
					{Name: "白月光-瑰夏", Spec: "227g", Qty: "1", Unit: "件", Warehouse: "finished_goods"},
				},
			},
		},
	}

	if err := snapshot.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	snapshot.Groups[1].Items = nil
	if err := snapshot.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want grouped order items required")
	}
}

func TestNextCombinedDocumentVersion(t *testing.T) {
	if got := NextCombinedDocumentVersion(nil); got != 1 {
		t.Fatalf("NextCombinedDocumentVersion(nil) = %d, want 1", got)
	}
	if got := NextCombinedDocumentVersion([]int{1, 3, 2}); got != 4 {
		t.Fatalf("NextCombinedDocumentVersion([1 3 2]) = %d, want 4", got)
	}
}
