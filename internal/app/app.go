package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/example/summary-tasks-bot/internal/repository"
	"github.com/example/summary-tasks-bot/internal/service"
	"github.com/example/summary-tasks-bot/pkg/openai"
	"github.com/example/summary-tasks-bot/pkg/telegram"
)

// App coordinates the services and telegram client.
type App struct {
	repo        repository.UserSettingsRepository
	userService *service.UserService
	tgClient    *telegram.Client
	aiClient    *openai.Client
}

func New(telegramToken string, aiToken string, repo repository.UserSettingsRepository) *App {
	return &App{
		repo:     repo,
		tgClient: telegram.NewClient(telegramToken),
		aiClient: openai.NewClient(aiToken),
	}
}

func (a *App) Run(ctx context.Context) error {
	a.userService = service.NewUserService(a.repo, a.aiClient)

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
	return nil
}

func (a *App) handleUpdates(ctx context.Context) {
	offset := 0
	for {
		updates, err := a.tgClient.GetUpdates(ctx, offset)
		if err != nil {
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
	switch m.Text {
	case "/start":
		if err := a.userService.Start(ctx, m.Chat.ID); err != nil {
			log.Println("start:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Welcome! Use /update_topics to set topics.")
		}
	case "/stop":
		if err := a.userService.Stop(ctx, m.Chat.ID); err != nil {
			log.Println("stop:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Stopped updates")
		}
	default:
		if strings.HasPrefix(m.Text, "/update_topics ") {
			topics := strings.Fields(m.Text[len("/update_topics "):])
			if err := a.userService.UpdateTopics(ctx, m.Chat.ID, topics); err != nil {
				log.Println("update_topics:", err)
			} else {
				a.tgClient.SendMessage(ctx, m.Chat.ID, "Topics updated")
			}
		}
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
				msg, err := a.userService.GetNews(ctx, u.Topics)
				if err != nil {
					log.Println("get news:", err)
					continue
				}
				a.tgClient.SendMessage(ctx, u.UserID, msg)
			}
		}
	}
}
