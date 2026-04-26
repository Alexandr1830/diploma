package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"diploma/internal/models"
	"diploma/internal/repository"
	"diploma/pkg/token"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSamePassword       = errors.New("new password must differ from current")
)

type authService struct {
	users     repository.UserRepository
	jwtSecret string
	tokenTTL  time.Duration
}

func NewAuthService(users repository.UserRepository, jwtSecret string, tokenTTL time.Duration) AuthService {
	return &authService{
		users:     users,
		jwtSecret: jwtSecret,
		tokenTTL:  tokenTTL,
	}
}

func (s *authService) Login(ctx context.Context, req models.LoginRequest) (string, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if !user.IsActive {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return "", ErrInvalidCredentials
	}

	t, err := token.Generate(user.ID, string(user.Role), s.jwtSecret, s.tokenTTL)
	if err != nil {
		return "", fmt.Errorf("authService.Login: generate token: %w", err)
	}
	return t, nil
}

func (s *authService) Me(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("authService.Me: %w", err)
	}
	return user, nil
}

func (s *authService) ChangePassword(ctx context.Context, userID int64, req models.ChangePasswordRequest) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("authService.ChangePassword: load user: %w", err)
	}
	// Если у пользователя must_change_password=true — это форсированная первая
	// смена. Старый (временный) пароль он уже ввёл при логине, повторно его
	// спрашивать не надо. В обычном сценарии (добровольная смена) старый
	// пароль обязателен.
	if !user.MustChangePassword {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
			return ErrInvalidCredentials
		}
	}
	// Защита от установки того же самого пароля. Покрывает оба сценария:
	// в обычной смене пользователь мог бы случайно ввести старый пароль в оба
	// поля, а в форсированной — повторно ввести временный.
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.NewPassword)) == nil {
		return ErrSamePassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("authService.ChangePassword: hash: %w", err)
	}
	return s.users.UpdatePassword(ctx, userID, string(hash), false)
}
