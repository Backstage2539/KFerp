package customerportal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrPhoneCredentialUsed = errors.New("手机号凭证已使用，请重新授权")

type RegistrationProfile struct {
	MiniUserID    int64     `json:"mini_user_id"`
	Nickname      string    `json:"nickname"`
	VerifiedPhone string    `json:"verified_phone"`
	VerifiedAt    time.Time `json:"verified_at"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	Active        bool      `json:"active"`
}

func ValidRegistrationNickname(s string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n > 0 && n <= 32
}
func PhoneCredentialHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

type registrationRepository interface {
	UpdateRegistrationNickname(context.Context, int64, string) error
	ListRegistrations(context.Context, string, int, int) ([]RegistrationProfile, int, error)
	DisableRegistration(context.Context, int64, string) error
}

func (s *Service) UpdateRegistration(ctx context.Context, token, nickname string) (CurrentContext, error) {
	cur, err := s.Me(ctx, token)
	if err != nil {
		return CurrentContext{}, err
	}
	if !cur.RegistrationComplete || !ValidRegistrationNickname(nickname) {
		return CurrentContext{}, ErrMiniInvalidLogin
	}
	r, ok := s.repo.(registrationRepository)
	if !ok {
		return CurrentContext{}, ErrMiniLoginDisabled
	}
	if err = r.UpdateRegistrationNickname(ctx, cur.MiniUserID, strings.TrimSpace(nickname)); err != nil {
		return CurrentContext{}, err
	}
	return s.Me(ctx, token)
}
func (s *Service) ListRegistrations(ctx context.Context, q string, page, size int) ([]RegistrationProfile, int, error) {
	r, ok := s.repo.(registrationRepository)
	if !ok {
		return nil, 0, ErrMiniLoginDisabled
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 30
	}
	return r.ListRegistrations(ctx, strings.TrimSpace(q), page, size)
}
func (s *Service) DisableRegistration(ctx context.Context, id int64, actor string) error {
	r, ok := s.repo.(registrationRepository)
	if !ok {
		return ErrMiniLoginDisabled
	}
	if id <= 0 {
		return ErrMiniInvalidLogin
	}
	return r.DisableRegistration(ctx, id, actor)
}
