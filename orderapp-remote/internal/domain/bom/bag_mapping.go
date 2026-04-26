package bom

import "strings"

type BagSpecMapping struct {
	SpecG        int64  `json:"spec_g"`
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
}

func MappingNameBySpec(mappings []BagSpecMapping) map[int64]string {
	out := map[int64]string{}
	for _, m := range mappings {
		name := strings.TrimSpace(m.MaterialName)
		if m.SpecG > 0 && name != "" {
			out[m.SpecG] = name
		}
	}
	return out
}
