package production

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
)

// A database retry retains the original request and version. A committed request
// is replayed; changed stock produces a refresh conflict instead of new quantities.
func retryOperationTransaction[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var out T
	var err error
	for i := 0; i < 3; i++ {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out, err = fn()
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || (pgerr.Code != "40001" && pgerr.Code != "40P01") {
			return out, err
		}
	}
	return out, err
}
