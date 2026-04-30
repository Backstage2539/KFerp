package sales

import (
	"fmt"
	"math"
	"strings"
)

type SalesOrderSnapshot struct {
	OrderID      int64                    `json:"order_id"`
	OrderNo      string                   `json:"order_no"`
	OrderDate    string                   `json:"order_date"`
	CustomerName string                   `json:"customer_name"`
	CompanyName  string                   `json:"company_name"`
	PaymentText  string                   `json:"payment_text"`
	Note         string                   `json:"note"`
	Items        []SalesOrderSnapshotItem `json:"items"`
	TotalAmount  string                   `json:"total_amount"`
	Shipping     string                   `json:"shipping"`
	Discount     string                   `json:"discount"`
	GrandTotal   string                   `json:"grand_total"`
	PaymentCodes []SalesOrderAssetRef     `json:"payment_codes"`
	Seal         *SalesOrderAssetRef      `json:"seal,omitempty"`
}

type SalesOrderSnapshotItem struct {
	Name      string `json:"name"`
	Spec      string `json:"spec"`
	Qty       string `json:"qty"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unit_price"`
	LineTotal string `json:"line_total"`
}

type SalesOrderAssetRef struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
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
