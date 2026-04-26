package service

import (
	"context"
	"fmt"

	"diploma/internal/models"
	"diploma/internal/repository"
)

type userListService struct {
	users repository.UserRepository
}

func NewUserListService(users repository.UserRepository) UserListService {
	return &userListService{users: users}
}

func (s *userListService) ListByRole(ctx context.Context, role models.UserRole) ([]models.UserShort, error) {
	users, err := s.users.ListByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("userListService.ListByRole: %w", err)
	}
	result := make([]models.UserShort, len(users))
	for i, u := range users {
		result[i] = models.UserShort{ID: u.ID, Name: u.Name}
	}
	return result, nil
}
