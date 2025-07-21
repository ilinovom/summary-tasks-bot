package service

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strings"

	"github.com/example/summary-tasks-bot/internal/config"
	"github.com/example/summary-tasks-bot/internal/model"
	"github.com/example/summary-tasks-bot/internal/repository"
)

// AIClient describes the part of the OpenAI client used by the service.
type AIClient interface {
	ChatCompletion(ctx context.Context, prompt string) (string, error)
}

type UserService struct {
	repo   repository.UserSettingsRepository
	openai AIClient
	prompt config.PromptConfig
}

func NewUserService(repo repository.UserSettingsRepository, ai AIClient, p config.PromptConfig) *UserService {
	return &UserService{repo: repo, openai: ai, prompt: p}
}

// Start activates a user with default topics.
func (s *UserService) Start(ctx context.Context, userID int64) error {
	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		settings = &model.UserSettings{UserID: userID, Topics: []string{"golang"}}
	}
	if len(settings.Topics) == 0 {
		settings.Topics = []string{"golang"}
	}
	settings.Active = true
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

// GetNews returns news about the provided topics using the OpenAI API.
func (s *UserService) GetNews(ctx context.Context, u *model.UserSettings) (string, error) {
	info := ""
	if len(u.InfoTypes) > 0 {
		info = u.InfoTypes[rand.Intn(len(u.InfoTypes))]
	}
	category := ""
	if len(u.Categories) > 0 {
		category = u.Categories[rand.Intn(len(u.Categories))]
	}
	prompt := s.prompt.Prompt
	prompt = strings.ReplaceAll(prompt, "{тип}", info)
	prompt = strings.ReplaceAll(prompt, "{категория}", category)
	prompt = strings.ReplaceAll(prompt, "{тон}", s.prompt.Style)
	prompt = strings.ReplaceAll(prompt, "{объём}", s.prompt.Volume)
	var resp string
	var err error
	if s.openai == nil {
		resp = prompt
	} else {
		resp, err = s.openai.ChatCompletion(ctx, prompt)
		if err != nil {
			return "", err
		}
	}
	prefixParts := []string{}
	if info != "" {
		prefixParts = append(prefixParts, "Тип: "+info)
	}
	if category != "" {
		prefixParts = append(prefixParts, "Категория: "+category)
	}
	prefix := strings.Join(prefixParts, "\n")
	if prefix != "" {
		prefix += "\n\n"
	}
	return prefix + resp, nil
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
