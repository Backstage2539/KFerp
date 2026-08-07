package customer

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRecipientTextMatchesERPAddressParserExamples(t *testing.T) {
	tests := []struct {
		name string
		text string
		want RecipientParseResult
	}{
		{
			name: "labeled multiline recipient block",
			text: "\n收件人：张三\n电话：13800138000\n地址：云南省普洱市思茅区咖啡路 88 号\n",
			want: RecipientParseResult{
				RecipientName: "张三",
				Phone:         "13800138000",
				Address:       "云南省普洱市思茅区咖啡路 88 号",
				Province:      "云南省",
				City:          "普洱市",
				District:      "思茅区",
				DetailAddress: "咖啡路 88 号",
			},
		},
		{
			name: "wechat common recipient mobile region and detail labels",
			text: "收货人：张三\n手机号码：13800138000\n所在地区：云南省 普洱市 思茅区\n详细地址：咖啡路 88 号",
			want: RecipientParseResult{
				RecipientName: "张三",
				Phone:         "13800138000",
				Address:       "云南省 普洱市 思茅区 咖啡路 88 号",
				Province:      "云南省",
				City:          "普洱市",
				District:      "思茅区",
				DetailAddress: "咖啡路 88 号",
			},
		},
		{
			name: "compact name phone address",
			text: "李四 13900139000 浙江省杭州市西湖区文三路 10 号",
			want: RecipientParseResult{RecipientName: "李四", Phone: "13900139000", Address: "浙江省杭州市西湖区文三路 10 号", Province: "浙江省", City: "杭州市", District: "西湖区", DetailAddress: "文三路 10 号"},
		},
		{
			name: "address phone name",
			text: "云南省昆明市西山区西坝新村30号C区 15302787466 刘祎泊",
			want: RecipientParseResult{RecipientName: "刘祎泊", Phone: "15302787466", Address: "云南省昆明市西山区西坝新村30号C区", Province: "云南省", City: "昆明市", District: "西山区", DetailAddress: "西坝新村30号C区"},
		},
		{
			name: "address name receiver marker phone",
			text: "四川省攀枝花市东区炳草岗湖滨路30号4栋 郑莉 收 18608120905",
			want: RecipientParseResult{RecipientName: "郑莉", Phone: "18608120905", Address: "四川省攀枝花市东区炳草岗湖滨路30号4栋", Province: "四川省", City: "攀枝花市", District: "东区", DetailAddress: "炳草岗湖滨路30号4栋"},
		},
		{
			name: "numeric customer name before phone",
			text: "QA浏览器客户20260616 13900001616 上海市浦东新区测试路16号",
			want: RecipientParseResult{RecipientName: "QA浏览器客户20260616", Phone: "13900001616", Address: "上海市浦东新区测试路16号", Province: "上海市", City: "上海市", District: "浦东新区", DetailAddress: "测试路16号"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRecipientText(tc.text)
			if err != nil {
				t.Fatalf("ParseRecipientText: %v", err)
			}
			if got != tc.want {
				t.Fatalf("result=%+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseRecipientTextSplitsDirectMunicipalityAndKeepsFullAddress(t *testing.T) {
	got, err := ParseRecipientText("刘五 13700137000 上海市浦东新区测试路16号")
	if err != nil {
		t.Fatalf("ParseRecipientText: %v", err)
	}
	if got.Address != "上海市浦东新区测试路16号" || got.Province != "上海市" || got.City != "上海市" || got.District != "浦东新区" || got.DetailAddress != "测试路16号" {
		t.Fatalf("result=%+v", got)
	}
}

func TestParseRecipientTextRejectsEmptyAndOverlongInput(t *testing.T) {
	if _, err := ParseRecipientText(" \n\t "); !errors.Is(err, ErrRecipientTextRequired) {
		t.Fatalf("empty error=%v, want ErrRecipientTextRequired", err)
	}
	if _, err := ParseRecipientText(strings.Repeat("收", MaxRecipientTextRunes+1)); !errors.Is(err, ErrRecipientTextTooLong) {
		t.Fatalf("overlong error=%v, want ErrRecipientTextTooLong", err)
	}
}
