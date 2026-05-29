package sales

import (
	"fmt"
	"strings"
)

type CombinedSalesOrderSnapshot struct {
	CombinationKey         string                    `json:"combination_key"`
	CombinedNo             string                    `json:"combined_no"`
	CustomerID             int64                     `json:"customer_id"`
	CustomerName           string                    `json:"customer_name"`
	CompanyName            string                    `json:"company_name"`
	CompanyAddress         string                    `json:"company_address"`
	CustomerCompanyName    string                    `json:"customer_company_name"`
	CustomerCompanyAddress string                    `json:"customer_company_address"`
	CustomerCompanyPhone   string                    `json:"customer_company_phone"`
	OrderIDs               []int64                   `json:"order_ids"`
	OrderNos               []string                  `json:"order_nos"`
	Groups                 []CombinedSalesOrderGroup `json:"groups"`
	TotalAmount            string                    `json:"total_amount"`
	Shipping               string                    `json:"shipping"`
	Discount               string                    `json:"discount"`
	GrandTotal             string                    `json:"grand_total"`
	PaymentText            string                    `json:"payment_text"`
	TaxpayerID             string                    `json:"taxpayer_id"`
	BankAccountName        string                    `json:"bank_account_name"`
	BankName               string                    `json:"bank_name"`
	BankAccountNo          string                    `json:"bank_account_no"`
	Note                   string                    `json:"note"`
	PaymentCodes           []SalesOrderAssetRef      `json:"payment_codes"`
	PaymentTextBox         SalesOrderLayoutBox       `json:"payment_text_box"`
	PaymentCodeBox         SalesOrderLayoutBox       `json:"payment_code_box"`
	Seal                   *SalesOrderAssetRef       `json:"seal,omitempty"`
}

type CombinedSalesOrderGroup struct {
	OrderID            int64                         `json:"order_id"`
	OrderNo            string                        `json:"order_no"`
	DocumentDate       string                        `json:"document_date"`
	OrderDate          string                        `json:"order_date"`
	Items              []SalesOrderSnapshotItem      `json:"items"`
	TotalAmount        string                        `json:"total_amount"`
	Shipping           string                        `json:"shipping"`
	Discount           string                        `json:"discount"`
	ExpressFee         string                        `json:"express_fee"`
	SalesOrderNote     string                        `json:"sales_order_note"`
	GrandTotal         string                        `json:"grand_total"`
	DiscountBreakdowns []SalesOrderDiscountBreakdown `json:"discount_breakdowns"`
}

type CombinedDeliveryNoteSnapshot struct {
	CombinationKey         string                      `json:"combination_key"`
	DeliveryNoteNo         string                      `json:"delivery_note_no"`
	CustomerID             int64                       `json:"customer_id"`
	CustomerName           string                      `json:"customer_name"`
	CompanyName            string                      `json:"company_name"`
	CompanyAddress         string                      `json:"company_address"`
	CustomerCompanyName    string                      `json:"customer_company_name"`
	CustomerCompanyAddress string                      `json:"customer_company_address"`
	CustomerCompanyPhone   string                      `json:"customer_company_phone"`
	OrderIDs               []int64                     `json:"order_ids"`
	OrderNos               []string                    `json:"order_nos"`
	Groups                 []CombinedDeliveryNoteGroup `json:"groups"`
	Seal                   *SalesOrderAssetRef         `json:"seal,omitempty"`
}

type CombinedDeliveryNoteGroup struct {
	OrderID             int64                      `json:"order_id"`
	OrderNo             string                     `json:"order_no"`
	DocumentDate        string                     `json:"document_date"`
	OrderDate           string                     `json:"order_date"`
	PostingDate         string                     `json:"posting_date"`
	ReceiverName        string                     `json:"receiver_name"`
	ReceiverPhone       string                     `json:"receiver_phone"`
	ReceiverAddress     string                     `json:"receiver_address"`
	SourceWarehouse     string                     `json:"source_warehouse"`
	SourceWarehouseName string                     `json:"source_warehouse_name"`
	DeliveryMethod      string                     `json:"delivery_method"`
	TrackingNo          string                     `json:"tracking_no"`
	Note                string                     `json:"note"`
	Items               []DeliveryNoteSnapshotItem `json:"items"`
}

func NextCombinedDocumentVersion(existing []int) int {
	maxVersion := 0
	for _, version := range existing {
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1
}

func (s CombinedSalesOrderSnapshot) Validate() error {
	if strings.TrimSpace(s.CombinationKey) == "" {
		return fmt.Errorf("combination_key required")
	}
	if strings.TrimSpace(s.CombinedNo) == "" {
		return fmt.Errorf("combined_no required")
	}
	if s.CustomerID <= 0 {
		return fmt.Errorf("customer_id required")
	}
	if strings.TrimSpace(s.CustomerName) == "" {
		return fmt.Errorf("customer_name required")
	}
	if strings.TrimSpace(s.CompanyName) == "" {
		return fmt.Errorf("company_name required")
	}
	if len(s.OrderIDs) < 2 || len(s.Groups) < 2 {
		return fmt.Errorf("at least two orders required")
	}
	if strings.TrimSpace(s.GrandTotal) == "" {
		return fmt.Errorf("grand_total required")
	}
	for _, group := range s.Groups {
		if group.OrderID <= 0 || strings.TrimSpace(group.OrderNo) == "" {
			return fmt.Errorf("group order required")
		}
		if len(group.Items) == 0 {
			return fmt.Errorf("group items required")
		}
	}
	return nil
}

func (s CombinedDeliveryNoteSnapshot) Validate() error {
	if strings.TrimSpace(s.CombinationKey) == "" {
		return fmt.Errorf("combination_key required")
	}
	if strings.TrimSpace(s.DeliveryNoteNo) == "" {
		return fmt.Errorf("delivery_note_no required")
	}
	if s.CustomerID <= 0 {
		return fmt.Errorf("customer_id required")
	}
	if strings.TrimSpace(s.CustomerName) == "" {
		return fmt.Errorf("customer_name required")
	}
	if strings.TrimSpace(s.CompanyName) == "" {
		return fmt.Errorf("company_name required")
	}
	if len(s.OrderIDs) < 2 || len(s.Groups) < 2 {
		return fmt.Errorf("at least two orders required")
	}
	for _, group := range s.Groups {
		if group.OrderID <= 0 || strings.TrimSpace(group.OrderNo) == "" {
			return fmt.Errorf("group order required")
		}
		if strings.TrimSpace(group.SourceWarehouse) == "" {
			return fmt.Errorf("source_warehouse required")
		}
		if len(group.Items) == 0 {
			return fmt.Errorf("group items required")
		}
	}
	return nil
}
