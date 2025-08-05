package cmdHandlers

import (
	"context"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
)

// handleStartCommand processes the /start command.
// It initializes user settings and begins the welcome conversation
// if the user is unknown. Otherwise it simply reactivates the user.
func (c *CmdHandler) handleStartCommand(ctx context.Context, m *telegram.Message, isNewUser bool) {
	log.Printf("user %d (@%s) called /start", m.Chat.ID, m.Chat.Username)

	////  если пользователь заходит впервые и его нет в базе, то просим задать его категории информации
	//if _, err := c.repo.Get(ctx, m.Chat.ID); err != nil {
	//	conv := &ConversationState{
	//		Cmd:         StartCmd,
	//		Stage:       StageStartWelcome,
	//		TopicsConvP: &topicsConvParams{},
	//	}
	//	c.convs[m.Chat.ID] = conv
	//	msgID, err := c.sendMessage(ctx, m.Chat.ID, c.messages["start"], [][]string{{"Продолжить"}})
	//	if err != nil {
	//		log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
	//	}
	//	conv.LastMsgID = msgID
	//	return
	//}

	if isNewUser {
		if _, err := c.sendMessage(ctx, m.Chat.ID, c.messages["first_message"], nil); err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
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
