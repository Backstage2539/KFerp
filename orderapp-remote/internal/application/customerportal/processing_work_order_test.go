package customerportal

import (
	"context"
	"testing"
)

func TestCreateProcessingRequestAcceptsMultipleTargetsWithoutClientSelectedInputs(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			MiniUserID:        31,
			CurrentCustomerID: 47,
			Capabilities:      []Capability{{Code: CapabilityProcessing, Enabled: true}},
		},
		processingRequest: ProcessingRequest{ID: 9, RequestNo: "PJ-9", Status: "submitted"},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{
		Items: []ProcessingRequestItemCommand{
			{ProductID: 101, SpecG: 227, Qty: 3},
			{ProductID: 102, SpecG: 454, Qty: 2},
		},
		Note: "多目标生产申请",
	})
	if err != nil {
		t.Fatalf("CreateProcessingRequest() error = %v", err)
	}
	if repo.processingCommand.CustomerID != 47 || repo.processingCommand.CreatedByMiniUserID != 31 {
		t.Fatalf("processing command identity = %+v", repo.processingCommand)
	}
	if len(repo.processingCommand.Items) != 2 || repo.processingCommand.Items[0].ProductID != 101 || repo.processingCommand.Items[1].Qty != 2 {
		t.Fatalf("processing command items = %+v", repo.processingCommand.Items)
	}
	if repo.processingCommand.InputMaterialID != 0 || repo.processingCommand.InputQtyG != 0 {
		t.Fatalf("client-selected input leaked into command = %+v", repo.processingCommand)
	}
}

func TestCreateProcessingRequestRejectsDuplicateAndInvalidTargetRows(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			MiniUserID:        31,
			CurrentCustomerID: 47,
			Capabilities:      []Capability{{Code: CapabilityProcessing, Enabled: true}},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	for _, tc := range []struct {
		name  string
		items []ProcessingRequestItemCommand
		want  string
	}{
		{name: "empty", want: "items required"},
		{name: "invalid product", items: []ProcessingRequestItemCommand{{SpecG: 227, Qty: 1}}, want: "target_product required"},
		{name: "invalid spec", items: []ProcessingRequestItemCommand{{ProductID: 1, Qty: 1}}, want: "target_spec required"},
		{name: "invalid qty", items: []ProcessingRequestItemCommand{{ProductID: 1, SpecG: 227}}, want: "target_qty required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: tc.items})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}

	_, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: []ProcessingRequestItemCommand{
		{ProductID: 1, SpecG: 227, Qty: 1},
		{ProductID: 1, SpecG: 227, Qty: 2},
	}})
	if err != nil {
		t.Fatalf("duplicate rows should merge: %v", err)
	}
	if len(repo.processingCommand.Items) != 1 || repo.processingCommand.Items[0].Qty != 3 {
		t.Fatalf("merged items = %+v, want one 3-unit row", repo.processingCommand.Items)
	}
}

func TestCreateProcessingRequestNormalizesBOMSpecIdentityWithoutChangingLegacyRows(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			MiniUserID:        31,
			CurrentCustomerID: 47,
			Capabilities:      []Capability{{Code: CapabilityProcessing, Enabled: true}},
		},
		processingRequest: ProcessingRequest{ID: 9, RequestNo: "PJ-9", Status: "submitted"},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: []ProcessingRequestItemCommand{
		{ProductID: 101, BomSpecID: 501, BomVariantID: 601, SpecG: 227, Qty: 2},
		{ProductID: 101, BomSpecID: 501, BomVariantID: 601, Qty: 3},
		{ProductID: 102, SpecG: 454, Qty: 4},
	}})
	if err != nil {
		t.Fatalf("CreateProcessingRequest() error = %v", err)
	}
	if len(repo.processingCommand.Items) != 2 {
		t.Fatalf("normalized items = %+v", repo.processingCommand.Items)
	}
	canonical := repo.processingCommand.Items[0]
	if canonical.ProductID != 101 || canonical.BomSpecID != 501 || canonical.BomVariantID != 601 || canonical.SpecG != 0 || canonical.Qty != 5 {
		t.Fatalf("canonical item = %+v", canonical)
	}
	legacy := repo.processingCommand.Items[1]
	if legacy.ProductID != 102 || legacy.BomSpecID != 0 || legacy.BomVariantID != 0 || legacy.SpecG != 454 || legacy.Qty != 4 {
		t.Fatalf("legacy item = %+v", legacy)
	}

	for _, tc := range []struct {
		name string
		item ProcessingRequestItemCommand
		want string
	}{
		{name: "missing spec", item: ProcessingRequestItemCommand{ProductID: 101, BomVariantID: 601, Qty: 1}, want: "bom_spec_id required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: []ProcessingRequestItemCommand{tc.item}})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	_, err = svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: []ProcessingRequestItemCommand{{ProductID: 101, BomSpecID: 501, Qty: 1}}})
	if err != nil || repo.processingCommand.Items[0].BomVariantID != 0 {
		t.Fatalf("server-resolved variant command=%+v err=%v", repo.processingCommand, err)
	}

	_, err = svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{Items: []ProcessingRequestItemCommand{
		{ProductID: 101, BomSpecID: 501, BomVariantID: 601, Qty: 1},
		{ProductID: 101, BomSpecID: 501, BomVariantID: 602, Qty: 1},
	}})
	if err == nil || err.Error() != "bom_variant_id mismatch for BOM spec" {
		t.Fatalf("variant mismatch error = %v", err)
	}
}
