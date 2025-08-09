package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/internal/utils"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

func (t *topicsConvParams) addToUpdateCats(uc string) bool {
	if len(t.ToUpdateCats) == 0 {
		t.ToUpdateCats = append(t.ToUpdateCats, uc)
		return true
	}

	for _, cat := range t.ToUpdateCats {
		if cat == uc {
			return false
		}
	}

	t.ToUpdateCats = append(t.ToUpdateCats, uc)
	return true
}

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
			Topics:              make(map[string][]string),
		},
	}

	if err == nil && len(settings.Topics) > 0 {
		conv.Stage = StageUpdateTopicsChoice
		conv.TopicsConvP.Topics = settings.Topics

		c.convs[m.Chat.ID] = conv
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		conv.LastMsgID = msgID
		return
	}

	// если категорий заданных нет в таблице, то переходим на добавление категорий
	conv.Stage = StageAddTopicsCategory
	conv.Cmd = AddTopicsCmd
	c.convs[m.Chat.ID] = conv
	prompt := c.messages["update_without_cats"] + fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(c.categoryOptions))

	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(c.categoryOptions))))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueUpdateFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	switch cs.Stage {
	case StageUpdateTopicsChoice:
		c.handleStageUpdateTopicsUpdateChoice(ctx, m, cs)
	case StageUpdateTopicsSelectManyExisting:
		c.handleStageUpdateTopicsSelectManyExisting(ctx, m, cs)
		//case StageUpdateTopicsCategory:
		//	c.handleStageUpdateTopicsCategory(ctx, m, cs)
		//case StageUpdateTopicsCustomCategory:
		//	c.handleStageUpdateTopicsCustomCategory(ctx, m, cs)
		//case StageUpdateTopicsInfoTypes:
		//	c.handleStageUpdateTopicsInfoTypes(ctx, m, cs)
	}

	return true
}

func (c *CmdHandler) handleStageUpdateTopicsUpdateChoice(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	choice := parseSelectionOne(m.Text, []string{UpdateEverything, UpdateSome})
	if choice == "" {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if choice == UpdateSome {
		cs.setStage(StageUpdateTopicsSelectManyExisting)
		cs.TopicsConvP.ToUpdateCats = make([]string, 0)
		c.sendAnswerChooseExistingMulti(ctx, m, cs, false, addCancel(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	cs.TopicsConvP.Topics = map[string][]string{}
	cs.TopicsConvP.increaseCatStep()
	cs.Cmd = AddTopicsCmd
	cs.setStage(StageAddTopicsCategory)
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.TopicsConvP.CategoryLimit, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsSelectManyExisting(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if strings.EqualFold(m.Text, DoneButton) {
		if len(cs.TopicsConvP.ToUpdateCats) != 0 {
			deleteKeysFromMap(cs.TopicsConvP.Topics, cs.TopicsConvP.ToUpdateCats)
			cs.Cmd = AddTopicsCmd
			cs.setStage(StageAddTopicsCategory)
			cs.TopicsConvP.CategoryLimit = len(cs.TopicsConvP.ToUpdateCats)
			c.sendAnswerChooseCategory(ctx, m, cs, false, addCancel(numberKeyboard(len(c.categoryOptions))))
			return
		}
	}

	selectedCat := parseSelectionOne(m.Text, utils.GetSortedKeys(cs.TopicsConvP.Topics))
	if selectedCat == "" {
		c.sendAnswerChooseExistingMulti(ctx, m, cs, false, addCancel(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	if !cs.TopicsConvP.addToUpdateCats(selectedCat) {
		c.sendAnswerChooseExistingMulti(ctx, m, cs, true, addCancelDone(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	// предлагаем добавлять категории, пока не исчерпан лимит
	if len(cs.TopicsConvP.ToUpdateCats) < cs.TopicsConvP.CategoryLimit {
		c.sendAnswerChooseExistingMulti(ctx, m, cs, false, addCancelDone(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	deleteKeysFromMap(cs.TopicsConvP.Topics, cs.TopicsConvP.ToUpdateCats)
	cs.Cmd = AddTopicsCmd
	cs.setStage(StageAddTopicsCategory)
	cs.TopicsConvP.CategoryLimit = len(cs.TopicsConvP.ToUpdateCats)
	c.sendAnswerChooseCategory(ctx, m, cs, false, addCancel(numberKeyboard(len(c.categoryOptions))))
}

func (c *CmdHandler) handleStageUpdateTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if strings.EqualFold(m.Text, DoneButton) {
		if len(cs.TopicsConvP.SelectedCats) != 0 {
			cs.Cmd = AddTopicsCmd
			c.goToAddTopicsInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
			return
		}
	}

	// получаем все доступные пользователю категории
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
	selectedCat := parseSelectionOne(m.Text, opts)

	// если пользователь выбрал категорию, то добавляем категории в список и увеличиваем шаг
	if selectedCat == "" {
		c.sendAnswerChooseCategory(ctx, m, cs, false, addCancel(numberKeyboard(len(opts))))
		return
	}

	if !cs.TopicsConvP.addSelectedCats(selectedCat) {
		c.sendAnswerChooseCategory(ctx, m, cs, true, addCancelDone(numberKeyboard(len(opts))))
		return
	}

	cs.TopicsConvP.Topics[selectedCat] = nil

	// предлагаем добавлять категории, пока не исчерпан лимит
	if len(cs.TopicsConvP.SelectedCats) < cs.TopicsConvP.CategoryLimit {
		c.sendAnswerChooseCategory(ctx, m, cs, false, addCancelDone(numberKeyboard(len(opts))))
		return
	}

	if cs.TopicsConvP.AllowCustomCategory && selectedCat == "😇Своя категория" {
		cs.setStage(StageAddTopicsCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
		cs.LastMsgID = msgID
		return
	}

	c.goToAddTopicsInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
}

//func (c *CmdHandler) handleStageUpdateTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
//	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
//
//	cats := parseSelection(m.Text, opts, 1)
//	if len(cats) == 0 {
//		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBackCancel(numberKeyboard(len(opts))))
//		cs.LastMsgID = msg
//		return
//	}
//	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
//	if cs.TopicsConvP.AllowCustomCategory && cats[0] == "😇Своя категория" {
//		cs.setStage(StageUpdateTopicsCustomCategory)
//		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
//		cs.LastMsgID = msgID
//	}
//	cs.TopicsConvP.CurrentCat = cats[0]
//	//cs.TopicsConvP.SelectedInfos = nil
//	cs.setStage(StageUpdateTopicsInfoTypes)
//	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.TopicsConvP.CurrentCat, cs.TopicsConvP.InfoLimit, formatOptions(c.infoOptions))
//
//	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboard(len(c.infoOptions))))
//	cs.LastMsgID = msgID
//}

func (c *CmdHandler) handleStageUpdateTopicsCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
	cs.TopicsConvP.CurrentCat = "😇" + strings.Join(words, " ")
	cs.setStage(StageUpdateTopicsInfoTypes)
	cs.TopicsConvP.SelectedInfos = nil
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.TopicsConvP.CurrentCat, cs.TopicsConvP.InfoLimit, formatOptions(c.infoOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageUpdateTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, DoneButton) {
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
