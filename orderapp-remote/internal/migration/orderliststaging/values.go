package orderliststaging

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var (
	simpleAmountPattern = regexp.MustCompile(`^[0-9０-９.,，+＋\s]+$`)
	partialDatePattern  = regexp.MustCompile(`^(\d{1,2})\s*[月/-]\s*(\d{1,2})\s*(?:日)?$`)
	fullDatePattern     = regexp.MustCompile(`^(\d{4})\s*[年/-]\s*(\d{1,2})\s*[月/-]\s*(\d{1,2})\s*(?:日)?$`)
)

func ParseAmount(raw string) AmountResult {
	raw = strings.TrimSpace(raw)
	result := AmountResult{Raw: raw}
	if raw == "" {
		return result
	}
	normalized := normalizeDigits(raw)
	normalized = strings.ReplaceAll(normalized, "，", ",")
	normalized = strings.ReplaceAll(normalized, "＋", "+")
	if !simpleAmountPattern.MatchString(raw) {
		result.NeedsReview = true
		return result
	}
	parts := strings.Split(normalized, "+")
	total := 0.0
	for _, part := range parts {
		part = strings.TrimSpace(strings.ReplaceAll(part, ",", ""))
		if part == "" {
			result.NeedsReview = true
			return result
		}
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			result.NeedsReview = true
			return result
		}
		total += v
	}
	result.Value = &total
	result.Derived = len(parts) > 1
	return result
}

func ParseDate(raw, sheetPeriod string) (string, error) {
	raw = strings.TrimSpace(normalizeDigits(raw))
	if raw == "" {
		return "", nil
	}
	if serial, err := strconv.ParseFloat(raw, 64); err == nil && serial > 1000 {
		date, err := excelize.ExcelDateToTime(serial, false)
		if err != nil {
			return "", err
		}
		return date.Format("2006-01-02"), nil
	}
	if match := fullDatePattern.FindStringSubmatch(raw); len(match) == 4 {
		return validatedDate(match[1], match[2], match[3])
	}
	if match := partialDatePattern.FindStringSubmatch(raw); len(match) == 3 {
		if len(sheetPeriod) != 7 {
			return "", fmt.Errorf("sheet period required for partial date %q", raw)
		}
		return validatedDate(sheetPeriod[:4], match[1], match[2])
	}
	for _, layout := range []string{"2006-01-02", "2006/1/2", "2006.1.2"} {
		if date, err := time.Parse(layout, raw); err == nil {
			return date.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date %q", raw)
}

func validatedDate(yearRaw, monthRaw, dayRaw string) (string, error) {
	year, _ := strconv.Atoi(yearRaw)
	month, _ := strconv.Atoi(monthRaw)
	day, _ := strconv.Atoi(dayRaw)
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return "", fmt.Errorf("invalid date %s-%s-%s", yearRaw, monthRaw, dayRaw)
	}
	return date.Format("2006-01-02"), nil
}

func normalizeDigits(raw string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + (r - '０')
		}
		return r
	}, raw)
}
