package catalog

import (
	"strings"

	pinyin "github.com/mozillazg/go-pinyin"
)

// SearchPinyin returns full pinyin for product search.
func SearchPinyin(s string) string {
	return pinyinToken(s, pinyin.Normal)
}

// SearchInitials returns first-letter pinyin for product search.
func SearchInitials(s string) string {
	return pinyinToken(s, pinyin.FirstLetter)
}

func pinyinToken(s string, style int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	a := pinyin.NewArgs()
	a.Style = style
	ps := pinyin.Pinyin(s, a)
	buf := make([]string, 0, len(ps))
	for _, arr := range ps {
		if len(arr) == 0 {
			continue
		}
		buf = append(buf, arr[0])
	}
	return strings.ToLower(strings.Join(buf, ""))
}
