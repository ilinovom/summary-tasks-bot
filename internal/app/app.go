package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

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
	Stage      convStage
	InfoTypes  []string
	Categories []string
}

var infoOptions = []string{
	"Инсайты / Мыслеформы",
	"Интересные факты",
	"Проблемы, которые ждут решения",
	"Бизнес-идеи / модели / форматы",
	"Сводка трендов / новостей",
	"Альтернативные взгляды на привычные вещи",
	"Истории провалов и взлётов",
	"Вопросы для самоанализа и мышления",
	"Кейсы / разборы чужих продуктов",
	"Технологические находки",
	"Социальные, культурные, психологические сдвиги",
	"Наблюдения за повседневностью",
}

var categoryOptions = []string{
	"🚀 Бизнес и стартапы",
	"🧠 Психология и мышление",
	"🔧 Боли и проблемы",
	"🌐 Технологии",
	"📚 Образование и навыки",
	"🏙 Общество и культура",
	"🌱 Экология и устойчивость",
	"💡 Необычные идеи",
	"📦 Бизнес-платформы и сервисы",
	"📊 Цифры и сравнения",
	"❓ Вопрос дня",
	"🤖 Пример использования GPT / AI",
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
	repo        repository.UserSettingsRepository
	userService *service.UserService
	tgClient    *telegram.Client
	aiClient    *openai.Client
	convs       map[int64]*conversationState
}

func New(telegramToken string, aiToken string, repo repository.UserSettingsRepository) *App {
	return &App{
		repo:     repo,
		tgClient: telegram.NewClient(telegramToken),
		aiClient: openai.NewClient(aiToken),
		convs:    map[int64]*conversationState{},
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
	if conv, ok := a.convs[m.Chat.ID]; ok && conv.Stage != 0 && m.Text != "/start" {
		a.continueConversation(ctx, m, conv)
		return
	}

	switch m.Text {
	case "/start":
		if _, err := a.repo.Get(ctx, m.Chat.ID); err != nil {
			a.convs[m.Chat.ID] = &conversationState{Stage: stageInfoTypes}
			prompt := "Какую информацию вы хотели бы получать?\n" + formatOptions(infoOptions) + "\nВведите номера через запятую (не более 5)."
			a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, nil)
			return
		}
		if err := a.userService.Start(ctx, m.Chat.ID); err != nil {
			log.Println("start:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Welcome! Use /update_topics to set topics.", nil)
		}
	case "/stop":
		if err := a.userService.Stop(ctx, m.Chat.ID); err != nil {
			log.Println("stop:", err)
		} else {
			a.tgClient.SendMessage(ctx, m.Chat.ID, "Stopped updates", nil)
		}
	default:
		if strings.HasPrefix(m.Text, "/update_topics ") {
			topics := strings.Fields(m.Text[len("/update_topics "):])
			if err := a.userService.UpdateTopics(ctx, m.Chat.ID, topics); err != nil {
				log.Println("update_topics:", err)
			} else {
				a.tgClient.SendMessage(ctx, m.Chat.ID, "Topics updated", nil)
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
				a.tgClient.SendMessage(ctx, u.UserID, msg, nil)
			}
		}
	}
}

func (a *App) continueConversation(ctx context.Context, m *telegram.Message, c *conversationState) {
	switch c.Stage {
	case stageInfoTypes:
		c.InfoTypes = parseSelection(m.Text, infoOptions, 5)
		c.Stage = stageCategories
		prompt := "Выберите категории или топики:\n" + formatOptions(categoryOptions) + "\nВведите номера через запятую (не более 5)."
		a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, nil)
	case stageCategories:
		c.Categories = parseSelection(m.Text, categoryOptions, 5)
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
