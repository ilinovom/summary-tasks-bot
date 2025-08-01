package cmdHandlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

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
	conv := &ConversationState{Cmd: GetNewsNowCmd, Stage: StageGetNewsCategory, Settings: settings}
	conv.AvailableCats = make([]string, 0, len(settings.Topics))
	for cat := range settings.Topics {
		conv.AvailableCats = append(conv.AvailableCats, cat)
	}
	c.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(c.messages["prompt_choose_news_cat"], formatOptions(conv.AvailableCats))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(conv.AvailableCats))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueNewsFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	if cs.Stage != StageGetNewsCategory {
		return false
	}
	cats := parseSelection(m.Text, cs.AvailableCats, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addCancel(numberKeyboard(len(cs.AvailableCats))))
		cs.LastMsgID = msg
		return true
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	tariff, ok := c.cfg.Tariffs[cs.Settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(cs.Settings.LastGetNewsNow, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		cs.Settings.GetNewsNowCount = 0
	}
	if cs.Settings.GetNewsNowCount >= tariff.Limits.GetNewsNowPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		delete(c.convs, m.Chat.ID)
		return true
	}
	cs.Settings.GetNewsNowCount++
	cs.Settings.LastGetNewsNow = now.Unix()
	if err := c.repo.Save(ctx, cs.Settings); err != nil {
		log.Println("save settings:", err)
	}
	msg, err := c.userService.GetNewsForCategoryMultiInfo(ctx, cs.Settings, cats[0])
	if err != nil {
		log.Println("get news:", err)
		delete(c.convs, m.Chat.ID)
		return true
	}
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
	conv := &ConversationState{Cmd: GetLast24hNewsCmd, Stage: stageGetLast24hCategory, Settings: settings}
	conv.AvailableCats = make([]string, 0, len(settings.Topics))
	for cat := range settings.Topics {
		conv.AvailableCats = append(conv.AvailableCats, cat)
	}
	c.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(c.messages["prompt_choose_last24_cat"], formatOptions(conv.AvailableCats))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(conv.AvailableCats))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueLast24hFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	if cs.Stage != stageGetLast24hCategory {
		return false
	}
	cats := parseSelection(m.Text, cs.AvailableCats, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addCancel(numberKeyboard(len(cs.AvailableCats))))
		cs.LastMsgID = msg
		return true
	}

	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	tariff, ok := c.cfg.Tariffs[cs.Settings.Tariff]
	if !ok {
		tariff = c.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(cs.Settings.LastGetLast24h, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		cs.Settings.GetLast24hCount = 0
	}
	if cs.Settings.GetLast24hCount >= tariff.Limits.GetLast24hNewPerDay {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_today"], nil)
		delete(c.convs, m.Chat.ID)
		return true
	}

	msgWait, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["wait_search"], nil)

	msg, err := c.userService.GetLast24hNewsForCategory(ctx, cs.Settings, cats[0])
	if err != nil {
		log.Println("get news:", err)
		delete(c.convs, m.Chat.ID)
		return true
	}

	c.deleteMessage(ctx, m.Chat.ID, msgWait)
	cs.Settings.GetLast24hCount++
	cs.Settings.LastGetLast24h = now.Unix()
	if err := c.repo.Save(ctx, cs.Settings); err != nil {
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
