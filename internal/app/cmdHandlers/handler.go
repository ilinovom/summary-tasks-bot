package cmdHandlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
	"github.com/ilinovom/summary-tasks-bot/internal/service"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"os"
	"strings"
	"time"
)

const (
	StartCmd          = "/start"
	StopCmd           = "/stop"
	GetNewsNowCmd     = "/get_news_now"
	GetLast24hNewsCmd = "/get_last_24h_news"
	TopicsCmd         = "/topics"
	MyTopicsCmd       = "/my_topics"
	UpdateTopicsCmd   = "/update_topics"
	AddTopicsCmd      = "/add_topics"
	DeleteTopicsCmd   = "/delete_topics"
	InfoCmd           = "/info"
	TariffsCmd        = "/tariffs"
	SettCmd           = "/sett"
)

type CmdHandler struct {
	cfg             *config.Config
	tgClient        *telegram.Client
	userService     *service.UserService
	repo            repository.UserSettingsRepository
	convs           map[int64]*ConversationState
	infoOptions     []string
	categoryOptions []string
	messages        map[string]string
}

func NewCmdHandler(cfg *config.Config, userService *service.UserService, repo repository.UserSettingsRepository, tgClient *telegram.Client) *CmdHandler {
	return &CmdHandler{
		cfg:             cfg,
		tgClient:        tgClient,
		userService:     userService,
		repo:            repo,
		infoOptions:     cfg.Options.InfoOptions,
		categoryOptions: cfg.Options.CategoryOptions,
		messages:        cfg.Messages,
		convs:           map[int64]*ConversationState{},
	}
}

func (c *CmdHandler) HandleMessages(ctx context.Context, m *telegram.Message) {
	// пользователь вызвал команду /start и нажал кнопку Продолжить
	if conv, ok := c.convs[m.Chat.ID]; ok && conv.Stage != 0 && m.Text != StartCmd {
		c.continueConversation(ctx, m, conv)
		return
	}

	switch m.Text {
	case StartCmd:
		c.handleStartCommand(ctx, m)
	case StopCmd:
		c.handleStopCommand(ctx, m)
	case InfoCmd:
		c.handleInfoCommand(ctx, m)
	case TariffsCmd:
		c.handleTariffsCommand(ctx, m)
	case TopicsCmd:
		c.handleTopicsCommand(ctx, m)
	case MyTopicsCmd:
		c.handleMyTopicsCommand(ctx, m)

	case UpdateTopicsCmd:
		c.handleUpdateTopicsCommand(ctx, m)
	case AddTopicsCmd:
		c.handleAddTopicCommand(ctx, m)
	case DeleteTopicsCmd:
		c.handleDeleteTopicsCommand(ctx, m)

	case GetNewsNowCmd:
		c.handleGetNewsNowCommand(ctx, m)
	case GetLast24hNewsCmd:
		c.handleGetLast24hNewsCommand(ctx, m)

	case SettCmd:
		c.handleSetTariffCommand(ctx, m)
	default:
		log.Printf("user %d(@%s) texted: %s", m.Chat.ID, m.Chat.Username, m.Text)
		promt := c.messages["unknown_text"]
		c.sendMessage(ctx, m.Chat.ID, promt, nil)
	}
}

// sendMessage is a small wrapper around the Telegram client that logs failures
// but still returns the message ID to the caller.
func (c *CmdHandler) sendMessage(ctx context.Context, chatID int64, text string, kb [][]string) (int, error) {
	msgID, err := c.tgClient.SendMessage(ctx, chatID, text, kb)
	if err != nil {
		log.Printf("telegram send message: %v\ntext: %s", err, text)
	}
	return msgID, err
}

// sendLongMessage splits a long message into several Telegram messages so that
// each part fits into the platform's limit.
func (c *CmdHandler) sendLongMessage(ctx context.Context, chatID int64, text string) error {
	const limit = 4096
	runes := []rune(text)
	for len(runes) > 0 {
		n := limit
		if n > len(runes) {
			n = len(runes)
		}
		part := string(runes[:n])
		if _, err := c.sendMessage(ctx, chatID, part, nil); err != nil {
			return err
		}
		runes = runes[n:]
	}
	return nil
}

// deleteMessage removes a previously sent message and logs any deletion error.
func (c *CmdHandler) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	if err := c.tgClient.DeleteMessage(ctx, chatID, messageID); err != nil {
		log.Printf("telegram delete message: %v", err)
	}
}

// setUserTariff changes the tariff for the specified username.
func (с *CmdHandler) setUserTariff(ctx context.Context, username, tariff string) error {
	users, err := с.repo.List(ctx)
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
	if _, ok := с.cfg.Tariffs[tariff]; !ok {
		return fmt.Errorf("unknown tariff")
	}
	user.Tariff = tariff
	return с.repo.Save(ctx, user)
}

// saveTopics persists the conversation topics to the repository. It also sends
// a confirmation message to the user about the updated or created settings.
func (c *CmdHandler) saveTopics(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	var settings *model.UserSettings
	var err error
	if cs.Cmd == UpdateTopicsCmd || cs.Cmd == AddTopicsCmd {
		settings, err = c.repo.Get(ctx, m.Chat.ID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("save settings:", err)
			delete(c.convs, m.Chat.ID)
			return
		}
		if err != nil && errors.Is(err, os.ErrNotExist) {
			settings = &model.UserSettings{UserID: m.Chat.ID, UserName: m.Chat.Username}
		}
		settings.Topics = cs.Topics
		if err := c.repo.Save(ctx, settings); err != nil {
			log.Println("save settings:", err)
		} else {
			parts := []string{}
			for cat, types := range cs.Topics {
				parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
			}
			c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["settings_updated"], strings.Join(parts, "\n")), nil)
		}
		delete(c.convs, m.Chat.ID)
		return
	}

	settings = &model.UserSettings{
		UserID:            m.Chat.ID,
		UserName:          m.Chat.Username,
		Topics:            cs.Topics,
		Tariff:            "base",
		LastScheduledSent: time.Now().Unix(),
		LastGetNewsNow:    0,
		GetNewsNowCount:   0,
		LastGetLast24h:    0,
		GetLast24hCount:   0,
		Active:            true,
	}
	if err := c.repo.Save(ctx, settings); err != nil {
		log.Println("save settings:", err)
	} else {
		parts := []string{}
		for cat, types := range cs.Topics {
			parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
		}
		c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["settings_saved"], strings.Join(parts, "\n")), nil)
		msg, err := c.userService.GetNewsMultiInfo(ctx, settings)
		if err == nil {
			if len([]rune(msg)) > 4096 {
				if err := c.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
					log.Println("send msg err: ", err)
				}
			} else {
				c.sendMessage(ctx, m.Chat.ID, msg, nil)
			}
		} else {
			log.Println("get news:", err)
		}
	}
	delete(c.convs, m.Chat.ID)
}

// setCommands registers the list of bot commands with Telegram so that users
// see available commands in the UI.
func (c *CmdHandler) SetCommands(ctx context.Context) {
	cmds := []telegram.BotCommand{
		{Command: strings.Replace(StartCmd, "/", "", 1), Description: "Начать взаимодействие со мной"},
		{Command: strings.Replace(InfoCmd, "/", "", 1), Description: "Посмотреть доступные команды"},
		{Command: strings.Replace(TopicsCmd, "/", "", 1), Description: "Управление категориями и типам информации"},
		{Command: strings.Replace(TariffsCmd, "/", "", 1), Description: "Посмотреть существующие тарифы и их возможности"},
		{Command: strings.Replace(GetNewsNowCmd, "/", "", 1), Description: "Получить информацию по заданной категории сейчас"},
		{Command: strings.Replace(GetLast24hNewsCmd, "/", "", 1), Description: "Получить новости за 24 часа по заданной категории сейчас"},
		{Command: strings.Replace(StopCmd, "/", "", 1), Description: "Остановить отправку сообщений"},
		//{Command: "update_topics", Description: "Обновить категории и типы информации"},
		//{Command: "add_topic", Description: "Добавить категории с типом информации"},
		//{Command: "delete_topics", Description: "Удалить категории"},
		//{Command: "my_topics", Description: "Показать список заданных категорий и типов информации"},
	}
	if err := c.tgClient.SetCommands(ctx, cmds); err != nil {
		log.Println("set commands:", err)
	}
}

func (c *CmdHandler) sendAnswerChooseInfo(ctx context.Context, m *telegram.Message, cs *ConversationState, kb [][]string) {
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))
	if len(cs.SelectedInfos) > 0 {
		prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedInfos, ", "))
	}
	msg, _ := c.sendMessage(ctx, m.Chat.ID, prompt, kb)
	cs.LastMsgID = msg
}

func (c *CmdHandler) deleteCurrentAndLastMsg(ctx context.Context, curId int64, lastId int) {
	c.deleteMessage(ctx, curId, lastId)
	c.deleteMessage(ctx, curId, lastId)
}
