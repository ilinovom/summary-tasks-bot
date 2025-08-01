package app

import (
	"context"
	"log"

	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
)

// handleInfoCommand sends the list of available commands.
func (a *App) handleInfoCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /info", m.Chat.ID, m.Chat.Username)
	a.sendLongMessage(ctx, m.Chat.ID, a.messages["info"])
}

// handleTariffsCommand prints information about available tariffs.
func (a *App) handleTariffsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /tariffs", m.Chat.ID, m.Chat.Username)
	a.sendLongMessage(ctx, m.Chat.ID, a.messages["tariffs"])
}

// handleSetTariffCommand is an admin-only command that changes another user's tariff.
func (a *App) handleSetTariffCommand(ctx context.Context, m *telegram.Message) {
	if m.Chat.Username != "omilinov" {
		return
	}
	conv := &conversationState{Stage: stageSetTariffUser}
	a.convs[m.Chat.ID] = conv
	msgID, _ := a.sendMessage(ctx, m.Chat.ID, "Введите username пользователя", nil)
	conv.LastMsgID = msgID
}
