package freight

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/mozillazg/go-pinyin"
	"sort"
	"strings"
)

//go:embed regions/area-data-2.2.0.json
var regionData []byte

const RegionVersion = "@vant/area-data@2.2.0"

type Region struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	ParentCode string `json:"parent_code"`
	Level      int    `json:"level"`
	Alias      string `json:"alias"`
	Pinyin     string `json:"pinyin"`
	Initials   string `json:"initials"`
	Path       string `json:"path"`
}

var regionIndex map[string]Region
var regions []Region

func init() {
	var data struct {
		Provinces map[string]string `json:"province_list"`
		Cities    map[string]string `json:"city_list"`
		Counties  map[string]string `json:"county_list"`
	}
	if err := json.Unmarshal(regionData, &data); err != nil {
		panic(err)
	}
	regionIndex = map[string]Region{}
	for level, m := range []map[string]string{data.Provinces, data.Cities, data.Counties} {
		codes := []string{}
		for c := range m {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		for _, c := range codes {
			r := Region{Code: c, Name: m[c], Level: level + 1}
			if level == 1 {
				r.ParentCode = c[:2] + "0000"
			}
			if level == 2 {
				r.ParentCode = c[:4] + "00"
			}
			r.Alias = r.Name
			for _, s := range []string{"特别行政区", "维吾尔自治区", "壮族自治区", "回族自治区", "自治区", "省", "市"} {
				r.Alias = strings.TrimSuffix(r.Alias, s)
			}
			a := pinyin.NewArgs()
			a.Style = pinyin.Normal
			py := pinyin.Pinyin(r.Name, a)
			for _, sy := range py {
				if len(sy) > 0 {
					r.Pinyin += sy[0]
					if len(sy[0]) > 0 {
						r.Initials += sy[0][:1]
					}
				}
			}
			r.Path = r.Name
			if parent, ok := regionIndex[r.ParentCode]; ok {
				r.Path = parent.Path + " / " + r.Name
			}
			regionIndex[c] = r
			regions = append(regions, r)
		}
	}
}
func Regions() []Region { return append([]Region(nil), regions...) }
func SearchRegions(s string) []Region {
	s = strings.ToLower(strings.TrimSpace(s))
	out := []Region{}
	for _, r := range regions {
		if s == "" || strings.Contains(strings.ToLower(r.Name+" "+r.Alias+" "+r.Pinyin+" "+r.Initials+" "+r.Code), s) {
			out = append(out, r)
		}
	}
	return out
}
func ResolveDestination(d Destination) (Destination, error) {
	out := Destination{}
	parent := ""
	for i, v := range []struct{ code, name string }{{d.ProvinceCode, d.Province}, {d.CityCode, d.City}, {d.DistrictCode, d.District}} {
		if v.code == "" && strings.TrimSpace(v.name) == "" {
			continue
		}
		matches := []Region{}
		for _, r := range regions {
			if r.Level != i+1 {
				continue
			}
			if parent != "" && !strings.HasPrefix(r.Code, parent) {
				continue
			}
			if v.code != "" {
				if r.Code != v.code {
					continue
				}
				if v.name != "" && r.Name != v.name && r.Alias != v.name {
					return out, fmt.Errorf("地区代码与名称不一致")
				}
			} else if r.Name != strings.TrimSpace(v.name) && r.Alias != strings.TrimSpace(v.name) {
				continue
			}
			matches = append(matches, r)
		}
		if len(matches) != 1 {
			return out, fmt.Errorf("收件地区无法唯一识别，请选择省市区")
		}
		r := matches[0]
		switch i {
		case 0:
			out.Province, out.ProvinceCode = r.Name, r.Code
			parent = r.Code[:2]
		case 1:
			out.City, out.CityCode = r.Name, r.Code
			parent = r.Code[:4]
		case 2:
			out.District, out.DistrictCode = r.Name, r.Code
		}
	}
	if out.ProvinceCode == "" {
		if out.DistrictCode != "" {
			out.CityCode = out.DistrictCode[:4] + "00"
		}
		if out.CityCode != "" {
			out.ProvinceCode = out.CityCode[:2] + "0000"
			out.Province = regionIndex[out.ProvinceCode].Name
			out.City = regionIndex[out.CityCode].Name
		}
	}
	if out.ProvinceCode == "" {
		return out, fmt.Errorf("请选择收件省市区")
	}
	return out, nil
}
