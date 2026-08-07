package customer

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxRecipientTextRunes = 4096

var (
	ErrRecipientTextRequired = errors.New("recipient text required")
	ErrRecipientTextTooLong  = errors.New("recipient text too long")

	mobileRecipientPhonePattern   = regexp.MustCompile(`(?:\+?86[-[:space:]]?)?(1[3-9][0-9]{9})`)
	landlineRecipientPhonePattern = regexp.MustCompile(`(^|[^0-9A-Za-z])([0-9]{3,4}[-[:space:]]?[0-9]{7,8})($|[^0-9A-Za-z])`)
	recipientAddressPattern       = regexp.MustCompile(`省|市|区|县|镇|街道|路|号|室|村|组`)
	recipientPersonNamePattern    = regexp.MustCompile(`^[\p{Han}A-Za-z·]{2,16}$`)
)

var (
	recipientNameLabels          = []string{"收货人", "收件人", "姓名", "联系人", "客户"}
	recipientAddressLabels       = []string{"收货地址", "地址"}
	recipientRegionLabels        = []string{"所在地区", "省市区", "地区"}
	recipientDetailAddressLabels = []string{"详细地址"}
	recipientAllLabels           = []string{"手机号码", "联系电话", "联系方式", "所在地区", "收货地址", "详细地址", "收货人", "收件人", "联系人", "省市区", "电话", "手机", "姓名", "客户", "地区", "地址"}
)

type RecipientParseResult struct {
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	Address       string `json:"address"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district"`
	DetailAddress string `json:"detail_address"`
}

// ParseRecipientText is the single server-side source of truth used by ERP and
// employee mini-program customer forms. It only transforms caller-provided text.
func ParseRecipientText(input string) (RecipientParseResult, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return RecipientParseResult{}, ErrRecipientTextRequired
	}
	if utf8.RuneCountInString(raw) > MaxRecipientTextRunes {
		return RecipientParseResult{}, ErrRecipientTextTooLong
	}

	normalized := strings.ReplaceAll(raw, "\r", "\n")
	phone := findRecipientPhone(normalized)
	phoneValue := ""
	if phone != nil {
		phoneValue = cleanRecipientLine(phone.value)
	}
	labeledName := recipientValueAfterLabel(normalized, recipientNameLabels)
	labeledAddress := recipientValueAfterLabel(normalized, recipientAddressLabels)
	labeledRegion := recipientValueAfterLabel(normalized, recipientRegionLabels)
	labeledDetailAddress := recipientValueAfterLabel(normalized, recipientDetailAddressLabels)
	if labeledRegion != "" {
		labeledAddress = cleanRecipientLine(strings.TrimSpace(labeledRegion + " " + labeledDetailAddress))
	} else if labeledAddress == "" {
		labeledAddress = labeledDetailAddress
	}

	withoutPhone := normalized
	if phone != nil {
		withoutPhone = withoutPhone[:phone.start] + " " + withoutPhone[phone.end:]
	}
	compact := make([]string, 0)
	for _, line := range strings.Split(withoutPhone, "\n") {
		line = cleanRecipientLine(stripRecipientLabel(line))
		if line != "" {
			compact = append(compact, line)
		}
	}

	recipientName := labeledName
	beforePhone := ""
	afterPhone := ""
	if phone != nil {
		beforePhone = cleanRecipientLine(normalized[:phone.start])
		afterPhone = cleanRecipientLine(normalized[phone.end:])
	}
	fromAddressBlock := extractRecipientNameFromAddressBlock(beforePhone)
	if recipientName == "" && phone != nil {
		switch {
		case recipientAddressPattern.MatchString(beforePhone) && fromAddressBlock.name != "":
			recipientName = fromAddressBlock.name
		case recipientAddressPattern.MatchString(beforePhone) && afterPhone != "" && !recipientAddressPattern.MatchString(afterPhone):
			recipientName = firstRecipientToken(afterPhone)
		default:
			recipientName = firstRecipientToken(stripRecipientNameLabel(beforePhone))
		}
	}
	if recipientName == "" {
		for _, line := range compact {
			if !recipientAddressPattern.MatchString(line) && !strings.Contains(line, "地址") {
				recipientName = firstRecipientToken(line)
				break
			}
		}
	}

	address := labeledAddress
	if address == "" && phone != nil && recipientAddressPattern.MatchString(beforePhone) && fromAddressBlock.address != "" {
		address = fromAddressBlock.address
	}
	if address == "" && phone != nil && recipientAddressPattern.MatchString(beforePhone) && afterPhone != "" && recipientName == firstRecipientToken(afterPhone) {
		address = beforePhone
	}
	if address == "" {
		for _, line := range compact {
			if recipientAddressPattern.MatchString(line) {
				address = line
				break
			}
		}
		if address == "" {
			parts := make([]string, 0, len(compact))
			for _, line := range compact {
				if line != recipientName {
					parts = append(parts, line)
				}
			}
			address = strings.Join(parts, " ")
		}
	}
	if recipientName != "" {
		trailingName := regexp.MustCompile(`[[:space:]]*` + regexp.QuoteMeta(recipientName) + `[[:space:]]*(?:收件人|收件|收货|收)?[[:space:]]*$`)
		address = cleanRecipientLine(trailingName.ReplaceAllString(address, " "))
	}
	if recipientName != "" && strings.HasPrefix(address, recipientName) {
		address = cleanRecipientLine(strings.TrimPrefix(address, recipientName))
	}

	address = cleanRecipientLine(address)
	province, city, district, detailAddress := splitRecipientAddress(address)
	return RecipientParseResult{
		RecipientName: recipientName,
		Phone:         phoneValue,
		Address:       address,
		Province:      province,
		City:          city,
		District:      district,
		DetailAddress: detailAddress,
	}, nil
}

var directMunicipalities = []string{"北京市", "上海市", "天津市", "重庆市"}

var autonomousRegions = []string{
	"内蒙古自治区", "广西壮族自治区", "西藏自治区", "宁夏回族自治区", "新疆维吾尔自治区",
	"香港特别行政区", "澳门特别行政区",
}

var (
	recipientProvincePattern = regexp.MustCompile(`^(.{2,}?省)`)
	recipientCityPattern     = regexp.MustCompile(`^(.{2,}?(?:自治州|地区|盟|市))`)
	recipientDistrictPattern = regexp.MustCompile(`^(.{1,}?(?:自治县|新区|林区|特区|区|县|旗|市))`)
)

func splitRecipientAddress(address string) (province, city, district, detail string) {
	rest := strings.TrimSpace(address)
	if rest == "" {
		return "", "", "", ""
	}
	for _, municipality := range directMunicipalities {
		if strings.HasPrefix(rest, municipality) {
			province, city = municipality, municipality
			rest = strings.TrimSpace(strings.TrimPrefix(rest, municipality))
			district, rest = takeRecipientAddressPart(rest, recipientDistrictPattern)
			return province, city, district, strings.TrimSpace(rest)
		}
	}
	for _, region := range autonomousRegions {
		if strings.HasPrefix(rest, region) {
			province = region
			rest = strings.TrimSpace(strings.TrimPrefix(rest, region))
			break
		}
	}
	if province == "" {
		province, rest = takeRecipientAddressPart(rest, recipientProvincePattern)
	}
	city, rest = takeRecipientAddressPart(rest, recipientCityPattern)
	district, rest = takeRecipientAddressPart(rest, recipientDistrictPattern)
	return province, city, district, strings.TrimSpace(rest)
}

func takeRecipientAddressPart(text string, pattern *regexp.Regexp) (string, string) {
	match := pattern.FindString(text)
	if match == "" {
		return "", text
	}
	return strings.TrimSpace(match), strings.TrimSpace(strings.TrimPrefix(text, match))
}

type recipientPhoneMatch struct {
	value      string
	start, end int
}

func findRecipientPhone(text string) *recipientPhoneMatch {
	if indexes := mobileRecipientPhonePattern.FindStringSubmatchIndex(text); len(indexes) >= 4 {
		return &recipientPhoneMatch{value: text[indexes[2]:indexes[3]], start: indexes[0], end: indexes[1]}
	}
	indexes := landlineRecipientPhonePattern.FindStringSubmatchIndex(text)
	if len(indexes) < 6 || indexes[4] < 0 || indexes[5] < 0 {
		return nil
	}
	return &recipientPhoneMatch{value: text[indexes[4]:indexes[5]], start: indexes[4], end: indexes[5]}
}

func cleanRecipientLine(line string) string {
	line = strings.NewReplacer("，", " ", ",", " ", "；", " ", ";", " ").Replace(line)
	return strings.Join(strings.Fields(line), " ")
}

func recipientValueAfterLabel(text string, labels []string) string {
	for _, label := range labels {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimLeftFunc(line, unicode.IsSpace)
			if !strings.HasPrefix(line, label) {
				continue
			}
			rest := strings.TrimPrefix(line, label)
			if rest == "" {
				continue
			}
			first, _ := utf8.DecodeRuneInString(rest)
			if first != '：' && first != ':' && !unicode.IsSpace(first) {
				continue
			}
			rest = strings.TrimLeftFunc(rest, func(r rune) bool { return r == '：' || r == ':' || unicode.IsSpace(r) })
			if value := cleanRecipientLine(rest); value != "" {
				return value
			}
		}
	}
	return ""
}

func stripRecipientLabel(text string) string {
	text = strings.TrimSpace(text)
	for _, label := range recipientAllLabels {
		if strings.HasPrefix(text, label) {
			return strings.TrimLeftFunc(strings.TrimPrefix(text, label), func(r rune) bool { return r == '：' || r == ':' || unicode.IsSpace(r) })
		}
	}
	return text
}

func stripRecipientNameLabel(text string) string {
	text = strings.TrimSpace(text)
	for _, label := range recipientNameLabels {
		if strings.HasPrefix(text, label) {
			return strings.TrimLeftFunc(strings.TrimPrefix(text, label), func(r rune) bool { return r == '：' || r == ':' || unicode.IsSpace(r) })
		}
	}
	return text
}

func firstRecipientToken(text string) string {
	parts := strings.Fields(cleanRecipientLine(text))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

type recipientAddressBlock struct {
	name, address string
}

func extractRecipientNameFromAddressBlock(text string) recipientAddressBlock {
	tokens := strings.Fields(cleanRecipientLine(text))
	if len(tokens) < 2 {
		return recipientAddressBlock{}
	}
	last := tokens[len(tokens)-1]
	penultimate := tokens[len(tokens)-2]
	if isRecipientMarker(last) && likelyRecipientPersonName(penultimate) {
		return recipientAddressBlock{name: penultimate, address: strings.Join(tokens[:len(tokens)-2], " ")}
	}
	if likelyRecipientPersonName(last) && recipientAddressPattern.MatchString(strings.Join(tokens[:len(tokens)-1], " ")) {
		return recipientAddressBlock{name: last, address: strings.Join(tokens[:len(tokens)-1], " ")}
	}
	return recipientAddressBlock{}
}

func likelyRecipientPersonName(text string) bool {
	return recipientPersonNamePattern.MatchString(text)
}

func isRecipientMarker(text string) bool {
	switch text {
	case "收", "收件", "收件人", "收货":
		return true
	default:
		return false
	}
}
