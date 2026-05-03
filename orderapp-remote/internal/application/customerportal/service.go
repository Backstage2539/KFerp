package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	CapabilityBeanList         = "bean_list"
	CapabilityProductOrder     = "product_order"
	CapabilityDirectShip       = "direct_ship"
	CapabilityProcessing       = "processing"
	CapabilityInventoryCustody = "inventory_custody"
	CapabilityShippingQuery    = "shipping_query"
	CapabilitySettlement       = "settlement"
)

var (
	ErrCustomerBindingNotFound = errors.New("customer binding not found")
	ErrMiniSessionNotFound     = errors.New("mini session not found")
	ErrMiniLoginDisabled       = errors.New("mini login disabled")
)

type LoginCommand struct {
	Code     string
	Phone    string
	Nickname string
}

type MiniIdentity struct {
	OpenID  string
	UnionID string
}

type CreateLoginSessionCommand struct {
	OpenID   string
	UnionID  string
	Phone    string
	Nickname string
}

type LoginResult struct {
	Token             string            `json:"token"`
	MiniUserID        int64             `json:"mini_user_id"`
	CurrentCustomerID int64             `json:"current_customer_id"`
	Bindings          []CustomerBinding `json:"bindings"`
	Capabilities      []Capability      `json:"capabilities"`
}

type CustomerBinding struct {
	CustomerID   int64  `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
}

type Capability struct {
	Code    string         `json:"code"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

type CurrentContext struct {
	MiniUserID          int64             `json:"mini_user_id"`
	CurrentCustomerID   int64             `json:"current_customer_id"`
	CurrentCustomerName string            `json:"current_customer_name"`
	Bindings            []CustomerBinding `json:"bindings"`
	Capabilities        []Capability      `json:"capabilities"`
}

func (c CurrentContext) HasCapability(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range c.Capabilities {
		if capability.Enabled && capability.Code == code {
			return true
		}
	}
	return false
}

type IdentityProvider interface {
	Resolve(ctx context.Context, code string) (MiniIdentity, error)
}

type Repository interface {
	CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error)
	CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error)
	SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error)
}

type Service struct {
	repo     Repository
	identity IdentityProvider
}

func NewService(repo Repository, identity IdentityProvider) *Service {
	return &Service{repo: repo, identity: identity}
}

func (s *Service) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	code := strings.TrimSpace(cmd.Code)
	if code == "" {
		return LoginResult{}, fmt.Errorf("code required")
	}
	if s.repo == nil {
		return LoginResult{}, fmt.Errorf("repository required")
	}
	if s.identity == nil {
		return LoginResult{}, fmt.Errorf("identity provider required")
	}
	identity, err := s.identity.Resolve(ctx, code)
	if err != nil {
		return LoginResult{}, err
	}
	identity.OpenID = strings.TrimSpace(identity.OpenID)
	if identity.OpenID == "" {
		return LoginResult{}, fmt.Errorf("openid required")
	}
	return s.repo.CreateLoginSession(ctx, CreateLoginSessionCommand{
		OpenID:   identity.OpenID,
		UnionID:  strings.TrimSpace(identity.UnionID),
		Phone:    strings.TrimSpace(cmd.Phone),
		Nickname: strings.TrimSpace(cmd.Nickname),
	})
}

func (s *Service) Me(ctx context.Context, token string) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	return s.repo.CurrentContextByToken(ctx, token)
}

func (s *Service) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if customerID <= 0 {
		return CurrentContext{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	return s.repo.SwitchCurrentCustomer(ctx, token, customerID)
}
