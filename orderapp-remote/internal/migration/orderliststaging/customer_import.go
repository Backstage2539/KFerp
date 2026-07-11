package orderliststaging

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	recipientLabelPattern        = regexp.MustCompile(`(?i)(?:【|\[)?(?:收货人名字|收货人|收件人|联系人|姓名)(?:】|\])?\s*[:：]?\s*([\p{Han}A-Za-z][\p{Han}A-Za-z·]{1,11})`)
	recipientBeforePhonePattern  = regexp.MustCompile(`([\p{Han}A-Za-z][\p{Han}A-Za-z·]{1,11}(?:先生|女士|小姐)?)\s*(?:\+?86[\s-]?)?1[3-9](?:[\s-]?\d){9}`)
	remarkPrefixPattern          = regexp.MustCompile(`(?i)^(?:备注|客户|客户名称)\s*[:：]?\s*`)
	remarkSuffixPattern          = regexp.MustCompile(`(?i)(?:的)?(?:订单|下单|订货|标签|零售|批发|渠道|客户|样品单|样品|送货|补发货|补货|一件代发|代发|寄样|参赛豆|精品豆|咖啡豆|生豆|标|单|(?:做|定制)?库存(?:消耗)?|(?:样品|赠品)?赠送|订)+$`)
	remarkCustomerPrefixPattern  = regexp.MustCompile(`^(.{2,20}?)(?:客户|订单|样品单|样品|标签|零售|批发|渠道|代加工|常用烘焙度|不用贴|新版贴纸|咖啡豆袋货款)`)
	remarkLabelCustomerPattern   = regexp.MustCompile(`(?i)^贴\s*([\p{Han}A-Za-z0-9· ]{1,20}?)(?:正面)?标签`)
	remarkLogoCustomerPattern    = regexp.MustCompile(`(?i)^贴\s*([\p{Han}A-Za-z0-9· ]{2,20}?)(?:logo)`)
	remarkListPrefixPattern      = regexp.MustCompile(`^(?:\d{1,2}[.、)）]|\d️?⃣|[①②③④⑤⑥⑦⑧⑨⑩❶❷❸❹❺❻❼❽❾❿])\s*`)
	remarkQuantityPattern        = regexp.MustCompile(`(?i)^\d+(?:\.\d+)?\s*(?:kg|g|克|公斤|千克|磅|斤|个|袋|包|盒|月|号|种|批)`)
	remarkChineseQuantityPattern = regexp.MustCompile(`^[一二三四五六七八九十百]+(?:个|袋|包|盒|磅|克|件|种)`)
	remarkDatePattern            = regexp.MustCompile(`^\d{1,2}(?:[./-]\d{1,2}|月\d{1,2}日?)`)
)

type customerImportObservation struct {
	row                    *RawOrder
	phones                 []string
	name                   string
	nameKey                string
	recipientName          string
	hasRemark              bool
	remarkName             string
	remarkNameKey          string
	deliveryAddressKey     string
	deliveryAddressDisplay string
	targetMatch            *ERPReferenceCustomer
	devMatch               *ERPReferenceCustomer
	targetIDs              []int64
	devIDs                 []int64
}

type disjointSet struct {
	parent []int
}

func newDisjointSet(size int) *disjointSet {
	parent := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &disjointSet{parent: parent}
}

func (d *disjointSet) find(value int) int {
	if d.parent[value] != value {
		d.parent[value] = d.find(d.parent[value])
	}
	return d.parent[value]
}

func (d *disjointSet) union(left, right int) {
	leftRoot := d.find(left)
	rightRoot := d.find(right)
	if leftRoot != rightRoot {
		d.parent[rightRoot] = leftRoot
	}
}

func BuildCustomerImportRows(rows []RawOrder, devRefs, targetRefs []ERPReferenceCustomer, options CustomerImportOptions) ([]CustomerImportRow, []Issue) {
	targetByPhone := referenceCustomersByPhone(targetRefs, false)
	devByPhone := referenceCustomersByPhone(devRefs, true)
	targetByName := referenceCustomersByName(targetRefs, false)
	devByName := referenceCustomersByName(devRefs, true)
	observations := make([]customerImportObservation, 0, len(rows))
	issues := make([]Issue, 0)
	for i := range rows {
		phones := NormalizePhones(rows[i].CustomerRaw)
		recipientName := extractRecipientName(rows[i].CustomerRaw)
		remarkRaw := strings.TrimSpace(rows[i].RemarkRaw)
		hasRemark := remarkRaw != ""
		remarkName := extractRemarkCustomerName(remarkRaw)
		identityName := recipientName
		var targetMatch, devMatch *ERPReferenceCustomer
		var targetIDs, devIDs []int64
		if hasRemark {
			identityName = remarkName
			if remarkName != "" {
				targetMatch, targetIDs = uniqueReferenceNameMatch(remarkName, targetByName)
				devMatch, devIDs = uniqueReferenceNameMatch(remarkName, devByName)
			} else {
				issues = append(issues, newIssue(
					"customer_import", "remark:"+shortHash(nonEmpty(rows[i].SourceOrderKey, rowLocator(rows[i]))),
					"customer_remark_name_unresolved", "warning", "备注非空但无法安全提取客户名称，需要人工确认", rows[i],
				))
			}
		} else {
			targetMatch, targetIDs = uniqueReferenceMatch(phones, targetByPhone)
			devMatch, devIDs = uniqueReferenceMatch(phones, devByPhone)
		}
		addressKey, addressDisplay := normalizeDeliveryAddress(rows[i].CustomerRaw, recipientName)
		observations = append(observations, customerImportObservation{
			row: &rows[i], phones: phones, name: identityName, nameKey: normalizeCustomerName(identityName),
			recipientName: recipientName, hasRemark: hasRemark, remarkName: remarkName,
			remarkNameKey: normalizeCustomerName(remarkName), deliveryAddressKey: addressKey,
			deliveryAddressDisplay: addressDisplay, targetMatch: targetMatch, devMatch: devMatch,
			targetIDs: targetIDs, devIDs: devIDs,
		})
	}

	groupsByPhone := map[string][]int{}
	groupsByTarget := map[string][]int{}
	groupsByDev := map[string][]int{}
	groupsByRetailName := map[string][]int{}
	groupsByRemarkName := map[string][]int{}
	for index, observation := range observations {
		scope := "retail"
		if observation.hasRemark {
			scope = "remark"
		}
		if !observation.hasRemark {
			for _, phone := range observation.phones {
				groupsByPhone[phone] = append(groupsByPhone[phone], index)
			}
			if observation.nameKey != "" {
				groupsByRetailName[observation.nameKey] = append(groupsByRetailName[observation.nameKey], index)
			}
		} else if observation.remarkNameKey != "" {
			groupsByRemarkName[observation.remarkNameKey] = append(groupsByRemarkName[observation.remarkNameKey], index)
		}
		if observation.targetMatch != nil {
			key := scope + ":" + strconv.FormatInt(observation.targetMatch.ID, 10)
			groupsByTarget[key] = append(groupsByTarget[key], index)
		}
		if observation.devMatch != nil {
			key := scope + ":" + strconv.FormatInt(observation.devMatch.ID, 10)
			groupsByDev[key] = append(groupsByDev[key], index)
		}
	}

	set := newDisjointSet(len(observations))
	unionIndexGroups(set, groupsByPhone)
	unionIndexGroups(set, groupsByTarget)
	unionIndexGroups(set, groupsByDev)
	unionIndexGroups(set, groupsByRemarkName)
	for nameKey, indexes := range groupsByRetailName {
		if len(indexes) < 2 {
			continue
		}
		phoneSet := map[string]struct{}{}
		targetIDSet := map[int64]struct{}{}
		for _, index := range indexes {
			for _, phone := range observations[index].phones {
				phoneSet[phone] = struct{}{}
			}
			for _, id := range observations[index].targetIDs {
				targetIDSet[id] = struct{}{}
			}
		}
		if len(phoneSet) == 0 {
			continue
		}
		if len(phoneSet) == 1 {
			unionIndexes(set, indexes)
			continue
		}
		if !customerNameCanMergePhones(observations[indexes[0]].name) || len(targetIDSet) > 1 {
			row := *observations[indexes[0]].row
			issues = append(issues, newIssue(
				"customer_import", "customer_name:"+shortHash(nameKey), "customer_cross_phone_name_unsafe", "warning",
				"同一短名称或高风险名称出现多个手机号，未自动合并，请人工确认", row,
			))
			continue
		}
		unionIndexes(set, indexes)
	}

	components := map[int][]customerImportObservation{}
	for index, observation := range observations {
		root := set.find(index)
		components[root] = append(components[root], observation)
	}
	result := make([]CustomerImportRow, 0, len(components))
	for _, component := range components {
		row, rowIssues := buildCustomerImportRow(component, options)
		result = append(result, row)
		issues = append(issues, rowIssues...)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].CandidateKey < result[j].CandidateKey
	})
	return result, issues
}

func buildCustomerImportRow(observations []customerImportObservation, options CustomerImportOptions) (CustomerImportRow, []Issue) {
	sort.SliceStable(observations, func(i, j int) bool {
		return customerObservationMoreRecent(observations[i], observations[j])
	})
	latest := observations[0]
	row := CustomerImportRow{
		Action: "create", Active: true, PortalEnabled: false, ReviewStatus: ReviewAutoReady,
		OrderCount: len(observations), LatestSourceOrderKey: latest.row.SourceOrderKey,
		LatestCustomerRaw: strings.TrimSpace(latest.row.CustomerRaw), LatestRemarkRaw: strings.TrimSpace(latest.row.RemarkRaw),
	}

	targets := uniqueMatchedCustomers(observations, true)
	devMatches := uniqueMatchedCustomers(observations, false)
	var target *ERPReferenceCustomer
	var dev *ERPReferenceCustomer
	if len(targets) == 1 {
		target = &targets[0]
	}
	if len(devMatches) == 1 {
		dev = &devMatches[0]
	}

	phoneDates := map[string]string{}
	phoneOrder := make([]string, 0)
	nameSeen := map[string]struct{}{}
	names := make([]string, 0)
	recipientSeen := map[string]struct{}{}
	recipientNames := make([]string, 0)
	remarkSeen := map[string]struct{}{}
	remarks := make([]string, 0)
	addressSeen := map[string]struct{}{}
	addressSamples := make([]string, 0)
	sourceKeys := make([]string, 0, len(observations))
	reasons := make([]string, 0)
	for _, observation := range observations {
		if observation.row.OrderDate != "" {
			if row.LastOrderDate == "" || observation.row.OrderDate > row.LastOrderDate {
				row.LastOrderDate = observation.row.OrderDate
			}
			if row.FirstOrderDate == "" || observation.row.OrderDate < row.FirstOrderDate {
				row.FirstOrderDate = observation.row.OrderDate
			}
		}
		if observation.row.SourceOrderKey != "" {
			sourceKeys = append(sourceKeys, observation.row.SourceOrderKey)
		}
		if observation.name != "" {
			key := normalizeCustomerName(observation.name)
			if _, exists := nameSeen[key]; !exists {
				nameSeen[key] = struct{}{}
				names = append(names, observation.name)
			}
		}
		if observation.recipientName != "" {
			key := normalizeCustomerName(observation.recipientName)
			if _, exists := recipientSeen[key]; !exists {
				recipientSeen[key] = struct{}{}
				recipientNames = append(recipientNames, observation.recipientName)
			}
		}
		remarkRaw := strings.TrimSpace(observation.row.RemarkRaw)
		if remarkRaw != "" {
			key := normalizeCustomerName(remarkRaw)
			if _, exists := remarkSeen[key]; !exists {
				remarkSeen[key] = struct{}{}
				remarks = append(remarks, remarkRaw)
			}
		}
		if observation.deliveryAddressKey != "" {
			if _, exists := addressSeen[observation.deliveryAddressKey]; !exists {
				addressSeen[observation.deliveryAddressKey] = struct{}{}
				if observation.deliveryAddressDisplay != "" {
					addressSamples = append(addressSamples, observation.deliveryAddressDisplay)
				}
			}
		}
		for _, phone := range observation.phones {
			if _, exists := phoneDates[phone]; exists {
				continue
			}
			phoneDates[phone] = observation.row.OrderDate
			phoneOrder = append(phoneOrder, phone)
		}
	}
	row.PhoneCount = len(phoneOrder)
	row.HistoricalPhones = strings.Join(phoneOrder, " | ")
	row.HistoricalNames = strings.Join(names, " | ")
	row.RecipientNames = strings.Join(recipientNames, " | ")
	row.HistoricalRemarks = strings.Join(remarks, " | ")
	row.DeliveryAddressCount = len(addressSeen)
	row.DeliveryAddressSamples = joinEvidenceSamples(addressSamples, 12)
	row.SourceOrderKeys = strings.Join(sourceKeys, " | ")
	for _, observation := range observations {
		if len(observation.phones) == 1 {
			row.Phone = observation.phones[0]
			row.CompanyPhone = observation.phones[0]
			row.LatestPhoneObservedDate = observation.row.OrderDate
			break
		}
	}

	isRemarkCustomer := latest.hasRemark
	row.InferredCustomerType = "retail"
	row.CustomerTypeBasis = "备注为空，按收件人识别零售客户"
	if isRemarkCustomer {
		row.InferredCustomerType = "wholesale"
		row.CustomerTypeBasis = fmt.Sprintf("备注客户，%d 个规范收件地址，判定批发客户", row.DeliveryAddressCount)
		if row.DeliveryAddressCount == 0 {
			row.CustomerTypeBasis = "备注客户，未解析到规范收件地址，暂按批发客户，需人工确认"
			row.ReviewStatus = ReviewNeedsReview
			reasons = append(reasons, "收件地址待确认")
		} else if row.DeliveryAddressCount > 1 {
			row.InferredCustomerType = "channel"
			row.CustomerTypeBasis = fmt.Sprintf("备注客户，%d 个规范收件地址，判定渠道客户", row.DeliveryAddressCount)
		}
	} else if row.DeliveryAddressCount != 1 {
		row.CustomerTypeBasis = fmt.Sprintf("备注为空，按收件人识别零售客户；记录到 %d 个规范收件地址", row.DeliveryAddressCount)
	}

	latestName := ""
	for _, observation := range observations {
		if observation.name != "" {
			latestName = observation.name
			break
		}
	}
	nameKey := normalizeCustomerName(latestName)
	switch {
	case target != nil:
		row.CandidateKey = "erp_customer:" + strconv.FormatInt(target.ID, 10)
		row.Action = "update"
		row.ERPMatchID = target.ID
		row.ERPMatchName = strings.TrimSpace(target.Name)
		if isRemarkCustomer {
			row.MergeMethod = "production_erp_remark_name"
		} else {
			row.MergeMethod = "production_erp_phone"
		}
		applyTargetCustomer(&row, *target, options)
	case dev != nil:
		row.CandidateKey = "dev_customer:" + strconv.FormatInt(dev.ID, 10)
		if isRemarkCustomer {
			row.MergeMethod = "development_erp_remark_name"
		} else {
			row.MergeMethod = "development_erp_phone"
		}
		row.Name = strings.TrimSpace(dev.Name)
		if customerNameNeedsReview(row.Name) && latestName != "" && !customerNameNeedsReview(latestName) {
			row.Name = latestName
		}
		row.RawName = nonEmpty(latestName, row.Name)
	case isRemarkCustomer && latestName != "":
		row.CandidateKey = "remark_customer:" + shortHash(nameKey)
		row.MergeMethod = "remark_customer_exact"
		row.Name = latestName
		row.RawName = nonEmpty(row.LatestRemarkRaw, latestName)
	case !isRemarkCustomer && len(phoneOrder) > 0 && customerNameCanMergePhones(latestName):
		row.CandidateKey = "customer_name:" + shortHash(nameKey)
		row.MergeMethod = "retail_safe_name_exact"
		row.Name = latestName
		row.RawName = latestName
	case !isRemarkCustomer && len(phoneOrder) == 1:
		row.CandidateKey = "phone:" + phoneOrder[0]
		row.MergeMethod = "retail_single_phone"
		row.Name = latestName
		row.RawName = latestName
	default:
		row.CandidateKey = "source:" + shortHash(nonEmpty(latest.row.SourceOrderKey, rowLocator(*latest.row)))
		row.MergeMethod = "source_only"
		row.Name = latestName
		row.RawName = latestName
	}
	if row.Name == "" || customerNameNeedsReview(row.Name) || isCustomerPlaceholder(row.Name) {
		row.Name = nonEmpty(row.Name, "待确认客户-"+shortHash(row.CandidateKey))
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "客户名称待确认")
	}
	if target == nil {
		row.CustomerType = row.InferredCustomerType
		if isRemarkCustomer {
			row.CompanyName = row.Name
			row.Contact = latest.recipientName
		} else {
			row.Contact = row.Name
		}
		if row.InferredCustomerType != "channel" {
			row.Address = strings.TrimSpace(latest.row.CustomerRaw)
		}
		row.DefaultSourceID, row.DefaultSourceName = resolveReferenceOption(latestNonEmptyRaw(observations, func(raw RawOrder) string { return raw.OrderSourceRaw }), options.Sources, false)
		row.DefaultOrderTypeID, row.DefaultOrderTypeName = resolveReferenceOption(latestNonEmptyRaw(observations, func(raw RawOrder) string { return raw.OrderTypeRaw }), options.OrderTypes, false)
		row.ResponsibleEmployeeID, row.ResponsibleEmployeeName = resolveReferenceOption("", options.Employees, true)
	} else {
		if row.CustomerType == "" {
			row.CustomerType = row.InferredCustomerType
		}
		if row.CustomerType != row.InferredCustomerType {
			row.ReviewStatus = ReviewNeedsReview
			reasons = append(reasons, "ERP客户类型与历史推断不同")
		}
	}
	if isRemarkCustomer && latest.remarkName == "" {
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "备注客户名称待确认")
	}
	if row.Phone == "" {
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "最新有效手机号待确认")
	}
	if row.ResponsibleEmployeeID == 0 {
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "负责人待确认")
	}
	if row.PhoneCount > 1 {
		reasons = append(reasons, "已按最近订单选择主手机号")
	}
	if len(targets) > 1 {
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "匹配到多个生产ERP客户")
	}
	if len(observations) > 0 && len(observations[0].phones) > 1 {
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "最近记录包含多个手机号")
	}
	row.ReviewReasons = strings.Join(uniqueStrings(reasons), "；")

	rowIssues := make([]Issue, 0)
	if len(targets) > 1 {
		rowIssues = append(rowIssues, newIssue("customer_import", row.CandidateKey, "customer_target_erp_ambiguous", "error", "历史客户匹配到多个生产ERP客户，需要人工确认", *latest.row))
	}
	if len(observations) > 0 && len(observations[0].phones) > 1 {
		rowIssues = append(rowIssues, newIssue("customer_import", row.CandidateKey, "customer_latest_phone_ambiguous", "warning", "最近客户记录中包含多个手机号，主手机号需要人工确认", *latest.row))
	}
	return row, rowIssues
}

func extractRemarkCustomerName(raw string) string {
	raw = strings.TrimSpace(normalizeDigits(strings.ReplaceAll(strings.ReplaceAll(raw, "\r", "\n"), "\t", " ")))
	if raw == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == '，' || r == ';' || r == '；' || r == ':' || r == '：'
	})
	if len(parts) == 0 {
		return ""
	}
	candidate := strings.TrimSpace(parts[0])
	candidate = strings.TrimSpace(strings.TrimLeft(candidate, "⚠️☑✅❗❕※*# "))
	if remarkDatePattern.MatchString(candidate) {
		return ""
	}
	candidate = strings.TrimSpace(remarkListPrefixPattern.ReplaceAllString(candidate, ""))
	candidate = remarkPrefixPattern.ReplaceAllString(candidate, "")
	if index := strings.Index(candidate, "、"); index > 0 {
		candidate = strings.TrimSpace(candidate[:index])
	}
	if index := strings.IndexAny(candidate, "（("); index > 0 {
		candidate = strings.TrimSpace(candidate[:index])
	}
	if match := remarkLabelCustomerPattern.FindStringSubmatch(candidate); len(match) == 2 {
		candidate = strings.TrimSpace(match[1])
	} else if match := remarkLogoCustomerPattern.FindStringSubmatch(candidate); len(match) == 2 {
		candidate = strings.TrimSpace(match[1])
	} else if match := remarkCustomerPrefixPattern.FindStringSubmatch(candidate); len(match) == 2 {
		candidate = strings.TrimSpace(match[1])
	}
	for {
		next := strings.TrimSpace(remarkSuffixPattern.ReplaceAllString(candidate, ""))
		if next == candidate {
			break
		}
		candidate = next
	}
	candidate = strings.Trim(candidate, " :-_，,。.")
	if candidate == "" || len([]rune(candidate)) > 30 || looksLikeAddress(candidate) || len(NormalizePhones(candidate)) > 0 ||
		remarkQuantityPattern.MatchString(candidate) || remarkChineseQuantityPattern.MatchString(candidate) || remarkDatePattern.MatchString(candidate) ||
		strings.ContainsAny(candidate, "“”\"【】") || strings.HasSuffix(candidate, "的") {
		return ""
	}
	normalized := normalizeCustomerName(candidate)
	for _, generic := range []string{
		"零售", "淘宝", "样品", "咖啡店", "新咖啡店", "咖啡店发货", "咖啡店烘焙工厂", "咖啡培训机构",
		"随机赠送", "赠送", "门店自提", "工厂送货", "工厂发货", "工厂", "库存", "测试", "包装", "标签", "品名", "正标", "正面", "现货",
		"微店", "卷膜", "用", "熟豆快递", "红盒子", "蓝色盒子",
		"到付", "快团团", "旧账", "每类", "测试喷码机", "顺丰到付", "顺丰运费运费贵", "奶茶店", "民宿", "经销商",
		"咖啡", "咖啡生豆", "定制挂耳", "新豆子到再发", "补发", "补录", "定制logo", "定制标", "展会",
		"参赛豆", "培训老师的店", "意式", "拉萨经销商", "甜点店", "生豆", "蓝盒", "西双版纳线下店", "货款免费",
	} {
		if normalized == normalizeCustomerName(generic) || strings.HasPrefix(normalized, normalizeCustomerName("随机赠送")) {
			return ""
		}
	}
	for _, prefix := range []string{
		"随机赠送", "随机装", "随机送", "赠送", "赠品", "送一", "送两", "送2", "送3", "送4", "送5",
		"贴这个", "要4个", "烘", "送", "包装", "豆袋", "不用", "不要", "不贴", "正面不贴", "优先", "全部", "公版",
		"标签", "样品", "用库存", "最好", "含包装", "白绿色", "红色包装", "都用", "不需要", "其他工厂",
		"使用", "发货", "咖啡店", "喜欢", "顾客", "意式机", "寄快递", "挂耳", "放入", "有现货", "正面",
		"淘宝", "熟豆", "自己买", "红酒日晒", "零售", "需要", "盒子", "贴公版", "贴棵凡", "两个", "两种", "先烘焙",
		"封口", "走京东", "顺丰", "定制", "新豆子", "快团团", "贴纸",
	} {
		if strings.HasPrefix(normalized, normalizeCustomerName(prefix)) {
			return ""
		}
	}
	for _, marker := range []string{"可以发现货", "根据库存发货", "生产日期喷印", "烘焙日期", "烘焙度参考", "做好区分标签", "展会烘焙"} {
		if strings.Contains(normalized, normalizeCustomerName(marker)) {
			return ""
		}
	}
	if digitsOnly(candidate) != "" && len([]rune(candidate)) <= len([]rune(digitsOnly(candidate)))+2 {
		return ""
	}
	return candidate
}

func extractRecipientName(raw string) string {
	raw = strings.TrimSpace(normalizeDigits(raw))
	if raw == "" {
		return ""
	}
	if match := recipientLabelPattern.FindStringSubmatch(raw); len(match) == 2 {
		if candidate := cleanRecipientName(match[1]); candidate != "" {
			return candidate
		}
	}
	if match := recipientBeforePhonePattern.FindStringSubmatch(raw); len(match) == 2 {
		if candidate := cleanRecipientName(match[1]); candidate != "" {
			return candidate
		}
	}
	return cleanRecipientName(extractCustomerName(raw))
}

func cleanRecipientName(raw string) string {
	candidate := cleanNameCandidate(raw)
	if candidate == "" || len([]rune(candidate)) > 12 || looksLikeAddress(candidate) {
		return ""
	}
	for _, marker := range []string{"办公室", "前台", "客服", "仓库", "地址", "收货", "门店", "公司"} {
		if strings.Contains(candidate, marker) {
			return ""
		}
	}
	return candidate
}

func normalizeDeliveryAddress(raw, recipientName string) (string, string) {
	display := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n"))
	if display == "" {
		return "", ""
	}
	if strings.Contains(display, "门店自提") || normalizeCustomerName(display) == normalizeCustomerName("自提") {
		return "pickup:store", display
	}
	normalized := normalizeDigits(display)
	normalized = mobilePattern.ReplaceAllString(normalized, " ")
	if recipientName != "" {
		normalized = strings.ReplaceAll(normalized, recipientName, " ")
	}
	for _, marker := range []string{"收货人名字", "收货人", "收件人", "联系人", "姓名", "手机号", "手机号码", "联系电话", "电话", "地址"} {
		normalized = strings.ReplaceAll(normalized, marker, " ")
	}
	key := normalizeCustomerName(normalized)
	if key == "" {
		key = "self"
	}
	return key, display
}

func joinEvidenceSamples(values []string, limit int) string {
	if len(values) <= limit {
		return strings.Join(values, " | ")
	}
	return strings.Join(values[:limit], " | ") + fmt.Sprintf(" | 另有 %d 个地址，详见来源订单", len(values)-limit)
}

func applyTargetCustomer(row *CustomerImportRow, target ERPReferenceCustomer, options CustomerImportOptions) {
	row.Name = strings.TrimSpace(target.Name)
	row.RawName = nonEmpty(strings.TrimSpace(target.RawName), row.Name)
	row.CustomerType = strings.TrimSpace(target.CustomerType)
	row.CompanyName = strings.TrimSpace(target.CompanyName)
	row.CompanyAddress = strings.TrimSpace(target.CompanyAddress)
	row.CompanyPhone = nonEmpty(row.CompanyPhone, strings.TrimSpace(target.CompanyPhone))
	row.Contact = strings.TrimSpace(target.Contact)
	row.Phone = nonEmpty(row.Phone, strings.TrimSpace(target.Phone))
	row.Address = strings.TrimSpace(target.Address)
	row.DefaultSourceID = target.DefaultSourceID
	row.DefaultSourceName = optionLabelByID(target.DefaultSourceID, options.Sources)
	row.DefaultOrderTypeID = target.DefaultOrderTypeID
	row.DefaultOrderTypeName = optionLabelByID(target.DefaultOrderTypeID, options.OrderTypes)
	row.ResponsibleEmployeeID = target.ResponsibleEmployeeID
	row.ResponsibleEmployeeName = optionLabelByID(target.ResponsibleEmployeeID, options.Employees)
	row.PortalEnabled = target.PortalEnabled
	row.CapabilityTemplateKey = strings.TrimSpace(target.CapabilityTemplateKey)
	row.Active = target.Active
}

func referenceCustomersByPhone(refs []ERPReferenceCustomer, activeOnly bool) map[string][]ERPReferenceCustomer {
	result := map[string][]ERPReferenceCustomer{}
	for _, ref := range refs {
		if activeOnly && !ref.Active {
			continue
		}
		phones := append(NormalizePhones(ref.Phone), NormalizePhones(ref.CompanyPhone)...)
		seen := map[string]struct{}{}
		for _, phone := range phones {
			if _, exists := seen[phone]; exists {
				continue
			}
			seen[phone] = struct{}{}
			result[phone] = append(result[phone], ref)
		}
	}
	return result
}

func referenceCustomersByName(refs []ERPReferenceCustomer, activeOnly bool) map[string][]ERPReferenceCustomer {
	result := map[string][]ERPReferenceCustomer{}
	for _, ref := range refs {
		if activeOnly && !ref.Active {
			continue
		}
		seen := map[string]struct{}{}
		for _, name := range []string{ref.Name, ref.RawName, ref.CompanyName} {
			key := normalizeCustomerName(name)
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result[key] = append(result[key], ref)
		}
	}
	return result
}

func uniqueReferenceMatch(phones []string, byPhone map[string][]ERPReferenceCustomer) (*ERPReferenceCustomer, []int64) {
	byID := map[int64]ERPReferenceCustomer{}
	for _, phone := range phones {
		for _, ref := range byPhone[phone] {
			byID[ref.ID] = ref
		}
	}
	return uniqueReferenceFromMap(byID)
}

func uniqueReferenceNameMatch(name string, byName map[string][]ERPReferenceCustomer) (*ERPReferenceCustomer, []int64) {
	byID := map[int64]ERPReferenceCustomer{}
	for _, ref := range byName[normalizeCustomerName(name)] {
		byID[ref.ID] = ref
	}
	return uniqueReferenceFromMap(byID)
}

func uniqueReferenceFromMap(byID map[int64]ERPReferenceCustomer) (*ERPReferenceCustomer, []int64) {
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 1 {
		return nil, ids
	}
	ref := byID[ids[0]]
	return &ref, ids
}

func unionIndexGroups(set *disjointSet, groups map[string][]int) {
	for _, indexes := range groups {
		unionIndexes(set, indexes)
	}
}

func unionIndexes(set *disjointSet, indexes []int) {
	for index := 1; index < len(indexes); index++ {
		set.union(indexes[0], indexes[index])
	}
}

func customerNameCanMergePhones(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || isCustomerPlaceholder(name) || customerNameNeedsReview(name) {
		return false
	}
	if len([]rune(normalizeCustomerName(name))) >= 4 {
		return true
	}
	for _, marker := range []string{"公司", "酒店", "咖啡", "门店", "工作室", "餐厅", "商贸", "集团", "学校", "学院"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func customerObservationMoreRecent(left, right customerImportObservation) bool {
	if left.row.OrderDate != "" && right.row.OrderDate != "" && left.row.OrderDate != right.row.OrderDate {
		return left.row.OrderDate > right.row.OrderDate
	}
	if left.row.SheetPeriod != right.row.SheetPeriod {
		return left.row.SheetPeriod > right.row.SheetPeriod
	}
	if left.row.SourceRowNumber != right.row.SourceRowNumber {
		return left.row.SourceRowNumber < right.row.SourceRowNumber
	}
	return left.row.SourceOrderKey < right.row.SourceOrderKey
}

func uniqueMatchedCustomers(observations []customerImportObservation, target bool) []ERPReferenceCustomer {
	byID := map[int64]ERPReferenceCustomer{}
	for _, observation := range observations {
		match := observation.devMatch
		if target {
			match = observation.targetMatch
		}
		if match != nil {
			byID[match.ID] = *match
		}
	}
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := make([]ERPReferenceCustomer, 0, len(ids))
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result
}

func resolveReferenceOption(raw string, options []ERPReferenceOption, allowUnique bool) (int64, string) {
	rawKey := normalizeCustomerName(raw)
	active := make([]ERPReferenceOption, 0, len(options))
	for _, option := range options {
		if !option.Active {
			continue
		}
		active = append(active, option)
		if rawKey != "" && normalizeCustomerName(option.Label) == rawKey {
			id, _ := strconv.ParseInt(strings.TrimSpace(option.Value), 10, 64)
			return id, option.Label
		}
	}
	if allowUnique && len(active) == 1 {
		id, _ := strconv.ParseInt(strings.TrimSpace(active[0].Value), 10, 64)
		return id, active[0].Label
	}
	return 0, ""
}

func optionLabelByID(id int64, options []ERPReferenceOption) string {
	if id == 0 {
		return ""
	}
	want := strconv.FormatInt(id, 10)
	for _, option := range options {
		if strings.TrimSpace(option.Value) == want {
			return option.Label
		}
	}
	return ""
}

func latestNonEmptyRaw(observations []customerImportObservation, value func(RawOrder) string) string {
	for _, observation := range observations {
		if current := strings.TrimSpace(value(*observation.row)); current != "" {
			return current
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
