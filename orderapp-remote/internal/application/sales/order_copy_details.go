package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// OrderCopySnapshot reads the current saved order and the same recipient defaults
// as order detail. Product names and specifications remain frozen order data.
type OrderCopySnapshot struct {
	Order            OrderEditData
	CustomerName     string
	PaymentMethod    string
	PayStatus        string
	DefaultRecipient OrderCopyRecipient
}
type OrderCopyRecipient struct {
	Name, Phone, Address string
}
type OrderDetailsText struct {
	OrderID    int64  `json:"order_id"`
	Text       string `json:"text"`
	CustomerID int64  `json:"-"`
}
type OrderDetailsTextReader interface {
	CopyOrderDetails(context.Context, int64) (OrderDetailsText, error)
}
type orderCopySnapshotReader interface {
	LoadOrderCopySnapshot(context.Context, int64) (*OrderCopySnapshot, error)
}

var ErrOrderCopyNotFound = errors.New("order not found")

func (s *Service) CopyOrderDetails(ctx context.Context, id int64) (OrderDetailsText, error) {
	if id <= 0 {
		return OrderDetailsText{}, ErrOrderCopyNotFound
	}
	reader, ok := s.repo.(orderCopySnapshotReader)
	if !ok {
		return OrderDetailsText{}, errors.New("order copy service unavailable")
	}
	snapshot, err := reader.LoadOrderCopySnapshot(ctx, id)
	if err != nil {
		return OrderDetailsText{}, err
	}
	if snapshot == nil || snapshot.Order.ID != id {
		return OrderDetailsText{}, ErrOrderCopyNotFound
	}
	return OrderDetailsText{OrderID: id, CustomerID: snapshot.Order.CustomerID, Text: BuildOrderDetailsText(*snapshot)}, nil
}

var copyNumber = regexp.MustCompile(`^\d+(\.\d+)?$`)

func copyValue(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "未填写"
	}
	return value
}
func copyFirst(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func copyQuantity(q string) string {
	q = strings.TrimSpace(q)
	if copyNumber.MatchString(q) && strings.Contains(q, ".") {
		q = strings.TrimRight(strings.TrimRight(q, "0"), ".")
	}
	return copyValue(q)
}
func copySpec(item OrderEditItem) string {
	var frozen struct {
		Conversion struct {
			TargetSpecName string `json:"target_spec_name"`
		} `json:"spec_conversion"`
		BomSpecName string `json:"bom_spec_name"`
	}
	_ = json.Unmarshal([]byte(item.PriceSourceJSON), &frozen)
	if spec := copyFirst(frozen.Conversion.TargetSpecName, item.BomSpecName, frozen.BomSpecName); spec != "" {
		return copyValue(spec)
	}
	spec := strings.TrimSpace(item.Spec)
	if spec != "" && !copyNumber.MatchString(spec) {
		return copyValue(spec)
	}
	if item.ProductKind == "drip_bag" {
		if item.UnitConversionLabel != "" {
			return copyValue(item.UnitConversionLabel)
		}
		if item.SalesUnit == "box" && item.UnitBagCount > 0 {
			return fmt.Sprintf("%d袋/盒", item.UnitBagCount)
		}
		if grams := strings.TrimSpace(item.UnitBeanG); grams != "" && grams != "0" {
			return copyQuantity(grams) + "g/袋"
		}
	}
	if spec != "" {
		return copyQuantity(spec) + "g"
	}
	return "未填写"
}
func BuildOrderDetailsText(snapshot OrderCopySnapshot) string {
	o := snapshot.Order
	lines := []string{}
	if o.IsVoid {
		lines = append(lines, "【已作废订单】")
	}
	lines = append(lines,
		"订单号："+copyValue(o.OrderNo),
		"客户名称："+copyValue(snapshot.CustomerName),
		"订单日期："+copyValue(o.OrderDate),
		"付款状态："+copyValue(snapshot.PayStatus),
		"付款方式："+copyValue(snapshot.PaymentMethod),
		"",
		"订单详情：",
	)
	for i, item := range o.Items {
		name := copyValue(copyFirst(item.CustomerProductDisplayNameSnapshot, item.ProductNameSnapshot, item.Product))
		lines = append(lines, fmt.Sprintf("%d. %s｜%s｜%s %s", i+1, name, copySpec(item), copyQuantity(item.Qty), copyValue(item.Unit)))
	}
	if len(o.Items) == 0 {
		lines = append(lines, "未填写")
	}
	recipient := OrderCopyRecipient{Name: o.ReceiverName, Phone: o.ReceiverPhone, Address: o.ReceiverAddress}
	switch o.PortalServiceCode {
	case "direct_ship", "product_order", "processing_ship":
		// These orders must retain their explicitly saved destination.
	default:
		recipient.Name = copyFirst(recipient.Name, snapshot.DefaultRecipient.Name)
		recipient.Phone = copyFirst(recipient.Phone, snapshot.DefaultRecipient.Phone)
		recipient.Address = copyFirst(recipient.Address, snapshot.DefaultRecipient.Address)
	}
	lines = append(lines, "", "收件人："+copyValue(recipient.Name), "电话："+copyValue(recipient.Phone), "收件地址："+copyValue(recipient.Address))
	return strings.Join(lines, "\n")
}
