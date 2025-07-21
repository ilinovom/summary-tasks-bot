package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/summary-tasks-bot/internal/config"
	"github.com/example/summary-tasks-bot/internal/model"
	"github.com/example/summary-tasks-bot/internal/repository"
	"github.com/example/summary-tasks-bot/internal/service"
	"github.com/example/summary-tasks-bot/pkg/openai"
	"github.com/example/summary-tasks-bot/pkg/telegram"
)

type convStage int

const (
	stageInfoTypes convStage = iota + 1
	stageCategories
	stageFrequency
)

type conversationState struct {
	Stage        convStage
	InfoTypes    []string
	Categories   []string
	UpdateTopics bool
}

func formatOptions(opts []string) string {
	lines := make([]string, len(opts))
	for i, o := range opts {
		lines[i] = fmt.Sprintf("%d. %s", i+1, o)
	}
	return strings.Join(lines, "\n")
}

func parseSelection(text string, opts []string, limit int) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' })
	out := []string{}
	seen := map[int]bool{}
	for _, f := range fields {
		idx, err := strconv.Atoi(f)
		if err != nil || idx < 1 || idx > len(opts) || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, opts[idx-1])
		if len(out) == limit {
			break
		}
	}
	return out
}

// App coordinates the services and telegram client.
type App struct {
	cfg             *config.Config
	repo            repository.UserSettingsRepository
	userService     *service.UserService
	tgClient        *telegram.Client
	aiClient        *openai.Client
	convs           map[int64]*conversationState
	infoOptions     []string
	categoryOptions []string
}

func New(cfg *config.Config, repo repository.UserSettingsRepository) *App {
	return &App{
		cfg:             cfg,
		repo:            repo,
		tgClient:        telegram.NewClient(cfg.TelegramToken),
		aiClient:        openai.NewClient(cfg.OpenAIToken, cfg.OpenAIBaseURL, cfg.OpenAIModel),
		convs:           map[int64]*conversationState{},
		infoOptions:     cfg.Options.InfoOptions,
		categoryOptions: cfg.Options.CategoryOptions,
	}
}

func (a *App) Run(ctx context.Context) error {
	log.Println("application starting")
	a.userService = service.NewUserService(a.repo, a.aiClient, a.cfg.Tariffs, a.cfg.Prompt)

	a.setCommands(ctx)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.handleUpdates(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.scheduleMessages(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
	log.Println("application stopped")
	return nil
}

func (a *App) handleUpdates(ctx context.Context) {
	offset := 0
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := a.tgClient.GetUpdates(ctx, offset)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Println("get updates:", err)
			time.Sleep(time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil {
				continue
			}
			a.handleMessage(ctx, u.Message)
		}
	}
}

func (a *App) handleMessage(ctx context.Context, m *telegram.Message) {
	if conv, ok := a.convs[m.Chat.ID]; ok && conv.Stage != 0 && m.Text != "/start" {
		a.continueConversation(ctx, m, conv)
		return
	}

	switch m.Text {
	case "/start":
		log.Printf("user %d called /start", m.Chat.ID)
		if _, err := a.repo.Get(ctx, m.Chat.ID); err != nil {
			a.convs[m.Chat.ID] = &conversationState{Stage: stageInfoTypes}
			prompt := "Какую информацию вы хотели бы получать?\n" + formatOptions(a.infoOptions) + "\nВведите номера через запятую (не более 5)."
			a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, nil)
			return
		}
		if err := a.userService.Start(ctx, m.Chat.ID); err != nil {
			log.Println("start:", err)
		} else {
			const msg = `Привет! 
/start для старта/возобновления отправки сообщений.
/update_topics для обновления типов и категорий.
/update_news_schedule для обновления расписания отправки сообщений.
/get_news_now, чтобы получить информацию прямо сейчас.
/stop, чтобы остановить отправку сообщений.`

			err := a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
			if err != nil {
				log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
			}
		}
	case "/stop":
		log.Printf("user %d called /stop", m.Chat.ID)
		if err := a.userService.Stop(ctx, m.Chat.ID); err != nil {
			log.Println("stop:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Stopped updates", nil)
		}
	case "/get_news_now":
		log.Printf("user %d called /get_news_now", m.Chat.ID)
		settings, err := a.repo.Get(ctx, m.Chat.ID)
		if err != nil {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Use /start first", nil)
			return
		}
		msg, err := a.userService.GetNews(ctx, settings)
		if err != nil {
			log.Println("get_news_now:", err)
			return
		}
		a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
	case "/my_topics":
		log.Printf("user %d called /my_topics", m.Chat.ID)
		settings, err := a.repo.Get(ctx, m.Chat.ID)
		if err != nil {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Use /start first", nil)
			return
		}
		info := strings.Join(settings.InfoTypes, ", ")
		cats := strings.Join(settings.Categories, ", ")
		msg := fmt.Sprintf("Ваши типы: %s\nВаши категории: %s", info, cats)
		a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
	case "/update_topics":
		log.Printf("user %d called /update_topics", m.Chat.ID)
		a.convs[m.Chat.ID] = &conversationState{Stage: stageInfoTypes, UpdateTopics: true}
		prompt := "Какую информацию вы хотели бы получать?\n" + formatOptions(a.infoOptions) + "\nВведите номера через запятую (не более 5)."
		a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, nil)
	default:
		// ignore other messages
	}
}

func (a *App) scheduleMessages(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			users, err := a.userService.ActiveUsers(ctx)
			if err != nil {
				log.Println("active users:", err)
				continue
			}
			for _, u := range users {
				msg, err := a.userService.GetNews(ctx, u)
				if err != nil {
					log.Println("get news:", err)
					continue
				}
				a.tgClient.SendMessage(ctx, u.UserID, msg, nil)
			}
		}
	}
}

func (a *App) setCommands(ctx context.Context) {
	cmds := []telegram.BotCommand{
		{Command: "start", Description: "Start interaction"},
		{Command: "update_topics", Description: "Update your topics"},
		{Command: "get_news_now", Description: "Get news immediately"},
		{Command: "my_topics", Description: "Show my topics"},
		{Command: "stop", Description: "Stop receiving updates"},
	}
	if err := a.tgClient.SetCommands(ctx, cmds); err != nil {
		log.Println("set commands:", err)
	}
}

func (a *App) continueConversation(ctx context.Context, m *telegram.Message, c *conversationState) {
	switch c.Stage {
	case stageInfoTypes:
		c.InfoTypes = parseSelection(m.Text, a.infoOptions, 5)
		c.Stage = stageCategories
		prompt := "Выберите категории или топики:\n" + formatOptions(a.categoryOptions) + "\nВведите номера через запятую (не более 5)."
		a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, nil)
	case stageCategories:
		c.Categories = parseSelection(m.Text, a.categoryOptions, 5)
		if c.UpdateTopics {
			settings, err := a.repo.Get(ctx, m.Chat.ID)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Println("save settings:", err)
				delete(a.convs, m.Chat.ID)
				return
			}
			if err != nil && errors.Is(err, os.ErrNotExist) {
				settings = &model.UserSettings{UserID: m.Chat.ID}
			}
			settings.InfoTypes = c.InfoTypes
			settings.Categories = c.Categories
			if err := a.repo.Save(ctx, settings); err != nil {
				log.Println("save settings:", err)
			} else {
				a.tgClient.SendMessage(ctx, m.Chat.ID, "Настройки обновлены", nil)
			}
			delete(a.convs, m.Chat.ID)
			return
		}
		c.Stage = stageFrequency
		a.tgClient.SendMessage(ctx, m.Chat.ID, "Как часто хотите получать информацию? 0 - один раз, 1-3 - раз в день.", nil)
	case stageFrequency:
		freq, err := strconv.Atoi(strings.TrimSpace(m.Text))
		if err != nil || freq < 0 || freq > 3 {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Укажите число от 0 до 3", nil)
			return
		}
		settings := &model.UserSettings{
			UserID:     m.Chat.ID,
			Topics:     c.Categories,
			InfoTypes:  c.InfoTypes,
			Categories: c.Categories,
			Frequency:  freq,
			Tariff:     "base",
			Active:     true,
		}
		if err := a.repo.Save(ctx, settings); err != nil {
			log.Println("save settings:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Настройки сохранены", nil)
		}
		delete(a.convs, m.Chat.ID)
	}
}
