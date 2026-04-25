package sales

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	saveCmd   SaveOrderCommand
	inlineCmd InlineUpdateCommand
}

func (r *fakeRepo) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	r.saveCmd = cmd
	return SaveOrderResult{OrderID: 7, OrderNo: "SO-TEST", Edited: cmd.EditID > 0}, nil
}

func (r *fakeRepo) UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error {
	return nil
}

func (r *fakeRepo) InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error {
	r.inlineCmd = cmd
	return nil
}

func (r *fakeRepo) Void(ctx context.Context, id int64, actor, reason string) error {
	return nil
}

func (r *fakeRepo) Unvoid(ctx context.Context, id int64, actor string) error {
	return nil
}

func TestServiceDelegatesSaveOrder(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	res, err := svc.SaveOrder(context.Background(), SaveOrderCommand{
		EditID:    10,
		OrderDate: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		CustomerID: 3,
		Items: []OrderItemCommand{{
			Name:  "橘皮乌龙",
			Units: 2,
			SpecG: 227,
		}},
	})
	if err != nil {
		t.Fatalf("SaveOrder() error = %v", err)
	}
	if !res.Edited || res.OrderID != 7 || res.OrderNo != "SO-TEST" {
		t.Fatalf("SaveOrder() result = %+v", res)
	}
	if repo.saveCmd.CustomerID != 3 {
		t.Fatalf("repo command = %+v", repo.saveCmd)
	}
	if len(repo.saveCmd.Items) != 1 || repo.saveCmd.Items[0].SpecG != 227 {
		t.Fatalf("repo items = %+v", repo.saveCmd.Items)
	}
}

func TestSaveOrderCommandUsesTypedFields(t *testing.T) {
	cmd := SaveOrderCommand{
		ShippingAmount:        9.5,
		DiscountAmount:        1.25,
		RoundToInt:            true,
		OutsourceMaterialFee:  1,
		OutsourceRoastFee:     2,
		OutsourcePackagingFee: 3,
		OutsourceManualFee:    4,
		OutsourceTaxFee:       5,
		OutsourceOtherFee:     6,
	}
	if cmd.ShippingAmount != 9.5 || cmd.DiscountAmount != 1.25 || !cmd.RoundToInt {
		t.Fatalf("unexpected amount fields: %+v", cmd)
	}
	if cmd.OutsourceMaterialFee+cmd.OutsourceRoastFee+cmd.OutsourcePackagingFee+cmd.OutsourceManualFee+cmd.OutsourceTaxFee+cmd.OutsourceOtherFee != 21 {
		t.Fatalf("unexpected outsource fields: %+v", cmd)
	}
}
