package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

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
		Cmd:    DeleteTopicsCmd,
		Topics: make(map[string][]string, len(settings.Topics)),
	}

	for k, v := range settings.Topics {
		conv.Topics[k] = append([]string(nil), v...)
	}

	conv.Stage = StageDeleteTopicsChoice
	c.convs[m.Chat.ID] = conv
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_delete_action"], addCancel(numberKeyboard(2)))
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueDeleteFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	switch cs.Stage {
	case StageDeleteTopicsChoice:
		if strings.EqualFold(m.Text, "Готово") {
			c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

			c.sendMessage(ctx, m.Chat.ID, c.messages["no_changes"], nil)
			delete(c.convs, m.Chat.ID)
		}
		choice := parseSelection(m.Text, []string{"Удалить все", "Удалить несколько"}, 1)
		if len(choice) == 0 {
			msg, _ := c.sendMessage(ctx, m.Chat.ID, c.messages["choose_delete_action"], addBack(numberKeyboardWithDone(2)))
			cs.LastMsgID = msg
		}
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		if choice[0] == "Удалить несколько" {
			cs.AvailableCats = make([]string, 0, len(cs.Topics))
			for cat := range cs.Topics {
				cs.AvailableCats = append(cs.AvailableCats, cat)
			}
			cs.setStage(StageDeleteTopicsSelect)
			prompt := fmt.Sprintf(c.messages["prompt_choose_delete_multi"], formatOptions(cs.AvailableCats))
			if len(cs.SelectedCats) > 0 {
				prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedCats, ", "))
			}
			msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack(numberKeyboardWithDone(len(cs.AvailableCats))))
			cs.LastMsgID = msgID

		}
		cs.Topics = map[string][]string{}
		c.saveTopics(ctx, m, cs)

	case StageDeleteTopicsSelect:
		if strings.EqualFold(m.Text, "Готово") {
			c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

			if len(cs.SelectedCats) == 0 {
				c.sendMessage(ctx, m.Chat.ID, c.messages["no_changes"], nil)
				delete(c.convs, m.Chat.ID)
			}
			for _, cat := range cs.SelectedCats {
				delete(cs.Topics, cat)
			}
			c.saveTopics(ctx, m, cs)
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
		prompt := fmt.Sprintf(c.messages["prompt_choose_delete_multi"], formatOptions(cs.AvailableCats))
		if len(cs.SelectedCats) > 0 {
			prompt += "\n\n" + fmt.Sprintf(c.messages["already_selected"], strings.Join(cs.SelectedCats, ", "))
		}
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, numberKeyboardWithDone(len(cs.AvailableCats)))
		cs.LastMsgID = msgID
	}
}
