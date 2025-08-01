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
		Cmd:                 UpdateTopicsCmd,
		CategoryLimit:       tariff.Limits.CategoryLimit,
		InfoLimit:           tariff.Limits.InfoTypeLimit,
		AllowCustomCategory: tariff.AllowCustomCategory,
	}

	if err == nil && len(settings.Topics) > 0 {
		conv.Stage = StageUpdateTopicsChoice
		conv.Topics = make(map[string][]string, len(settings.Topics))
		for k, v := range settings.Topics {
			conv.Topics[k] = append([]string(nil), v...)
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
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	if choice[0] == "Обновить несколько" {
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
	}
	cs.Topics = map[string][]string{}
	cs.Step = 0
	cs.setStage(StageUpdateTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsSelectManyExisting(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Готово") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		if len(cs.SelectedCats) == 0 {
			c.sendMessage(ctx, m.Chat.ID, c.messages["no_changes"], nil)
			delete(c.convs, m.Chat.ID)
		}
		cs.CategoryLimit = len(cs.SelectedCats)
		cs.Step = 0
		cs.OldCat = cs.SelectedCats[0]
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msgID
	}
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
		cs.setStage(StageUpdateTopicsChoice)
		cs.SelectedCats = nil
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msgID
	}
	cats := parseSelection(m.Text, cs.AvailableCats, len(cs.AvailableCats))
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBack(numberKeyboardWithDone(len(cs.AvailableCats))))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
	for _, cat := range cats {
		exists := false
		for _, ex := range cs.SelectedCats {
			if ex == cat {
				exists = true
				break
			}
		}
		if !exists {
			cs.SelectedCats = append(cs.SelectedCats, cat)
		}
	}

	prompt := fmt.Sprintf(c.messages["prompt_choose_existing_multi"], formatOptions(cs.AvailableCats))
	if len(cs.SelectedCats) > 0 {
		prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedCats, ", "))
	}

	if len(cs.AvailableCats) == 1 {
		cs.CategoryLimit = len(cs.SelectedCats)
		cs.Step = 0
		cs.OldCat = cs.SelectedCats[0]
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt = fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msgID
	}

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(cs.AvailableCats))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)

	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
		if cs.PrevStage == StageUpdateTopicsSelectManyExisting {
			cs.setStage(StageUpdateTopicsChoice)
			cs.SelectedCats = nil
			m.Text = ""
			cs.OldCat = ""
			msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
			cs.LastMsgID = msgID
		}
		cs.setStage(StageUpdateTopicsChoice)
		cs.OldCat = ""
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msgID
	}
	cats := parseSelection(m.Text, opts, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msg
		return
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
	if cs.AllowCustomCategory && cats[0] == "😇Своя категория" {
		cs.setStage(StageUpdateTopicsCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
		cs.LastMsgID = msgID
	}
	cs.CurrentCat = cats[0]
	//cs.SelectedInfos = nil
	cs.setStage(StageUpdateTopicsInfoTypes)
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboard(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
	cs.CurrentCat = "🫆" + strings.Join(words, " ")
	cs.setStage(StageUpdateTopicsInfoTypes)
	cs.SelectedInfos = nil
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
		cs.setStage(StageUpdateTopicsCategory)
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		var prompt string
		var msgID int
		if cs.OldCat != "" {
			prompt = fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
			msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(opts))))
		} else {
			prompt = fmt.Sprintf(c.messages["prompt_choose_category"], cs.Step+1, formatOptions(opts))
			msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
		}
		cs.LastMsgID = msgID
	}
	if strings.EqualFold(m.Text, "Готово") {
		if len(cs.SelectedInfos) == 0 && len(cs.Topics[cs.CurrentCat]) == 0 {
			c.sendAnswerChooseInfo(ctx, m, cs, addBack(numberKeyboardWithDone(len(c.infoOptions))))
		}
	} else {
		infos := parseSelection(m.Text, c.infoOptions, cs.InfoLimit-len(cs.SelectedInfos))
		if len(infos) == 0 {
			c.sendAnswerChooseInfo(ctx, m, cs, addBack(numberKeyboardWithDone(len(c.infoOptions))))
		}
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		addInfosInfoSelected(infos, cs)

		if len(cs.SelectedInfos) < cs.InfoLimit {
			c.sendAnswerChooseInfo(ctx, m, cs, addBack(numberKeyboardWithDone(len(c.infoOptions))))
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
		cs.Stage = StageUpdateTopicsCategory
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
		cs.LastMsgID = msgID
	}
	cs.setStage(StageUpdateTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.Step+1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(opts))))
	cs.LastMsgID = msgID
}
