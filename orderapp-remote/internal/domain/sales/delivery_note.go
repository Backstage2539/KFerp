package sales

import (
	"fmt"
	"strings"
)

type DeliveryNoteSnapshot struct {
	OrderID        int64  `json:"order_id"`
	OrderNo        string `json:"order_no"`
	DeliveryNoteNo string `json:"delivery_note_no"`
	PostingDate    string `json:"posting_date"`

	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`

	CustomerName           string `json:"customer_name"`
	CustomerCompanyName    string `json:"customer_company_name"`
	CustomerCompanyAddress string `json:"customer_company_address"`
	CustomerCompanyPhone   string `json:"customer_company_phone"`
	ReceiverName           string `json:"receiver_name"`
	ReceiverPhone          string `json:"receiver_phone"`
	ReceiverAddress        string `json:"receiver_address"`

	SourceWarehouse     string                     `json:"source_warehouse"`
	SourceWarehouseName string                     `json:"source_warehouse_name"`
	DeliveryMethod      string                     `json:"delivery_method"`
	TrackingNo          string                     `json:"tracking_no"`
	Note                string                     `json:"note"`
	Items               []DeliveryNoteSnapshotItem `json:"items"`
}

type DeliveryNoteSnapshotItem struct {
	Name          string `json:"name"`
	Spec          string `json:"spec"`
	Qty           string `json:"qty"`
	Unit          string `json:"unit"`
	Warehouse     string `json:"warehouse"`
	WarehouseName string `json:"warehouse_name"`
}

func NextDeliveryNoteVersion(existing []int) int {
	maxVersion := 0
	for _, version := range existing {
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1
}

func (s DeliveryNoteSnapshot) Validate() error {
	if s.OrderID <= 0 {
		return fmt.Errorf("order_id required")
	}
	if strings.TrimSpace(s.OrderNo) == "" {
		return fmt.Errorf("order_no required")
	}
	if strings.TrimSpace(s.DeliveryNoteNo) == "" {
		return fmt.Errorf("delivery_note_no required")
	}
	if strings.TrimSpace(s.CompanyName) == "" {
		return fmt.Errorf("company_name required")
	}
	if strings.TrimSpace(s.SourceWarehouse) == "" {
		return fmt.Errorf("source_warehouse required")
	}
	if len(s.Items) == 0 {
		return fmt.Errorf("items required")
	}
	return nil
}
