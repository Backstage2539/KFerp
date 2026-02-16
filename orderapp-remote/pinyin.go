package main

import (
	"strings"

	pinyin "github.com/mozillazg/go-pinyin"
)

func pinyinFull(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
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

func pinyinInitials(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	a := pinyin.NewArgs()
	a.Style = pinyin.FirstLetter
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
