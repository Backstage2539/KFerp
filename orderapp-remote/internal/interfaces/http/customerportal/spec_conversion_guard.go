package customerportal

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"io"
	"strings"
)

// Customer endpoints do not accept employee pricing instructions, even when
// their ordinary request DTO would otherwise discard these unknown fields.
func rejectCustomerSpecConversion(c echo.Context) error {
	raw, err := io.ReadAll(io.LimitReader(c.Request().Body, 4<<20+1))
	if err != nil || len(raw) > 4<<20 {
		return echo.NewHTTPError(400, "请求数据无效")
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(raw))
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return nil
	} // the normal binder reports malformed JSON
	forbidden := func(row map[string]any) bool {
		if value, ok := row["spec_conversion"]; ok && value != nil {
			return true
		}
		switch value := row["price_selection_mode"].(type) {
		case string:
			return strings.EqualFold(strings.TrimSpace(value), "converted_tier")
		case []any:
			for _, mode := range value {
				if mode == "converted_tier" {
					return true
				}
			}
		}
		return false
	}
	if forbidden(body) {
		return echo.NewHTTPError(403, "客户自行下单不能使用员工规格换算计价")
	}
	if rows, ok := body["items"].([]any); ok {
		for _, rawRow := range rows {
			if row, ok := rawRow.(map[string]any); ok && forbidden(row) {
				return echo.NewHTTPError(403, "客户自行下单不能使用员工规格换算计价")
			}
		}
	}
	return nil
}
