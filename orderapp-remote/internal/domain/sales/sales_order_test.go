package sales

import "testing"

func TestNextSalesOrderVersion(t *testing.T) {
	if got := NextSalesOrderVersion(nil); got != 1 {
		t.Fatalf("NextSalesOrderVersion(nil) = %d, want 1", got)
	}
	if got := NextSalesOrderVersion([]int{1, 2, 4}); got != 5 {
		t.Fatalf("NextSalesOrderVersion([1,2,4]) = %d, want 5", got)
	}
}

func TestFormatSalesOrderMoney(t *testing.T) {
	if got := FormatSalesOrderMoney(322); got != "322.00" {
		t.Fatalf("FormatSalesOrderMoney(322) = %q, want 322.00", got)
	}
	if got := FormatSalesOrderMoney(67.125); got != "67.13" {
		t.Fatalf("FormatSalesOrderMoney(67.125) = %q, want 67.13", got)
	}
}

func TestSalesOrderSnapshotValidate(t *testing.T) {
	s := SalesOrderSnapshot{
		OrderID:      9,
		OrderNo:      "SO-20260430-0008",
		CompanyName:  "浅焙作坊咖啡",
		CustomerName: "某某咖啡馆",
		Items: []SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		GrandTotal: "134.00",
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSalesOrderSnapshotValidateRequiresItems(t *testing.T) {
	s := SalesOrderSnapshot{OrderID: 9, OrderNo: "SO-20260430-0008", CompanyName: "浅焙作坊咖啡", GrandTotal: "134.00"}
	if err := s.Validate(); err == nil {
		t.Fatalf("Validate() error = nil, want missing items error")
	}
}
