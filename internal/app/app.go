package app

import (
	"context"
	"errors"
	"github.com/ilinovom/summary-tasks-bot/internal/app/cmdHandlers"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
	"github.com/ilinovom/summary-tasks-bot/internal/service"
	"github.com/ilinovom/summary-tasks-bot/pkg/openai"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

// App coordinates the services and telegram client.
type App struct {
	cfg         *config.Config
	repo        repository.UserSettingsRepository
	userService *service.UserService
	tgClient    *telegram.Client
	aiClient    *openai.Client
	ch          *cmdHandlers.CmdHandler
}

// New constructs the application instance with all dependencies wired.
func New(cfg *config.Config, repo repository.UserSettingsRepository) *App {

	tgClient := telegram.NewClient(cfg.TelegramToken)
	aiClient := openai.NewClient(cfg.OpenAIToken, cfg.OpenAIBaseURL)
	userService := service.NewUserService(repo, aiClient, cfg.Tariffs)

	return &App{
		cfg:         cfg,
		repo:        repo,
		tgClient:    tgClient,
		aiClient:    aiClient,
		userService: userService,
		ch:          cmdHandlers.NewCmdHandler(cfg, userService, repo, tgClient),
	}
}

// sendMessage is a small wrapper around the Telegram client that logs failures
// but still returns the message ID to the caller.
func (a *App) sendMessage(ctx context.Context, chatID int64, text string, kb [][]string) (int, error) {
	msgID, err := a.tgClient.SendMessage(ctx, chatID, text, kb)
	if err != nil {
		log.Printf("telegram send message: %v\ntext: %s", err, text)
	}
	return msgID, err
}

// Run starts the main application logic and blocks until the context is
// cancelled. It launches goroutines for updates and scheduled messages.
func (a *App) Run(ctx context.Context) error {
	log.Println("application starting")

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

// handleUpdates continuously polls Telegram for updates and dispatches them
// for further processing.
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

// handleMessage routes incoming user messages to the appropriate command
// handlers or continues an existing conversation.
func (a *App) handleMessage(ctx context.Context, m *telegram.Message) {
	a.ch.HandleMessages(ctx, m)
}

// inTimeRange checks whether the provided time falls within the "HH:MM-HH:MM"
// range specified in rng. If the range is invalid the function returns true.
func inTimeRange(now time.Time, rng string) bool {
	parts := strings.Split(rng, "-")
	if len(parts) != 2 {
		return true
	}
	start, err1 := time.Parse("15:04", parts[0])
	end, err2 := time.Parse("15:04", parts[1])
	if err1 != nil || err2 != nil {
		return true
	}
	y, m, d := now.Date()
	start = time.Date(y, m, d, start.Hour(), start.Minute(), 0, 0, now.Location())
	end = time.Date(y, m, d, end.Hour(), end.Minute(), 0, 0, now.Location())
	if end.Before(start) {
		return now.After(start) || now.Before(end)
	}
	return !now.Before(start) && !now.After(end)
}

// scheduleMessages periodically sends news digests to active users respecting
// their tariff restrictions and configured time range.
func (a *App) scheduleMessages(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
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
			now := time.Now()
			for _, u := range users {
				tariff, ok := a.cfg.Tariffs[u.Tariff]
				if !ok {
					tariff = a.cfg.Tariffs["base"]
				}
				if !inTimeRange(now, tariff.Schedule.TimeRange) {
					continue
				}
				last := time.Unix(u.LastScheduledSent, 0)
				if now.Sub(last) < time.Duration(tariff.Schedule.FrequencyMinutes)*time.Minute {
					continue
				}
				if len(u.Topics) == 0 {
					a.sendMessage(ctx, u.UserID, "Вы не задали категории. Если хотите получать автоматические сообщения для расширения кругозора, то задайте темы с помощью /update_topics или же остановите автоматическую рассылку с помощью команды /stop", nil)
					u.LastScheduledSent = now.Unix()
					if err := a.repo.Save(ctx, u); err != nil {
						log.Println("save settings:", err)
					}
					continue
				}

				msg, err := a.userService.GetNewsMultiInfo(ctx, u)
				if err != nil {
					log.Println("get news:", err)
					continue
				}
				a.sendMessage(ctx, u.UserID, msg, nil)
				log.Printf("user %d(@%s) got scheduled news", u.UserID, u.UserName)

				u.LastScheduledSent = now.Unix()
				if err := a.repo.Save(ctx, u); err != nil {
					log.Println("save settings:", err)
				}
			}
		}
	}
} // setCommands registers the list of bot commands with Telegram so that users
// see available commands in the UI.
func (a *App) setCommands(ctx context.Context) {
	a.ch.SetCommands(ctx)
}
