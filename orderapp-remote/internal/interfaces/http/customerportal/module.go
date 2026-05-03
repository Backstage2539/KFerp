package customerportal

import (
	"context"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error)
	Me(context.Context, string) (customerportalapp.CurrentContext, error)
	SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error)
}

type Dependencies struct {
	CustomerPortal Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerMiniAPI(e, deps.CustomerPortal)
}

type StaticIdentityProvider struct{}

func (StaticIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	return customerportalapp.MiniIdentity{OpenID: "dev-openid-" + code}, nil
}

type DisabledIdentityProvider struct{}

func (DisabledIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	return customerportalapp.MiniIdentity{}, customerportalapp.ErrMiniLoginDisabled
}
