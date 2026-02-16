package main

func maxLen(ss ...[]string) int {
	m := 0
	for _, s := range ss {
		if len(s) > m {
			m = len(s)
		}
	}
	return m
}
