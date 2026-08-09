package customerportal

import (
	"context"
	"errors"
	"testing"
)

type processingBillRepositoryFake struct {
	Repository
	current          CurrentContext
	listCustomerID   int64
	detailCustomerID int64
	detailBillID     int64
}

func (r *processingBillRepositoryFake) CurrentContextByToken(context.Context, string) (CurrentContext, error) {
	return r.current, nil
}

func (r *processingBillRepositoryFake) ListCustomerProcessingBills(_ context.Context, customerID int64, limit int) ([]CustomerBillSummary, error) {
	r.listCustomerID = customerID
	return []CustomerBillSummary{{ID: 41, SettlementNo: "CPB-19-41", TotalAmount: "88.50", WorkOrderCount: 2}}, nil
}

func (r *processingBillRepositoryFake) GetCustomerProcessingBill(_ context.Context, customerID, billID int64) (CustomerBillDetail, error) {
	r.detailCustomerID = customerID
	r.detailBillID = billID
	if billID == 404 {
		return CustomerBillDetail{}, ErrCustomerBillNotFound
	}
	return CustomerBillDetail{CustomerBillSummary: CustomerBillSummary{ID: billID, SettlementNo: "CPB-19-41"}}, nil
}

func TestCustomerBillsUseOnlyCurrentBoundCustomerAndSettlementCapability(t *testing.T) {
	repo := &processingBillRepositoryFake{current: CurrentContext{
		CurrentCustomerID: 19,
		Capabilities:      []Capability{{Code: CapabilitySettlement, Enabled: true}},
	}}
	svc := NewService(repo, nil)
	rows, err := svc.ListCustomerBills(context.Background(), "token-19")
	if err != nil || len(rows) != 1 || repo.listCustomerID != 19 {
		t.Fatalf("ListCustomerBills() rows=%+v customer=%d err=%v", rows, repo.listCustomerID, err)
	}
	bill, err := svc.GetCustomerBill(context.Background(), "token-19", 41)
	if err != nil || bill.ID != 41 || repo.detailCustomerID != 19 || repo.detailBillID != 41 {
		t.Fatalf("GetCustomerBill() bill=%+v customer=%d id=%d err=%v", bill, repo.detailCustomerID, repo.detailBillID, err)
	}

	repo.current.Capabilities = nil
	if _, err := svc.ListCustomerBills(context.Background(), "token-19"); !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("ListCustomerBills() capability error=%v", err)
	}
	if _, err := svc.GetCustomerBill(context.Background(), "token-19", 41); !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("GetCustomerBill() capability error=%v", err)
	}
}

func TestCustomerBillDetailNotFoundDoesNotFallBackAcrossCustomers(t *testing.T) {
	repo := &processingBillRepositoryFake{current: CurrentContext{
		CurrentCustomerID: 19,
		Capabilities:      []Capability{{Code: CapabilitySettlement, Enabled: true}},
	}}
	_, err := NewService(repo, nil).GetCustomerBill(context.Background(), "token-19", 404)
	if !errors.Is(err, ErrCustomerBillNotFound) || repo.detailCustomerID != 19 {
		t.Fatalf("GetCustomerBill(not found) customer=%d err=%v", repo.detailCustomerID, err)
	}
}
