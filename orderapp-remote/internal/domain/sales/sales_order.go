package sales

import (
	"fmt"
	"math"
	"strings"
)

type SalesOrderSnapshot struct {
	OrderID        int64  `json:"order_id"`
	OrderNo        string `json:"order_no"`
	OrderDate      string `json:"order_date"`
	CustomerName   string `json:"customer_name"`
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`

	CustomerCompanyName    string `json:"customer_company_name"`
	CustomerCompanyAddress string `json:"customer_company_address"`
	CustomerCompanyPhone   string `json:"customer_company_phone"`

	PaymentText        string                        `json:"payment_text"`
	TaxpayerID         string                        `json:"taxpayer_id"`
	BankAccountName    string                        `json:"bank_account_name"`
	BankName           string                        `json:"bank_name"`
	BankAccountNo      string                        `json:"bank_account_no"`
	Note               string                        `json:"note"`
	Items              []SalesOrderSnapshotItem      `json:"items"`
	TotalAmount        string                        `json:"total_amount"`
	Shipping           string                        `json:"shipping"`
	Discount           string                        `json:"discount"`
	ExpressFee         string                        `json:"express_fee"`
	SalesOrderNote     string                        `json:"sales_order_note"`
	GrandTotal         string                        `json:"grand_total"`
	DiscountBreakdowns []SalesOrderDiscountBreakdown `json:"discount_breakdowns"`
	PaymentCodes       []SalesOrderAssetRef          `json:"payment_codes"`
	PaymentTextBox     SalesOrderLayoutBox           `json:"payment_text_box"`
	PaymentCodeBox     SalesOrderLayoutBox           `json:"payment_code_box"`
	Seal               *SalesOrderAssetRef           `json:"seal,omitempty"`
}

type SalesOrderDiscountBreakdown struct {
	Type   string `json:"type"`
	Amount string `json:"amount"`
}

type SalesOrderSnapshotItem struct {
	Name           string `json:"name"`
	Note           string `json:"note"`
	Spec           string `json:"spec"`
	Qty            string `json:"qty"`
	Unit           string `json:"unit"`
	UnitPrice      string `json:"unit_price"`
	DiscountAmount string `json:"discount_amount"`
	LineTotal      string `json:"line_total"`
}

type SalesOrderAssetRef struct {
	ID          int64   `json:"id"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	ObjectKey   string  `json:"object_key"`
	ContentType string  `json:"content_type"`
	URL         string  `json:"url"`
	XMM         float64 `json:"x_mm,omitempty"`
	YMM         float64 `json:"y_mm,omitempty"`
	WidthMM     float64 `json:"width_mm,omitempty"`
}

type SalesOrderLayoutBox struct {
	XMM      float64 `json:"x_mm"`
	YMM      float64 `json:"y_mm"`
	WidthMM  float64 `json:"width_mm"`
	HeightMM float64 `json:"height_mm"`
}

func NextSalesOrderVersion(existing []int) int {
	maxVersion := 0
	for _, version := range existing {
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1
}

func FormatSalesOrderMoney(v float64) string {
	return fmt.Sprintf("%.2f", math.Round(v*100)/100)
}

func (s SalesOrderSnapshot) Validate() error {
	if s.OrderID <= 0 {
		return fmt.Errorf("order_id required")
	}
	if strings.TrimSpace(s.OrderNo) == "" {
		return fmt.Errorf("order_no required")
	}
	if strings.TrimSpace(s.CompanyName) == "" {
		return fmt.Errorf("company_name required")
	}
	if len(s.Items) == 0 {
		return fmt.Errorf("items required")
	}
	if strings.TrimSpace(s.GrandTotal) == "" {
		return fmt.Errorf("grand_total required")
	}
	return nil
}
