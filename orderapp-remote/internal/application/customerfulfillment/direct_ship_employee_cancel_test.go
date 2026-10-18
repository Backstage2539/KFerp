package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
)

type employeeCancelRepo struct {
	Repository
	MiniDirectShipRepository
	customerID, orderID, requestID int64
	actor                          string
	resolveError                   error
}

func (r *employeeCancelRepo) FindDirectShipRequestForOrder(_ context.Context, customerID, orderID int64) (int64, error) {
	r.customerID = customerID
	r.orderID = orderID
	if r.resolveError != nil {
		return 0, r.resolveError
	}
	return 71, nil
}
func (r *employeeCancelRepo) CancelMiniDirectShipRequest(_ context.Context, customerID, requestID int64, actor string) (MiniDirectShipRequest, error) {
	r.customerID = customerID
	r.requestID = requestID
	r.actor = actor
	return MiniDirectShipRequest{ID: requestID, Status: "cancelled"}, nil
}
func TestEmployeeCancelDirectShipOrderResolvesScopedRequestBeforeAtomicCancel(t *testing.T) {
	r := &employeeCancelRepo{}
	s := NewService(r)
	out, err := s.CancelEmployeeDirectShipOrder(context.Background(), CancelEmployeeDirectShipOrderCommand{CustomerID: 147, OrderID: 98, Actor: "内部员工"})
	if err != nil || out.ID != 71 || r.customerID != 147 || r.orderID != 98 || r.requestID != 71 || r.actor != "内部员工" {
		t.Fatalf("out=%+v repo=%+v err=%v", out, r, err)
	}
	r.requestID = 0
	r.resolveError = fmt.Errorf("scope denied")
	if _, err = s.CancelEmployeeDirectShipOrder(context.Background(), CancelEmployeeDirectShipOrderCommand{CustomerID: 148, OrderID: 98, Actor: "内部员工"}); err == nil || r.requestID != 0 {
		t.Fatal("scope error still cancelled request")
	}
}
