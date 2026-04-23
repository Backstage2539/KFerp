package ports

import "context"

type Actor struct {
	Login      string
	EmployeeID int64
	Name       string
}

type Authenticator interface {
	AuthenticateBasic(ctx context.Context, user, pass string) (Actor, error)
	AuthenticateBearer(ctx context.Context, token string) (Actor, error)
}

type SessionResolver interface {
	ResolveSession(ctx context.Context, token string) (Actor, error)
}
