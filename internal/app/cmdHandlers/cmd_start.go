package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strconv"
	"strings"
)

// handleStartCommand processes the /start command.
// It initializes user settings and begins the welcome conversation
// if the user is unknown. Otherwise it simply reactivates the user.
func (c *CmdHandler) handleStartCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d (@%s) called /start", m.Chat.ID, m.Chat.Username)

	//  если пользователь заходит впервые и его нет в базе, то просим задать его категории информации
	if _, err := c.repo.Get(ctx, m.Chat.ID); err != nil {
		conv := &ConversationState{Cmd: StartCmd, Stage: StageStartWelcome}
		c.convs[m.Chat.ID] = conv
		msgID, err := c.sendMessage(ctx, m.Chat.ID, c.messages["start"], [][]string{{"Продолжить"}})
		if err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
		conv.LastMsgID = msgID
		return
	}
	if err := c.userService.Start(ctx, m.Chat.ID, m.Chat.Username); err != nil {
		log.Println("start:", err)
	} else {
		if _, err := c.sendMessage(ctx, m.Chat.ID, c.messages["start"], nil); err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
	}
}

func (c *CmdHandler) continueStartFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	switch cs.Stage {
	case StageStartWelcome:
		c.handleStageStartWelcome(ctx, m, cs)
	case StageStartChooseCategoryCount:
		c.handleStageStartChooseCategoryCount(ctx, m, cs)
	case StageStartCategory:
		c.handleStageStartCategory(ctx, m, cs)
	case StageStartCustomCategory:
		c.handleStageStartCustomCategory(ctx, m, cs)
	case StageStartInfoTypes:
		c.handleStageStartInfoTypes(ctx, m, cs)
	}
}

func (c *CmdHandler) handleStageStartWelcome(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.TrimSpace(m.Text) != "Продолжить" {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["press_continue"], [][]string{{"Продолжить"}})
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	t := c.cfg.Tariffs["base"]
	cs.Step = 0
	cs.CategoryLimit = t.Limits.CategoryLimit
	cs.InfoLimit = t.Limits.InfoTypeLimit
	cs.AllowCustomCategory = t.AllowCustomCategory
	cs.setStage(StageStartChooseCategoryCount)
	prompt := fmt.Sprintf(c.messages["prompt_choose_count"], cs.CategoryLimit)
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(cs.CategoryLimit))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageStartChooseCategoryCount(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	count, err := strconv.Atoi(strings.TrimSpace(m.Text))
	if err != nil || count < 1 || count > cs.CategoryLimit {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, fmt.Sprintf(c.messages["prompt_choose_count"], cs.CategoryLimit), addBack(numberKeyboard(cs.CategoryLimit)))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	cs.CategoryLimit = count
	cs.setStage(StageStartCategory)
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], 1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageStartCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)

	cats := parseSelection(m.Text, opts, 1)
	if len(cats) == 0 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_category_number"], addBackCancel(numberKeyboard(len(opts))))
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	if cs.AllowCustomCategory && cats[0] == "😇Своя категория" {
		cs.setStage(StageCustomCategory)
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_custom_category"], nil)
		cs.LastMsgID = msgID
	}
	cs.CurrentCat = cats[0]
	cs.SelectedInfos = nil
	cs.setStage(StageStartInfoTypes)
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))

	var msgID int
	// мы должны отдать другую клавиатуру, чтобы пользователь могу идти только вперёд до сохранения
	if cs.Cmd == StartCmd {
		msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboard(len(c.infoOptions))))
	} else {
		msgID, _ = c.sendMessage(ctx, m.Chat.ID, prompt, addBackCancel(numberKeyboard(len(c.infoOptions))))
	}

	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageStartCustomCategory(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	words := strings.Fields(m.Text)
	if len(words) < 1 || len(words) > 3 {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["enter_words_1_3"], nil)
		cs.LastMsgID = msg
	}
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

	cs.CurrentCat = "🫆" + strings.Join(words, " ")
	cs.setStage(StageStartInfoTypes)
	cs.SelectedInfos = nil
	prompt := fmt.Sprintf(c.messages["prompt_choose_info"], cs.CurrentCat, cs.InfoLimit, formatOptions(c.infoOptions))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(c.infoOptions))))
	cs.LastMsgID = msgID
}

func (c *CmdHandler) handleStageStartInfoTypes(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Назад") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		cs.back()
		cs.SelectedInfos = nil
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.Step+1, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
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
			c.sendAnswerChooseInfo(ctx, m, cs, addBackDone(numberKeyboard(len(c.infoOptions))))
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
		cs.Stage = StageStartCategory
		opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
		prompt := fmt.Sprintf(c.messages["prompt_choose_new"], cs.OldCat, formatOptions(opts))
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
		cs.LastMsgID = msgID
	}
	cs.setStage(StageStartCategory)
	opts := addCustomOption(c.categoryOptions, cs.AllowCustomCategory)
	prompt := fmt.Sprintf(c.messages["prompt_choose_category"], cs.Step+1, formatOptions(opts))
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboard(len(opts)))
	cs.LastMsgID = msgID
}
