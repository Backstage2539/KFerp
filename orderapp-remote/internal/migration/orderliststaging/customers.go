package orderliststaging

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	mobilePattern     = regexp.MustCompile(`(?:\+?86[\s-]?)?(1[3-9](?:[\s-]?\d){9})`)
	labelNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:收件人|姓名|联系人)\s*[:：]\s*([^\n,，;；]{1,30})`),
	}
	customerPlaceholders = map[string]struct{}{
		"送货": {}, "送货上门": {}, "上门送货": {}, "门店自提": {}, "自提": {}, "做库存": {}, "库存": {},
	}
)

type customerObservation struct {
	row      *RawOrder
	phones   []string
	name     string
	date     string
	groupKey string
}

func NormalizePhones(raw string) []string {
	matches := mobilePattern.FindAllStringSubmatch(normalizeDigits(raw), -1)
	seen := map[string]struct{}{}
	phones := make([]string, 0, len(matches))
	for _, match := range matches {
		phone := digitsOnly(match[1])
		if len(phone) != 11 {
			continue
		}
		if _, ok := seen[phone]; ok {
			continue
		}
		seen[phone] = struct{}{}
		phones = append(phones, phone)
	}
	sort.Strings(phones)
	return phones
}

func CurateCustomers(rows []RawOrder, refs []ERPReferenceCustomer) ([]Customer, []CustomerAlias, []CustomerPhone, []Issue) {
	refByPhone := map[string][]ERPReferenceCustomer{}
	for _, ref := range refs {
		if !ref.Active {
			continue
		}
		phones := NormalizePhones(ref.Phone)
		if len(phones) == 1 {
			refByPhone[phones[0]] = append(refByPhone[phones[0]], ref)
		}
	}

	groups := map[string][]customerObservation{}
	issues := make([]Issue, 0)
	for i := range rows {
		phones := NormalizePhones(rows[i].CustomerRaw)
		name := extractCustomerName(rows[i].CustomerRaw)
		groupKey := ""
		switch len(phones) {
		case 1:
			groupKey = "phone:" + phones[0]
		case 0:
			groupKey = "source:" + shortHash(nonEmpty(rows[i].SourceOrderKey, rowLocator(rows[i])))
			rows[i].ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("customer", groupKey, "customer_phone_missing", "warning", "未识别到唯一手机号，不能自动与其他客户合并", rows[i]))
		default:
			groupKey = "source:" + shortHash(nonEmpty(rows[i].SourceOrderKey, rowLocator(rows[i])))
			rows[i].ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("customer", groupKey, "customer_phone_multiple", "error", "同一订单识别到多个手机号，需要人工确认客户", rows[i]))
		}
		rows[i].CustomerKey = groupKey
		groups[groupKey] = append(groups[groupKey], customerObservation{row: &rows[i], phones: phones, name: name, date: rows[i].OrderDate, groupKey: groupKey})
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	customers := make([]Customer, 0, len(groupKeys))
	aliases := make([]CustomerAlias, 0)
	phoneRows := make([]CustomerPhone, 0)
	for _, key := range groupKeys {
		observations := groups[key]
		sort.SliceStable(observations, func(i, j int) bool {
			if observations[i].date != observations[j].date {
				return observations[i].date > observations[j].date
			}
			return observations[i].row.SourceRowNumber < observations[j].row.SourceRowNumber
		})
		customer := Customer{CustomerKey: key, ReviewStatus: ReviewAutoReady}
		if strings.HasPrefix(key, "phone:") {
			customer.NormalizedPhone = strings.TrimPrefix(key, "phone:")
		}

		aliasSeen := map[string]struct{}{}
		for _, observation := range observations {
			if observation.name != "" {
				normalized := normalizeCustomerName(observation.name)
				if _, ok := aliasSeen[normalized]; !ok {
					aliases = append(aliases, CustomerAlias{
						CustomerKey: key, Alias: observation.name, AliasNormalized: normalized,
						SourceOrderKey: observation.row.SourceOrderKey, ObservedDate: observation.date,
					})
					aliasSeen[normalized] = struct{}{}
				}
				if customer.CanonicalName == "" {
					customer.CanonicalName = observation.name
					customer.CurrentContact = observation.name
				}
			}
			if customer.CurrentAddress == "" && strings.TrimSpace(observation.row.CustomerRaw) != "" {
				customer.CurrentAddress = strings.TrimSpace(observation.row.CustomerRaw)
			}
			for _, phone := range observation.phones {
				phoneRows = append(phoneRows, CustomerPhone{
					CustomerKey: key, PhoneRaw: phone, PhoneNormalized: phone,
					IsPrimary: phone == customer.NormalizedPhone, SourceOrderKey: observation.row.SourceOrderKey,
				})
			}
		}

		if customer.NormalizedPhone != "" {
			matches := refByPhone[customer.NormalizedPhone]
			switch len(matches) {
			case 1:
				customer.CanonicalName = strings.TrimSpace(matches[0].Name)
				customer.ERPMatchID = matches[0].ID
				customer.ERPMatchName = strings.TrimSpace(matches[0].Name)
				customer.MatchMethod = "erp_phone_exact"
			case 0:
				customer.MatchMethod = "latest_explicit_name"
			default:
				customer.MatchMethod = "erp_phone_ambiguous"
				customer.ReviewStatus = ReviewNeedsReview
				issues = append(issues, newIssue("customer", key, "erp_customer_phone_duplicate", "error", "开发ERP中同一手机号匹配到多个启用客户", *observations[0].row))
			}
		}
		if len(aliasSeen) > 1 {
			issues = append(issues, newIssue("customer", key, "customer_name_conflict", "warning", "同一客户手机号存在多个历史名称，已全部保留", *observations[0].row))
		}
		if customer.CanonicalName == "" || isCustomerPlaceholder(customer.CanonicalName) {
			customer.CanonicalName = "待确认客户-" + shortHash(key)
			customer.ReviewStatus = ReviewNeedsReview
		} else if customer.MatchMethod != "erp_phone_exact" && customerNameNeedsReview(customer.CanonicalName) {
			customer.ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("customer", key, "customer_name_needs_review", "warning", "候选客户名称更像地址、联系方式或配送说明，需要人工确认", *observations[0].row))
		}
		for _, observation := range observations {
			if observation.row.ReviewStatus == ReviewNeedsReview {
				customer.ReviewStatus = ReviewNeedsReview
			}
		}
		customers = append(customers, customer)
	}
	phoneRows = uniqueCustomerPhones(phoneRows)
	return customers, aliases, phoneRows, issues
}

func extractCustomerName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, pattern := range labelNamePatterns {
		if match := pattern.FindStringSubmatch(raw); len(match) == 2 {
			candidate := cleanNameCandidate(match[1])
			if candidate != "" && !isCustomerPlaceholder(candidate) {
				return candidate
			}
		}
	}
	withoutPhone := mobilePattern.ReplaceAllString(raw, " ")
	lines := strings.FieldsFunc(withoutPhone, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	for _, line := range lines {
		candidate := cleanNameCandidate(line)
		if candidate == "" || isCustomerPlaceholder(candidate) || looksLikeAddress(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

func cleanNameCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = regexp.MustCompile(`(?i)^(?:收件人|姓名|联系人)\s*[:：]?\s*`).ReplaceAllString(raw, "")
	if len([]rune(raw)) > 30 {
		return ""
	}
	return strings.Trim(raw, " :-_，,。.")
}

func looksLikeAddress(raw string) bool {
	markers := []string{"省", "市", "区", "县", "镇", "街道", "路", "村", "号", "栋", "室", "地址"}
	count := 0
	for _, marker := range markers {
		if strings.Contains(raw, marker) {
			count++
		}
	}
	return count >= 2
}

func customerNameNeedsReview(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if looksLikeAddress(raw) || digitsOnly(raw) != "" || len([]rune(raw)) > 20 {
		return true
	}
	for _, marker := range []string{"送到", "寄到", "送货", "班车", "前台", "地址"} {
		if strings.Contains(raw, marker) {
			return true
		}
	}
	if len([]rune(raw)) <= 8 {
		for _, suffix := range []string{"省", "市", "区", "县", "镇", "街道"} {
			if strings.HasSuffix(raw, suffix) {
				return true
			}
		}
	}
	return false
}

func normalizeCustomerName(raw string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-_，,。.:：;；()（）[]【】", r) {
			return -1
		}
		return r
	}, strings.TrimSpace(raw)))
}

func isCustomerPlaceholder(raw string) bool {
	_, ok := customerPlaceholders[normalizeCustomerName(raw)]
	return ok
}

func digitsOnly(raw string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
}

func shortHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:6])
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueCustomerPhones(rows []CustomerPhone) []CustomerPhone {
	seen := map[string]struct{}{}
	out := make([]CustomerPhone, 0, len(rows))
	for _, row := range rows {
		key := row.CustomerKey + "\x00" + row.PhoneNormalized
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out
}
