package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

// handleAddTopicCommand allows adding additional topics without resetting all settings.
func (c *CmdHandler) handleAddTopicCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /add_topic", m.Chat.ID, m.Chat.Username)
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		c.sendMessage(ctx, m.Chat.ID, c.messages["start_first"], nil)
		return
	}
	tariff := c.cfg.Tariffs["base"]
	if t, ok := c.cfg.Tariffs[settings.Tariff]; ok {
		tariff = t
	}
	if len(settings.Topics) >= tariff.Limits.CategoryLimit {
		c.sendMessage(ctx, m.Chat.ID, c.messages["limit_categories"], nil)
		return
	}
	conv := &ConversationState{
		Cmd:                 AddTopicsCmd,
		Stage:               StageAddTopicsCategory,
		CategoryLimit:       tariff.Limits.CategoryLimit - len(settings.Topics),
		InfoLimit:           tariff.Limits.InfoTypeLimit,
		AllowCustomCategory: tariff.AllowCustomCategory,
		Topics:              make(map[string][]string, len(settings.Topics)),
	}
	for k, v := range settings.Topics {
		conv.Topics[k] = append([]string(nil), v...)
	}

	c.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], len(conv.Topics)+1, formatOptions(c.categoryOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(c.categoryOptions))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueAddFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	switch cs.Stage {
	//case StageAddTopicsUpdateChoice:
	//
	case StageAddTopicsCustomCategory:
		c.handleStageAddTopicsCustomCategory(ctx, m, cs)
	case StageAddTopicsCategory:
		c.handleStageAddTopicsCategory(ctx, m, cs)
	case StageAddTopicsInfoTypes:
		c.handleStageAddTopicsInfoTypes(ctx, m, cs)
	case StageAddTopicsAddMore:
		c.handleStageAddTopicsAddMore(ctx, m, cs)
	}
}

//func (c *CmdHandler) handleStageAddTopicsUpdateChoice(ctx context.Context, m *telegram.Message, cs *ConversationState) {
//	choice := parseSelection(m.Text, []string{"Обновить все", "Обновить несколько"}, 1)
//	if len(choice) == 0 {
//		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
//		cs.LastMsgID = msg
//	}
//	c.deleteMessage(ctx, m.Chat.ID, m.MessageID)
//	c.deleteMessage(ctx, m.Chat.ID, cs.LastMsgID)
//	if choice[0] == "Обновить несколько" {
//		cs.AvailableCats = make([]string, 0, len(cs.Topics))
//		for cat := range cs.Topics {
//			cs.AvailableCats = append(cs.AvailableCats, cat)
//		}
//		cs.setStage(StageUpdateTopicsSelectManyExisting)
//		prompt := fmt.Sprintf(c.messages["prompt_choose_existing_multi"], formatOptions(cs.AvailableCats))
//		if len(cs.SelectedCats) > 0 {
//			prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedCats, ", "))
//		}
//		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(cs.AvailableCats))))
//		cs.LastMsgID = msgID
//	}
//	cs.Topics = map[string][]string{}
//	cs.Step = 0
//	cs.setStage(StageAddTopicsCategory)
//	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
//	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
//	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
//	cs.LastMsgID = msgID
//}

func (c *CmdHandler) handleStageAddTopicsCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	cs.CurrentCat = "🫆" + strings.Join(words, " ")
	cs.setStage(StageAddTopicsInfoTypes)
	cs.SelectedInfos = nil
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageAddTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Готово") {
		if len(cs.Topics) == 0 {
			c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
			c.saveTopics(ctx, m, cs)
			delete(c.convs, m.Chat.ID)
			return
		}
	}

	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	cats := parseSelection(m.Text, opts, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addCancelDone(numberKeyboard(len(opts))))
		cs.LastMsgID = msg
		return
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	if cs.AllowCustomCategory && cats[0] == "😇Своя категория" {
		cs.setStage(StageAddTopicsCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
		cs.LastMsgID = msgID
	}
	cs.CurrentCat = cats[0]
	cs.setStage(StageAddTopicsInfoTypes)
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackDone(numberKeyboard(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageAddTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		cs.back()
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)

		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msgID
	}
	if strings.EqualFold(m.Text, "Готово") {
		cs.Step++
		cs.SelectedInfos = nil
		cs.Topics[cs.CurrentCat] = cs.SelectedInfos
		if cs.Step >= cs.CategoryLimit {
			c.saveTopics(ctx, m, cs)
			return
		}
		if len(cs.SelectedInfos) != 0 {
			cs.Topics[cs.CurrentCat] = cs.SelectedInfos
			c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

			cs.setStage(StageAddTopicsAddMore)
			msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["add_more"], addCancel(numberKeyboard(2)))
			cs.LastMsgID = msg
			return
		}
	} else {
		infos := parseSelection(m.Text, c.infoOptions, cs.InfoLimit-len(cs.SelectedInfos))
		if len(infos) == 0 {
			c.sendAnswerChooseInfo(ctx, m, cs, addBackDone(numberKeyboard(len(c.infoOptions))))
		}

		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		addInfosInfoSelected(infos, cs)

		if len(cs.SelectedInfos) < cs.InfoLimit {
			c.sendAnswerChooseInfo(ctx, m, cs, addBack(numberKeyboardWithDone(len(c.infoOptions))))
			return
		}
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	if cs.Topics == nil {
		cs.Topics = map[string][]string{}
	}
	if cs.OldCat != "" {
		delete(cs.Topics, cs.OldCat)
		cs.OldCat = ""
	}
	existing := cs.Topics[cs.CurrentCat]
	for _, inf := range cs.SelectedInfos {
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
	cs.Topics[cs.CurrentCat] = existing
	cs.SelectedInfos = nil
	cs.Step++
	if cs.Step >= cs.CategoryLimit {
		c.saveTopics(ctx, m, cs)
	}
	if len(cs.SelectedCats) > 0 && cs.Step < len(cs.SelectedCats) {
		cs.OldCat = cs.SelectedCats[cs.Step]
		cs.Stage = StageAddTopicsCategory
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
		cs.LastMsgID = msgID
	}
	cs.setStage(StageAddTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.Step+1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageAddTopicsAddMore(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	choice := parseSelection(m.Text, []string{"Да", "Нет"}, 1)
	if len(choice) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	if choice[0] == "Нет" {
		c.saveTopics(ctx, m, cs)
		return
	}

	cs.AvailableCats = make([]string, 0, len(cs.Topics))
	for cat := range cs.Topics {
		cs.AvailableCats = append(cs.AvailableCats, cat)
	}
	cs.setStage(StageUpdateTopicsSelectManyExisting)
	prompt := fmt.Sprintf(c.messages["prompt_choose_existing_multi"], formatOptions(cs.AvailableCats))
	if len(cs.SelectedCats) > 0 {
		prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedCats, ", "))
	}
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(cs.AvailableCats))))
	cs.LastMsgID = msgID

	//cs.Topics = map[string][]string{}
	//cs.Step = 1
	//cs.setStage(StageUpdateTopicsCategory)
	//opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	//prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
	//msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
	//cs.LastMsgID = msgID
}
