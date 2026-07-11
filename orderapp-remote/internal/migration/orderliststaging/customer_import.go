package orderliststaging

import (
	"sort"
	"strconv"
	"strings"
)

type customerImportObservation struct {
	row         *RawOrder
	phones      []string
	name        string
	nameKey     string
	targetMatch *ERPReferenceCustomer
	devMatch    *ERPReferenceCustomer
	targetIDs   []int64
	devIDs      []int64
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
	observations := make([]customerImportObservation, 0, len(rows))
	for i := range rows {
		phones := NormalizePhones(rows[i].CustomerRaw)
		name := extractCustomerName(rows[i].CustomerRaw)
		targetMatch, targetIDs := uniqueReferenceMatch(phones, targetByPhone)
		devMatch, devIDs := uniqueReferenceMatch(phones, devByPhone)
		observations = append(observations, customerImportObservation{
			row: &rows[i], phones: phones, name: name, nameKey: normalizeCustomerName(name),
			targetMatch: targetMatch, devMatch: devMatch, targetIDs: targetIDs, devIDs: devIDs,
		})
	}

	groupsByPhone := map[string][]int{}
	groupsByTarget := map[int64][]int{}
	groupsByDev := map[int64][]int{}
	groupsByName := map[string][]int{}
	for index, observation := range observations {
		for _, phone := range observation.phones {
			groupsByPhone[phone] = append(groupsByPhone[phone], index)
		}
		if observation.targetMatch != nil {
			groupsByTarget[observation.targetMatch.ID] = append(groupsByTarget[observation.targetMatch.ID], index)
		}
		if observation.devMatch != nil {
			groupsByDev[observation.devMatch.ID] = append(groupsByDev[observation.devMatch.ID], index)
		}
		if observation.nameKey != "" {
			groupsByName[observation.nameKey] = append(groupsByName[observation.nameKey], index)
		}
	}

	set := newDisjointSet(len(observations))
	unionIndexGroups(set, groupsByPhone)
	unionInt64IndexGroups(set, groupsByTarget)
	unionInt64IndexGroups(set, groupsByDev)
	issues := make([]Issue, 0)
	for nameKey, indexes := range groupsByName {
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
		LatestCustomerRaw: strings.TrimSpace(latest.row.CustomerRaw),
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
	row.SourceOrderKeys = strings.Join(sourceKeys, " | ")
	for _, observation := range observations {
		if len(observation.phones) == 1 {
			row.Phone = observation.phones[0]
			row.CompanyPhone = observation.phones[0]
			row.LatestPhoneObservedDate = observation.row.OrderDate
			break
		}
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
		row.MergeMethod = "production_erp_phone"
		applyTargetCustomer(&row, *target, options)
	case dev != nil:
		row.CandidateKey = "dev_customer:" + strconv.FormatInt(dev.ID, 10)
		row.MergeMethod = "development_erp_phone"
		row.Name = strings.TrimSpace(dev.Name)
		if customerNameNeedsReview(row.Name) && latestName != "" && !customerNameNeedsReview(latestName) {
			row.Name = latestName
		}
		row.RawName = nonEmpty(latestName, row.Name)
	case len(phoneOrder) > 0 && customerNameCanMergePhones(latestName):
		row.CandidateKey = "customer_name:" + shortHash(nameKey)
		row.MergeMethod = "safe_name_exact"
		row.Name = latestName
		row.RawName = latestName
	case len(phoneOrder) == 1:
		row.CandidateKey = "phone:" + phoneOrder[0]
		row.MergeMethod = "single_phone"
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
		row.CustomerType = ""
		row.ReviewStatus = ReviewNeedsReview
		reasons = append(reasons, "客户类型待确认")
		row.DefaultSourceID, row.DefaultSourceName = resolveReferenceOption(latestNonEmptyRaw(observations, func(raw RawOrder) string { return raw.OrderSourceRaw }), options.Sources, false)
		row.DefaultOrderTypeID, row.DefaultOrderTypeName = resolveReferenceOption(latestNonEmptyRaw(observations, func(raw RawOrder) string { return raw.OrderTypeRaw }), options.OrderTypes, false)
		row.ResponsibleEmployeeID, row.ResponsibleEmployeeName = resolveReferenceOption("", options.Employees, true)
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

	issues := make([]Issue, 0)
	if len(targets) > 1 {
		issues = append(issues, newIssue("customer_import", row.CandidateKey, "customer_target_erp_ambiguous", "error", "历史号码匹配到多个生产ERP客户，需要人工确认", *latest.row))
	}
	if len(observations) > 0 && len(observations[0].phones) > 1 {
		issues = append(issues, newIssue("customer_import", row.CandidateKey, "customer_latest_phone_ambiguous", "warning", "最近客户记录中包含多个手机号，主手机号需要人工确认", *latest.row))
	}
	return row, issues
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

func uniqueReferenceMatch(phones []string, byPhone map[string][]ERPReferenceCustomer) (*ERPReferenceCustomer, []int64) {
	byID := map[int64]ERPReferenceCustomer{}
	for _, phone := range phones {
		for _, ref := range byPhone[phone] {
			byID[ref.ID] = ref
		}
	}
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

func unionInt64IndexGroups(set *disjointSet, groups map[int64][]int) {
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
