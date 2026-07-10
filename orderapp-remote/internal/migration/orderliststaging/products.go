package orderliststaging

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	weightPackPattern     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(kg|g|lb)\s*(?:装|/)\s*(\d+(?:\.\d+)?)\s*(袋|包|盒|箱|条|个)`)
	weightTimesPattern    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(kg|g|lb)\s*\*\s*(\d+(?:\.\d+)?)\s*(袋|包|盒|箱|条|个)?`)
	combinedWeightPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(kg|g|lb)\s*\+\s*(\d+(?:\.\d+)?)\s*(kg|g|lb)`)
	singleWeightPattern   = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(kg|g|lb)`)
	packageQtyPattern     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(袋|包|盒|箱|条|个)`)
	roastPattern          = regexp.MustCompile(`(极浅烘|浅烘|中浅烘|中烘|中深烘|深烘)`)
)

func ParseProductLine(raw string) ProductLine {
	line := normalizeProductText(raw)
	result := ProductLine{RawLine: strings.TrimSpace(raw), NormalizedLine: line, ProductKind: detectProductKind(line)}
	if line == "" {
		result.NeedsReview = true
		result.ReviewReason = "商品描述为空"
		return result
	}
	if match := roastPattern.FindStringSubmatch(line); len(match) == 2 {
		result.RoastLevel = match[1]
	}

	quantitySegment := ""
	if match := weightPackPattern.FindStringSubmatch(line); len(match) == 5 {
		netQty := mustFloat(match[1])
		netUnit := canonicalWeightUnit(match[2])
		count := mustFloat(match[3])
		packUnit := match[4]
		result.NetContentQty = netQty
		result.NetContentUnit = netUnit
		result.OrderQuantity = count
		result.OrderUnit = packUnit
		result.NormalizedWeightG = weightToGrams(netQty, netUnit) * count
		result.SpecName = formatNumber(netQty) + netUnit + "包装"
		quantitySegment = match[0]
	} else if match := weightTimesPattern.FindStringSubmatch(line); len(match) == 5 {
		netQty := mustFloat(match[1])
		netUnit := canonicalWeightUnit(match[2])
		count := mustFloat(match[3])
		packUnit := strings.TrimSpace(match[4])
		result.NetContentQty = netQty
		result.NetContentUnit = netUnit
		result.OrderQuantity = count
		result.OrderUnit = packUnit
		result.NormalizedWeightG = weightToGrams(netQty, netUnit) * count
		if packUnit == "" {
			result.SpecName = formatNumber(netQty) + netUnit
			result.NeedsReview = true
			result.ReviewReason = "数量未填写包装单位"
		} else {
			result.SpecName = formatNumber(netQty) + netUnit + packUnit + "装"
		}
		quantitySegment = match[0]
	} else if match := combinedWeightPattern.FindStringSubmatch(line); len(match) == 5 {
		first := weightToGrams(mustFloat(match[1]), canonicalWeightUnit(match[2]))
		second := weightToGrams(mustFloat(match[3]), canonicalWeightUnit(match[4]))
		totalG := first + second
		result.NormalizedWeightG = totalG
		result.OrderQuantity = totalG / 1000
		result.OrderUnit = "kg"
		quantitySegment = match[0]
	} else if match := singleWeightPattern.FindStringSubmatch(line); len(match) == 3 {
		qty := mustFloat(match[1])
		unit := canonicalWeightUnit(match[2])
		result.OrderQuantity = qty
		result.OrderUnit = unit
		result.NormalizedWeightG = weightToGrams(qty, unit)
		quantitySegment = match[0]
	} else if match := packageQtyPattern.FindStringSubmatch(line); len(match) == 3 {
		result.OrderQuantity = mustFloat(match[1])
		result.OrderUnit = match[2]
		result.NeedsReview = true
		result.ReviewReason = "包装数量没有明确净含量，不能换算重量"
		quantitySegment = match[0]
	} else {
		result.NeedsReview = true
		result.ReviewReason = "未识别到明确数量或规格"
	}

	result.ParentName = productParentName(line, quantitySegment)
	if result.ParentName == "" || isInstructionLine(result.ParentName) {
		result.NeedsReview = true
		if result.ReviewReason == "" {
			result.ReviewReason = "无法识别父商品名称"
		}
	}
	return result
}

func CurateProducts(rows []RawOrder, refs []ERPReferenceProduct) ([]Product, []SKU, []ProductAlias, []OrderItem, []Issue) {
	refByName := map[string][]ERPReferenceProduct{}
	for _, ref := range refs {
		if !ref.Active {
			continue
		}
		key := normalizeProductName(ref.Name)
		if key != "" {
			refByName[key] = append(refByName[key], ref)
		}
	}

	productMap := map[string]Product{}
	skuMap := map[string]SKU{}
	aliases := make([]ProductAlias, 0)
	items := make([]OrderItem, 0)
	issues := make([]Issue, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.ProductRaw) == "" {
			issues = append(issues, newIssue("order", nonEmpty(row.SourceOrderKey, rowLocator(row)), "order_product_missing", "error", "订单未填写商品描述", row))
			continue
		}
		lines := splitProductLines(row.ProductRaw)
		for lineNo, line := range lines {
			parsed := ParseProductLine(line)
			parentNorm := normalizeProductName(parsed.ParentName)
			productKey := "product:" + shortHash(parsed.ProductKind+"\x00"+parentNorm)
			if parentNorm == "" {
				productKey = "product-review:" + shortHash(row.SourceOrderKey+"\x00"+line)
			}
			product, ok := productMap[productKey]
			if !ok {
				product = Product{
					ProductKey: productKey, CanonicalName: parsed.ParentName, ProductKind: parsed.ProductKind,
					RoastLevel: parsed.RoastLevel, MatchMethod: "unmatched", ReviewStatus: ReviewAutoReady,
				}
				matches := refByName[parentNorm]
				if len(matches) == 1 {
					product.CanonicalName = matches[0].Name
					product.ERPMatchID = matches[0].ID
					product.ERPMatchName = matches[0].Name
					product.MatchMethod = "erp_name_exact"
					product.MatchScore = 1
				} else if len(matches) > 1 {
					product.MatchMethod = "erp_name_ambiguous"
					product.ReviewStatus = ReviewNeedsReview
				}
				if product.MatchMethod == "unmatched" {
					if suggestion, method, score := productReferenceSuggestion(refs, parsed.ParentName, parsed.NormalizedLine); method != "" {
						product.ERPMatchID = suggestion.ID
						product.ERPMatchName = suggestion.Name
						product.MatchMethod = method
						product.MatchScore = score
						product.ReviewStatus = ReviewNeedsReview
					}
				}
				if parentNorm == "" || isInstructionLine(parsed.ParentName) {
					product.ReviewStatus = ReviewNeedsReview
				}
				productMap[productKey] = product
			} else if product.MatchMethod != "erp_name_exact" {
				if matches := refByName[parentNorm]; len(matches) == 1 {
					product.CanonicalName = matches[0].Name
					product.ERPMatchID = matches[0].ID
					product.ERPMatchName = matches[0].Name
					product.MatchMethod = "erp_name_exact"
					product.MatchScore = 1
					productMap[productKey] = product
				}
			}

			skuKey := ""
			if parsed.SpecName != "" {
				skuKey = "sku:" + shortHash(productKey+"\x00"+normalizeProductName(parsed.SpecName))
				if _, ok := skuMap[skuKey]; !ok {
					skuMap[skuKey] = SKU{
						SKUKey: skuKey, ProductKey: productKey, SpecName: parsed.SpecName, SalesUnit: parsed.OrderUnit,
						NetContentQty: parsed.NetContentQty, NetContentUnit: parsed.NetContentUnit,
						NormalizedWeightG: weightToGrams(parsed.NetContentQty, parsed.NetContentUnit),
						ReviewStatus:      ternaryReview(parsed.NeedsReview),
					}
				}
			}
			status := ternaryReview(parsed.NeedsReview)
			itemKey := nonEmpty(row.SourceOrderKey, rowLocator(row)) + ":" + strconv.Itoa(lineNo+1)
			items = append(items, OrderItem{
				SourceItemKey: itemKey, SourceOrderKey: row.SourceOrderKey, LineNo: lineNo + 1, RawLine: line,
				ProductKey: productKey, SKUKey: skuKey, ParentName: parsed.ParentName, SpecName: parsed.SpecName,
				ProductKind: parsed.ProductKind, RoastLevel: parsed.RoastLevel, OrderQuantity: parsed.OrderQuantity,
				OrderUnit: parsed.OrderUnit, NormalizedWeightG: parsed.NormalizedWeightG, ReviewStatus: status,
			})
			aliases = append(aliases, ProductAlias{
				ProductKey: productKey, SKUKey: skuKey, RawLine: line, NormalizedLine: parsed.NormalizedLine,
				SourceOrderKey: row.SourceOrderKey, MatchMethod: product.MatchMethod, MatchScore: product.MatchScore,
			})
			if parsed.NeedsReview {
				issueRow := row
				issues = append(issues, newIssue("order_item", itemKey, "product_line_needs_review", "warning", parsed.ReviewReason, issueRow))
			}
		}
	}

	products := make([]Product, 0, len(productMap))
	for _, product := range productMap {
		products = append(products, product)
	}
	sort.Slice(products, func(i, j int) bool { return products[i].ProductKey < products[j].ProductKey })
	skus := make([]SKU, 0, len(skuMap))
	for _, sku := range skuMap {
		skus = append(skus, sku)
	}
	sort.Slice(skus, func(i, j int) bool { return skus[i].SKUKey < skus[j].SKUKey })
	return products, skus, aliases, items, issues
}

func splitProductLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, "•·-—"))
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func normalizeProductText(raw string) string {
	raw = normalizeDigits(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		"ＫＧ", "kg", "KG", "kg", "Kg", "kg", "公斤", "kg", "千克", "kg",
		"克", "g", "Ｇ", "g", "磅", "lb", "LB", "lb", "LBS", "lb",
		"×", "*", "✖", "*", "✖️", "*", "Ｘ", "*", "ｘ", "*",
		"（", "(", "）", ")", "＋", "+", "：", ":",
	)
	normalized := strings.Join(strings.Fields(replacer.Replace(raw)), " ")
	normalized = regexp.MustCompile(`^\d{1,2}\s*[.、,，)]\s*`).ReplaceAllString(normalized, "")
	normalized = regexp.MustCompile(`^\d{1,2}\s+`).ReplaceAllString(normalized, "")
	return strings.TrimSpace(normalized)
}

func productParentName(line, quantitySegment string) string {
	name := line
	if quantitySegment != "" {
		name = strings.Replace(name, quantitySegment, " ", 1)
	}
	name = weightPackPattern.ReplaceAllString(name, " ")
	name = weightTimesPattern.ReplaceAllString(name, " ")
	name = combinedWeightPattern.ReplaceAllString(name, " ")
	name = singleWeightPattern.ReplaceAllString(name, " ")
	name = packageQtyPattern.ReplaceAllString(name, " ")
	name = roastPattern.ReplaceAllString(name, " ")
	name = regexp.MustCompile(`(?i)^(?:生豆|熟豆|咖啡豆|挂耳|速溶|订单)\s*[:：]?\s*`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`[()（）].*?[)）]`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(strings.Trim(name, " :：,，。;；+-_"))
	return strings.Join(strings.Fields(name), " ")
}

func detectProductKind(line string) string {
	switch {
	case strings.Contains(line, "生豆"):
		return "green_bean"
	case strings.Contains(line, "挂耳"):
		return "drip_bag"
	case strings.Contains(line, "速溶"):
		return "instant"
	case strings.Contains(line, "样品") || strings.Contains(line, "赠送") || strings.Contains(line, "送"):
		return "sample"
	case strings.Contains(line, "包装袋") || strings.Contains(line, "豆袋") || strings.Contains(line, "纸杯"):
		return "accessory"
	case strings.Contains(line, "加工") || strings.Contains(line, "服务费") || strings.Contains(line, "烘焙费"):
		return "service"
	default:
		return "roasted_bean"
	}
}

func isInstructionLine(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsLetter(first) && !unicode.IsNumber(first) {
		return true
	}
	markers := []string{
		"二维码", "联系方式", "联系电话", "有现货", "没现货", "可以替换", "每款",
		"订单", "备注", "随机", "标签", "包装要求", "发货", "货款", "快递",
	}
	for _, marker := range markers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func normalizeProductName(raw string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-_，,。.:：;；()（）[]【】/", r) {
			return -1
		}
		return r
	}, strings.TrimSpace(raw)))
}

func canonicalWeightUnit(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "kg", "公斤", "千克":
		return "kg"
	case "lb", "lbs", "磅":
		return "lb"
	default:
		return "g"
	}
}

func weightToGrams(qty float64, unit string) float64 {
	switch canonicalWeightUnit(unit) {
	case "kg":
		return qty * 1000
	case "lb":
		return qty * 453.592
	default:
		return qty
	}
}

func mustFloat(raw string) float64 {
	v, _ := strconv.ParseFloat(raw, 64)
	return v
}

func formatNumber(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.000001 {
		return strconv.FormatInt(int64(math.Round(v)), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func ternaryReview(needs bool) string {
	if needs {
		return ReviewNeedsReview
	}
	return ReviewAutoReady
}

func productSuggestion(refs []ERPReferenceProduct, candidate string) (ERPReferenceProduct, float64) {
	var best ERPReferenceProduct
	bestScore := 0.0
	for _, ref := range refs {
		if !ref.Active {
			continue
		}
		score := bigramSimilarity(normalizeProductName(candidate), normalizeProductName(ref.Name))
		if score > bestScore {
			best, bestScore = ref, score
		}
	}
	return best, bestScore
}

func productReferenceSuggestion(refs []ERPReferenceProduct, candidate, normalizedLine string) (ERPReferenceProduct, string, float64) {
	candidateNorm := normalizeProductName(candidate)
	lineNorm := normalizeProductName(normalizedLine)
	containsMatches := make([]ERPReferenceProduct, 0)
	longest := 0
	for _, ref := range refs {
		if !ref.Active {
			continue
		}
		refNorm := normalizeProductName(ref.Name)
		if refNorm == "" || (!strings.Contains(lineNorm, refNorm) && !strings.Contains(candidateNorm, refNorm)) {
			continue
		}
		length := len([]rune(refNorm))
		if length > longest {
			containsMatches = []ERPReferenceProduct{ref}
			longest = length
		} else if length == longest {
			containsMatches = append(containsMatches, ref)
		}
	}
	if len(containsMatches) == 1 {
		return containsMatches[0], "erp_name_contains_suggestion", 0.95
	}
	best, score := productSuggestion(refs, candidate)
	if best.ID > 0 && score >= 0.6 {
		return best, "erp_name_fuzzy_suggestion", score
	}
	return ERPReferenceProduct{}, "", 0
}

func bigramSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	grams := func(s string) map[string]struct{} {
		runes := []rune(s)
		out := map[string]struct{}{}
		if len(runes) == 1 {
			out[string(runes)] = struct{}{}
			return out
		}
		for i := 0; i < len(runes)-1; i++ {
			out[string(runes[i:i+2])] = struct{}{}
		}
		return out
	}
	ga, gb := grams(a), grams(b)
	intersection := 0
	for gram := range ga {
		if _, ok := gb[gram]; ok {
			intersection++
		}
	}
	return 2 * float64(intersection) / float64(len(ga)+len(gb))
}

func debugProductLine(line ProductLine) string {
	return fmt.Sprintf("%s %s %g%s", line.ParentName, line.SpecName, line.OrderQuantity, line.OrderUnit)
}
