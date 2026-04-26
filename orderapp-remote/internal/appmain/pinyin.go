package appmain

import catalogdomain "orderapp/internal/domain/catalog"

func pinyinFull(s string) string {
	return catalogdomain.SearchPinyin(s)
}

func pinyinInitials(s string) string {
	return catalogdomain.SearchInitials(s)
}
