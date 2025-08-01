package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

// handleUpdateTopicsCommand launches the flow for updating all topics.
func (a *App) handleUpdateTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /update_topics", m.Chat.ID, m.Chat.Username)
	tariff := a.cfg.Tariffs["base"]
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err == nil {
		if t, ok := a.cfg.Tariffs[settings.Tariff]; ok {
			tariff = t
		}
	}
	conv := &conversationState{UpdateTopics: true, CategoryLimit: tariff.Limits.CategoryLimit, InfoLimit: tariff.Limits.InfoTypeLimit, AllowCustomCategory: tariff.AllowCustomCategory}
	if err == nil && len(settings.Topics) > 0 {
		conv.Stage = stageUpdateChoice
		conv.Topics = make(map[string][]string, len(settings.Topics))
		for k, v := range settings.Topics {
			conv.Topics[k] = append([]string(nil), v...)
		}
		a.convs[m.Chat.ID] = conv
		msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_action"], addCancel(numberKeyboard(2)))
		conv.LastMsgID = msgID
		return
	}
	conv.Stage = stageCategory
	a.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(a.messages["prompt_choose_category"], 1, formatOptions(a.categoryOptions))
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(a.categoryOptions))))
	conv.LastMsgID = msgID
}

// handleAddTopicCommand allows adding additional topics without resetting all settings.
func (a *App) handleAddTopicCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /add_topic", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.sendMessage(ctx, m.Chat.ID, a.messages["start_first"], nil)
		return
	}
	tariff := a.cfg.Tariffs["base"]
	if t, ok := a.cfg.Tariffs[settings.Tariff]; ok {
		tariff = t
	}
	if len(settings.Topics) >= tariff.Limits.CategoryLimit {
		a.sendMessage(ctx, m.Chat.ID, a.messages["limit_categories"], nil)
		return
	}
	conv := &conversationState{
		UpdateTopics:        true,
		CategoryLimit:       tariff.Limits.CategoryLimit - len(settings.Topics),
		InfoLimit:           tariff.Limits.InfoTypeLimit,
		AllowCustomCategory: tariff.AllowCustomCategory,
		Topics:              make(map[string][]string, len(settings.Topics)),
	}
	for k, v := range settings.Topics {
		conv.Topics[k] = append([]string(nil), v...)
	}
	conv.Stage = stageCategory
	a.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(a.messages["prompt_choose_category"], len(conv.Topics)+1, formatOptions(a.categoryOptions))
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(a.categoryOptions))))
	conv.LastMsgID = msgID
}

// handleTopicsCommand shows the topics submenu.
func (a *App) handleTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /topics", m.Chat.ID, m.Chat.Username)
	a.sendMessage(ctx, m.Chat.ID, a.messages["topics_menu"], nil)
}

// handleDeleteTopicsCommand removes selected topics from user preferences.
func (a *App) handleDeleteTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /delete_topics", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.sendMessage(ctx, m.Chat.ID, a.messages["start_first"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		a.sendMessage(ctx, m.Chat.ID, a.messages["no_topics"], nil)
		return
	}
	conv := &conversationState{UpdateTopics: true, DeleteTopics: true, Topics: make(map[string][]string, len(settings.Topics))}
	for k, v := range settings.Topics {
		conv.Topics[k] = append([]string(nil), v...)
	}
	conv.Stage = stageDeleteChoice
	a.convs[m.Chat.ID] = conv
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, a.messages["choose_delete_action"], addCancel(numberKeyboard(2)))
	conv.LastMsgID = msgID
}

// handleMyTopicsCommand displays user's current topics.
func (a *App) handleMyTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /my_topics", m.Chat.ID, m.Chat.Username)
	settings, err := a.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		a.sendMessage(ctx, m.Chat.ID, a.messages["start_first"], nil)
		return
	}
	parts := []string{}
	for cat, types := range settings.Topics {
		parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
	}
	msg := fmt.Sprintf(a.messages["your_topics"], strings.Join(parts, "\n\n"))
	a.sendMessage(ctx, m.Chat.ID, msg, nil)
}
