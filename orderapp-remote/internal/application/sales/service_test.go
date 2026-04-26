package sales

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	saveCmd        SaveOrderCommand
	inlineCmd      InlineUpdateCommand
	saveCalls      int
	outsourceSaved SaveOutsourceTemplateCommand
}

func (r *fakeRepo) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	r.saveCmd = cmd
	r.saveCalls++
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

func (r *fakeRepo) ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error) {
	return []OutsourceTemplate{{ID: 1, Name: "默认", IsDefault: true, RoastUnitPrice: 2.5}}, nil
}

func (r *fakeRepo) SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error {
	r.outsourceSaved = cmd
	return nil
}

func TestServiceDelegatesSaveOrder(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	res, err := svc.SaveOrder(context.Background(), SaveOrderCommand{
		EditID:     10,
		OrderDate:  time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		CustomerID: 3,
		Items: []OrderItemCommand{{
			ProductID: int64Ptr(11),
			Name:      "橘皮乌龙",
			Units:     2,
			SpecG:     227,
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

func int64Ptr(v int64) *int64 {
	return &v
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

func TestServiceValidatesSaveOrderBeforeRepository(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveOrder(context.Background(), SaveOrderCommand{
		OrderDate:  time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		CustomerID: 3,
		Items: []OrderItemCommand{{
			Name:  "missing spec",
			Units: 2,
		}},
	})
	if err == nil {
		t.Fatal("SaveOrder() error = nil, want validation error")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("repository was called %d times for invalid command", repo.saveCalls)
	}
}

func TestServiceOwnsOutsourceTemplateUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	rows, err := svc.ListOutsourceTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "默认" {
		t.Fatalf("ListOutsourceTemplates() = %+v", rows)
	}

	err = svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{
		Name:              " 默认外包 ",
		IsDefault:         true,
		RoastUnitPrice:    1.5,
		BeanPackUnitPrice: 2.5,
		DripPackUnitPrice: 3.5,
		SCUnitPrice:       4.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.outsourceSaved.Name != "默认外包" || !repo.outsourceSaved.IsDefault {
		t.Fatalf("SaveOutsourceTemplate normalized command = %+v", repo.outsourceSaved)
	}

	if err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{Name: ""}); err == nil {
		t.Fatal("SaveOutsourceTemplate empty name error = nil")
	}
	if err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{Name: "坏价格", RoastUnitPrice: -1}); err == nil {
		t.Fatal("SaveOutsourceTemplate negative price error = nil")
	}
}
