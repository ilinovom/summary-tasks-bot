package service

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
)

// AIClient describes the part of the OpenAI client used by the service.
type AIClient interface {
	ChatCompletion(ctx context.Context, model, prompt string, maxTokens int) (string, error)
	ChatResponses(ctx context.Context, model, prompt string, maxTokens int) (string, error)
}

type UserService struct {
	repo    repository.UserSettingsRepository
	openai  AIClient
	tariffs map[string]config.Tariff
}

func NewUserService(repo repository.UserSettingsRepository, ai AIClient, tariffs map[string]config.Tariff) *UserService {
	return &UserService{repo: repo, openai: ai, tariffs: tariffs}
}

// Start activates a user with default settings.
func (s *UserService) Start(ctx context.Context, userID int64, userName string) error {
	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		settings = &model.UserSettings{UserID: userID, UserName: userName}
	}
	if settings.Tariff == "" {
		settings.Tariff = "base"
	}
	settings.Active = true
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

// GetNews returns news according to the user preferences using the OpenAI API.
func (s *UserService) GetNews(ctx context.Context, u *model.UserSettings) (string, error) {
	info := ""
	category := ""
	if len(u.Topics) > 0 {
		cats := make([]string, 0, len(u.Topics))
		for c := range u.Topics {
			cats = append(cats, c)
		}
		category = cats[rand.Intn(len(cats))]
		infos := u.Topics[category]
		if len(infos) > 0 {
			info = infos[rand.Intn(len(infos))]
		}
	}
	t, ok := s.tariffs[u.Tariff]
	if !ok {
		log.Fatal("tariff for user is not set", u.UserID)
	}
	prompt := t.GPT.PromptMain
	prompt = strings.ReplaceAll(prompt, "{тип}", info)
	prompt = strings.ReplaceAll(prompt, "{категория}", category)
	prompt = strings.ReplaceAll(prompt, "{тон}", t.GPT.Style)
	prompt = strings.ReplaceAll(prompt, "{объём}", t.GPT.Volume)
	var resp string
	var err error
	if s.openai == nil {
		resp = prompt
	} else {
		resp, err = s.openai.ChatCompletion(ctx, t.GPT.Model, prompt, t.GPT.MaxTokens)
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

// GetNewsMultiInfo returns news for one random category with all selected info types.
func (s *UserService) GetNewsMultiInfo(ctx context.Context, u *model.UserSettings) (string, error) {
	if len(u.Topics) == 0 {
		return "", errors.New("no topics")
	}
	cats := make([]string, 0, len(u.Topics))
	for c := range u.Topics {
		cats = append(cats, c)
	}
	category := cats[rand.Intn(len(cats))]
	infos := u.Topics[category]
	t, ok := s.tariffs[u.Tariff]
	if !ok {
		log.Fatal("tariff for user is not set", u.UserID)
	}
	var parts []string
	parts = append(parts, "Категория: "+category)
	for _, info := range infos {
		prompt := t.GPT.PromptMain
		prompt = strings.ReplaceAll(prompt, "{тип}", info)
		prompt = strings.ReplaceAll(prompt, "{категория}", category)
		prompt = strings.ReplaceAll(prompt, "{тон}", t.GPT.Style)
		prompt = strings.ReplaceAll(prompt, "{объём}", t.GPT.Volume)
		var resp string
		var err error
		if s.openai == nil {
			resp = prompt
		} else {
			resp, err = s.openai.ChatCompletion(ctx, t.GPT.Model, prompt, t.GPT.MaxTokens)
			if err != nil {
				return "", err
			}
		}
		parts = append(parts, "\nТип: "+info+"\n"+resp)
	}
	return strings.Join(parts, "\n"), nil
}

// GetNewsForCategory returns news for a specific category.
func (s *UserService) GetNewsForCategory(ctx context.Context, u *model.UserSettings, category string) (string, error) {
	info := ""
	if infos, ok := u.Topics[category]; ok && len(infos) > 0 {
		info = infos[rand.Intn(len(infos))]
	}
	t, ok := s.tariffs[u.Tariff]
	if !ok {
		log.Fatal("tariff for user is not set", u.UserID)
	}
	prompt := t.GPT.PromptMain
	prompt = strings.ReplaceAll(prompt, "{тип}", info)
	prompt = strings.ReplaceAll(prompt, "{категория}", category)
	prompt = strings.ReplaceAll(prompt, "{тон}", t.GPT.Style)
	prompt = strings.ReplaceAll(prompt, "{объём}", t.GPT.Volume)
	var resp string
	var err error
	if s.openai == nil {
		resp = prompt
	} else {
		resp, err = s.openai.ChatCompletion(ctx, t.GPT.Model, prompt, t.GPT.MaxTokens)
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

// GetNewsForCategoryMultiInfo returns news for a specific category with all selected info types.
func (s *UserService) GetNewsForCategoryMultiInfo(ctx context.Context, u *model.UserSettings, category string) (string, error) {
	infos, ok := u.Topics[category]
	if !ok || len(infos) == 0 {
		return "", errors.New("no infos for category")
	}
	t, ok := s.tariffs[u.Tariff]
	if !ok {
		log.Fatal("tariff for user is not set", u.UserID)
	}
	var parts []string
	parts = append(parts, "Категория: "+category)
	for _, info := range infos {
		prompt := t.GPT.PromptMain
		prompt = strings.ReplaceAll(prompt, "{тип}", info)
		prompt = strings.ReplaceAll(prompt, "{категория}", category)
		prompt = strings.ReplaceAll(prompt, "{тон}", t.GPT.Style)
		prompt = strings.ReplaceAll(prompt, "{объём}", t.GPT.Volume)
		var resp string
		var err error
		if s.openai == nil {
			resp = prompt
		} else {
			resp, err = s.openai.ChatCompletion(ctx, t.GPT.Model, prompt, t.GPT.MaxTokens)
			if err != nil {
				return "", err
			}
		}
		parts = append(parts, "\nТип: "+info+"\n"+resp)
	}
	return strings.Join(parts, "\n"), nil
}

// GetLast24hNewsForCategory returns news for a category from the last 24 hours.
func (s *UserService) GetLast24hNewsForCategory(ctx context.Context, u *model.UserSettings, category string) (string, error) {
	t, ok := s.tariffs[u.Tariff]
	if !ok {
		log.Fatal("tariff for user is not set", u.UserID)
	}
	prompt := t.GPT.PromptLast24h
	prompt = strings.ReplaceAll(prompt, "{категория}", category)
	prompt = strings.ReplaceAll(prompt, "{тон}", t.GPT.Style)
	prompt = strings.ReplaceAll(prompt, "{объём}", t.GPT.Volume)
	var resp string
	var err error
	if s.openai == nil {
		resp = prompt
	} else {
		resp, err = s.openai.ChatResponses(ctx, t.GPT.Model, prompt, t.GPT.MaxTokens)
		if err != nil {
			return "", err
		}
	}
	if category != "" {
		resp = "Категория: " + category + "\n\n" + resp
	}
	return resp, nil
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

func (s *UserService) GetByUsername(ctx context.Context, username string) (*model.UserSettings, error) {
	all, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range all {
		if strings.EqualFold(u.UserName, username) {
			return u, nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *UserService) SetTariff(ctx context.Context, userID int64, tariff string) error {
	if _, ok := s.tariffs[tariff]; !ok {
		return errors.New("unknown tariff")
	}
	u, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	u.Tariff = tariff
	return s.repo.Save(ctx, u)
}
