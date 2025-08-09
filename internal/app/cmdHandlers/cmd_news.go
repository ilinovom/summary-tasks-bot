package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"log"
	"time"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

type newsConvParams struct {
	Settings *model.UserSettings // используется только в новостях. подумать чтоы убрать отсюда
}

// handleGetNewsNowCommand starts the flow for the /get_news_now command.
// It asks the user to choose a category and records usage stats.
func (c *CmdHandler) handleGetNewsNowCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /get_news_now", m.Chat.ID, m.Chat.Username)
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		c.sendMessage(ctx, m.Chat.ID, c.messages["start_first"], nil)
		return
	}
	tariff, ok := c.cfg.Tariffs[settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(settings.LastGetNewsNow, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		settings.GetNewsNowCount = 0
	}
	if settings.GetNewsNowCount >= tariff.Limits.GetNewsNowPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		c.sendMessage(ctx, m.Chat.ID, c.messages["no_topics"], nil)
		return
	}

	conv := &ConversationState{
		Cmd:   GetNewsNowCmd,
		Stage: StageGetNewsCategory,
		NewsConvP: &newsConvParams{
			Settings: settings,
		},
		TopicsConvP: &topicsConvParams{
			Topics: settings.Topics,
		},
	}

	setCategories := getKeys(settings.Topics)

	c.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(c.messages["prompt_choose_news_cat"], formatOptions(setCategories))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(setCategories))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueNewsFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	if cs.Cmd != GetNewsNowCmd {
		return false
	}

	setCategories := getKeys(cs.TopicsConvP.Topics)

	cat := parseSelectionOne(m.Text, setCategories)
	if cat == "" {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addCancel(numberKeyboard(len(setCategories))))
		cs.LastMsgID = msg
		return true
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	tariff, ok := c.cfg.Tariffs[cs.NewsConvP.Settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}

	now := time.Now()
	last := time.Unix(cs.NewsConvP.Settings.LastGetNewsNow, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		cs.NewsConvP.Settings.GetNewsNowCount = 0
	}
	if cs.NewsConvP.Settings.GetNewsNowCount >= tariff.Limits.GetNewsNowPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		delete(c.convs, m.Chat.ID)
		return true
	}
	cs.NewsConvP.Settings.GetNewsNowCount++
	cs.NewsConvP.Settings.LastGetNewsNow = now.Unix()
	if err := c.repo.Save(ctx, cs.NewsConvP.Settings); err != nil {
		log.Println("save settings:", err)
	}

	msgWait, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["wait_search"], nil)

	msg, err := c.userService.GetNewsForCategoryMultiInfo(ctx, cs.NewsConvP.Settings, cat)
	if err != nil {
		log.Println("get news:", err)
		delete(c.convs, m.Chat.ID)
		return true
	}

	c.deleteMessage(ctx, m.Chat.ID, msgWait)

	if len([]rune(msg)) > 4096 {
		if err := c.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
			log.Println("send msg err: ", err)
		}
	} else {
		c.sendMessage(ctx, m.Chat.ID, msg, nil)
	}
	delete(c.convs, m.Chat.ID)
	return true
}

// handleGetLast24hNewsCommand handles the /get_last_24h_news command for Plus tariff users.
func (c *CmdHandler) handleGetLast24hNewsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /get_last_24h_news", m.Chat.ID, m.Chat.Username)
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		c.sendMessage(ctx, m.Chat.ID, c.messages["start_first"], nil)
		return
	}
	if settings.Tariff != "plus" && settings.Tariff != "premium" && settings.Tariff != "ultimate" {
		c.sendMessage(ctx, m.Chat.ID, c.messages["plus_only"], nil)
		return
	}
	tariff, ok := c.cfg.Tariffs[settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(settings.LastGetLast24h, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		settings.GetLast24hCount = 0
	}
	if settings.GetLast24hCount >= tariff.Limits.GetLast24hNewPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		c.sendMessage(ctx, m.Chat.ID, c.messages["no_topics"], nil)
		return
	}
	conv := &ConversationState{
		Cmd:   GetLast24hNewsCmd,
		Stage: stageGetLast24hCategory,
		NewsConvP: &newsConvParams{
			Settings: settings,
		},
		TopicsConvP: &topicsConvParams{
			Topics: settings.Topics,
		},
	}

	c.convs[m.Chat.ID] = conv
	cats := getKeys(conv.TopicsConvP.Topics)
	prompt := fmt.Sprintf(c.messages["prompt_choose_last24_cat"], formatOptions(cats))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(cats))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueLast24hFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	if cs.Cmd != GetLast24hNewsCmd {
		return false
	}

	setCategories := getKeys(cs.TopicsConvP.Topics)

	cat := parseSelectionOne(m.Text, setCategories)
	if cat == "" {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addCancel(numberKeyboard(len(setCategories))))
		cs.LastMsgID = msg
		return true
	}

	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	tariff, ok := c.cfg.Tariffs[cs.NewsConvP.Settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}

	now := time.Now()
	last := time.Unix(cs.NewsConvP.Settings.LastGetLast24h, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		cs.NewsConvP.Settings.GetLast24hCount = 0
	}

	if cs.NewsConvP.Settings.GetLast24hCount >= tariff.Limits.GetLast24hNewPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		delete(c.convs, m.Chat.ID)
		return true
	}

	msgWait, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["wait_search"], nil)

	msg, err := c.userService.GetLast24hNewsForCategory(ctx, cs.NewsConvP.Settings, cat)
	if err != nil {
		log.Println("get news:", err)
		delete(c.convs, m.Chat.ID)
		return true
	}

	c.deleteMessage(ctx, m.Chat.ID, msgWait)
	cs.NewsConvP.Settings.GetLast24hCount++
	cs.NewsConvP.Settings.LastGetLast24h = now.Unix()
	if err := c.repo.Save(ctx, cs.NewsConvP.Settings); err != nil {
		log.Println("save settings:", err)
	}
	if len([]rune(msg)) > 4096 {
		if err := c.sendLongMessage(ctx, m.Chat.ID, msg); err != nil {
			log.Println("send msg err: ", err)
		}
	} else {
		_, err = c.sendMessage(ctx, m.Chat.ID, msg, nil)
		if err != nil {
			log.Println("send msg err: ", err)
		}
	}
	delete(c.convs, m.Chat.ID)
	return true
}
