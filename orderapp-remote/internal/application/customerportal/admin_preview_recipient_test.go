package customerportal

import (
	"context"
	"testing"
)

func TestAdminPreviewRecipientAddressesUsePreviewCustomer(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	repo.recipientAddresses = []CustomerRecipientAddress{{ID: 8, CustomerID: 74}}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListRecipientAddresses(ctx, issued.Token)
	if err != nil || len(rows) != 1 || repo.recipientListCustomerID != 74 {
		t.Fatalf("addresses: rows=%+v customer=%d err=%v", rows, repo.recipientListCustomerID, err)
	}
	repo.admin = false
	if _, err := svc.ListRecipientAddresses(ctx, issued.Token); err == nil {
		t.Fatal("revoked administrator can still read addresses")
	}
}
