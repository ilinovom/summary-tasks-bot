package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

// handleUpdateTopicsCommand launches the flow for updating all topics.
func (c *CmdHandler) handleUpdateTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /update_topics", m.Chat.ID, m.Chat.Username)
	tariff := c.cfg.Tariffs["base"]
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err == nil {
		if t, ok := c.cfg.Tariffs[settings.Tariff]; ok {
			tariff = t
		}
	}
	conv := &ConversationState{
		Cmd: UpdateTopicsCmd,
		TopicsConvP: &topicsConvParams{
			CategoryLimit:       tariff.Limits.CategoryLimit,
			InfoLimit:           tariff.Limits.InfoTypeLimit,
			AllowCustomCategory: tariff.AllowCustomCategory,
		},
	}

	if err == nil && len(settings.Topics) > 0 {
		conv.Stage = StageUpdateTopicsChoice
		conv.TopicsConvP.Topics = make(map[string][]string, len(settings.Topics))
		for k, v := range settings.Topics {
			conv.TopicsConvP.Topics[k] = append([]string(nil), v...)
		}
		c.convs[m.Chat.ID] = conv
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		conv.LastMsgID = msgID
		return
	}
	conv.Stage = StageUpdateTopicsCategory
	c.convs[m.Chat.ID] = conv
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(c.categoryOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(c.categoryOptions))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueUpdateFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	switch cs.Stage {
	case StageUpdateTopicsChoice:
		c.handleStageUpdateTopicsUpdateChoice(ctx, m, cs)
	case StageUpdateTopicsSelectManyExisting:
		c.handleStageUpdateTopicsSelectManyExisting(ctx, m, cs)
	case StageUpdateTopicsCategory:
		c.handleStageUpdateTopicsCategory(ctx, m, cs)
	case StageUpdateTopicsCustomCategory:
		c.handleStageUpdateTopicsCustomCategory(ctx, m, cs)
	case StageUpdateTopicsInfoTypes:
		c.handleStageUpdateTopicsInfoTypes(ctx, m, cs)
	}

	return true
}

func (c *CmdHandler) handleStageUpdateTopicsUpdateChoice(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	choice := parseSelection(m.Text, []string{"Обновить все", "Обновить несколько"}, 1)
	if len(choice) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if choice[0] == "Обновить несколько" {
		cs.TopicsConvP.AvailableCats = make([]string, 0, len(cs.TopicsConvP.Topics))
		for cat := range cs.TopicsConvP.Topics {
			cs.TopicsConvP.AvailableCats = append(cs.TopicsConvP.AvailableCats, cat)
		}
		cs.setStage(StageUpdateTopicsSelectManyExisting)
		prompt := fmt.Sprintf(c.messages["prompt_choose_existing_multi"], formatOptions(cs.TopicsConvP.AvailableCats))
		if len(cs.TopicsConvP.SelectedCats) > 0 {
			prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.TopicsConvP.SelectedCats, ", "))
		}
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(cs.TopicsConvP.AvailableCats))))
		cs.LastMsgID = msgID
	}
	cs.TopicsConvP.Topics = map[string][]string{}
	cs.TopicsConvP.CatStep = 0
	cs.setStage(StageUpdateTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsSelectManyExisting(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Готово") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

		if len(cs.TopicsConvP.SelectedCats) == 0 {
			c.sendMessage(ctx, m.Chat.ID, c.messages["no_changes"], nil)
			delete(c.convs, m.Chat.ID)
		}
		cs.TopicsConvP.CategoryLimit = len(cs.TopicsConvP.SelectedCats)
		cs.TopicsConvP.CatStep = 0
		cs.TopicsConvP.OldCat = cs.TopicsConvP.SelectedCats[0]
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.TopicsConvP.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msgID
	}
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
		cs.setStage(StageUpdateTopicsChoice)
		cs.TopicsConvP.SelectedCats = nil
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msgID
	}
	cats := parseSelection(m.Text, cs.TopicsConvP.AvailableCats, len(cs.TopicsConvP.AvailableCats))
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBack(numberKeyboardWithDone(len(cs.TopicsConvP.AvailableCats))))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
	for _, cat := range cats {
		exists := false
		for _, ex := range cs.TopicsConvP.SelectedCats {
			if ex == cat {
				exists = true
				break
			}
		}
		if !exists {
			cs.TopicsConvP.SelectedCats = append(cs.TopicsConvP.SelectedCats, cat)
		}
	}

	prompt := fmt.Sprintf(c.messages["prompt_choose_existing_multi"], formatOptions(cs.TopicsConvP.AvailableCats))
	if len(cs.TopicsConvP.SelectedCats) > 0 {
		prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.TopicsConvP.SelectedCats, ", "))
	}

	if len(cs.TopicsConvP.AvailableCats) == 1 {
		cs.TopicsConvP.CategoryLimit = len(cs.TopicsConvP.SelectedCats)
		cs.TopicsConvP.CatStep = 0
		cs.TopicsConvP.OldCat = cs.TopicsConvP.SelectedCats[0]
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
		prompt = fmt.Sprintf(c.messages["prompt_choose_new"], cs.TopicsConvP.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msgID
	}

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(cs.TopicsConvP.AvailableCats))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)

	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
		if cs.PrevStage == StageUpdateTopicsSelectManyExisting {
			cs.setStage(StageUpdateTopicsChoice)
			cs.TopicsConvP.SelectedCats = nil
			m.Text = ""
			cs.TopicsConvP.OldCat = ""
			msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
			cs.LastMsgID = msgID
		}
		cs.setStage(StageUpdateTopicsChoice)
		cs.TopicsConvP.OldCat = ""
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msgID
	}
	cats := parseSelection(m.Text, opts, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msg
		return
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
	if cs.TopicsConvP.AllowCustomCategory && cats[0] == "😇Своя категория" {
		cs.setStage(StageUpdateTopicsCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
		cs.LastMsgID = msgID
	}
	cs.TopicsConvP.CurrentCat = cats[0]
	//cs.TopicsConvP.SelectedInfos = nil
	cs.setStage(StageUpdateTopicsInfoTypes)
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.TopicsConvP.CurrentCat, cs.TopicsConvP.InfoLimit, formatOptions(c.infoOptions))

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboard(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
	cs.TopicsConvP.CurrentCat = "🫆" + strings.Join(words, " ")
	cs.setStage(StageUpdateTopicsInfoTypes)
	cs.TopicsConvP.SelectedInfos = nil
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.TopicsConvP.CurrentCat, cs.TopicsConvP.InfoLimit, formatOptions(c.infoOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
		var prompt string
		var msgID int
		if cs.TopicsConvP.OldCat != "" {
			prompt = fmt.Sprintf(c.messages["prompt_choose_new"], cs.TopicsConvP.OldCat, formatOptions(opts))
			msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		} else {
			prompt = fmt.Sprintf(c.messages["prompt_choose_category"], cs.TopicsConvP.CatStep+1, formatOptions(opts))
			msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
		}
		cs.LastMsgID = msgID
	}
	if strings.EqualFold(m.Text, "Готово") {
		if len(cs.TopicsConvP.SelectedInfos) == 0 && len(cs.TopicsConvP.Topics[cs.TopicsConvP.CurrentCat]) == 0 {
			c.sendAnswerChooseInfo(ctx, m, cs, false, addBack(numberKeyboardWithDone(len(c.infoOptions))))
		}
	} else {
		infos := parseSelection(m.Text, c.infoOptions, cs.TopicsConvP.InfoLimit-len(cs.TopicsConvP.SelectedInfos))
		if len(infos) == 0 {
			c.sendAnswerChooseInfo(ctx, m, cs, false, addBack(numberKeyboardWithDone(len(c.infoOptions))))
		}
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

		addSelectedInfo(infos, cs)

		if len(cs.TopicsConvP.SelectedInfos) < cs.TopicsConvP.InfoLimit {
			c.sendAnswerChooseInfo(ctx, m, cs, false, addBack(numberKeyboardWithDone(len(c.infoOptions))))
		}
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
	if cs.TopicsConvP.Topics == nil {
		cs.TopicsConvP.Topics = map[string][]string{}
	}
	if cs.TopicsConvP.OldCat != "" {
		delete(cs.TopicsConvP.Topics, cs.TopicsConvP.OldCat)
		cs.TopicsConvP.OldCat = ""
	}
	existing := cs.TopicsConvP.Topics[cs.TopicsConvP.CurrentCat]
	for _, inf := range cs.TopicsConvP.SelectedInfos {
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
	cs.TopicsConvP.Topics[cs.TopicsConvP.CurrentCat] = existing
	cs.TopicsConvP.SelectedInfos = nil
	cs.TopicsConvP.CatStep++
	if cs.TopicsConvP.CatStep >= cs.TopicsConvP.CategoryLimit {
		c.saveTopics(ctx, m, cs)
	}
	if len(cs.TopicsConvP.SelectedCats) > 0 && cs.TopicsConvP.CatStep < len(cs.TopicsConvP.SelectedCats) {
		cs.TopicsConvP.OldCat = cs.TopicsConvP.SelectedCats[cs.TopicsConvP.CatStep]
		cs.Stage = StageUpdateTopicsCategory
		opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.TopicsConvP.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
		cs.LastMsgID = msgID
	}
	cs.setStage(StageUpdateTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.TopicsConvP.CatStep+1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
	cs.LastMsgID = msgID
}
