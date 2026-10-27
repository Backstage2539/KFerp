// Package shippingreturn defines configurable courier return files and recipient matching.
package shippingreturn

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Column struct {
	Column string `json:"column"`
	Header string `json:"header"`
}
type Template struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Sheet      string            `json:"sheet"`
	HeaderRow  int               `json:"header_row"`
	Active     bool              `json:"active"`
	CarrierID  int64             `json:"carrier_id"`
	Revision   int64             `json:"revision"`
	WeightUnit string            `json:"weight_unit,omitempty"`
	Columns    map[string]Column `json:"columns"`
}
type Row struct {
	ID                 int64  `json:"id"`
	RowNo              int    `json:"row_no"`
	TrackingNo         string `json:"tracking_no"`
	ActualCents        int64  `json:"actual_cents"`
	Receiver           string `json:"receiver"`
	Phone              string `json:"phone"`
	Sender             string `json:"sender"`
	SenderPhone        string `json:"sender_phone"`
	Time               string `json:"time"`
	SettlementWeightKG string `json:"settlement_weight_kg,omitempty"`
	PickupTime         string `json:"pickup_time,omitempty"`
	Error              string `json:"error,omitempty"`
	Status             string `json:"status"`
	OrderID            int64  `json:"order_id,omitempty"`
	Version            string `json:"version,omitempty"`
}
type Batch struct {
	Rematched bool    `json:"rematched,omitempty"`
	ID        int64   `json:"batch_id"`
	Updated   int     `json:"updated"`
	Pending   int     `json:"pending"`
	Skipped   int     `json:"skipped"`
	Total     int     `json:"total"`
	Rows      []Row   `json:"rows"`
	OrderIDs  []int64 `json:"order_ids"`
}

func DefaultTemplate() Template {
	return Template{ID: "sf-recipient", Name: "顺丰寄件列表", Sheet: "寄件列表", HeaderRow: 1, Active: true, Revision: 1, WeightUnit: "kg", Columns: map[string]Column{"tracking": {"C", "运单号"}, "amount": {"S", "应收金额"}, "receiver": {"J", "收件人"}, "phone": {"K", "收件人手机"}, "sender": {"F", "寄付人"}, "sender_phone": {"G", "寄件人手机"}, "time": {"AF", "下单时间"}, "settlement_weight": {"P", "结算重量（kg）"}, "pickup_time": {"AG", "揽件时间"}}}
}
func (t Template) Validate() error {
	if t.ID == "" || strings.TrimSpace(t.Name) == "" || t.Sheet == "" || t.HeaderRow < 1 || t.HeaderRow > 100 || t.CarrierID < 0 {
		return fmt.Errorf("请完整配置模板名称、工作表和标题行")
	}
	for _, k := range []string{"tracking", "amount", "receiver", "phone", "sender", "sender_phone", "time"} {
		v := t.Columns[k]
		n, e := excelize.ColumnNameToNumber(v.Column)
		if e != nil || n > 16384 || v.Header == "" {
			return fmt.Errorf("列映射无效：%s", k)
		}
	}
	return nil
}

// PhoneMatchKey validates a mainland mobile and returns the visible 3 + 4 digits.
func PhoneMatchKey(s string) string {
	s = strings.NewReplacer(" ", "", "-", "", "+86", "", "（", "", "）", "").Replace(strings.TrimSpace(s))
	if !regexp.MustCompile(`^1[0-9]{2}([0-9]{4}|\*{4})[0-9]{4}$`).MatchString(s) {
		return ""
	}
	return s[:3] + s[7:]
}
func RecipientMatches(name, phone, orderName, orderPhone string) bool {
	a, b := PhoneMatchKey(phone), PhoneMatchKey(orderPhone)
	if a == "" || a != b {
		return false
	}
	name = strings.TrimSpace(name)
	orderName = strings.TrimSpace(orderName)
	if strings.Trim(name, "* ") == "" || strings.Contains(orderName, "*") {
		return false
	}
	name = regexp.MustCompile(`\*+`).ReplaceAllString(name, "*")
	pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(name), `\*`, `.+`) + "$"
	return regexp.MustCompile(pattern).MatchString(orderName)
}
func money(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`).MatchString(s) {
		return 0, fmt.Errorf("运费格式错误")
	}
	parts := strings.SplitN(s, ".", 2)
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole > 9999999999 {
		return 0, fmt.Errorf("运费超出范围")
	}
	frac := ""
	if len(parts) > 1 {
		frac = parts[1]
	}
	if len(frac) > 2 && strings.Trim(frac[2:], "0") != "" {
		return 0, fmt.Errorf("运费最多两位有效小数")
	}
	frac = (frac + "00")[:2]
	c, _ := strconv.ParseInt(frac, 10, 64)
	return whole*100 + c, nil
}
func Parse(reader io.Reader, t Template) ([]Row, error) {
	if e := t.Validate(); e != nil {
		return nil, e
	}
	f, e := excelize.OpenReader(reader, excelize.Options{UnzipSizeLimit: 80 << 20})
	if e != nil {
		return nil, fmt.Errorf("请上传有效的 xlsx 文件")
	}
	defer f.Close()
	resolved := map[string]Column{}
	optional := map[string]bool{"settlement_weight": true, "pickup_time": true}
	for key, c := range t.Columns {
		v, err := f.GetCellValue(t.Sheet, fmt.Sprintf("%s%d", c.Column, t.HeaderRow))
		if err == nil && strings.TrimSpace(v) == c.Header {
			resolved[key] = c
			continue
		}
		if optional[key] {
			if c.Header != "" {
				cols, _ := f.GetCols(t.Sheet)
				for n, values := range cols {
					if t.HeaderRow-1 < len(values) && strings.TrimSpace(values[t.HeaderRow-1]) == c.Header {
						name, _ := excelize.ColumnNumberToName(n + 1)
						c.Column = name
						resolved[key] = c
						break
					}
				}
			}
			continue
		}
		return nil, fmt.Errorf("模板表头不符：%s 列应为 %s", c.Column, c.Header)
	}
	iterator, e := f.Rows(t.Sheet)
	if e != nil {
		return nil, e
	}
	defer iterator.Close()
	out := []Row{}
	i := 0
	for iterator.Next() {
		i++
		if i <= t.HeaderRow {
			continue
		}
		if i-t.HeaderRow > 10000 {
			return nil, fmt.Errorf("最多上传 10000 条寄件记录")
		}
		values, e := iterator.Columns()
		if e != nil {
			return nil, e
		}
		get := func(k string) string {
			c, exists := resolved[k]
			if !exists {
				return ""
			}
			n, _ := excelize.ColumnNameToNumber(c.Column)
			if n > len(values) {
				return ""
			}
			return strings.TrimSpace(values[n-1])
		}
		r := Row{RowNo: i, TrackingNo: get("tracking"), Receiver: get("receiver"), Phone: get("phone"), Sender: get("sender"), SenderPhone: get("sender_phone"), Time: get("time"), Status: "pending"}
		if strings.Join(values, "") == "" {
			continue
		}
		if r.TrackingNo == "" || get("amount") == "" {
			r.Error = "单号或运费为空"
		} else {
			r.ActualCents, e = money(get("amount"))
			if e != nil {
				r.Error = e.Error()
			}
		}
		if weight := get("settlement_weight"); weight != "" {
			unit := strings.ToLower(strings.TrimSpace(t.WeightUnit))
			if unit == "" {
				unit = "kg"
			}
			r.SettlementWeightKG, e = normalizeWeightKG(weight, unit)
			if e != nil {
				r.Error = e.Error()
			}
		}
		if pickup := get("pickup_time"); pickup != "" {
			r.PickupTime, e = normalizePickupTime(pickup)
			if e != nil {
				r.Error = e.Error()
			}
		}
		trackingCell := fmt.Sprintf("%s%d", t.Columns["tracking"].Column, i)
		trackingType, err := f.GetCellType(t.Sheet, trackingCell)
		if err != nil {
			return nil, err
		}
		if len(r.TrackingNo) > 100 || !regexp.MustCompile(`^[A-Za-z0-9-]+$`).MatchString(r.TrackingNo) || regexp.MustCompile(`(?i)^[0-9.]+e[+-]?[0-9]+$`).MatchString(r.TrackingNo) {
			r.Error = "运单号必须为单个完整单号，不支持分隔符或科学计数法"
		} else if trackingType != excelize.CellTypeSharedString && trackingType != excelize.CellTypeInlineString && len(r.TrackingNo) > 15 {
			r.Error = "长运单号必须使用文本，避免 Excel 数值精度丢失"
		}
		for _, k := range []string{"tracking", "amount", "receiver", "phone", "settlement_weight", "pickup_time"} {
			if _, exists := resolved[k]; !exists {
				continue
			}
			formula, err := f.GetCellFormula(t.Sheet, fmt.Sprintf("%s%d", t.Columns[k].Column, i))
			if err != nil {
				return nil, err
			}
			if formula != "" {
				r.Error = "匹配与金额列不能使用公式"
			}
		}
		if PhoneMatchKey(r.Phone) == "" || strings.Trim(r.Receiver, "* ") == "" {
			r.Error = "收件姓名或手机号无法识别"
		}
		if r.Error != "" {
			r.Status = "skipped"
		}
		out = append(out, r)
	}
	return out, iterator.Error()
}

func normalizeWeightKG(value, unit string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if !regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,6})?$`).MatchString(value) {
		return "", fmt.Errorf("结算重量格式错误")
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(n, 0) || math.IsNaN(n) || n < 0 {
		return "", fmt.Errorf("结算重量超出范围")
	}
	switch unit {
	case "kg", "公斤":
	case "g", "克":
		n /= 1000
	default:
		return "", fmt.Errorf("结算重量单位无效")
	}
	if math.IsInf(n, 0) || math.IsNaN(n) || n > 1000 {
		return "", fmt.Errorf("结算重量超出范围")
	}
	return strconv.FormatFloat(n, 'f', 3, 64), nil
}

func normalizePickupTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02 15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.FixedZone("Asia/Shanghai", 8*3600)); err == nil {
			return parsed.Format("2006-01-02 15:04:05"), nil
		}
	}
	return "", fmt.Errorf("揽件时间格式错误")
}
