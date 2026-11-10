package customerportal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

const previewPrefix = "admin-preview."
const previewTTL = 30 * time.Minute
const maxPreviewSessions = 5000

var ErrAdminPreviewDenied = errors.New("管理员只读预览不可用")

type AdminPreviewSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type previewSession struct {
	employeeToken          string
	employeeID, customerID int64
	expiresAt              time.Time
}
type previewSessions struct {
	mu      sync.Mutex
	entries map[string]previewSession
}
type previewRepository interface {
	AdminPreviewCustomer(context.Context, int64) (CurrentContext, error)
	AdminPreviewAudit(context.Context, int64, int64, string) error
}

func IsAdminPreviewToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), previewPrefix)
}
func isPreviewAdmin(c CurrentContext) bool {
	if c.AccountType != "employee" || c.EmployeeID <= 0 {
		return false
	}
	for _, role := range c.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

// StartAdminPreview never exposes the employee token; server memory is the
// authority. Restart or expiry invalidates all temporary preview credentials.
func (s *Service) StartAdminPreview(ctx context.Context, employeeToken string, customerID int64) (AdminPreviewSession, error) {
	if IsAdminPreviewToken(employeeToken) || customerID <= 0 || s.repo == nil {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	admin, err := s.repo.CurrentContextByToken(ctx, employeeToken)
	if err != nil || !isPreviewAdmin(admin) {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	if _, err = repo.AdminPreviewCustomer(ctx, customerID); err != nil {
		return AdminPreviewSession{}, err
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return AdminPreviewSession{}, err
	}
	token := previewPrefix + base64.RawURLEncoding.EncodeToString(random[:])
	now := time.Now()
	expires := now.Add(previewTTL)
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	if s.previews.entries == nil {
		s.previews.entries = make(map[string]previewSession)
	}
	for key, session := range s.previews.entries {
		if !now.Before(session.expiresAt) {
			delete(s.previews.entries, key)
		}
	}
	if len(s.previews.entries) >= maxPreviewSessions {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	if err = repo.AdminPreviewAudit(ctx, admin.EmployeeID, customerID, "enter"); err != nil {
		return AdminPreviewSession{}, err
	}
	s.previews.entries[token] = previewSession{employeeToken: employeeToken, employeeID: admin.EmployeeID, customerID: customerID, expiresAt: expires}
	return AdminPreviewSession{Token: token, ExpiresAt: expires}, nil
}

func (s *Service) lookupPreview(token string) (previewSession, bool) {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	p, ok := s.previews.entries[token]
	if !ok || !time.Now().Before(p.expiresAt) {
		delete(s.previews.entries, token)
		return previewSession{}, false
	}
	return p, true
}
func (s *Service) removePreview(token string) {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	delete(s.previews.entries, token)
}

func (s *Service) previewContext(ctx context.Context, token string) (CurrentContext, error) {
	p, ok := s.lookupPreview(token)
	if !ok {
		return CurrentContext{}, ErrMiniSessionNotFound
	}
	admin, err := s.repo.CurrentContextByToken(ctx, p.employeeToken)
	if err != nil || !isPreviewAdmin(admin) || admin.EmployeeID != p.employeeID {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	current, err := repo.AdminPreviewCustomer(ctx, p.customerID)
	if err != nil {
		return CurrentContext{}, err
	}
	if current.CurrentCustomerID != p.customerID {
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	current.AccountType = "preview"
	current.EmployeeID = 0
	current.EmployeeName = ""
	current.Roles = nil
	current.Permissions = nil
	current.Bindings = nil
	current.MiniUserID = 0
	current.RegistrationComplete = true
	if modes, ok := s.repo.(interface {
		FulfillmentModes(context.Context, int64) ([]string, error)
	}); ok {
		current.FulfillmentModes, err = modes.FulfillmentModes(ctx, p.customerID)
		if err != nil {
			return CurrentContext{}, err
		}
		current.Capabilities = ApplyFulfillmentCapabilityDependencies(current.Capabilities, current.FulfillmentModes)
	}
	return normalizeCurrentContext(current), nil
}

func (s *Service) EndAdminPreview(ctx context.Context, token string) error {
	p, ok := s.lookupPreview(token)
	if !ok {
		return ErrMiniSessionNotFound
	}
	admin, err := s.repo.CurrentContextByToken(ctx, p.employeeToken)
	if err != nil || !isPreviewAdmin(admin) || admin.EmployeeID != p.employeeID {
		s.removePreview(token)
		return ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return ErrAdminPreviewDenied
	}
	if err = repo.AdminPreviewAudit(ctx, p.employeeID, p.customerID, "exit"); err != nil {
		return err
	}
	s.removePreview(token)
	return nil
}
