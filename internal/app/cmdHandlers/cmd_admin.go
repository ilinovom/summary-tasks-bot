package cmdHandlers

import (
	"context"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"strings"
)

const adminUserName = "omilinov"

// handleSetTariffCommand is an admin-only command that changes another user's tariff.
func (c *CmdHandler) handleSetTariffCommand(ctx context.Context, m *telegram.Message) {
	if m.Chat.Username != adminUserName {
		return
	}
	conv := &ConversationState{
		Cmd:   SettCmd,
		Stage: stageSetTariffUser,
	}

	c.convs[m.Chat.ID] = conv
	msgID, _ := c.sendMessage(ctx, m.Chat.ID, "Введите username пользователя", nil)
	conv.LastMsgID = msgID
}

func (c *CmdHandler) continueSetTariffFlow(ctx context.Context, m *telegram.Message, cs *ConversationState) bool {
	switch cs.Stage {
	case stageSetTariffUser:
		username := strings.TrimPrefix(strings.TrimSpace(m.Text), "@")
		if username == "" {
			msg, _ := c.sendMessage(ctx, m.Chat.ID, "Введите username пользователя", addBack(nil))
			cs.LastMsgID = msg
			return true
		}
		cs.TargetUser = username
		tariffs := []string{"base", "plus", "premium", "ultimate"}
		prompt := "Выберите тариф"
		msgID, _ := c.sendMessage(ctx, m.Chat.ID, prompt, addBack([][]string{tariffs}))
		cs.setStage(stageSetTariffChoice)
		cs.LastMsgID = msgID
		return true
	case stageSetTariffChoice:
		choice := strings.TrimSpace(m.Text)
		if choice != "base" && choice != "plus" && choice != "premium" && choice != "ultimate" {
			msg, _ := c.sendMessage(ctx, m.Chat.ID, "Выберите тариф", addBack([][]string{{"base", "plus", "premium", "ultimate"}}))
			cs.LastMsgID = msg
			return true
		}
		cs.NewTariff = choice
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)

		if err := c.setUserTariff(ctx, cs.TargetUser, cs.NewTariff); err != nil {
			c.sendMessage(ctx, m.Chat.ID, "Ошибка: "+err.Error(), nil)
		} else {
			c.sendMessage(ctx, m.Chat.ID, "Тариф обновлен", nil)
		}
		delete(c.convs, m.Chat.ID)
		return true
	}
	return false
}
