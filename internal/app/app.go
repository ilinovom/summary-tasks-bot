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

	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
	"github.com/ilinovom/summary-tasks-bot/internal/service"
	"github.com/ilinovom/summary-tasks-bot/pkg/openai"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

type convStage int

const (
	stageUpdateChoice convStage = iota + 1
	stageSelectExistingCategory
	stageCategory
	stageInfoTypes
)

type conversationState struct {
	Stage         convStage
	Step          int
	CurrentCat    string
	OldCat        string
	Topics        map[string][]string
	UpdateTopics  bool
	CategoryLimit int
	InfoLimit     int
	LastMsgID     int
	AvailableCats []string
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

func numberKeyboard(n int) [][]string {
	rows := [][]string{}
	row := []string{}
	for i := 1; i <= n; i++ {
		row = append(row, strconv.Itoa(i))
		if len(row) == 5 {
			rows = append(rows, row)
			row = []string{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
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
		aiClient:        openai.NewClient(cfg.OpenAIToken, cfg.OpenAIBaseURL),
		convs:           map[int64]*conversationState{},
		infoOptions:     cfg.Options.InfoOptions,
		categoryOptions: cfg.Options.CategoryOptions,
	}
}

func (a *App) Run(ctx context.Context) error {
	log.Println("application starting")
	a.userService = service.NewUserService(a.repo, a.aiClient, a.cfg.Tariffs)

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
	// if user text first time
	if conv, ok := a.convs[m.Chat.ID]; ok && conv.Stage != 0 && m.Text != "/start" {
		a.continueConversation(ctx, m, conv)
		return
	}

	switch m.Text {
	case "/start":
		a.handleStartCommand(ctx, m)
	case "/stop":
		a.handleStopCommand(ctx, m)
	case "/get_news_now":
		a.handleGetNewsNowCommand(ctx, m)
	case "/my_topics":
		a.handleMyTopicsCommand(ctx, m)
	case "/update_topics":
		a.handleUpdateTopicsCommand(ctx, m)
	default:
		log.Printf("user %d(@%s) texted: %s", m.Chat.ID, m.Chat.Username, m.Text)
		promt := "Я не понимаю текст вне команд. Чтобы увидеть команды, нажми кнопку `Меню` или вызови команду  `/start`"
		_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, promt, nil)
	}
}

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
				if !inTimeRange(now, tariff.TimeRangeScheduledMsgSendPerDay) {
					continue
				}
				last := time.Unix(u.LastScheduledSent, 0)
				if now.Sub(last) < time.Duration(tariff.FrequencyScheduledMsgSendInMinutes)*time.Minute {
					continue
				}
				msg, err := a.userService.GetNews(ctx, u)
				if err != nil {
					log.Println("get news:", err)
					continue
				}
				_, _ = a.tgClient.SendMessage(ctx, u.UserID, msg, nil)
				log.Printf("user %d(@%s) got scheduled news", u.UserID, u.UserName)

				u.LastScheduledSent = now.Unix()
				if err := a.repo.Save(ctx, u); err != nil {
					log.Println("save settings:", err)
				}
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
	case stageUpdateChoice:
		choice := parseSelection(m.Text, []string{"Обновить все", "Обновить одну"}, 1)
		if len(choice) == 0 {
			msg, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, "Выберите действие", [][]string{{"1", "2"}})
			c.LastMsgID = msg
			return
		}
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if choice[0] == "Обновить одну" {
			c.CategoryLimit = 1
			c.AvailableCats = make([]string, 0, len(c.Topics))
			for cat := range c.Topics {
				c.AvailableCats = append(c.AvailableCats, cat)
			}
			c.Stage = stageSelectExistingCategory
			prompt := fmt.Sprintf("Какую категорию обновить?\n%s\nВведите номер.", formatOptions(c.AvailableCats))
			msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(c.AvailableCats)))
			c.LastMsgID = msgID
			return
		}

		c.Topics = map[string][]string{}
		c.Step = 0
		c.Stage = stageCategory
		prompt := fmt.Sprintf("Выберите категорию №1:\n%s\nВведите номер.", formatOptions(a.categoryOptions))
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.categoryOptions)))
		c.LastMsgID = msgID

	case stageSelectExistingCategory:
		cats := parseSelection(m.Text, c.AvailableCats, 1)
		if len(cats) == 0 {
			msg, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, "Выберите номер категории", numberKeyboard(len(c.AvailableCats)))
			c.LastMsgID = msg
			return
		}
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		c.OldCat = cats[0]
		c.Stage = stageCategory
		prompt := fmt.Sprintf("Выберите новую категорию вместо '%s':\n%s\nВведите номер.", c.OldCat, formatOptions(a.categoryOptions))
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.categoryOptions)))
		c.LastMsgID = msgID

	case stageCategory:
		cats := parseSelection(m.Text, a.categoryOptions, 1)
		if len(cats) == 0 {
			msg, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, "Выберите номер категории", numberKeyboard(len(a.categoryOptions)))
			c.LastMsgID = msg
			return
		}
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		c.CurrentCat = cats[0]
		c.Stage = stageInfoTypes
		prompt := fmt.Sprintf("Выберите типы информации для категории '%s':\n%s\nВведите номера через запятую (не более %d).", c.CurrentCat, formatOptions(a.infoOptions), c.InfoLimit)
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.infoOptions)))
		c.LastMsgID = msgID

	case stageInfoTypes:
		infos := parseSelection(m.Text, a.infoOptions, c.InfoLimit)
		if len(infos) == 0 {
			msg, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, "Введите номера типов информации", numberKeyboard(len(a.infoOptions)))
			c.LastMsgID = msg
			return
		}
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.tgClient.DeleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if c.Topics == nil {
			c.Topics = map[string][]string{}
		}
		if c.OldCat != "" {
			delete(c.Topics, c.OldCat)
			c.OldCat = ""
		}
		existing := c.Topics[c.CurrentCat]
		for _, inf := range infos {
			found := false
			for _, ex := range existing {
				if ex == inf {
					found = true
					break
				}
			}
			if !found {
				existing = append(existing, inf)
			}
		}
		c.Topics[c.CurrentCat] = existing
		c.Step++
		if c.Step >= c.CategoryLimit {
			var settings *model.UserSettings
			var err error
			if c.UpdateTopics {
				settings, err = a.repo.Get(ctx, m.Chat.ID)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					log.Println("save settings:", err)
					delete(a.convs, m.Chat.ID)
					return
				}
				if err != nil && errors.Is(err, os.ErrNotExist) {
					settings = &model.UserSettings{UserID: m.Chat.ID, UserName: m.Chat.Username}
				}
				settings.Topics = c.Topics
				if err := a.repo.Save(ctx, settings); err != nil {
					log.Println("save settings:", err)
				} else {
					parts := []string{}
					for cat, types := range c.Topics {
						parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
					}
					_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, "Настройки обновлены:\n"+strings.Join(parts, "\n"), nil)
				}
				delete(a.convs, m.Chat.ID)
				return
			}

			settings = &model.UserSettings{
				UserID:            m.Chat.ID,
				UserName:          m.Chat.Username,
				Topics:            c.Topics,
				Tariff:            "base",
				LastScheduledSent: 0,
				LastGetNewsNow:    0,
				GetNewsNowCount:   0,
				Active:            true,
			}
			if err := a.repo.Save(ctx, settings); err != nil {
				log.Println("save settings:", err)
			} else {
				parts := []string{}
				for cat, types := range c.Topics {
					parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
				}
				_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, "Настройки сохранены:\n"+strings.Join(parts, "\n"), nil)
				msg, err := a.userService.GetNews(ctx, settings)
				if err == nil {
					_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
				} else {
					log.Println("get news:", err)
				}
			}
			delete(a.convs, m.Chat.ID)
			return
		}

		c.Stage = stageCategory
		prompt := fmt.Sprintf("Выберите категорию №%d:\n%s\nВведите номер.", c.Step+1, formatOptions(a.categoryOptions))
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.categoryOptions)))
		c.LastMsgID = msgID
	}
}

const startMsg = `Привет! Я бот для расширения кругозора.
Что я умею?
	- по выбранной категории и типу информации присылать тебе сообщения, которые будут развивать твой кругозор.

Как часто можно получать сообщения?
1) На бесплатном тарифе можно:
	- получать до 5 сообщений моментально (команда /get_news_now)
	- получать сообщения каждые 3 часа в интервале с 08:00 до 23:00
   * суммарно можно получить за день 8 сообщений
2) Другие тарифы пока прорабатываются ...

Какие у меня есть команды?
	- /start для старта/возобновления отправки сообщений.
	- /update_topics для обновления типов и категорий.
	- /get_news_now, чтобы получить информацию прямо сейчас.
	- /my_topics, чтобы получить установленные категории и типы информации.
	- /stop, чтобы остановить отправку сообщений.`

func (a *App) handleStartCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d (@%s) called /start", m.Chat.ID, m.Chat.Username)
	if _, err := a.repo.Get(ctx, m.Chat.ID); err != nil {
		_, err := a.tgClient.SendMessage(ctx, m.Chat.ID, startMsg, nil)
		if err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
		time.Sleep(time.Second * 5)
		t := a.cfg.Tariffs["base"]
		conv := &conversationState{Stage: stageCategory, CategoryLimit: t.CategoryNumLimit, InfoLimit: t.InfoTypeNumLimit}
		a.convs[m.Chat.ID] = conv
		prompt := fmt.Sprintf("Выберите категорию №1:\n%s\nВведите номер.", formatOptions(a.categoryOptions))
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.categoryOptions)))
		conv.LastMsgID = msgID
		return
	}
	if err := a.userService.Start(ctx, m.Chat.ID, m.Chat.Username); err != nil {
		log.Println("start:", err)
	} else {
		_, err := a.tgClient.SendMessage(ctx, m.Chat.ID, startMsg, nil)
		if err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
	}
}

func (a *App) handleStopCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /stop", m.Chat.ID, m.Chat.Username)
	if err := a.userService.Stop(ctx, m.Chat.ID); err != nil {
		log.Println("stop:", err)
	} else {
		_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, "Stopped updates", nil)
	}
}

func (a *App) handleGetNewsNowCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /get_news_now", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.tgClient.SendMessage(ctx, m.Chat.ID, "Use /start first", nil)
		return
	}
	tariff, ok := a.cfg.Tariffs[settings.Tariff]
	if !ok {
		tariff = a.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(settings.LastGetNewsNow, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		settings.GetNewsNowCount = 0
	}
	if settings.GetNewsNowCount >= tariff.NumberGetNewsNowMessagesPerDay {
		_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, "Лимит исчерпан на сегодня", nil)
		return
	}
	settings.GetNewsNowCount++
	settings.LastGetNewsNow = now.Unix()
	if err := a.repo.Save(ctx, settings); err != nil {
		log.Println("save settings:", err)
	}
	msg, err := a.userService.GetNews(ctx, settings)
	if err != nil {
		log.Println("get_news_now:", err)
		return
	}
	_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
}

func (a *App) handleUpdateTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /update_topics", m.Chat.ID, m.Chat.Username)
	tariff := a.cfg.Tariffs["base"]
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err == nil {
		if t, ok := a.cfg.Tariffs[settings.Tariff]; ok {
			tariff = t
		}
	}
	conv := &conversationState{UpdateTopics: true, CategoryLimit: tariff.CategoryNumLimit, InfoLimit: tariff.InfoTypeNumLimit}
	if err == nil && len(settings.Topics) > 0 {
		conv.Stage = stageUpdateChoice
		conv.Topics = make(map[string][]string, len(settings.Topics))
		for k, v := range settings.Topics {
			conv.Topics[k] = append([]string(nil), v...)
		}
		a.convs[m.Chat.ID] = conv
		msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, "Что будем обновлять?\n1. Обновить все\n2. Обновить одну", [][]string{{"1", "2"}})
		conv.LastMsgID = msgID
		return
	}
	conv.Stage = stageCategory
	a.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf("Выберите категорию №1:\n%s\nВведите номер.", formatOptions(a.categoryOptions))
	msgID, _ := a.tgClient.SendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(a.categoryOptions)))
	conv.LastMsgID = msgID
}

func (a *App) handleMyTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /my_topics", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, "Use /start first", nil)
		return
	}
	parts := []string{}
	for cat, types := range settings.Topics {
		parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
	}
	msg := "Ваши темы:\n" + strings.Join(parts, "\n")
	_, _ = a.tgClient.SendMessage(ctx, m.Chat.ID, msg, nil)
}
