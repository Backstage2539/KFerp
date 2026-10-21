package customerfulfillment

import (
	"context"
	"errors"
	"testing"
)

type stockCatalogPriceBindingsRepository struct {
	*miniDirectShipApplicationRepositoryStub
	tables     []MiniDirectShipPriceTable
	tableErr   error
	customerID int64
	usage      string
}

func (r *stockCatalogPriceBindingsRepository) LoadFulfillmentSettings(_ context.Context, id int64) (FulfillmentSettings, error) {
	return FulfillmentSettings{CustomerID: id, Revision: 1, Modes: []FulfillmentModeSettings{{Mode: "stock", Enabled: true, WarehouseCode: "CUSTOMER-STOCK"}}}, nil
}
func (r *stockCatalogPriceBindingsRepository) BoundCustomerOrderPriceTables(_ context.Context, id int64, usage string) ([]MiniDirectShipPriceTable, error) {
	r.customerID, r.usage = id, usage
	return r.tables, r.tableErr
}

func TestStockCatalogReportsBoundPriceMetadataIndependentlyOfInventory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tables []MiniDirectShipPriceTable
		err    error
	}{
		{"empty inventory retains configured table", []MiniDirectShipPriceTable{{ID: 31, TableName: "已绑定代发表", VersionNo: "V1"}}, nil},
		{"unconfigured pricing does not block owned stock", []MiniDirectShipPriceTable{}, nil},
		{"catalog does not hide binding lookup failures", nil, errors.New("binding storage unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &stockCatalogPriceBindingsRepository{miniDirectShipApplicationRepositoryStub: &miniDirectShipApplicationRepositoryStub{}, tables: tc.tables, tableErr: tc.err}
			catalog, err := NewService(r).MiniDirectShipCatalog(context.Background(), MiniDirectShipCatalogQuery{CustomerID: 74, FulfillmentMode: "stock"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("error=%v want=%v", err, tc.err)
			}
			if r.customerID != 74 || r.usage != "direct_ship" {
				t.Fatalf("wrong binding lookup scope: customer=%d usage=%q", r.customerID, r.usage)
			}
			if tc.err == nil && (len(catalog.PriceTables) != len(tc.tables) || catalog.SourceWarehouse != "CUSTOMER-STOCK" || catalog.FulfillmentMode != "stock") {
				t.Fatalf("catalog lost authoritative metadata: %+v", catalog)
			}
			if len(tc.tables) > 0 && (catalog.PriceTables[0].ID != 31 || catalog.PriceTables[0].VersionNo != "V1") {
				t.Fatalf("bound price table changed: %+v", catalog.PriceTables)
			}
		})
	}
}
