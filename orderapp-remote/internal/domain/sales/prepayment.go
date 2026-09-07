package sales

import (
	"fmt"
	"math"
	"strings"
)

const PrepaymentStatusName = "预付款（付款未完成）"

func IsFullyPaid(status string) bool {
	return strings.Contains(status, "已付款") || strings.Contains(status, "已收款") || strings.Contains(status, "已支付")
}
func ValidatePrepayment(status string, amount, total float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || math.Abs(amount*100-math.Round(amount*100)) > 0.00001 {
		return fmt.Errorf("预付款金额必须为非负金额，最多两位小数")
	}
	if amount > total && amount > 0 {
		return fmt.Errorf("预付款不能超过订单应收总额")
	}
	if strings.Contains(status, "预付款") {
		if amount <= 0 || amount >= total {
			return fmt.Errorf("预付款必须大于零且小于订单应收总额；收齐款项请选择已付款")
		}
	} else if amount > 0 && !IsFullyPaid(status) {
		return fmt.Errorf("已登记预付款，请选择预付款或已付款状态")
	}
	return nil
}
func OrderPaymentAmounts(status string, prepayment, total float64) (string, string) {
	paid := prepayment
	if IsFullyPaid(status) {
		paid = total
	}
	return FormatSalesOrderMoney(paid), FormatSalesOrderMoney(math.Max(0, total-paid))
}
