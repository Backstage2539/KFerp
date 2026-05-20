package authz

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const AdminRoleCode = "admin"

type Role struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
}

type Actor struct {
	EmployeeID      int64             `json:"employee_id"`
	Name            string            `json:"name"`
	AccountType     string            `json:"account_type"`
	Roles           []Role            `json:"roles"`
	Permissions     []string          `json:"permissions"`
	ViewPermissions map[string]string `json:"-"`
	AllowedViewKeys []string          `json:"allowed_views"`
	BasicAuthAdmin  bool              `json:"basic_auth_admin"`
}

func (a Actor) IsAdmin() bool {
	if a.BasicAuthAdmin {
		return true
	}
	for _, role := range a.Roles {
		if strings.EqualFold(strings.TrimSpace(role.Code), AdminRoleCode) {
			return true
		}
	}
	return false
}

func (a Actor) Can(permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return true
	}
	if a.IsAdmin() {
		return true
	}
	for _, p := range a.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func (a Actor) CanView(viewKey string) bool {
	viewKey = strings.TrimSpace(viewKey)
	if viewKey == "" {
		return true
	}
	if a.IsAdmin() {
		return true
	}
	required := strings.TrimSpace(a.ViewPermissions[viewKey])
	if required == "" {
		return false
	}
	return a.Can(required)
}

func (a Actor) AllowedViews() []string {
	if a.IsAdmin() {
		return nil
	}
	out := make([]string, 0, len(a.ViewPermissions))
	for viewKey := range a.ViewPermissions {
		if a.CanView(viewKey) {
			out = append(out, viewKey)
		}
	}
	sort.Strings(out)
	return out
}

type AssignmentCommand struct {
	EmployeeID int64    `json:"employee_id"`
	RoleCodes  []string `json:"role_codes"`
}

func (cmd AssignmentCommand) Normalized() (AssignmentCommand, error) {
	if cmd.EmployeeID <= 0 {
		return AssignmentCommand{}, fmt.Errorf("employee required")
	}
	seen := map[string]bool{}
	out := AssignmentCommand{EmployeeID: cmd.EmployeeID}
	for _, raw := range cmd.RoleCodes {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out.RoleCodes = append(out.RoleCodes, code)
	}
	sort.Strings(out.RoleCodes)
	return out, nil
}

type Repository interface {
	ActorByEmployeeID(ctx context.Context, employeeID int64) (Actor, error)
	ListRoles(ctx context.Context) ([]Role, error)
	ListEmployeeRoles(ctx context.Context) (map[int64][]string, error)
	AssignEmployeeRoles(ctx context.Context, cmd AssignmentCommand) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ActorByEmployeeID(ctx context.Context, employeeID int64) (Actor, error) {
	if employeeID <= 0 {
		return Actor{}, fmt.Errorf("employee required")
	}
	actor, err := s.repo.ActorByEmployeeID(ctx, employeeID)
	if err != nil {
		return Actor{}, err
	}
	actor.EmployeeID = employeeID
	return normalizeActor(actor), nil
}

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *Service) AssignEmployeeRoles(ctx context.Context, cmd AssignmentCommand) error {
	normalized, err := cmd.Normalized()
	if err != nil {
		return err
	}
	return s.repo.AssignEmployeeRoles(ctx, normalized)
}

func (s *Service) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	return s.repo.ListEmployeeRoles(ctx)
}

func normalizeActor(actor Actor) Actor {
	actor.Name = strings.TrimSpace(actor.Name)
	actor.AccountType = strings.TrimSpace(actor.AccountType)
	if actor.AccountType == "" {
		actor.AccountType = "internal_employee"
	}
	permissions := map[string]bool{}
	for i := range actor.Roles {
		actor.Roles[i].Code = strings.ToLower(strings.TrimSpace(actor.Roles[i].Code))
		actor.Roles[i].Name = strings.TrimSpace(actor.Roles[i].Name)
		for _, raw := range actor.Roles[i].Permissions {
			permission := strings.TrimSpace(raw)
			if permission != "" {
				permissions[permission] = true
			}
		}
	}
	for _, raw := range actor.Permissions {
		permission := strings.TrimSpace(raw)
		if permission != "" {
			permissions[permission] = true
		}
	}
	actor.Permissions = actor.Permissions[:0]
	for permission := range permissions {
		actor.Permissions = append(actor.Permissions, permission)
	}
	sort.Strings(actor.Permissions)
	actor.AllowedViewKeys = actor.AllowedViews()
	return actor
}
