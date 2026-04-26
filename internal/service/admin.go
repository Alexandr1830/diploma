package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"diploma/internal/models"
	"diploma/internal/repository"
)

var ErrUserNotFound = errors.New("user not found")

type adminService struct {
	users repository.UserRepository
}

func NewAdminService(users repository.UserRepository) AdminService {
	return &adminService{users: users}
}

func (s *adminService) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	_, err := s.users.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("adminService.CreateUser: check email: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("adminService.CreateUser: hash: %w", err)
	}

	u := &models.User{
		Name:               req.Name,
		Email:              req.Email,
		PasswordHash:       string(hash),
		Role:               req.Role,
		IsActive:           true,
		MustChangePassword: true,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("adminService.CreateUser: %w", err)
	}
	return u, nil
}

func (s *adminService) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.users.List(ctx)
}

// UpdateUser изменяет имя/email/роль существующего пользователя.
// Активность и пароль меняются другими эндпоинтами (SetUserActive,
// ResetPassword) — здесь только профиль.
func (s *adminService) UpdateUser(ctx context.Context, id int64, req models.UpdateUserAdminRequest) (*models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("adminService.UpdateUser: load: %w", err)
	}

	// Если меняется email — убедимся, что он не занят другим пользователем.
	if req.Email != user.Email {
		existing, err := s.users.GetByEmail(ctx, req.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("adminService.UpdateUser: check email: %w", err)
		}
		if existing != nil && existing.ID != id {
			return nil, ErrEmailTaken
		}
	}

	user.Name = req.Name
	user.Email = req.Email
	user.Role = req.Role
	if err := s.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("adminService.UpdateUser: %w", err)
	}
	return user, nil
}

func (s *adminService) SetUserActive(ctx context.Context, id int64, active bool) error {
	if _, err := s.users.GetByID(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("adminService.SetUserActive: %w", err)
	}
	return s.users.SetActive(ctx, id, active)
}

func (s *adminService) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	if _, err := s.users.GetByID(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("adminService.ResetPassword: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("adminService.ResetPassword: hash: %w", err)
	}
	return s.users.UpdatePassword(ctx, id, string(hash), true)
}
