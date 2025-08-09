package cmdHandlers

import (
	"context"
	"github.com/ilinovom/summary-tasks-bot/internal/utils"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

//
//func (t *topicsConvParams) addDeletedCats(dc string) bool {
//	if len(t.SelectedCats) == 0 {
//		t.increaseCatStep()
//		t.SelectedCats = append(t.SelectedCats, dc)
//		return true
//	}
//
//	for _, cat := range t.SelectedCats {
//		if cat == dc {
//			return false
//		}
//	}
//
//	t.increaseCatStep()
//	t.SelectedCats = append(t.SelectedCats, dc)
//	return true
//}

// handleDeleteTopicsCommand removes selected topics from user preferences.
func (c *CmdHandler) handleDeleteTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /delete_topics", m.Chat.ID, m.Chat.Username)
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		c.sendMessage(ctx, m.Chat.ID, c.messages["start_first"], nil)
		return
	}
	if len(settings.Topics) == 0 {
		c.sendMessage(ctx, m.Chat.ID, c.messages["no_topics"], nil)
		return
	}
	conv := &ConversationState{
		Cmd: DeleteTopicsCmd,
		TopicsConvP: &topicsConvParams{
			Topics: settings.Topics,
		},
	}

	for k, v := range settings.Topics {
		conv.TopicsConvP.Topics[k] = append([]string(nil), v...)
	}

	conv.Stage = StageDeleteTopicsChoice
	c.convs[m.Chat.ID] = conv
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_delete_action"], addCancel(numberKeyboard(2)))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueDeleteFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	switch cs.Stage {
	case StageDeleteTopicsChoice:
		c.handleStageDeleteTopicsChoice(ctx, m, cs)
		return
	case StageDeleteTopicsSelect:
		c.handleStageDeleteTopicsSelect(ctx, m, cs)
		return
	}
}

func (c *CmdHandler) handleStageDeleteTopicsChoice(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	choice := parseSelectionOne(m.Text, []string{DeleteEverything, DeleteSome})
	if choice == "" {
		msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_delete_action"], addCancel(numberKeyboard(2)))
		cs.LastMsgID = msg
		return
	}

	if choice == DeleteSome {
		cs.setStage(StageDeleteTopicsSelect)
		//TODO надо ли флаг прокидывать?
		c.sendDeleteChooseMultiCategory(ctx, m, cs, addCancel(numberKeyboard(len(cs.TopicsConvP.Topics))))

		return
	}

	cs.TopicsConvP.Topics = map[string][]string{}
	c.saveTopics(ctx, m, cs)
}

func (c *CmdHandler) handleStageDeleteTopicsSelect(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID, cs.LastMsgID)

	if strings.EqualFold(m.Text, DoneButton) {
		if len(cs.TopicsConvP.SelectedCats) == 0 {
			c.sendMessage(ctx, m.Chat.ID, c.messages["no_changes"], nil)
			delete(c.convs, m.Chat.ID)
		}
		for _, cat := range cs.TopicsConvP.SelectedCats {
			delete(cs.TopicsConvP.Topics, cat)
		}
		c.saveTopics(ctx, m, cs)
		return
	}

	selectedCat := parseSelectionOne(m.Text, utils.GetSortedKeys(cs.TopicsConvP.Topics))
	if selectedCat == "" {
		c.sendAnswerDeleteChooseCategory(ctx, m, cs, false, addCancelDone(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	if !cs.TopicsConvP.addSelectedCats(selectedCat) {
		c.sendAnswerDeleteChooseCategory(ctx, m, cs, true, addCancelDone(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	if len(cs.TopicsConvP.SelectedCats) < len(cs.TopicsConvP.Topics) {
		c.sendAnswerDeleteChooseCategory(ctx, m, cs, false, addCancelDone(numberKeyboard(len(cs.TopicsConvP.Topics))))
		return
	}

	for _, cat := range cs.TopicsConvP.SelectedCats {
		delete(cs.TopicsConvP.Topics, cat)
	}
	c.saveTopics(ctx, m, cs)
}
