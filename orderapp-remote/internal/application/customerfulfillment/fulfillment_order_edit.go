package customerfulfillment

import (
	"context"
	"fmt"
	freightdomain "orderapp/internal/domain/freight"
	"strings"
)

// FulfillmentOrderEditCommand describes a replacement draft of the same order.
// Customer and mode are always checked against the locked persisted order.
type FulfillmentOrderEditCommand struct {
	OrderID              int64                             `json:"-"`
	CustomerID           int64                             `json:"-"`
	Actor                string                            `json:"-"`
	RecipientOnly        bool                              `json:"-"`
	ExpectedEditRevision string                            `json:"edit_revision"`
	PreviewToken         string                            `json:"preview_token,omitempty"`
	OrderDate            string                            `json:"order_date"`
	ReceiverName         string                            `json:"receiver_name"`
	ReceiverPhone        string                            `json:"receiver_phone"`
	ReceiverAddress      string                            `json:"receiver_address"`
	ReceiverCompany      *string                           `json:"receiver_company,omitempty"`
	Province             string                            `json:"province"`
	City                 string                            `json:"city"`
	District             string                            `json:"district"`
	Notes                string                            `json:"notes"`
	Items                []FulfillmentOrderEditItemCommand `json:"items"`
	ServiceFeeCents      *int64                            `json:"service_fee_cents,omitempty"`
	AdjustmentReason     string                            `json:"adjustment_reason,omitempty"`
}

type FulfillmentOrderEditItemCommand struct {
	ItemID              int64    `json:"item_id"`
	ProductID           int64    `json:"product_id"`
	BomSpecID           int64    `json:"bom_spec_id"`
	BomVariantID        int64    `json:"bom_variant_id"`
	SpecG               int64    `json:"spec_g,omitempty"`
	Qty                 int64    `json:"qty"`
	SalesUnit           string   `json:"sales_unit,omitempty"`
	PublicationID       int64    `json:"publication_id,omitempty"`
	PriceSelectionMode  string   `json:"price_selection_mode,omitempty"`
	SelectedPriceRowKey string   `json:"selected_price_row_key,omitempty"`
	UnitPrice           *float64 `json:"unit_price,omitempty"`
}

type FulfillmentOrderEditItem struct {
	ItemID              int64   `json:"item_id"`
	RequestItemID       int64   `json:"request_item_id"`
	ProductID           int64   `json:"product_id"`
	BomSpecID           int64   `json:"bom_spec_id"`
	BomVariantID        int64   `json:"bom_variant_id"`
	BomSpecKey          string  `json:"bom_spec_key"`
	ProductName         string  `json:"product_name"`
	SKUCode             string  `json:"sku_code"`
	SpecName            string  `json:"spec_name"`
	SpecLabel           string  `json:"spec_label"`
	InventoryUnit       string  `json:"inventory_unit"`
	SalesUnit           string  `json:"sales_unit"`
	SpecG               int64   `json:"spec_g"`
	Qty                 int64   `json:"qty"`
	UnitPrice           float64 `json:"unit_price"`
	LineAmount          float64 `json:"line_amount"`
	BaseLineAmount      float64 `json:"base_line_amount"`
	DiscountAmount      float64 `json:"discount_amount"`
	DiscountType        string  `json:"discount_type,omitempty"`
	DiscountValue       float64 `json:"discount_value,omitempty"`
	PublicationID       int64   `json:"publication_id"`
	PriceSelectionMode  string  `json:"price_selection_mode,omitempty"`
	SelectedPriceRowKey string  `json:"selected_price_row_key,omitempty"`
	PriceSourceJSON     string  `json:"-"`
}

type FulfillmentOrderEditContext struct {
	GoodsTotalCents int64                      `json:"goods_total_cents"`
	OrderID         int64                      `json:"order_id"`
	OrderNo         string                     `json:"order_no"`
	CustomerID      int64                      `json:"customer_id"`
	RequestID       int64                      `json:"request_id"`
	FulfillmentMode string                     `json:"fulfillment_mode"`
	EditRevision    string                     `json:"edit_revision"`
	CanEdit         bool                       `json:"can_edit"`
	EditBlockReason string                     `json:"edit_block_reason,omitempty"`
	OrderDate       string                     `json:"order_date"`
	ReceiverName    string                     `json:"receiver_name"`
	ReceiverPhone   string                     `json:"receiver_phone"`
	ReceiverAddress string                     `json:"receiver_address"`
	ReceiverCompany string                     `json:"receiver_company"`
	Province        string                     `json:"province"`
	City            string                     `json:"city"`
	District        string                     `json:"district"`
	Notes           string                     `json:"notes"`
	Items           []FulfillmentOrderEditItem `json:"items"`
	CatalogWarning  string                     `json:"catalog_warning,omitempty"`
	Catalog         MiniDirectShipCatalog      `json:"catalog"`
	Freight         *freightdomain.OrderView   `json:"freight,omitempty"`
	Fees            FulfillmentFeeQuote        `json:"fees"`
}

type FulfillmentOrderEditPreview struct {
	OrderID         int64                      `json:"order_id"`
	OrderNo         string                     `json:"order_no"`
	RequestID       int64                      `json:"request_id"`
	EditRevision    string                     `json:"edit_revision"`
	PreviewToken    string                     `json:"preview_token"`
	GoodsTotalCents int64                      `json:"goods_total_cents"`
	Items           []FulfillmentOrderEditItem `json:"items"`
	Fees            FulfillmentFeeQuote        `json:"fees"`
	StockReady      bool                       `json:"stock_ready"`
	Shortages       []MiniDirectShipShortage   `json:"shortages,omitempty"`
}

type FulfillmentOrderEditResult struct {
	OK bool `json:"ok"`
	FulfillmentOrderEditPreview
}

type FulfillmentOrderEditRepository interface {
	FulfillmentOrderEdit(context.Context, int64) (FulfillmentOrderEditContext, error)
	PreviewFulfillmentOrderEdit(context.Context, FulfillmentOrderEditCommand) (FulfillmentOrderEditPreview, error)
	UpdateFulfillmentOrder(context.Context, FulfillmentOrderEditCommand) (FulfillmentOrderEditResult, error)
}

func (s *Service) fulfillmentOrderEditRepository() (FulfillmentOrderEditRepository, error) {
	r, ok := s.repo.(FulfillmentOrderEditRepository)
	if !ok {
		return nil, fmt.Errorf("代发订单编辑服务不可用")
	}
	return r, nil
}
func (s *Service) FulfillmentOrderEdit(ctx context.Context, id int64) (FulfillmentOrderEditContext, error) {
	r, e := s.fulfillmentOrderEditRepository()
	if e != nil {
		return FulfillmentOrderEditContext{}, e
	}
	v, e := r.FulfillmentOrderEdit(ctx, id)
	if e != nil {
		return v, e
	}
	if v.FulfillmentMode == "on_demand" {
		v.Catalog, e = s.MiniDirectShipCatalog(ctx, MiniDirectShipCatalogQuery{CustomerID: v.CustomerID, FulfillmentMode: v.FulfillmentMode, UsageCode: "direct_ship"})
		if e != nil {
			v.Catalog = MiniDirectShipCatalog{CurrentCustomerID: v.CustomerID, FulfillmentMode: v.FulfillmentMode, ProductFamilies: []map[string]any{}, PriceTables: []MiniDirectShipPriceTable{}}
			v.CatalogWarning = e.Error()
			e = nil
		}
	}
	return v, e
}
func (s *Service) PreviewFulfillmentOrderEdit(ctx context.Context, c FulfillmentOrderEditCommand) (FulfillmentOrderEditPreview, error) {
	r, e := s.fulfillmentOrderEditRepository()
	if e != nil {
		return FulfillmentOrderEditPreview{}, e
	}
	return r.PreviewFulfillmentOrderEdit(ctx, c)
}
func (s *Service) UpdateFulfillmentOrder(ctx context.Context, c FulfillmentOrderEditCommand) (FulfillmentOrderEditResult, error) {
	if strings.TrimSpace(c.ExpectedEditRevision) == "" {
		return FulfillmentOrderEditResult{}, fmt.Errorf("订单数据版本缺失，请重新打开订单")
	}
	if !c.RecipientOnly && strings.TrimSpace(c.PreviewToken) == "" {
		return FulfillmentOrderEditResult{}, fmt.Errorf("请先重新预览代发订单后保存")
	}
	r, e := s.fulfillmentOrderEditRepository()
	if e != nil {
		return FulfillmentOrderEditResult{}, e
	}
	return r.UpdateFulfillmentOrder(ctx, c)
}
