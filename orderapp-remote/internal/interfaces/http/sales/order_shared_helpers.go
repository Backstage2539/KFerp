package sales

func getStr(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}
