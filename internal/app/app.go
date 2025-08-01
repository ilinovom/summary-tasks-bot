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
	stageCategory
	stageCustomCategory
	stageInfoTypes
	stageWelcome
	stageGetNewsCategory
	stageGetLast24hCategory
	stageSelectManyExisting
	stageDeleteChoice
	stageSelectDelete
	stageChooseCategoryCount
	stageSetTariffUser
	stageSetTariffChoice
)

type conversationState struct {
	Stage               convStage
	PrevStage           convStage
	Step                int
	CurrentCat          string
	OldCat              string
	Topics              map[string][]string
	UpdateTopics        bool
	DeleteTopics        bool
	CategoryLimit       int
	InfoLimit           int
	LastMsgID           int
	AvailableCats       []string
	Settings            *model.UserSettings
	AllowCustomCategory bool
	SelectedInfos       []string
	SelectedCats        []string
	TargetUser          string
	NewTariff           string
}

// formatOptions turns the list of options into numbered lines suitable for a
// Telegram message.
func formatOptions(opts []string) string {
	lines := make([]string, len(opts))
	for i, o := range opts {
		lines[i] = fmt.Sprintf("%d. %s", i+1, o)
	}
	return strings.Join(lines, "\n")
}

// addCustomOption adds the "custom" option to the provided slice if the user
// is allowed to specify their own category.
func addCustomOption(opts []string, allow bool) []string {
	if !allow {
		return opts
	}
	out := make([]string, len(opts)+1)
	copy(out, opts)
	out[len(opts)] = "😇Своя категория"
	return out
}

// parseSelection parses comma or space separated option indexes from the user
// input and returns the corresponding option values up to the provided limit.
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

// setStage updates the conversation state and remembers the previous stage to
// support navigating backward.
func (c *conversationState) setStage(s convStage) {
	c.PrevStage = c.Stage
	c.Stage = s
}

// back returns the conversation to the previous stage if possible.
func (c *conversationState) back() {
	if c.PrevStage != 0 {
		c.Stage = c.PrevStage
		c.PrevStage = 0
	}
}

// numberKeyboard builds a keyboard with numeric buttons from 1 to n.
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

// numberKeyboardWithDone builds a numeric keyboard and adds the "Done" button
// as the last row.
func numberKeyboardWithDone(n int) [][]string {
	rows := numberKeyboard(n)
	rows = append(rows, []string{"Готово"})
	return rows
}

// addBack appends a "Back" button to the given keyboard.
func addBack(kb [][]string) [][]string {
	return append(kb, []string{"Назад"})
}

// addBackCancel appends "Back" and "Cancel" buttons to the keyboard.
func addBackCancel(kb [][]string) [][]string {
	return append(kb, []string{"Назад", "Отмена"})
}

// addCancel appends a "Cancel" button to the keyboard.
func addCancel(kb [][]string) [][]string {
	return append(kb, []string{"Отмена"})
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
	messages        map[string]string
}

// New constructs the application instance with all dependencies wired.
func New(cfg *config.Config, repo repository.UserSettingsRepository) *App {
	return &App{
		cfg:             cfg,
		repo:            repo,
		tgClient:        telegram.NewClient(cfg.TelegramToken),
		aiClient:        openai.NewClient(cfg.OpenAIToken, cfg.OpenAIBaseURL),
		convs:           map[int64]*conversationState{},
		infoOptions:     cfg.Options.InfoOptions,
		categoryOptions: cfg.Options.CategoryOptions,
		messages:        cfg.Messages,
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

// sendLongMessage splits a long message into several Telegram messages so that
// each part fits into the platform's limit.
func (a *App) sendLongMessage(ctx context.Context, chatID int64, text string) error {
	const limit = 4096
	runes := []rune(text)
	for len(runes) > 0 {
		n := limit
		if n > len(runes) {
			n = len(runes)
		}
		part := string(runes[:n])
		if _, err := a.sendMessage(ctx, chatID, part, nil); err != nil {
			return err
		}
		runes = runes[n:]
	}
	return nil
}

// deleteMessage removes a previously sent message and logs any deletion error.
func (a *App) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	if err := a.tgClient.DeleteMessage(ctx, chatID, messageID); err != nil {
		log.Printf("telegram delete message: %v", err)
	}
}

// saveTopics persists the conversation topics to the repository. It also sends
// a confirmation message to the user about the updated or created settings.
func (a *App) saveTopics(ctx context.Context, m *telegram.Message, c *conversationState) {
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
			a.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(a.messages["settings_updated"], strings.Join(parts, "\n")), nil)
		}
		delete(a.convs, m.Chat.ID)
		return
	}

	settings = &model.UserSettings{
		UserID:            m.Chat.ID,
		UserName:          m.Chat.Username,
		Topics:            c.Topics,
		Tariff:            "base",
		LastScheduledSent: time.Now().Unix(),
		LastGetNewsNow:    0,
		GetNewsNowCount:   0,
		LastGetLast24h:    0,
		GetLast24hCount:   0,
		Active:            true,
	}
	if err := a.repo.Save(ctx, settings); err != nil {
		log.Println("save settings:", err)
	} else {
		parts := []string{}
		for cat, types := range c.Topics {
			parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
		}
		a.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(a.messages["settings_saved"], strings.Join(parts, "\n")), nil)
		msg, err := a.userService.GetNewsMultiInfo(ctx, settings)
		if err == nil {
			if len([]rune(msg)) > 4096 {
				if err := a.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
					log.Println("send msg err: ", err)
				}
			} else {
				a.sendMessage(ctx, m.Chat.ID, msg, nil)
			}
		} else {
			log.Println("get news:", err)
		}
	}
	delete(a.convs, m.Chat.ID)
}

// Run starts the main application logic and blocks until the context is
// cancelled. It launches goroutines for updates and scheduled messages.
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
	case "/get_last_24h_news":
		a.handleGetLast24hNewsCommand(ctx, m)
	case "/topics":
		a.handleTopicsCommand(ctx, m)
	case "/my_topics":
		a.handleMyTopicsCommand(ctx, m)
	case "/update_topics":
		a.handleUpdateTopicsCommand(ctx, m)
	case "/add_topic", "/add_topics":
		a.handleAddTopicCommand(ctx, m)
	case "/delete_topics":
		a.handleDeleteTopicsCommand(ctx, m)
	case "/info":
		a.handleInfoCommand(ctx, m)
	case "/tariffs":
		a.handleTariffsCommand(ctx, m)
	case "/sett":
		a.handleSetTariffCommand(ctx, m)
		//case "/test":
	//	a.handleTestCmd(ctx, m)
	default:
		log.Printf("user %d(@%s) texted: %s", m.Chat.ID, m.Chat.Username, m.Text)
		promt := a.messages["unknown_text"]
		a.sendMessage(ctx, m.Chat.ID, promt, nil)
	}
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
}

// setCommands registers the list of bot commands with Telegram so that users
// see available commands in the UI.
func (a *App) setCommands(ctx context.Context) {
	cmds := []telegram.BotCommand{
		{Command: "start", Description: "Начать взаимодействие со мной"},
		{Command: "info", Description: "Посмотреть доступные команды"},
		{Command: "topics", Description: "Управление категориями и типам информации"},
		{Command: "tariffs", Description: "Посмотреть существующие тарифы и их возможности"},
		{Command: "get_news_now", Description: "Получить информацию по заданной категории сейчас"},
		{Command: "get_last_24h_news", Description: "Получить новости за 24 часа по заданной категории сейчас"},
		{Command: "stop", Description: "Остановить отправку сообщений"},
		//{Command: "update_topics", Description: "Обновить категории и типы информации"},
		//{Command: "add_topic", Description: "Добавить категории с типом информации"},
		//{Command: "delete_topics", Description: "Удалить категории"},
		//{Command: "my_topics", Description: "Показать список заданных категорий и типов информации"},
	}
	if err := a.tgClient.SetCommands(ctx, cmds); err != nil {
		log.Println("set commands:", err)
	}
}

// setUserTariff changes the tariff for the specified username.
func (a *App) setUserTariff(ctx context.Context, username, tariff string) error {
	users, err := a.repo.List(ctx)
	if err != nil {
		return err
	}
	var user *model.UserSettings
	for _, u := range users {
		if strings.EqualFold(u.UserName, username) {
			user = u
			break
		}
	}
	if user == nil {
		return fmt.Errorf("user %s not found", username)
	}
	if _, ok := a.cfg.Tariffs[tariff]; !ok {
		return fmt.Errorf("unknown tariff")
	}
	user.Tariff = tariff
	return a.repo.Save(ctx, user)
}

// continueConversation processes messages that are part of a multi-step dialog
// and advances the conversation state machine accordingly.
func (a *App) continueConversation(ctx context.Context, m *telegram.Message, c *conversationState) {
	if strings.EqualFold(m.Text, "Отмена") {
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		a.sendMessage(ctx, m.Chat.ID, a.messages["cancelled"], nil)
		delete(a.convs, m.Chat.ID)
		return
	}
	if strings.EqualFold(m.Text, "Назад") && c.PrevStage == 0 {
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		c.back()
		a.continueConversation(ctx, &telegram.Message{Chat: m.Chat}, c)
		return
	}

	if a.continueStartFlow(ctx, m, c) {
		return
	}
	if a.continueUpdateFlow(ctx, m, c) {
		return
	}
	if a.continueDeleteFlow(ctx, m, c) {
		return
	}
	if a.continueNewsFlow(ctx, m, c) {
		return
	}
	if a.continueLast24hFlow(ctx, m, c) {
		return
	}
	if a.continueSetTariffFlow(ctx, m, c) {
		return
	}
}

func (a *App) continueStartFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	if c.UpdateTopics || c.DeleteTopics {
		return false
	}
	switch c.Stage {
	case stageWelcome:
		if strings.TrimSpace(m.Text) != "Продолжить" {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["press_continue"], [][]string{{"Продолжить"}})
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		t := a.cfg.Tariffs["base"]
		c.Step = 0
		c.CategoryLimit = t.Limits.CategoryLimit
		c.InfoLimit = t.Limits.InfoTypeLimit
		c.AllowCustomCategory = t.AllowCustomCategory
		c.setStage(stageChooseCategoryCount)
		prompt := fmt.Sprintf(a.messages["prompt_choose_count"], c.CategoryLimit)
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboard(c.CategoryLimit)))
		c.LastMsgID = msgID
		return true
	case stageChooseCategoryCount, stageCategory, stageCustomCategory, stageInfoTypes:
		return a.continueUpdateFlow(ctx, m, c)
	}
	return false
}

func (a *App) continueUpdateFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	if !c.UpdateTopics || c.DeleteTopics {
		return false
	}
	switch c.Stage {
	case stageChooseCategoryCount:
		count, err := strconv.Atoi(strings.TrimSpace(m.Text))
		if err != nil || count < 1 || count > c.CategoryLimit {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(a.messages["prompt_choose_count"], c.CategoryLimit), addBack(numberKeyboard(c.CategoryLimit)))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		c.CategoryLimit = count
		c.setStage(stageCategory)
		opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
		prompt := fmt.Sprintf(a.messages["prompt_choose_category"], 1, formatOptions(opts))
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
		c.LastMsgID = msgID
		return true
	case stageUpdateChoice:
		choice := parseSelection(m.Text, []string{"Обновить все", "Обновить несколько"}, 1)
		if len(choice) == 0 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_action"], addCancel(numberKeyboard(2)))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if choice[0] == "Обновить несколько" {
			c.AvailableCats = make([]string, 0, len(c.Topics))
			for cat := range c.Topics {
				c.AvailableCats = append(c.AvailableCats, cat)
			}
			c.setStage(stageSelectManyExisting)
			prompt := fmt.Sprintf(a.messages["prompt_choose_existing_multi"], formatOptions(c.AvailableCats))
			if len(c.SelectedCats) > 0 {
				prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedCats, ", "))
			}
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(c.AvailableCats))))
			c.LastMsgID = msgID
			return true
		}
		c.Topics = map[string][]string{}
		c.Step = 0
		c.setStage(stageCategory)
		opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
		prompt := fmt.Sprintf(a.messages["prompt_choose_category"], 1, formatOptions(opts))
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		c.LastMsgID = msgID
		return true
	case stageSelectManyExisting:
		if strings.EqualFold(m.Text, "Готово") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			if len(c.SelectedCats) == 0 {
				a.sendMessage(ctx, m.Chat.ID, a.messages["no_changes"], nil)
				delete(a.convs, m.Chat.ID)
				return true
			}
			c.CategoryLimit = len(c.SelectedCats)
			c.Step = 0
			c.OldCat = c.SelectedCats[0]
			c.setStage(stageCategory)
			opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
			prompt := fmt.Sprintf(a.messages["prompt_choose_new"], c.OldCat, formatOptions(opts))
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
			c.LastMsgID = msgID
			return true
		}
		if strings.EqualFold(m.Text, "Назад") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			c.setStage(stageUpdateChoice)
			c.SelectedCats = nil
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_action"], addCancel(numberKeyboard(2)))
			c.LastMsgID = msgID
			return true
		}
		cats := parseSelection(m.Text, c.AvailableCats, len(c.AvailableCats))
		if len(cats) == 0 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_category_number"], addBack(numberKeyboardWithDone(len(c.AvailableCats))))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		for _, cat := range cats {
			exists := false
			for _, ex := range c.SelectedCats {
				if ex == cat {
					exists = true
					break
				}
			}
			if !exists {
				c.SelectedCats = append(c.SelectedCats, cat)
			}
		}
		prompt := fmt.Sprintf(a.messages["prompt_choose_existing_multi"], formatOptions(c.AvailableCats))
		if len(c.SelectedCats) > 0 {
			prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedCats, ", "))
		}
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboardWithDone(len(c.AvailableCats))))
		c.LastMsgID = msgID
		return true
	case stageCategory:
		opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
		if strings.EqualFold(m.Text, "Готово") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			if c.Step == 0 {
				a.sendMessage(ctx, m.Chat.ID, a.messages["no_changes"], nil)
				delete(a.convs, m.Chat.ID)
				return true
			}
			a.saveTopics(ctx, m, c)
			return true
		}
		if strings.EqualFold(m.Text, "Назад") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			if c.PrevStage == stageSelectManyExisting {
				c.setStage(stageUpdateChoice)
				c.SelectedCats = nil
				m.Text = ""
				msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_action"], addCancel(numberKeyboard(2)))
				c.LastMsgID = msgID
				return true
			}
			c.setStage(stageUpdateChoice)
			c.OldCat = ""
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_action"], addCancel(numberKeyboard(2)))
			c.LastMsgID = msgID
			return true
		}
		cats := parseSelection(m.Text, opts, 1)
		if len(cats) == 0 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_category_number"], addBackCancel(numberKeyboard(len(opts))))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if c.AllowCustomCategory && cats[0] == "😇Своя категория" {
			c.setStage(stageCustomCategory)
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["enter_custom_category"], nil)
			c.LastMsgID = msgID
			return true
		}
		c.CurrentCat = cats[0]
		c.SelectedInfos = nil
		c.setStage(stageInfoTypes)
		prompt := fmt.Sprintf(a.messages["prompt_choose_info"], c.CurrentCat, c.InfoLimit, formatOptions(a.infoOptions))
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(a.infoOptions))))
		c.LastMsgID = msgID
		return true
	case stageCustomCategory:
		words := strings.Fields(m.Text)
		if len(words) < 1 || len(words) > 3 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["enter_words_1_3"], nil)
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		c.CurrentCat = "🫆" + strings.Join(words, " ")
		c.setStage(stageInfoTypes)
		c.SelectedInfos = nil
		prompt := fmt.Sprintf(a.messages["prompt_choose_info"], c.CurrentCat, c.InfoLimit, formatOptions(a.infoOptions))
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(a.infoOptions))))
		c.LastMsgID = msgID
		return true
	case stageInfoTypes:
		if strings.EqualFold(m.Text, "Назад") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			c.setStage(stageCategory)
			opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
			var prompt string
			var msgID int
			if c.OldCat != "" {
				prompt = fmt.Sprintf(a.messages["prompt_choose_new"], c.OldCat, formatOptions(opts))
				msgID, _ = a.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
			} else {
				prompt = fmt.Sprintf(a.messages["prompt_choose_category"], c.Step+1, formatOptions(opts))
				msgID, _ = a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
			}
			c.LastMsgID = msgID
			return true
		}
		if strings.EqualFold(m.Text, "Готово") {
			if len(c.SelectedInfos) == 0 && len(c.Topics[c.CurrentCat]) == 0 {
				prompt := fmt.Sprintf(a.messages["prompt_choose_info"], c.CurrentCat, c.InfoLimit, formatOptions(a.infoOptions))
				if len(c.SelectedInfos) > 0 {
					prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedInfos, ", "))
				}
				msg, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(a.infoOptions))))
				c.LastMsgID = msg
				return true
			}
		} else {
			infos := parseSelection(m.Text, a.infoOptions, c.InfoLimit-len(c.SelectedInfos))
			if len(infos) == 0 {
				prompt := fmt.Sprintf(a.messages["prompt_choose_info"], c.CurrentCat, c.InfoLimit, formatOptions(a.infoOptions))
				if len(c.SelectedInfos) > 0 {
					prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedInfos, ", "))
				}
				msg, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(a.infoOptions))))
				c.LastMsgID = msg
				return true
			}
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			for _, inf := range infos {
				found := false
				for _, ex := range c.SelectedInfos {
					if ex == inf {
						found = true
						break
					}
				}
				if !found && len(c.SelectedInfos) < c.InfoLimit {
					c.SelectedInfos = append(c.SelectedInfos, inf)
				}
			}
			if len(c.SelectedInfos) < c.InfoLimit {
				prompt := fmt.Sprintf(a.messages["prompt_choose_info"], c.CurrentCat, c.InfoLimit, formatOptions(a.infoOptions))
				if len(c.SelectedInfos) > 0 {
					prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedInfos, ", "))
				}
				msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(a.infoOptions))))
				c.LastMsgID = msgID
				return true
			}
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if c.Topics == nil {
			c.Topics = map[string][]string{}
		}
		if c.OldCat != "" {
			delete(c.Topics, c.OldCat)
			c.OldCat = ""
		}
		existing := c.Topics[c.CurrentCat]
		for _, inf := range c.SelectedInfos {
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
		c.SelectedInfos = nil
		c.Step++
		if c.Step >= c.CategoryLimit {
			a.saveTopics(ctx, m, c)
			return true
		}
		if len(c.SelectedCats) > 0 && c.Step < len(c.SelectedCats) {
			c.OldCat = c.SelectedCats[c.Step]
			c.Stage = stageCategory
			opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
			prompt := fmt.Sprintf(a.messages["prompt_choose_new"], c.OldCat, formatOptions(opts))
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
			c.LastMsgID = msgID
			return true
		}
		c.setStage(stageCategory)
		opts := addCustomOption(a.categoryOptions, c.AllowCustomCategory)
		prompt := fmt.Sprintf(a.messages["prompt_choose_category"], c.Step+1, formatOptions(opts))
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
		c.LastMsgID = msgID
		return true
	}
	return false
}

func (a *App) continueDeleteFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	if !c.DeleteTopics {
		return false
	}
	switch c.Stage {
	case stageDeleteChoice:
		if strings.EqualFold(m.Text, "Готово") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			a.sendMessage(ctx, m.Chat.ID, a.messages["no_changes"], nil)
			delete(a.convs, m.Chat.ID)
			return true
		}
		choice := parseSelection(m.Text, []string{"Удалить все", "Удалить несколько"}, 1)
		if len(choice) == 0 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_delete_action"], addBack(numberKeyboardWithDone(2)))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if choice[0] == "Удалить несколько" {
			c.AvailableCats = make([]string, 0, len(c.Topics))
			for cat := range c.Topics {
				c.AvailableCats = append(c.AvailableCats, cat)
			}
			c.setStage(stageSelectDelete)
			prompt := fmt.Sprintf(a.messages["prompt_choose_delete_multi"], formatOptions(c.AvailableCats))
			if len(c.SelectedCats) > 0 {
				prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedCats, ", "))
			}
			msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.AvailableCats))))
			c.LastMsgID = msgID
			return true
		}
		c.Topics = map[string][]string{}
		a.saveTopics(ctx, m, c)
		return true
	case stageSelectDelete:
		if strings.EqualFold(m.Text, "Готово") {
			a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
			a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
			if len(c.SelectedCats) == 0 {
				a.sendMessage(ctx, m.Chat.ID, a.messages["no_changes"], nil)
				delete(a.convs, m.Chat.ID)
				return true
			}
			for _, cat := range c.SelectedCats {
				delete(c.Topics, cat)
			}
			a.saveTopics(ctx, m, c)
			return true
		}
		cats := parseSelection(m.Text, c.AvailableCats, len(c.AvailableCats))
		if len(cats) == 0 {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_category_number"], addBack(numberKeyboardWithDone(len(c.AvailableCats))))
			c.LastMsgID = msg
			return true
		}
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		for _, cat := range cats {
			exists := false
			for _, ex := range c.SelectedCats {
				if ex == cat {
					exists = true
					break
				}
			}
			if !exists {
				c.SelectedCats = append(c.SelectedCats, cat)
			}
		}
		prompt := fmt.Sprintf(a.messages["prompt_choose_delete_multi"], formatOptions(c.AvailableCats))
		if len(c.SelectedCats) > 0 {
			prompt += "\n\n" + fmt.Sprintf(a.messages["already_selected"], strings.Join(c.SelectedCats, ", "))
		}
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboardWithDone(len(c.AvailableCats)))
		c.LastMsgID = msgID
		return true
	}
	return false
}

func (a *App) continueNewsFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	if c.Stage != stageGetNewsCategory {
		return false
	}
	cats := parseSelection(m.Text, c.AvailableCats, 1)
	if len(cats) == 0 {
		msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_category_number"], addCancel(numberKeyboard(len(c.AvailableCats))))
		c.LastMsgID = msg
		return true
	}
	a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
	a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
	tariff, ok := a.cfg.Tariffs[c.Settings.Tariff]
	if !ok {
		tariff = a.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(c.Settings.LastGetNewsNow, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		c.Settings.GetNewsNowCount = 0
	}
	if c.Settings.GetNewsNowCount >= tariff.Limits.GetNewsNowPerDay {
		a.sendMessage(ctx, m.Chat.ID, a.messages["limit_today"], nil)
		delete(a.convs, m.Chat.ID)
		return true
	}
	c.Settings.GetNewsNowCount++
	c.Settings.LastGetNewsNow = now.Unix()
	if err := a.repo.Save(ctx, c.Settings); err != nil {
		log.Println("save settings:", err)
	}
	msg, err := a.userService.GetNewsForCategoryMultiInfo(ctx, c.Settings, cats[0])
	if err != nil {
		log.Println("get news:", err)
		delete(a.convs, m.Chat.ID)
		return true
	}
	if len([]rune(msg)) > 4096 {
		if err := a.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
			log.Println("send msg err: ", err)
		}
	} else {
		a.sendMessage(ctx, m.Chat.ID, msg, nil)
	}
	delete(a.convs, m.Chat.ID)
	return true
}

func (a *App) continueLast24hFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	if c.Stage != stageGetLast24hCategory {
		return false
	}
	cats := parseSelection(m.Text, c.AvailableCats, 1)
	if len(cats) == 0 {
		msg, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_category_number"], addCancel(numberKeyboard(len(c.AvailableCats))))
		c.LastMsgID = msg
		return true
	}
	a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
	a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)

	tariff, ok := a.cfg.Tariffs[c.Settings.Tariff]
	if !ok {
		tariff = a.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(c.Settings.LastGetLast24h, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		c.Settings.GetLast24hCount = 0
	}
	if c.Settings.GetLast24hCount >= tariff.Limits.GetLast24hNewPerDay {
		a.sendMessage(ctx, m.Chat.ID, a.messages["limit_today"], nil)
		delete(a.convs, m.Chat.ID)
		return true
	}

	msgWait, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["wait_search"], nil)

	msg, err := a.userService.GetLast24hNewsForCategory(ctx, c.Settings, cats[0])
	if err != nil {
		log.Println("get news:", err)
		delete(a.convs, m.Chat.ID)
		return true
	}

	a.deleteMessage(ctx, m.Chat.ID, msgWait)
	c.Settings.GetLast24hCount++
	c.Settings.LastGetLast24h = now.Unix()
	if err := a.repo.Save(ctx, c.Settings); err != nil {
		log.Println("save settings:", err)
	}
	if len([]rune(msg)) > 4096 {
		if err := a.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
			log.Println("send msg err: ", err)
		}
	} else {
		_, err = a.sendMessage(ctx, m.Chat.ID, msg, nil)
		if err != nil {
			log.Println("send msg err: ", err)
		}
	}
	delete(a.convs, m.Chat.ID)
	return true
}

func (a *App) continueSetTariffFlow(ctx context.Context, m *telegram.Message, c *conversationState) bool {
	switch c.Stage {
	case stageSetTariffUser:
		username := strings.TrimPrefix(strings.TrimSpace(m.Text), "@")
		if username == "" {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, "Введите username пользователя", addBack(nil))
			c.LastMsgID = msg
			return true
		}
		c.TargetUser = username
		tariffs := []string{"base", "plus", "premium", "ultimate"}
		prompt := "Выберите тариф"
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addBack([][]string{tariffs}))
		c.setStage(stageSetTariffChoice)
		c.LastMsgID = msgID
		return true
	case stageSetTariffChoice:
		choice := strings.TrimSpace(m.Text)
		if choice != "base" && choice != "plus" && choice != "premium" && choice != "ultimate" {
			msg, _ := a.sendMessage(ctx, m.Chat.ID, "Выберите тариф", addBack([][]string{{"base", "plus", "premium", "ultimate"}}))
			c.LastMsgID = msg
			return true
		}
		c.NewTariff = choice
		a.deleteMessage(ctx, m.Chat.ID, m.MessageID)
		a.deleteMessage(ctx, m.Chat.ID, c.LastMsgID)
		if err := a.setUserTariff(ctx, c.TargetUser, c.NewTariff); err != nil {
			a.sendMessage(ctx, m.Chat.ID, "Ошибка: "+err.Error(), nil)
		} else {
			a.sendMessage(ctx, m.Chat.ID, "Тариф обновлен", nil)
		}
		delete(a.convs, m.Chat.ID)
		return true
	}
	return false
}
