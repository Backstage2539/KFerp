package sales

import (
	"context"
	"testing"
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

	res, err := svc.SaveOrder(context.Background(), SaveOrderCommand{EditID: 10, CustomerID: 3})
	if err != nil {
		t.Fatalf("SaveOrder() error = %v", err)
	}
	if !res.Edited || res.OrderID != 7 || res.OrderNo != "SO-TEST" {
		t.Fatalf("SaveOrder() result = %+v", res)
	}
	if repo.saveCmd.CustomerID != 3 {
		t.Fatalf("repo command = %+v", repo.saveCmd)
	}
}

func TestSaveOrderCommandOutsourceGetters(t *testing.T) {
	cmd := SaveOrderCommand{
		OutsourceMaterialFee:  "1",
		OutsourceRoastFee:     "2",
		OutsourcePackagingFee: "3",
		OutsourceManualFee:    "4",
		OutsourceTaxFee:       "5",
		OutsourceOtherFee:     "6",
	}
	if cmd.GetMaterial() != "1" || cmd.GetRoast() != "2" || cmd.GetPackaging() != "3" || cmd.GetManual() != "4" || cmd.GetTax() != "5" || cmd.GetOther() != "6" {
		t.Fatalf("unexpected outsource getters: %+v", cmd)
	}
}
