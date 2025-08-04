package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

type topicsConvParams struct {
	Topics map[string][]string // здесь хранится категория в ключе и типы информации в значении, которые выбрал юзер

	CategoryLimit       int      // здесь хранится всего категорий, которые доступны для выбора пользователем в рамках выполняемой команды
	CatStep             int      // показывает какую категорию по счёту пользователь добавляет
	CurrentCat          string   // это переменная хранит значений выбранной категории stage add/update_topics
	OldCat              string   // здесь сохраняется категория которую мы будем обновлять в рамках /update_topics
	AvailableCats       []string //TODO разобраться что это? и зачем?
	SelectedCats        []string // выбранные категории
	AllowCustomCategory bool     // разрешена ли кастомная категории (тариф != base)

	InfoLimit     int      // здесь хранится всего типов информации, которые доступны для выбора пользователем для 1 категории
	InfoStep      int      // показывает какую категорию по счёту пользователь добавляет
	SelectedInfos []string // выбранные типы информвции
}

func (t *topicsConvParams) increaseCatStep() {
	t.CatStep++
}

func (t *topicsConvParams) decreaseCatStep() {
	t.CatStep--
}

func (t *topicsConvParams) addSelectedCats(sc string) bool {
	if len(t.SelectedCats) == 0 {
		t.increaseCatStep()
		t.SelectedCats = append(t.SelectedCats, sc)
		return true
	}

	for _, cat := range t.SelectedCats {
		if cat == sc {
			return false
		}
	}

	t.increaseCatStep()
	t.SelectedCats = append(t.SelectedCats, sc)
	return true
}

func (t *topicsConvParams) addSelectedInfo(si string) bool {
	if len(t.SelectedInfos) == 0 {
		t.SelectedInfos = append(t.SelectedInfos, si)
		return true
	}

	for _, i := range t.SelectedInfos {
		if i == si {
			return false
		}
	}

	t.SelectedInfos = append(t.SelectedInfos, si)
	return true
}

func (t *topicsConvParams) setNextCat() bool {
	l := len(t.SelectedCats)

	if l == 0 {
		log.Println("ERROR: setNextCat() вызывается, но выбранные категории пустые")
		return false
	}

	if t.CurrentCat == "" {
		t.CurrentCat = t.SelectedCats[0]
		return true
	}

	for i, cat := range t.SelectedCats {
		if t.CurrentCat == cat && i < l-1 {
			t.CurrentCat = t.SelectedCats[i+1]
			return true
		}
	}

	return false
}

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
		Cmd:   AddTopicsCmd,
		Stage: StageAddTopicsCategory,
		TopicsConvP: &topicsConvParams{
			CategoryLimit:       tariff.Limits.CategoryLimit - len(settings.Topics),
			InfoLimit:           tariff.Limits.InfoTypeLimit,
			AllowCustomCategory: tariff.AllowCustomCategory,
			Topics:              make(map[string][]string),
		},
	}

	conv.TopicsConvP.increaseCatStep()

	for k, v := range settings.Topics {
		conv.TopicsConvP.Topics[k] = append([]string(nil), v...)
	}

	c.convs[m.Chat.ID] = conv

	c.sendAnswerChooseCategory(ctx, m, conv, false, addCancel(numberKeyboard(len(c.categoryOptions))))
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
		//case StageAddTopicsAddMore:
		//	c.handleStageAddTopicsAddMore(ctx, m, cs)
	}
}

func (c *CmdHandler) handleStageAddTopicsCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	cs.TopicsConvP.CurrentCat = "🫆" + strings.Join(words, " ")
	c.goToInfoTypesStage(ctx, m, cs, addBackDone(numberKeyboard(len(c.infoOptions))))
}

func (c *CmdHandler) handleStageAddTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, DoneButton) {
		if len(cs.TopicsConvP.SelectedCats) != 0 {
			c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)
			c.goToInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
			return
		}
	}

	// получаем все доступные пользователю категории
	opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
	selectedCat := parseSelectionOne(m.Text, opts)
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

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
	}

	c.goToInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
}

func (c *CmdHandler) handleStageAddTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, DoneButton) {
		cs.TopicsConvP.decreaseCatStep()
		cs.TopicsConvP.Topics[cs.TopicsConvP.CurrentCat] = cs.TopicsConvP.SelectedInfos
		cs.TopicsConvP.SelectedInfos = nil

		if cs.TopicsConvP.setNextCat() {
			c.sendAnswerChooseInfo(ctx, m, cs, false, addCancel(numberKeyboard(len(c.infoOptions))))
			return
		}

		c.saveTopics(ctx, m, cs)
		return
	}

	info := parseSelectionOne(m.Text, c.infoOptions)
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if info == "" {
		c.sendAnswerChooseInfo(ctx, m, cs, false, addCancel(numberKeyboard(len(c.infoOptions))))
		return
	}

	if !cs.TopicsConvP.addSelectedInfo(info) {
		c.sendAnswerChooseInfo(ctx, m, cs, true, addCancel(numberKeyboard(len(c.infoOptions))))
		return
	}

	cs.TopicsConvP.Topics[cs.TopicsConvP.CurrentCat] = cs.TopicsConvP.SelectedInfos

	if len(cs.TopicsConvP.SelectedInfos) < cs.TopicsConvP.InfoLimit {
		c.sendAnswerChooseInfo(ctx, m, cs, false, addCancelDone(numberKeyboard(len(c.infoOptions))))
		return
	}

	cs.TopicsConvP.decreaseCatStep()
	cs.TopicsConvP.SelectedInfos = nil

	if cs.TopicsConvP.setNextCat() {
		c.sendAnswerChooseInfo(ctx, m, cs, false, addCancel(numberKeyboard(len(c.infoOptions))))
		return
	}

	c.saveTopics(ctx, m, cs)

	return

	/*if len(cs.TopicsConvP.SelectedCats) > 0 && cs.TopicsConvP.CatStep < len(cs.TopicsConvP.SelectedCats) {
		cs.TopicsConvP.OldCat = cs.TopicsConvP.SelectedCats[cs.TopicsConvP.CatStep]
		cs.Stage = StageAddTopicsCategory
		opts := addCustomOption(c.categoryOptions, cs.TopicsConvP.AllowCustomCategory)
		//TODO и тут не должен использоваться
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.TopicsConvP.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
		cs.LastMsgID = msgID
	}*/
}

func (c *CmdHandler) handleStageAddTopicsAddMore(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	choice := parseSelection(m.Text, []string{"Да", "Нет"}, 1)
	if len(choice) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if choice[0] == "Нет" {
		c.saveTopics(ctx, m, cs)
		return
	}

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

	//cs.Topics = map[string][]string{}
	//cs.CatStep = 1
	//cs.setStage(StageUpdateTopicsCategory)
	//opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	//prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
	//msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addCancel(numberKeyboard(len(opts))))
	//cs.LastMsgID = msgID
}

func (c *CmdHandler) goToInfoTypesStage(ctx context.Context, m *telegram.Message, cs *ConversationState, kb [][]string) {
	cs.setStage(StageAddTopicsInfoTypes)
	cs.TopicsConvP.setNextCat()

	c.sendAnswerChooseInfo(ctx, m, cs, false, kb)
}
