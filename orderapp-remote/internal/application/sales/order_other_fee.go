package sales

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// OtherFeeInput accepts plain yuan amounts from the ERP and employee app.
type OtherFeeInput float64

var feeDecimal = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)
var ErrInvalidOtherFee = errors.New("其他费用须为非负金额，最多两位小数")

func (f *OtherFeeInput) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}
	raw = strings.TrimSpace(raw)
	if !feeDecimal.MatchString(raw) {
		return ErrInvalidOtherFee
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return ErrInvalidOtherFee
	}
	*f = OtherFeeInput(amount)
	return nil
}

func validOtherFee(f *float64) bool {
	if f == nil {
		return true
	}
	v := *f
	return v >= 0 && v <= 9999999999.99 && v == math.Round(v*100)/100
}
