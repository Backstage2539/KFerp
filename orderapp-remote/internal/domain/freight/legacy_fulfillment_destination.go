package freight

import "strings"

// NormalizeLegacyFulfillmentDestination handles the historical address-book
// shape where a county-level city was saved as city and its locality as district.
// It is deliberately limited to an unambiguous city and a locality bearing the
// city's name; other invalid addresses remain subject to normal validation.
func NormalizeLegacyFulfillmentDestination(d Destination) (Destination, bool) {
	if d.ProvinceCode != "" || d.CityCode != "" || d.DistrictCode != "" {
		return d, false
	}
	province, city, locality := strings.TrimSpace(d.Province), strings.TrimSpace(d.City), strings.TrimSpace(d.District)
	if !strings.HasSuffix(city, "市") || locality == "" {
		return d, false
	}
	p, err := ResolveDestination(Destination{Province: province})
	if err != nil {
		return d, false
	}
	// A valid city-level field is never treated as a misplaced county.
	if _, err = ResolveDestination(Destination{Province: province, City: city}); err == nil {
		return d, false
	}
	var candidate Region
	count := 0
	for _, region := range regions {
		if region.Level == 3 && strings.HasPrefix(region.Code, p.ProvinceCode[:2]) && region.Name == city {
			candidate = region
			count++
		}
	}
	if count != 1 {
		return d, false
	}
	prefix := strings.TrimSuffix(city, "市")
	if len([]rune(prefix)) < 2 || !strings.HasPrefix(locality, prefix) || locality == prefix {
		return d, false
	}
	// Do not reinterpret an explicit different county, even if a locality is
	// prefixed by the selected city's short name.
	for _, region := range regions {
		if region.Level != 3 || region.Code == candidate.Code {
			continue
		}
		if strings.Contains(locality, region.Name) || region.Alias != region.Name && strings.Contains(locality, region.Alias) {
			return d, false
		}
	}
	return destinationForRegion(candidate), true
}
