package production

import (
	"sort"
	"strconv"
	"strings"
)

func normalizeMachineLoadSettings(raw string, minRoastG, maxRoastG int64) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	seen := map[int64]bool{}
	out := make([]int64, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil || v < minRoastG || v > maxRoastG {
			return "", false
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	parts := make([]string, 0, len(out))
	for _, v := range out {
		parts = append(parts, strconv.FormatInt(v, 10))
	}
	return strings.Join(parts, ","), true
}
