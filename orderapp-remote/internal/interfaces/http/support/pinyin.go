package support

import catalogdomain "orderapp/internal/domain/catalog"

func PinyinFull(s string) string {
	return catalogdomain.SearchPinyin(s)
}

func PinyinInitials(s string) string {
	return catalogdomain.SearchInitials(s)
}
