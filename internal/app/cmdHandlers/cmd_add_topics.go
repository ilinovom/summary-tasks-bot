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

	CategoryLimit int      // здесь хранится всего категорий, которые доступны для выбора пользователем в рамках выполняемой команды
	CatStep       int      // показывает какую категорию по счёту пользователь добавляет
	CurrentCat    string   // это переменная хранит значений выбранной категории stage add/update_topics
	OldCat        string   // здесь сохраняется категория которую мы будем обновлять в рамках /update_topics
	AvailableCats []string //TODO перевести news на использование других и удалить
	ToUpdateCats  []string
	SelectedCats  []string // выбранные категории

	CustomCatCount      int
	AllowCustomCategory bool // разрешена ли кастомная категории (тариф != base)

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
	opts := addCustomOption(c.categoryOptions, conv.TopicsConvP.AllowCustomCategory)

	c.sendAnswerChooseCategory(ctx, m, conv, false, addCancel(numberKeyboard(len(opts))))
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

	newCatName := "🫆" + strings.Join(words, " ")
	cs.TopicsConvP.Topics[newCatName] = nil
	cs.TopicsConvP.SelectedCats = append(cs.TopicsConvP.SelectedCats, newCatName)
	cs.TopicsConvP.CustomCatCount--

	if cs.TopicsConvP.CustomCatCount > 0 {
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["enter_custom_category"], getNextCustomCat(cs.TopicsConvP.SelectedCats, cs.TopicsConvP.CustomCatCount)), nil)
		cs.LastMsgID = msgID
		return
	}

	cs.TopicsConvP.SelectedCats = removeCustomCats(cs.TopicsConvP.SelectedCats)
	cs.TopicsConvP.setNextCat()
	cs.TopicsConvP.CategoryLimit = cs.TopicsConvP.CategoryLimit - len(cs.TopicsConvP.SelectedCats)
	c.goToAddTopicsInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
}

func (c *CmdHandler) handleStageAddTopicsCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if strings.EqualFold(m.Text, DoneButton) {
		if len(cs.TopicsConvP.SelectedCats) != 0 {
			if cs.checkCustomCategory() {
				cs.setStage(StageAddTopicsCustomCategory)
				msgID, _ := c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["enter_custom_category"], getNextCustomCat(cs.TopicsConvP.SelectedCats, cs.TopicsConvP.CustomCatCount)), nil)
				cs.LastMsgID = msgID
				return
			}
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

	if cs.TopicsConvP.AllowCustomCategory && selectedCat == CustomCatName {
		cs.TopicsConvP.CustomCatCount++
		selectedCat = fmt.Sprintf("%s_%d", selectedCat, cs.TopicsConvP.CustomCatCount)
	}

	if !cs.TopicsConvP.addSelectedCats(selectedCat) {
		c.sendAnswerChooseCategory(ctx, m, cs, true, addCancelDone(numberKeyboard(len(opts))))
		return
	}

	if !strings.Contains(selectedCat, CustomCatName) {
		cs.TopicsConvP.Topics[selectedCat] = nil
	}

	// предлагаем добавлять категории, пока не исчерпан лимит
	if len(cs.TopicsConvP.SelectedCats) < cs.TopicsConvP.CategoryLimit {
		c.sendAnswerChooseCategory(ctx, m, cs, false, addCancelDone(numberKeyboard(len(opts))))
		return
	}

	if cs.checkCustomCategory() {
		cs.setStage(StageAddTopicsCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["enter_custom_category"], getNextCustomCat(cs.TopicsConvP.SelectedCats, cs.TopicsConvP.CustomCatCount)), nil)
		cs.LastMsgID = msgID
		return
	}

	c.goToAddTopicsInfoTypesStage(ctx, m, cs, addCancel(numberKeyboard(len(c.infoOptions))))
}

func (c *CmdHandler) handleStageAddTopicsInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

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

func (c *CmdHandler) goToAddTopicsInfoTypesStage(ctx context.Context, m *telegram.Message, cs *ConversationState, kb [][]string) {
	cs.setStage(StageAddTopicsInfoTypes)
	cs.TopicsConvP.setNextCat()

	c.sendAnswerChooseInfo(ctx, m, cs, false, kb)
}
