package orderliststaging

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestAssignSourceKeysAppendsOneDigitAndReusesFingerprintMapping(t *testing.T) {
	rows := make([]RawOrder, 9)
	for i := range rows {
		rows[i] = RawOrder{
			SheetName:        "2026年5月",
			SourceRowNumber:  3 + i,
			SequenceOriginal: "119",
			Fingerprint:      "fingerprint-" + string(rune('a'+i)),
		}
	}

	issues := AssignSourceKeys(rows, nil)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	want := []string{"119", "1191", "1192", "1193", "1194", "1195", "1196", "1197", "1198"}
	for i := range rows {
		if rows[i].SequenceEffective != want[i] {
			t.Fatalf("row %d effective sequence=%q want=%q", i, rows[i].SequenceEffective, want[i])
		}
		if rows[i].SourceOrderKey != "2026年5月:"+want[i] {
			t.Fatalf("row %d source key=%q", i, rows[i].SourceOrderKey)
		}
	}

	previous := map[string]SourceKeyAssignment{}
	for _, row := range rows {
		previous[row.Fingerprint] = SourceKeyAssignment{
			SheetName:         row.SheetName,
			OriginalSequence:  row.SequenceOriginal,
			EffectiveSequence: row.SequenceEffective,
			DuplicateSuffix:   row.DuplicateSuffix,
		}
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	for i := range rows {
		rows[i].SourceRowNumber = 3 + i
		rows[i].SequenceEffective = ""
		rows[i].SourceOrderKey = ""
		rows[i].DuplicateSuffix = 0
	}
	issues = AssignSourceKeys(rows, previous)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues after row movement: %+v", issues)
	}
	for _, row := range rows {
		if row.SequenceEffective != previous[row.Fingerprint].EffectiveSequence {
			t.Fatalf("fingerprint %s changed key to %s", row.Fingerprint, row.SequenceEffective)
		}
	}
}

func TestAssignSourceKeysRejectsSuffixCollisionAndMissingSequence(t *testing.T) {
	rows := []RawOrder{
		{SheetName: "2026年5月", SourceRowNumber: 3, SequenceOriginal: "119", Fingerprint: "a"},
		{SheetName: "2026年5月", SourceRowNumber: 4, SequenceOriginal: "119", Fingerprint: "b"},
		{SheetName: "2026年5月", SourceRowNumber: 5, SequenceOriginal: "1191", Fingerprint: "c"},
		{SheetName: "2026年5月", SourceRowNumber: 6, Fingerprint: "d"},
	}
	issues := AssignSourceKeys(rows, nil)
	if !hasIssueCode(issues, "duplicate_suffix_collision") {
		t.Fatalf("missing collision issue: %+v", issues)
	}
	if !hasIssueCode(issues, "source_sequence_missing") {
		t.Fatalf("missing sequence issue: %+v", issues)
	}
	if rows[1].SourceOrderKey != "" || rows[3].SourceOrderKey != "" {
		t.Fatalf("unresolved rows must not receive source keys: %+v", rows)
	}
}

func TestUnkeyedRowLocatorCannotCollideWithBusinessSourceKey(t *testing.T) {
	keyed := RawOrder{
		SheetName:       "2025年3月",
		SourceRowNumber: 34,
		SourceOrderKey:  "2025年3月:45",
		ProductRaw:      "红岩 1kg",
	}
	unkeyed := RawOrder{
		SheetName:       "2025年3月",
		SourceRowNumber: 45,
		ProductRaw:      "仅备注文字",
	}
	_, _, _, _, issues := CurateProducts([]RawOrder{keyed, unkeyed}, nil)
	seen := map[string]struct{}{}
	for _, issue := range issues {
		if _, exists := seen[issue.IssueKey]; exists {
			t.Fatalf("duplicate issue key from row locator collision: %+v", issues)
		}
		seen[issue.IssueKey] = struct{}{}
	}
	if rowLocator(unkeyed) == keyed.SourceOrderKey {
		t.Fatalf("row locator %q collides with business source key", rowLocator(unkeyed))
	}
}

func TestCurateCustomersGroupsPhoneAndUsesUniqueERPName(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年5月:1", CustomerRaw: "旧称 13800138000", OrderDate: "2026-05-01"},
		{SourceOrderKey: "2026年5月:2", CustomerRaw: "新称 138 0013 8000", OrderDate: "2026-05-20"},
		{SourceOrderKey: "2026年5月:3", CustomerRaw: "送货上门"},
	}
	refs := []ERPReferenceCustomer{{ID: 7, Name: "ERP规范客户", Phone: "13800138000", Active: true}}
	customers, aliases, phones, issues := CurateCustomers(rows, refs)
	if len(customers) != 2 {
		t.Fatalf("customers=%d want=2", len(customers))
	}
	var matched *Customer
	for i := range customers {
		if customers[i].NormalizedPhone == "13800138000" {
			matched = &customers[i]
		}
	}
	if matched == nil || matched.CanonicalName != "ERP规范客户" || matched.ERPMatchID != 7 {
		t.Fatalf("ERP match not applied: %+v", matched)
	}
	if len(aliases) < 2 || len(phones) != 1 {
		t.Fatalf("aliases=%d phones=%d", len(aliases), len(phones))
	}
	if !hasIssueCode(issues, "customer_phone_missing") {
		t.Fatalf("missing no-phone issue: %+v", issues)
	}
}

func TestCurateCustomersFlagsMultiplePhonesAndDuplicateERPPhone(t *testing.T) {
	rows := []RawOrder{{SourceOrderKey: "2026年5月:1", CustomerRaw: "甲 13800138000 乙 13900139000"}}
	refs := []ERPReferenceCustomer{
		{ID: 1, Name: "ERP甲", Phone: "13800138000", Active: true},
		{ID: 2, Name: "ERP乙", Phone: "13800138000", Active: true},
	}
	_, _, _, issues := CurateCustomers(rows, refs)
	if !hasIssueCode(issues, "customer_phone_multiple") {
		t.Fatalf("missing multi-phone issue: %+v", issues)
	}

	rows = []RawOrder{{SourceOrderKey: "2026年5月:2", CustomerRaw: "甲 13800138000"}}
	_, _, _, issues = CurateCustomers(rows, refs)
	if !hasIssueCode(issues, "erp_customer_phone_duplicate") {
		t.Fatalf("missing ERP duplicate issue: %+v", issues)
	}
}

func TestCurateCustomersFlagsAddressLikeCanonicalName(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年5月:1",
		CustomerRaw:    "联系人：山东省淄博市张店区华光路288号商管办公室 13800138000",
		OrderDate:      "2026-05-01",
	}}
	customers, _, _, issues := CurateCustomers(rows, nil)
	if len(customers) != 1 || customers[0].ReviewStatus != ReviewNeedsReview {
		t.Fatalf("address-like customer must need review: %+v", customers)
	}
	if !hasIssueCode(issues, "customer_name_needs_review") {
		t.Fatalf("missing address-like name issue: %+v", issues)
	}
}

func TestBuildCustomerImportRowsMergesReliableNameAndUsesLatestPhone(t *testing.T) {
	rows := []RawOrder{
		{
			SourceOrderKey: "2025年5月:1", SheetPeriod: "2025-05", SourceRowNumber: 12,
			CustomerRaw: "星河咖啡 13800138000", OrderDate: "2025-05-10",
			OrderSourceRaw: "历史销售", OrderTypeRaw: "产品订单",
		},
		{
			SourceOrderKey: "2026年5月:2", SheetPeriod: "2026-05", SourceRowNumber: 3,
			CustomerRaw: "星河咖啡 13900139000", OrderDate: "2026-05-20",
			OrderSourceRaw: "历史销售", OrderTypeRaw: "产品订单",
		},
	}
	options := CustomerImportOptions{
		Sources:       []ERPReferenceOption{{Value: "9", Label: "历史销售", Active: true}},
		OrderTypes:    []ERPReferenceOption{{Value: "3", Label: "产品订单", Active: true}},
		Employees:     []ERPReferenceOption{{Value: "7", Label: "负责人甲", Active: true}},
		CustomerTypes: []ERPReferenceOption{{Value: "wholesale", Label: "批发客户", Active: true}},
	}

	got, issues := BuildCustomerImportRows(rows, nil, nil, options)
	if len(got) != 1 {
		t.Fatalf("customer import rows=%d want=1, issues=%+v", len(got), issues)
	}
	row := got[0]
	if row.Name != "星河咖啡" || row.Phone != "13900139000" || row.CompanyPhone != row.Phone {
		t.Fatalf("latest customer identity not selected: %+v", row)
	}
	if row.PhoneCount != 2 || !strings.Contains(row.HistoricalPhones, "13800138000") || !strings.Contains(row.HistoricalPhones, "13900139000") {
		t.Fatalf("historical phones not preserved: %+v", row)
	}
	if row.LatestPhoneObservedDate != "2026-05-20" || row.FirstOrderDate != "2025-05-10" || row.LastOrderDate != "2026-05-20" {
		t.Fatalf("observation dates incorrect: %+v", row)
	}
	if row.DefaultSourceID != 9 || row.DefaultOrderTypeID != 3 || row.ResponsibleEmployeeID != 7 {
		t.Fatalf("ERP option resolution incorrect: %+v", row)
	}
	if row.CustomerType != "retail" || row.InferredCustomerType != "retail" || !row.Active || row.PortalEnabled {
		t.Fatalf("new customer defaults should stay reviewable: %+v", row)
	}
}

func TestBuildCustomerImportRowsUsesRecipientAsRetailCustomerWhenRemarkIsBlank(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年6月:1", SheetPeriod: "2026-06", SourceRowNumber: 3,
		CustomerRaw: "云南省昆明市西山区某路907罗小姐18788500415", OrderDate: "2026-06-23",
	}}
	got, issues := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || len(issues) != 0 {
		t.Fatalf("unexpected retail result: rows=%+v issues=%+v", got, issues)
	}
	row := got[0]
	if row.Name != "罗小姐" || row.CustomerType != "retail" || row.InferredCustomerType != "retail" {
		t.Fatalf("blank remark must create recipient retail customer: %+v", row)
	}
	if row.CustomerTypeBasis != "备注为空，按收件人识别零售客户" || row.DeliveryAddressCount != 1 {
		t.Fatalf("retail type evidence missing: %+v", row)
	}
}

func TestBuildCustomerImportRowsStripsBracketedRecipientLabel(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2025年9月:42", SheetPeriod: "2025-09", SourceRowNumber: 3,
		CustomerRaw: "【收件人】王小姐\n【联系方式】19129219429\n【收货地址】广东省佛山市禅城区某路1号", OrderDate: "2025-09-11",
	}}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || got[0].Name != "王小姐" || got[0].Contact != "王小姐" {
		t.Fatalf("bracketed recipient label not stripped: %+v", got)
	}
}

func TestBuildCustomerImportRowsUsesSingleAddressRemarkCustomerAsWholesale(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年6月:1", SheetPeriod: "2026-06", SourceRowNumber: 3, RemarkRaw: "星河咖啡订单", CustomerRaw: "张三 13800138000 云南省昆明市五华区某路1号", OrderDate: "2026-06-01"},
		{SourceOrderKey: "2026年6月:2", SheetPeriod: "2026-06", SourceRowNumber: 4, RemarkRaw: "星河咖啡", CustomerRaw: "张三，13800138000，云南省昆明市五华区某路1号", OrderDate: "2026-06-02"},
	}
	got, issues := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || len(issues) != 0 {
		t.Fatalf("unexpected wholesale result: rows=%+v issues=%+v", got, issues)
	}
	row := got[0]
	if row.Name != "星河咖啡" || row.CustomerType != "wholesale" || row.InferredCustomerType != "wholesale" {
		t.Fatalf("single-address remark customer must be wholesale: %+v", row)
	}
	if row.DeliveryAddressCount != 1 || !strings.Contains(row.CustomerTypeBasis, "1 个规范收件地址") {
		t.Fatalf("wholesale address evidence missing: %+v", row)
	}
}

func TestBuildCustomerImportRowsMarksRemarkCustomerWithoutAddressForReview(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年6月:1", SheetPeriod: "2026-06", SourceRowNumber: 3,
		RemarkRaw: "星河咖啡订单", OrderDate: "2026-06-01",
	}}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	row := got[0]
	if row.InferredCustomerType != "wholesale" || row.ReviewStatus != ReviewNeedsReview {
		t.Fatalf("missing-address remark customer must stay reviewable: %+v", row)
	}
	if !strings.Contains(row.CustomerTypeBasis, "未解析到规范收件地址") || !strings.Contains(row.ReviewReasons, "收件地址待确认") {
		t.Fatalf("missing-address evidence not explained: %+v", row)
	}
}

func TestBuildCustomerImportRowsMergesRemarkVariantsAndClassifiesMultipleAddressesAsChannel(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年6月:1", SheetPeriod: "2026-06", SourceRowNumber: 3, RemarkRaw: "慕恬标签", CustomerRaw: "刘甲 13800138000 云南省昆明市五华区甲路1号", OrderDate: "2026-06-01"},
		{SourceOrderKey: "2026年6月:2", SheetPeriod: "2026-06", SourceRowNumber: 4, RemarkRaw: "慕恬订单", CustomerRaw: "李乙 13900139000 云南省昆明市盘龙区乙路2号", OrderDate: "2026-06-02"},
	}
	got, issues := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || len(issues) != 0 {
		t.Fatalf("unexpected channel result: rows=%+v issues=%+v", got, issues)
	}
	row := got[0]
	if row.Name != "慕恬" || row.CustomerType != "channel" || row.InferredCustomerType != "channel" {
		t.Fatalf("remark variants with multiple addresses must be channel: %+v", row)
	}
	if row.DeliveryAddressCount != 2 || !strings.Contains(row.HistoricalRemarks, "慕恬标签") || !strings.Contains(row.HistoricalRemarks, "慕恬订单") {
		t.Fatalf("channel evidence missing: %+v", row)
	}
}

func TestBuildCustomerImportRowsDoesNotMergeRemarkCustomerWithRetailRecipientByPhone(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年6月:1", SheetPeriod: "2026-06", SourceRowNumber: 3, RemarkRaw: "星河咖啡订单", CustomerRaw: "张三 13800138000 云南省昆明市五华区某路1号", OrderDate: "2026-06-01"},
		{SourceOrderKey: "2026年6月:2", SheetPeriod: "2026-06", SourceRowNumber: 4, CustomerRaw: "张三 13800138000 云南省昆明市五华区某路1号", OrderDate: "2026-06-02"},
	}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 2 {
		t.Fatalf("remark customer and retail recipient must remain distinct: %+v", got)
	}
	types := map[string]bool{}
	for _, row := range got {
		types[row.InferredCustomerType] = true
	}
	if !types["wholesale"] || !types["retail"] {
		t.Fatalf("missing separate wholesale/retail candidates: %+v", got)
	}
}

func TestExtractRemarkCustomerNameHandlesBusinessPrefixesAndRejectsInstructions(t *testing.T) {
	for raw, want := range map[string]string{
		"8Am coffee订单": "8Am coffee",
		"王宝宝样品单：（记录数量月结）":        "王宝宝",
		"银辉客户代加工挂耳订单（生产共1312袋挂耳": "银辉",
		"誉观山：包装好需要填生产工单":         "誉观山",
		"橘子送货": "橘子",
		"1、曼辉咖啡\n2、挂耳需要打印二维码": "曼辉咖啡",
		"慕恬做库存":             "慕恬",
		"喜沁咖啡库存":            "喜沁咖啡",
		"刘豪朋友赠送":            "刘豪朋友",
		"贴NB标签，走顺丰":         "NB",
		"NB咖啡常用烘焙度，不贴标签":    "NB咖啡",
		"爆赤食堂补发货（批次问题召回）":   "爆赤食堂",
		"郎康达民宿、品类、烘焙度、生产日期": "郎康达民宿",
		"上海鲜烹一件代发":          "上海鲜烹",
		"橘子代发":              "橘子",
		"王宝宝咖啡豆袋货款":         "王宝宝",
		"斑雀咖啡 新版贴纸":         "斑雀咖啡",
		"贴翠微酒店LOGO":         "翠微酒店",
		"Boc参赛豆":            "Boc",
		"光宗豆号生豆":            "光宗豆号",
		"萃取液公司寄样":           "萃取液公司",
		"宋泊白汀标":             "宋泊白汀",
		"岩总单":               "岩总",
		"淘宝订单，随机赠送2袋挂耳":     "",
		"随机赠送4袋挂耳":          "",
		"样品":                "",
		"1.包装：白蓝色半磅包装":      "",
		"1️⃣包装用全白454g装袋子":   "",
		"⚠️不贴标签，白版发货":       "",
		"⚠️9.30库存（咖啡店发货）":   "",
		"4月7日烘焙":            "",
		"标签加logo":           "",
		"1kg包装":             "",
		"1.豆袋不贴标签":          "",
		"送100g酒心巧克力":        "",
		"随机送一袋挂耳":           "",
		"咖啡培训机构（可以发现货库存":    "",
		"不需要贴正面标签":          "",
		"发货人填，熊亚良":          "",
		"咖啡店发货":             "",
		"工厂发货":              "",
		"红盒子":               "",
		"蓝色盒子":              "",
		"昆明展会烘焙":            "",
		"有现货的话装水洗浅烘":        "",
		"需要打印logo":          "",
		"淘宝磨粉":              "",
		"一个红色手提袋":           "",
		"十袋装纸盒":             "",
		"定制logo":            "",
		"封口普通贴纸":            "",
		"补发":                "",
		"走京东快递":             "",
		"到付":                "",
		"快团团":               "",
		"旧账":                "",
		"每类":                "",
		"测试喷码机":             "",
		"顺丰到付":              "",
		"奶茶店":               "",
		"民宿":                "",
		"经销商":               "",
		"咖啡":                "",
		"咖啡生豆":              "",
		"定制挂耳":              "",
		"新豆子到再发":            "",
		"补录":                "",
		"参赛豆":               "",
		"培训老师的店":            "",
		"意式":                "",
		"拉萨经销商":             "",
		"甜点店":               "",
		"生豆":                "",
		"蓝盒":                "",
		"西双版纳线下店":           "",
		"货款免费":              "",
		"贴纸LOGO李婉琪发给小段":     "",
	} {
		if got := extractRemarkCustomerName(raw); got != want {
			t.Fatalf("extractRemarkCustomerName(%q)=%q want=%q", raw, got, want)
		}
	}
}

func TestBuildCustomerImportRowsUsesTopInsertedRowForSameDatePhone(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 8, CustomerRaw: "星河咖啡 13800138000", OrderDate: "2026-05-20"},
		{SourceOrderKey: "2026年5月:2", SheetPeriod: "2026-05", SourceRowNumber: 3, CustomerRaw: "星河咖啡 13900139000", OrderDate: "2026-05-20"},
	}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || got[0].Phone != "13900139000" || got[0].LatestSourceOrderKey != "2026年5月:2" {
		t.Fatalf("same-date latest inserted row not selected: %+v", got)
	}
}

func TestCustomerNameNeedsReviewFlagsRelationshipAliases(t *testing.T) {
	for _, raw := range []string{"刘豪朋友", "陈梁哥同事", "白三儿子"} {
		if !customerNameNeedsReview(raw) {
			t.Fatalf("relationship alias %q must need review", raw)
		}
	}
}

func TestBuildCustomerImportRowsUsesNewerSheetWhenLatestDateIsBlank(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2025年5月:1", SheetPeriod: "2025-05", SourceRowNumber: 3, CustomerRaw: "星河咖啡 13800138000", OrderDate: "2025-05-20"},
		{SourceOrderKey: "2026年5月:2", SheetPeriod: "2026-05", SourceRowNumber: 3, CustomerRaw: "星河咖啡 13900139000"},
	}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || got[0].Phone != "13900139000" || got[0].LatestSourceOrderKey != "2026年5月:2" {
		t.Fatalf("newer sheet must win when order date is blank: %+v", got)
	}
}

func TestBuildCustomerImportRowsDoesNotGuessUniqueSourceWhenLabelDiffers(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3,
		CustomerRaw: "星河咖啡 13800138000", OrderDate: "2026-05-01", OrderSourceRaw: "销售甲",
	}}
	options := CustomerImportOptions{Sources: []ERPReferenceOption{{Value: "1", Label: "小程序", Active: true}}}
	got, _ := BuildCustomerImportRows(rows, nil, nil, options)
	if len(got) != 1 || got[0].DefaultSourceID != 0 || got[0].DefaultSourceName != "" {
		t.Fatalf("unmatched source must remain blank for review: %+v", got)
	}
}

func TestBuildCustomerImportRowsDoesNotMergeShortNamesAcrossPhones(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3, CustomerRaw: "张三 13800138000", OrderDate: "2026-05-01"},
		{SourceOrderKey: "2026年5月:2", SheetPeriod: "2026-05", SourceRowNumber: 4, CustomerRaw: "张三 13900139000", OrderDate: "2026-05-02"},
	}
	got, issues := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 2 {
		t.Fatalf("short same-name rows must remain separate: got=%d rows=%+v", len(got), got)
	}
	for _, row := range got {
		if row.ReviewStatus != ReviewNeedsReview || row.PhoneCount != 1 {
			t.Fatalf("short-name row must need review: %+v", row)
		}
	}
	if !hasIssueCode(issues, "customer_cross_phone_name_unsafe") {
		t.Fatalf("missing unsafe-name issue: %+v", issues)
	}
}

func TestBuildCustomerImportRowsDoesNotMergeRepeatedNoPhoneInstructions(t *testing.T) {
	rows := []RawOrder{
		{SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3, CustomerRaw: "咖啡店自提", OrderDate: "2026-05-01"},
		{SourceOrderKey: "2026年5月:2", SheetPeriod: "2026-05", SourceRowNumber: 4, CustomerRaw: "咖啡店自提", OrderDate: "2026-05-02"},
	}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 2 {
		t.Fatalf("no-phone instructions must remain source-specific: got=%d rows=%+v", len(got), got)
	}
	for _, row := range got {
		if row.MergeMethod != "source_only" || row.ReviewStatus != ReviewNeedsReview {
			t.Fatalf("no-phone instruction must need review: %+v", row)
		}
	}
}

func TestBuildCustomerImportRowsCleansRecipientLabelAndPhoneFromName(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3,
		CustomerRaw: "收货人：张三 13800138000", OrderDate: "2026-05-01",
	}}
	got, _ := BuildCustomerImportRows(rows, nil, nil, CustomerImportOptions{})
	if len(got) != 1 || got[0].Name != "张三" {
		t.Fatalf("recipient label/phone not cleaned: %+v", got)
	}
}

func TestBuildCustomerImportRowsPreservesProductionERPFields(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3,
		CustomerRaw: "ERP旧别名 13800138000", OrderDate: "2026-05-01",
	}}
	target := []ERPReferenceCustomer{{
		ID: 88, Name: "ERP规范客户", RawName: "ERP原名", CustomerType: "channel",
		CompanyName: "ERP企业", CompanyAddress: "企业地址", CompanyPhone: "13800138000",
		Contact: "联系人", Phone: "13800138000", Address: "收货地址",
		DefaultSourceID: 1, DefaultOrderTypeID: 2, ResponsibleEmployeeID: 3,
		PortalEnabled: true, CapabilityTemplateKey: "channel_direct_ship", Active: true,
	}}
	got, issues := BuildCustomerImportRows(rows, nil, target, CustomerImportOptions{})
	if len(got) != 1 || len(issues) != 0 {
		t.Fatalf("unexpected result: rows=%+v issues=%+v", got, issues)
	}
	row := got[0]
	if row.Action != "update" || row.ERPMatchID != 88 || row.Name != "ERP规范客户" || row.RawName != "ERP原名" || row.CustomerType != "channel" {
		t.Fatalf("ERP identity fields not preserved: %+v", row)
	}
	if row.CompanyName != "ERP企业" || row.CompanyAddress != "企业地址" || row.Contact != "联系人" || row.Address != "收货地址" {
		t.Fatalf("ERP customer fields not preserved: %+v", row)
	}
	if !row.PortalEnabled || row.CapabilityTemplateKey != "channel_direct_ship" || !row.Active {
		t.Fatalf("ERP portal/status fields not preserved: %+v", row)
	}
	if row.InferredCustomerType != "retail" {
		t.Fatalf("historical inference should still be visible: %+v", row)
	}
}

func TestBuildCustomerImportRowsDoesNotReuseDirtyDevelopmentERPName(t *testing.T) {
	rows := []RawOrder{{
		SourceOrderKey: "2026年5月:1", SheetPeriod: "2026-05", SourceRowNumber: 3,
		CustomerRaw: "星河咖啡 13800138000", OrderDate: "2026-05-01",
	}}
	dev := []ERPReferenceCustomer{{
		ID: 7, Name: "某省某市某区某路100号收货地址 13800138000", Phone: "13800138000", Active: true,
	}}
	got, _ := BuildCustomerImportRows(rows, dev, nil, CustomerImportOptions{})
	if len(got) != 1 || got[0].Name != "星河咖啡" {
		t.Fatalf("dirty development ERP name should not replace safe source name: %+v", got)
	}
}

func TestParseProductLineWeightAndPackageExamples(t *testing.T) {
	tests := []struct {
		raw         string
		wantParent  string
		wantSpec    string
		wantQty     float64
		wantUnit    string
		wantWeightG float64
		wantReview  bool
	}{
		{raw: "曜石 454g*24袋", wantParent: "曜石", wantSpec: "454g袋装", wantQty: 24, wantUnit: "袋", wantWeightG: 10896},
		{raw: "芒霜 2磅", wantParent: "芒霜", wantQty: 2, wantUnit: "lb", wantWeightG: 907.184},
		{raw: "红岩 15kg+839g", wantParent: "红岩", wantQty: 15.839, wantUnit: "kg", wantWeightG: 15839},
		{raw: "曜石100磅（2磅装50包）", wantParent: "曜石", wantSpec: "2lb包装", wantQty: 50, wantUnit: "包", wantWeightG: 45359.2},
		{raw: "小菠萝 10盒", wantParent: "小菠萝", wantQty: 10, wantUnit: "盒", wantWeightG: 0, wantReview: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := ParseProductLine(tt.raw)
			if got.ParentName != tt.wantParent || got.SpecName != tt.wantSpec || got.OrderUnit != tt.wantUnit || !almostEqual(got.OrderQuantity, tt.wantQty) || !almostEqual(got.NormalizedWeightG, tt.wantWeightG) || got.NeedsReview != tt.wantReview {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestParseProductLineRemovesListNumberAndDoesNotGuessMissingPackageUnit(t *testing.T) {
	got := ParseProductLine("1. 曜石 1kg*1袋")
	if got.ParentName != "曜石" || got.SpecName != "1kg袋装" || got.OrderQuantity != 1 || got.OrderUnit != "袋" {
		t.Fatalf("numbered product line=%+v", got)
	}
	got = ParseProductLine("曜石 454g*24")
	if !got.NeedsReview || got.OrderUnit != "" || got.ReviewReason != "数量未填写包装单位" {
		t.Fatalf("missing package unit must stay unresolved: %+v", got)
	}
}

func TestParseProductLineFlagsInstructionTextAsProductName(t *testing.T) {
	for _, raw := range []string{
		"【】精品二维码，联系方式 1kg",
		"有现货再发，没现货可以替换 2kg",
		"每款 1kg",
	} {
		got := ParseProductLine(raw)
		if !got.NeedsReview || got.ReviewReason != "无法识别父商品名称" {
			t.Fatalf("instruction text must need review: raw=%q got=%+v", raw, got)
		}
	}
}

func TestParseAmountAndDate(t *testing.T) {
	if got := ParseAmount("192+12"); got.Value == nil || *got.Value != 204 || !got.Derived {
		t.Fatalf("sum amount=%+v", got)
	}
	if got := ParseAmount("1024（一笔1109）"); got.Value != nil || !got.NeedsReview {
		t.Fatalf("annotated amount must remain unresolved: %+v", got)
	}
	if got := ParseAmount(""); got.Value != nil || got.Raw != "" {
		t.Fatalf("blank amount=%+v", got)
	}

	d, err := ParseDate("46173", "2026-05")
	if err != nil || d != "2026-05-31" {
		t.Fatalf("excel date=%q err=%v", d, err)
	}
	d, err = ParseDate("5月20日", "2026-05")
	if err != nil || d != "2026-05-20" {
		t.Fatalf("partial date=%q err=%v", d, err)
	}
}

func TestParseWorkbookSupportsBothMonthlyLayoutsAndScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.xlsx")
	wb := excelize.NewFile()
	defaultSheet := wb.GetSheetName(0)
	_ = wb.SetSheetName(defaultSheet, "2026年5月")
	writeRows(t, wb, "2026年5月", [][]any{
		{nil, nil, nil, "发货信息"},
		{"序号", "发货状态", "订单日期", "客户", "备注", "磨粉", "品种", "烘焙程度", "付款状态", "货款+运费（元）", "运费（元）", "收款时间", "订单来源", "快递费", "订单类型"},
		{2, "已发货", 46173, "客户甲 13800138000", nil, nil, "曜石 454g*2袋", nil, "微信收款", 200, 10, 46173, "销售甲", "已付", "产品订单"},
	})
	_, _ = wb.NewSheet("2025 年 4 月")
	writeRows(t, wb, "2025 年 4 月", [][]any{
		{nil, nil, nil, nil, nil, nil, nil, nil, nil, "发货信息"},
		{"序号", "发货状态", "订单日期", "订单来源", "订单类型", "付款状态", "货款（元）", "运费（元）", "收款时间", "客户", "备注", "是否磨粉", "品种"},
		{1, "已发货", 45779, "销售乙", "产品订单", "公账收款", 100, 5, 45779, "客户乙 13900139000", nil, nil, "红岩 2磅"},
	})
	_, _ = wb.NewSheet("2024年12月")
	writeRows(t, wb, "2024年12月", [][]any{{"序号"}, {1}})
	if err := wb.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	dataset, err := PrepareWorkbook(path, PrepareOptions{StartPeriod: "2025-01", EndPeriod: "2026-06"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.RawOrders) != 2 {
		t.Fatalf("raw orders=%d want=2", len(dataset.RawOrders))
	}
	if dataset.RawOrders[0].CustomerRaw == "" || dataset.RawOrders[1].ProductRaw == "" {
		t.Fatalf("layout fields not mapped: %+v", dataset.RawOrders)
	}
	var excluded bool
	for _, sheet := range dataset.Sheets {
		if sheet.SheetName == "2024年12月" && !sheet.Included {
			excluded = true
		}
	}
	if !excluded {
		t.Fatal("out-of-scope sheet was not inventoried as excluded")
	}
}

func TestStagingSchemaIsIsolatedAndIdempotent(t *testing.T) {
	sql := StagingSchemaSQL()
	for _, marker := range []string{
		"CREATE SCHEMA IF NOT EXISTS raw",
		"CREATE SCHEMA IF NOT EXISTS reference",
		"CREATE SCHEMA IF NOT EXISTS curated",
		"CREATE SCHEMA IF NOT EXISTS review",
		"UNIQUE (source_order_key)",
		"raw.order_revisions",
	} {
		if !strings.Contains(sql, marker) {
			t.Fatalf("schema missing %q", marker)
		}
	}
	for _, forbidden := range []string{"p2rms15pepb5ciz", "DROP DATABASE nocodb", "TRUNCATE nocodb"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("schema references formal database marker %q", forbidden)
		}
	}
}

func TestReviewDataContractIncludesAllAuditSections(t *testing.T) {
	dataset := Dataset{Run: ImportRun{RunID: "run-1", SourceSHA256: "abc", CreatedAt: time.Now().UTC()}}
	contract := BuildReviewContract(dataset)
	want := []string{"导入汇总", "序号映射", "客户候选", "客户导入审核", "客户别名", "父商品候选", "SKU规格", "订单候选", "订单明细", "ERP匹配建议", "待审核问题", "排除工作表"}
	if !reflect.DeepEqual(contract.SheetNames, want) {
		t.Fatalf("sheet names=%v want=%v", contract.SheetNames, want)
	}
}

func TestGenerateLoadSQLIsTransactionalIdempotentAndRevisionAware(t *testing.T) {
	amount := 204.0
	dataset := Dataset{
		Run:        ImportRun{RunID: "run-1", SourceSHA256: "abc", StartPeriod: "2025-01", EndPeriod: "2026-06", CreatedAt: time.Now().UTC()},
		Customers:  []Customer{{CustomerKey: "phone:13800138000", CanonicalName: "测试客户", NormalizedPhone: "13800138000", ReviewStatus: ReviewAutoReady}},
		Products:   []Product{{ProductKey: "product:1", CanonicalName: "曜石", ProductKind: "roasted_bean", ReviewStatus: ReviewAutoReady}},
		Orders:     []Order{{SourceOrderKey: "2026年5月:119", SourceFingerprint: "fp-new", CustomerKey: "phone:13800138000", AmountValue: &amount, ReviewStatus: ReviewAutoReady}},
		OrderItems: []OrderItem{{SourceItemKey: "2026年5月:119:1", SourceOrderKey: "2026年5月:119", LineNo: 1, ProductKey: "product:1", RawLine: "曜石2磅", ReviewStatus: ReviewAutoReady}},
	}
	sql, err := GenerateLoadSQL(dataset)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"BEGIN;",
		"INSERT INTO raw.order_revisions",
		"ON CONFLICT (source_order_key) DO UPDATE",
		"DELETE FROM curated.order_items",
		"COMMIT;",
	} {
		if !strings.Contains(sql, marker) {
			t.Fatalf("load SQL missing %q", marker)
		}
	}
	if strings.Contains(sql, "p2rms15pepb5ciz") || strings.Contains(sql, "nocodb.") {
		t.Fatal("load SQL must not reference the formal business database")
	}
}

func TestWriteExportsCreatesProtectedAuditFiles(t *testing.T) {
	dir := t.TempDir()
	dataset := Dataset{Run: ImportRun{RunID: "run-1", SourceSHA256: "abc", CreatedAt: time.Now().UTC()}}
	if err := WriteExports(dataset, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"dataset.json", "manifest.json", "source-key-mapping.json", "schema.sql", "load.sql",
		"sheet_inventory.csv", "raw_orders.csv", "customers.csv", "customer_import_review.csv", "products.csv", "skus.csv", "orders.csv", "order_items.csv", "issues.csv",
	} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("%s permissions=%o want no group/world access", name, info.Mode().Perm())
		}
	}
}

func TestCustomerImportCSVIncludesTypeInferenceEvidence(t *testing.T) {
	rows := customerImportRowsCSV([]CustomerImportRow{{
		CandidateKey:           "remark_customer:test",
		Name:                   "测试渠道",
		CustomerType:           "channel",
		InferredCustomerType:   "channel",
		CustomerTypeBasis:      "备注客户，2 个规范收件地址，判定渠道客户",
		RecipientNames:         "收件人甲 | 收件人乙",
		DeliveryAddressCount:   2,
		DeliveryAddressSamples: "地址甲 | 地址乙",
		LatestRemarkRaw:        "测试渠道订单",
		HistoricalRemarks:      "测试渠道订单 | 测试渠道补单",
	}})
	if len(rows) != 2 {
		t.Fatalf("csv rows=%d want=2", len(rows))
	}
	header := strings.Join(rows[0], "|")
	for _, want := range []string{"推断客户类型", "客户类型判定依据", "收件人名称", "规范收件地址数", "收件地址样本", "最新备注原文", "历史备注"} {
		if !strings.Contains(header, want) {
			t.Fatalf("customer import csv missing header %q: %s", want, header)
		}
	}
	values := strings.Join(rows[1], "|")
	for _, want := range []string{"channel", "2 个规范收件地址", "收件人甲", "地址甲", "测试渠道补单"} {
		if !strings.Contains(values, want) {
			t.Fatalf("customer import csv missing value %q: %s", want, values)
		}
	}
}

func hasIssueCode(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.001
}

func writeRows(t *testing.T, wb *excelize.File, sheet string, rows [][]any) {
	t.Helper()
	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := wb.SetCellValue(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
