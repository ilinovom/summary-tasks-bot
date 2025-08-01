package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

// handleGetNewsNowCommand starts the flow for the /get_news_now command.
// It asks the user to choose a category and records usage stats.
func (a *App) handleGetNewsNowCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /get_news_now", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.sendMessage(ctx, m.Chat.ID, a.messages["start_first"], nil)
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
	if settings.GetNewsNowCount >= tariff.Limits.GetNewsNowPerDay {
		a.sendMessage(ctx, m.Chat.ID, a.messages["limit_today"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		a.sendMessage(ctx, m.Chat.ID, a.messages["no_topics"], nil)
		return
	}
	conv := &conversationState{Stage: stageGetNewsCategory, Settings: settings}
	conv.AvailableCats = make([]string, 0, len(settings.Topics))
	for cat := range settings.Topics {
		conv.AvailableCats = append(conv.AvailableCats, cat)
	}
	a.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(a.messages["prompt_choose_news_cat"], formatOptions(conv.AvailableCats))
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(conv.AvailableCats))))
	conv.LastMsgID = msgID
}

// handleGetLast24hNewsCommand handles the /get_last_24h_news command for Plus tariff users.
func (a *App) handleGetLast24hNewsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /get_last_24h_news", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.sendMessage(ctx, m.Chat.ID, a.messages["start_first"], nil)
		return
	}
	if settings.Tariff != "plus" && settings.Tariff != "premium" && settings.Tariff != "ultimate" {
		a.sendMessage(ctx, m.Chat.ID, a.messages["plus_only"], nil)
		return
	}
	tariff, ok := a.cfg.Tariffs[settings.Tariff]
	if !ok {
		tariff = a.cfg.Tariffs["base"]
	}
	now := time.Now()
	last := time.Unix(settings.LastGetLast24h, 0)
	if now.YearDay() != last.YearDay() || now.Year() != last.Year() {
		settings.GetLast24hCount = 0
	}
	if settings.GetLast24hCount >= tariff.Limits.GetLast24hNewPerDay {
		a.sendMessage(ctx, m.Chat.ID, a.messages["limit_today"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		a.sendMessage(ctx, m.Chat.ID, a.messages["no_topics"], nil)
		return
	}
	conv := &conversationState{Stage: stageGetLast24hCategory, Settings: settings}
	conv.AvailableCats = make([]string, 0, len(settings.Topics))
	for cat := range settings.Topics {
		conv.AvailableCats = append(conv.AvailableCats, cat)
	}
	a.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(a.messages["prompt_choose_last24_cat"], formatOptions(conv.AvailableCats))
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(conv.AvailableCats))))
	conv.LastMsgID = msgID
}
