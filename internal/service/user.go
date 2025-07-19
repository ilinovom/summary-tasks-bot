package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/summary-tasks-bot/internal/model"
	"github.com/example/summary-tasks-bot/internal/repository"
)

type UserService struct {
	repo repository.UserSettingsRepository
}

func NewUserService(repo repository.UserSettingsRepository) *UserService {
	return &UserService{repo: repo}
}

// Start activates a user with default topics.
func (s *UserService) Start(ctx context.Context, userID int64) error {
	settings := &model.UserSettings{UserID: userID, Topics: []string{"golang"}, Active: true}
	return s.repo.Save(ctx, settings)
}

// UpdateTopics sets the topics for the user.
func (s *UserService) UpdateTopics(ctx context.Context, userID int64, topics []string) error {
	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	settings.Topics = topics
	return s.repo.Save(ctx, settings)
}

// Stop deactivates the user.
func (s *UserService) Stop(ctx context.Context, userID int64) error {
	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	settings.Active = false
	return s.repo.Save(ctx, settings)
}

// GetNews returns a stubbed news string for topics.
func (s *UserService) GetNews(ctx context.Context, topics []string) (string, error) {
	// In real implementation we would query an API.
	return fmt.Sprintf("Latest news about %s", strings.Join(topics, ", ")), nil
}

// ActiveUsers returns all active users.
func (s *UserService) ActiveUsers(ctx context.Context) ([]*model.UserSettings, error) {
	all, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []*model.UserSettings{}
	for _, u := range all {
		if u.Active {
			out = append(out, u)
		}
	}
	return out, nil
}
