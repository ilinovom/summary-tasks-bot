package app

import (
	"context"
	"log"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

// handleStartCommand processes the /start command.
// It initializes user settings and begins the welcome conversation
// if the user is unknown. Otherwise it simply reactivates the user.
func (a *App) handleStartCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d (@%s) called /start", m.Chat.ID, m.Chat.Username)
	if _, err := a.repo.Get(ctx, m.Chat.ID); err != nil {
		conv := &conversationState{Stage: stageWelcome}
		a.convs[m.Chat.ID] = conv
		msgID, err := a.sendMessage(ctx, m.Chat.ID, a.messages["start"], [][]string{{"Продолжить"}})
		if err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
		conv.LastMsgID = msgID
		return
	}
	if err := a.userService.Start(ctx, m.Chat.ID, m.Chat.Username); err != nil {
		log.Println("start:", err)
	} else {
		if _, err := a.sendMessage(ctx, m.Chat.ID, a.messages["start"], nil); err != nil {
			log.Printf("error when sending message to chat id %v: %v", m.Chat.ID, err)
		}
	}
}

// handleStopCommand processes the /stop command.
// It disables scheduled news for the user.
func (a *App) handleStopCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /stop", m.Chat.ID, m.Chat.Username)
	if err := a.userService.Stop(ctx, m.Chat.ID); err != nil {
		log.Println("stop:", err)
	} else {
		a.sendMessage(ctx, m.Chat.ID, a.messages["stopped"], nil)
	}
}
