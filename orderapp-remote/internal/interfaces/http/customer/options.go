package customer

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type apiOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type option struct {
	ID   int64
	Name string
}

func fetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]option, 0)
	for rows.Next() {
		var o option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

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
