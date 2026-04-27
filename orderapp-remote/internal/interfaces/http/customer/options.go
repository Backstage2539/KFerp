package customer

import (
	customerapp "orderapp/internal/application/customer"
	"strings"
)

type apiOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type option = customerapp.Option

func apiOptions(in []option) []apiOption {
	out := make([]apiOption, 0, len(in))
	for _, o := range in {
		out = append(out, apiOption{ID: o.ID, Name: o.Name})
	}
	return out
}

func nullText(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}
