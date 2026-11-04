package freight

import "testing"

func TestLegacyFulfillmentDestinationOnlyRepairsUnambiguousCountyCity(t *testing.T) {
	cases := []struct {
		name  string
		input Destination
		want  bool
	}{
		{"historic Dali", Destination{Province: "云南省", City: "大理市", District: "大理古城"}, true},
		{"valid address remains untouched", Destination{Province: "云南省", City: "大理白族自治州", District: "大理市"}, false},
		{"other county conflict", Destination{Province: "云南省", City: "大理市", District: "祥云县"}, false},
		{"prefixed county conflict", Destination{Province: "云南省", City: "大理市", District: "大理祥云县"}, false},
		{"unrelated unknown district", Destination{Province: "云南省", City: "大理市", District: "未知地区"}, false},
		{"wrong province", Destination{Province: "四川省", City: "大理市", District: "大理古城"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeLegacyFulfillmentDestination(tc.input)
			if ok != tc.want {
				t.Fatalf("repair = %v, want %v (%+v)", ok, tc.want, got)
			}
			if ok && (got.ProvinceCode != "530000" || got.CityCode != "532900" || got.DistrictCode != "532901" || got.City != "大理白族自治州" || got.District != "大理市") {
				t.Fatalf("unexpected canonical destination: %+v", got)
			}
		})
	}
}
